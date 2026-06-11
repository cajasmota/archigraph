// Package rust — Rust test→endpoint route-hit linkage.
//
// #4749 (the Rust slice of epic #4615/#4615-tail #4749, all-framework
// test→endpoint coverage linkage). Emits ONE test_suite entity per Rust test
// file that drives an HTTP endpoint by ROUTE STRING, stamping the captured
// `VERB route` pairs onto the suite's `e2e_route_calls` property. The shared
// resolve pass (engine.linkE2ERouteTestsToEndpoints, #4351/#4369) then matches
// each pair against the cross-file http_endpoint_definition index and emits a
// TESTS edge to the exact endpoint exercised — exactly as the Go httptest
// (internal/custom/golang/httptest_e2e.go) and Kotlin
// (internal/custom/kotlin/tests_route_e2e.go) extractors do. The engine pass is
// language-agnostic (it fires on any test_suite carrying e2e_route_calls), so
// the only Rust-specific work is the route capture.
//
// Rust tests are NAMED functions annotated `#[test]` / `#[tokio::test]` /
// `#[actix_web::test]` / `#[actix_rt::test]` — there is no closure test DSL, so
// no anonymous-test-block scope-owner is needed (same as Go/Java). The dominant
// linkage mechanism for Rust is therefore route-hit linkage; Rust handlers are
// usually free functions wired into a router by path, so local-variable
// receiver typing of a constructed handler is not the common shape (recorded
// N/A — see the issue note). Three route-driving idioms are captured:
//
//   - Actix-web test: build a request with `test::TestRequest::get().uri("/p")`
//     (or `.post()/.put()/.patch()/.delete()/.head()`, or the explicit
//     `TestRequest::with_uri("/p")` / `.method(Method::POST)` form) and dispatch
//     via `test::call_service(&app, req)`.
//   - Axum / tower test: `app.oneshot(Request::get("/p").body(...))` (also
//     `Request::builder().method(Method::GET).uri("/p")`).
//   - Rocket test: `client.get("/p").dispatch()` (and the other verb builders).
//   - reqwest against a spawned test server: `reqwest::Client::new().get(addr +
//     "/p")` / `client.post(format!("{addr}/p"))` — the literal "/..."-shaped
//     path suffix is captured; a fully-variable URL is dropped (conservative).
//
// Registration key: "custom_rust_tests_route_e2e".
package rust

import (
	"context"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/cajasmota/archigraph/internal/extractor"
	"github.com/cajasmota/archigraph/internal/types"
)

func init() {
	extractor.Register("custom_rust_tests_route_e2e", &rustTestRouteE2EExtractor{})
}

type rustTestRouteE2EExtractor struct{}

func (e *rustTestRouteE2EExtractor) Language() string {
	return "custom_rust_tests_route_e2e"
}

