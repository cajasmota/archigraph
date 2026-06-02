package daemon

import (
	"os"
	"path/filepath"
	"testing"
)

// fakeForgetter records Forget calls and returns a fixed slot count per repo.
type fakeForgetter struct {
	slots     map[string]int // repo -> slots it "holds"
	forgotten []string
}

func (f *fakeForgetter) Forget(repoPath string) int {
	f.forgotten = append(f.forgotten, repoPath)
	n := f.slots[repoPath]
	delete(f.slots, repoPath) // accounting decremented
	return n
}

// writeStore creates a fake store dir with `bytes` worth of content and returns
// its path.
func writeStore(t *testing.T, root, name string, bytes int) string {
	t.Helper()
	dir := filepath.Join(root, name)
	if err := os.MkdirAll(filepath.Join(dir, "refs", "main"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "refs", "main", "graph.fb"), make([]byte, bytes), 0o644); err != nil {
		t.Fatal(err)
	}
	return dir
}

// TestReaper_evictsVanishedRepoStore is the value-assertion for fix #1: a
// tracked repo whose directory has been deleted gets its store removed, its
// tier slots forgotten, its accounting decremented, and Untrack invoked.
func TestReaper_evictsVanishedRepoStore(t *testing.T) {
	tmp := t.TempDir()
	storeRoot := filepath.Join(tmp, "store")

	// A repo that STILL exists on disk.
	liveRepo := filepath.Join(tmp, "live-repo")
	if err := os.MkdirAll(liveRepo, 0o755); err != nil {
		t.Fatal(err)
	}
	liveStore := writeStore(t, storeRoot, "live-abcdef0123456789", 1000)

	// A repo that has VANISHED (we never create its dir).
	goneRepo := filepath.Join(tmp, "gone-worktree")
	goneStore := writeStore(t, storeRoot, "gone-fedcba9876543210", 4096)

	forgetter := &fakeForgetter{slots: map[string]int{
		liveRepo: 2,
		goneRepo: 3,
	}}
	var untracked []string

	storeFor := map[string]string{liveRepo: liveStore, goneRepo: goneStore}
	r := NewReaper(ReaperConfig{
		TrackedRepos:    func() []string { return []string{liveRepo, goneRepo} },
		StoreDirForRepo: func(repo string) string { return storeFor[repo] },
		Tier:            forgetter,
		Untrack:         func(repo string) { untracked = append(untracked, repo) },
	})

	res := r.Sweep()

	// --- vanished repo reaped ---
	if res.Vanished != 1 {
		t.Fatalf("Vanished = %d, want 1 (only gone-worktree)", res.Vanished)
	}
	if res.StoresRemoved != 1 {
		t.Fatalf("StoresRemoved = %d, want 1", res.StoresRemoved)
	}
	if _, err := os.Stat(goneStore); !os.IsNotExist(err) {
		t.Fatalf("vanished repo's store still on disk: %v", err)
	}
	if res.FreedBytes < 4096 {
		t.Fatalf("FreedBytes = %d, want >= 4096 (the graph.fb payload)", res.FreedBytes)
	}
	// tier slots forgotten + accounting decremented.
	if res.SlotsForgotten != 3 {
		t.Fatalf("SlotsForgotten = %d, want 3", res.SlotsForgotten)
	}
	if _, still := forgetter.slots[goneRepo]; still {
		t.Fatalf("vanished repo's tier accounting was not decremented")
	}
	// Untrack invoked for the vanished repo.
	if len(untracked) != 1 || untracked[0] != goneRepo {
		t.Fatalf("Untrack = %v, want [%s]", untracked, goneRepo)
	}

	// --- NEGATIVE: the live repo is untouched ---
	if _, err := os.Stat(liveStore); err != nil {
		t.Fatalf("live repo's store was wrongly removed: %v", err)
	}
	for _, f := range forgetter.forgotten {
		if f == liveRepo {
			t.Fatalf("live repo's tier slots were wrongly forgotten")
		}
	}
	if n, still := forgetter.slots[liveRepo]; !still || n != 2 {
		t.Fatalf("live repo's accounting changed: got %d (present=%v), want 2", n, still)
	}
}

// TestReaper_failSafeOnStatError asserts the reaper does NOT reap a repo when
// the path exists (a permission/IO error must never nuke a live graph). We
// model "exists" by creating the dir; a vanished dir is the only reap trigger.
func TestReaper_existingRepoNeverReaped(t *testing.T) {
	tmp := t.TempDir()
	repo := filepath.Join(tmp, "present")
	if err := os.MkdirAll(repo, 0o755); err != nil {
		t.Fatal(err)
	}
	store := writeStore(t, filepath.Join(tmp, "store"), "present-0011223344556677", 512)

	r := NewReaper(ReaperConfig{
		TrackedRepos:    func() []string { return []string{repo} },
		StoreDirForRepo: func(string) string { return store },
		Tier:            &fakeForgetter{slots: map[string]int{repo: 1}},
	})
	res := r.Sweep()

	if res.Vanished != 0 || res.StoresRemoved != 0 || res.SlotsForgotten != 0 {
		t.Fatalf("existing repo was reaped: %+v", res)
	}
	if _, err := os.Stat(store); err != nil {
		t.Fatalf("existing repo's store was removed: %v", err)
	}
}

// TestReaper_noOpWhenNoTracker guards the nil-config path.
func TestReaper_noOpWhenNoTracker(t *testing.T) {
	r := NewReaper(ReaperConfig{})
	if res := r.Sweep(); res != (ReapResult{}) {
		t.Fatalf("nil TrackedRepos should be a no-op, got %+v", res)
	}
}
