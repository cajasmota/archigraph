package caps

import (
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"
)

func writeFile(t *testing.T, path, content string) {
	t.Helper()
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatalf("write %s: %v", path, err)
	}
}

func intp(n int) *int { return &n }

// TestLoad_MissingFile is a valid "no overrides" state: zero Config, no error.
func TestLoad_MissingFile(t *testing.T) {
	s := NewStore(filepath.Join(t.TempDir(), "cpu.json"))
	cfg, err := s.Load()
	if err != nil {
		t.Fatalf("missing file should not error: %v", err)
	}
	if cfg != (Config{}) {
		t.Fatalf("missing file should yield zero Config, got %+v", cfg)
	}
}

// TestLoad_NilStoreAndEmptyPath both resolve to the zero Config.
func TestLoad_NilStoreAndEmptyPath(t *testing.T) {
	var nilStore *Store
	if cfg, err := nilStore.Load(); err != nil || cfg != (Config{}) {
		t.Fatalf("nil store: got (%+v, %v), want (zero, nil)", cfg, err)
	}
	if cfg, err := NewStore("").Load(); err != nil || cfg != (Config{}) {
		t.Fatalf("empty path: got (%+v, %v), want (zero, nil)", cfg, err)
	}
}

// TestLoad_ParsesOverrides confirms the JSON schema and the *Value() accessors.
func TestLoad_ParsesOverrides(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "cpu.json")
	writeFile(t, path, `{
		"extract_gomaxprocs": 3,
		"rebuild_gomaxprocs": 7,
		"extract_concurrency": 2,
		"daemon_gomaxprocs": 4
	}`)
	cfg, err := NewStore(path).Load()
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	if got := cfg.ExtractGOMAXPROCSValue(); got != 3 {
		t.Errorf("extract gomaxprocs = %d, want 3", got)
	}
	if got := cfg.RebuildGOMAXPROCSValue(); got != 7 {
		t.Errorf("rebuild gomaxprocs = %d, want 7", got)
	}
	if got := cfg.ExtractConcurrencyValue(); got != 2 {
		t.Errorf("extract concurrency = %d, want 2", got)
	}
	if got := cfg.DaemonGOMAXPROCSValue(); got != 4 {
		t.Errorf("daemon gomaxprocs = %d, want 4", got)
	}
}

// TestValue_UnsetAndNonPositive: nil pointer or non-positive value reads as 0.
func TestValue_UnsetAndNonPositive(t *testing.T) {
	if got := (Config{}).ExtractGOMAXPROCSValue(); got != 0 {
		t.Errorf("nil pointer should read 0, got %d", got)
	}
	if got := (Config{ExtractGOMAXPROCS: intp(0)}).ExtractGOMAXPROCSValue(); got != 0 {
		t.Errorf("zero should read 0, got %d", got)
	}
	if got := (Config{DaemonGOMAXPROCS: intp(-2)}).DaemonGOMAXPROCSValue(); got != 0 {
		t.Errorf("negative should read 0, got %d", got)
	}
}

// TestLoad_ReReadOnChange is the core #5137 live-reload proof: editing the file
// changes the resolved value on the NEXT Load with no restart, and an unchanged
// file is served from cache (no re-parse needed for correctness, but the value
// must stay stable).
func TestLoad_ReReadOnChange(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "cpu.json")
	writeFile(t, path, `{"extract_gomaxprocs": 2}`)
	s := NewStore(path)

	cfg, err := s.Load()
	if err != nil || cfg.ExtractGOMAXPROCSValue() != 2 {
		t.Fatalf("first load: got (%d, %v), want (2, nil)", cfg.ExtractGOMAXPROCSValue(), err)
	}

	// Unchanged file → same value (cache hit).
	cfg, _ = s.Load()
	if cfg.ExtractGOMAXPROCSValue() != 2 {
		t.Fatalf("cached load changed value: %d", cfg.ExtractGOMAXPROCSValue())
	}

	// Edit the file. Bump mtime forward so the (mtime,size) key changes even on
	// coarse-resolution filesystems where two writes in the same second would
	// otherwise collide.
	writeFile(t, path, `{"extract_gomaxprocs": 5}`)
	future := time.Now().Add(2 * time.Second)
	if err := os.Chtimes(path, future, future); err != nil {
		t.Fatalf("chtimes: %v", err)
	}

	cfg, err = s.Load()
	if err != nil || cfg.ExtractGOMAXPROCSValue() != 5 {
		t.Fatalf("after edit: got (%d, %v), want (5, nil) — file edit must take effect without restart",
			cfg.ExtractGOMAXPROCSValue(), err)
	}
}

// TestLoad_MalformedKeepsLastGood: a bad file surfaces the parse error but does
// NOT change the effective value from the last good parse — the daemon never
// wedges on corrupt JSON.
func TestLoad_MalformedKeepsLastGood(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "cpu.json")
	writeFile(t, path, `{"daemon_gomaxprocs": 4}`)
	s := NewStore(path)
	if cfg, err := s.Load(); err != nil || cfg.DaemonGOMAXPROCSValue() != 4 {
		t.Fatalf("good load: got (%d, %v)", cfg.DaemonGOMAXPROCSValue(), err)
	}

	writeFile(t, path, `{ this is not json `)
	future := time.Now().Add(2 * time.Second)
	_ = os.Chtimes(path, future, future)

	cfg, err := s.Load()
	if err == nil {
		t.Fatalf("malformed file should surface a parse error")
	}
	if cfg.DaemonGOMAXPROCSValue() != 4 {
		t.Fatalf("malformed file must keep last good value 4, got %d", cfg.DaemonGOMAXPROCSValue())
	}
}

// TestLoad_Concurrent exercises the mutex: the SIGHUP handler and the scheduler
// worker pool may Load concurrently.
func TestLoad_Concurrent(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "cpu.json")
	writeFile(t, path, `{"extract_concurrency": 3}`)
	s := NewStore(path)

	var wg sync.WaitGroup
	for i := 0; i < 32; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			if cfg, _ := s.Load(); cfg.ExtractConcurrencyValue() != 3 {
				t.Errorf("concurrent load got %d, want 3", cfg.ExtractConcurrencyValue())
			}
		}()
	}
	wg.Wait()
}

// TestDefaultPath joins the root with the canonical filename and treats an empty
// root as "no config".
func TestDefaultPath(t *testing.T) {
	if got := DefaultPath("/x/y"); got != filepath.Join("/x/y", FileName) {
		t.Errorf("DefaultPath = %q", got)
	}
	if got := DefaultPath(""); got != "" {
		t.Errorf("empty root should give empty path, got %q", got)
	}
}
