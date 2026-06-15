package lua_test

import (
	"context"
	"strings"
	"testing"

	extreg "github.com/cajasmota/grafel/internal/extractor"

	_ "github.com/cajasmota/grafel/internal/custom/lua"
)

func luaFI(path, lang, src string) extreg.FileInput {
	return extreg.FileInput{Path: path, Language: lang, Content: []byte(src)}
}

// TestLuaRouteE2E_Capture proves the lapis.spec request helpers are captured
// onto a single test_suite's e2e_route_calls property, with verbs read from the
// { method = "..." } options table (GET default).
func TestLuaRouteE2E_Capture(t *testing.T) {
	src := `
local request = require("lapis.spec.request").request

describe("Users", function()
  it("lists users", function()
    local status = request(app, "/users")
    assert.same(200, status)
  end)

  it("shows one", function()
    request(app, "/users/" .. id)
  end)

  it("creates", function()
    request(app, "/users", { method = "POST" })
  end)

  it("deletes", function()
    mock_request(app, "/users/1", { method = "DELETE" })
  end)
end)
`
	e, ok := extreg.Get("custom_lua_tests_route_e2e")
	if !ok {
		t.Fatal("custom_lua_tests_route_e2e not registered")
	}
	ents, err := e.Extract(context.Background(), luaFI("spec/users_spec.lua", "lua", src))
	if err != nil {
		t.Fatalf("extract: %v", err)
	}
	if len(ents) != 1 {
		t.Fatalf("expected exactly 1 test_suite, got %d", len(ents))
	}
	rec := ents[0]
	if rec.Subtype != "test_suite" {
		t.Errorf("expected test_suite, got %q", rec.Subtype)
	}
	calls := rec.Properties["e2e_route_calls"]
	for _, want := range []string{"GET /users", "POST /users", "DELETE /users/1"} {
		if !strings.Contains(calls, want) {
			t.Errorf("expected route call %q in %q", want, calls)
		}
	}
}

// TestLuaRouteE2E_PathOnly proves the path-only request("/x") form is captured.
func TestLuaRouteE2E_PathOnly(t *testing.T) {
	src := `
describe("Health", function()
  it("ok", function()
    request("/health")
  end)
end)
`
	e, _ := extreg.Get("custom_lua_tests_route_e2e")
	ents, _ := e.Extract(context.Background(), luaFI("spec/health_spec.lua", "lua", src))
	if len(ents) != 1 {
		t.Fatalf("expected 1 test_suite, got %d", len(ents))
	}
	if !strings.Contains(ents[0].Properties["e2e_route_calls"], "GET /health") {
		t.Errorf("expected GET /health, got %q", ents[0].Properties["e2e_route_calls"])
	}
}

// TestLuaRouteE2E_NonSpecExcluded proves a non-spec file (production route
// registration) is NOT captured as a test_suite.
func TestLuaRouteE2E_NonSpecExcluded(t *testing.T) {
	src := `
local lapis = require("lapis")
local app = lapis.Application()
app:get("/users", function(self) end)
`
	e, _ := extreg.Get("custom_lua_tests_route_e2e")
	ents, _ := e.Extract(context.Background(), luaFI("app.lua", "lua", src))
	if len(ents) != 0 {
		t.Fatalf("expected no test_suite for a non-spec file, got %d", len(ents))
	}
}

// TestLuaRouteE2E_ShapeOnlySpecExcluded proves a unit spec that never hits a
// route emits no suite.
func TestLuaRouteE2E_ShapeOnlySpecExcluded(t *testing.T) {
	src := `
describe("Validator", function()
  it("rejects empty", function()
    assert.is_false(validate(""))
  end)
end)
`
	e, _ := extreg.Get("custom_lua_tests_route_e2e")
	ents, _ := e.Extract(context.Background(), luaFI("spec/validator_spec.lua", "lua", src))
	if len(ents) != 0 {
		t.Fatalf("expected no test_suite for a shape-only spec, got %d", len(ents))
	}
}

// TestLuaRouteE2E_WrongLanguageNoop proves the extractor gates on language=="lua".
func TestLuaRouteE2E_WrongLanguageNoop(t *testing.T) {
	src := `request(app, "/users")`
	e, _ := extreg.Get("custom_lua_tests_route_e2e")
	ents, _ := e.Extract(context.Background(), luaFI("spec/users_spec.lua", "ruby", src))
	if len(ents) != 0 {
		t.Fatalf("expected no entities for non-lua language, got %d", len(ents))
	}
}
