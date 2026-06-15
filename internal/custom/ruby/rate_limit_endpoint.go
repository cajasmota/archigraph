// rate_limit_endpoint.go — endpoint rate-limit / throttle stamping for Ruby web
// frameworks (#4072, child of #3628 area #6; Ruby sibling of the Python pass
// internal/custom/python/rate_limit_endpoint.go and the JS/TS + Java engine
// passes shipped under #3778).
//
// A `rate_limit` SCOPE.Pattern node already records "this file uses a rate
// limiter" (internal/patterns/rate_limit_extractor.go). This file attributes the
// limiter to the SPECIFIC surface and resolves the numeric rate, stamping the
// SAME flat property contract the other languages use so the graph answers
// "which endpoints are throttled and at what rate?":
//
//	rate_limited      — "true" when a throttle applies.
//	rate_limit        — human rate "10/60s" / "5/60s" when statically resolvable;
//	                    OMITTED (honest-partial) when config-/dynamic-driven.
//	rate_limit_scope  — "route" (Rails controller action) | "request" / "ip"
//	                    (rack-attack discriminator).
//	rate_limit_source — the recognised idiom (`rate_limit`, `rack-attack`).
//	rate_limit_name   — the rack-attack throttle name (evidence; rack-attack only).
//
// Two recognised Ruby surfaces:
//
//	Rails 8 ActionController — `rate_limit to: 10, within: 1.minute, only:
//	                           [:create]` declared in the controller body →
//	                           rate_limited stamped on each affected controller
//	                           action's `SCOPE.Operation/endpoint` op, honouring
//	                           `only:`/`except:`. The route extractor's routes.rb
//	                           op and this op share the `controller#action`
//	                           handler key (same pattern as controller_auth.go).
//
//	rack-attack — `Rack::Attack.throttle("logins/ip", limit: 5, period: 1.minute)
//	              { |req| ... }` → a `SCOPE.Pattern/rate_limit` MARKER naming the
//	              throttle + limit/period. rack-attack throttles bind to a request
//	              discriminator (the block), NOT a named route, so a route join is
//	              heuristic — the marker records the throttle posture honestly
//	              without fabricating a route binding (honest-partial scope).
//	              `Rack::Attack.blocklist`/`safelist` are NOT throttles → not
//	              stamped as a rate limit.
//
// Like the sibling passes, the Rails surface adds NO new node — it stamps the
// flat contract on a controller-action endpoint op. The rack-attack surface adds
// a marker entity (there is no per-route endpoint to attach to).
//
// Refs #4072.
package ruby

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
	extractor.Register("custom_ruby_rate_limit", &rubyRateLimitExtractor{})
}

type rubyRateLimitExtractor struct{}

func (e *rubyRateLimitExtractor) Language() string { return "custom_ruby_rate_limit" }

var (
	// Rails 8 ActionController throttle:
	//   rate_limit to: 10, within: 1.minute, only: [:create]
	// Group 1 = the option string after `rate_limit` (to the end of line).
	rlRailsRateLimitRe = regexp.MustCompile(
		`(?m)^[ \t]*rate_limit\s+([^\n\r]*)$`)

	// `to: 10` — the request cap.
	rlRailsToRe = regexp.MustCompile(`\bto:\s*(\d+)`)

	// `within: 1.minute` / `within: 60` / `within: 1.hour` — the window.
	rlRailsWithinRe = regexp.MustCompile(`\bwithin:\s*([0-9][\w.]*)`)

	// rack-attack throttle:
	//   Rack::Attack.throttle("logins/ip", limit: 5, period: 1.minute) do |req|
	// Group 1 = throttle name, group 2 = the option string (limit/period).
	rlRackAttackThrottleRe = regexp.MustCompile(
		`Rack::Attack\.throttle\s*\(\s*["']([^"']+)["']\s*,\s*([^)]*)\)`)

	// `limit: 5` inside a rack-attack throttle.
	rlRackLimitRe = regexp.MustCompile(`\blimit:\s*(\d+)`)

	// `period: 1.minute` / `period: 60` inside a rack-attack throttle.
	rlRackPeriodRe = regexp.MustCompile(`\bperiod:\s*([0-9][\w.]*)`)
)

