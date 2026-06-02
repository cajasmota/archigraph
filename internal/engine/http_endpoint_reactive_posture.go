// Lightweight / reactive JVM-framework endpoint posture stamping
// (#3858, epic #3854).
//
// Spring was built first, so the language-agnostic posture passes
// (response_codes — see http_endpoint_response_codes.go) only recognised SPRING
// idioms in their `java` branch (ResponseEntity / @ResponseStatus /
// ResponseStatusException). #3857 added the JAX-RS / Jakarta-family + Micronaut
// resolvers (jaxrsResponseCodes). This file completes the JVM REST surface by
// adding the LIGHTWEIGHT / REACTIVE family that still uses a DIFFERENT
// response API again:
//
//   - Javalin       — `ctx.status(404)` / `ctx.status(HttpStatus.NOT_FOUND)`;
//     `@OpenApi(responses = {@OpenApiResponse(status = "404")})`.
//   - Vert.x Web    — `rc.response().setStatusCode(404)` / `.setStatusCode(201)`.
//   - Akka-HTTP     — `complete(StatusCodes.NotFound)` /
//     `complete(StatusCodes.CREATED)` / `complete((404, ...))`.
//   - Struts 2      — `@Action` result codes mapped to HTTP status (SUCCESS->200,
//     ERROR->500, NONE->204, redirect results -> 302).
//   - Spring WebFlux — `ServerResponse.status(404)` / `.notFound()` / `.ok()` /
//     `.created(uri)` / `.badRequest()` / `.unprocessableEntity()`.
//
// All of these are merged into the SAME flat property contract Spring + JAX-RS
// use (response_codes / success_code / response_codes_source) — no new
// properties, no new entities. The resolvers are appended to the existing `java`
// branch of resolveEndpointResponseCodes (see http_endpoint_response_codes.go),
// alongside javaResponseCodes (Spring) and jaxrsResponseCodes (JAX-RS).
//
// A small deprecation helper (reactiveDeprecationVerdict) recognises the
// Javalin `@OpenApi(deprecated = true)` annotation flag, merged into the `java`
// branch of resolveEndpointDeprecation. (The cross-language `@Deprecated`
// annotation, the path-derived api_version, and the param-shape pagination
// fallback already cover the rest of this family.)
//
// HONEST-PARTIAL: a dynamic status variable (`ctx.status(code)`,
// `setStatusCode(s)`) yields no literal and is skipped; an Akka
// `complete(someEntity)` with no StatusCodes / numeric literal carries no code;
// a Struts result whose name is not a recognised built-in is omitted.
//
// Refs #3858.
package engine

import (
	"regexp"
	"strconv"
	"strings"
)

// ---------------------------------------------------------------------------
// Response codes — Javalin
// ---------------------------------------------------------------------------

// javalinCtxStatusNumericRe matches `ctx.status(404)` / `context.status(201)` —
// a status(...) call on a Javalin Context whose argument is a 3-digit literal. A
// non-literal argument (a variable) does not match — honest-partial.
var javalinCtxStatusNumericRe = regexp.MustCompile(`\b(?:ctx|context)\s*\.\s*status\s*\(\s*(\d{3})\s*\)`)

// javalinCtxStatusEnumRe matches `ctx.status(HttpStatus.NOT_FOUND)` /
// `ctx.status(HttpStatus.CREATED)` — Javalin's io.javalin.http.HttpStatus enum
// (whose constant names match the shared JAX-RS / Spring status vocabulary).
var javalinCtxStatusEnumRe = regexp.MustCompile(`\b(?:ctx|context)\s*\.\s*status\s*\(\s*HttpStatus\s*\.\s*([A-Z][A-Z0-9_]+)\s*\)`)

// javalinOpenAPIResponseStatusRe matches the Javalin `@OpenApi`
// `@OpenApiResponse(status = "404")` declaration — the documented status string.
var javalinOpenAPIResponseStatusRe = regexp.MustCompile(`@OpenApiResponse\s*\(\s*status\s*=\s*"(\d{3})"`)

