// rate_limit_route.go — endpoint rate-limit / throttle stamping for Go gin/echo
// route ops (child of #3628; sibling of #3734's route_auth.go).
//
// A `rate_limit` SCOPE.Pattern node already records "this file uses a rate
// limiter" (internal/patterns/rate_limit_extractor.go). This resolver attributes
// the limiter to the SPECIFIC route op and resolves the numeric rate, stamping
// the SAME flat property contract the JS/TS pass
// (internal/engine/http_endpoint_jsts_ratelimit.go) and the Python pass
// (internal/custom/python/rate_limit_endpoint.go) use so the graph answers
// "which endpoints are throttled and at what rate?":
//
//	rate_limited      — "true" when a limiter applies to the endpoint.
//	rate_limit        — human rate "5/s" / "100/s" when statically resolvable;
//	                    OMITTED (honest-partial) when the limit is config-/env-/
//	                    dynamically driven and cannot be resolved without
//	                    fabrication.
//	rate_limit_scope  — "route" | "group" | "engine" — where the limiter applies.
//	rate_limit_source — the recognised limiter symbol / library (evidence).
//
// Recognised Go limiter surfaces, bound to a route op exactly as the auth
// resolver binds auth middleware (inline > group > engine-wide):
//
//	golang.org/x/time/rate — a `rate.NewLimiter(rate.Limit(10), 1)` binding (or
//	                         a tollbooth `tollbooth.NewLimiter(...).SetMax(100)` /
//	                         `tollbooth.NewLimiter(100, nil)` binding) is captured
//	                         so its resolved rate attaches wherever the limiter
//	                         is applied as middleware.
//	tollbooth              — `tollbooth.LimitHandler(limiter, h)` and the
//	                         `tollbooth.LimitFuncHandler` variant.
//	ulule/limiter          — `limiter.New(...)` / `mgin.NewMiddleware(...)` and a
//	                         throttle-named middleware applied to a route/group.
//
// A limiter middleware is recognised by name (tollbooth / Limit / RateLimit /
// throttle / x-time-rate-derived) the same way `classifyAuthMiddleware`
// recognises auth middleware. Precedence mirrors route_auth.go:
//
//	inline route middleware (HIGH) > the route's group var (HIGH) > engine-wide
//	`.Use(limiterMw)` (MEDIUM-equivalent "engine" scope) > none.
//
// Honest-partial: a limiter constructed dynamically (a rate read from config, a
// `rate.Limit` built from a non-literal expression) marks rate_limited=true with
// the rate omitted. A limiter binding that is NEVER applied to a route/group/
// engine is NOT stamped — the negative case the spec requires.
//
// Refs the #3628 rate-limit child ticket.
package golang

import (
	"regexp"
	"strconv"
	"strings"
)

// goRouteRateLimit is the resolved throttle posture for one route op.
type goRouteRateLimit struct {
	Rate   string // "5/s", "100/s", … or "" (honest-partial)
	Scope  string // "route" | "group" | "engine"
	Source string // recognised limiter symbol / library (evidence)
	found  bool
}

// stamp writes the resolved posture onto a route op's Properties map using the
// shared flat contract. No-op when no rate-limit signal was found (never
// fabricates rate_limited=false).
func (r goRouteRateLimit) stamp(props map[string]string) {
	if props == nil || !r.found {
		return
	}
	props["rate_limited"] = "true"
	if r.Scope != "" {
		props["rate_limit_scope"] = r.Scope
	}
	if r.Source != "" {
		props["rate_limit_source"] = r.Source
	}
	if r.Rate != "" {
		props["rate_limit"] = r.Rate
	}
}

// goRateLimitNeedles maps a lowercased substring of a middleware expression to a
// recognised limiter library/symbol. Ordered most-specific first. Covers the
// tollbooth, ulule/limiter, and golang.org/x/time/rate gin/echo middleware
// surface plus the conventional `RateLimit`/`Throttle` wrapper names.
var goRateLimitNeedles = []string{
	"tollbooth",
	"limithandler",
	"limitfunchandler",
	"ratelimiter",
	"ratelimit",
	"rate_limit",
	"throttle",
	"newmiddleware", // ulule mgin.NewMiddleware(rate)
	"limitmiddleware",
	"limitermiddleware",
}