var (
	// Actix builder: test::TestRequest::get().uri("/api/v1/x")
	// (also TestRequest::post()/put()/patch()/delete()/head()). The verb is the
	// builder method; the route the first uri("...") string literal. The receiver
	// may be bare `TestRequest::` or `test::TestRequest::`.
	rustActixTestReqVerbRE = regexp.MustCompile(
		`(?s)\bTestRequest\s*::\s*(get|post|put|patch|delete|head)\s*\(\s*\)\s*\.\s*uri\s*\(\s*"([^"\n\r]+)"`)

	// Actix explicit form: TestRequest::with_uri("/api/v1/x").method(Method::POST)
	// — verb defaults to GET when no .method() follows. Captured route first; the
	// verb (if any) is recovered from a following `.method(Method::VERB)`.
	rustActixWithURIRE = regexp.MustCompile(
		`(?s)\bTestRequest\s*::\s*with_uri\s*\(\s*"([^"\n\r]+)"\s*\)((?:\s*\.\s*method\s*\(\s*Method\s*::\s*[A-Za-z]+\s*\))?)`)
	rustActixMethodRE = regexp.MustCompile(
		`\.\s*method\s*\(\s*Method\s*::\s*([A-Za-z]+)\s*\)`)

	// Axum / hyper Request builder: Request::get("/p") / Request::post("/p"),
	// also Request::builder().method(Method::GET).uri("/p"). The first form is a
	// verb-named constructor; the second is captured by rustReqBuilderRE.
	rustAxumRequestVerbRE = regexp.MustCompile(
		`\bRequest\s*::\s*(get|post|put|patch|delete|head)\s*\(\s*"([^"\n\r]+)"`)
	// Bounded non-greedy span (no lookahead — RE2): from the builder() up to the
	// first .uri("..."), capturing the intervening chain (which may carry a
	// .method(Method::VERB)). Stops at the first uri literal.
	rustReqBuilderRE = regexp.MustCompile(
		`(?s)\bRequest\s*::\s*builder\s*\(\s*\)(.*?)\.\s*uri\s*\(\s*"([^"\n\r]+)"`)
	rustReqBuilderMethodRE = regexp.MustCompile(
		`\.\s*method\s*\(\s*Method\s*::\s*([A-Za-z]+)\s*\)`)

	// Rocket client: client.get("/p").dispatch() (and post/put/patch/delete/head).
	// Anchored on a `.` receiver so a Rocket route-attr factory is not captured.
	rustRocketClientRE = regexp.MustCompile(
		`\.\s*(get|post|put|patch|delete|head)\s*\(\s*"(/[^"\n\r]*)"\s*\)`)

	// reqwest client against a test server: client.get(addr + "/p"),
	// reqwest::Client::new().post(format!("{}/p", addr)). We match the FIRST
	// "/..."-shaped string literal in a reqwest verb call; a bare base URL with no
	// literal path suffix yields no route (dropped, conservative).
	rustReqwestVerbRE = regexp.MustCompile(
		`\breqwest\s*::\s*(?:Client\s*::\s*(?:new|builder)\s*\([^)]*\)[^;]*?|get|post|put|patch|delete|head)`)
	// A reqwest verb call: client.get( ... "..."-literal ... ). The route literal
	// may be a bare "/path" OR a format!("{}/path", addr) / format!("{addr}/path")
	// where the path suffix follows a `{...}` placeholder. Group 2 captures the
	// raw literal body; extractReqwestPath then recovers the leading-slash path.
	rustReqwestCallRE = regexp.MustCompile(
		`\.\s*(get|post|put|patch|delete|head)\s*\([^)\n]*?"([^"\n\r]*)"`)
	// rustReqwestPlaceholderPathRE recovers a "/path" suffix that follows a
	// format! placeholder — `"{}/api/v1/health"` → `/api/v1/health`,
	// `"{addr}/users/1"` → `/users/1`.
	rustReqwestPlaceholderPathRE = regexp.MustCompile(`\}(/[^{}\n\r]*)`)

	// rocketVerb / rustVerb canonicalisers reuse strings.ToUpper.
)

func (e *rustTestRouteE2EExtractor) Extract(
	ctx context.Context,
	file extractor.FileInput,
) ([]types.EntityRecord, error) {
	if len(file.Content) == 0 || file.Language != "rust" {
		return nil, nil
	}
	source := string(file.Content)
	if !isRustTestFile(file.Path, source) {
		return nil, nil
	}
	routeCalls := collectRustTestRouteCalls(source)
	if len(routeCalls) == 0 {
		return nil, nil
	}
	framework := detectRustTestRouteFramework(source)
	ent := makeEntity(
		"rust_route_suite:"+rustTestSuiteBaseName(file.Path),
		"SCOPE.Operation", "test_suite", file.Path, "rust", 1)
	setProps(&ent,
		"framework", framework,
		"provenance", "INFERRED_FROM_RUST_TEST_ROUTE_E2E",
		"test_framework", framework,
		"e2e_route_calls", strings.Join(routeCalls, "\n"),
	)
	return []types.EntityRecord{ent}, nil
}