// javalinResponseCodes resolves the status set for a Javalin endpoint from the
// annotation region (@OpenApi) + handler body (ctx.status(...)).
func javalinResponseCodes(region, body string) responseCodesVerdict {
	var v responseCodesVerdict

	// @OpenApi(responses = {@OpenApiResponse(status = "404")}) in the region.
	for _, m := range javalinOpenAPIResponseStatusRe.FindAllStringSubmatch(region, -1) {
		if c, err := strconv.Atoi(m[1]); err == nil {
			v.add(c)
			if v.source == "" {
				v.source = "@OpenApiResponse"
			}
		}
	}

	// ctx.status(NNN) in the body.
	for _, m := range javalinCtxStatusNumericRe.FindAllStringSubmatch(body, -1) {
		if c, err := strconv.Atoi(m[1]); err == nil {
			v.add(c)
			if v.source == "" {
				v.source = "ctx.status()"
			}
		}
	}
	// ctx.status(HttpStatus.X) in the body — shared status enum vocabulary.
	for _, m := range javalinCtxStatusEnumRe.FindAllStringSubmatch(body, -1) {
		if c, ok := jaxrsStatusEnumCode(m[1]); ok {
			v.add(c)
			if v.source == "" {
				v.source = "ctx.status()"
			}
		}
	}

	return v
}

// ---------------------------------------------------------------------------
// Response codes — Vert.x Web
// ---------------------------------------------------------------------------

// vertxSetStatusCodeRe matches `rc.response().setStatusCode(404)` /
// `ctx.response().setStatusCode(201)` — a setStatusCode(...) call whose argument
// is a 3-digit literal. The receiver is any name (the routing context); we
// anchor on the well-known method. A non-literal argument is skipped.
var vertxSetStatusCodeRe = regexp.MustCompile(`\.\s*setStatusCode\s*\(\s*(\d{3})\s*\)`)

// vertxEndStatusRe matches the convenience `.setStatusCode(201).end()` chain is
// already covered by vertxSetStatusCodeRe; no extra helper needed.

// vertxResponseCodes resolves the status set for a Vert.x Web endpoint from the
// handler body (setStatusCode(NNN)).
func vertxResponseCodes(body string) responseCodesVerdict {
	var v responseCodesVerdict
	for _, m := range vertxSetStatusCodeRe.FindAllStringSubmatch(body, -1) {
		if c, err := strconv.Atoi(m[1]); err == nil {
			v.add(c)
			if v.source == "" {
				v.source = "setStatusCode()"
			}
		}
	}
	return v
}

// ---------------------------------------------------------------------------
// Response codes — Akka-HTTP (Java + Scala DSL)
// ---------------------------------------------------------------------------

// akkaCompleteStatusEnumRe matches `complete(StatusCodes.NotFound)` /
// `complete(StatusCodes.Created)` — the Akka-HTTP StatusCodes object (whose
// constant names are CamelCase, e.g. NotFound / Created / NoContent). Group 1 is
// the CamelCase constant name.
var akkaCompleteStatusEnumRe = regexp.MustCompile(`\bStatusCodes\s*\.\s*([A-Z][A-Za-z0-9_]+)`)

// akkaCompleteNumericRe matches `complete((404, ...))` / `complete(404, ...)` /
// `complete(StatusCode.int2StatusCode(201))` — a 3-digit literal supplied to a
// complete(...) directive. We require the literal to follow `complete(` (with an
// optional opening paren for the tuple form) so a bare number elsewhere in the
// body is not mistaken for a status.
var akkaCompleteNumericRe = regexp.MustCompile(`\bcomplete\s*\(\s*\(?\s*(\d{3})\b`)

// akkaStatusCodesName maps the Akka-HTTP StatusCodes CamelCase constant names to
// their numeric codes (akka.http.javadsl.model.StatusCodes /
// akka.http.scaladsl.model.StatusCodes). Covers the codes endpoints return.
var akkaStatusCodesName = map[string]int{
	"OK":                   200,
	"Created":              201,
	"Accepted":             202,
	"NoContent":            204,
	"MovedPermanently":     301,
	"Found":                302,
	"SeeOther":             303,
	"NotModified":          304,
	"TemporaryRedirect":    307,
	"PermanentRedirect":    308,
	"BadRequest":           400,
	"Unauthorized":         401,
	"PaymentRequired":      402,
	"Forbidden":            403,
	"NotFound":             404,
	"MethodNotAllowed":     405,
	"NotAcceptable":        406,
	"RequestTimeout":       408,
	"Conflict":             409,
	"Gone":                 410,
	"PreconditionFailed":   412,
	"UnsupportedMediaType": 415,
	"UnprocessableEntity":  422,
	"TooManyRequests":      429,
	"InternalServerError":  500,
	"NotImplemented":       501,
	"BadGateway":           502,
	"ServiceUnavailable":   503,
	"GatewayTimeout":       504,
}

