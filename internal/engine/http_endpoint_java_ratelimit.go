// http_endpoint_java_ratelimit.go — endpoint rate-limit / throttle stamping for
// the Java backend-HTTP frameworks (child of #3628, "[api] endpoint rate-limit /
// throttle stamping"). Sibling of the JS/TS pass
// (http_endpoint_jsts_ratelimit.go) and the Python pass
// (internal/custom/python/rate_limit_endpoint.go); stamps the SAME flat property
// contract on the endpoint op (no parallel node):
//
//	rate_limited      — "true" when a throttle applies to the endpoint.
//	rate_limit        — human rate "10/s" / "100/m" when statically resolvable
//	                    (Spring Cloud Gateway replenishRate, bucket4j literal
//	                    capacity); OMITTED (honest-partial) when the rate lives in
//	                    config (a Resilience4j `@RateLimiter(name="x")` whose
//	                    `limitForPeriod` is in application.yml).
//	rate_limit_scope  — "route" (method-level annotation) | "gateway"
//	                    (Spring Cloud Gateway route filter).
//	rate_limit_source — the recognised annotation / filter symbol (evidence).
//
// Recognised Java surfaces:
//
//	Resilience4j  — `@RateLimiter(name="api")` on a `@GetMapping`/`@PostMapping`/
//	                `@RequestMapping`/`@Path`-mapped method → rate_limited=true.
//	                The numeric limit lives in `resilience4j.ratelimiter.*`
//	                config, so the rate is honest-partial (omitted) unless an
//	                inline `limitForPeriod`/`limitRefreshPeriod` is present.
//	bucket4j      — `@RateLimiting(...)` / `@RateLimit(...)` method annotations
//	                (bucket4j-spring-boot-starter); a literal `capacity`/`rate`
//	                attribute resolves the rate, else honest-partial.
//	Spring Cloud  — a `RequestRateLimiter` GatewayFilter with `replenishRate` /
//	  Gateway       `burstCapacity` args, matched to the route's `Path=` predicate
//	                → rate="<replenishRate>/s" at gateway scope.
//
// Like the other passes this adds NO entity — it mutates the Properties of the
// http_endpoint_definition entities this file already emitted. `before` is the
// entity-slice length captured before the Java synthesizers ran (the same window
// applyJavaMiddlewareCoverage uses).
//
// Refs the #3628 rate-limit child ticket.
package engine

import (
	"regexp"
	"strconv"
	"strings"

	"github.com/cajasmota/archigraph/internal/types"
)

// javaRateLimit is a resolved throttle posture for one endpoint.
type javaRateLimit struct {
	rate   string // "10/s", "100/m", … or "" (honest-partial)
	scope  string // "route" | "gateway"
	source string // evidence annotation / filter symbol
	found  bool
}

