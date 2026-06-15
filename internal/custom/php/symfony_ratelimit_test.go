package php_test

import (
	"context"
	"testing"

	extreg "github.com/cajasmota/grafel/internal/extractor"
	"github.com/cajasmota/grafel/internal/types"

	_ "github.com/cajasmota/grafel/internal/custom/php"
)

// symEndpoint runs the custom_php_symfony extractor and returns the route
// endpoint entity named `name` (e.g. "POST /login"), with full Properties.
func symEndpoint(t *testing.T, src, name string) types.EntityRecord {
	t.Helper()
	e, ok := extreg.Get("custom_php_symfony")
	if !ok {
		t.Fatal("extractor custom_php_symfony not registered")
	}
	ents, err := e.Extract(context.Background(), extreg.FileInput{
		Path: "src/Controller/ApiController.php", Language: "php", Content: []byte(src),
	})
	if err != nil {
		t.Fatalf("extract error: %v", err)
	}
	for _, ent := range ents {
		if ent.Kind == "SCOPE.Operation" && ent.Name == name {
			return ent
		}
	}
	names := make([]string, 0, len(ents))
	for _, ent := range ents {
		if ent.Kind == "SCOPE.Operation" {
			names = append(names, ent.Name)
		}
	}
	t.Fatalf("endpoint %q not found (got: %v)", name, names)
	return types.EntityRecord{}
}

// TestSymfonyRateLimit_AttributeHonestPartial — the canonical spec case: a
// #[RateLimiter('login')] attribute co-located with a #[Route(...)] action →
// the endpoint is rate_limited=true naming the limiter at route scope. The rate
// is honest-partial (omitted): the limit/window live in
// config/packages/rate_limiter.yaml.
func TestSymfonyRateLimit_AttributeHonestPartial(t *testing.T) {
	src := `<?php
namespace App\Controller;
use Symfony\Component\RateLimiter\Attribute\RateLimiter;
use Symfony\Component\Routing\Annotation\Route;
class ApiController {
    #[Route('/login', methods: ['POST'], name: 'login')]
    #[RateLimiter('login')]
    public function login() { return null; }

    #[Route('/health', methods: ['GET'], name: 'health')]
    public function health() { return null; }
}
`
	login := symEndpoint(t, src, "POST /login")
	if login.Properties["rate_limited"] != "true" {
		t.Errorf("POST /login: rate_limited=%q, want true (props: %v)", login.Properties["rate_limited"], login.Properties)
	}
	if login.Properties["rate_limit_scope"] != "route" {
		t.Errorf("POST /login: rate_limit_scope=%q, want route", login.Properties["rate_limit_scope"])
	}
	if login.Properties["rate_limit_source"] != "#[RateLimiter:login]" {
		t.Errorf("POST /login: rate_limit_source=%q, want #[RateLimiter:login]", login.Properties["rate_limit_source"])
	}
	// Honest-partial: the named limiter's rate lives in config; never fabricate.
	if login.Properties["rate_limit"] != "" {
		t.Errorf("POST /login: rate_limit=%q, want omitted (config-driven honest-partial)", login.Properties["rate_limit"])
	}

	// Negative: a sibling action with no RateLimiter attribute is unthrottled.
	health := symEndpoint(t, src, "GET /health")
	if health.Properties["rate_limited"] == "true" {
		t.Errorf("GET /health: rate_limited=true, want unthrottled (props: %v)", health.Properties)
	}
	if health.Properties["rate_limit_source"] != "" {
		t.Errorf("GET /health: leaked rate_limit_source=%q (props: %v)", health.Properties["rate_limit_source"], health.Properties)
	}
}

// TestSymfonyRateLimit_LimiterKeyword — the keyword form
// #[RateLimiter(limiter: 'anonymous_api')] resolves the limiter name too.
func TestSymfonyRateLimit_LimiterKeyword(t *testing.T) {
	src := `<?php
namespace App\Controller;
class ApiController {
    #[Route('/api/items', methods: ['GET'])]
    #[RateLimiter(limiter: 'anonymous_api')]
    public function items() { return null; }
}
`
	p := symEndpoint(t, src, "GET /api/items").Properties
	if p["rate_limited"] != "true" {
		t.Errorf("GET /api/items: rate_limited=%q, want true (props: %v)", p["rate_limited"], p)
	}
	if p["rate_limit_source"] != "#[RateLimiter:anonymous_api]" {
		t.Errorf("GET /api/items: rate_limit_source=%q, want #[RateLimiter:anonymous_api]", p["rate_limit_source"])
	}
}

// TestSymfonyRateLimit_AttributeAboveRoute — the RateLimiter attribute may sit
// ABOVE the Route attribute (order within the block is free); it must still bind.
func TestSymfonyRateLimit_AttributeAboveRoute(t *testing.T) {
	src := `<?php
namespace App\Controller;
class ApiController {
    #[RateLimiter('signup')]
    #[Route('/signup', methods: ['POST'])]
    public function signup() { return null; }
}
`
	p := symEndpoint(t, src, "POST /signup").Properties
	if p["rate_limited"] != "true" || p["rate_limit_source"] != "#[RateLimiter:signup]" {
		t.Errorf("POST /signup: want rate_limited + #[RateLimiter:signup] (props: %v)", p)
	}
}

// TestSymfonyRateLimit_NotMisPairedToSibling — a RateLimiter on one action must
// NOT leak onto a different action's route in the same class.
func TestSymfonyRateLimit_NotMisPairedToSibling(t *testing.T) {
	src := `<?php
namespace App\Controller;
class ApiController {
    #[Route('/a', methods: ['GET'])]
    public function a() { return null; }

    #[Route('/b', methods: ['GET'])]
    #[RateLimiter('b_limiter')]
    public function b() { return null; }
}
`
	a := symEndpoint(t, src, "GET /a").Properties
	if a["rate_limited"] == "true" {
		t.Errorf("GET /a: rate_limited=true, want unthrottled — RateLimiter belongs to /b (props: %v)", a)
	}
	b := symEndpoint(t, src, "GET /b").Properties
	if b["rate_limit_source"] != "#[RateLimiter:b_limiter]" {
		t.Errorf("GET /b: rate_limit_source=%q, want #[RateLimiter:b_limiter]", b["rate_limit_source"])
	}
}
