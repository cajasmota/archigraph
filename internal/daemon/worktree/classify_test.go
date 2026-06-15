package worktree_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/cajasmota/grafel/internal/daemon/worktree"
)

// TestClassifyRoot_standalone asserts a real standalone repo (.git is a DIR)
// is classified as a root that should be indexed normally — NOT as a worktree.
func TestClassifyRoot_standalone(t *testing.T) {
	repo := t.TempDir()
	initGitRepo(t, repo)

	got := worktree.ClassifyRoot(repo)
	if got.Kind != worktree.RootKindStandalone {
		t.Fatalf("standalone repo classified as %v, want standalone", got.Kind)
	}
	if got.PrimaryRepoPath != "" {
		t.Fatalf("standalone repo should have no primary, got %q", got.PrimaryRepoPath)
	}
}

// TestClassifyRoot_linkedWorktree asserts a path whose .git is a worktree
// pointer FILE is classified as a linked worktree and resolves back to the
// primary checkout — so it will NOT be cold-indexed as a new root repo.
func TestClassifyRoot_linkedWorktree(t *testing.T) {
	primary := t.TempDir()
	initGitRepo(t, primary)
	wt := filepath.Join(t.TempDir(), "wt-feature")
	addWorktree(t, primary, wt, "feature-x")

	// Sanity: the worktree's .git must be a FILE, not a dir.
	fi, err := os.Lstat(filepath.Join(wt, ".git"))
	if err != nil {
		t.Fatal(err)
	}
	if fi.IsDir() {
		t.Fatalf("expected worktree .git to be a file, got dir")
	}

	got := worktree.ClassifyRoot(wt)
	if got.Kind != worktree.RootKindLinkedWorktree {
		t.Fatalf("linked worktree classified as %v, want linked_worktree", got.Kind)
	}
	// Primary must resolve back to the original checkout (EvalSymlinks because
	// macOS /var → /private/var aliases the TempDir).
	gotPrimary, _ := filepath.EvalSymlinks(got.PrimaryRepoPath)
	wantPrimary, _ := filepath.EvalSymlinks(primary)
	if gotPrimary != wantPrimary {
		t.Fatalf("primary = %q, want %q", gotPrimary, wantPrimary)
	}
}

// TestClassifyRoot_missingGit returns unknown for a non-repo directory.
func TestClassifyRoot_missingGit(t *testing.T) {
	dir := t.TempDir()
	if got := worktree.ClassifyRoot(dir); got.Kind != worktree.RootKindUnknown {
		t.Fatalf("non-repo dir classified as %v, want unknown", got.Kind)
	}
}

// TestClassifyRoot_submodulePointerNotWorktree guards against misclassifying a
// submodule (gitdir: <common>/modules/<name>) as a worktree.
func TestClassifyRoot_submodulePointerNotWorktree(t *testing.T) {
	dir := t.TempDir()
	gitFile := filepath.Join(dir, ".git")
	// Submodule-shaped pointer — "modules", not "worktrees".
	if err := os.WriteFile(gitFile, []byte("gitdir: /some/super/.git/modules/sub\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if got := worktree.ClassifyRoot(dir); got.Kind == worktree.RootKindLinkedWorktree {
		t.Fatalf("submodule pointer misclassified as linked_worktree")
	}
}

// TestIsLinkedWorktreeOf is the onboarding-gate predicate: a worktree of an
// INDEXED primary returns true (skip new-root onboarding); a worktree whose
// primary is NOT indexed returns false; a standalone repo returns false.
func TestIsLinkedWorktreeOf(t *testing.T) {
	primary := t.TempDir()
	initGitRepo(t, primary)
	wt := filepath.Join(t.TempDir(), "wt-a")
	addWorktree(t, primary, wt, "feat-a")

	// Positive: primary is in the indexed set → gate fires (skip onboarding).
	if !worktree.IsLinkedWorktreeOf(wt, []string{primary}) {
		t.Fatalf("worktree of indexed primary should be gated as linked worktree")
	}

	// Negative: primary NOT in the indexed set → do not gate (unrelated repo).
	if worktree.IsLinkedWorktreeOf(wt, []string{filepath.Join(t.TempDir(), "other")}) {
		t.Fatalf("worktree of an UNindexed primary must not be gated")
	}

	// Negative: a standalone repo is never a linked worktree.
	other := t.TempDir()
	initGitRepo(t, other)
	if worktree.IsLinkedWorktreeOf(other, []string{primary, other}) {
		t.Fatalf("standalone repo must not be classified as a linked worktree")
	}
}
