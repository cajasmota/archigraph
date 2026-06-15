// rate_limit_endpoint.go — endpoint rate-limit / throttle stamping for Kotlin web
// frameworks (#4095, child of #3628 area #6). Greenfield: Kotlin carried no
// rate_limit coverage. Kotlin sibling of the Ruby pass
// (internal/custom/ruby/rate_limit_endpoint.go) and the Java engine pass
// (internal/engine/http_endpoint_java_ratelimit.go); stamps the SAME flat
// property contract on the endpoint op (no parallel node) so the graph answers
// "which Kotlin endpoints are throttled and at what rate?":
//
//	rate_limited      — "true" when a throttle applies to the endpoint.
//	rate_limit        — human rate "100/60s" when statically resolvable (a Ktor
//	                    rateLimiter(limit=N, refillPeriod=T) literal); OMITTED
//	                    (honest-partial) when the rate lives in config (a
//	                    Resilience4j @RateLimiter(name="x") whose limitForPeriod
//	                    is in application.yml).
//	rate_limit_scope  — "route" (both surfaces guard a specific route/handler).
//	rate_limit_source — the recognised idiom (`ktor`, `@RateLimiter(<name>)`).
//	rate_limit_name   — the limiter name (Ktor RateLimitName / Resilience4j name).
//
// Two recognised Kotlin surfaces:
//
//	Ktor RateLimit plugin — `install(RateLimit){ register(RateLimitName("api")){
//	                        rateLimiter(limit=100, refillPeriod=60.seconds) } }`
//	                        declares a NAMED limiter (limit/window resolved when
//	                        literal). A `rateLimit(RateLimitName("api")){ … }`
//	                        block (or the no-arg `rateLimit { … }` using the
//	                        default limiter) then GUARDS the verb handlers nested
//	                        inside it → each `get/post/…("/path")` within the
//	                        guarded block is stamped, composing any enclosing
//	                        `route("/prefix")` prefixes (same naming as
//	                        ktor_routes.go so the rate-limited op merges onto the
//	                        plain route op by Name).
//
//	Spring-Boot-Kotlin Resilience4j — `@RateLimiter(name="orders")` on a
//	                        `@GetMapping`/`@PostMapping`/… handler inside a
//	                        `@RestController`/`@Controller` → rate_limited on that
//	                        handler's endpoint op (same name as routing.go's
//	                        Spring extractor). The numeric limit lives in
//	                        `resilience4j.ratelimiter.<name>` config, so the rate
//	                        is honest-partial (omitted); the limiter name is folded
//	                        into rate_limit_source as evidence. This is the KOTLIN
//	                        native path — custom_java_patterns hard-skips .kt
//	                        (#3584), so .kt @RateLimiter would otherwise be unseen.
//
// Like the sibling passes this adds NO new node kind beyond the endpoint op it
// re-emits with the contract attached; MergeWithCustom / downstream dedup folds
// it onto the plain route op sharing the same Name.
//
// Refs #4095.
package kotlin

import (
	"context"
	"regexp"
	"strconv"
	"strings"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/trace"

	"github.com/cajasmota/grafel/internal/extractor"
	"github.com/cajasmota/grafel/internal/types"
)

func init() {
	extractor.Register("custom_kotlin_rate_limit", &kotlinRateLimitExtractor{})
}

type kotlinRateLimitExtractor struct{}

func (e *kotlinRateLimitExtractor) Language() string { return "custom_kotlin_rate_limit" }

