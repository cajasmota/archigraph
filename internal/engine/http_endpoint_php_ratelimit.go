// http_endpoint_php_ratelimit.go — endpoint rate-limit / throttle stamping for
// the PHP backend-HTTP frameworks (child of #3628 → #4073, "[api] PHP endpoint
// rate-limit / throttle stamping"). Sibling of the Java pass
// (http_endpoint_java_ratelimit.go), the JS/TS pass
// (http_endpoint_jsts_ratelimit.go) and the Python pass
// (internal/custom/python/rate_limit_endpoint.go); stamps the SAME flat property
// contract on the producer-side http_endpoint_definition entity (no parallel
// node):
//
//	rate_limited      — "true" when a throttle applies to the endpoint.
//	rate_limit        — human rate "60/1min" when statically resolvable from a
//	                    `throttle:<max>,<minutes>` middleware argument; OMITTED
//	                    (honest-partial) for a NAMED limiter
//	                    (`throttle:api`) whose limit/window live in a
//	                    `RateLimiter::for('api', …)` registration (usually a
//	                    different file).
//	rate_limit_scope  — "route" (per-route `->middleware('throttle:…')`) |
//	                    "group" (a `Route::group(['middleware'=>['throttle:…']])`
//	                    wrapping the route).
//	rate_limit_source — the recognised middleware token (evidence):
//	                    "throttle:60,1" for a literal limiter, "throttle:api" for
//	                    a named one.
//
// Recognised Laravel surfaces:
//
//	per-route   — `Route::get('/x', …)->middleware('throttle:60,1')` and the
//	              array form `->middleware(['throttle:60,1','auth'])`. The
//	              throttle token is paired to the route by statement proximity
//	              (the chain sits between this `Route::<verb>(` and the next).
//	  group     — `Route::group(['middleware' => ['throttle:30,1']], fn)` /
//	              `'middleware' => 'throttle:30,1'`: every route whose byte
//	              offset falls inside the group body inherits the throttle at
//	              "group" scope. A per-route throttle takes precedence.
//
// `throttle:<max>,<minutes>` → rate "<max>/<minutes>min". A named limiter
// `throttle:<name>` (non-numeric first arg) is honest-partial: rate_limited +
// source resolve, the numeric rate is omitted.
//
// Like the other passes this adds NO entity — it mutates the Properties of the
// http_endpoint_definition entities synthesizeLaravel already emitted. `before`
// is the entity-slice length captured before the PHP synthesizers ran.
//
// Refs #3628, #4073.
package engine

import (
	"regexp"
	"strconv"
	"strings"

	"github.com/cajasmota/grafel/internal/engine/httproutes"
	"github.com/cajasmota/grafel/internal/types"
)

// phpRateLimit is a resolved throttle posture for one endpoint.
type phpRateLimit struct {
	rate   string // "60/1min", "30/1min", … or "" (honest-partial named limiter)
	scope  string // "route" | "group"
	source string // evidence middleware token, e.g. "throttle:60,1" / "throttle:api"
	found  bool
}

// stamp writes the resolved posture onto an endpoint Properties map using the
// shared flat contract. No-op when no throttle signal was recognised (never
// fabricates rate_limited=false).
func (r phpRateLimit) stamp(props map[string]string) {
	if props == nil || !r.found {
		return
	}
	props["rate_limited"] = "true"
	if r.scope != "" {
		props["rate_limit_scope"] = r.scope
	}
	if r.source != "" {
		props["rate_limit_source"] = r.source
	}
	if r.rate != "" {
		props["rate_limit"] = r.rate
	}
}

// lrThrottleTokenRe captures a Laravel `throttle:<args>` middleware token from a
// quoted string. Group 1 = the args after the colon ("60,1", "api", "10,1,key").
var lrThrottleTokenRe = regexp.MustCompile(`['"]throttle:([a-zA-Z0-9_,]+)['"]`)

// lrThrottleNumericArgsRe matches a literal limiter argument list: <max>,<minutes>
// (the optional 3rd `:key` segment is not part of the comma list). Group 1 =
// max, group 2 = minutes.
var lrThrottleNumericArgsRe = regexp.MustCompile(`^([0-9]+),([0-9]+)$`)

