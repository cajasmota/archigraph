// http_endpoint_kemal.go — Crystal Kemal / Amber route registration → canonical
// http_endpoint_definition synthesis (#4749, the Crystal slice of the
// coverage-linkage tail epic #4749/#4615; mirrors the Swift/Vapor slice #4755).
//
// Background
// ----------
// The Crystal base extractor (internal/extractors/crystal/extractor.go) is a
// regex-based structural extractor: it mines classes / modules / defs / macros
// and CALLS edges, but has NO web-framework awareness — Kemal `get "/path" do`
// blocks and Amber `routes :web do get "/path", Ctrl, :index end` registrations
// are not recognised as HTTP endpoints, and no `http_endpoint_definition`
// entity is ever produced for Crystal. The shared e2e route-test linker
// (linkE2ERouteTestsToEndpoints, #4351) matches a test's route hit against
// `http_endpoint_definition` + `path`, so a Crystal route could never be hit by
// a route-string test. As with Swift/Vapor (#4755), the PRODUCER-side gap has
// to be closed first.
//
// This pass emits one canonical http_endpoint_definition per statically-known
// Crystal route, in the SAME shape axum / Rocket / Express / Vapor emit, so the
// existing resolver and the language-agnostic e2e route-test linker light up for
// Crystal exactly as they do for the flagship stacks.
//
// Crystal route syntax
// --------------------
// Kemal (the dominant Crystal web framework, Sinatra-like) registers a route
// with a top-level verb macro taking a single string-literal path and a block:
//
//	get "/" do ... end                         → GET /
//	get "/users/:id" do |env| ... end          → GET /users/{id}
//	post "/users" do ... end                   → POST /users
//	delete "/users/:id" { ... }                → DELETE /users/{id}
//	ws "/socket" do |socket| ... end           → (websocket — not an HTTP verb, skipped)
//
// Amber registers routes inside a `routes` pipeline block, mapping a path to a
// controller + action symbol:
//
//	routes :web do
//	  get "/", HomeController, :index
//	  post "/users", UsersController, :create
//	end
//
// Both use the Sinatra/Express-style `:name` colon path-parameter convention,
// canonicalised through FrameworkKemal into the `{param}` form shared with every
// other framework.
//
// Lucky uses one Action CLASS per endpoint with a `route` / `get "/path"` macro
// DSL inside the class body; the verb+path are statically recoverable from the
// same top-level-looking `get "/path"` macro the regex below already captures
// (Lucky's `get "/path"` inside an Action class). The class-scoped Action form
// without an inline path literal (route derived from the class NAME, e.g.
// `Users::Index` → `/users`) is NOT statically recoverable here and is a
// documented honest exclusion.
//
// Honest exclusions (no fabricated routes)
// ----------------------------------------
//   - Interpolated / variable paths (`get url do`, `get "#{prefix}/x" do`) —
//     not statically recoverable, dropped.
//   - `ws "/..."` websocket routes — not an HTTP verb, skipped (the resolver
//     keys off HTTP verbs; a WS endpoint is a different surface).
//   - Lucky name-derived Action routes (no inline path literal) — left to the
//     producer follow-up; route-string linkage covers the inline-path form.
package engine

import (
	"regexp"
	"strings"

	"github.com/cajasmota/grafel/internal/engine/httproutes"
)

// kemalRouteRe matches a Crystal verb-macro route registration with a leading
// string-literal path:
//
//	get "/users/:id" do ... end
//	post "/users", UsersController, :create   (Amber controller form)
//	delete "/users/:id" { ... }
//
// The verb macro must appear at a statement boundary (start of line, after
// optional indentation) so an arbitrary method named `get`/`post` invoked as
// `obj.get("...")` is NOT misread as a route (that has a `.` receiver before
// the verb and fails the `^[ \t]*` anchor). Capture group 1 is the verb;
// group 2 is the path literal.
var kemalRouteRe = regexp.MustCompile(
	`(?m)^[ \t]*(get|post|put|delete|patch|options|head)\s+"([^"\n\r]*)"`,
)

// kemalHasRoute is a fast pre-filter: the file must reference a Crystal web
// marker (Kemal / Lucky / Amber) AND a verb-macro call to be worth scanning, so
// we never misfire on arbitrary Crystal code that happens to call a `get`
// method.
func kemalHasRoute(content string) bool {
	if !strings.Contains(content, `get "`) &&
		!strings.Contains(content, `post "`) &&
		!strings.Contains(content, `put "`) &&
		!strings.Contains(content, `delete "`) &&
		!strings.Contains(content, `patch "`) &&
		!strings.Contains(content, `options "`) &&
		!strings.Contains(content, `head "`) {
		return false
	}
	return strings.Contains(content, "Kemal") ||
		strings.Contains(content, "kemal") ||
		strings.Contains(content, "Amber") ||
		strings.Contains(content, "Lucky") ||
		strings.Contains(content, "Action") ||
		strings.Contains(content, "routes ") ||
		strings.Contains(content, "env.") ||
		strings.Contains(content, "HTTP::")
}

// synthesizeKemalRoutes scans a Crystal source file for Kemal / Amber / Lucky
// route registrations and emits one http_endpoint_definition per statically-
// known (verb, path).
func synthesizeKemalRoutes(content string, emit emitFn) {
	if !kemalHasRoute(content) {
		return
	}
	for _, m := range kemalRouteRe.FindAllStringSubmatch(content, -1) {
		if len(m) < 3 {
			continue
		}
		verb := strings.ToUpper(m[1])
		raw := strings.TrimSpace(m[2])
		// Drop interpolated / empty literals — not statically recoverable.
		if raw == "" || strings.Contains(raw, "#{") {
			continue
		}
		if !strings.HasPrefix(raw, "/") {
			raw = "/" + raw
		}
		canonical := httproutes.Canonicalize(httproutes.FrameworkKemal, raw)
		if canonical == "" {
			continue
		}
		// Handler kind "Controller" maps to SCOPE.Operation in the resolver,
		// the kind Crystal defs land as — matching the axum/vapor convention so
		// ResolveHTTPEndpointHandlers can bind a handler IMPLEMENTS edge when a
		// same-named handler exists (no name forces a handler when absent).
		emit(verb, canonical, "kemal", "Controller", "")
	}
}
