// Package groovy — Groovy test→endpoint route-hit linkage.
//
// #4749 (the Groovy slice of the coverage-linkage tail epic #4749/#4615; the JVM
// analog of the Java junit5 and Kotlin #4687 slices). Emits ONE test_suite
// entity per Groovy test file that drives an HTTP endpoint by ROUTE STRING,
// stamping the captured `VERB route` pairs onto the suite's `e2e_route_calls`
// property (one "VERB route" per line). The shared resolve pass
// (engine.linkE2ERouteTestsToEndpoints, #4351/#4369) then matches each pair
// against the cross-file http_endpoint_definition index — which, for Groovy, is
// populated by synthesizeGroovyRoutes (#4749, Grails/Ratpack) — and emits a TESTS
// edge to the exact endpoint exercised, exactly as the Java/Kotlin/Crystal slices
// do. The engine pass is language-agnostic (it fires on any test_suite carrying
// e2e_route_calls), so the only Groovy-specific work here is the route capture.
//
// Route-driving idioms captured
// ------------------------------
//   - Spring on Groovy (Spock + Spring Boot Test): MockMvc
//     `mockMvc.perform(get("/path"))` and WebTestClient
//     `webTestClient.get().uri("/path")` — identical syntax to Java/Kotlin Spring.
//   - Grails functional/integration: the Grails REST client DSL and the
//     low-level `controller.request` form drive a route by string. The common
//     statically-recoverable shape is `get "/path"` / `post "/path"` (the Grails
//     RestBuilder / functional-test `get "$baseUrl/books"` idiom) and
//     `restBuilder.get("/path")`.
//   - Ratpack test harness: `testHttpClient.get("path")` /
//     `client.post("path")` (the EmbeddedApp / GroovyRatpackMainApplicationUnderTest
//     test client).
//
// Scope-owner (anonymous-block) note
// ----------------------------------
// A Spock feature method is declared `def "lists books"() { … }` with a STRING
// name. The Groovy tree-sitter grammar does NOT parse that string-named method as
// a normal method_definition (it surfaces as an ERROR/function_call node), so the
// base extractor emits NO method entity for it and any route hit inside its
// `when:` / `expect:` block would be orphaned. Therefore — exactly like the
// Crystal #4760 / Ruby #4684 / JS #4680 anonymous-block case — the route-hit
// signal is carried by the SUITE-LEVEL test_suite entity emitted here (the
// scope-owner role), not by a per-feature operation.
//
// Honest exclusions (no fabricated edges)
// ---------------------------------------
//   - Interpolated / variable routes — a fully `"${path}"` route is dropped; a
//     `"$baseUrl/books"` route keeps the static `/books` suffix only when a
//     static path segment is statically recoverable (leading-interpolation is
//     stripped to its trailing static path).
//   - Non-request specs (pure unit/model specs that never hit a route) emit no
//     suite.
//
// Registration key: "custom_groovy_tests_route_e2e".
package groovy

import (
	"context"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/cajasmota/grafel/internal/extractor"
	"github.com/cajasmota/grafel/internal/types"
)

func init() {
	extractor.Register("custom_groovy_tests_route_e2e", &groovyTestRouteE2EExtractor{})
}

type groovyTestRouteE2EExtractor struct{}

func (e *groovyTestRouteE2EExtractor) Language() string {
	return "custom_groovy_tests_route_e2e"
}

var (
	// Spring MockMvc: mockMvc.perform(get("/api/v1/x"))
	gvMockMvcRE = regexp.MustCompile(
		`\.perform\s*\(\s*(get|post|put|delete|patch)\s*\(\s*"([^"\n\r]+)"`)

	// Spring WebTestClient: webTestClient.get().uri("/api/v1/x").exchange()
	gvWebTestClientRE = regexp.MustCompile(
		`(?s)\.(get|post|put|delete|patch)\s*\(\s*\)\s*\.uri\s*\(\s*"([^"\n\r]+)"`)

	// Test-client receiver call: testHttpClient.get("path") / restBuilder.post("/x")
	// / client.put("path"). Anchored on a `.` receiver so a bare static helper
	// `get("/x")` (the Spring MockMvc factory, handled above) is not double-counted.
	gvClientRE = regexp.MustCompile(
		`\.(get|post|put|delete|patch)\s*\(\s*"([^"\n\r]*)"`)

	// Grails RestBuilder / functional-test top-level form: `get "/books"` /
	// `post "$baseUrl/books"`. Anchored at a statement boundary so an
	// `obj.get("...")` receiver call is not captured here.
	gvBareVerbRE = regexp.MustCompile(
		`(?m)(?:^|[={(]|\bwhen:|\bexpect:|\bthen:)\s*(get|post|put|delete|patch)\s+"([^"\n\r]*)"`)
)

func (e *groovyTestRouteE2EExtractor) Extract(
	ctx context.Context,
	file extractor.FileInput,
) ([]types.EntityRecord, error) {
	if len(file.Content) == 0 || file.Language != "groovy" {
		return nil, nil
	}
	if !isGroovyTestFile(file.Path) {
		return nil, nil
	}
	source := string(file.Content)
	routeCalls := collectGroovyTestRouteCalls(source)
	if len(routeCalls) == 0 {
		return nil, nil
	}
	framework := groovyTestFramework(source)
	rec := types.EntityRecord{
		Name:       groovyTestSuiteBaseName(file.Path),
		Kind:       "SCOPE.Operation",
		Subtype:    "test_suite",
		SourceFile: file.Path,
		Language:   "groovy",
		StartLine:  1,
		EndLine:    1,
		Properties: map[string]string{
			"framework":       framework,
			"provenance":      "INFERRED_FROM_GROOVY_TEST_ROUTE_E2E",
			"e2e_route_calls": strings.Join(routeCalls, "\n"),
		},
	}
	rec.ID = rec.ComputeID()
	return []types.EntityRecord{rec}, nil
}

