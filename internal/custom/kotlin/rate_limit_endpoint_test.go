package kotlin

import (
	"context"
	"testing"

	"github.com/cajasmota/grafel/internal/extractor"
	"github.com/cajasmota/grafel/internal/types"
)

func runKtRateLimit(t *testing.T, path, src string) []types.EntityRecord {
	t.Helper()
	ents, err := (&kotlinRateLimitExtractor{}).Extract(context.Background(), extractor.FileInput{
		Path: path, Language: "kotlin", Content: []byte(src),
	})
	if err != nil {
		t.Fatalf("rate_limit extract: %v", err)
	}
	return ents
}

func ktRLByName(ents []types.EntityRecord) map[string]types.EntityRecord {
	out := map[string]types.EntityRecord{}
	for _, e := range ents {
		out[e.Name] = e
	}
	return out
}

// --- Ktor RateLimit plugin ----------------------------------------------------

// TestKtorRateLimitNamedLimiterStampsGuardedRoute — install(RateLimit) declares
// a named limiter with literal limit/refillPeriod; the rateLimit(RateLimitName)
// block stamps the guarded GET with the resolved 100/60s rate + name=api, and
// the UNGUARDED route is NOT stamped.
func TestKtorRateLimitNamedLimiterStampsGuardedRoute(t *testing.T) {
	src := `fun Application.module() {
    install(RateLimit) {
        register(RateLimitName("api")) {
            rateLimiter(limit = 100, refillPeriod = 60.seconds)
        }
    }
    routing {
        rateLimit(RateLimitName("api")) {
            get("/throttled") { call.respond("ok") }
        }
        get("/open") { call.respond("ok") }
    }
}`
	eps := ktRLByName(runKtRateLimit(t, "App.kt", src))

	e, ok := eps["GET /throttled"]
	if !ok {
		t.Fatalf("missing rate-limited op GET /throttled; got %v", keysOf(eps))
	}
	if e.Properties["rate_limited"] != "true" {
		t.Errorf("rate_limited=%q, want true", e.Properties["rate_limited"])
	}
	if e.Properties["rate_limit"] != "100/60s" {
		t.Errorf("rate_limit=%q, want 100/60s (limit=100 refillPeriod=60.seconds)", e.Properties["rate_limit"])
	}
	if e.Properties["rate_limit_scope"] != "route" {
		t.Errorf("rate_limit_scope=%q, want route", e.Properties["rate_limit_scope"])
	}
	if e.Properties["rate_limit_source"] != "ktor" {
		t.Errorf("rate_limit_source=%q, want ktor", e.Properties["rate_limit_source"])
	}
	if e.Properties["rate_limit_name"] != "api" {
		t.Errorf("rate_limit_name=%q, want api", e.Properties["rate_limit_name"])
	}
	// NEGATIVE: the unguarded route must NOT be stamped.
	if _, leaked := eps["GET /open"]; leaked {
		t.Errorf("unguarded GET /open was stamped rate_limited")
	}
}

// TestKtorRateLimitComposesNestedRoutePrefix — a verb handler nested under
// route("/api"){ route("/v1"){ … } } inside a rateLimit guard composes the full
// path.
func TestKtorRateLimitComposesNestedRoutePrefix(t *testing.T) {
	src := `fun Application.module() {
    install(RateLimit) {
        register(RateLimitName("api")) {
            rateLimiter(limit = 5, refillPeriod = 1.minutes)
        }
    }
    routing {
        rateLimit(RateLimitName("api")) {
            route("/api") {
                route("/v1") {
                    post("/users") { call.respond("ok") }
                }
            }
        }
    }
}`
	eps := ktRLByName(runKtRateLimit(t, "App.kt", src))
	e, ok := eps["POST /api/v1/users"]
	if !ok {
		t.Fatalf("missing composed op POST /api/v1/users; got %v", keysOf(eps))
	}
	if e.Properties["rate_limit"] != "5/60s" {
		t.Errorf("rate_limit=%q, want 5/60s (limit=5 refillPeriod=1.minutes)", e.Properties["rate_limit"])
	}
	if e.Properties["rate_limit_name"] != "api" {
		t.Errorf("rate_limit_name=%q, want api", e.Properties["rate_limit_name"])
	}
}

// TestKtorRateLimitConfigDrivenIsHonestPartial — a named limiter whose
// limit/refillPeriod are NOT inline literals (config-/variable-driven) stamps
// rate_limited+name but OMITS the numeric rate (honest-partial).
func TestKtorRateLimitConfigDrivenIsHonestPartial(t *testing.T) {
	src := `fun Application.module() {
    install(RateLimit) {
        register(RateLimitName("dynamic")) {
            rateLimiter(limit = config.limit, refillPeriod = config.window)
        }
    }
    routing {
        rateLimit(RateLimitName("dynamic")) {
            get("/cfg") { call.respond("ok") }
        }
    }
}`
	e, ok := ktRLByName(runKtRateLimit(t, "App.kt", src))["GET /cfg"]
	if !ok {
		t.Fatalf("missing GET /cfg")
	}
	if e.Properties["rate_limited"] != "true" {
		t.Errorf("rate_limited=%q, want true", e.Properties["rate_limited"])
	}
	if r, has := e.Properties["rate_limit"]; has {
		t.Errorf("rate_limit=%q present, want OMITTED (config-driven honest-partial)", r)
	}
	if e.Properties["rate_limit_name"] != "dynamic" {
		t.Errorf("rate_limit_name=%q, want dynamic", e.Properties["rate_limit_name"])
	}
}

