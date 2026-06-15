package daemon

import (
	"log/slog"
	"os"
	"path/filepath"
	"testing"
	"time"
)

// TestResolveFleetRepoPaths_SkipsNonexistentDir covers issue #2084:
// when a fleet config entry holds a relative or nonexistent path (e.g. a
// deleted worktree), ResolveFleetRepoPaths must skip it with a warning and
// never return it in the output list.  The daemon must not panic and must not
// spawn a watcher for a path that cannot be stat'd as a directory.
func TestResolveFleetRepoPaths_SkipsNonexistentDir(t *testing.T) {
	// Create one real directory and one that does not exist.
	realDir := t.TempDir()
	gonePath := filepath.Join(t.TempDir(), "deleted-worktree")
	// gonePath intentionally NOT created — it simulates a removed worktree.

	// Use a relative path for the nonexistent entry to mirror the bug report
	// (go-chi-mini.fleet.json had "path": "internal/quality/golden/go-chi-mini").
	// We pass the raw relative string; it will be resolved but won't exist.
	relativeNonexistent := "some/relative/path/to/nowhere"

	logger := slog.New(slog.NewTextHandler(os.Stderr, nil))
	got := ResolveFleetRepoPaths([]string{realDir, gonePath, relativeNonexistent}, logger)

	if len(got) != 1 {
		t.Fatalf("expected exactly 1 path (the real dir), got %d: %v", len(got), got)
	}
	if got[0] != realDir {
		t.Fatalf("expected %q, got %q", realDir, got[0])
	}
}

// TestResolveFleetRepoPaths_EmptyInput checks the nil/empty-slice edge case.
func TestResolveFleetRepoPaths_EmptyInput(t *testing.T) {
	got := ResolveFleetRepoPaths(nil, nil)
	if len(got) != 0 {
		t.Fatalf("expected empty result for nil input, got %v", got)
	}
}

// TestResolveFleetRepoPaths_FileNotDir verifies that a path that exists but is
// a regular file (not a directory) is also skipped.
func TestResolveFleetRepoPaths_FileNotDir(t *testing.T) {
	dir := t.TempDir()
	filePath := filepath.Join(dir, "notadir.txt")
	if err := os.WriteFile(filePath, []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	got := ResolveFleetRepoPaths([]string{filePath}, nil)
	if len(got) != 0 {
		t.Fatalf("expected file path to be skipped, got %v", got)
	}
}

// TestPruneStaleGenerations covers issue #2085: after simulating 3 store
// generations of the same logical repo (same base name, different 16-hex hash
// suffix), PruneStaleGenerations must remove the oldest one and retain the two
// most recent (defaultKeepGenerations = 2).
func TestPruneStaleGenerations_KeepsRecentN(t *testing.T) {
	storeDir := t.TempDir()
	base := "myrepo"

	// Create 3 generation directories with distinct mtimes.
	// We use fake 16-hex hashes that follow the naming convention.
	slots := []struct {
		name  string
		delay time.Duration
	}{
		{base + "-aabbccddeeff0011", 0},                      // oldest
		{base + "-1122334455667788", 50 * time.Millisecond},  // middle
		{base + "-99aabbccddeeff00", 100 * time.Millisecond}, // newest
	}
	for _, s := range slots {
		dir := filepath.Join(storeDir, s.name)
		if err := os.MkdirAll(dir, 0o755); err != nil {
			t.Fatal(err)
		}
		// Write a sentinel file so latestMtime has something to read.
		f := filepath.Join(dir, "graph.fb")
		if err := os.WriteFile(f, []byte("data"), 0o644); err != nil {
			t.Fatal(err)
		}
		// Space out mtimes so sorting is deterministic.
		if s.delay > 0 {
			mtime := time.Now().Add(s.delay)
			if err := os.Chtimes(f, mtime, mtime); err != nil {
				t.Fatal(err)
			}
			if err := os.Chtimes(dir, mtime, mtime); err != nil {
				t.Fatal(err)
			}
		}
	}

	logger := slog.New(slog.NewTextHandler(os.Stderr, nil))
	removed, _ := PruneStaleGenerations(storeDir, 2, logger)

	if removed != 1 {
		t.Fatalf("expected 1 generation removed (oldest), got %d", removed)
	}

	// The oldest slot must be gone.
	if _, err := os.Stat(filepath.Join(storeDir, slots[0].name)); !os.IsNotExist(err) {
		t.Errorf("oldest generation %q should have been removed", slots[0].name)
	}
	// The two most recent must still exist.
	for _, s := range slots[1:] {
		if _, err := os.Stat(filepath.Join(storeDir, s.name)); err != nil {
			t.Errorf("recent generation %q should still exist: %v", s.name, err)
		}
	}
}

// TestPruneStaleGenerations_NoPruneWhenFewSlots verifies that no directories
// are removed when the number of slots is already ≤ keepN.
func TestPruneStaleGenerations_NoPruneWhenFewSlots(t *testing.T) {
	storeDir := t.TempDir()
	base := "svc"
	// Create exactly 2 slots (= keepN default).
	for _, hash := range []string{"aabbccddeeff0011", "1122334455667788"} {
		dir := filepath.Join(storeDir, base+"-"+hash)
		if err := os.MkdirAll(dir, 0o755); err != nil {
			t.Fatal(err)
		}
	}
	removed, _ := PruneStaleGenerations(storeDir, 2, nil)
	if removed != 0 {
		t.Fatalf("expected 0 removals when slot count == keepN, got %d", removed)
	}
}

// TestKeepGenerations_EnvOverride verifies that GRAFEL_STORE_KEEP_GENERATIONS
// overrides the default.
func TestKeepGenerations_EnvOverride(t *testing.T) {
	t.Setenv("GRAFEL_STORE_KEEP_GENERATIONS", "5")
	if got := KeepGenerations(); got != 5 {
		t.Fatalf("expected 5, got %d", got)
	}
}

// TestKeepGenerations_Default verifies the default value when no env var is set.
func TestKeepGenerations_Default(t *testing.T) {
	t.Setenv("GRAFEL_STORE_KEEP_GENERATIONS", "")
	if got := KeepGenerations(); got != defaultKeepGenerations {
		t.Fatalf("expected %d (defaultKeepGenerations), got %d", defaultKeepGenerations, got)
	}
}