var (
	// ktRLRegisterRe matches a Ktor limiter registration inside install(RateLimit):
	//   register(RateLimitName("api")) { rateLimiter(limit = 100, refillPeriod = 60.seconds) }
	// Group 1 = the limiter name, group 2 = the register block body (rateLimiter args).
	ktRLRegisterRe = regexp.MustCompile(
		`register\s*\(\s*RateLimitName\s*\(\s*"([^"]+)"\s*\)\s*\)\s*\{([^}]*)\}`)

	// ktRLLimitRe / ktRLRefillRe pull literal limit / refillPeriod out of a
	// rateLimiter(...) call body.
	ktRLLimitRe  = regexp.MustCompile(`\blimit\s*=\s*(\d+)`)
	ktRLRefillRe = regexp.MustCompile(`\brefillPeriod\s*=\s*([0-9][\w.]*)`)

	// ktRLGuardNamedRe matches a named rate-limit guard opening:
	//   rateLimit(RateLimitName("api")) {
	// Group 1 = the guarded limiter name. Index form so the caller can scan the
	// brace-balanced block that follows.
	ktRLGuardNamedRe = regexp.MustCompile(
		`\brateLimit\s*\(\s*RateLimitName\s*\(\s*"([^"]+)"\s*\)\s*\)\s*\{`)

	// ktRLGuardBareRe matches a no-arg default-limiter guard opening:
	//   rateLimit {
	// (the global default limiter registered via register(RateLimitName.Default)).
	ktRLGuardBareRe = regexp.MustCompile(`\brateLimit\s*\{`)

	// ktRLRouteRe matches an enclosing route("/prefix") { opener inside a guarded
	// block; group 1 = the prefix. Used to compose nested paths.
	ktRLRouteRe = regexp.MustCompile(`\broute\s*\(\s*"([^"]*)"\s*\)\s*\{`)

	// ktRLVerbRe matches a Ktor verb handler get("/path") / post("/path"); group
	// 1 = verb, group 2 = path.
	ktRLVerbRe = regexp.MustCompile(
		`\b(get|post|put|delete|patch|head|options)\s*\(\s*"([^"]*)"`)

	// ktRLSpringAnnoRe matches a Resilience4j @RateLimiter annotation; group 1 =
	// the (optional) annotation argument body carrying name="x".
	ktRLSpringAnnoRe = regexp.MustCompile(`@RateLimiter\b\s*(?:\(([^)]*)\))?`)

	// ktRLSpringNameRe pulls name = "x" out of a @RateLimiter body.
	ktRLSpringNameRe = regexp.MustCompile(`\bname\s*=\s*"([^"]+)"`)

	// ktRLSpringVerbRe matches a Spring verb mapping with optional path; group 1 =
	// verb stem, group 2 = path.
	ktRLSpringVerbRe = regexp.MustCompile(
		`@(Get|Post|Put|Delete|Patch|Head|Options)Mapping\s*(?:\(\s*(?:value\s*=\s*|path\s*=\s*)?"([^"]*)"\s*\))?`)

	// ktRLSpringClassMappingRe matches a class-level @RequestMapping prefix.
	ktRLSpringClassMappingRe = regexp.MustCompile(
		`@RequestMapping\s*(?:\(\s*(?:value\s*=\s*|path\s*=\s*)?"([^"]*)"\s*\))?`)

	// ktRLSpringControllerRe gates the Spring surface on a controller annotation.
	ktRLSpringControllerRe = regexp.MustCompile(`@(?:Rest)?Controller\b`)
)

var ktRLSpringVerbMap = map[string]string{
	"Get": "GET", "Post": "POST", "Put": "PUT", "Delete": "DELETE",
	"Patch": "PATCH", "Head": "HEAD", "Options": "OPTIONS",
}

// ktRLDurationSeconds resolves a Kotlin Duration literal to whole seconds,
// returning (seconds, true) only when statically literal. Recognises a bare
// integer ("60", milliseconds are NOT assumed — a bare int in refillPeriod is
// treated as seconds only when it has no unit suffix, matching the kotlin.time
// `Int.seconds`-style call) and the `N.seconds/.minutes/.hours/.days`
// kotlin.time extension form ("60.seconds" → 60, "2.minutes" → 120). Anything
// else (a config constant, an expression) is honest-partial → (0, false).
func ktRLDurationSeconds(lit string) (int, bool) {
	lit = strings.TrimSpace(lit)
	if n, err := strconv.Atoi(lit); err == nil {
		return n, true
	}
	m := regexp.MustCompile(`^(\d+)\.(second|minute|hour|day)s?$`).FindStringSubmatch(lit)
	if m == nil {
		return 0, false
	}
	n, _ := strconv.Atoi(m[1])
	switch m[2] {
	case "second":
		return n, true
	case "minute":
		return n * 60, true
	case "hour":
		return n * 3600, true
	case "day":
		return n * 86400, true
	}
	return 0, false
}

