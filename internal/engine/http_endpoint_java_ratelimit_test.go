package engine

import "testing"

// javaRLProps runs full synthesis over a Java source and returns the endpoint
// at "<VERB> <path>".
func javaRLProps(t *testing.T, content, key string) map[string]string {
	t.Helper()
	eps := authProps(t, "java", "src/main/java/com/x/App.java", content)
	e, ok := eps[key]
	if !ok {
		keys := make([]string, 0, len(eps))
		for k := range eps {
			keys = append(keys, k)
		}
		t.Fatalf("endpoint %q not synthesised (got: %v)", key, keys)
	}
	return e.Properties
}

// TestJavaRateLimit_Resilience4jAnnotation — the canonical spec case:
// `@RateLimiter(name="api")` on a `@GetMapping` method → that endpoint is
// rate_limited=true (rate honest-partial: it lives in config); a sibling method
// with no throttle is NOT stamped (negative).
func TestJavaRateLimit_Resilience4jAnnotation(t *testing.T) {
	src := `package com.x;
import org.springframework.web.bind.annotation.*;
import io.github.resilience4j.ratelimiter.annotation.RateLimiter;
@RestController
@RequestMapping("/api")
class ApiController {
  @RateLimiter(name="api")
  @GetMapping("/x")
  public Object getX() { return null; }

  @GetMapping("/free")
  public Object free() { return null; }
}
`
	p := javaRLProps(t, src, "GET /api/x")
	if p["rate_limited"] != "true" {
		t.Errorf("GET /api/x: rate_limited=%q, want true (props: %v)", p["rate_limited"], p)
	}
	if p["rate_limit_scope"] != "route" {
		t.Errorf("GET /api/x: rate_limit_scope=%q, want route", p["rate_limit_scope"])
	}
	if p["rate_limit_source"] != "@RateLimiter" {
		t.Errorf("GET /api/x: rate_limit_source=%q, want @RateLimiter", p["rate_limit_source"])
	}
	// Honest-partial: a bare Resilience4j @RateLimiter's limit lives in
	// application.yml, so the rate MUST be omitted (never fabricated).
	if p["rate_limit"] != "" {
		t.Errorf("GET /api/x: rate_limit=%q, want omitted (config-driven honest-partial)", p["rate_limit"])
	}

	free := javaRLProps(t, src, "GET /api/free")
	if free["rate_limited"] == "true" {
		t.Errorf("GET /api/free: rate_limited=true, want unthrottled (props: %v)", free)
	}
}

// TestJavaRateLimit_Bucket4jLiteral — a bucket4j `@RateLimiting(capacity = 100)`
// with a literal capacity resolves the rate.
func TestJavaRateLimit_Bucket4jLiteral(t *testing.T) {
	src := `package com.x;
import org.springframework.web.bind.annotation.*;
@RestController
class BucketController {
  @RateLimiting(name = "ep", capacity = 100)
  @PostMapping("/orders")
  public Object create() { return null; }
}
`
	p := javaRLProps(t, src, "ANY /orders")
	if p["rate_limited"] != "true" {
		t.Errorf("/orders: rate_limited=%q, want true (props: %v)", p["rate_limited"], p)
	}
	if p["rate_limit"] != "100/s" {
		t.Errorf("/orders: rate_limit=%q, want 100/s", p["rate_limit"])
	}
	if p["rate_limit_source"] != "@RateLimiting" {
		t.Errorf("/orders: rate_limit_source=%q, want @RateLimiting", p["rate_limit_source"])
	}
}

// TestJavaRateLimit_SpringCloudGateway — a Spring Cloud Gateway YAML route with
// a RequestRateLimiter filter (replenishRate=10) matched to its Path= predicate
// → endpoints under that path are rate_limited=true rate="10/s" scope=gateway.
func TestJavaRateLimit_SpringCloudGateway(t *testing.T) {
	src := `package com.x;
import org.springframework.web.bind.annotation.*;
@RestController
class GatewayBackedController {
  @GetMapping("/api/items")
  public Object items() { return null; }
}
/* Spring Cloud Gateway route config (application.yml-equivalent):
   - id: items_route
     uri: lb://items
     predicates:
       - Path=/api/**
     filters:
       - name: RequestRateLimiter
         args:
           replenishRate: 10
           burstCapacity: 20
*/
`
	p := javaRLProps(t, src, "ANY /api/items")
	if p["rate_limited"] != "true" {
		t.Errorf("/api/items: rate_limited=%q, want true (props: %v)", p["rate_limited"], p)
	}
	if p["rate_limit"] != "10/s" {
		t.Errorf("/api/items: rate_limit=%q, want 10/s", p["rate_limit"])
	}
	if p["rate_limit_scope"] != "gateway" {
		t.Errorf("/api/items: rate_limit_scope=%q, want gateway", p["rate_limit_scope"])
	}
	if p["rate_limit_source"] != "RequestRateLimiter" {
		t.Errorf("/api/items: rate_limit_source=%q, want RequestRateLimiter", p["rate_limit_source"])
	}
}

// TestJavaRateLimit_NonThrottleUnaffected — a non-throttle annotation
// (@Validated) on a mapped method must NOT stamp a rate limit.
func TestJavaRateLimit_NonThrottleUnaffected(t *testing.T) {
	src := `package com.x;
import org.springframework.web.bind.annotation.*;
import org.springframework.validation.annotation.Validated;
@RestController
class PlainController {
  @Validated
  @GetMapping("/plain")
  public Object plain() { return null; }
}
`
	p := javaRLProps(t, src, "ANY /plain")
	if p["rate_limited"] == "true" {
		t.Errorf("/plain: rate_limited=true, want unthrottled (@Validated is not a limiter; props: %v)", p)
	}
}
