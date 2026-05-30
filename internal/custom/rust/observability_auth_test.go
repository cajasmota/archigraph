package rust_test

// observability_auth_test.go — tests for custom_rust_observability and
// custom_rust_auth extractors (issue #3269).

import (
	"os"
	"testing"
)

// ---------------------------------------------------------------------------
// helpers (reuse the fi/extract/containsEntity helpers from extractors_test.go)
// ---------------------------------------------------------------------------

func readFixture(t *testing.T, path string) string {
	t.Helper()
	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("readFixture %q: %v", path, err)
	}
	return string(b)
}

// ---------------------------------------------------------------------------
// Observability — logging signals
// ---------------------------------------------------------------------------

func TestRustObs_TracingInfoMacro(t *testing.T) {
	src := `
use axum::Router;
fn handler() {
    tracing::info!("user logged in");
    tracing::warn!("low memory");
}
`
	ents := extract(t, "custom_rust_observability", fi("handler.rs", "rust", src))
	if !containsEntity(ents, "SCOPE.Pattern", "obs:logging:tracing_macro:info") {
		t.Error("expected obs:logging:tracing_macro:info")
	}
	if !containsEntity(ents, "SCOPE.Pattern", "obs:logging:tracing_macro:warn") {
		t.Error("expected obs:logging:tracing_macro:warn")
	}
}

func TestRustObs_InstrumentAttribute(t *testing.T) {
	src := `
use axum::Router;
#[instrument]
async fn my_handler() {}
`
	ents := extract(t, "custom_rust_observability", fi("handler.rs", "rust", src))
	found := false
	for _, e := range ents {
		if e.Kind == "SCOPE.Pattern" && e.Name != "" &&
			contains(e.Name, "instrument") {
			found = true
			break
		}
	}
	if !found {
		t.Error("expected instrument entity")
	}
}

// ---------------------------------------------------------------------------
// Observability — metrics signals
// ---------------------------------------------------------------------------

func TestRustObs_MetricsMacro(t *testing.T) {
	src := `
use axum::Router;
fn record() {
    metrics::counter!("requests_total", 1);
    metrics::histogram!("latency_seconds", 0.05);
}
`
	ents := extract(t, "custom_rust_observability", fi("metrics.rs", "rust", src))
	if !containsEntity(ents, "SCOPE.Pattern", "obs:metrics:metrics_macro:counter") {
		t.Error("expected obs:metrics:metrics_macro:counter")
	}
	if !containsEntity(ents, "SCOPE.Pattern", "obs:metrics:metrics_macro:histogram") {
		t.Error("expected obs:metrics:metrics_macro:histogram")
	}
}

func TestRustObs_PrometheusTypes(t *testing.T) {
	src := `
use axum::Router;
use prometheus::{IntCounter, Histogram};
fn setup() {
    let counter = prometheus::IntCounter::new("reqs", "help").unwrap();
    let hist = prometheus::Histogram::with_opts(opts).unwrap();
}
`
	ents := extract(t, "custom_rust_observability", fi("prom.rs", "rust", src))
	found := false
	for _, e := range ents {
		if e.Kind == "SCOPE.Pattern" && contains(e.Name, "prometheus") {
			found = true
			break
		}
	}
	if !found {
		t.Error("expected prometheus metric entity")
	}
}

// ---------------------------------------------------------------------------
// Observability — tracing signals
// ---------------------------------------------------------------------------

func TestRustObs_OtelTracer(t *testing.T) {
	src := `
use axum::Router;
fn setup() {
    let tracer = opentelemetry::global::tracer("my_service");
}
`
	ents := extract(t, "custom_rust_observability", fi("tracing.rs", "rust", src))
	found := false
	for _, e := range ents {
		if e.Kind == "SCOPE.Pattern" && contains(e.Name, "otel_tracer") {
			found = true
			break
		}
	}
	if !found {
		t.Error("expected otel_tracer entity")
	}
}

// ---------------------------------------------------------------------------
// Observability — fixture-based test
// ---------------------------------------------------------------------------

func TestRustObs_AxumFixture(t *testing.T) {
	src := readFixture(t, "testdata/axum_observability.rs")
	ents := extract(t, "custom_rust_observability", fi("axum_observability.rs", "rust", src))
	if len(ents) == 0 {
		t.Error("expected entities from axum observability fixture")
	}
	// Should detect tracing::info!
	found := false
	for _, e := range ents {
		if e.Kind == "SCOPE.Pattern" && contains(e.Name, "tracing_macro") {
			found = true
			break
		}
	}
	if !found {
		t.Error("expected tracing_macro entity from fixture")
	}
}

func TestRustObs_RocketFixture(t *testing.T) {
	src := readFixture(t, "testdata/rocket_observability.rs")
	ents := extract(t, "custom_rust_observability", fi("rocket_observability.rs", "rust", src))
	if len(ents) == 0 {
		t.Error("expected entities from rocket observability fixture")
	}
}

// ---------------------------------------------------------------------------
// Observability — no match
// ---------------------------------------------------------------------------

func TestRustObs_NoMatch(t *testing.T) {
	src := `
use axum::Router;
fn plain() -> u32 { 42 }
`
	ents := extract(t, "custom_rust_observability", fi("plain.rs", "rust", src))
	if len(ents) != 0 {
		t.Errorf("expected no obs entities for plain file, got %d", len(ents))
	}
}

