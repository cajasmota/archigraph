package engine

import (
	"testing"

	"github.com/cajasmota/archigraph/internal/types"
)

// Issue #4487 — contract specs name the endpoint in their describe/it label and
// the Jest extractor (after this fix) folds that into `e2e_route_calls`. This
// engine test proves the EXISTING resolve pass links such a label-derived route
// to the synthesized NestJS endpoint, so the endpoint's Tests panel is no longer
// (0). The controller route is the real
// `@Get(':inspectionId/counter-party-results')` under
// `@Controller({ path: 'inspections', version: '1' })`, synthesized with the
// `/api/v1` mount prefix and a `{inspectionId}` template param; the spec label
// says `:id`. The segment matcher must wildcard BOTH param notations.
func TestIssue4487_LabelRouteLinksToEndpoint(t *testing.T) {
	defs := []types.EntityRecord{
		// Real inspection.controller.ts routes (verb + synthesized path).
		def("GET", "/api/v1/inspections/{inspectionId}/counter-party-results"),
		def("GET", "/api/v1/inspections/{inspectionGroupId}"),
		def("POST", "/api/v1/inspections/notes"),
		// Unrelated endpoint that must NOT be linked.
		def("GET", "/api/v1/buildings/{id}"),
	}

	// Route as the Jest extractor now stamps it from the describe label
	// `… — GET /api/v1/inspections/:id/counter-party-results`.
	suite := e2eSuite("GET /api/v1/inspections/:id/counter-party-results")

	out, stats := ResolveHTTPEndpointHandlers(append(append([]types.EntityRecord{}, defs...), suite))
	edges := testsEdgesToEndpoints(out)
	if stats.E2ERouteTestEdges != 1 {
		t.Fatalf("expected 1 label-route TESTS edge, got %d (edges=%v)", stats.E2ERouteTestEdges, edges)
	}
	want := "http_endpoint_definition:http:GET:/api/v1/inspections/{inspectionId}/counter-party-results"
	if edges[0].ToID != want {
		t.Fatalf("TESTS edge target = %q, want %q", edges[0].ToID, want)
	}
}

// TestIssue4487_ControlNoLabelRouteNoEdge is the BEFORE control: with no
// e2e_route_calls (the pre-fix extractor output for a label-only contract spec)
// the endpoint gains no TESTS edge — reproducing the reported Tests (0).
func TestIssue4487_ControlNoLabelRouteNoEdge(t *testing.T) {
	defs := []types.EntityRecord{
		def("GET", "/api/v1/inspections/{inspectionId}/counter-party-results"),
	}
	control := e2eSuite("")
	control.Properties = map[string]string{"framework": "jest"} // no e2e_route_calls
	_, stats := ResolveHTTPEndpointHandlers(append(defs, control))
	if stats.E2ERouteTestEdges != 0 {
		t.Fatalf("control must emit 0 TESTS edges, got %d", stats.E2ERouteTestEdges)
	}
}
