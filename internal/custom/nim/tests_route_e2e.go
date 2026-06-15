// tests_route_e2e.go — Nim unittest → endpoint route-hit linkage.
//
// #4749 (the Nim slice of the coverage-linkage tail epic #4749/#4615; the LAST
// language in the tail; mirrors the Crystal/Kemal slice and the Lua/Lapis
// slice). Emits ONE test_suite entity per Nim test file that drives an HTTP
// endpoint by ROUTE STRING via the std/httpclient helpers, stamping the
// captured `VERB route` pairs onto the suite's `e2e_route_calls` property (one
// "VERB route" per line). The shared resolve pass
// (engine.linkE2ERouteTestsToEndpoints, #4351/#4369) then matches each pair
// against the cross-file http_endpoint_definition index — which, for Nim, is
// populated by synthesizeJester / synthesizePrologue (#4749,
// internal/engine/http_endpoint_jester.go) — and emits a TESTS edge to the
// exact Jester/Prologue endpoint exercised, exactly as the Crystal, Swift,
// Rust, Java, Ruby, PHP, C# and Elixir slices do for their stacks. The engine
// pass is language-agnostic (it fires on any test_suite carrying
// e2e_route_calls), so the only Nim-specific work here is the httpclient route
// capture.
//
// std/httpclient route-driving idioms captured:
//   - client.get("http://localhost:8080/users")          → GET    /users
//   - client.post(baseUrl & "/users", body = ...)         → POST   /users
//   - client.delete("http://127.0.0.1:5000/users/" & $id) → DELETE /users
//   - client.request(url & "/users", httpMethod = HttpPut)→ PUT    /users
//   - newHttpClient().get(addr & "/health")               → GET    /health
//
// Nim's `std/unittest` test DSL uses `suite "...":` containers and `test "...":`
// blocks. A `test "...":` block IS a named, statement-list scope (the
// description is a string literal, the body a colon-indented block) — but the
// description is PROSE, not a code symbol, and the block carries no callable
// entity name the production-symbol resolver can bind to. So, like the JS/Ruby
// anonymous-closure case, the route-hit signal is carried by the SUITE-LEVEL
// test_suite entity emitted here (the scope-owner role). This is the Nim analog
// of the Ruby #4684 / JS #4680 / Crystal #4760 anonymous-block scope-owner: the
// test_suite is the owner that carries the e2e_route_calls the engine links
// from. (The named-symbol test→SUT linkage for unittest is handled separately
// by the shared cross/testmap detector; see frameworks_nim.go.)
//
// Local-variable / receiver typing (#4749 part a) is N/A for Nim coverage
// linkage: the nim base extractor names proc/method entities by their BARE
// name (with a CONTAINS edge from an attached type, not a `Type.method`
// qualified id), and there is no class-qualified receiver resolver to consume a
// `receiver_type` stamp on a Nim CALLS edge — it would be a dead annotation.
// The honest, working coverage mechanism for Nim is the route-hit → endpoint
// linkage in THIS file (route dispatch is keyed by the literal route string,
// not by an `obj.method()` receiver), exactly as for the functional Elixir and
// Crystal slices. See the coverage-doc note for the recorded N/A rationale.
//
// Honest exclusions (no fabricated edges):
//   - A route built entirely from variables / `&` concatenation with no static
//     `/segment` literal is dropped; the static prefix is captured when present.
//   - Non-request test files (pure unit tests that never hit a route) emit no
//     suite.
//   - A `client.get(...)` appearing OUTSIDE a test file is left to the
//     producer-side synthesizers; this extractor only runs on test files.
//
// Registration key: "custom_nim_tests_route_e2e".
package nim

import (
	"context"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/cajasmota/grafel/internal/extractor"
	"github.com/cajasmota/grafel/internal/types"
)

func init() {
	extractor.Register("custom_nim_tests_route_e2e", &nimTestRouteE2EExtractor{})
}

type nimTestRouteE2EExtractor struct{}

func (e *nimTestRouteE2EExtractor) Language() string {
	return "custom_nim_tests_route_e2e"
}

var (
	// nimHTTPVerbCallRE matches a std/httpclient verb call whose first argument
	// contains a string-literal URL/path: `client.get("…")`,
	// `client.post(base & "/users", …)`, `newHttpClient().delete("…")`. Capture
	// group 1 is the verb; group 2 is the FIRST string literal in the argument
	// list (the URL or path fragment).
	nimHTTPVerbCallRE = regexp.MustCompile(
		`(?m)\.(get|post|put|delete|patch|options|head)\s*\(\s*(?:[^"(),\n]*&\s*)?"([^"\n\r]*)"`)

	// nimHTTPRequestRE matches the generic `client.request(url, httpMethod =
	// HttpPost)` form: capture group 1 is the FIRST string literal (URL/path),
	// group 2 the `Http<Verb>` enum tail.
	nimHTTPRequestRE = regexp.MustCompile(
		`(?m)\.request\s*\(\s*(?:[^"(),\n]*&\s*)?"([^"\n\r]*)"[^)\n]*?Http([A-Za-z]+)`)
)