func TestRustObs_WrongLang(t *testing.T) {
	src := `tracing::info!("hello");`
	ents := extract(t, "custom_rust_observability", fi("file.go", "go", src))
	if len(ents) != 0 {
		t.Errorf("expected no entities for wrong language, got %d", len(ents))
	}
}

// ---------------------------------------------------------------------------
// Auth — JWT signals
// ---------------------------------------------------------------------------

func TestRustAuth_Jsonwebtoken(t *testing.T) {
	src := `
use actix_web::App;
use jsonwebtoken::{decode, encode, DecodingKey, Validation};
fn validate(token: &str) {
    let key = DecodingKey::from_secret(b"secret");
    decode::<Claims>(token, &key, &Validation::default()).unwrap();
}
`
	ents := extract(t, "custom_rust_auth", fi("auth.rs", "rust", src))
	found := false
	for _, e := range ents {
		if e.Kind == "SCOPE.Pattern" && contains(e.Name, "jwt") {
			found = true
			break
		}
	}
	if !found {
		t.Error("expected jwt auth entity")
	}
}

func TestRustAuth_ActixWebHttpAuth(t *testing.T) {
	src := `
use actix_web::App;
use actix_web_httpauth::middleware::HttpAuthentication;
fn setup() {
    let mw = HttpAuthentication::bearer(validator);
}
`
	ents := extract(t, "custom_rust_auth", fi("auth.rs", "rust", src))
	found := false
	for _, e := range ents {
		if e.Kind == "SCOPE.Pattern" && contains(e.Name, "auth") {
			found = true
			break
		}
	}
	if !found {
		t.Error("expected auth entity for actix-web-httpauth")
	}
}

// ---------------------------------------------------------------------------
// Auth — middleware signals
// ---------------------------------------------------------------------------

func TestRustAuth_ActixWrap(t *testing.T) {
	src := `
use actix_web::App;
fn setup() {
    App::new().wrap(AuthMiddleware::new());
}
`
	ents := extract(t, "custom_rust_auth", fi("app.rs", "rust", src))
	found := false
	for _, e := range ents {
		if e.Kind == "SCOPE.Pattern" && contains(e.Name, "middleware:wrap") {
			found = true
			break
		}
	}
	if !found {
		t.Error("expected middleware:wrap entity")
	}
}

func TestRustAuth_AxumLayer(t *testing.T) {
	src := `
use axum::Router;
fn setup() {
    Router::new().layer(TraceLayer::new_for_http());
}
`
	ents := extract(t, "custom_rust_auth", fi("router.rs", "rust", src))
	found := false
	for _, e := range ents {
		if e.Kind == "SCOPE.Pattern" && contains(e.Name, "middleware:layer") {
			found = true
			break
		}
	}
	if !found {
		t.Error("expected middleware:layer entity")
	}
}

func TestRustAuth_TideMiddleware(t *testing.T) {
	src := `
use tide::Server;
fn setup(mut app: tide::Server<()>) {
    app.middleware(CorsMiddleware::new());
}
`
	ents := extract(t, "custom_rust_auth", fi("app.rs", "rust", src))
	found := false
	for _, e := range ents {
		if e.Kind == "SCOPE.Pattern" && contains(e.Name, "middleware:middleware") {
			found = true
			break
		}
	}
	if !found {
		t.Error("expected middleware:middleware entity for tide")
	}
}

func TestRustAuth_WarpFilter(t *testing.T) {
	src := `
use warp::Filter;
fn setup() {
    let log = warp::log("api");
    let cors = warp::cors().allow_any_origin();
}
`
	ents := extract(t, "custom_rust_auth", fi("filters.rs", "rust", src))
	found := false
	for _, e := range ents {
		if e.Kind == "SCOPE.Pattern" && contains(e.Name, "middleware:filter") {
			found = true
			break
		}
	}
	if !found {
		t.Error("expected middleware:filter entity for warp")
	}
}

// ---------------------------------------------------------------------------
// Auth — fixture-based
// ---------------------------------------------------------------------------

func TestRustAuth_ActixFixture(t *testing.T) {
	src := readFixture(t, "testdata/actix_auth.rs")
	ents := extract(t, "custom_rust_auth", fi("actix_auth.rs", "rust", src))
	if len(ents) == 0 {
		t.Error("expected entities from actix auth fixture")
	}
}

// ---------------------------------------------------------------------------
// Auth — no match
// ---------------------------------------------------------------------------

func TestRustAuth_NoMatch(t *testing.T) {
	src := `
use actix_web::App;
fn plain() -> u32 { 42 }
`
	ents := extract(t, "custom_rust_auth", fi("plain.rs", "rust", src))
	if len(ents) != 0 {
		t.Errorf("expected no auth entities for plain file, got %d", len(ents))
	}
}

func TestRustAuth_WrongLang(t *testing.T) {
	src := `jsonwebtoken::decode(...)`
	ents := extract(t, "custom_rust_auth", fi("file.py", "python", src))
	if len(ents) != 0 {
		t.Errorf("expected no entities for wrong language, got %d", len(ents))
	}
}

// ---------------------------------------------------------------------------
// helper: string contains
// ---------------------------------------------------------------------------

func contains(s, sub string) bool {
	return len(s) >= len(sub) && (s == sub || len(sub) == 0 ||
		func() bool {
			for i := 0; i <= len(s)-len(sub); i++ {
				if s[i:i+len(sub)] == sub {
					return true
				}
			}
			return false
		}())
}
