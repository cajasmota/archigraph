package daemon

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/cajasmota/grafel/internal/daemon/worktree"
)

func gitInit(t *testing.T, dir string) {
	t.Helper()
	run := func(args ...string) {
		t.Helper()
		cmd := exec.Command("git", args...)
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("%v: %v\n%s", args, err, out)
		}
	}
	run("init", "-q", dir)
	run("-C", dir, "config", "user.email", "t@t")
	run("-C", dir, "config", "user.name", "t")
	if err := os.WriteFile(filepath.Join(dir, "f"), []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	run("-C", dir, "add", ".")
	run("-C", dir, "commit", "-qm", "init")
}

// TestMakeWorktreeEnqueueGate asserts the gate fires for a linked worktree of a
// registered primary and stays silent for the primary itself and for unrelated
// standalone repos (#3680).
func TestMakeWorktreeEnqueueGate(t *testing.T) {
	primary := t.TempDir()
	gitInit(t, primary)
	wt := filepath.Join(t.TempDir(), "agent-1")
	cmd := exec.Command("git", "-C", primary, "worktree", "add", wt, "-b", "feat")
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("worktree add: %v\n%s", err, out)
	}

	gate := makeWorktreeEnqueueGate(func() []string { return []string{primary} })
	if gate == nil {
		t.Fatal("gate must be non-nil when reposToWatch is provided")
	}

	// Positive: the worktree of a registered primary is gated.
	if !gate(wt) {
		t.Errorf("linked worktree of a registered primary must be gated")
	}
	// Negative: the primary checkout itself is a normal root — never gated.
	if gate(primary) {
		t.Errorf("primary checkout must not be gated")
	}
	// Negative: an unrelated standalone repo is never gated.
	other := t.TempDir()
	gitInit(t, other)
	if gate(other) {
		t.Errorf("unrelated standalone repo must not be gated")
	}

	// nil reposToWatch disables the gate entirely.
	if makeWorktreeEnqueueGate(nil) != nil {
		t.Errorf("nil reposToWatch must yield a nil gate (legacy behaviour)")
	}
}

// TestMakeReaperTrackedRepos asserts the tracked set is the de-duplicated union
// of registered repos and active worktree children.
func TestMakeReaperTrackedRepos(t *testing.T) {
	store := worktree.NewStore(filepath.Join(t.TempDir(), "wt.json"))
	// Inject an active child via the watcher's discovery path would need git;
	// instead drive Active() by polling a real worktree.
	primary := t.TempDir()
	gitInit(t, primary)
	wtPath := filepath.Join(t.TempDir(), "child")
	cmd := exec.Command("git", "-C", primary, "worktree", "add", wtPath, "-b", "b")
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("worktree add: %v\n%s", err, out)
	}
	w := worktree.NewWatcher(store, func() []worktree.ParentRepo {
		return []worktree.ParentRepo{{GroupName: "g", Slug: "s", Path: primary}}
	}, nil)
	w.Poll() // discovers wtPath → active child

	tracked := makeReaperTrackedRepos(func() []string { return []string{primary, primary} }, store)
	got := tracked()

	canon := func(p string) string {
		if r, err := filepath.EvalSymlinks(p); err == nil {
			return r
		}
		return filepath.Clean(p)
	}
	cPrimary, cChild := canon(primary), canon(wtPath)
	wantPrimary, wantChild := false, false
	for _, p := range got {
		switch canon(p) {
		case cPrimary:
			wantPrimary = true
		case cChild:
			wantChild = true
		}
	}
	if !wantPrimary {
		t.Errorf("tracked set missing the registered primary; got %v", got)
	}
	if !wantChild {
		t.Errorf("tracked set missing the active worktree child; got %v", got)
	}
	// De-dup: primary appears once despite being listed twice.
	count := 0
	for _, p := range got {
		if canon(p) == cPrimary {
			count++
		}
	}
	if count != 1 {
		t.Errorf("primary appeared %d times, want 1 (de-dup)", count)
	}
}