// resolveLaravelThrottleToken turns a `throttle:<args>` token's args into a
// posture. A numeric `<max>,<minutes>` resolves the rate; a named limiter (or a
// single-number / non-numeric form) is honest-partial (rate omitted).
func resolveLaravelThrottleToken(args string) phpRateLimit {
	rl := phpRateLimit{found: true, source: "throttle:" + args}
	if am := lrThrottleNumericArgsRe.FindStringSubmatch(args); am != nil {
		// max requests per <minutes> minute window. minutes==1 → "60/1min".
		max := am[1]
		mins := am[2]
		if n, err := strconv.Atoi(mins); err == nil && n > 0 {
			rl.rate = max + "/" + mins + "min"
		}
	}
	return rl
}

// lrRouteThrottleSite is a per-route throttle keyed by the route's canonical
// (verb, path) — the same key synthesizeLaravel stamps on the endpoint — plus
// the resolved posture. Path-keyed (not line-keyed) because the Laravel
// producer leaves StartLine=0 on its synthetics; this mirrors the Java pass.
type lrRouteThrottleSite struct {
	verb string
	path string // canonical path (group prefix applied, Express-canonicalized)
	rl   phpRateLimit
}

// indexLaravelRouteThrottles finds every `Route::<verb>(` registration that
// carries a `throttle:…` token in its method-chain. The chain is the byte span
// from this route match to just before the next route match (or EOF); the first
// throttle token found in that span is paired to the route. This binds
// `Route::get('/x', …)->middleware('throttle:60,1')` and the array form
// `->middleware(['throttle:60,1','auth'])` while ignoring throttles that belong
// to a later route. The route is keyed by the SAME canonical (verb, path)
// synthesizeLaravel emits (group prefix applied), so the apply pass matches the
// producer endpoint exactly.
func indexLaravelRouteThrottles(content string, groupSpans []lrGroupSpan) []lrRouteThrottleSite {
	verbMatches := lrRouteVerbRe.FindAllStringSubmatchIndex(content, -1)
	if len(verbMatches) == 0 {
		return nil
	}
	var out []lrRouteThrottleSite
	for i, m := range verbMatches {
		if len(m) < 8 {
			continue
		}
		// Bound the route's statement at the next route match OR the first `;`
		// terminator, whichever comes first, so a `throttle:` token that belongs
		// to a LATER statement (e.g. a following Route::group middleware) is not
		// mis-paired to this route. The `->middleware('throttle:…')` chain of a
		// fluent route registration sits before that route's own `;`.
		spanEnd := len(content)
		if i+1 < len(verbMatches) {
			spanEnd = verbMatches[i+1][0]
		}
		if semi := strings.IndexByte(content[m[0]:spanEnd], ';'); semi >= 0 {
			spanEnd = m[0] + semi
		}
		span := content[m[0]:spanEnd]
		tm := lrThrottleTokenRe.FindStringSubmatch(span)
		if tm == nil {
			continue
		}
		verb := strings.ToUpper(content[m[2]:m[3]])
		raw := ""
		if m[4] >= 0 {
			raw = content[m[4]:m[5]]
		} else if m[6] >= 0 {
			raw = content[m[6]:m[7]]
		}
		if raw == "" {
			continue
		}
		// Apply group prefix the SAME way synthesizeLaravel does (left-to-right
		// over containing group spans).
		prefix := ""
		for _, gs := range groupSpans {
			if m[0] >= gs.bodyStart && m[0] < gs.bodyEnd {
				prefix = prefix + "/" + strings.Trim(gs.prefix, "/")
			}
		}
		if prefix != "" {
			raw = prefix + "/" + strings.TrimLeft(raw, "/")
		}
		canonical := httproutes.Canonicalize(httproutes.FrameworkExpress, raw)
		rl := resolveLaravelThrottleToken(tm[1])
		rl.scope = "route"
		out = append(out, lrRouteThrottleSite{verb: verb, path: canonical, rl: rl})
	}
	return out
}

