package csharp_test

// Issue #2114 — same self-recursion guard bug class as the Java fix.
// The guard was:
//
//	leaf := target
//	if dot := strings.LastIndexByte(target, '.'); dot >= 0 { leaf = target[dot+1:] }
//	if leaf == callerName { continue }   // BUG: fires on cross-type dotted calls
//
// When client-fixture-A.OrderController.Process() called
// _orderService.Process(req), the resolved target "OrderService.Process" had
// its leaf "Process" matched against the caller's bare "Process" → edge
// silently dropped as "self-recursion".
//
// Fix: restrict the self-recursion skip to bare-name (undotted) targets only
// (strings.IndexByte(target, '.') < 0 && target == callerName).

import (
	"testing"
)

// TestCSharp_CallsFieldReceiverDottedTarget_SameLeaf (#2114): a controller
// method named "Process" that delegates to a field receiver
// "_orderService.Process(req)" MUST emit a CALLS edge to "OrderService.Process".
// Before the fix, the leaf "Process" matched callerName "Process" and the edge
// was silently dropped.
func TestCSharp_CallsFieldReceiverDottedTarget_SameLeaf(t *testing.T) {
	src := `
public class OrderController
{
    private readonly OrderService _orderService;

    public OrderController(OrderService orderService)
    {
        _orderService = orderService;
    }

    public string Process(string req)
    {
        return _orderService.Process(req);
    }
}
`
	ents := runCSharp(t, src)
	ctrl := csFind(ents, "OrderController.Process", "SCOPE.Operation")
	if ctrl == nil {
		t.Fatal("expected entity 'OrderController.Process' (SCOPE.Operation)")
	}
	for _, r := range ctrl.Relationships {
		if r.Kind == "CALLS" && r.ToID == "OrderService.Process" {
			return // pass
		}
	}
	t.Errorf("OrderController.Process has no CALLS edge to OrderService.Process; got: %+v",
		ctrl.Relationships)
}

// TestCSharp_SelfRecursionStillDropped (#2114): true self-recursion (a bare
// method calling itself without a receiver) MUST still be suppressed.
func TestCSharp_SelfRecursionStillDropped(t *testing.T) {
	src := `
public class Looper
{
    public void Loop() { Loop(); }
}
`
	ents := runCSharp(t, src)
	looper := csFind(ents, "Looper.Loop", "SCOPE.Operation")
	if looper == nil {
		t.Fatal("expected entity 'Looper.Loop' (SCOPE.Operation)")
	}
	for _, r := range looper.Relationships {
		if r.Kind == "CALLS" && (r.ToID == "Loop" || r.ToID == "Looper.Loop") {
			t.Errorf("self-recursion should not produce a CALLS edge: %+v", r)
		}
	}
}