// classifyRateLimitMiddleware reports whether a middleware expression looks like
// a rate limiter, returning the recognised evidence head symbol, or "" when it
// is not a limiter. The check is case-insensitive substring match against
// goRateLimitNeedles. A bare `limiter`/`limiterMw` identifier is recognised too
// (a common binding name) but ONLY when it is not also an auth middleware — auth
// is classified by its own resolver and must not be double-counted as a throttle.
func classifyRateLimitMiddleware(expr string) string {
	low := strings.ToLower(expr)
	for _, n := range goRateLimitNeedles {
		if strings.Contains(low, n) {
			return rateLimitHead(expr)
		}
	}
	// A bare `limiter`-named identifier (e.g. `limiterMw`, `apiLimiter`) is a
	// throttle by convention, but `rate.NewLimiter`/`tollbooth.NewLimiter` are
	// CONSTRUCTORS (not middleware) and are matched by the needles above when
	// applied, so a plain `*Limiter`/`limiter` middleware reference qualifies
	// only when it is not an auth guard.
	if reGoLimiterName.MatchString(low) && classifyAuthMiddleware(expr) == "" {
		return rateLimitHead(expr)
	}
	return ""
}

// reGoLimiterName recognises a limiter-conventional identifier as a whole token,
// e.g. `limiter`, `apiLimiter`, `limiterMiddleware` — used to catch an applied
// limiter binding whose constructor lives elsewhere (honest-partial rate).
var reGoLimiterName = regexp.MustCompile(`(?i)\blimiter\b|limiter[a-z]*\b`)

// rateLimitHead returns the leading call/selector head of a middleware
// expression as the evidence symbol (e.g. `tollbooth.LimitHandler` → that head).
func rateLimitHead(expr string) string {
	if head := reMiddlewareCallHead.FindString(strings.TrimSpace(expr)); head != "" {
		return head
	}
	return strings.TrimSpace(expr)
}

// reGoRateLimiterBinding captures a limiter constructor binding so its resolved
// rate can attach wherever the limiter is applied:
//
//	lim := rate.NewLimiter(rate.Limit(5), 1)
//	lim := tollbooth.NewLimiter(100, nil)
//
// Group 1 = binding identifier, group 2 = constructor selector, group 3 = the
// full argument list (balanced enough for the literal-rate cases above).
var reGoRateLimiterBinding = regexp.MustCompile(
	`(?m)(\w+)\s*:?=\s*([A-Za-z_][\w.]*)\s*\(([^\n\r]*)\)`)

// reGoRateLimitArg pulls a literal `rate.Limit(N)` / `rate.Every(...)`-free
// numeric rate out of a `rate.NewLimiter(...)` arg list. Group 1 = the limit.
var reGoRateLimitArg = regexp.MustCompile(`rate\.Limit\s*\(\s*([0-9]+(?:\.[0-9]+)?)\s*\)`)

// reGoTollboothSetMax captures a tollbooth `.SetMax(100)` chained call (or the
// positional `tollbooth.NewLimiter(100, nil)` first-arg form). Group 1 = max.
var (
	reGoTollboothSetMax  = regexp.MustCompile(`\.SetMax\s*\(\s*([0-9]+(?:\.[0-9]+)?)\s*\)`)
	reGoTollboothNewArg0 = regexp.MustCompile(`tollbooth\.NewLimiter\s*\(\s*([0-9]+(?:\.[0-9]+)?)`)
)

// goLimiterBinding is a resolved limiter constructor binding: its evidence
// source symbol and (when a literal) human-readable per-second rate.
type goLimiterBinding struct {
	source string
	rate   string // "5/s" or "" when unresolved (honest-partial)
}