// indexLaravelGroupThrottles scans for Route::group(['middleware' => …]) calls
// whose middleware carries a `throttle:…` token, then enumerates the
// `Route::<verb>(` registrations nested INSIDE each such group body and returns
// them as path-keyed sites at "group" scope. Enumerating inner routes by path
// (rather than recording a byte span) keeps binding robust even though the
// Laravel producer leaves StartLine=0 on its synthetics, and naturally handles
// prefix-less groups. groupSpans is the full set of prefix group spans (from
// synthesizeLaravel's helper) used to canonicalize each inner route's path the
// same way the producer does.
func indexLaravelGroupThrottles(content string, groupSpans []lrGroupSpan) []lrRouteThrottleSite {
	var out []lrRouteThrottleSite
	for _, m := range lrRouteGroupMwRe.FindAllStringSubmatchIndex(content, -1) {
		// lrRouteGroupMwRe groups: 1=single-quoted, 2=double-quoted, 3=first
		// array element. A group can carry a list `['auth','throttle:30,1']`
		// where a non-first element is the throttle, so re-scan a bounded window
		// of the options array for any throttle token.
		winEnd := m[1] + 400
		if winEnd > len(content) {
			winEnd = len(content)
		}
		tok := lrThrottleTokenRe.FindStringSubmatch(content[m[0]:winEnd])
		if tok == nil {
			continue
		}
		rl := resolveLaravelThrottleToken(tok[1])
		rl.scope = "group"

		// Locate the group callback body span (the `{ … }` after the options).
		bodyOpen := -1
		for i := m[1]; i < len(content) && i < m[1]+1000; i++ {
			if content[i] == '{' {
				bodyOpen = i
				break
			}
		}
		if bodyOpen < 0 {
			continue
		}
		bodyEnd := lrFindMatchingBrace(content, bodyOpen)
		if bodyEnd < 0 {
			continue
		}

		// Enumerate inner Route::<verb>( routes and key them by canonical path.
		for _, vm := range lrRouteVerbRe.FindAllStringSubmatchIndex(content, -1) {
			if len(vm) < 8 || vm[0] < bodyOpen+1 || vm[0] >= bodyEnd {
				continue
			}
			verb := strings.ToUpper(content[vm[2]:vm[3]])
			raw := ""
			if vm[4] >= 0 {
				raw = content[vm[4]:vm[5]]
			} else if vm[6] >= 0 {
				raw = content[vm[6]:vm[7]]
			}
			if raw == "" {
				continue
			}
			prefix := ""
			for _, gs := range groupSpans {
				if vm[0] >= gs.bodyStart && vm[0] < gs.bodyEnd {
					prefix = prefix + "/" + strings.Trim(gs.prefix, "/")
				}
			}
			if prefix != "" {
				raw = prefix + "/" + strings.TrimLeft(raw, "/")
			}
			canonical := httproutes.Canonicalize(httproutes.FrameworkExpress, raw)
			out = append(out, lrRouteThrottleSite{verb: verb, path: canonical, rl: rl})
		}
	}
	return out
}

// applyLaravelRateLimit resolves and stamps the flat rate-limit contract on
// every Laravel synthetic backend endpoint synthesizeLaravel emitted. It mutates
// Properties in place and never adds or removes entities. `before` is the
// entity-slice length captured before the PHP synthesizers ran.
//
// Resolution order per endpoint: a per-route `->middleware('throttle:…')`
// (strongest, "route" scope) wins; otherwise a Route::group throttle whose body
// span contains the route's source line applies at "group" scope.
func applyLaravelRateLimit(content, path string, entities []types.EntityRecord, before int) {
	if len(content) == 0 || before >= len(entities) {
		return
	}
	groupSpans := lrExtractGroupSpans(content)
	routeSites := indexLaravelRouteThrottles(content, groupSpans)
	groupSites := indexLaravelGroupThrottles(content, groupSpans)
	if len(routeSites) == 0 && len(groupSites) == 0 {
		return
	}
	// Per-route sites take precedence over group sites for the same key, so
	// index groups first then overlay routes.
	byKey := make(map[string]phpRateLimit, len(routeSites)+len(groupSites))
	for _, s := range groupSites {
		byKey[s.verb+" "+s.path] = s.rl
	}
	for _, s := range routeSites {
		byKey[s.verb+" "+s.path] = s.rl
	}

	for i := before; i < len(entities); i++ {
		e := &entities[i]
		if e.Kind != httpEndpointDefinitionKind || e.SourceFile != path || e.Properties == nil {
			continue
		}
		// Only stamp endpoints the Laravel route synthesizer produced.
		if e.Properties["framework"] != "laravel" {
			continue
		}
		key := e.Properties["verb"] + " " + e.Properties["path"]
		if rl, ok := byKey[key]; ok {
			rl.stamp(e.Properties)
		}
	}
}
