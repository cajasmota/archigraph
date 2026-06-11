package engine

import (
	"strings"
	"testing"

	"github.com/cajasmota/archigraph/internal/types"

	// Register the Nim route-hit extractor so the test_suite (carrying
	// e2e_route_calls) comes from the REAL extractor, not a hand-built fixture.
	_ "github.com/cajasmota/archigraph/internal/custom/nim"
)

// Issue #4749 LIVE-REPRO (resolve side) — Nim std/httpclient route-hit tests.
// Proves end-to-end that a Nim unittest calling a route by string
// (`client.get("http://localhost/api/v1/todos")`, `client.post(...)`) links to
// the http_endpoint_definition it exercises — the Nim slice (LAST in the tail)
// of the all-language program (#4615/#4749). The shared
// linkE2ERouteTestsToEndpoints pass is language-agnostic; only the Nim route
// capture is new.

const nimUnittestHTTPSrc4749 = `
import std/unittest
import std/httpclient

suite "Todos":
  test "lists todos":
    let client = newHttpClient()
    let resp = client.get("http://localhost:8080/api/v1/todos")
    check resp.code == Http200

  test "creates a todo":
    let client = newHttpClient()
    let resp = client.post("http://localhost:8080/api/v1/todos", body = "{}")
    check resp.code == Http201
`

func TestIssue4749_NimUnittestE2ERouteTestsLinkToEndpoints(t *testing.T) {
	defs := []types.EntityRecord{
		def("GET", "/api/v1/todos"),
		def("POST", "/api/v1/todos"),
	}
	suite := realSuite(t, "custom_nim_tests_route_e2e",
		"tests/tTodos.nim", "nim", nimUnittestHTTPSrc4749)

	afterOut, edges := runE2ERouteResolve(t, defs, suite)
	if edges < 2 {
		t.Fatalf("expected >=2 e2e route TESTS edges (GET + POST), got %d", edges)
	}
	assertNimRouteEdges(t, edgeTargets(afterOut))
}

func assertNimRouteEdges(t *testing.T, targets map[string]bool) {
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
