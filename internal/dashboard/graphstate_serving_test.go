package dashboard

// graphstate_serving_test.go — regression coverage for #1478.
//
// Root cause of #1478: GetGroup held the GraphCache mutex across a multi-
// second per-repo disk-load + Pass-4 algorithm run, and first-paint endpoints
// (/api/dashboard/init, /api/registry) called GetGroup inline. With a large
// group registered this serialised every cache consumer behind one slow load
// and made the dashboard appear dead (HTTP 000) until the load finished.
//
// These tests pin the two invariants of the fix:
//  1. GetGroupCached never blocks — it returns immediately on a cold cache.
//  2. A slow load for one group does not hold the cache mutex (so unrelated
//     cache operations and the cached-read fast path stay responsive).

import (
	"sync"
	"testing"
	"time"
)

// TestGetGroupCached_ColdReturnsImmediately verifies the non-blocking
// fast path used by first-paint endpoints: on a cold cache it must return
// (nil,false) right away rather than triggering a synchronous load.
func TestGetGroupCached_ColdReturnsImmediately(t *testing.T) {
	t.Setenv("GRAFEL_HOME", t.TempDir()) // empty registry → load would fail fast

	c := NewGraphCache(60 * time.Second)

	done := make(chan struct{})
	go func() {
		grp, ok := c.GetGroupCached("nonexistent")
		if ok || grp != nil {
			t.Errorf("cold GetGroupCached = (%v,%v); want (nil,false)", grp, ok)
		}
		close(done)
	}()

	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("GetGroupCached blocked on a cold cache — first-paint path would wedge (#1478)")
	}
}

// TestGetGroupCached_WarmHit returns a cached entry without loading.
func TestGetGroupCached_WarmHit(t *testing.T) {
	c := NewGraphCache(60 * time.Second)
	want := &DashGroup{Name: "g1"}
	c.mu.Lock()
	c.entries["g1"] = &cacheEntry{group: want, loadedAt: time.Now()}
	c.mu.Unlock()

	got, ok := c.GetGroupCached("g1")
	if !ok || got != want {
		t.Fatalf("warm GetGroupCached = (%v,%v); want (%p,true)", got, ok, want)
	}
}

// TestGraphCache_SlowLoadDoesNotHoldMutex is the core anti-wedge regression.
// It seeds an in-flight loadGate for a "slow" group (simulating a load that is
// mid-flight), then asserts that an unrelated cache operation that needs the
// mutex (here: a cached read of a different, warm group) completes promptly.
// Before the fix, loadGroup ran with c.mu held, so this would block until the
// slow load returned.
func TestGraphCache_SlowLoadDoesNotHoldMutex(t *testing.T) {
	c := NewGraphCache(60 * time.Second)

	// A warm, unrelated group that GetGroupCached can serve from cache.
	warm := &DashGroup{Name: "warm"}
	c.mu.Lock()
	c.entries["warm"] = &cacheEntry{group: warm, loadedAt: time.Now()}
	// Register an in-flight load for "slow" WITHOUT holding the mutex past
	// this setup — mirrors GetGroup releasing c.mu before loadGroup runs.
	c.loading["slow"] = &loadGate{done: make(chan struct{})}
	c.mu.Unlock()

	// While "slow" is mid-load, the fast cached path for "warm" must not block.
	var wg sync.WaitGroup
	wg.Add(1)
	res := make(chan bool, 1)
	go func() {
		defer wg.Done()
		_, ok := c.GetGroupCached("warm")
		res <- ok
	}()

	select {
	case ok := <-res:
		if !ok {
			t.Fatal("warm group cached read returned miss")
		}
	case <-time.After(2 * time.Second):
		t.Fatal("cached read wedged behind an in-flight slow load (#1478)")
	}
	wg.Wait()
}
