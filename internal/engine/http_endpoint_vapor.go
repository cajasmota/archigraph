// http_endpoint_vapor.go — Swift Vapor route registration → canonical
// http_endpoint_definition synthesis (#4749, the Swift slice of the
// coverage-linkage tail epic #4749/#4615).
//
// Background
// ----------
// The custom_swift_vapor extractor (internal/custom/swift/vapor.go) already
// emits `SCOPE.Operation` / Subtype="endpoint" markers for Vapor routes, but
// those are NOT `http_endpoint_definition` entities and carry a `route_path`
// (not `path`) property — so the shared endpoint resolver
// (ResolveHTTPEndpointHandlers) and, in particular, the e2e route-test linker
// (linkE2ERouteTestsToEndpoints, #4351) never saw them. Vapor endpoints could
// therefore never be matched by a route-string test call, and the XCTVapor
// `app.test(.GET, "/path")` coverage signal had no definition to bind to.
//
// This pass closes the PRODUCER-side gap: it emits one canonical
// http_endpoint_definition per statically-known Vapor route, in the SAME shape
// the axum / Rocket / Express synthesizers emit, so the existing resolver and
// the language-agnostic e2e route-test linker light up for Swift exactly as
// they do for the nine flagship stacks.
//
// Vapor route syntax
// ------------------
// A Vapor route is registered on the application or a RoutesBuilder via one of
// the HTTP-verb helpers, taking a VARIADIC list of path COMPONENTS followed by
// a trailing handler closure / handler reference:
//
//	app.get("todos") { req in ... }                       → GET /todos
//	app.get("todos", ":todoID") { req in ... }            → GET /todos/{todoID}
//	routes.post("users", "all")                           → POST /users/all
//	app.on(.GET, "health") { ... }                        → GET /health
//
// Each component is either a STRING LITERAL segment or a `:param` dynamic
// component (Vapor's `.parameter` shorthand). Components are joined with `/`
// and canonicalised through FrameworkVapor (colon-param convention) into the
// `{param}` form shared with every other framework.
//
// Honest exclusions (no fabricated routes)
// ----------------------------------------
//   - Interpolated / variable components (`app.get(somePath)`,
//     `app.get("\(prefix)")`) — not statically recoverable, dropped.
//   - PathComponent catch-alls (`"**"`, `"*"`) are passed through as literal
//     segments (the canonicaliser leaves them intact); they still match a
//     concrete test segment via the resolver's wildcard handling.
//   - `.grouped("prefix")` route-group prefixes are NOT yet threaded onto the
//     nested routes (single-statement scan only); a flat `app.verb(...)` route
//     is the common Vapor controller shape and is fully covered. Group-prefix
//     threading is left as a documented follow-up.
package engine

import (
	"regexp"
	"strings"

	"github.com/cajasmota/grafel/internal/engine/httproutes"
)

// vaporRouteRe matches a Vapor verb registration up to (but not including) the
// trailing handler. Capture group 1 is the verb; group 2 is the raw argument
// list (the variadic path components, possibly followed by middleware/handler
// args we filter out downstream).
//
//	app.get("todos", ":todoID") { ... }
//	routes.post("users")
//	grouped.delete("todos", ":id")
//
// The receiver is `app`, `routes`, `grouped`, or any identifier ending in
// `routes`/`Routes`/`Builder` (a RoutesBuilder convention) — kept permissive
// since Vapor controllers receive an opaque `routes: RoutesBuilder`.
var vaporRouteRe = regexp.MustCompile(
	`(?m)\b([A-Za-z_]\w*)\.(get|post|put|delete|patch)\s*\(([^){}]*)\)`,
)

// vaporOnRe matches the explicit `.on(.VERB, components...)` form.
//
//	app.on(.GET, "health") { ... }
var vaporOnRe = regexp.MustCompile(
	`(?m)\b[A-Za-z_]\w*\.on\s*\(\s*\.(GET|POST|PUT|DELETE|PATCH|HEAD|OPTIONS)\s*,([^){}]*)\)`,
)

// vaporStringComponentRe extracts each double-quoted string-literal path
// component from a Vapor route argument list.
var vaporStringComponentRe = regexp.MustCompile(`"([^"\n\r]*)"`)

