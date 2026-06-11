// tests_route_e2e.go — Crystal spec → endpoint route-hit linkage.
//
// #4749 (the Crystal slice of the coverage-linkage tail epic #4749/#4615;
// mirrors the Swift/Vapor slice #4755 and the functional Elixir slice #4688).
// Emits ONE test_suite entity per Crystal spec file that drives an HTTP endpoint
// by ROUTE STRING via the spec-kemal request helpers (`get "/path"`,
// `post "/path"`, …), stamping the captured `VERB route` pairs onto the suite's
// `e2e_route_calls` property (one "VERB route" per line). The shared resolve
// pass (engine.linkE2ERouteTestsToEndpoints, #4351/#4369) then matches each pair
// against the cross-file http_endpoint_definition index — which, for Crystal, is
// populated by synthesizeKemalRoutes (#4749) — and emits a TESTS edge to the
// exact Kemal/Amber/Lucky endpoint exercised, exactly as the Swift, Rust, Java,
// Ruby, PHP, C# and Elixir slices do for their stacks. The engine pass is
// language-agnostic (it fires on any test_suite carrying e2e_route_calls), so
// the only Crystal-specific work here is the spec-kemal route capture.
//
// spec-kemal route-driving idioms captured:
//   - get "/todos"
//   - post "/todos", body: "...", headers: ...
//   - delete "/todos/#{id}"
//
// The request helper is a top-level verb call with a leading string-literal
// path. Crystal's `spec` test DSL (`describe "..." do ... it "..." do ... end`)
// uses ANONYMOUS closure blocks (like Ruby RSpec) — the `it` example body owns
// no named call-bearing entity — so the route-hit signal is carried by the
// SUITE-LEVEL test_suite entity emitted here (the scope-owner role), NOT by a
// per-example operation. This is the Crystal analog of the Ruby #4684 /
// JS #4680 anonymous-block scope-owner: the test_suite is the owner that carries
// the e2e_route_calls the engine links from.
//
// Local-variable / receiver typing (#4749 part a) is N/A for Crystal coverage
// linkage: the crystal base extractor names method (def) entities by their BARE
// name (not `Type.method`), and there is no class-qualified receiver resolver to
// consume a `receiver_type` stamp on a Crystal CALLS edge — it would be a dead
// annotation. The honest, working coverage mechanism for Crystal is the
// route-hit → endpoint linkage in THIS file (route dispatch is keyed by the
// literal route string, not by an `obj.method()` receiver), exactly as for the
// functional Elixir and Swift slices. See the coverage-doc note for the recorded
// N/A rationale.
//
// Honest exclusions (no fabricated edges):
//   - Interpolated / variable routes capture the literal prefix only when a
//     static string is present; a fully interpolated route (`"#{path}"`) is
//     dropped.
//   - Non-request specs (pure model/unit specs that never hit a route) emit no
//     suite.
//   - A `get "/x"` appearing OUTSIDE a spec file is left to the producer-side
//     synthesizer (synthesizeKemalRoutes); this extractor only runs on specs.
//
// Registration key: "custom_crystal_tests_route_e2e".
package crystal

import (
	"context"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/cajasmota/archigraph/internal/extractor"
	"github.com/cajasmota/archigraph/internal/types"
)

func init() {
	extractor.Register("custom_crystal_tests_route_e2e", &crystalTestRouteE2EExtractor{})
}

type crystalTestRouteE2EExtractor struct{}

func (e *crystalTestRouteE2EExtractor) Language() string {
	return "custom_crystal_tests_route_e2e"
}

// crystalSpecRouteRE matches a spec-kemal request helper call with a leading
// string-literal path:
//
//	get "/todos"
//	post "/todos", body: "..."
//	delete "/todos/#{id}"
//
// Anchored at a statement boundary (`^[ \t]*`) so an `obj.get("...")` receiver
// call is not captured. Capture group 1 is the verb; group 2 is the route
// literal.
var crystalSpecRouteRE = regexp.MustCompile(
	`(?m)^[ \t]*(get|post|put|delete|patch|options|head)\s+"([^"\n\r]*)"`,
)

func (e *crystalTestRouteE2EExtractor) Extract(
	ctx context.Context,
	file extractor.FileInput,
) ([]types.EntityRecord, error) {
	if len(file.Content) == 0 || file.Language != "crystal" {
		return nil, nil
	}
	if !isCrystalSpecFile(file.Path) {
		return nil, nil
	}
	source := string(file.Content)
	routeCalls := collectCrystalSpecRouteCalls(source)
	if len(routeCalls) == 0 {
		return nil, nil
	}
	rec := types.EntityRecord{
		Name:       crystalSpecSuiteBaseName(file.Path),
		Kind:       "SCOPE.Operation",
		Subtype:    "test_suite",
		SourceFile: file.Path,
		Language:   "crystal",
		StartLine:  1,
		EndLine:    1,
		Properties: map[string]string{
			"framework":       "spec-kemal",
			"provenance":      "INFERRED_FROM_CRYSTAL_TEST_ROUTE_E2E",
			"e2e_route_calls": strings.Join(routeCalls, "\n"),
		},
	}
	rec.ID = rec.ComputeID()
	return []types.EntityRecord{rec}, nil
}

// collectCrystalSpecRouteCalls returns the de-duplicated "VERB route" pairs a
// spec file drives by route string. Routes are normalised to a leading-slash
// path; fully-interpolated routes (no static literal) are dropped.
func collectCrystalSpecRouteCalls(source string) []string {
	var out []string
	seen := map[string]bool{}
	for _, m := range crystalSpecRouteRE.FindAllStringSubmatch(source, -1) {
		verb := strings.ToUpper(m[1])
		route := normaliseCrystalSpecRoute(m[2])
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

// normaliseCrystalSpecRoute reduces a raw spec-kemal route literal to a path:
// ensures a single leading slash, drops a query/fragment tail, collapses
// repeated slashes. A route whose value is entirely a string interpolation
// (`#{...}`) with no static prefix is dropped (returns ""). Path-param
// placeholders (`:id`) and casing are preserved (the resolver wildcards
// templates and compares case-insensitively).
func normaliseCrystalSpecRoute(raw string) string {
	p := strings.TrimSpace(raw)
	if p == "" {
		return ""
	}
	// Truncate at the first interpolation marker — the static prefix is the
	// only statically-recoverable part. `"todos/#{id}"` → `/todos`.
	if i := strings.Index(p, "#{"); i >= 0 {
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

// isCrystalSpecFile reports whether path looks like a Crystal spec — a
// `*_spec.cr` file or a file under a `/spec/` directory. Keeps the route-hit
// suite off production code that merely registers a route.
func isCrystalSpecFile(path string) bool {
	lp := filepath.ToSlash(path)
	base := filepath.Base(lp)
	if strings.HasSuffix(base, "_spec.cr") {
		return true
	}
	if strings.Contains(lp, "/spec/") && strings.HasSuffix(lp, ".cr") {
		return true
	}
	return false
}

// crystalSpecSuiteBaseName derives a suite label from the spec file path
// (`.../todos_spec.cr` → `todos_spec`).
func crystalSpecSuiteBaseName(path string) string {
	base := filepath.Base(filepath.ToSlash(path))
	return strings.TrimSuffix(base, filepath.Ext(base))
}
