package scala_test

// Issue #2114 — same self-recursion guard bug class as the Java fix.
// The guard was:
//
//	leaf := target
//	if dot := strings.LastIndexByte(target, '.'); dot >= 0 { leaf = target[dot+1:] }
//	if leaf == callerName { continue }   // BUG: fires on cross-type dotted calls
//
// When client-fixture-A.OrderController.process() called
// client-fixture-B.orderService.process(req), the resolved target
// "OrderService.process" had its leaf "process" matched against the
// caller's bare "process" → edge silently dropped as "self-recursion".
//
// Fix: restrict the self-recursion skip to bare-name (undotted) targets only
// (strings.IndexByte(target, '.') < 0 && target == callerName).

import (
	"testing"
)

// TestScala_CallsFieldReceiverDottedTarget_SameLeaf (#2114): a controller
// method named "process" that delegates to a field receiver
// "orderService.process(req)" MUST emit a CALLS edge to "OrderService.process".
// Before the fix, the leaf "process" matched callerName "process" and the edge
// was silently dropped.
//
// The Scala extractor names operations by their bare method identifier (no
// class prefix), so scalaFind uses "process" not "OrderController.process".
func TestScala_CallsFieldReceiverDottedTarget_SameLeaf(t *testing.T) {
	src := `class OrderController(orderService: OrderService) {
  def process(req: String): String = {
    orderService.process(req)
  }
}
`
	ents := runScala(t, src)
	ctrl := scalaFind(ents, "process", "SCOPE.Operation")
	if ctrl == nil {
		t.Fatal("expected entity 'process' (SCOPE.Operation)")
	}
	for _, r := range ctrl.Relationships {
		if r.Kind == "CALLS" && r.ToID == "OrderService.process" {
			return // pass
		}
	}
	t.Errorf("process has no CALLS edge to OrderService.process; got: %+v",
		ctrl.Relationships)
}

// TestScala_SelfRecursionStillDropped (#2114): true self-recursion (a bare
// method calling itself without a receiver) MUST still be suppressed.
func TestScala_SelfRecursionStillDropped(t *testing.T) {
	src := `class Looper {
  def loop(): Unit = {
    loop()
  }
}
`
	ents := runScala(t, src)
	looper := scalaFind(ents, "loop", "SCOPE.Operation")
	if looper == nil {
		t.Fatal("expected entity 'loop' (SCOPE.Operation)")
	}
	for _, r := range looper.Relationships {
		if r.Kind == "CALLS" && (r.ToID == "loop" || r.ToID == "Looper.loop") {
			t.Errorf("self-recursion should not produce a CALLS edge: %+v", r)
		}
	}
}