// vaporHasVapor is a fast pre-filter: the file must reference a Vapor routing
// receiver verb call to be worth scanning.
func vaporHasVapor(content string) bool {
	if !strings.Contains(content, ".get(") &&
		!strings.Contains(content, ".post(") &&
		!strings.Contains(content, ".put(") &&
		!strings.Contains(content, ".delete(") &&
		!strings.Contains(content, ".patch(") &&
		!strings.Contains(content, ".on(") {
		return false
	}
	// Require a Vapor/RoutesBuilder marker so we don't misfire on arbitrary
	// Swift `.get(`/`.post(` calls (e.g. URLSession, dictionary access).
	return strings.Contains(content, "Vapor") ||
		strings.Contains(content, "RouteCollection") ||
		strings.Contains(content, "RoutesBuilder") ||
		strings.Contains(content, "req.") ||
		strings.Contains(content, "Request") ||
		regexp.MustCompile(`\b(app|routes|grouped)\.(get|post|put|delete|patch|on)\b`).MatchString(content)
}

// vaporRouteReceivers is the set of receiver identifiers that denote a Vapor
// RoutesBuilder. A bare `.get(`/`.post(` on some OTHER receiver (a dictionary,
// an Array, a URLSession) must NOT be misread as a route, so the synthesizer
// requires the receiver to look like a routes builder.
func isVaporRouteReceiver(recv string) bool {
	switch recv {
	case "app", "routes", "grouped", "group", "builder", "api", "protected", "authed":
		return true
	}
	lc := strings.ToLower(recv)
	return strings.HasSuffix(lc, "routes") || strings.HasSuffix(lc, "builder") ||
		strings.HasSuffix(lc, "group")
}

// synthesizeVaporRoutes scans a Swift source file for Vapor route registrations
// and emits one http_endpoint_definition per statically-known (verb, path).
func synthesizeVaporRoutes(content string, emit emitFn) {
	if !vaporHasVapor(content) {
		return
	}

	emitRoute := func(verb, argList string) {
		path, ok := vaporComponentsToPath(argList)
		if !ok {
			return
		}
		canonical := httproutes.Canonicalize(httproutes.FrameworkVapor, path)
		if canonical == "" {
			return
		}
		// Handler kind "Controller" maps to SCOPE.Operation in the resolver,
		// the kind Swift functions land as — matching the axum convention so
		// ResolveHTTPEndpointHandlers can bind a handler IMPLEMENTS edge when a
		// same-named handler exists (no name forces a handler when absent).
		emit(strings.ToUpper(verb), canonical, "vapor", "Controller", "")
	}

	for _, m := range vaporRouteRe.FindAllStringSubmatch(content, -1) {
		if len(m) < 4 {
			continue
		}
		if !isVaporRouteReceiver(m[1]) {
			continue
		}
		emitRoute(m[2], m[3])
	}
	for _, m := range vaporOnRe.FindAllStringSubmatch(content, -1) {
		if len(m) < 3 {
			continue
		}
		emitRoute(m[1], m[2])
	}
}

// vaporComponentsToPath joins a Vapor route argument list's string-literal path
// components into a single `/`-separated path. Returns ok=false when the route
// has NO static string components (e.g. `app.get(somePath)` — a fully dynamic
// route that is not statically recoverable). A route with at least one literal
// component yields a path; `:param` dynamic components are carried INSIDE the
// quoted literals (Vapor's `":todoID"` form), so the string-literal walk
// captures them and canonicalizeColonParams folds them to `{todoID}`.
func vaporComponentsToPath(argList string) (string, bool) {
	matches := vaporStringComponentRe.FindAllStringSubmatch(argList, -1)
	if len(matches) == 0 {
		return "", false
	}
	var segs []string
	for _, sm := range matches {
		comp := strings.TrimSpace(sm[1])
		// Drop interpolated literals — not statically recoverable.
		if comp == "" || strings.Contains(comp, "\\(") {
			continue
		}
		// A single literal may itself contain slashes (`"todos/all"`); split so
		// each becomes its own canonical segment.
		for _, part := range strings.Split(comp, "/") {
			part = strings.TrimSpace(part)
			if part != "" {
				segs = append(segs, part)
			}
		}
	}
	if len(segs) == 0 {
		return "", false
	}
	return "/" + strings.Join(segs, "/"), true
}
