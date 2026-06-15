package mcp

import (
	"testing"

	"github.com/cajasmota/grafel/internal/graph"
)

// neighbors4242Doc builds the fixture that reproduces the live deploy-10 bug
// (#4242): non-CALLS edges were surfaced on the forward (callees/out) side but
// silently dropped from the reverse (callers/in) side, and no edge was labelled
// with its kind.
//
// Topology:
//
//	DevicesService    --INJECTED_INTO--> DevicesReadController   (NestJS DI)
//	DeviceHandler     --THROWS-->        DeviceNotFoundException (error flow)
//	OrderService      --CALLS-->         PaymentGateway          (control: CALLS)
func neighbors4242Doc() *graph.Document {
	return minDoc(
		[]graph.Entity{
			{ID: "svc", Name: "DevicesService", Kind: "SCOPE.Component", SourceFile: "devices.service.ts", StartLine: 10},
			{ID: "ctrl", Name: "DevicesReadController", Kind: "SCOPE.Component", SourceFile: "devices.read.controller.ts", StartLine: 5},
			{ID: "handler", Name: "DeviceHandler", Kind: "SCOPE.Operation", SourceFile: "device.handler.ts", StartLine: 20},
			{ID: "exc", Name: "DeviceNotFoundException", Kind: "SCOPE.Schema", SourceFile: "errors.ts", StartLine: 1},
			{ID: "order", Name: "OrderService", Kind: "SCOPE.Operation", SourceFile: "order.service.ts", StartLine: 3},
			{ID: "gateway", Name: "PaymentGateway", Kind: "SCOPE.Operation", SourceFile: "payment.ts", StartLine: 7},
		},
		[]graph.Relationship{
			{FromID: "svc", ToID: "ctrl", Kind: "INJECTED_INTO"},
			{FromID: "handler", ToID: "exc", Kind: "THROWS"},
			{FromID: "order", ToID: "gateway", Kind: "CALLS"},
		},
	)
}

// findNeighbor returns the neighbor entry whose name matches, or nil.
func findNeighbor(list []any, name string) map[string]any {
	for _, item := range list {
		if m, ok := item.(map[string]any); ok && m["name"] == name {
			return m
		}
	}
	return nil
}

// TestNeighbors4242_InboundSurfacesInjectedInto is the headline non-vacuous
// assertion: neighbors(Controller, direction=in) must return the Service that
// was INJECTED_INTO it, labelled edge_kind=INJECTED_INTO. On the pre-fix code
// the inbound walk was restricted to inboundRefKinds (CALLS/REFERENCES/…),
// which excludes INJECTED_INTO, so callers came back empty — this test FAILS
// on pre-fix code, proving it reproduces the live bug.
func TestNeighbors4242_InboundSurfacesInjectedInto(t *testing.T) {
	srv := newTestServer(t, neighbors4242Doc())

	out := callFlowTool(t, srv.handleFindCallers, map[string]any{
		"entity_id": "ctrl",
		"depth":     float64(1),
	})
	callers, ok := out["callers"].([]any)
	if !ok {
		t.Fatalf("expected callers array, got %T", out["callers"])
	}
	svc := findNeighbor(callers, "DevicesService")
	if svc == nil {
		t.Fatalf("INJECTED_INTO predecessor DevicesService missing from callers (the #4242 bug): %v", callers)
	}
	if svc["edge_kind"] != "INJECTED_INTO" {
		t.Errorf("expected edge_kind=INJECTED_INTO, got %v", svc["edge_kind"])
	}
}

// TestNeighbors4242_InboundSurfacesThrows asserts the THROWS error-flow edge is
// reachable from the exception type via direction=in, labelled edge_kind=THROWS.
// Also fails on pre-fix code (THROWS is not in inboundRefKinds).
func TestNeighbors4242_InboundSurfacesThrows(t *testing.T) {
	srv := newTestServer(t, neighbors4242Doc())

	out := callFlowTool(t, srv.handleFindCallers, map[string]any{
		"entity_id": "exc",
		"depth":     float64(1),
	})
	callers, ok := out["callers"].([]any)
	if !ok {
		t.Fatalf("expected callers array, got %T", out["callers"])
	}
	h := findNeighbor(callers, "DeviceHandler")
	if h == nil {
		t.Fatalf("THROWS predecessor DeviceHandler missing from callers (the #4242 bug): %v", callers)
	}
	if h["edge_kind"] != "THROWS" {
		t.Errorf("expected edge_kind=THROWS, got %v", h["edge_kind"])
	}
}