// stamp writes the resolved posture onto an endpoint Properties map using the
// shared flat contract. No-op when no throttle signal was recognised.
func (r javaRateLimit) stamp(props map[string]string) {
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

// javaRateLimitAnnoRe captures a Resilience4j `@RateLimiter(name="x")` or a
// bucket4j `@RateLimiting(...)`/`@RateLimit(...)` method annotation. Group 1 =
// the annotation simple name, group 2 = its argument body (may be empty).
var javaRateLimitAnnoRe = regexp.MustCompile(
	`@(RateLimiter|RateLimiting|RateLimit)\s*(?:\(([^)]*)\))?`)

// javaMethodMappingRe captures a Spring/JAX-RS route-mapping annotation with its
// path literal. Group 1 = the mapping annotation, group 2 = the path. A
// verb-only mapping with no path (`@GetMapping`) is matched with an empty path.
var javaMethodMappingRe = regexp.MustCompile(
	`@(GetMapping|PostMapping|PutMapping|DeleteMapping|PatchMapping|RequestMapping|Path)\s*\(\s*(?:(?:value|path)\s*=\s*)?"([^"]*)"`)

// javaBucket4jCapacityRe / javaResilience4jPeriodRe pull a literal limit out of a
// bucket4j / inline-Resilience4j annotation body, when present.
var (
	javaBucket4jCapacityRe = regexp.MustCompile(`\b(?:capacity|rate|limit|limitForPeriod)\s*=\s*"?([0-9]+)"?`)
	javaRateLimitRefreshRe = regexp.MustCompile(`\blimitRefreshPeriod\s*=\s*"?(\d+)\s*([smhd])?"?`)
	javaGatewayReplenishRe = regexp.MustCompile(`replenishRate\s*[=:]\s*"?([0-9]+)"?`)
	// javaGatewayPathPredRe matches a Spring Cloud Gateway Path predicate in both
	// the YAML shortcut form (`Path=/api/**`, unquoted) and the Java DSL form
	// (`.path("/api/**")`, quoted). Group 1 = the quoted path, group 2 = the
	// unquoted YAML path; the caller takes whichever is non-empty.
	javaGatewayPathPredRe   = regexp.MustCompile(`\.path\s*\(\s*"([^"]+)"|Path\s*=\s*([^\s,"'\]]+)`)
	javaGatewayRateFilterRe = regexp.MustCompile(`RequestRateLimiter`)
)

// javaRateLimitedMethod is a method-level annotation pairing: the route path the
// method is mapped to (suffix, may be ""), the resolved throttle posture, and
// the byte span used for proximity pairing.
type javaRateLimitedMethod struct {
	path  string
	rl    javaRateLimit
	start int
}

// indexJavaRateLimitMethods scans the file for methods carrying a rate-limit
// annotation co-located with a route-mapping annotation, keyed by the mapping
// path. A `@RateLimiter` with no co-located mapping is recorded with path "" so
// it can fall back to class-level matching by the controller's base path.
func indexJavaRateLimitMethods(content string) []javaRateLimitedMethod {
	var out []javaRateLimitedMethod
	for _, m := range javaRateLimitAnnoRe.FindAllStringSubmatchIndex(content, -1) {
		anno := content[m[2]:m[3]]
		var body string
		if m[4] >= 0 {
			body = content[m[4]:m[5]]
		}
		rl := resolveJavaRateLimitAnno(anno, body)
		// Pair with the nearest route-mapping annotation in the same method
		// annotation block: scan a forward window from the throttle annotation
		// for the next mapping annotation (within ~600 chars / one method head).
		win := m[1]
		end := win + 600
		if end > len(content) {
			end = len(content)
		}
		path := ""
		if mm := javaMethodMappingRe.FindStringSubmatch(content[m[0]:end]); mm != nil {
			path = mm[2]
		}
		rl.scope = "route"
		out = append(out, javaRateLimitedMethod{path: path, rl: rl, start: m[0]})
	}
	return out
}

// resolveJavaRateLimitAnno turns a recognised rate-limit annotation into a
// posture, resolving a literal rate from a bucket4j capacity / inline
// Resilience4j limitForPeriod when present (else honest-partial).
func resolveJavaRateLimitAnno(anno, body string) javaRateLimit {
	rl := javaRateLimit{found: true, source: "@" + anno}
	if body == "" {
		return rl
	}
	if cm := javaBucket4jCapacityRe.FindStringSubmatch(body); cm != nil {
		n := cm[1]
		// A `limitRefreshPeriod = N <unit>` refines the window; default to /s.
		unit := "s"
		if rm := javaRateLimitRefreshRe.FindStringSubmatch(body); rm != nil && rm[2] != "" {
			unit = rm[2]
		}
		rl.rate = n + "/" + unit
	}
	return rl
}

// javaGatewayRoute is a Spring Cloud Gateway route with a RequestRateLimiter
// filter: the path predicate it matches and the resolved replenish rate.
type javaGatewayRoute struct {
	path string
	rate string // "10/s" or "" (honest-partial)
}

// indexJavaGatewayRateLimiters scans a Spring Cloud Gateway route definition
// (Java DSL `route(r -> r.path("/api/**").filters(f ->
// f.requestRateLimiter(...))...)` or application.yml-style
// `RequestRateLimiter` + `replenishRate` + `Path=`) and returns the
// rate-limited routes. Honest-partial: a RequestRateLimiter whose replenishRate
// is config-driven leaves the rate empty.
func indexJavaGatewayRateLimiters(content string) []javaGatewayRoute {
	if !javaGatewayRateFilterRe.MatchString(content) {
		return nil
	}
	var out []javaGatewayRoute
	// YAML-style: a route block with a Path= predicate and a RequestRateLimiter
	// filter; pair each Path= with the nearest replenishRate in scope.
	for _, pm := range javaGatewayPathPredRe.FindAllStringSubmatchIndex(content, -1) {
		// Group 1 = quoted DSL path, group 2 = unquoted YAML path.
		path := ""
		if pm[2] >= 0 {
			path = content[pm[2]:pm[3]]
		} else if pm[4] >= 0 {
			path = content[pm[4]:pm[5]]
		}
		if path == "" {
			continue
		}
		// Look in a window around this predicate for replenishRate.
		start := pm[0] - 400
		if start < 0 {
			start = 0
		}
		end := pm[1] + 400
		if end > len(content) {
			end = len(content)
		}
		win := content[start:end]
		if !strings.Contains(win, "RequestRateLimiter") {
			continue
		}
		gr := javaGatewayRoute{path: path}
		if rm := javaGatewayReplenishRe.FindStringSubmatch(win); rm != nil {
			if n, err := strconv.Atoi(rm[1]); err == nil && n > 0 {
				gr.rate = strconv.Itoa(n) + "/s"
			}
		}
		out = append(out, gr)
	}
	return out
}

// applyJavaRateLimit resolves and stamps the flat rate-limit contract on every
// Java synthetic backend endpoint this file emitted. It mutates Properties in
// place and never adds or removes entities. `before` is the entity-slice length
// captured before the Java synthesizers ran (same window the middleware pass
// uses).
func applyJavaRateLimit(content, path string, entities []types.EntityRecord, before int) {
	if len(content) == 0 || before >= len(entities) {
		return
	}
	methods := indexJavaRateLimitMethods(content)
	gateways := indexJavaGatewayRateLimiters(content)
	if len(methods) == 0 && len(gateways) == 0 {
		return
	}

	// A single bare `@RateLimiter` with no co-located mapping applies to every
	// route in a same-file controller (class-level throttle). Detect that so the
	// fallback path-agnostic binding stays honest (only when there is exactly one
	// such throttle and the file is a single controller).
	var classWide *javaRateLimit
	for i := range methods {
		if methods[i].path == "" {
			m := methods[i].rl
			classWide = &m
			break
		}
	}

	for i := before; i < len(entities); i++ {
		e := &entities[i]
		if e.Kind != httpEndpointDefinitionKind || e.SourceFile != path || e.Properties == nil {
			continue
		}
		routePath := e.Properties["path"]

		// 1. Spring Cloud Gateway filter matched by path predicate (strongest,
		//    path-keyed).
		if gr, ok := matchJavaGateway(gateways, routePath); ok {
			javaRateLimit{found: true, scope: "gateway", source: "RequestRateLimiter", rate: gr.rate}.stamp(e.Properties)
			continue
		}

		// 2. Method-level annotation matched by mapping-path suffix.
		if rl, ok := matchJavaRateLimitMethod(methods, routePath); ok {
			rl.stamp(e.Properties)
			continue
		}

		// 3. Class-wide bare @RateLimiter fallback (honest only when a single
		//    same-file class throttle exists and no path-specific match applied).
		if classWide != nil {
			classWide.stamp(e.Properties)
		}
	}
}

// matchJavaGateway returns the gateway route whose path predicate matches the
// endpoint path (Ant-style prefix matching reusing springPatternMatches).
func matchJavaGateway(gateways []javaGatewayRoute, routePath string) (javaGatewayRoute, bool) {
	for _, gr := range gateways {
		if springPatternMatches(gr.path, routePath, false) || springPathEqual(gr.path, routePath) {
			return gr, true
		}
	}
	return javaGatewayRoute{}, false
}

// matchJavaRateLimitMethod returns the method-level throttle whose mapping path
// matches the endpoint path. A method mapping path is a SUFFIX of the composed
// endpoint path (which includes the class-level @RequestMapping prefix), so a
// suffix / exact match binds. An empty mapping path is skipped here (handled by
// the class-wide fallback).
func matchJavaRateLimitMethod(methods []javaRateLimitedMethod, routePath string) (javaRateLimit, bool) {
	for _, m := range methods {
		if m.path == "" {
			continue
		}
		mp := strings.TrimRight(m.path, "/")
		rp := strings.TrimRight(routePath, "/")
		if mp == rp || strings.HasSuffix(rp, mp) || strings.HasSuffix(rp, strings.TrimPrefix(mp, "/")) {
			return m.rl, true
		}
	}
	return javaRateLimit{}, false
}
