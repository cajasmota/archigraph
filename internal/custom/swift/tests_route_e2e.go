// tests_route_e2e.go — Swift XCTVapor test→endpoint route-hit linkage.
//
// #4749 (the Swift slice of the coverage-linkage tail epic #4749/#4615; mirrors
// the Elixir/Phoenix slice #4688). Emits ONE test_suite entity per Swift
// XCTVapor test file that drives an HTTP endpoint by ROUTE STRING via
// `app.test(.VERB, "path")` / `app.testable().test(.VERB, "path")`, stamping the
// captured `VERB route` pairs onto the suite's `e2e_route_calls` property. The
// shared resolve pass (engine.linkE2ERouteTestsToEndpoints, #4351/#4369) then
// matches each pair against the cross-file http_endpoint_definition index —
// which, for Swift, is populated by synthesizeVaporRoutes (#4749) — and emits a
// TESTS edge to the exact Vapor endpoint exercised, exactly as the Java junit5,
// Kotlin, Ruby, PHP, C# and Elixir slices do for their stacks. The engine pass
// is language-agnostic (it fires on any test_suite carrying e2e_route_calls),
// so the only Swift-specific work here is the XCTVapor route capture.
//
// XCTVapor route-driving idioms captured:
//   - app.test(.GET, "todos") { res in ... }
//   - try app.test(.POST, "todos", beforeRequest: { ... })
//   - try app.testable().test(.GET, "todos/\(id)") { res in ... }
//
// The first argument is a `.VERB` HTTPMethod literal; the second is the route
// (a string literal). The route may contain a leading slash or not — Vapor and
// the resolver tolerate both (the segment matcher strips the API prefix and
// wildcards template params).
//
// Local-variable / receiver typing (#4749 part a) is N/A for Swift coverage
// linkage: Swift extractor Operation entities are named by their BARE method
// name (not `Type.method`), and the cross-file receiver resolver is
// package-directory scoped (Go-style), so a `receiver_type` stamp on a Swift
// CALLS edge has no consumer and would be a dead annotation. The honest,
// working coverage mechanism for Swift/Vapor is the route-hit → endpoint
// linkage in THIS file (route dispatch is keyed by the literal route string,
// not by an `obj.method()` receiver), exactly as for the functional Elixir
// slice. See the PR / coverage-doc note for the recorded N/A rationale.
//
// XCTest test methods are NAMED `func testX()` operations the swift extractor
// already mines as call-bearing entities, so there is NO anonymous-test-block
// scope-owner gap (unlike JS/Ruby/PHP/Kotlin closure DSLs). Quick/Nimble's
// closure DSL (`it("...") { ... }`) is NOT covered here — if a repo uses it,
// file a follow-up (it would need a scope-owner like #4680/#4719).
//
// Honest exclusions (no fabricated edges):
//   - Interpolated / variable routes capture the literal prefix only when a
//     static string is present; a fully interpolated route (`"\(path)"`) is
//     dropped.
//   - Shape-only tests (assert on a model, never hit a route) emit no suite.
//
// Registration key: "custom_swift_tests_route_e2e".
package swift

import (
	"context"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/cajasmota/archigraph/internal/extractor"
	"github.com/cajasmota/archigraph/internal/types"
)

func init() {
	extractor.Register("custom_swift_tests_route_e2e", &swiftTestRouteE2EExtractor{})
}

type swiftTestRouteE2EExtractor struct{}

func (e *swiftTestRouteE2EExtractor) Language() string {
	return "custom_swift_tests_route_e2e"
}

// swiftXCTVaporTestRE matches an XCTVapor route-driving call:
//
//	app.test(.GET, "todos") { ... }
//	try app.testable().test(.POST, "todos/:id")
//
// Capture group 1 is the verb (.GET → GET); group 2 is the route literal. The
// `(?:testable\(\)\.)?` optional segment tolerates the `app.testable().test`
// form. Anchored on `.test(` so a production `.get(`/`.post(` route is never
// captured.
var swiftXCTVaporTestRE = regexp.MustCompile(
	`\.test\s*\(\s*\.(GET|POST|PUT|DELETE|PATCH|HEAD|OPTIONS)\s*,\s*"([^"\n\r]*)"`,
)

func (e *swiftTestRouteE2EExtractor) Extract(
	ctx context.Context,
	file extractor.FileInput,
) ([]types.EntityRecord, error) {
	if len(file.Content) == 0 || file.Language != "swift" {
		return nil, nil
	}
	if !isSwiftTestFile(file.Path) {
		return nil, nil
	}
	source := string(file.Content)
	routeCalls := collectSwiftTestRouteCalls(source)
	if len(routeCalls) == 0 {
		return nil, nil
	}
	rec := types.EntityRecord{
		Name:       swiftTestSuiteBaseName(file.Path),
		Kind:       "SCOPE.Operation",
		Subtype:    "test_suite",
		SourceFile: file.Path,
		Language:   "swift",
		StartLine:  1,
		EndLine:    1,
		Properties: map[string]string{
			"framework":       "xctvapor",
			"provenance":      "INFERRED_FROM_SWIFT_TEST_ROUTE_E2E",
			"e2e_route_calls": strings.Join(routeCalls, "\n"),
		},
	}
	rec.ID = rec.ComputeID()
	return []types.EntityRecord{rec}, nil
}

// collectSwiftTestRouteCalls returns the de-duplicated "VERB route" pairs an
// XCTVapor test file drives by route string. Routes are normalised to a
// leading-slash path; fully-interpolated routes (no static literal) are dropped.
func collectSwiftTestRouteCalls(source string) []string {
	var out []string
	seen := map[string]bool{}
	for _, m := range swiftXCTVaporTestRE.FindAllStringSubmatch(source, -1) {
		verb := strings.ToUpper(m[1])
		route := normaliseSwiftTestRoute(m[2])
		if route == "" {
			continue
		}
		line := verb + " " + route
		if seen[line] {
			continue
		}
		seen[line] = true
		out = append(out, line)
	}
	return out
}

// normaliseSwiftTestRoute reduces a raw XCTVapor route literal to a path:
// ensures a single leading slash, drops a query/fragment tail, collapses
// repeated slashes. A route whose value is entirely a string interpolation
// (`\(...)`) with no static prefix is dropped (returns ""). Vapor path-param
// placeholders (`:id`) and casing are preserved (the resolver wildcards
// templates and compares case-insensitively).
func normaliseSwiftTestRoute(raw string) string {
	p := strings.TrimSpace(raw)
	if p == "" {
		return ""
	}
	// Truncate at the first interpolation marker — the static prefix is the
	// only statically-recoverable part. `"todos/\(id)"` → `/todos`.
	if i := strings.Index(p, `\(`); i >= 0 {
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

// isSwiftTestFile reports whether path looks like an XCTest source — a
// `*Tests.swift` file or a file under a `/Tests/` directory. Keeps the
// route-hit suite off production controllers that merely mention a route.
func isSwiftTestFile(path string) bool {
	lp := filepath.ToSlash(path)
	base := filepath.Base(lp)
	if strings.HasSuffix(base, "Tests.swift") || strings.HasSuffix(base, "Test.swift") {
		return true
	}
	if strings.Contains(lp, "/Tests/") && strings.HasSuffix(lp, ".swift") {
		return true
	}
	return false
}

// swiftTestSuiteBaseName derives a suite label from the test file path
// (`.../TodoControllerTests.swift` → `TodoControllerTests`).
func swiftTestSuiteBaseName(path string) string {
	base := filepath.Base(filepath.ToSlash(path))
	return strings.TrimSuffix(base, filepath.Ext(base))
}