// resolveGoLimiterRate turns a recognised limiter constructor invocation into a
// per-second human rate, or "" when the rate is not a static literal.
//
//	rate.NewLimiter(rate.Limit(5), 1)   → "5/s"
//	tollbooth.NewLimiter(100, nil)      → "100/s"
//	x.SetMax(100)                       → "100/s"
func resolveGoLimiterRate(selector, args string) string {
	sel := strings.ToLower(selector)
	// golang.org/x/time/rate: rate.NewLimiter(rate.Limit(N), burst).
	if strings.Contains(sel, "newlimiter") && strings.Contains(args, "rate.Limit") {
		if m := reGoRateLimitArg.FindStringSubmatch(args); m != nil {
			return formatGoRate(m[1])
		}
		return ""
	}
	// tollbooth.NewLimiter(max, nil) — first positional arg is the max/s.
	if strings.Contains(sel, "newlimiter") {
		if m := reGoTollboothNewArg0.FindStringSubmatch(selector + "(" + args + ")"); m != nil {
			return formatGoRate(m[1])
		}
	}
	return ""
}

// formatGoRate renders a numeric per-second limit as "N/s", dropping a trailing
// ".0" so an integral float (`5.0`) reads as "5/s".
func formatGoRate(num string) string {
	num = strings.TrimSpace(num)
	if f, err := strconv.ParseFloat(num, 64); err == nil {
		if f == float64(int64(f)) {
			return strconv.FormatInt(int64(f), 10) + "/s"
		}
		return strconv.FormatFloat(f, 'g', -1, 64) + "/s"
	}
	return ""
}

// goRouteRateLimitIndex holds the per-file resolved rate-limit signals: which
// group vars carry a limiter, which (verb,path) routes carry an inline limiter,
// the engine-wide limiter, and the resolved limiter-binding rates.
type goRouteRateLimitIndex struct {
	groupVars  map[string]goRouteRateLimit // group var → posture
	inline     map[string]goRouteRateLimit // "<VERB> <ownPath>" → posture
	engineWide goRouteRateLimit            // engine-level `.Use(limiterMw)`
	bindings   map[string]goLimiterBinding // limiter binding ident → resolved rate
}

// buildGoRouteRateLimitIndex scans a Go source file once and resolves every
// route/group/engine rate-limit signal. Framework-shared (gin and echo use the
// same .Group / .VERB / .Use shapes the auth/middleware indexes already parse).
func buildGoRouteRateLimitIndex(src string) goRouteRateLimitIndex {
	idx := goRouteRateLimitIndex{
		groupVars: map[string]goRouteRateLimit{},
		inline:    map[string]goRouteRateLimit{},
		bindings:  map[string]goLimiterBinding{},
	}

	// 1. Index limiter constructor bindings so an applied binding resolves to its
	//    rate. A `.SetMax(N)` chained later in the file refines a tollbooth
	//    binding's rate even when the constructor arg was non-literal.
	for _, m := range reGoRateLimiterBinding.FindAllStringSubmatchIndex(src, -1) {
		ident := src[m[2]:m[3]]
		selector := src[m[4]:m[5]]
		args := src[m[6]:m[7]]
		if classifyRateLimitMiddleware(selector) == "" && !strings.Contains(strings.ToLower(selector), "newlimiter") {
			continue
		}
		b := goLimiterBinding{source: rateLimitHead(selector), rate: resolveGoLimiterRate(selector, args)}
		idx.bindings[ident] = b
	}
	// tollbooth `.SetMax(N)` refinement: a `.SetMax(N)` call is the AUTHORITATIVE
	// limit (it overrides the constructor's placeholder first arg, which is often
	// a sentinel like `1`). Apply it to every tollbooth binding in file scope —
	// the common single-limiter case has one tollbooth limiter per file.
	if mm := reGoTollboothSetMax.FindStringSubmatch(src); mm != nil {
		if rate := formatGoRate(mm[1]); rate != "" {
			for ident, b := range idx.bindings {
				if strings.Contains(strings.ToLower(b.source), "tollbooth") {
					b.rate = rate
					idx.bindings[ident] = b
				}
			}
		}
	}

	// 2. Group-level: `g := r.Group("/x", limiterMw)` → g is throttled.
	for _, m := range reGoGroupDecl.FindAllStringSubmatchIndex(src, -1) {
		groupVar := src[m[2]:m[3]]
		args := src[m[6]:m[7]]
		if rl, ok := idx.rateLimitFromArgChain(args); ok {
			rl.Scope = goMWScopeGroup
			idx.groupVars[groupVar] = rl
		}
	}

	// 3. Inline route middleware: `r.GET("/me", limiterMw, h)` → that route.
	for _, m := range reGoRouteAuthScan.FindAllStringSubmatchIndex(src, -1) {
		verb := strings.ToUpper(src[m[4]:m[5]])
		path := src[m[6]:m[7]]
		args := src[m[8]:m[9]]
		if rl, ok := idx.rateLimitFromArgChain(args); ok {
			rl.Scope = goMWScopeRoute
			idx.inline[verb+" "+path] = rl
		}
	}

	// 4. Engine-wide `.Use(limiterMw)` → applies to every route ("engine" scope).
	for _, uc := range findUseCalls(src) {
		if rl, ok := idx.rateLimitFromArgChain(uc.Args); ok {
			rl.Scope = goMWScopeEngine
			idx.engineWide = rl
			break
		}
	}

	return idx
}

