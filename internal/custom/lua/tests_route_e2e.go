// tests_route_e2e.go — Lua busted / lapis.spec → endpoint route-hit linkage.
//
// #4749 (the Lua slice of the coverage-linkage tail epic #4749/#4615; mirrors
// the Crystal/Kemal slice #4760 and the Swift/Vapor slice #4755). Emits ONE
// test_suite entity per Lua spec file that drives an HTTP endpoint by ROUTE
// STRING via the lapis.spec request helpers, stamping the captured
// `VERB route` pairs onto the suite's `e2e_route_calls` property (one
// "VERB route" per line). The shared resolve pass
// (engine.linkE2ERouteTestsToEndpoints, #4351/#4369) then matches each pair
// against the cross-file http_endpoint_definition index — which, for Lua, is
// populated by synthesizeLapis (#3484, internal/engine/lua_routes.go) — and
// emits a TESTS edge to the exact Lapis endpoint exercised, exactly as the
// Crystal, Swift, Rust, Java, Ruby, PHP, C# and Elixir slices do for their
// stacks. The engine pass is language-agnostic (it fires on any test_suite
// carrying e2e_route_calls), so the only Lua-specific work here is the
// lapis.spec route capture.
//
// lapis.spec route-driving idioms captured (lapis.spec.request / mock_request):
//   - request(app, "/users")                            → GET  /users
//   - request(app, "/users", { method = "POST" })       → POST /users
//   - request("/users/" .. id)                          → GET  /users
//   - mock_request(app, "/users", { method = "DELETE" })→ DELETE /users
//
// The request helper is a top-level call whose FIRST string-literal argument is
// the route path; the optional `{ method = "VERB" }` options table carries the
// HTTP verb (GET when absent). busted's test DSL
// (`describe("...", function() it("...", function() ... end) end)`) uses
// ANONYMOUS function closures (like JS/Ruby) — the `it` example body owns no
// named, call-bearing entity — so the route-hit signal is carried by the
// SUITE-LEVEL test_suite entity emitted here (the scope-owner role), NOT by a
// per-example operation. This is the Lua analog of the Ruby #4684 / JS #4680
// anonymous-block scope-owner: the test_suite is the owner that carries the
// e2e_route_calls the engine links from.
//
// Local-variable / receiver typing (#4749 part a) is N/A for Lua coverage
// linkage: Lapis handlers are anonymous functions / table methods, and the lua
// base extractor does not produce a class-qualified receiver resolver that could
// consume a `receiver_type` stamp on a Lua CALLS edge — it would be a dead
// annotation. The honest, working coverage mechanism for Lua is the route-hit →
// endpoint linkage in THIS file (route dispatch is keyed by the literal route
// string, not by an `obj.method()` receiver), exactly as for the functional
// Elixir, Crystal and Swift slices. See the coverage-doc note for the recorded
// N/A rationale.
//
// Honest exclusions (no fabricated edges):
//   - Concatenated / variable routes capture the literal prefix only when a
//     static string is present; a fully dynamic route (`request(app, path)`) is
//     dropped.
//   - Non-request specs (pure unit specs that never hit a route) emit no suite.
//   - A `request(...)` appearing OUTSIDE a spec file is ignored; this extractor
//     only runs on busted/lapis spec files.
//
// Registration key: "custom_lua_tests_route_e2e".
package lua

import (
	"context"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/cajasmota/grafel/internal/extractor"
	"github.com/cajasmota/grafel/internal/types"
)

func init() {
	extractor.Register("custom_lua_tests_route_e2e", &luaTestRouteE2EExtractor{})
}

type luaTestRouteE2EExtractor struct{}

func (e *luaTestRouteE2EExtractor) Language() string {
	return "custom_lua_tests_route_e2e"
}

// luaSpecRequestRE matches a lapis.spec request helper call whose first
// argument resolves to a string-literal route path. Two shapes are supported:
//
//	request(app, "/users", { method = "POST" })   — app + path form
//	request("/users")                             — path-only form
//	mock_request(app, "/users")                   — mock alias
//
// Capture group 1 is an optional first string argument (path-only form);
// group 2 is the string argument after a non-string first arg (app + path
// form). The optional `{ method = "VERB" }` table is matched separately on the
// same line.
var (
	// request(app, "/path", ...) / mock_request(app, "/path", ...) — the path
	// is the SECOND argument (after a non-string `app` identifier).
	luaSpecReqAppPathRE = regexp.MustCompile(
		`(?m)\b(?:mock_)?request\s*\(\s*[A-Za-z_][\w.]*\s*,\s*["']([^"'\n\r]*)["']`)

	// request("/path", ...) — path is the FIRST argument (a string literal).
	luaSpecReqPathOnlyRE = regexp.MustCompile(
		`(?m)\b(?:mock_)?request\s*\(\s*["']([^"'\n\r]*)["']`)

	// { method = "POST" } / { method = 'post' } options-table verb.
	luaSpecMethodRE = regexp.MustCompile(
		`method\s*=\s*["']([A-Za-z]+)["']`)
)

