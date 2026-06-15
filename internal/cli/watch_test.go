package cli

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/cajasmota/grafel/internal/daemon"
	"github.com/cajasmota/grafel/internal/registry"
)

// TestDetectGraphChanges_TriggersOnMtimeBump verifies the watcher's
// inner change-detection helper: a registered group-mate's graph.json
// being touched between ticks must surface in detectGraphChanges' output
// so the surrounding loop knows to re-run the link passes.
func TestDetectGraphChanges_TriggersOnMtimeBump(t *testing.T) {
	home := withSandboxHome(t)

	// Two repos, registered as a group.
	repoA := filepath.Join(home, "repos", "alpha")
	repoB := filepath.Join(home, "repos", "beta")
	for _, r := range []string{repoA, repoB} {
		if err := os.MkdirAll(daemon.StateDirForRepo(r), 0o755); err != nil {
			t.Fatal(err)
		}
		gj := daemon.GraphPathForRepo(r)
		if err := os.WriteFile(gj, []byte(`{"version":1,"repo":"x","entities":[],"relationships":[]}`), 0o644); err != nil {
			t.Fatal(err)
		}
	}

	// Register the group.
	cfgDir, err := registry.ConfigDir()
	if err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(cfgDir, 0o755); err != nil {
		t.Fatal(err)
	}
	cfgPath := filepath.Join(cfgDir, "g.fleet.json")
	cfg := registry.GroupConfig{
		Name: "g",
		Repos: []registry.Repo{
			{Slug: "alpha", Path: repoA},
			{Slug: "beta", Path: repoB},
		},
	}
	b, _ := json.Marshal(cfg)
	if err := os.WriteFile(cfgPath, b, 0o644); err != nil {
		t.Fatal(err)
	}
	if err := registry.AddGroup("g", cfgPath); err != nil {
		t.Fatal(err)
	}

	// Initial snapshot.
	prev := snapshotGraphMtimes(repoA, "")
	if len(prev) != 2 {
		t.Fatalf("snapshot expected 2 graph.json entries, got %d (%v)", len(prev), prev)
	}

	// First call without changes: no group should be reported.
	if got := detectGraphChanges(repoA, "", prev); len(got) != 0 {
		t.Fatalf("expected no changes initially, got %v", got)
	}

	// Bump beta's graph.json mtime forward by 2s.
	gjBeta := daemon.GraphPathForRepo(repoB)
	future := time.Now().Add(2 * time.Second)
	if err := os.Chtimes(gjBeta, future, future); err != nil {
		t.Fatal(err)
	}

	got := detectGraphChanges(repoA, "", prev)
	if len(got) != 1 || got[0] != "g" {
		t.Fatalf("expected group 'g' to be reported as changed, got %v", got)
	}

	// Second call with no further mtime change: should report nothing.
	if again := detectGraphChanges(repoA, "", prev); len(again) != 0 {
		t.Fatalf("change should not repeat without a new mtime bump, got %v", again)
	}
}

