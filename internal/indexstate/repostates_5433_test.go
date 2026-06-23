package indexstate

import "testing"

// TestSetRepoStatesDefensiveCopyAndSort verifies SetRepoStates stores a copy
// (mutating the caller's slice afterwards does not corrupt the published view)
// and RepoStates returns Path-sorted rows.
func TestSetRepoStatesDefensiveCopyAndSort(t *testing.T) {
	in := []RepoState{
		{Path: "/b", State: StateCurrent},
		{Path: "/a", State: StateIndexing},
	}
	SetRepoStates(in)
	// Mutate the caller's slice after publishing — must not affect the snapshot.
	in[0].State = "MUTATED"
	in[0].Path = "/zzz"

	got := RepoStates()
	if len(got) != 2 {
		t.Fatalf("len=%d want 2", len(got))
	}
	if got[0].Path != "/a" || got[1].Path != "/b" {
		t.Fatalf("not sorted by path: %+v", got)
	}
	if got[1].State != StateCurrent {
		t.Fatalf("defensive copy failed, state=%q", got[1].State)
	}
	// Reset for other tests in the package.
	SetRepoStates(nil)
}

// TestRepoStatesEmpty verifies the zero-value path returns an empty slice.
func TestRepoStatesEmpty(t *testing.T) {
	SetRepoStates(nil)
	if got := RepoStates(); len(got) != 0 {
		t.Fatalf("want empty, got %+v", got)
	}
}