func (e *luaTestRouteE2EExtractor) Extract(
	ctx context.Context,
	file extractor.FileInput,
) ([]types.EntityRecord, error) {
	if len(file.Content) == 0 || file.Language != "lua" {
		return nil, nil
	}
	if !isLuaSpecFile(file.Path) {
		return nil, nil
	}
	source := string(file.Content)
	routeCalls := collectLuaSpecRouteCalls(source)
	if len(routeCalls) == 0 {
		return nil, nil
	}
	rec := types.EntityRecord{
		Name:       luaSpecSuiteBaseName(file.Path),
		Kind:       "SCOPE.Operation",
		Subtype:    "test_suite",
		SourceFile: file.Path,
		Language:   "lua",
		StartLine:  1,
		EndLine:    1,
		Properties: map[string]string{
			"framework":       "lapis.spec",
			"provenance":      "INFERRED_FROM_LUA_TEST_ROUTE_E2E",
			"e2e_route_calls": strings.Join(routeCalls, "\n"),
		},
	}
	rec.ID = rec.ComputeID()
	return []types.EntityRecord{rec}, nil
}

// collectLuaSpecRouteCalls returns the de-duplicated "VERB route" pairs a spec
// file drives by route string. The verb is read from a sibling
// `{ method = "VERB" }` options table on the same call line; GET is the
// lapis.spec default when no method is given. Fully dynamic routes (no static
// literal) are dropped.
func collectLuaSpecRouteCalls(source string) []string {
	var out []string
	seen := map[string]bool{}
	add := func(rawPath, line string) {
		route := normaliseLuaSpecRoute(rawPath)
		if route == "" {
			return
		}
		verb := luaSpecLineVerb(line)
		key := verb + " " + route
		if seen[key] {
			return
		}
		seen[key] = true
		out = append(out, key)
	}
	for _, line := range strings.Split(source, "\n") {
		// app + path form takes precedence (first arg is the app identifier).
		if m := luaSpecReqAppPathRE.FindStringSubmatch(line); m != nil {
			add(m[1], line)
			continue
		}
		if m := luaSpecReqPathOnlyRE.FindStringSubmatch(line); m != nil {
			add(m[1], line)
		}
	}
	return out
}

// luaSpecLineVerb extracts the HTTP verb from a `{ method = "VERB" }` options
// table on the request line; defaults to GET (the lapis.spec default).
func luaSpecLineVerb(line string) string {
	if m := luaSpecMethodRE.FindStringSubmatch(line); m != nil {
		return strings.ToUpper(m[1])
	}
	return "GET"
}

// normaliseLuaSpecRoute reduces a raw lapis.spec route literal to a path:
// ensures a single leading slash, drops a query/fragment tail, truncates at a
// Lua string-concatenation marker (`..`) so a concatenated route keeps only its
// static prefix, collapses repeated slashes. A route whose value has no static
// prefix is dropped (returns ""). Path-param placeholders (`:id`) and casing are
// preserved (the resolver wildcards templates and compares case-insensitively).
func normaliseLuaSpecRoute(raw string) string {
	p := strings.TrimSpace(raw)
	if p == "" {
		return ""
	}
	// Truncate at a Lua concatenation marker — the static prefix is the only
	// statically-recoverable part. `"/users/" .. id` is captured as the literal
	// `/users/` before the closing quote, so this is mostly defensive.
	if i := strings.Index(p, ".."); i >= 0 {
		p = p[:i]
	}
	if q := strings.IndexAny(p, "?#"); q >= 0 {
		p = p[:q]
	}
	p = strings.TrimSpace(p)
	if p == "" {
		return ""
	}
	if !strings.HasPrefix(p, "/") {
		p = "/" + p
	}
	for strings.Contains(p, "//") {
		p = strings.ReplaceAll(p, "//", "/")
	}
	// A bare "/" after truncation carries no route identity — drop it.
	if p == "/" {
		return ""
	}
	return strings.TrimRight(p, "/")
}

// isLuaSpecFile reports whether path looks like a Lua spec — a `*_spec.lua`
// file or a file under a `/spec/` directory. Keeps the route-hit suite off
// production code that merely registers a route.
func isLuaSpecFile(path string) bool {
	lp := filepath.ToSlash(path)
	base := filepath.Base(lp)
	if strings.HasSuffix(base, "_spec.lua") {
		return true
	}
	if strings.Contains(lp, "/spec/") && strings.HasSuffix(lp, ".lua") {
		return true
	}
	return false
}

// luaSpecSuiteBaseName derives a suite label from the spec file path
// (`.../users_spec.lua` → `users_spec`).
func luaSpecSuiteBaseName(path string) string {
	base := filepath.Base(filepath.ToSlash(path))
	return strings.TrimSuffix(base, filepath.Ext(base))
}