// rlDurationSeconds resolves a Ruby duration literal to whole seconds, returning
// (seconds, true) only when statically literal. Recognises a bare integer
// ("60"), and ActiveSupport `.second(s)/.minute(s)/.hour(s)/.day(s)` on an
// integer receiver ("1.minute" → 60, "2.hours" → 7200). Anything else (a config
// constant, an expression) is honest-partial → (0, false).
func rlDurationSeconds(lit string) (int, bool) {
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

// rlHumanRate builds the shared "<count>/<window>s" human rate from a numeric
// cap and a window literal, or "" (honest-partial) when the window is not a
// static duration literal.
func rlHumanRate(count int, windowLit string) string {
	secs, ok := rlDurationSeconds(windowLit)
	if !ok {
		return ""
	}
	return strconv.Itoa(count) + "/" + strconv.Itoa(secs) + "s"
}

func (e *rubyRateLimitExtractor) Extract(ctx context.Context, file extractor.FileInput) ([]types.EntityRecord, error) {
	tracer := otel.Tracer("grafel/custom/ruby")
	_, span := tracer.Start(ctx, "indexer.rails_rate_limit.extract",
		trace.WithAttributes(
			attribute.String("language", file.Language),
			attribute.String("file_path", file.Path),
		))
	defer span.End()

	if len(file.Content) == 0 || file.Language != "ruby" {
		return nil, nil
	}
	src := string(file.Content)
	// Fast guard: must mention one of the recognised idioms.
	if !strings.Contains(src, "rate_limit") && !strings.Contains(src, "Rack::Attack.throttle") {
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

	entities = append(entities, e.extractRailsRateLimit(src, file)...)
	for _, ent := range entities {
		seen[ent.Kind+":"+ent.Name] = true
	}
	for _, ent := range e.extractRackAttack(src, file) {
		add(ent)
	}

	span.SetAttributes(attribute.Int("entity_count", len(entities)))
	return entities, nil
}

// extractRailsRateLimit stamps `rate_limit to: N, within: T` onto each affected
// controller-action endpoint op, honouring only:/except:. Reuses the
// controller_auth.go class/action helpers (same package) so the handler keys
// match the routes.rb ops.
func (e *rubyRateLimitExtractor) extractRailsRateLimit(src string, file extractor.FileInput) []types.EntityRecord {
	if !strings.Contains(src, "Controller") {
		return nil
	}
	var out []types.EntityRecord
	seen := make(map[string]bool)

	for _, cls := range caClassRe.FindAllStringSubmatchIndex(src, -1) {
		className := src[cls[2]:cls[3]]
		body := caClassBody(src, cls[1])
		bodyStart := cls[1]

		posture, ok := resolveRailsRateLimit(body)
		if !ok {
			continue
		}
		actions := caCollectActions(body)
		if len(actions) == 0 {
			continue
		}
		resource := caControllerResource(className)

		for _, act := range actions {
			if !posture.applies(act.name) {
				continue
			}
			handler := resource + "#" + act.name
			key := "endpoint:" + handler
			if seen[key] {
				continue
			}
			seen[key] = true
			ln := lineOf(src, bodyStart+act.off)
			ent := makeEntity(handler, "SCOPE.Operation", "endpoint", file.Path, file.Language, ln)
			setProps(&ent, "framework", "rails",
				"provenance", "INFERRED_FROM_RAILS_RATE_LIMIT",
				"controller", className,
				"controller_action", handler,
				"action", act.name)
			posture.stamp(ent.Properties)
			out = append(out, ent)
		}
	}
	return out
}

// railsRateLimitPosture is the resolved `rate_limit` posture for a controller.
type railsRateLimitPosture struct {
	rate      string // "10/60s" or "" (honest-partial window)
	onlyset   map[string]bool
	exceptset map[string]bool
}

// applies reports whether the controller-level throttle covers the given action.
func (p railsRateLimitPosture) applies(action string) bool {
	if p.onlyset != nil {
		return p.onlyset[action]
	}
	if p.exceptset != nil {
		return !p.exceptset[action]
	}
	return true
}

// stamp writes the shared flat rate-limit contract for a Rails action.
func (p railsRateLimitPosture) stamp(props map[string]string) {
	props["rate_limited"] = "true"
	props["rate_limit_scope"] = "route"
	props["rate_limit_source"] = "rate_limit"
	if p.rate != "" {
		props["rate_limit"] = p.rate
	}
}

// resolveRailsRateLimit finds a controller-level `rate_limit to: N, within: T`
// declaration and resolves its rate + only:/except: scope. Returns ok=false when
// no `rate_limit` declaration with a `to:` cap is present (a plain
// `before_action`, or an unrelated `rate_limit`-named method, never matches).
func resolveRailsRateLimit(body string) (railsRateLimitPosture, bool) {
	m := rlRailsRateLimitRe.FindStringSubmatch(body)
	if m == nil {
		return railsRateLimitPosture{}, false
	}
	opts := m[1]
	toM := rlRailsToRe.FindStringSubmatch(opts)
	if toM == nil {
		// `rate_limit` without a `to:` cap is not the ActionController throttle
		// (it is some unrelated method) — honest no-match.
		return railsRateLimitPosture{}, false
	}
	cap, _ := strconv.Atoi(toM[1])

	var p railsRateLimitPosture
	if wM := rlRailsWithinRe.FindStringSubmatch(opts); wM != nil {
		p.rate = rlHumanRate(cap, wM[1])
	}
	// only:/except: scope, reusing controller_auth.go's option parsers.
	if om := caOnlyRe.FindStringSubmatch(opts); om != nil {
		p.onlyset = map[string]bool{}
		for _, a := range caParseActionList(om[1] + om[2]) {
			p.onlyset[a] = true
		}
	}
	if em := caExceptRe.FindStringSubmatch(opts); em != nil {
		p.exceptset = map[string]bool{}
		for _, a := range caParseActionList(em[1] + em[2]) {
			p.exceptset[a] = true
		}
	}
	return p, true
}

// extractRackAttack emits a marker entity per `Rack::Attack.throttle(...)`. The
// throttle binds to a request discriminator (the block), not a named route, so
// the marker records the throttle posture without a route join (honest-partial).
// blocklist/safelist are intentionally NOT matched (they are not rate limits).
func (e *rubyRateLimitExtractor) extractRackAttack(src string, file extractor.FileInput) []types.EntityRecord {
	if !strings.Contains(src, "Rack::Attack.throttle") {
		return nil
	}
	var out []types.EntityRecord
	for _, m := range rlRackAttackThrottleRe.FindAllStringSubmatchIndex(src, -1) {
		name := src[m[2]:m[3]]
		opts := src[m[4]:m[5]]
		limM := rlRackLimitRe.FindStringSubmatch(opts)
		if limM == nil {
			// A throttle with no literal `limit:` (e.g. a lambda limit) — record
			// the throttle but omit the numeric rate (honest-partial).
			ent := e.rackThrottleEntity(name, "", "", file, lineOf(src, m[0]))
			out = append(out, ent)
			continue
		}
		cap, _ := strconv.Atoi(limM[1])
		periodLit := ""
		if pM := rlRackPeriodRe.FindStringSubmatch(opts); pM != nil {
			periodLit = pM[1]
		}
		out = append(out, e.rackThrottleEntity(name, strconv.Itoa(cap), periodLit, file, lineOf(src, m[0])))
	}
	return out
}

// rackThrottleEntity builds the rate_limit marker for one rack-attack throttle.
func (e *rubyRateLimitExtractor) rackThrottleEntity(name, cap, periodLit string, file extractor.FileInput, line int) types.EntityRecord {
	ent := makeEntity("rack_attack_throttle:"+name, "SCOPE.Pattern", "rate_limit", file.Path, file.Language, line)
	setProps(&ent, "framework", "rack-attack",
		"kind", "rate_limit",
		"rate_limited", "true",
		"rate_limit_source", "rack-attack",
		"rate_limit_name", name,
		// rack-attack discriminates by request (commonly the client IP via
		// req.ip); the binding is a request matcher, not a named route.
		"rate_limit_scope", "request")
	if cap != "" {
		ent.Properties["limit"] = cap
		if rate := rlHumanRate(atoiOr0(cap), periodLit); rate != "" {
			ent.Properties["rate_limit"] = rate
		}
	}
	if secs, ok := rlDurationSeconds(periodLit); ok {
		ent.Properties["period"] = strconv.Itoa(secs)
	}
	return ent
}

// atoiOr0 parses a decimal string, returning 0 on failure (the caller only
// passes already-validated integer strings).
func atoiOr0(s string) int {
	n, _ := strconv.Atoi(s)
	return n
}
