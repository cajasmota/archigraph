package indexstate

import (
	"sync"
	"testing"
	"time"
)

// TestParseAccounting covers the in-process parse busy counter (#5630): a parse
// that previously bypassed all accounting now flips ParseInFlight + Busy, and
// stamps the busy-period start when the daemon was otherwise idle.
func TestParseAccounting(t *testing.T) {
	t.Cleanup(func() { ParseEnd(); Set(0); SetParseConcurrency(0) })

	// Idle baseline.
	Set(0)
	if s := Get(); s.Busy || s.ParseInFlight != 0 || !s.StartedAt.IsZero() {
		t.Fatalf("idle: got %+v, want not-busy/0/zero-time", s)
	}

	// A parse begins: ParseInFlight=1, Busy=true, started-at stamped even though
	// no index job is registered (the exact #5630 symptom: parsing while the
	// scheduler reports idle).
	ParseBegin()
	s := Get()
	if !s.Busy || s.ParseInFlight != 1 {
		t.Fatalf("parsing: got %+v, want busy with 1 parse in flight", s)
	}
	if s.IsIndexing {
		t.Fatalf("parsing must not be conflated with index-job IsIndexing: %+v", s)
	}
	if s.StartedAt.IsZero() {
		t.Fatalf("parsing should stamp the busy-period start when otherwise idle")
	}

	// Parse ends → back to idle.
	ParseEnd()
	if s := Get(); s.Busy || s.ParseInFlight != 0 {
		t.Fatalf("after parse: got %+v, want not-busy/0", s)
	}
}

// TestParseEndClampsAtZero proves an unbalanced ParseEnd cannot drive the
// counter negative.
func TestParseEndClampsAtZero(t *testing.T) {
	t.Cleanup(func() { Set(0) })
	ParseEnd()
	ParseEnd()
	if s := Get(); s.ParseInFlight != 0 {
		t.Fatalf("clamp: got ParseInFlight=%d, want 0", s.ParseInFlight)
	}
}

// TestParseGateCap proves the in-process parse cap actually bounds concurrency:
// with cap=1 a second acquirer blocks until the first releases.
func TestParseGateCap(t *testing.T) {
	t.Cleanup(func() { SetParseConcurrency(0); Set(0) })

	SetParseConcurrency(1)
	if got := ParseConcurrencyCap(); got != 1 {
		t.Fatalf("cap: got %d, want 1", got)
	}

	// First acquire takes the only slot and bumps the busy counter.
	AcquireParseSlot()
	if s := Get(); s.ParseInFlight != 1 {
		t.Fatalf("first acquire: ParseInFlight=%d, want 1", s.ParseInFlight)
	}

	// Second acquire must block until the first releases.
	acquired := make(chan struct{})
	go func() {
		AcquireParseSlot()
		close(acquired)
	}()

	select {
	case <-acquired:
		t.Fatalf("second AcquireParseSlot returned while slot held — cap not enforced")
	case <-time.After(50 * time.Millisecond):
		// Expected: blocked.
	}

	// Releasing the first slot must wake the blocked acquirer.
	ReleaseParseSlot()
	select {
	case <-acquired:
		// Expected.
	case <-time.After(time.Second):
		t.Fatalf("second AcquireParseSlot did not wake after release — FIFO wake broken")
	}
	ReleaseParseSlot()

	if s := Get(); s.ParseInFlight != 0 {
		t.Fatalf("after both release: ParseInFlight=%d, want 0", s.ParseInFlight)
	}
}

// TestParseGateUnboundedIsNoOp proves cap=0 (the non-daemon default) never
// blocks — many concurrent acquirers all proceed immediately.
func TestParseGateUnboundedIsNoOp(t *testing.T) {
	t.Cleanup(func() { SetParseConcurrency(0); Set(0) })
	SetParseConcurrency(0)

	const n = 8
	var wg sync.WaitGroup
	done := make(chan struct{})
	for i := 0; i < n; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			AcquireParseSlot()
			defer ReleaseParseSlot()
			<-done // hold the slot; under cap=0 all n proceed without blocking
		}()
	}

	// All n should have acquired despite holding; give them a moment then check.
	deadline := time.After(time.Second)
	for {
		if s := Get(); s.ParseInFlight == n {
			break
		}
		select {
		case <-deadline:
			t.Fatalf("unbounded gate blocked: ParseInFlight=%d, want %d", Get().ParseInFlight, n)
		case <-time.After(5 * time.Millisecond):
		}
	}
	close(done)
	wg.Wait()
	if s := Get(); s.ParseInFlight != 0 {
		t.Fatalf("after release: ParseInFlight=%d, want 0", s.ParseInFlight)
	}
}

// TestRaiseCapWakesWaiters proves that raising the cap (SetParseConcurrency)
// promotes already-queued waiters — so a daemon that tightens then loosens the
// cap does not strand parses.
func TestRaiseCapWakesWaiters(t *testing.T) {
	t.Cleanup(func() { SetParseConcurrency(0); Set(0) })

	SetParseConcurrency(1)
	AcquireParseSlot() // hold the only slot

	acquired := make(chan struct{})
	go func() {
		AcquireParseSlot()
		close(acquired)
	}()
	// Ensure the second acquirer is queued.
	time.Sleep(20 * time.Millisecond)

	// Raise the cap → the queued waiter should be admitted without a release.
	SetParseConcurrency(2)
	select {
	case <-acquired:
		// Expected.
	case <-time.After(time.Second):
		t.Fatalf("raising the cap did not wake the queued waiter")
	}
	ReleaseParseSlot()
	ReleaseParseSlot()
}