// TestKtorBareRateLimitGuard — the no-arg `rateLimit { … }` default-limiter
// guard stamps its nested verbs (no name, rate honest-partial unless a default
// registration resolved one).
func TestKtorBareRateLimitGuard(t *testing.T) {
	src := `fun Application.module() {
    routing {
        rateLimit {
            get("/default") { call.respond("ok") }
        }
    }
}`
	e, ok := ktRLByName(runKtRateLimit(t, "App.kt", src))["GET /default"]
	if !ok {
		t.Fatalf("missing GET /default")
	}
	if e.Properties["rate_limited"] != "true" {
		t.Errorf("rate_limited=%q, want true", e.Properties["rate_limited"])
	}
	if e.Properties["rate_limit_source"] != "ktor" {
		t.Errorf("rate_limit_source=%q, want ktor", e.Properties["rate_limit_source"])
	}
}

// TestKtorNonRateLimitPluginIsNotStamped — install(CORS) and a plain route with
// no rateLimit guard produce NO rate-limit stamps (negative).
func TestKtorNonRateLimitPluginIsNotStamped(t *testing.T) {
	src := `fun Application.module() {
    install(CORS) {
        anyHost()
    }
    routing {
        get("/plain") { call.respond("ok") }
    }
}`
	ents := runKtRateLimit(t, "App.kt", src)
	if len(ents) != 0 {
		t.Errorf("expected 0 rate-limit ops for non-rate-limit plugin + plain route, got %d: %v", len(ents), keysOf(ktRLByName(ents)))
	}
}

// --- Spring-Boot-Kotlin Resilience4j @RateLimiter -----------------------------

// TestSpringKotlinRateLimiterStampsHandler — @RateLimiter(name="orders") on a
// @GetMapping handler stamps the composed endpoint with source naming orders;
// rate is config-driven (honest-partial). The un-annotated handler is NOT
// stamped.
func TestSpringKotlinRateLimiterStampsHandler(t *testing.T) {
	src := `@RestController
@RequestMapping("/orders")
class OrderController {
    @RateLimiter(name = "orders")
    @GetMapping("/list")
    fun list() = "ok"

    @GetMapping("/free")
    fun free() = "ok"
}`
	eps := ktRLByName(runKtRateLimit(t, "OrderController.kt", src))

	e, ok := eps["GET /orders/list"]
	if !ok {
		t.Fatalf("missing rate-limited op GET /orders/list; got %v", keysOf(eps))
	}
	if e.Properties["rate_limited"] != "true" {
		t.Errorf("rate_limited=%q, want true", e.Properties["rate_limited"])
	}
	if e.Properties["rate_limit_source"] != "@RateLimiter(orders)" {
		t.Errorf("rate_limit_source=%q, want @RateLimiter(orders)", e.Properties["rate_limit_source"])
	}
	if e.Properties["rate_limit_name"] != "orders" {
		t.Errorf("rate_limit_name=%q, want orders", e.Properties["rate_limit_name"])
	}
	if e.Properties["rate_limit_scope"] != "route" {
		t.Errorf("rate_limit_scope=%q, want route", e.Properties["rate_limit_scope"])
	}
	// HONEST-PARTIAL: Resilience4j limit lives in resilience4j.ratelimiter.* config.
	if r, has := e.Properties["rate_limit"]; has {
		t.Errorf("rate_limit=%q present, want OMITTED (config-driven honest-partial)", r)
	}
	// NEGATIVE: the un-annotated handler must NOT be stamped.
	if _, leaked := eps["GET /orders/free"]; leaked {
		t.Errorf("un-annotated GET /orders/free was stamped")
	}
}

// TestSpringKotlinRateLimiterNoControllerNoStamp — @RateLimiter outside a
// controller (e.g. on a service method) is NOT a route throttle (negative).
func TestSpringKotlinRateLimiterNoControllerNoStamp(t *testing.T) {
	src := `@Service
class OrderService {
    @RateLimiter(name = "svc")
    fun process() = "ok"
}`
	ents := runKtRateLimit(t, "OrderService.kt", src)
	if len(ents) != 0 {
		t.Errorf("expected 0 ops for @RateLimiter on a non-controller service, got %d", len(ents))
	}
}

// keysOf is a tiny test helper returning the map keys for error messages.
func keysOf(m map[string]types.EntityRecord) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	return out
}