// TestRunWatch_TriggersLinksHookOnGraphChange asserts the live watcher
// loop wires the RunLinks hook when a group-mate's graph.json mtime
// advances between polling ticks.
func TestRunWatch_TriggersLinksHookOnGraphChange(t *testing.T) {
	home := withSandboxHome(t)

	repoA := filepath.Join(home, "repos", "alpha")
	repoB := filepath.Join(home, "repos", "beta")
	for _, r := range []string{repoA, repoB} {
		if err := os.MkdirAll(daemon.StateDirForRepo(r), 0o755); err != nil {
			t.Fatal(err)
		}
		gj := daemon.GraphPathForRepo(r)
		if err := os.WriteFile(gj,
			[]byte(`{"version":1,"repo":"x","entities":[],"relationships":[]}`), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	cfgDir, _ := registry.ConfigDir()
	if err := os.MkdirAll(cfgDir, 0o755); err != nil {
		t.Fatal(err)
	}
	cfgPath := filepath.Join(cfgDir, "g.fleet.json")
	cfg := registry.GroupConfig{Name: "g", Repos: []registry.Repo{
		{Slug: "alpha", Path: repoA}, {Slug: "beta", Path: repoB},
	}}
	b, _ := json.Marshal(cfg)
	_ = os.WriteFile(cfgPath, b, 0o644)
	_ = registry.AddGroup("g", cfgPath)

	// Pre-snapshot mirroring the watcher's first read.
	prev := snapshotGraphMtimes(repoA, "")

	// Bump alpha's mtime forward.
	future := time.Now().Add(3 * time.Second)
	if err := os.Chtimes(daemon.GraphPathForRepo(repoA), future, future); err != nil {
		t.Fatal(err)
	}

	// Install a counting RunLinks hook and call detectGraphChanges
	// directly (the daemon loop is otherwise time-driven).
	called := []string{}
	prevHooks := activeHooks
	activeHooks = Hooks{RunLinks: func(group string) error {
		called = append(called, group)
		return nil
	}}
	t.Cleanup(func() { activeHooks = prevHooks })

	changed := detectGraphChanges(repoA, "", prev)
	for _, g := range changed {
		if activeHooks.RunLinks != nil {
			_ = activeHooks.RunLinks(g)
		}
	}
	if len(called) != 1 || called[0] != "g" {
		t.Fatalf("expected RunLinks hook to fire once for group 'g', got %v", called)
	}
}

// TestWatchBackoff_SleepSchedule verifies the standalone watcher's
// exponential backoff schedule (issue #5140): the Nth consecutive
// failure sleeps base*2^(N-1), capped at max.
func TestWatchBackoff_SleepSchedule(t *testing.T) {
	c := watchBackoffConfig{base: 1 * time.Second, max: 8 * time.Second, maxConsecutive: 10}
	cases := []struct {
		failures int
		want     time.Duration
	}{
		{1, 1 * time.Second},
		{2, 2 * time.Second},
		{3, 4 * time.Second},
		{4, 8 * time.Second},
		{5, 8 * time.Second},  // capped
		{99, 8 * time.Second}, // still capped, no overflow
	}
	for _, tc := range cases {
		if got := c.backoffSleep(tc.failures); got != tc.want {
			t.Errorf("backoffSleep(%d) = %s, want %s", tc.failures, got, tc.want)
		}
	}
}

// TestWatchBackoff_ShouldDie verifies the consecutive-failure ceiling:
// the watcher exits (rather than tight-looping) once it has hit
// maxConsecutive back-to-back daemon-unreachable failures (issue #5140).
func TestWatchBackoff_ShouldDie(t *testing.T) {
	c := watchBackoffConfig{base: time.Second, max: time.Minute, maxConsecutive: 3}
	for _, tc := range []struct {
		failures int
		want     bool
	}{
		{0, false},
		{1, false},
		{2, false},
		{3, true},
		{4, true},
	} {
		if got := c.shouldDie(tc.failures); got != tc.want {
			t.Errorf("shouldDie(%d) = %v, want %v", tc.failures, got, tc.want)
		}
	}

	// maxConsecutive == 0 disables the ceiling entirely.
	never := watchBackoffConfig{base: time.Second, max: time.Minute, maxConsecutive: 0}
	if never.shouldDie(1000) {
		t.Fatal("maxConsecutive==0 must never trigger shouldDie")
	}
}

// TestRunWatch_BacksOffAndDiesWhenDaemonUnreachable is an end-to-end
// check that runWatch returns (exits) after the consecutive-failure
// ceiling instead of looping forever when the daemon is not running
// (issue #5140). No live daemon is started; Dial fails fast.
func TestRunWatch_BacksOffAndDiesWhenDaemonUnreachable(t *testing.T) {
	home := withSandboxHome(t)
	repo := filepath.Join(home, "repos", "solo")
	if err := os.MkdirAll(repo, 0o755); err != nil {
		t.Fatal(err)
	}

	// Substitute a fast backoff so the test finishes in well under a
	// second while still exercising the real backoff+die loop.
	prev := activeWatchBackoff
	activeWatchBackoff = func() watchBackoffConfig {
		return watchBackoffConfig{base: time.Millisecond, max: 5 * time.Millisecond, maxConsecutive: 3}
	}
	t.Cleanup(func() { activeWatchBackoff = prev })

	done := make(chan error, 1)
	go func() {
		done <- runWatch(repo, "", 5*time.Millisecond)
	}()

	select {
	case err := <-done:
		if err == nil {
			t.Fatal("expected runWatch to return a give-up error, got nil")
		}
	case <-time.After(30 * time.Second):
		t.Fatal("runWatch did not exit after repeated daemon-unreachable failures (issue #5140 regression)")
	}
}
