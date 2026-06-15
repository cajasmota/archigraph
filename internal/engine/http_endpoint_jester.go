// http_endpoint_jester.go — Nim Jester / Prologue / HappyX route registration →
// canonical http_endpoint_definition synthesis (#4749, the Nim slice of the
// coverage-linkage tail epic #4749/#4615; mirrors the Crystal/Kemal slice and
// the Lua/Lapis slice).
//
// Background
// ----------
// The Nim base extractor (internal/extractors/nim/nim.go) is a regex-based
// structural extractor: it mines proc/func/method/template/macro declarations,
// types, imports and CALLS edges, but has NO web-framework awareness. Jester
// `routes:` blocks, Prologue `app.get("/path", handler)` registrations and
// HappyX route macros are not recognised as HTTP endpoints, and no
// `http_endpoint_definition` entity is ever produced for Nim. The shared e2e
// route-test linker (linkE2ERouteTestsToEndpoints, #4351) matches a test's
// route hit against `http_endpoint_definition` + `path`, so a Nim route could
// never be hit by a route-string test. As with Crystal/Kemal and Swift/Vapor,
// the PRODUCER-side gap has to be closed first.
//
// This pass emits one canonical http_endpoint_definition per statically-known
// Nim route, in the SAME shape axum / Rocket / Express / Kemal / Vapor emit, so
// the existing resolver and the language-agnostic e2e route-test linker light up
// for Nim exactly as they do for the flagship stacks.
//
// Nim web route syntax
// --------------------
// Jester (the dominant Nim web framework, Sinatra-like) declares routes inside a
// `routes:` block as indented verb entries taking a single string-literal path
// followed by a colon (block body):
//
//	routes:
//	  get "/":            ...                 → GET /
//	  get "/users/@id":   resp ...            → GET /users/{id}
//	  post "/users":      ...                 → POST /users
//	  delete "/users/@id": ...                → DELETE /users/{id}
//
// Jester path parameters use the `@name` AT-prefixed convention, canonicalised
// through FrameworkJester into the `{param}` form shared with every other
// framework.
//
// Prologue registers routes with a verb method on the app/router taking a
// string-literal path and a handler reference, using the curly-brace `{name}`
// path-parameter convention:
//
//	app.get("/users/{id}", getUser)          → GET /users/{id}
//	app.post("/users", createUser)           → POST /users
//	app.addRoute("/x", handler, HttpGet)     → GET /x   (verb is the 3rd arg)
//
// HappyX uses a `/path/{id}` curly-brace convention shared with Prologue and is
// covered by the same Prologue-shaped synthesis when the route is registered via
// a verb method.
//
// Honest exclusions (no fabricated routes)
// ----------------------------------------
//   - Interpolated / variable paths (`get pathVar:`, `get "/x/" & id:`) — not
//     statically recoverable, dropped.
//   - Jester `re"…"` regex routes and a method-set route (`get, post "/x":`)
//     beyond a single leading verb — left to a producer follow-up.
//   - Prologue grouped/prefixed routers where the mount prefix lives on a
//     separate `newGroup`/`addGroup` call — the per-verb path is still emitted,
//     the cross-call prefix join is a documented follow-up.
package engine

import (
	"regexp"
	"strings"

	"github.com/cajasmota/grafel/internal/engine/httproutes"
)

var (
	// jesterRouteRe matches a Jester `routes:`-block verb entry: a verb keyword
	// at a statement boundary (start of line, after indentation), a
	// string-literal path, then a `:` introducing the block body. The trailing
	// `:` after the closing quote distinguishes a Jester route from a Prologue
	// `app.get("…", …)` call (which has a `(` before the string and `,`/`)`
	// after it). Capture group 1 is the verb; group 2 is the path literal.
	jesterRouteRe = regexp.MustCompile(
		`(?m)^[ \t]*(get|post|put|delete|patch|options|head)\s+"([^"\n\r]*)"\s*:`)

	// prologueVerbRe matches a Prologue verb-method route registration:
	// `app.get("/users/{id}", handler)` / `router.post("/x", h)`. The receiver
	// (`app`/`router`/any ident) precedes the `.verb(` call; capture group 1 is
	// the verb, group 2 the path literal.
	prologueVerbRe = regexp.MustCompile(
		`(?m)\b[A-Za-z_]\w*\.(get|post|put|delete|patch|options|head)\s*\(\s*"([^"\n\r]*)"`)

	// prologueAddRouteRe matches Prologue's `addRoute("/path", handler,
	// HttpGet)` form where the verb is the THIRD argument as an `Http<Verb>`
	// enum value. Capture group 1 is the path literal, group 2 the verb enum
	// tail (`Get`/`Post`/…).
	prologueAddRouteRe = regexp.MustCompile(
		`(?m)\baddRoute\s*\(\s*"([^"\n\r]*)"\s*,[^,)\n]*,\s*Http([A-Za-z]+)`)
)

