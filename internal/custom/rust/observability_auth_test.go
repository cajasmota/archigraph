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
	if !containsEntity(ents, "SCOPE.Pattern", "obs:logging:tracing_macro:info:user logged in") {
		t.Error("expected obs:logging:tracing_macro:info:user logged in")
	}
	if !containsEntity(ents, "SCOPE.Pattern", "obs:logging:tracing_macro:warn:low memory") {
		t.Error("expected obs:logging:tracing_macro:warn:low memory")
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
	if !containsEntity(ents, "SCOPE.Pattern", "obs:metrics:metrics_macro:counter:requests_total") {
		t.Error("expected obs:metrics:metrics_macro:counter:requests_total")
	}
	if !containsEntity(ents, "SCOPE.Pattern", "obs:metrics:metrics_macro:histogram:latency_seconds") {
		t.Error("expected obs:metrics:metrics_macro:histogram:latency_seconds")
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

// obsNameOf returns the observability_name prop of the first entity whose Name
// matches, or "" if absent. Used by value-asserting tests.
func obsNameOf(ents []entitySummary, entName string) string {
	for _, e := range ents {
		if e.Name == entName {
			return e.Props["observability_name"]
		}
	}
	return ""
}

// ---------------------------------------------------------------------------
// Observability — metric_extraction (value-asserting; basis for `full`)
// ---------------------------------------------------------------------------

func TestRustObs_MetricsMacro_CapturesName_Issue3416(t *testing.T) {
	src := `
use axum::Router;
fn record() {
    metrics::counter!("http_requests_total", 1);
    gauge!("queue_depth", 7.0);
    metrics::histogram!("request_latency_seconds", 0.05);
}
`
	ents := extract(t, "custom_rust_observability", fi("m.rs", "rust", src))
	cases := map[string]string{
		"obs:metrics:metrics_macro:counter:http_requests_total":       "http_requests_total",
		"obs:metrics:metrics_macro:gauge:queue_depth":                 "queue_depth",
		"obs:metrics:metrics_macro:histogram:request_latency_seconds": "request_latency_seconds",
	}
	for name, want := range cases {
		if got := obsNameOf(ents, name); got != want {
			t.Errorf("metric %q: observability_name = %q, want %q", name, got, want)
		}
	}
}

func TestRustObs_PrometheusName_Issue3416(t *testing.T) {
	src := `
use axum::Router;
use prometheus::{IntCounter, Opts, register_counter};
fn setup() {
    let c = prometheus::IntCounter::new("api_calls", "total api calls").unwrap();
    register_counter!("jobs_processed", "jobs done").unwrap();
    let opts = Opts::new("build_info", "build metadata");
}
`
	ents := extract(t, "custom_rust_observability", fi("p.rs", "rust", src))
	for _, want := range []string{"api_calls", "jobs_processed", "build_info"} {
		found := false
		for _, e := range ents {
			if e.Props["observability_type"] == "metrics" && e.Props["observability_name"] == want {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("expected a metric entity with observability_name=%q", want)
		}
	}
}

func TestRustObs_OtelMeter_Issue3416(t *testing.T) {
	src := `
use axum::Router;
fn setup(meter: Meter) {
    let c = meter.u64_counter("orders_created");
    let h = meter.f64_histogram("payment_amount");
}
`
	ents := extract(t, "custom_rust_observability", fi("otelm.rs", "rust", src))
	for _, want := range []string{"orders_created", "payment_amount"} {
		found := false
		for _, e := range ents {
			if e.Props["observability_type"] == "metrics" && e.Props["observability_name"] == want {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("expected otel meter metric with observability_name=%q", want)
		}
	}
}

// ---------------------------------------------------------------------------
// Observability — trace_extraction (value-asserting; basis for `full`)
// ---------------------------------------------------------------------------

func TestRustObs_SpanName_Issue3416(t *testing.T) {
	src := `
use axum::Router;
use tracing::{span, info_span, Level};
fn work() {
    let s1 = span!(Level::INFO, "db_query");
    let s2 = info_span!("handle_request");
    let s3 = tracing::error_span!("recover_panic");
}
`
	ents := extract(t, "custom_rust_observability", fi("s.rs", "rust", src))
	cases := map[string]string{
		"obs:tracing:tracing_span:INFO:db_query":             "db_query",
		"obs:tracing:tracing_level_span:info:handle_request": "handle_request",
		"obs:tracing:tracing_level_span:error:recover_panic": "recover_panic",
	}
	for name, want := range cases {
		if got := obsNameOf(ents, name); got != want {
			t.Errorf("span %q: observability_name = %q, want %q", name, got, want)
		}
	}
}

func TestRustObs_OtelSpanName_Issue3416(t *testing.T) {
	src := `
use axum::Router;
fn setup() {
    let tracer = opentelemetry::global::tracer("checkout_service");
    let span = tracer.start("process_order");
    let b = tracer.span_builder("validate_cart");
}
`
	ents := extract(t, "custom_rust_observability", fi("ot.rs", "rust", src))
	for _, want := range []string{"checkout_service", "process_order", "validate_cart"} {
		found := false
		for _, e := range ents {
			if e.Props["observability_type"] == "tracing" && e.Props["observability_name"] == want {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("expected tracing entity with observability_name=%q", want)
		}
	}
}

// ---------------------------------------------------------------------------
// Observability — log crate + bare macros + event! + slog (call-site surface)
// ---------------------------------------------------------------------------

func TestRustObs_LogCrateAndVariants_Issue3416(t *testing.T) {
	src := `
use axum::Router;
fn handlers() {
    info!("bare tracing macro");
    log::error!("disk full");
    event!(Level::WARN, "deprecated path");
    slog::info!(logger, "slog message");
}
`
	ents := extract(t, "custom_rust_observability", fi("logs.rs", "rust", src))
	wantNames := []string{
		"obs:logging:tracing_macro_bare:info:bare tracing macro",
		"obs:logging:log_macro:error:disk full",
		"obs:logging:tracing_event:WARN:deprecated path",
		"obs:logging:slog_macro:info:slog message",
	}
	for _, n := range wantNames {
		if !containsEntity(ents, "SCOPE.Pattern", n) {
			t.Errorf("expected log entity %q", n)
		}
	}
	// library prop must be set per call site.
	for _, e := range ents {
		if e.Props["observability_type"] == "logging" && e.Props["observability_library"] == "" {
			t.Errorf("logging entity %q missing observability_library", e.Name)
		}
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
