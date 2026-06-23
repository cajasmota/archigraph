package sched

import (
	"context"
	"testing"
	"time"

	"github.com/cajasmota/grafel/internal/indexstate"
)

// TestPublishRepoStatesDerivation drives the in-flight/queued/dirty/current
// derivation directly against the scheduler maps (no goroutines) and asserts
// publishRepoStatesLocked emits the right per-repo state, indexed_ref, and
// head_ref through the indexstate bridge (#5433).
func TestPublishRepoStatesDerivation(t *testing.T) {
	t.Cleanup(func() { indexstate.SetRepoStates(nil) })

	s := New(Config{Workers: 1})

	s.mu.Lock()
	// /idx: indexing now, head ref captured, last completed index at an older ref.
	s.inflight["/idx"] = 100
	s.pendingRefs["/idx"] = "headsha-idx"
	s.indexedRepos["/idx"] = repoStats{LastIndexedRef: "oldsha-idx"}
	// /queued: enqueued, not yet running.
	s.pendingIndex["/queued"] = true
	s.pendingRefs["/queued"] = "headsha-queued"
	// /dirty: indexing AND a follow-up already pending → strongest signal.
	s.inflight["/dirty"] = 50
	s.dirty["/dirty"] = true
	s.pendingRefs["/dirty"] = "headsha-dirty"
	// /current: fully indexed, idle.
	s.indexedRepos["/current"] = repoStats{LastIndexedRef: "sha-current"}

	s.publishRepoStatesLocked()
	s.mu.Unlock()

	byPath := map[string]indexstate.RepoState{}
	for _, st := range indexstate.RepoStates() {
		byPath[st.Path] = st
	}

	if got := byPath["/idx"]; got.State != indexstate.StateIndexing ||
		got.HeadRef != "headsha-idx" || got.IndexedRef != "oldsha-idx" {
		t.Errorf("/idx = %+v, want indexing head=headsha-idx indexed=oldsha-idx", got)
	}
	if got := byPath["/queued"]; got.State != indexstate.StateQueued || got.HeadRef != "headsha-queued" {
		t.Errorf("/queued = %+v, want queued head=headsha-queued", got)
	}
	if got := byPath["/dirty"]; got.State != indexstate.StateDirty || !got.Dirty {
		t.Errorf("/dirty = %+v, want dirty with Dirty=true", got)
	}
	if got := byPath["/current"]; got.State != indexstate.StateCurrent || got.IndexedRef != "sha-current" {
		t.Errorf("/current = %+v, want current indexed=sha-current", got)
	}
	if len(byPath) != 4 {
		t.Errorf("expected 4 repos, got %d: %+v", len(byPath), byPath)
	}
}

// TestPublishRepoStatesLiveQueuedThenCurrent proves the live path: an enqueued
// repo is observable as queued/indexing while in flight, then current after.
func TestPublishRepoStatesLiveQueuedThenCurrent(t *testing.T) {
	t.Cleanup(func() { indexstate.SetRepoStates(nil); indexstate.Set(0) })

	started := make(chan struct{})
	release := make(chan struct{})
	s := New(Config{
		Workers: 1,
		Index: func(_ context.Context, _ string, _ string) error {
			close(started)
			<-release
			return nil
		},
	})
	s.Start()
	defer s.Stop()

	s.EnqueueRef("/repo-live", "live-ref")

	select {
	case <-started:
	case <-time.After(10 * time.Second):
		t.Fatal("Index callback never entered")
	}

	var inflightState string
	for _, st := range indexstate.RepoStates() {
		if st.Path == "/repo-live" {
			inflightState = st.State
		}
	}
	if inflightState != indexstate.StateIndexing {
		t.Fatalf("mid-run state=%q want indexing", inflightState)
	}

	close(release)
	deadline := time.After(10 * time.Second)
	for {
		var cur string
		for _, st := range indexstate.RepoStates() {
			if st.Path == "/repo-live" {
				cur = st.State
			}
		}
		if cur == indexstate.StateCurrent {
			break
		}
		select {
		case <-deadline:
			t.Fatalf("repo never reached current: %+v", indexstate.RepoStates())
		case <-time.After(10 * time.Millisecond):
		}
	}
}