// rateLimitFromArgChain inspects a trailing-argument region (the text after a
// path literal, or a `.Use(...)` / `.Group(...)` arg list) for a recognised
// limiter middleware and returns the first match's posture, resolving its rate
// from an applied limiter binding when the arg is a known binding ident.
func (idx goRouteRateLimitIndex) rateLimitFromArgChain(args string) (goRouteRateLimit, bool) {
	args = strings.TrimSpace(strings.TrimPrefix(strings.TrimSpace(args), ","))
	if args == "" {
		return goRouteRateLimit{}, false
	}
	for _, part := range splitTopLevelArgs(args) {
		part = strings.TrimSpace(part)
		if part == "" || isStringLiteral(part) {
			continue
		}
		if rl, ok := idx.recognizeLimiterArg(part); ok {
			return rl, true
		}
	}
	return goRouteRateLimit{}, false
}

// recognizeLimiterArg resolves a single applied middleware argument to a
// rate-limit posture. A tollbooth/ulule middleware call resolves its rate from
// the limiter binding it wraps; a bare applied limiter binding resolves its rate
// from the binding index; an imported/dynamic limiter is honest-partial.
func (idx goRouteRateLimitIndex) recognizeLimiterArg(arg string) (goRouteRateLimit, bool) {
	raw := strings.TrimSpace(arg)
	if raw == "" {
		return goRouteRateLimit{}, false
	}
	// A bare applied limiter binding, e.g. `r.Use(limiterMw)` where
	// `limiterMw := rate.NewLimiter(...)`.
	if b, ok := idx.bindings[raw]; ok {
		return goRouteRateLimit{found: true, Source: b.source, Rate: b.rate}, true
	}
	source := classifyRateLimitMiddleware(raw)
	if source == "" {
		return goRouteRateLimit{}, false
	}
	rl := goRouteRateLimit{found: true, Source: source}
	// A wrapper that references a known binding, e.g.
	// `tollbooth.LimitHandler(lim, h)` / `mgin.NewMiddleware(lim)` — resolve the
	// wrapped binding's rate so the route carries the numeric rate.
	for _, inner := range reGoIdent.FindAllString(raw, -1) {
		if b, ok := idx.bindings[inner]; ok && b.rate != "" {
			rl.Rate = b.rate
			break
		}
	}
	return rl, true
}

// reGoIdent matches bare Go identifiers, used to find a limiter binding name
// referenced inside a middleware wrapper call.
var reGoIdent = regexp.MustCompile(`[A-Za-z_]\w*`)

// resolve returns the rate-limit posture for one route op, applying the same
// precedence as route_auth.go: inline route middleware (HIGH) > the route's
// group var (HIGH) > engine-wide `.Use` (engine scope) > none.
func (idx goRouteRateLimitIndex) resolve(routerVar, verb, ownPath string) goRouteRateLimit {
	if rl, ok := idx.inline[verb+" "+ownPath]; ok && rl.found {
		return rl
	}
	if rl, ok := idx.groupVars[routerVar]; ok && rl.found {
		return rl
	}
	if idx.engineWide.found {
		return idx.engineWide
	}
	return goRouteRateLimit{}
}
