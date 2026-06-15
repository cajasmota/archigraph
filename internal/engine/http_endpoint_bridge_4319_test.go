package engine

import (
	"testing"

	"github.com/cajasmota/grafel/internal/types"
)

// #4319 — bridge HTTP endpoint definitions to their handler operations across
// frameworks. The shared mechanism is the source_handler → IMPLEMENTS resolver
// in ResolveHTTPEndpointHandlers. Before #4319 the resolver matched the
// synthesizer's BARE source_handler name (e.g. "Controller:merge") only against
// a handler entity whose Name was character-for-character equal. When an
// extractor lands a controller method QUALIFIED (`Controller.merge` — the shape
// Django/Spring/ASP.NET handlers already use) the lookup missed and the
// http_endpoint_definition was dropped → graph island (the handler's
// VALIDATES/CALLS/RETURNS edges unreachable from the endpoint). The fix adds a
// same-file bare↔qualified reconciliation step.

// implementsEdgeFor returns the IMPLEMENTS edge on `handler` that targets the
// http_endpoint_definition, or a zero RelationshipRecord if absent.
func implementsEdgeFor(handler types.EntityRecord) (types.RelationshipRecord, bool) {
	for _, r := range handler.Relationships {
		if r.Kind == implementsEdgeKind {
			return r, true
		}
	}
	return types.RelationshipRecord{}, false
}

// TestBridge4319_NestJSQualifiedHandler is the concrete NestJS validation case
// from the issue: a @Post('merge') route whose handler method is indexed as a
// qualified `BuildingController.mergeMaintenanceEvaluations` SCOPE.Operation
// carrying VALIDATES (payload DTO) + CALLS (downstream service). Traversing
// from the endpoint must reach the handler (and therefore its VALIDATES/CALLS).
func TestBridge4319_NestJSQualifiedHandler(t *testing.T) {
	handler := types.EntityRecord{
		Kind:       "SCOPE.Operation",
		Name:       "BuildingController.mergeMaintenanceEvaluations",
		SourceFile: "src/buildings/building.controller.ts",
		Language:   "typescript",
		Subtype:    "method",
		Relationships: []types.RelationshipRecord{
			{Kind: string(types.RelationshipKindValidates), ToID: "Class:BuildingMeMergeBody"},
			{Kind: string(types.RelationshipKindCalls), ToID: "SCOPE.Operation:BuildingService.mergeMaintenanceEvaluations"},
		},
	}
	synth := types.EntityRecord{
		Kind:       httpEndpointDefinitionKind,
		Name:       "http:POST:/inspections/me-manage/merge",
		SourceFile: "src/buildings/building.controller.ts",
		Language:   "typescript",
		Properties: map[string]string{
			"verb":           "POST",
			"path":           "/inspections/me-manage/merge",
			"framework":      "nestjs",
			"pattern_type":   "http_endpoint_synthesis",
			"source_handler": "Controller:mergeMaintenanceEvaluations",
		},
	}
	merged := []types.EntityRecord{handler, synth}
	out, stats := ResolveHTTPEndpointHandlers(merged)

	if stats.HandlerResolved != 1 || stats.HandlerDropped != 0 {
		t.Fatalf("expected resolved=1 dropped=0, got %+v", stats)
	}
	if len(out) != 2 {
		t.Fatalf("endpoint must not be dropped: got %d entities", len(out))
	}
	edge, ok := implementsEdgeFor(out[0])
	if !ok {
		t.Fatal("ISLAND: no IMPLEMENTS edge from qualified NestJS handler to endpoint (#4319)")
	}
	if edge.ToID != "http_endpoint_definition:http:POST:/inspections/me-manage/merge" {
		t.Errorf("IMPLEMENTS targets wrong endpoint: %s", edge.ToID)
	}
	// Traversing endpoint → handler reaches the handler's VALIDATES + CALLS.
	if _, ok := out[0].Properties["source_handler"]; ok {
		t.Error("source_handler should be cleared once bridged")
	}
	var sawValidates, sawCalls bool
	for _, r := range out[0].Relationships {
		switch r.Kind {
		case string(types.RelationshipKindValidates):
			sawValidates = true
		case string(types.RelationshipKindCalls):
			sawCalls = true
		}
	}
	if !sawValidates || !sawCalls {
		t.Errorf("bridged handler lost its downstream edges: validates=%v calls=%v", sawValidates, sawCalls)
	}
}

// TestBridge4319_ExpressQualifiedHandler covers a second framework (Express)
// proving the bridge is generalized, not a NestJS special-case. Express stamps
// `source_handler=Controller:<handler>` bare; here the handler is indexed
// qualified as `UserController.listUsers`.
func TestBridge4319_ExpressQualifiedHandler(t *testing.T) {
	handler := types.EntityRecord{
		Kind:       "SCOPE.Operation",
		Name:       "UserController.listUsers",
		SourceFile: "src/routes/users.ts",
		Language:   "typescript",
		Subtype:    "method",
		Relationships: []types.RelationshipRecord{
			{Kind: string(types.RelationshipKindCalls), ToID: "SCOPE.Operation:UserService.findAll"},
		},
	}
	synth := types.EntityRecord{
		Kind:       httpEndpointDefinitionKind,
		Name:       "http:GET:/users",
		SourceFile: "src/routes/users.ts",
		Language:   "typescript",
		Properties: map[string]string{
			"verb":           "GET",
			"path":           "/users",
			"framework":      "express",
			"pattern_type":   "http_endpoint_synthesis",
			"source_handler": "Controller:listUsers",
		},
	}
	out, stats := ResolveHTTPEndpointHandlers([]types.EntityRecord{handler, synth})
	if stats.HandlerResolved != 1 || stats.HandlerDropped != 0 {
		t.Fatalf("expected resolved=1 dropped=0, got %+v", stats)
	}
	if _, ok := implementsEdgeFor(out[0]); !ok {
		t.Fatal("ISLAND: Express qualified handler not bridged (#4319)")
	}
}