// collectRustTestRouteCalls returns the de-duplicated "VERB route" pairs a Rust
// test file drives by route string (Actix TestRequest / Axum-tower oneshot /
// Rocket client / reqwest test-server). Routes are normalised to a leading-slash
// path; non-path / variable-route literals are dropped (honest exclusion).
func collectRustTestRouteCalls(source string) []string {
	var out []string
	seen := map[string]bool{}
	add := func(verb, rawRoute string) {
		route := normaliseRustTestRoute(rawRoute)
		if route == "" || !strings.HasPrefix(route, "/") {
			return
		}
		verb = strings.ToUpper(strings.TrimSpace(verb))
		if verb == "" {
			return
		}
		line := verb + " " + route
		if seen[line] {
			return
		}
		seen[line] = true
		out = append(out, line)
	}

	for _, m := range rustActixTestReqVerbRE.FindAllStringSubmatch(source, -1) {
		add(m[1], m[2])
	}
	for _, m := range rustActixWithURIRE.FindAllStringSubmatch(source, -1) {
		verb := "GET"
		if mm := rustActixMethodRE.FindStringSubmatch(m[2]); mm != nil {
			verb = mm[1]
		}
		add(verb, m[1])
	}
	for _, m := range rustAxumRequestVerbRE.FindAllStringSubmatch(source, -1) {
		add(m[1], m[2])
	}
	for _, m := range rustReqBuilderRE.FindAllStringSubmatch(source, -1) {
		verb := "GET"
		if mm := rustReqBuilderMethodRE.FindStringSubmatch(m[1]); mm != nil {
			verb = mm[1]
		}
		add(verb, m[2])
	}
	for _, m := range rustRocketClientRE.FindAllStringSubmatch(source, -1) {
		add(m[1], m[2])
	}
	if rustReqwestVerbRE.MatchString(source) {
		for _, m := range rustReqwestCallRE.FindAllStringSubmatch(source, -1) {
			add(m[1], extractReqwestPath(m[2]))
		}
	}
	return out
}

// extractReqwestPath recovers the leading-slash path from a reqwest route
// literal: a bare "/path" is returned as-is; a format!-style literal
// ("{}/api/v1/health" / "{addr}/users/1") yields the suffix after the last
// `{...}` placeholder. A literal with no leading-slash path yields "" (dropped).
func extractReqwestPath(lit string) string {
	if strings.HasPrefix(lit, "/") {
		return lit
	}
	if m := rustReqwestPlaceholderPathRE.FindStringSubmatch(lit); m != nil {
		return m[1]
	}
	return ""
}

// detectRustTestRouteFramework labels the suite by the dominant route-driving
// idiom present, defaulting to "cargo_test".
func detectRustTestRouteFramework(source string) string {
	switch {
	case strings.Contains(source, "TestRequest") || strings.Contains(source, "actix_web::test"):
		return "actix-web"
	case strings.Contains(source, ".oneshot(") || strings.Contains(source, "Request::builder"):
		return "axum"
	case strings.Contains(source, ".dispatch("):
		return "rocket"
	case strings.Contains(source, "reqwest"):
		return "reqwest"
	default:
		return "cargo_test"
	}
}

// normaliseRustTestRoute reduces a raw route literal to a path: strips a
// scheme+authority prefix (http://127.0.0.1:8080/x → /x), drops query/fragment,
// collapses repeated slashes. Path-param placeholders ({id}) and casing are
// preserved (the resolver wildcards templates and compares case-insensitively).
func normaliseRustTestRoute(raw string) string {
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

// isRustTestFile reports whether path/source looks like a Rust test source — a
// file under a `tests/` integration dir, or any `.rs` file carrying a recognised
// test attribute (#[test] / #[tokio::test] / #[actix_web::test] / #[cfg(test)]).
// Keeps the route-hit suite off production handlers that merely mention a route.
func isRustTestFile(path, source string) bool {
	lp := strings.ToLower(filepath.ToSlash(path))
	if !strings.HasSuffix(lp, ".rs") {
		return false
	}
	if strings.Contains(lp, "/tests/") {
		return true
	}
	return strings.Contains(source, "#[cfg(test)]") ||
		strings.Contains(source, "#[test]") ||
		strings.Contains(source, "#[tokio::test]") ||
		strings.Contains(source, "#[actix_web::test]") ||
		strings.Contains(source, "#[actix_rt::test]") ||
		strings.Contains(source, "#[async_std::test]")
}

// rustTestSuiteBaseName derives a suite label from the test file path
// (`.../users_api_test.rs` → `users_api_test`).
func rustTestSuiteBaseName(path string) string {
	base := filepath.Base(filepath.ToSlash(path))
	return strings.TrimSuffix(base, filepath.Ext(base))
}
