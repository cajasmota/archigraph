// Package clojure — Clojure test→endpoint route-hit linkage.
//
// #4749 (the Clojure slice of epic #4615, all-framework test→endpoint coverage
// linkage). Emits ONE test_suite entity per Clojure clojure.test test file that
// drives an HTTP endpoint by ROUTE STRING via a Ring handler, stamping the
// captured `VERB route` pairs onto the suite's `e2e_route_calls` property. The
// shared resolve pass (engine.linkE2ERouteTestsToEndpoints, #4351/#4369) then
// matches each pair against the cross-file http_endpoint_definition index
// (produced for Clojure by engine.synthesizeClojureRoutes) and emits a TESTS
// edge to the exact endpoint exercised — exactly as the Elixir/Phoenix, Ruby,
// PHP, C# and Swift slices do for their stacks. The engine pass is
// language-agnostic (it fires on any test_suite carrying e2e_route_calls), so
// the only Clojure-specific work is the route capture.
//
// Ring route-driving idioms captured (clojure.test + ring-mock / ring core):
//
//	(app (mock/request :get "/todos"))
//	(app (ring.mock.request/request :post "/users" {...}))
//	(handler (mock/request :delete "/users/1"))
//	(app (-> (mock/request :get "/todos")))      ; threaded request builder
//
// The first arg to ring-mock's `request` is the HTTP-verb KEYWORD (`:get`,
// `:post`, …); the second is the string-literal route. peridot/kerodon drive the
// app the same way under the hood (`(request app "/path" :request-method :get)`)
// and are captured by the peridot form below.
//
// Clojure is FUNCTIONAL: there are no OO receiver objects, so the
// "local-variable receiver typing" gap (#4680/#4681) does NOT apply — a Ring
// handler is dispatched by the literal route string carried on the mock request
// map, not by an `obj.method()` receiver. The route-hit → endpoint linkage IS
// the coverage mechanism here (mirrors the Elixir #4688 documented N/A).
//
// Test scope: clojure.test `(deftest name ...)` forms are NAMED top-level
// functions already mined by the base extractor's CALLS graph, and `(testing
// "..." ...)` blocks nest inside them — the route-hit calls live inside the
// deftest body, so the suite is keyed to the FILE (one suite per test file) the
// same way the Jest/ExUnit one-suite-per-file model works; no synthetic
// anonymous-test-block scope owner is needed.
//
// Honest exclusion (matches the negative acceptance of the slice):
//   - Interpolated / built routes (`(mock/request :get (str "/x/" id))`,
//     `(mock/request :get path)`) — the route is not a string literal and is not
//     statically recoverable here, so it is dropped.
//   - Shape-only tests (assert on a value, never hit a route) emit no suite.
//
// Registration key: "custom_clojure_tests_route_e2e".
package clojure

import (
	"context"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/cajasmota/archigraph/internal/extractor"
	"github.com/cajasmota/archigraph/internal/types"
)

func init() {
	extractor.Register("custom_clojure_tests_route_e2e", &clojureTestRouteE2EExtractor{})
}

type clojureTestRouteE2EExtractor struct{}

func (e *clojureTestRouteE2EExtractor) Language() string {
	return "custom_clojure_tests_route_e2e"
}

var (
	// ring-mock / ring.mock.request request builder:
	//   (mock/request :get "/todos")
	//   (ring.mock.request/request :post "/users" {...})
	//   (request :delete "/users/1")
	// Capture group 1 is the verb keyword (without the leading colon); group 2 is
	// the string-literal route. The `request` symbol may be namespace-qualified.
	cljRingMockRequestRE = regexp.MustCompile(
		`\(\s*(?:[\w.\-]+/)?request\s+:([a-z]+)\s+"([^"\n\r]*)"`)

	// peridot / kerodon session form. The session is threaded in via `->`, so
	// the `request` call takes the route literal as its FIRST arg (threaded
	// form) or after an explicit session arg (direct form):
	//   (-> (session app) (request "/path" :request-method :get))
	//   (request session "/path" :request-method :get)
	// The route literal is the first string literal; the verb is the
	// `:request-method` keyword. An optional non-string session arg is tolerated
	// before the route literal.
	cljPeridotRequestRE = regexp.MustCompile(
		`\(\s*request\s+(?:[\w.\-]+\s+)?"([^"\n\r]*)"[^)]*:request-method\s+:([a-z]+)`)
)

