// http_endpoint_grails.go — Groovy Grails / Ratpack route registration →
// canonical http_endpoint_definition synthesis (#4749, the Groovy slice of the
// coverage-linkage tail epic #4749/#4615; mirrors the Crystal/Kemal #4760 and
// Swift/Vapor #4755 producer-first slices).
//
// Background
// ----------
// The Groovy base extractor (internal/extractors/groovy/groovy.go) is a
// tree-sitter structural extractor: it mines classes / methods / CALLS edges and
// Gradle DSL, but has NO web-framework awareness — Grails controller actions and
// Ratpack handler-DSL routes are not recognised as HTTP endpoints, and no
// http_endpoint_definition entity is ever produced for Groovy (the Grails YAML
// rule set is framework-DETECTION only; spring_routes.go is gated `lang=="java"`).
// The shared e2e route-test linker (linkE2ERouteTestsToEndpoints, #4351) matches
// a test's route hit against http_endpoint_definition + path, so a Groovy route
// could never be hit by a route-string test. As with Crystal/Swift, the
// PRODUCER-side gap has to be closed first.
//
// This pass emits one canonical http_endpoint_definition per statically-known
// Groovy route, in the SAME shape axum / Express / Vapor / Kemal emit, so the
// existing resolver and the language-agnostic e2e route-test linker light up for
// Groovy exactly as for the flagship stacks.
//
// Grails (convention)
// -------------------
// Grails maps a controller class + action methods to `/<controller>/<action>`
// by convention (the controller's leaf name minus the `Controller` suffix,
// lower-cased; each action method/closure becomes an action segment):
//
//	// grails-app/controllers/com/x/BookController.groovy
//	class BookController {
//	  def index() { ... }              → /book/index
//	  def show() { ... }               → /book/show
//	  def save() { ... }               → /book/save
//	}
//
// A Grails action responds to whatever HTTP method the URL mapping permits
// (GET by default, but `static allowedMethods` can widen it and the convention
// surface is method-agnostic), so the synthesized verb is ANY — the honest
// statically-recoverable contract. The shared linker's verbsMatchCompat treats
// ANY as matching every test verb, so a `GET "/book/index"` or
// `POST "/book/save"` integration test still links.
//
// Grails (explicit UrlMappings.groovy)
// ------------------------------------
// An explicit mapping declares a path with Grails dollar-prefixed parameters:
//
//	// grails-app/controllers/UrlMappings.groovy
//	"/book/$id"(controller: "book", action: "show")   → /book/{id}
//
// The synthesizer rewrites `$name` → `{name}` and canonicalises through
// FrameworkGrails (curly-brace pass). The verb is ANY unless a `method:` is set.
//
// Ratpack (handler DSL)
// ---------------------
// Ratpack registers routes with a verb-DSL handler taking a string-literal path:
//
//	get("api/books") { ... }            → GET /api/books
//	get("api/book/:id") { ... }         → GET /api/book/{id}
//	post("api/books") { ... }           → POST /api/books
//	path("health") { ... }              → (path() is verb-agnostic — ANY)
//
// Ratpack paths are declared WITHOUT a leading slash and use the Express-style
// `:name` colon parameter; canonicalised through FrameworkRatpack.
//
// Honest exclusions (no fabricated routes)
// ----------------------------------------
//   - Interpolated / variable paths (`get("${prefix}/x")`) — dropped.
//   - A controller class with NO action methods emits nothing.
//   - Ratpack `get(...)` WITHOUT a string-literal path (chain delegation) — dropped.
//   - Spring-on-Groovy `@GetMapping` controllers are already covered by the
//     shared annotation route surface where Groovy is treated as a JVM language;
//     this pass focuses on the Grails-convention + Ratpack idioms that have no
//     other producer.
package engine

import (
	"path/filepath"
	"regexp"
	"strings"

	"github.com/cajasmota/archigraph/internal/engine/httproutes"
)

// ratpackRouteRe matches a Ratpack verb-DSL handler with a leading
// string-literal path: `get("api/books") { ... }`, `post("books", handler)`.
// The verb is a method-call identifier at a statement boundary (the chain DSL
// puts each verb at the start of its segment), followed by `("path"`.
var ratpackRouteRe = regexp.MustCompile(
	`\b(get|post|put|delete|patch|options)\s*\(\s*"([^"\n\r]*)"`,
)