func (e *nimTestRouteE2EExtractor) Extract(
	ctx context.Context,
	file extractor.FileInput,
) ([]types.EntityRecord, error) {
	if len(file.Content) == 0 || file.Language != "nim" {
		return nil, nil
	}
	if !isNimTestFile(file.Path) {
		return nil, nil
	}
	source := string(file.Content)
	routeCalls := collectNimTestRouteCalls(source)
	if len(routeCalls) == 0 {
		return nil, nil
	}
	rec := types.EntityRecord{
		Name:       nimTestSuiteBaseName(file.Path),
		Kind:       "SCOPE.Operation",
		Subtype:    "test_suite",
		SourceFile: file.Path,
		Language:   "nim",
		StartLine:  1,
		EndLine:    1,
		Properties: map[string]string{
			"framework":       "unittest",
			"provenance":      "INFERRED_FROM_NIM_TEST_ROUTE_E2E",
			"e2e_route_calls": strings.Join(routeCalls, "\n"),
		},
	}
	rec.ID = rec.ComputeID()
	return []types.EntityRecord{rec}, nil
}

// collectNimTestRouteCalls returns the de-duplicated "VERB route" pairs a Nim
// test file drives by route string via std/httpclient. The verb is the method
// name for the `.get/.post/…` form, or the `Http<Verb>` enum for the generic
// `.request(...)` form. Routes are normalised to a leading-slash path; a route
// with no static `/segment` literal is dropped.
func collectNimTestRouteCalls(source string) []string {
	var out []string
	seen := map[string]bool{}
	add := func(verb, rawURL string) {
		route := normaliseNimTestRoute(rawURL)
		if route == "" {
			return
		}
		line := strings.ToUpper(verb) + " " + route
		if seen[line] {
			return
		}
		seen[line] = true
		out = append(out, line)
	}
	for _, m := range nimHTTPVerbCallRE.FindAllStringSubmatch(source, -1) {
		add(m[1], m[2])
	}
	for _, m := range nimHTTPRequestRE.FindAllStringSubmatch(source, -1) {
		add(m[2], m[1])
	}
	return out
}

// normaliseNimTestRoute reduces a raw httpclient URL/path literal to a path:
// strips a scheme+authority prefix (http://127.0.0.1:8080/x → /x), drops a
// query/fragment tail, ensures a single leading slash, collapses repeated
// slashes. A literal that is only a host/scheme with no path, or that has no
// static `/segment`, is dropped (returns ""). Path-param placeholders (`@id`,
// `{id}`) and casing are preserved (the resolver wildcards templates and
// compares case-insensitively).
func normaliseNimTestRoute(raw string) string {
	p := strings.TrimSpace(raw)
	if p == "" {
		return ""
	}
	// Strip scheme://authority — keep the path that follows the first slash
	// after the authority. A literal with scheme but no path (`http://host`)
	// yields "" (no route identity).
	if i := strings.Index(p, "://"); i >= 0 {
		rest := p[i+3:]
		if slash := strings.IndexByte(rest, '/'); slash >= 0 {
			p = rest[slash:]
		} else {
			return ""
		}
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
	if p == "/" {
		return ""
	}
	return strings.TrimRight(p, "/")
}

// isNimTestFile reports whether path looks like a Nim test — a `t*.nim` /
// `*_test.nim` / `test_*.nim` file or a file under a `/tests/` directory. The
// Nim convention (nimble test) is `tests/tFoo.nim`. Keeps the route-hit suite
// off production code that merely registers a route.
func isNimTestFile(path string) bool {
	lp := filepath.ToSlash(path)
	base := filepath.Base(lp)
	if !strings.HasSuffix(base, ".nim") {
		return false
	}
	stem := strings.TrimSuffix(base, ".nim")
	switch {
	case strings.HasSuffix(stem, "_test"),
		strings.HasPrefix(stem, "test_"),
		strings.HasPrefix(stem, "test"),
		// nimble's `tests/tFoo.nim` convention: a leading lowercase `t`
		// followed by an uppercase letter.
		len(stem) >= 2 && stem[0] == 't' && stem[1] >= 'A' && stem[1] <= 'Z':
		return true
	}
	return strings.Contains(lp, "/tests/")
}

// nimTestSuiteBaseName derives a suite label from the test file path
// (`.../tUsers.nim` → `tUsers`).
func nimTestSuiteBaseName(path string) string {
	base := filepath.Base(filepath.ToSlash(path))
	return strings.TrimSuffix(base, filepath.Ext(base))
}