// collectGroovyTestRouteCalls returns the de-duplicated "VERB route" pairs a
// Groovy test file drives by route string (Spring MockMvc / WebTestClient,
// receiver test-clients, and the Grails bare-verb form). Routes are normalised to
// a leading-slash path; non-path / fully-variable literals are dropped.
func collectGroovyTestRouteCalls(source string) []string {
	var out []string
	seen := map[string]bool{}
	add := func(verb, rawRoute string) {
		route := normaliseGroovyTestRoute(rawRoute)
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
	for _, m := range gvMockMvcRE.FindAllStringSubmatch(source, -1) {
		add(m[1], m[2])
	}
	for _, m := range gvWebTestClientRE.FindAllStringSubmatch(source, -1) {
		add(m[1], m[2])
	}
	for _, m := range gvClientRE.FindAllStringSubmatch(source, -1) {
		add(m[1], m[2])
	}
	for _, m := range gvBareVerbRE.FindAllStringSubmatch(source, -1) {
		add(m[1], m[2])
	}
	return out
}

// normaliseGroovyTestRoute reduces a raw route literal to a path: strips a
// scheme+authority prefix, drops a Groovy string-interpolation prefix
// (`"$baseUrl/books"` → `/books`), drops query/fragment, collapses repeated
// slashes. A route with NO static path (`"${url}"`) is dropped. Path-param
// placeholders ({id}) and casing are preserved (the resolver wildcards templates
// and compares case-insensitively).
func normaliseGroovyTestRoute(raw string) string {
	p := strings.TrimSpace(raw)
	if p == "" {
		return ""
	}
	// Strip a scheme://authority prefix.
	if i := strings.Index(p, "://"); i >= 0 {
		rest := p[i+3:]
		if slash := strings.IndexByte(rest, '/'); slash >= 0 {
			p = rest[slash:]
		} else {
			return ""
		}
	}
	// A leading Groovy interpolation (`$baseUrl/books` or `${baseUrl}/books`) —
	// keep from the first `/` after the interpolation token. A trailing
	// interpolation (`/books/${id}`) keeps the static prefix.
	if strings.HasPrefix(p, "$") {
		if slash := strings.IndexByte(p, '/'); slash >= 0 {
			p = p[slash:]
		} else {
			return ""
		}
	}
	// Truncate at any remaining interpolation marker — only the static prefix is
	// statically recoverable.
	if i := strings.Index(p, "${"); i >= 0 {
		p = p[:i]
	}
	if i := strings.Index(p, "$"); i >= 0 {
		// A bare `$id` Grails-style param inside the path: keep the prefix.
		p = p[:i]
	}
	if q := strings.IndexAny(p, "?#"); q >= 0 {
		p = p[:q]
	}
	p = strings.TrimSpace(p)
	if p == "" || p == "/" {
		return ""
	}
	if !strings.HasPrefix(p, "/") {
		p = "/" + p
	}
	for strings.Contains(p, "//") {
		p = strings.ReplaceAll(p, "//", "/")
	}
	return strings.TrimRight(p, "/")
}

// groovyTestFramework labels the suite by the dominant test-client signal.
func groovyTestFramework(source string) string {
	switch {
	case strings.Contains(source, "mockMvc") || strings.Contains(source, "MockMvc") ||
		strings.Contains(source, "WebTestClient") || strings.Contains(source, "webTestClient"):
		return "spring"
	case strings.Contains(source, "testHttpClient") || strings.Contains(source, "EmbeddedApp") ||
		strings.Contains(source, "ratpack") || strings.Contains(source, "Ratpack"):
		return "ratpack"
	case strings.Contains(source, "RestBuilder") || strings.Contains(source, "grails") ||
		strings.Contains(source, "Grails"):
		return "grails"
	case strings.Contains(source, "spock") || strings.Contains(source, "Specification"):
		return "spock"
	default:
		return "spock"
	}
}

// isGroovyTestFile reports whether path looks like a Groovy test/spec source —
// under a test source set (`src/test/`, `src/integration-test/`, `/test/`) or a
// *Spec/*Test/*IT named file. Keeps the route-hit suite off production code.
func isGroovyTestFile(path string) bool {
	lp := strings.ToLower(filepath.ToSlash(path))
	if strings.Contains(lp, "/src/test/") || strings.Contains(lp, "/src/integration-test/") ||
		strings.Contains(lp, "/test/") || strings.Contains(lp, "/spec/") {
		return true
	}
	base := strings.TrimSuffix(filepath.Base(lp), ".groovy")
	return strings.HasSuffix(base, "spec") || strings.HasSuffix(base, "test") ||
		strings.HasSuffix(base, "tests") || strings.HasSuffix(base, "it")
}

// groovyTestSuiteBaseName derives a suite label from the test file path
// (`.../BookControllerSpec.groovy` → `BookControllerSpec`).
func groovyTestSuiteBaseName(path string) string {
	base := filepath.Base(filepath.ToSlash(path))
	return strings.TrimSuffix(base, filepath.Ext(base))
}