// ratpackPathRe matches the verb-agnostic `path("x") { … }` registration.
var ratpackPathRe = regexp.MustCompile(
	`\bpath\s*\(\s*"([^"\n\r]*)"`,
)

// grailsActionRe matches a Grails controller action declaration inside a
// controller class body — `def index() { … }`, `def show(Long id) { … }`,
// `def save = { … }` (closure-style action). Capture group 1 is the action name.
var grailsActionRe = regexp.MustCompile(
	`(?m)^\s*def\s+([a-zA-Z_]\w*)\s*(?:\([^)]*\)\s*\{|=\s*\{)`,
)

// grailsUrlMappingRe matches an explicit UrlMappings.groovy entry:
// `"/book/$id"(controller: "book", action: "show")`. Capture group 1 is the
// path, group 2 the trailing parenthesised config (controller/action/method).
var grailsUrlMappingRe = regexp.MustCompile(
	`(?m)^\s*"(/[^"\n\r]*)"\s*\(([^)]*)\)`,
)

// grailsControllerNameRe extracts the controller class name from a
// `class FooController { … }` declaration.
var grailsControllerNameRe = regexp.MustCompile(
	`\bclass\s+([A-Z]\w*Controller)\b`,
)

// synthesizeGroovyRoutes scans a Groovy source file for Grails controller
// conventions, Grails UrlMappings, and Ratpack handler-DSL routes, and emits one
// http_endpoint_definition per statically-known (verb, path).
func synthesizeGroovyRoutes(content, path string, emit emitFn) {
	src := content
	slashed := filepath.ToSlash(path)
	base := filepath.Base(slashed)

	// Grails explicit URL mappings (UrlMappings.groovy) take priority — they are
	// the authoritative route table when present.
	if strings.EqualFold(base, "UrlMappings.groovy") {
		synthesizeGrailsUrlMappings(src, emit)
		return
	}

	// Grails convention: a *Controller.groovy under grails-app/controllers/ (or
	// any file declaring a `class FooController`) maps each action to
	// `/<controller>/<action>`.
	if isGrailsControllerFile(slashed, src) {
		synthesizeGrailsConventionRoutes(src, emit)
	}

	// Ratpack handler DSL can appear in any Groovy file (ratpack.groovy, a
	// Handlers chain). The pre-filter keeps it off arbitrary Groovy.
	if ratpackLikely(src) {
		synthesizeRatpackRoutes(src, emit)
	}
}

// isGrailsControllerFile reports whether path/src looks like a Grails controller
// — under grails-app/controllers/ OR a file declaring a `class *Controller`.
func isGrailsControllerFile(slashed, src string) bool {
	if strings.Contains(slashed, "grails-app/controllers/") {
		return true
	}
	return grailsControllerNameRe.MatchString(src)
}

// synthesizeGrailsConventionRoutes emits `/<controller>/<action>` endpoints for
// each controller class + its action methods. Verb is ANY (Grails actions are
// method-agnostic by convention).
func synthesizeGrailsConventionRoutes(src string, emit emitFn) {
	m := grailsControllerNameRe.FindStringSubmatch(src)
	if m == nil {
		return
	}
	controller := grailsControllerSlug(m[1])
	if controller == "" {
		return
	}
	seen := map[string]bool{}
	for _, a := range grailsActionRe.FindAllStringSubmatch(src, -1) {
		action := a[1]
		if action == "" || grailsNonActionDef[action] {
			continue
		}
		if seen[action] {
			continue
		}
		seen[action] = true
		raw := "/" + controller + "/" + action
		canonical := httproutes.Canonicalize(httproutes.FrameworkGrails, raw)
		if canonical == "" {
			continue
		}
		// refKind=Controller, refName=action → same-file handler bridge binds to
		// the action method the Groovy base extractor emits.
		emit("ANY", canonical, "grails", "Controller", action)
	}
}