// ktRLHumanRate builds the shared "<count>/<window>s" human rate, or ""
// (honest-partial) when the window is not a static duration literal.
func ktRLHumanRate(count int, windowLit string) string {
	secs, ok := ktRLDurationSeconds(windowLit)
	if !ok {
		return ""
	}
	return strconv.Itoa(count) + "/" + strconv.Itoa(secs) + "s"
}

// ktRLLimiter is a resolved Ktor named limiter (limit/window when literal).
type ktRLLimiter struct {
	name string
	rate string // "100/60s" or "" (honest-partial)
}

func (e *kotlinRateLimitExtractor) Extract(ctx context.Context, file extractor.FileInput) ([]types.EntityRecord, error) {
	tracer := otel.Tracer("grafel/custom/kotlin")
	_, span := tracer.Start(ctx, "indexer.kotlin_rate_limit.extract",
		trace.WithAttributes(
			attribute.String("language", file.Language),
			attribute.String("file_path", file.Path),
		))
	defer span.End()

	if len(file.Content) == 0 || file.Language != "kotlin" {
		return nil, nil
	}
	src := string(file.Content)
	// Fast guard: must mention one of the recognised idioms.
	if !strings.Contains(src, "rateLimit") && !strings.Contains(src, "@RateLimiter") {
		return nil, nil
	}

	var entities []types.EntityRecord
	seen := make(map[string]bool)
	add := func(ent types.EntityRecord) {
		key := ent.Kind + ":" + ent.Name
		if seen[key] {
			return
		}
		seen[key] = true
		entities = append(entities, ent)
	}

	for _, ent := range e.extractKtor(src, file) {
		add(ent)
	}
	for _, ent := range e.extractSpring(src, file) {
		add(ent)
	}

	span.SetAttributes(attribute.Int("entity_count", len(entities)))
	return entities, nil
}