// akkaResponseCodes resolves the status set for an Akka-HTTP endpoint from the
// handler body (complete(StatusCodes.X) / complete((NNN, ...))).
func akkaResponseCodes(body string) responseCodesVerdict {
	var v responseCodesVerdict
	for _, m := range akkaCompleteStatusEnumRe.FindAllStringSubmatch(body, -1) {
		if c, ok := akkaStatusCodesName[m[1]]; ok {
			v.add(c)
			if v.source == "" {
				v.source = "complete(StatusCodes)"
			}
		}
	}
	for _, m := range akkaCompleteNumericRe.FindAllStringSubmatch(body, -1) {
		if c, err := strconv.Atoi(m[1]); err == nil {
			v.add(c)
			if v.source == "" {
				v.source = "complete(status)"
			}
		}
	}
	return v
}

// ---------------------------------------------------------------------------
// Response codes — Struts 2 (@Action result codes)
// ---------------------------------------------------------------------------

// strutsResultNameRe matches a Struts `@Result(name = "success", ...)` result
// declaration (inside an @Action results = { ... } list or a standalone
// @Result). Group 1 is the lower-cased result name. The shorthand
// `@Action(value="/x", results = {@Result(name="error", type="redirect")})` is
// covered because we scan the whole annotation region.
var strutsResultNameRe = regexp.MustCompile(`@Result\s*\(\s*(?:value\s*=\s*"[^"]*"\s*,\s*)?name\s*=\s*"([a-zA-Z]+)"`)

// strutsHTTPHeaderResultRe matches the Struts `httpheader` result with an
// explicit status, declared as `<param name="status">404</param>` (struts.xml)
// or `@Result(type="httpheader", params={"status","404"})`. Group 1 is the code.
var strutsHTTPHeaderResultRe = regexp.MustCompile(`(?i)status\s*["'>=,)\s]+\s*(\d{3})\b`)

// strutsResultCodes maps the Struts 2 built-in result NAMES to the HTTP status
// each conventionally produces (com.opensymphony.xwork2.Action constants).
// SUCCESS / INPUT render a view (200); ERROR is a server error (500); NONE
// returns no result (204); LOGIN is an auth challenge (401).
var strutsResultCodes = map[string]int{
	"success": 200,
	"input":   200,
	"none":    204,
	"error":   500,
	"login":   401,
}

// strutsResponseCodes resolves the status set for a Struts 2 action from its
// annotation region + handler body. The @Action(results = { @Result(...) })
// list is part of the route-declaring annotation: for the convention plugin the
// route is anchored on the @Action line, so the nested @Result entries (the
// multi-line results array) fall into the forward body window rather than the
// upward decorator region — scanning both covers every layout.
func strutsResponseCodes(scope string) responseCodesVerdict {
	var v responseCodesVerdict
	for _, m := range strutsResultNameRe.FindAllStringSubmatch(scope, -1) {
		if c, ok := strutsResultCodes[strings.ToLower(m[1])]; ok {
			v.add(c)
			if v.source == "" {
				v.source = "@Result name"
			}
		}
	}
	// Explicit httpheader status param (e.g. a 404/410 result).
	for _, m := range strutsHTTPHeaderResultRe.FindAllStringSubmatch(scope, -1) {
		if c, err := strconv.Atoi(m[1]); err == nil {
			v.add(c)
			if v.source == "" {
				v.source = "httpheader status"
			}
		}
	}
	return v
}

// ---------------------------------------------------------------------------
// Response codes — Spring WebFlux (ServerResponse builders)
// ---------------------------------------------------------------------------

// webfluxServerResponseStatusRe matches `ServerResponse.status(404)` /
// `ServerResponse.status(HttpStatus.CREATED)` — the functional reactive builder.
// Group 1 = enum name (when present); group 2 = numeric code (when present).
var webfluxServerResponseStatusRe = regexp.MustCompile(`ServerResponse\s*\.\s*status\s*\(\s*(?:HttpStatus\s*\.\s*([A-Z_0-9]+)|(\d{3}))`)