// synthesizeGrailsUrlMappings emits endpoints from explicit UrlMappings.groovy
// entries (`"/book/$id"(controller: "book", action: "show", method: "GET")`).
func synthesizeGrailsUrlMappings(src string, emit emitFn) {
	for _, m := range grailsUrlMappingRe.FindAllStringSubmatch(src, -1) {
		rawPath := m[1]
		cfg := m[2]
		// Drop interpolated paths.
		if strings.Contains(rawPath, "${") {
			continue
		}
		// Rewrite Grails dollar-params `$id` → `{id}` before canonicalisation.
		rewritten := grailsDollarParamsToCurly(rawPath)
		canonical := httproutes.Canonicalize(httproutes.FrameworkGrails, rewritten)
		if canonical == "" {
			continue
		}
		verb := "ANY"
		if mv := grailsMappingMethod(cfg); mv != "" {
			verb = mv
		}
		action := grailsMappingAction(cfg)
		refName := ""
		if action != "" {
			refName = action
		}
		emit(verb, canonical, "grails", "Controller", refName)
	}
}

// synthesizeRatpackRoutes emits endpoints for Ratpack verb-DSL handlers.
func synthesizeRatpackRoutes(src string, emit emitFn) {
	seen := map[string]bool{}
	add := func(verb, raw string) {
		raw = strings.TrimSpace(raw)
		if raw == "" || strings.Contains(raw, "${") {
			return
		}
		if !strings.HasPrefix(raw, "/") {
			raw = "/" + raw
		}
		canonical := httproutes.Canonicalize(httproutes.FrameworkRatpack, raw)
		if canonical == "" || canonical == "/" {
			return
		}
		key := verb + " " + canonical
		if seen[key] {
			return
		}
		seen[key] = true
		emit(verb, canonical, "ratpack", inlineHandlerRefKind, "")
	}
	for _, m := range ratpackRouteRe.FindAllStringSubmatch(src, -1) {
		add(strings.ToUpper(m[1]), m[2])
	}
	for _, m := range ratpackPathRe.FindAllStringSubmatch(src, -1) {
		add("ANY", m[1])
	}
}

// ratpackLikely is a fast pre-filter: the file must reference a Ratpack marker
// AND a verb-DSL call to be worth scanning, so we never misfire on arbitrary
// Groovy that happens to call a `get`/`post` method.
func ratpackLikely(src string) bool {
	if !strings.Contains(src, "ratpack") && !strings.Contains(src, "Ratpack") &&
		!strings.Contains(src, "Handlers") && !strings.Contains(src, "RatpackServer") &&
		!strings.Contains(src, "GroovyChainAction") {
		return false
	}
	return ratpackRouteRe.MatchString(src) || ratpackPathRe.MatchString(src)
}

// grailsControllerSlug turns `BookController` → `book`, `UserAccountController`
// → `userAccount` (Grails lower-cases the first letter of the de-suffixed name).
func grailsControllerSlug(className string) string {
	name := strings.TrimSuffix(className, "Controller")
	if name == "" {
		return ""
	}
	return strings.ToLower(name[:1]) + name[1:]
}

// grailsNonActionDef is the set of `def` names inside a Grails controller that
// are NOT request actions (lifecycle / interceptor hooks). Excluding them keeps
// phantom endpoints out of the catalogue.
var grailsNonActionDef = map[string]bool{
	"beforeInterceptor": true,
	"afterInterceptor":  true,
}

// grailsDollarParamsToCurly rewrites Grails dollar-prefixed path parameters
// (`/book/$id`, `/book/$author/$title`) to the canonical curly form
// (`/book/{id}`). A `$` not followed by an identifier is left intact.
func grailsDollarParamsToCurly(p string) string {
	return grailsDollarParamRe.ReplaceAllString(p, "{$1}")
}

var grailsDollarParamRe = regexp.MustCompile(`\$([a-zA-Z_]\w*)`)

// grailsMappingMethod extracts an explicit `method: "POST"` from a UrlMappings
// config fragment, upper-cased, or "" when absent.
func grailsMappingMethod(cfg string) string {
	if m := grailsMappingMethodRe.FindStringSubmatch(cfg); m != nil {
		return strings.ToUpper(m[1])
	}
	return ""
}

var grailsMappingMethodRe = regexp.MustCompile(`method\s*:\s*"([A-Za-z]+)"`)

// grailsMappingAction extracts `action: "show"` from a UrlMappings config
// fragment, or "" when absent (or a map-of-methods form we don't bind).
func grailsMappingAction(cfg string) string {
	if m := grailsMappingActionRe.FindStringSubmatch(cfg); m != nil {
		return m[1]
	}
	return ""
}

var grailsMappingActionRe = regexp.MustCompile(`action\s*:\s*"([a-zA-Z_]\w*)"`)
