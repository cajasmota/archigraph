package rust_test

// rate_limit_test.go — value-asserting fixtures for the Rust rate_limit_stamping
// pass (#4124, child of #3628). Proves the flat contract
// (rate_limited / rate_limit / rate_limit_scope / rate_limit_source /
// limit / period / rate_limit_burst) fires for the genuine Rust idioms
// (tower-governor on axum, actix-governor, tower::limit RateLimitLayer) and
// proves the negatives (plain route, non-rate tower layer) do NOT stamp.

import "testing"

// findRustRateLimit returns the first SCOPE.Pattern/rate_limit entity matching
// pred, or nil.
func findRustRateLimit(ents []entitySummary, pred func(entitySummary) bool) *entitySummary {
	for i := range ents {
		if ents[i].Kind == "SCOPE.Pattern" && ents[i].Subtype == "rate_limit" && pred(ents[i]) {
			return &ents[i]
		}
	}
	return nil
}

func anyRustRateLimit(ents []entitySummary) bool {
	return findRustRateLimit(ents, func(entitySummary) bool { return true }) != nil
}

// --- 1. tower-governor on axum: per_second(5) + burst_size(10) --------------

func TestRustRateLimit_TowerGovernorAxum(t *testing.T) {
	src := `
use axum::Router;
use tower_governor::{governor::GovernorConfigBuilder, GovernorLayer};

fn app() -> Router {
    let conf = GovernorConfigBuilder::default()
        .per_second(5)
        .burst_size(10)
        .finish()
        .unwrap();
    Router::new()
        .route("/users", axum::routing::get(list))
        .layer(GovernorLayer { config: conf })
}`
	ents := extract(t, "custom_rust_rate_limit", fi("main.rs", "rust", src))
	e := findRustRateLimit(ents, func(s entitySummary) bool {
		return s.Props["rate_limit_source"] == "tower_governor" && s.Props["rate_limit_scope"] == "router"
	})
	if e == nil {
		t.Fatalf("expected tower_governor router rate-limit entity, got %+v", ents)
	}
	if e.Props["rate_limited"] != "true" {
		t.Errorf("rate_limited = %q, want true", e.Props["rate_limited"])
	}
	if e.Props["rate_limit"] != "5/s" {
		t.Errorf("rate_limit = %q, want 5/s", e.Props["rate_limit"])
	}
	if e.Props["limit"] != "5" {
		t.Errorf("limit = %q, want 5", e.Props["limit"])
	}
	if e.Props["period"] != "1" {
		t.Errorf("period = %q, want 1", e.Props["period"])
	}
	if e.Props["rate_limit_burst"] != "10" {
		t.Errorf("rate_limit_burst = %q, want 10", e.Props["rate_limit_burst"])
	}
	if e.Props["framework"] != "axum" {
		t.Errorf("framework = %q, want axum", e.Props["framework"])
	}
}

// --- 2. actix-governor: per_second(2) + burst_size(5) -----------------------

func TestRustRateLimit_ActixGovernor(t *testing.T) {
	src := `
use actix_web::{App, HttpServer};
use actix_governor::{Governor, GovernorConfigBuilder};

fn build() -> App<()> {
    let governor_conf = GovernorConfigBuilder::default()
        .per_second(2)
        .burst_size(5)
        .finish()
        .unwrap();
    App::new()
        .wrap(Governor::new(&governor_conf))
}`
	ents := extract(t, "custom_rust_rate_limit", fi("server.rs", "rust", src))
	e := findRustRateLimit(ents, func(s entitySummary) bool {
		return s.Props["rate_limit_source"] == "actix_governor"
	})
	if e == nil {
		t.Fatalf("expected actix_governor rate-limit entity, got %+v", ents)
	}
	if e.Props["rate_limit_scope"] != "app" {
		t.Errorf("rate_limit_scope = %q, want app", e.Props["rate_limit_scope"])
	}
	if e.Props["rate_limit"] != "2/s" {
		t.Errorf("rate_limit = %q, want 2/s", e.Props["rate_limit"])
	}
	if e.Props["limit"] != "2" {
		t.Errorf("limit = %q, want 2", e.Props["limit"])
	}
	if e.Props["rate_limit_burst"] != "5" {
		t.Errorf("rate_limit_burst = %q, want 5", e.Props["rate_limit_burst"])
	}
	if e.Props["framework"] != "actix-web" {
		t.Errorf("framework = %q, want actix-web", e.Props["framework"])
	}
}

// --- 3. tower::limit RateLimitLayer::new(100, Duration::from_secs(60)) -------

func TestRustRateLimit_TowerRateLimitLayer(t *testing.T) {
	src := `
use tower::{ServiceBuilder, limit::RateLimitLayer};
use std::time::Duration;

fn svc() {
    let _ = ServiceBuilder::new()
        .layer(RateLimitLayer::new(100, Duration::from_secs(60)));
}`
	ents := extract(t, "custom_rust_rate_limit", fi("svc.rs", "rust", src))
	e := findRustRateLimit(ents, func(s entitySummary) bool {
		return s.Props["rate_limit_source"] == "tower_ratelimit"
	})
	if e == nil {
		t.Fatalf("expected tower_ratelimit entity, got %+v", ents)
	}
	if e.Props["limit"] != "100" {
		t.Errorf("limit = %q, want 100", e.Props["limit"])
	}
	if e.Props["period"] != "60" {
		t.Errorf("period = %q, want 60", e.Props["period"])
	}
	if e.Props["rate_limit"] != "100/60s" {
		t.Errorf("rate_limit = %q, want 100/60s", e.Props["rate_limit"])
	}
	if e.Props["rate_limit_scope"] != "engine" {
		t.Errorf("rate_limit_scope = %q, want engine", e.Props["rate_limit_scope"])
	}
}