// TestBridge4319_BareHandlerStillResolves guards that the pre-#4319 happy path
// (synthesizer bare name == handler bare Name) keeps working unchanged.
func TestBridge4319_BareHandlerStillResolves(t *testing.T) {
	handler := types.EntityRecord{
		Kind:       "SCOPE.Operation",
		Name:       "createOrder",
		SourceFile: "src/orders/orders.controller.ts",
		Language:   "typescript",
	}
	synth := types.EntityRecord{
		Kind:       httpEndpointDefinitionKind,
		Name:       "http:POST:/orders",
		SourceFile: "src/orders/orders.controller.ts",
		Language:   "typescript",
		Properties: map[string]string{
			"framework": "nestjs", "pattern_type": "http_endpoint_synthesis",
			"source_handler": "Controller:createOrder",
		},
	}
	out, stats := ResolveHTTPEndpointHandlers([]types.EntityRecord{handler, synth})
	if stats.HandlerResolved != 1 {
		t.Fatalf("bare-name happy path regressed: %+v", stats)
	}
	if _, ok := implementsEdgeFor(out[0]); !ok {
		t.Fatal("bare-name handler should still bridge")
	}
}

// TestBridge4319_MultiMethodControllerMapsEachEndpointToOwnHandler is the
// critical mis-bridge guard: a controller with two qualified handler methods in
// ONE file must bridge each endpoint to ITS OWN handler, never the wrong one.
func TestBridge4319_MultiMethodControllerMapsEachEndpointToOwnHandler(t *testing.T) {
	file := "src/buildings/building.controller.ts"
	getHandler := types.EntityRecord{
		Kind: "SCOPE.Operation", Name: "BuildingController.getBuilding",
		SourceFile: file, Language: "typescript",
		Relationships: []types.RelationshipRecord{{Kind: string(types.RelationshipKindCalls), ToID: "X:getOne"}},
	}
	mergeHandler := types.EntityRecord{
		Kind: "SCOPE.Operation", Name: "BuildingController.mergeMaintenanceEvaluations",
		SourceFile: file, Language: "typescript",
		Relationships: []types.RelationshipRecord{{Kind: string(types.RelationshipKindCalls), ToID: "X:merge"}},
	}
	getSynth := types.EntityRecord{
		Kind: httpEndpointDefinitionKind, Name: "http:GET:/buildings/{id}",
		SourceFile: file, Language: "typescript",
		Properties: map[string]string{"framework": "nestjs", "pattern_type": "http_endpoint_synthesis", "source_handler": "Controller:getBuilding"},
	}
	mergeSynth := types.EntityRecord{
		Kind: httpEndpointDefinitionKind, Name: "http:POST:/inspections/me-manage/merge",
		SourceFile: file, Language: "typescript",
		Properties: map[string]string{"framework": "nestjs", "pattern_type": "http_endpoint_synthesis", "source_handler": "Controller:mergeMaintenanceEvaluations"},
	}
	merged := []types.EntityRecord{getHandler, mergeHandler, getSynth, mergeSynth}
	out, stats := ResolveHTTPEndpointHandlers(merged)
	if stats.HandlerResolved != 2 || stats.HandlerDropped != 0 {
		t.Fatalf("expected both endpoints bridged, got %+v", stats)
	}
	// Build name → IMPLEMENTS-target map and assert correct pairing.
	want := map[string]string{
		"BuildingController.getBuilding":                 "http_endpoint_definition:http:GET:/buildings/{id}",
		"BuildingController.mergeMaintenanceEvaluations": "http_endpoint_definition:http:POST:/inspections/me-manage/merge",
	}
	for _, e := range out {
		if e.Kind != "SCOPE.Operation" {
			continue
		}
		edge, ok := implementsEdgeFor(e)
		if !ok {
			t.Errorf("handler %s not bridged", e.Name)
			continue
		}
		if want[e.Name] != edge.ToID {
			t.Errorf("MIS-BRIDGE: handler %s implements %s, want %s", e.Name, edge.ToID, want[e.Name])
		}
	}
}

// TestBridge4319_AmbiguousBareNameNotGuessed guards that when two same-file
// handlers share the same bare method name (e.g. an overload-like collision),
// the fix does NOT guess — it leaves resolution to the existing fallbacks
// rather than mis-bridging to an arbitrary candidate.
func TestBridge4319_AmbiguousBareNameNotGuessed(t *testing.T) {
	file := "src/svc.ts"
	h1 := types.EntityRecord{Kind: "SCOPE.Operation", Name: "A.handle", SourceFile: file, Language: "typescript"}
	h2 := types.EntityRecord{Kind: "SCOPE.Operation", Name: "B.handle", SourceFile: file, Language: "typescript"}
	synth := types.EntityRecord{
		Kind: httpEndpointDefinitionKind, Name: "http:GET:/x", SourceFile: file, Language: "typescript",
		Properties: map[string]string{"framework": "nestjs", "pattern_type": "http_endpoint_synthesis", "source_handler": "Controller:handle"},
	}
	out, _ := ResolveHTTPEndpointHandlers([]types.EntityRecord{h1, h2, synth})
	// Neither handler should receive an IMPLEMENTS edge via the ambiguous
	// same-file bare lookup (the fix only binds when exactly one candidate).
	for _, e := range out {
		if e.Kind != "SCOPE.Operation" {
			continue
		}
		if _, ok := implementsEdgeFor(e); ok {
			t.Errorf("ambiguous bare name must not be guess-bridged, but %s was", e.Name)
		}
	}
}