// extractKtor resolves named limiters from install(RateLimit){…} and stamps
// every verb handler nested inside a rateLimit(RateLimitName("x")){…} (or a
// no-arg rateLimit{…}) guard, composing enclosing route("/prefix") prefixes.
func (e *kotlinRateLimitExtractor) extractKtor(src string, file extractor.FileInput) []types.EntityRecord {
	if !strings.Contains(src, "rateLimit") {
		return nil
	}
	limiters := resolveKtorLimiters(src)

	var out []types.EntityRecord
	seen := make(map[string]bool)

	// Walk every guard opener (named first, then bare), claim its brace-balanced
	// block, and stamp the verbs inside. Named guards are processed before bare
	// guards so a `rateLimit(RateLimitName("x")){ rateLimit { … } }` resolves the
	// inner-most name correctly per region.
	type guard struct {
		bodyStart int // byte offset just after the opening '{'
		bodyEnd   int // byte offset of the matching '}'
		limiter   ktRLLimiter
	}
	var guards []guard

	for _, m := range ktRLGuardNamedRe.FindAllStringSubmatchIndex(src, -1) {
		name := src[m[2]:m[3]]
		bodyStart := m[1] // m[1] is just past the opening brace captured in the regex
		bodyEnd, ok := ktRLMatchBrace(src, bodyStart)
		if !ok {
			continue
		}
		guards = append(guards, guard{bodyStart, bodyEnd, ktRLLimiter{name: name, rate: limiters[name].rate}})
	}
	for _, m := range ktRLGuardBareRe.FindAllStringSubmatchIndex(src, -1) {
		// Skip a `rateLimit(` that the named regex already consumed — the bare
		// regex `rateLimit\s*\{` only matches the no-arg form, so a named guard's
		// `rateLimit(RateLimitName…){` never matches here. Default-limiter guard.
		bodyStart := m[1]
		bodyEnd, ok := ktRLMatchBrace(src, bodyStart)
		if !ok {
			continue
		}
		// Default limiter (RateLimitName.Default) — rate honest-partial unless a
		// single default registration resolved a literal rate.
		guards = append(guards, guard{bodyStart, bodyEnd, ktRLLimiter{name: "", rate: limiters[""].rate}})
	}

	for _, g := range guards {
		region := src[g.bodyStart:g.bodyEnd]
		// Collect enclosing route("/prefix") prefixes that wrap verb handlers
		// inside the guard, brace-aware, so nested paths compose.
		for _, v := range ktRLVerbRe.FindAllStringSubmatchIndex(region, -1) {
			verb := strings.ToUpper(region[v[2]:v[3]])
			leaf := region[v[4]:v[5]]
			prefix := ktRLEnclosingRoutePrefix(region, v[0])
			fullPath := joinKtRoutePaths(prefix, leaf)
			name := verb + " " + fullPath
			if seen[name] {
				continue
			}
			seen[name] = true
			ln := lineOf(src, g.bodyStart+v[0])
			ent := makeEntity(name, "SCOPE.Operation", "endpoint", file.Path, "kotlin", ln)
			setProps(&ent,
				"framework", "ktor",
				"http_method", verb,
				"path", fullPath,
				"provenance", "INFERRED_FROM_KTOR_RATE_LIMIT",
				"rate_limited", "true",
				"rate_limit_scope", "route",
				"rate_limit_source", "ktor",
			)
			if g.limiter.name != "" {
				ent.Properties["rate_limit_name"] = g.limiter.name
			}
			if g.limiter.rate != "" {
				ent.Properties["rate_limit"] = g.limiter.rate
			}
			out = append(out, ent)
		}
	}
	return out
}

// resolveKtorLimiters parses install(RateLimit){ register(RateLimitName("x")){
// rateLimiter(limit=N, refillPeriod=T) } } into name→limiter (rate resolved when
// literal). The empty-string key holds the default limiter when one is declared
// via RateLimitName.Default; named registrations key by their string name.
func resolveKtorLimiters(src string) map[string]ktRLLimiter {
	out := map[string]ktRLLimiter{}
	for _, m := range ktRLRegisterRe.FindAllStringSubmatch(src, -1) {
		name := m[1]
		body := m[2]
		lim := ktRLLimiter{name: name}
		if lm := ktRLLimitRe.FindStringSubmatch(body); lm != nil {
			n, _ := strconv.Atoi(lm[1])
			windowLit := ""
			if rm := ktRLRefillRe.FindStringSubmatch(body); rm != nil {
				windowLit = rm[1]
			}
			lim.rate = ktRLHumanRate(n, windowLit)
		}
		out[name] = lim
	}
	return out
}

// ktRLEnclosingRoutePrefix returns the composed route("/prefix") prefix path
// that brace-encloses the verb call at byte offset verbOff within region. It
// walks every route("/p"){ opener whose brace-balanced block contains verbOff,
// joining the prefixes outermost-first. Returns "" when no enclosing route.
func ktRLEnclosingRoutePrefix(region string, verbOff int) string {
	var prefix string
	for _, rm := range ktRLRouteRe.FindAllStringSubmatchIndex(region, -1) {
		bodyStart := rm[1] // past the opening brace
		bodyEnd, ok := ktRLMatchBrace(region, bodyStart)
		if !ok {
			continue
		}
		if rm[0] < verbOff && verbOff < bodyEnd {
			prefix = joinKtRoutePaths(prefix, region[rm[2]:rm[3]])
		}
	}
	return prefix
}