// webfluxServerResponseFactoryRe matches the ServerResponse factory helpers that
// imply a fixed status: ok()->200, created(uri)->201, accepted()->202,
// noContent()->204, notFound()->404, badRequest()->400,
// unprocessableEntity()->422. Anchored on `ServerResponse.` so a same-named
// helper on another type does not match.
var webfluxServerResponseFactoryRe = regexp.MustCompile(`ServerResponse\s*\.\s*(ok|created|accepted|noContent|notFound|badRequest|unprocessableEntity)\b`)

// webfluxServerResponseFactoryCodes maps a ServerResponse factory to its code.
var webfluxServerResponseFactoryCodes = map[string]int{
	"ok":                  200,
	"created":             201,
	"accepted":            202,
	"noContent":           204,
	"badRequest":          400,
	"notFound":            404,
	"unprocessableEntity": 422,
}

// webfluxResponseCodes resolves the status set for a Spring WebFlux functional
// endpoint from the handler body (ServerResponse.status(...) / factory helpers).
func webfluxResponseCodes(body string) responseCodesVerdict {
	var v responseCodesVerdict

	for _, m := range webfluxServerResponseStatusRe.FindAllStringSubmatch(body, -1) {
		if m[1] != "" {
			if c, ok := springHTTPStatusCode(m[1]); ok {
				v.add(c)
			}
		} else if m[2] != "" {
			if c, err := strconv.Atoi(m[2]); err == nil {
				v.add(c)
			}
		}
		if v.source == "" {
			v.source = "ServerResponse.status"
		}
	}

	for _, m := range webfluxServerResponseFactoryRe.FindAllStringSubmatch(body, -1) {
		if c, ok := webfluxServerResponseFactoryCodes[m[1]]; ok {
			v.add(c)
			if v.source == "" {
				v.source = "ServerResponse." + m[1] + "()"
			}
		}
	}

	return v
}

// ---------------------------------------------------------------------------
// Aggregate resolver — merged into the `java` branch of
// resolveEndpointResponseCodes (http_endpoint_response_codes.go).
// ---------------------------------------------------------------------------

// reactiveResponseCodes resolves the status-code set for a lightweight / reactive
// JVM-framework endpoint (Javalin / Vert.x / Akka-HTTP / Struts / Spring WebFlux)
// from the annotation/decorator region + handler body. The five framework
// shapes are mutually exclusive in practice (a file is Javalin OR Vert.x OR …),
// so merging all resolvers cannot double-count across frameworks; within one
// framework the per-signal de-dup (responseCodesVerdict.add) collapses repeats.
func reactiveResponseCodes(region, body string) responseCodesVerdict {
	var v responseCodesVerdict
	v.merge(javalinResponseCodes(region, body))
	v.merge(vertxResponseCodes(body))
	v.merge(akkaResponseCodes(body))
	v.merge(strutsResponseCodes(region + "\n" + body))
	v.merge(webfluxResponseCodes(body))
	return v
}

// ---------------------------------------------------------------------------
// Deprecation — Javalin @OpenApi(deprecated = true)
// ---------------------------------------------------------------------------

// javaOpenAPIDeprecatedRe matches the Javalin `@OpenApi(deprecated = true)` flag
// (and the generic OpenAPI `deprecated: true` / `deprecated=true` route flag in
// a JVM annotation region). Mirrors the JS-branch jsOpenAPIDeprecatedRe so the
// same documented-deprecation semantic is recognised on the JVM.
var javaOpenAPIDeprecatedRe = regexp.MustCompile(`(?i)\bdeprecated\s*[:=]\s*true\b`)

// reactiveDeprecationVerdict recognises the Javalin @OpenApi(deprecated = true)
// documented-deprecation flag in the annotation region. The cross-language
// @Deprecated annotation is already handled by javaDeprecationVerdict, which
// runs first in the `java` branch; this fires only when no @Deprecated is
// present but an @OpenApi(deprecated = true) flag is.
func reactiveDeprecationVerdict(region string) (deprecationVerdict, bool) {
	if javaOpenAPIDeprecatedRe.MatchString(region) {
		return deprecationVerdict{deprecated: true, source: "@OpenApi(deprecated)"}, true
	}
	return deprecationVerdict{}, false
}
