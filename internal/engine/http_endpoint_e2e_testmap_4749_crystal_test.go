package engine

import (
	"strings"
	"testing"

	"github.com/cajasmota/archigraph/internal/types"

	// Register the Crystal route-hit extractor so the test_suite (carrying
	// e2e_route_calls) comes from the REAL extractor, not a hand-built fixture.
	_ "github.com/cajasmota/archigraph/internal/custom/crystal"
)

// Issue #4749 LIVE-REPRO (resolve side) — Crystal spec-kemal route-hit tests.
// Proves end-to-end that a Crystal spec calling a route by string
// (`get "/path"`, `post "/path"`) links to the http_endpoint_definition it
// exercises — the Crystal slice of the all-language program (#4615/#4749). The
// shared linkE2ERouteTestsToEndpoints pass is language-agnostic; only the
// Crystal route capture is new.

const crystalSpecKemalSrc4749 = `
require "./spec_helper"

describe "Todos" do
  it "lists todos" do
    get "/api/v1/todos"
    response.status_code.should eq 200
  end

  it "creates a todo" do
    post "/api/v1/todos"
    response.status_code.should eq 201
  end
end
`

func TestIssue4749_CrystalSpecKemalE2ERouteTestsLinkToEndpoints(t *testing.T) {
	defs := []types.EntityRecord{
		def("GET", "/api/v1/todos"),
		def("POST", "/api/v1/todos"),
	}
	suite := realSuite(t, "custom_crystal_tests_route_e2e",
		"spec/requests/todos_spec.cr", "crystal", crystalSpecKemalSrc4749)

	afterOut, edges := runE2ERouteResolve(t, defs, suite)
	if edges < 2 {
		t.Fatalf("expected >=2 e2e route TESTS edges (GET + POST), got %d", edges)
	}
	assertCrystalRouteEdges(t, edgeTargets(afterOut))
}

func assertCrystalRouteEdges(t *testing.T, targets map[string]bool) {
	t.Helper()
	wantGet, wantPost := false, false
	for to := range targets {
		if strings.Contains(to, "GET:/api/v1/todos") {
			wantGet = true
		}
		if strings.Contains(to, "POST:/api/v1/todos") {
			wantPost = true
		}
	}
	if !wantGet {
		t.Errorf("expected a TESTS edge to GET /api/v1/todos; targets=%v", targets)
	}
	if !wantPost {
		t.Errorf("expected a TESTS edge to POST /api/v1/todos; targets=%v", targets)
	}
}