// ktRLMatchBrace returns the byte offset of the '}' matching the '{' that ends
// at openEnd-1 (i.e. openEnd is the first byte of the block body). It tracks
// brace depth, honouring string literals so a '}' inside a "…" does not
// mis-balance. ok=false on an unterminated block.
func ktRLMatchBrace(src string, openEnd int) (int, bool) {
	depth := 1
	inStr := false
	var quote byte
	for i := openEnd; i < len(src); i++ {
		c := src[i]
		if inStr {
			if c == '\\' {
				i++
				continue
			}
			if c == quote {
				inStr = false
			}
			continue
		}
		switch c {
		case '"', '\'':
			inStr = true
			quote = c
		case '{':
			depth++
		case '}':
			depth--
			if depth == 0 {
				return i, true
			}
		}
	}
	return 0, false
}

// extractSpring stamps a Resilience4j @RateLimiter(name="x") onto the endpoint
// op of the verb-mapped handler it annotates, inside a @RestController /
// @Controller. The numeric limit is config-driven (resilience4j.ratelimiter.*)
// so the rate is honest-partial; the limiter name is folded into the source as
// evidence. Endpoint Name matches routing.go's Spring extractor so they merge.
func (e *kotlinRateLimitExtractor) extractSpring(src string, file extractor.FileInput) []types.EntityRecord {
	if !strings.Contains(src, "@RateLimiter") || !ktRLSpringControllerRe.MatchString(src) {
		return nil
	}

	classPrefix := ""
	if m := ktRLSpringClassMappingRe.FindStringSubmatchIndex(src); m != nil && m[2] >= 0 {
		classPrefix = src[m[2]:m[3]]
	}

	var out []types.EntityRecord
	seen := make(map[string]bool)

	for _, am := range ktRLSpringAnnoRe.FindAllStringSubmatchIndex(src, -1) {
		// Limiter name (evidence) when present.
		name := ""
		if am[2] >= 0 {
			if nm := ktRLSpringNameRe.FindStringSubmatch(src[am[2]:am[3]]); nm != nil {
				name = nm[1]
			}
		}
		// Pair with the nearest verb mapping in the same handler annotation block:
		// scan a forward window from @RateLimiter for the next @<Verb>Mapping
		// (within one handler head). The annotation order is
		// @RateLimiter\n@GetMapping(...) fun … — both directions are checked so an
		// annotation placed after the mapping still binds.
		win := am[1]
		end := win + 400
		if end > len(src) {
			end = len(src)
		}
		fwd := src[am[0]:end]
		vm := ktRLSpringVerbRe.FindStringSubmatch(fwd)
		if vm == nil {
			// Look backward one short window (mapping above the @RateLimiter).
			bstart := am[0] - 400
			if bstart < 0 {
				bstart = 0
			}
			back := src[bstart:am[1]]
			all := ktRLSpringVerbRe.FindAllStringSubmatch(back, -1)
			if len(all) == 0 {
				continue
			}
			vm = all[len(all)-1] // nearest preceding mapping
		}
		verb := ktRLSpringVerbMap[vm[1]]
		methodPath := vm[2]
		fullPath := joinKtRoutePaths(classPrefix, methodPath)
		epName := verb + " " + fullPath
		if seen[epName] {
			continue
		}
		seen[epName] = true
		ln := lineOf(src, am[0])
		ent := makeEntity(epName, "SCOPE.Operation", "endpoint", file.Path, "kotlin", ln)
		source := "@RateLimiter"
		if name != "" {
			source = "@RateLimiter(" + name + ")"
		}
		setProps(&ent,
			"framework", "spring-boot",
			"http_method", verb,
			"path", fullPath,
			"provenance", "INFERRED_FROM_RESILIENCE4J_RATE_LIMITER",
			"rate_limited", "true",
			"rate_limit_scope", "route",
			"rate_limit_source", source,
		)
		if name != "" {
			ent.Properties["rate_limit_name"] = name
		}
		out = append(out, ent)
	}
	return out
}