// jesterHasRoute is a fast pre-filter: the file must reference a Jester web
// marker AND a verb-with-string route to be worth scanning, so we never misfire
// on arbitrary Nim code.
func jesterHasRoute(content string) bool {
	if !strings.Contains(content, "routes:") &&
		!strings.Contains(content, "import jester") &&
		!strings.Contains(content, "jester") {
		return false
	}
	return strings.Contains(content, `"`)
}

// prologueHasRoute is a fast pre-filter for Prologue / HappyX route
// registration files.
func prologueHasRoute(content string) bool {
	return strings.Contains(content, "prologue") ||
		strings.Contains(content, "Prologue") ||
		strings.Contains(content, "happyx") ||
		strings.Contains(content, "addRoute")
}

// synthesizeJester scans a Nim source file for Jester `routes:`-block verb
// entries and emits one http_endpoint_definition per statically-known
// (verb, path).
func synthesizeJester(content string, emit emitFn) {
	if !jesterHasRoute(content) {
		return
	}
	for _, m := range jesterRouteRe.FindAllStringSubmatch(content, -1) {
		if len(m) < 3 {
			continue
		}
		verb := strings.ToUpper(m[1])
		raw := strings.TrimSpace(m[2])
		if raw == "" || strings.Contains(raw, "&") || strings.Contains(raw, "$") {
			continue
		}
		if !strings.HasPrefix(raw, "/") {
			raw = "/" + raw
		}
		canonical := httproutes.Canonicalize(httproutes.FrameworkJester, raw)
		if canonical == "" {
			continue
		}
		// Handler kind "Controller" maps to SCOPE.Operation in the resolver,
		// the kind Nim procs land as — matching the Kemal/axum/vapor convention
		// so ResolveHTTPEndpointHandlers can bind a handler IMPLEMENTS edge when
		// a same-named handler exists (no name forces a handler when absent).
		emit(verb, canonical, "jester", "Controller", "")
	}
}

// synthesizePrologue scans a Nim source file for Prologue / HappyX verb-method
// and addRoute registrations and emits one http_endpoint_definition per
// statically-known (verb, path).
func synthesizePrologue(content string, emit emitFn) {
	if !prologueHasRoute(content) {
		return
	}
	for _, m := range prologueVerbRe.FindAllStringSubmatch(content, -1) {
		if len(m) < 3 {
			continue
		}
		verb := strings.ToUpper(m[1])
		raw := strings.TrimSpace(m[2])
		if raw == "" || strings.Contains(raw, "&") || strings.Contains(raw, "$") {
			continue
		}
		if !strings.HasPrefix(raw, "/") {
			raw = "/" + raw
		}
		canonical := httproutes.Canonicalize(httproutes.FrameworkPrologue, raw)
		if canonical == "" {
			continue
		}
		emit(verb, canonical, "prologue", "Controller", "")
	}
	for _, m := range prologueAddRouteRe.FindAllStringSubmatch(content, -1) {
		if len(m) < 3 {
			continue
		}
		raw := strings.TrimSpace(m[1])
		verb := strings.ToUpper(m[2])
		if raw == "" || strings.Contains(raw, "&") || strings.Contains(raw, "$") {
			continue
		}
		if !strings.HasPrefix(raw, "/") {
			raw = "/" + raw
		}
		canonical := httproutes.Canonicalize(httproutes.FrameworkPrologue, raw)
		if canonical == "" {
			continue
		}
		emit(verb, canonical, "prologue", "Controller", "")
	}
}
