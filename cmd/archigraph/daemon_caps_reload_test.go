package main

import (
	"os"
	"path/filepath"
	"runtime"
	"testing"
	"time"

	"github.com/cajasmota/archigraph/internal/daemon/caps"
)

// TestResolveDaemonGOMAXPROCSWith covers the #5137 env>file>none precedence for
// the daemon's own in-process GOMAXPROCS, including the host-ceiling no-op.
func TestResolveDaemonGOMAXPROCSWith(t *testing.T) {
	const host = 12
	cases := []struct {
		name    string
		env     string
		fileVal int
		want    int
	}{
		{"none", "", 0, 0},
		{"file-only", "", 3, 3},
		{"env-only", "5", 0, 5},
		{"env-beats-file", "5", 9, 5},
		{"file-at-host-noop", "", 12, 0},
		{"file-above-host-noop", "", 20, 0},
		{"env-at-host-noop-ignores-file", "12", 3, 0},
		{"file-nonpositive-treated-unset", "", 0, 0},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Setenv("ARCHIGRAPH_DAEMON_GOMAXPROCS", tc.env)
			if got := resolveDaemonGOMAXPROCSWith(host, tc.fileVal); got != tc.want {
				t.Fatalf("resolveDaemonGOMAXPROCSWith(host=%d, env=%q, file=%d) = %d, want %d",
					host, tc.env, tc.fileVal, got, tc.want)
			}
		})
	}
}

// writeCPUJSON writes cpu.json into dir and bumps its mtime forward so the
// caps.Store's (mtime,size) cache key changes deterministically.
func writeCPUJSON(t *testing.T, dir, content string) string {
	t.Helper()
	path := filepath.Join(dir, caps.FileName)
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatalf("write cpu.json: %v", err)
	}
	future := time.Now().Add(2 * time.Second)
	_ = os.Chtimes(path, future, future)
	return path
}

// TestApplyDaemonGOMAXPROCSFromCaps is the #5137 live re-apply proof: editing
// cpu.json and re-invoking the apply function (what the SIGHUP handler does)
// changes runtime.GOMAXPROCS with no restart, and clearing the cap restores the
// host default. The test restores the original GOMAXPROCS on exit so it does not
// leak global state into other tests.
func TestApplyDaemonGOMAXPROCSFromCaps(t *testing.T) {
	orig := runtime.GOMAXPROCS(0)
	t.Cleanup(func() { runtime.GOMAXPROCS(orig) })

	// Use a fixed synthetic host count well above any cap we set so the
	// host-ceiling no-op never interferes.
	const host = 64
	t.Setenv("ARCHIGRAPH_DAEMON_GOMAXPROCS", "") // env unset → file drives it

	dir := t.TempDir()
	store := caps.NewStore(caps.DefaultPath(dir))

	// 1. cpu.json caps the daemon to 2 → applied live.
	writeCPUJSON(t, dir, `{"daemon_gomaxprocs": 2}`)
	n, _, changed := applyDaemonGOMAXPROCSFromCaps(store, host)
	if n != 2 || !changed {
		t.Fatalf("first apply: got (n=%d, changed=%v), want (2, true)", n, changed)
	}
	if got := runtime.GOMAXPROCS(0); got != 2 {
		t.Fatalf("runtime.GOMAXPROCS not applied: got %d, want 2", got)
	}

	// 2. Re-apply with no change → no-op.
	if _, _, changed := applyDaemonGOMAXPROCSFromCaps(store, host); changed {
		t.Fatalf("re-apply with unchanged file should report changed=false")
	}

	// 3. Raise the cap to 5 → applied live without restart.
	writeCPUJSON(t, dir, `{"daemon_gomaxprocs": 5}`)
	n, prev, changed := applyDaemonGOMAXPROCSFromCaps(store, host)
	if n != 5 || prev != 2 || !changed {
		t.Fatalf("raise: got (n=%d, prev=%d, changed=%v), want (5, 2, true)", n, prev, changed)
	}
	if got := runtime.GOMAXPROCS(0); got != 5 {
		t.Fatalf("raise not applied: got %d, want 5", got)
	}

	// 4. Clear the cap → restore the host default (no restart).
	writeCPUJSON(t, dir, `{}`)
	n, _, changed = applyDaemonGOMAXPROCSFromCaps(store, host)
	if n != host || !changed {
		t.Fatalf("clear: got (n=%d, changed=%v), want (host=%d, true)", n, changed, host)
	}
	if got := runtime.GOMAXPROCS(0); got != host {
		t.Fatalf("clear not applied: got %d, want host %d", got, host)
	}
}

// TestApplyDaemonGOMAXPROCSFromCaps_NilStore: a nil store with no env cap leaves
// GOMAXPROCS at the host default (no-op when already there).
func TestApplyDaemonGOMAXPROCSFromCaps_NilStore(t *testing.T) {
	orig := runtime.GOMAXPROCS(0)
	t.Cleanup(func() { runtime.GOMAXPROCS(orig) })
	t.Setenv("ARCHIGRAPH_DAEMON_GOMAXPROCS", "")

	host := runtime.NumCPU()
	runtime.GOMAXPROCS(host) // ensure we start at host default
	n, _, changed := applyDaemonGOMAXPROCSFromCaps(nil, host)
	if n != host {
		t.Fatalf("nil store: n=%d, want host %d", n, host)
	}
	if changed {
		t.Fatalf("nil store at host default should be a no-op (changed=false)")
	}
}
