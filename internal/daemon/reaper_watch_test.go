package daemon

import (
	"path/filepath"
	"testing"

	"github.com/cajasmota/grafel/internal/daemon/watchreg"
)

// TestReaper_sweepsWatchRegistry verifies the #5142 wiring: Reaper.Sweep drops
// dead watcher PIDs and reaps orphaned ones via the injected WatchRegistry,
// independent of the vanished-repo GC. It uses the registry's real on-disk
// store but relies on the package-level liveness probe — so we register only
// PIDs that are genuinely dead (a never-allocated high PID and the reaper's own
// orphan logic) to keep the assertion deterministic without spawning processes.
func TestReaper_sweepsWatchRegistry(t *testing.T) {
	reg := watchreg.New(filepath.Join(t.TempDir(), watchreg.FileName))

	// A PID that is certainly dead (process enumeration treats it as gone).
	deadPID := 999_999_999
	if err := reg.Register(watchreg.Entry{PID: deadPID, Repo: "/gone", OwnerDaemonPID: 4242}); err != nil {
		t.Fatalf("register: %v", err)
	}

	r := NewReaper(ReaperConfig{
		WatchRegistry: reg,
		// No TrackedRepos → vanished-repo GC is a no-op; only watcher sweep runs.
	})
	res := r.Sweep()
	if res.WatchersReaped != 1 {
		t.Fatalf("WatchersReaped = %d, want 1 (dead PID reaped)", res.WatchersReaped)
	}
	got, _ := reg.List()
	if len(got) != 0 {
		t.Fatalf("dead watcher entry should be gone, registry has %d entries", len(got))
	}
}

// TestReaper_watchSweepDisabledWhenNil: with no WatchRegistry, the watcher
// sweep is a no-op and the zero-tracker result stays empty (matches the
// existing TestReaper_noOpWhenNoTracker contract).
func TestReaper_watchSweepDisabledWhenNil(t *testing.T) {
	r := NewReaper(ReaperConfig{})
	if res := r.Sweep(); res != (ReapResult{}) {
		t.Fatalf("nil WatchRegistry + nil TrackedRepos should yield zero result, got %+v", res)
	}
}
