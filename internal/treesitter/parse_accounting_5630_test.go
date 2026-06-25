package treesitter_test

import (
	"context"
	"testing"
	"time"

	"github.com/cajasmota/grafel/internal/indexstate"
	"github.com/cajasmota/grafel/internal/treesitter"
)

// TestParse_RegistersBusyCounter proves the #5630 fix: a real (non-empty) parse
// — the work that previously bypassed ALL accounting — now flips
// indexstate.ParseInFlight / Busy WHILE it runs, and returns the counter to 0
// afterwards. We observe "during" by holding the parse gate at cap=1 from a
// helper goroutine so the factory's Parse blocks in AcquireParseSlot with the
// busy counter already incremented.
func TestParse_RegistersBusyCounter(t *testing.T) {
	t.Cleanup(func() { indexstate.SetParseConcurrency(0); indexstate.Set(0) })

	// Baseline: idle.
	indexstate.Set(0)
	if s := indexstate.Get(); s.Busy {
		t.Fatalf("precondition: daemon must be idle, got %+v", s)
	}

	// Cap at 1 and take the only slot from a helper so the real Parse below must
	// queue. The act of queueing already increments ParseInFlight (AcquireParseSlot
	// calls ParseBegin before blocking), which is exactly the observability we want:
	// the daemon is provably accounted as "parsing" even before the parse runs.
	indexstate.SetParseConcurrency(1)
	indexstate.AcquireParseSlot() // helper holds the slot

	f := treesitter.NewParserFactory(nil)
	parseDone := make(chan struct{})
	go func() {
		_, _ = f.Parse(context.Background(), []byte("package main\nfunc main() {}\n"), "go")
		close(parseDone)
	}()

	// The factory's Parse must be blocked acquiring a slot, and the busy counter
	// must read 2 (helper + the queued parse).
	deadline := time.After(time.Second)
	for {
		s := indexstate.Get()
		if s.ParseInFlight == 2 {
			if !s.Busy {
				t.Fatalf("Busy must be true while parsing, got %+v", s)
			}
			break
		}
		select {
		case <-deadline:
			t.Fatalf("parse did not register as busy: ParseInFlight=%d, want 2", indexstate.Get().ParseInFlight)
		case <-parseDone:
			t.Fatalf("parse completed while the only slot was held — cap not enforced on the parse path")
		case <-time.After(2 * time.Millisecond):
		}
	}

	// Release the helper slot → the parse proceeds and balances the counter.
	indexstate.ReleaseParseSlot()
	select {
	case <-parseDone:
	case <-time.After(2 * time.Second):
		t.Fatalf("parse did not complete after the slot was freed")
	}

	if s := indexstate.Get(); s.ParseInFlight != 0 || s.Busy {
		t.Fatalf("after parse the busy counter must clear, got %+v", s)
	}
}

// TestParse_EmptySource_DoesNotAccount confirms the empty-source fast path (no
// real tree-sitter work) does not touch the busy counter — only actual parses
// count, so the signal stays meaningful.
func TestParse_EmptySource_DoesNotAccount(t *testing.T) {
	t.Cleanup(func() { indexstate.Set(0) })
	f := treesitter.NewParserFactory(nil)
	if _, err := f.Parse(context.Background(), nil, "go"); err != nil {
		t.Fatalf("empty parse: unexpected err %v", err)
	}
	if s := indexstate.Get(); s.ParseInFlight != 0 || s.Busy {
		t.Fatalf("empty-source parse must not register busy, got %+v", s)
	}
}