// --- 4. tower RateLimitLayer with a sub-second from_millis window ------------

func TestRustRateLimit_TowerRateLimitLayerMillis(t *testing.T) {
	src := `
use tower::limit::RateLimitLayer;
use std::time::Duration;
let layer = RateLimitLayer::new(50, Duration::from_millis(500));`
	ents := extract(t, "custom_rust_rate_limit", fi("svc.rs", "rust", src))
	e := findRustRateLimit(ents, func(s entitySummary) bool {
		return s.Props["rate_limit_source"] == "tower_ratelimit"
	})
	if e == nil {
		t.Fatalf("expected tower_ratelimit entity, got %+v", ents)
	}
	if e.Props["limit"] != "50" {
		t.Errorf("limit = %q, want 50", e.Props["limit"])
	}
	if e.Props["rate_limit"] != "50/500ms" {
		t.Errorf("rate_limit = %q, want 50/500ms", e.Props["rate_limit"])
	}
	// Sub-second window: period (whole seconds) is honestly omitted.
	if _, ok := e.Props["period"]; ok {
		t.Errorf("period unexpectedly set for sub-second window: %q", e.Props["period"])
	}
}

// --- 5. honest-partial: governor config with non-literal per_second ---------

func TestRustRateLimit_GovernorNonLiteralPartial(t *testing.T) {
	src := `
use axum::Router;
use tower_governor::{governor::GovernorConfigBuilder, GovernorLayer};

fn app(rps: u64) -> Router {
    let conf = GovernorConfigBuilder::default()
        .per_second(rps)
        .finish()
        .unwrap();
    Router::new().layer(GovernorLayer { config: conf })
}`
	ents := extract(t, "custom_rust_rate_limit", fi("main.rs", "rust", src))
	e := findRustRateLimit(ents, func(s entitySummary) bool {
		return s.Props["rate_limit_source"] == "tower_governor"
	})
	if e == nil {
		t.Fatalf("expected tower_governor entity, got %+v", ents)
	}
	// rate_limited is stamped; the numeric rate is honestly OMITTED (non-literal).
	if e.Props["rate_limited"] != "true" {
		t.Errorf("rate_limited = %q, want true", e.Props["rate_limited"])
	}
	if _, ok := e.Props["rate_limit"]; ok {
		t.Errorf("rate_limit should be omitted for non-literal per_second, got %q", e.Props["rate_limit"])
	}
	if _, ok := e.Props["limit"]; ok {
		t.Errorf("limit should be omitted for non-literal per_second, got %q", e.Props["limit"])
	}
}

// --- 6. governor config builder with no bound layer/wrap -> engine scope -----

func TestRustRateLimit_GovernorConfigUnbound(t *testing.T) {
	src := `
use tower_governor::governor::GovernorConfigBuilder;
fn make_conf() {
    let _conf = GovernorConfigBuilder::default()
        .per_second(3)
        .burst_size(7)
        .finish()
        .unwrap();
}`
	ents := extract(t, "custom_rust_rate_limit", fi("conf.rs", "rust", src))
	e := findRustRateLimit(ents, func(s entitySummary) bool {
		return s.Props["rate_limit_scope"] == "engine" && s.Props["rate_limit_source"] == "tower_governor"
	})
	if e == nil {
		t.Fatalf("expected unbound governor config (engine scope), got %+v", ents)
	}
	if e.Props["rate_limit"] != "3/s" {
		t.Errorf("rate_limit = %q, want 3/s", e.Props["rate_limit"])
	}
	if e.Props["rate_limit_burst"] != "7" {
		t.Errorf("rate_limit_burst = %q, want 7", e.Props["rate_limit_burst"])
	}
}

// --- 7. NEGATIVE: a plain axum route with no governor/rate layer ------------

func TestRustRateLimit_PlainRouteNoStamp(t *testing.T) {
	src := `
use axum::{Router, routing::get};
fn app() -> Router {
    Router::new().route("/health", get(|| async { "ok" }))
}`
	ents := extract(t, "custom_rust_rate_limit", fi("main.rs", "rust", src))
	if anyRustRateLimit(ents) {
		t.Errorf("plain route must not stamp a rate limit, got %+v", ents)
	}
}

// --- 8. NEGATIVE: a non-rate tower layer (CorsLayer / TraceLayer) -----------

func TestRustRateLimit_NonRateLayerNoStamp(t *testing.T) {
	src := `
use axum::Router;
use tower_http::cors::CorsLayer;
use tower_http::trace::TraceLayer;
fn app() -> Router {
    Router::new()
        .layer(CorsLayer::permissive())
        .layer(TraceLayer::new_for_http())
}`
	ents := extract(t, "custom_rust_rate_limit", fi("main.rs", "rust", src))
	if anyRustRateLimit(ents) {
		t.Errorf("non-rate tower layer must not stamp a rate limit, got %+v", ents)
	}
}