func (e *clojureTestRouteE2EExtractor) Extract(
	ctx context.Context,
	file extractor.FileInput,
) ([]types.EntityRecord, error) {
	if len(file.Content) == 0 || file.Language != "clojure" {
		return nil, nil
	}
	if !isClojureTestFile(file.Path) {
		return nil, nil
	}
	source := string(file.Content)
	routeCalls := collectClojureTestRouteCalls(source)
	if len(routeCalls) == 0 {
		return nil, nil
	}
	rec := types.EntityRecord{
		Name:       clojureTestSuiteBaseName(file.Path),
		Kind:       "SCOPE.Operation",
		Subtype:    "test_suite",
		SourceFile: file.Path,
		Language:   "clojure",
		StartLine:  1,
		EndLine:    1,
		Properties: map[string]string{
			"framework":       "ring",
			"provenance":      "INFERRED_FROM_CLOJURE_TEST_ROUTE_E2E",
			"e2e_route_calls": strings.Join(routeCalls, "\n"),
		},
	}
	return []types.EntityRecord{rec}, nil
}

// collectClojureTestRouteCalls returns the de-duplicated "VERB route" pairs a
// clojure.test test file drives by route string (ring-mock and peridot forms).
// Routes are normalised to a leading-slash path; interpolated / variable /
// non-literal routes are dropped (honest exclusion).
func collectClojureTestRouteCalls(source string) []string {
	var out []string
	seen := map[string]bool{}
	add := func(verb, rawRoute string) {
		route := normaliseClojureTestRoute(rawRoute)
		if route == "" || !strings.HasPrefix(route, "/") {
			return
		}
		line := strings.ToUpper(verb) + " " + route
		if seen[line] {
			return
		}
		seen[line] = true
		out = append(out, line)
	}
	for _, m := range cljRingMockRequestRE.FindAllStringSubmatch(source, -1) {
		add(m[1], m[2])
	}
	for _, m := range cljPeridotRequestRE.FindAllStringSubmatch(source, -1) {
		// peridot capture order is (route, verb).
		add(m[2], m[1])
	}
	return out
}

// normaliseClojureTestRoute reduces a raw route literal to a path: strips a
// scheme+authority prefix, drops query/fragment, collapses repeated slashes.
// Ring path-param placeholders (`:id`) and casing are preserved (the resolver
// wildcards templates and compares case-insensitively).
func normaliseClojureTestRoute(raw string) string {
	p := strings.TrimSpace(raw)
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
	for strings.Contains(p, "//") {
		p = strings.ReplaceAll(p, "//", "/")
	}
	return p
}

// isClojureTestFile reports whether path looks like a clojure.test source — a
// `*_test.clj`/`*_test.cljc` file or a file under a `/test/` directory. Keeps
// the route-hit suite off production handlers/routers that merely mention a
// route. Mirrors the Clojure convention `src/my/ns.clj → test/my/ns_test.clj`.
func isClojureTestFile(path string) bool {
	lp := strings.ToLower(filepath.ToSlash(path))
	if strings.HasSuffix(lp, "_test.clj") || strings.HasSuffix(lp, "_test.cljc") ||
		strings.HasSuffix(lp, "_test.cljs") {
		return true
	}
	if strings.Contains(lp, "/test/") &&
		(strings.HasSuffix(lp, ".clj") || strings.HasSuffix(lp, ".cljc") ||
			strings.HasSuffix(lp, ".cljs")) {
		return true
	}
	return false
}

// clojureTestSuiteBaseName derives a suite label from the test file path
// (`.../todo_handler_test.clj` → `todo_handler_test`).
func clojureTestSuiteBaseName(path string) string {
	base := filepath.Base(filepath.ToSlash(path))
	return strings.TrimSuffix(base, filepath.Ext(base))
}
