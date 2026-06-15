package engine

import (
	"strings"
	"testing"

	"github.com/cajasmota/grafel/internal/types"

	// Register the Lua route-hit extractor so the test_suite (carrying
	// e2e_route_calls) comes from the REAL extractor, not a hand-built fixture.
	_ "github.com/cajasmota/grafel/internal/custom/lua"
)

// Issue #4749 LIVE-REPRO (resolve side) — Lua lapis.spec route-hit tests.
// Proves end-to-end that a Lua busted/lapis spec calling a route by string
// (`request(app, "/path", { method = "POST" })`) links to the
// http_endpoint_definition it exercises — the Lua slice of the all-language
// program (#4615/#4749). The shared linkE2ERouteTestsToEndpoints pass is
// language-agnostic; only the Lua route capture is new (the Lapis producer
// already exists, #3484).

const luaSpecLapisSrc4749 = `
local request = require("lapis.spec.request").request

describe("Todos", function()
  it("lists todos", function()
    local status = request(app, "/api/v1/todos")
    assert.same(200, status)
  end)

  it("creates a todo", function()
    request(app, "/api/v1/todos", { method = "POST" })
  end)
end)
`

func TestIssue4749_LuaSpecLapisE2ERouteTestsLinkToEndpoints(t *testing.T) {
	defs := []types.EntityRecord{
		def("GET", "/api/v1/todos"),
		def("POST", "/api/v1/todos"),
	}
	suite := realSuite(t, "custom_lua_tests_route_e2e",
		"spec/requests/todos_spec.lua", "lua", luaSpecLapisSrc4749)

	afterOut, edges := runE2ERouteResolve(t, defs, suite)
	if edges < 2 {
		t.Fatalf("expected >=2 e2e route TESTS edges (GET + POST), got %d", edges)
	}
	assertLuaRouteEdges(t, edgeTargets(afterOut))
}

func assertLuaRouteEdges(t *testing.T, targets map[string]bool) {
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
