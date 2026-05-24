package groovy_test

// Issue #2114 — same self-recursion guard bug class as the Java fix.
// The guard was:
//
//	leaf := target
//	if dot := strings.LastIndexByte(target, '.'); dot >= 0 { leaf = target[dot+1:] }
//	if leaf == callerName { continue }   // BUG: fires on cross-type dotted calls
//
// When client-fixture-A.OrderController.process() called
// orderService.process(req), the resolved target "OrderService.process" had
// its leaf "process" matched against the caller's bare "process" → edge
// silently dropped as "self-recursion".
//
// Fix: restrict the self-recursion skip to bare-name (undotted) targets only
// (strings.IndexByte(target, '.') < 0 && target == callerName).

import (
	"testing"
)

// TestGroovy_CallsFieldReceiverDottedTarget_SameLeaf (#2114): a controller
// method named "process" that delegates to a PascalCase static/companion call
// "OrderService.process(req)" MUST emit a CALLS edge to "OrderService.process".
//
// Groovy's groovyCallTarget only emits dotted targets for PascalCase receivers
// (e.g. "Service.run()" → "Service.run"). For camelCase receivers it emits
// only the bare leaf. The bug fires when the PascalCase receiver's method name
// matches the caller's bare name: leaf("OrderService.process") == "process"
// == callerName "process" → edge incorrectly dropped as self-recursion.
func TestGroovy_CallsFieldReceiverDottedTarget_SameLeaf(t *testing.T) {
	src := `class OrderController {
    def process(String req) {
        OrderService.process(req)
    }
}
`
	ents := runGroovy(t, src)
	ctrl := gFind(ents, "process", "SCOPE.Operation")
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

// TestGroovy_SelfRecursionStillDropped (#2114): true self-recursion (a bare
// method calling itself without a receiver) MUST still be suppressed.
func TestGroovy_SelfRecursionStillDropped(t *testing.T) {
	src := `class Looper {
    def loop() {
        loop()
    }
}
`
	ents := runGroovy(t, src)
	looper := gFind(ents, "loop", "SCOPE.Operation")
	if looper == nil {
		t.Fatal("expected entity 'loop' (SCOPE.Operation)")
	}
	for _, r := range looper.Relationships {
		if r.Kind == "CALLS" && r.ToID == "loop" {
			t.Errorf("self-recursion should not produce a CALLS edge: %+v", r)
		}
	}
}