// TestNeighbors4242_OutboundStillWorks proves the forward side is symmetric and
// also labels the edge kind. neighbors(Service, direction=out) returns the
// Controller it is INJECTED_INTO; neighbors(Handler, direction=out) returns the
// exception it THROWS — each labelled with edge_kind.
func TestNeighbors4242_OutboundStillWorks(t *testing.T) {
	srv := newTestServer(t, neighbors4242Doc())

	out := callFlowTool(t, srv.handleFindCallees, map[string]any{
		"entity_id": "svc",
		"depth":     float64(1),
	})
	callees, ok := out["callees"].([]any)
	if !ok {
		t.Fatalf("expected callees array, got %T", out["callees"])
	}
	ctrl := findNeighbor(callees, "DevicesReadController")
	if ctrl == nil {
		t.Fatalf("INJECTED_INTO callee DevicesReadController missing: %v", callees)
	}
	if ctrl["edge_kind"] != "INJECTED_INTO" {
		t.Errorf("expected callee edge_kind=INJECTED_INTO, got %v", ctrl["edge_kind"])
	}

	out2 := callFlowTool(t, srv.handleFindCallees, map[string]any{
		"entity_id": "handler",
		"depth":     float64(1),
	})
	callees2, _ := out2["callees"].([]any)
	exc := findNeighbor(callees2, "DeviceNotFoundException")
	if exc == nil {
		t.Fatalf("THROWS callee DeviceNotFoundException missing: %v", callees2)
	}
	if exc["edge_kind"] != "THROWS" {
		t.Errorf("expected callee edge_kind=THROWS, got %v", exc["edge_kind"])
	}
}

// TestNeighbors4242_CallsUnaffected asserts the existing CALLS behaviour is a
// strict superset: a CALLS predecessor/successor still resolves on both sides
// and is now labelled edge_kind=CALLS (additive, no field removed).
func TestNeighbors4242_CallsUnaffected(t *testing.T) {
	srv := newTestServer(t, neighbors4242Doc())

	in := callFlowTool(t, srv.handleFindCallers, map[string]any{
		"entity_id": "gateway",
		"depth":     float64(1),
	})
	callers, _ := in["callers"].([]any)
	order := findNeighbor(callers, "OrderService")
	if order == nil {
		t.Fatalf("CALLS caller OrderService regressed (missing): %v", callers)
	}
	if order["edge_kind"] != "CALLS" {
		t.Errorf("expected edge_kind=CALLS, got %v", order["edge_kind"])
	}

	out := callFlowTool(t, srv.handleFindCallees, map[string]any{
		"entity_id": "order",
		"depth":     float64(1),
	})
	callees, _ := out["callees"].([]any)
	gw := findNeighbor(callees, "PaymentGateway")
	if gw == nil {
		t.Fatalf("CALLS callee PaymentGateway regressed (missing): %v", callees)
	}
	if gw["edge_kind"] != "CALLS" {
		t.Errorf("expected callee edge_kind=CALLS, got %v", gw["edge_kind"])
	}
}

// TestNeighbors4242_BothMergesDirections asserts the unified neighbors(both)
// tool merges the now-symmetric in/out sets: querying the Controller returns
// the INJECTED_INTO Service under callers (the in side) — the exact shape the
// rewrite oracle consumes.
func TestNeighbors4242_BothMergesDirections(t *testing.T) {
	srv := newTestServer(t, neighbors4242Doc())

	out := callFlowTool(t, srv.handleNeighbors, map[string]any{
		"entity_id": "ctrl",
		"direction": "both",
		"depth":     float64(1),
	})
	callers, ok := out["callers"].([]any)
	if !ok {
		t.Fatalf("expected callers array in both-merge, got %T (%v)", out["callers"], out)
	}
	if findNeighbor(callers, "DevicesService") == nil {
		t.Fatalf("direction=both dropped the INJECTED_INTO caller DevicesService: %v", out)
	}
}

// TestNeighbors4242_ContainsStillExcluded guards the #1915 contract: the
// broadened acceptance must NOT let a structural CONTAINS parent leak back in
// as a caller.
func TestNeighbors4242_ContainsStillExcluded(t *testing.T) {
	doc := minDoc(
		[]graph.Entity{
			{ID: "file", Name: "mod.ts", Kind: "SCOPE.Component", SourceFile: "mod.ts"},
			{ID: "fn", Name: "doThing", Kind: "SCOPE.Operation", SourceFile: "mod.ts", StartLine: 4},
			{ID: "real", Name: "RealCaller", Kind: "SCOPE.Operation", SourceFile: "caller.ts", StartLine: 2},
		},
		[]graph.Relationship{
			{FromID: "file", ToID: "fn", Kind: "CONTAINS"},
			{FromID: "real", ToID: "fn", Kind: "CALLS"},
		},
	)
	srv := newTestServer(t, doc)
	out := callFlowTool(t, srv.handleFindCallers, map[string]any{
		"entity_id": "fn",
		"depth":     float64(1),
	})
	callers, _ := out["callers"].([]any)
	if findNeighbor(callers, "mod.ts") != nil {
		t.Errorf("CONTAINS parent mod.ts leaked into callers: %v", callers)
	}
	if findNeighbor(callers, "RealCaller") == nil {
		t.Errorf("real CALLS caller missing: %v", callers)
	}
}
