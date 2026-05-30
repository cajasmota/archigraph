package rust

// observability.go — framework-agnostic observability scanner for Rust HTTP
// services (issue #3269). Detects three families of observability
// instrumentation across the 10 Rust HTTP frameworks:
//
//   - logging  : tracing::info!/warn!/error!/debug! macro calls,
//                tracing::span! / tracing::event! constructs, and the
//                #[instrument] attribute.
//   - metrics  : metrics::counter!/histogram!/gauge! macro calls, and
//                prometheus::Counter/Histogram/Gauge/IntCounter declarations
//                (prometheus crate).
//   - tracing  : opentelemetry::global::tracer(), otel span start
//                (tracer.start()), #[instrument] (already in logging too —
//                counted for both since it serves both purposes).
//
// Honesty:
//
//	partial — detection is a heuristic regex/identifier match on source text.
//	It does NOT perform import-resolution or data-flow analysis to confirm a
//	value is wired into a request handler/middleware, and it does not bind a
//	logger/metric/span to a specific route. Fixtures prove the detection
//	surface; wiring to routes requires semantic analysis beyond this scanner.
//
// Framework attribution: the scanner attributes files to a Rust HTTP
// framework using framework-specific import/usage markers (use actix_web, use
// axum, use rocket, etc.). A file with no recognised framework marker still
// emits entities but with framework="" so they are not credited to a
// per-framework cell.

import (
	"context"
	"regexp"
	"strings"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/trace"

	"github.com/cajasmota/archigraph/internal/extractor"
	"github.com/cajasmota/archigraph/internal/types"
)

func init() {
	extractor.Register("custom_rust_observability", &rustObsExtractor{})
}

type rustObsExtractor struct{}

func (e *rustObsExtractor) Language() string { return "custom_rust_observability" }

// ---------------------------------------------------------------------------
// Framework detection
// ---------------------------------------------------------------------------

// rustObsFrameworkMarker maps a Rust HTTP framework name to a regex that,
// when it matches the file source, attributes the file to that framework.
var rustObsFrameworkMarkers = []struct {
	name string
	re   *regexp.Regexp
}{
	{"actix", regexp.MustCompile(`\buse\s+actix_web\b|actix_web::`)},
	{"axum", regexp.MustCompile(`\buse\s+axum\b|axum::`)},
	{"rocket", regexp.MustCompile(`\buse\s+rocket\b|rocket::`)},
	{"poem", regexp.MustCompile(`\buse\s+poem\b|poem::`)},
	{"salvo", regexp.MustCompile(`\buse\s+salvo\b|salvo::`)},
	{"tide", regexp.MustCompile(`\buse\s+tide\b|tide::`)},
	{"warp", regexp.MustCompile(`\buse\s+warp\b|warp::`)},
	{"tower", regexp.MustCompile(`\buse\s+tower\b|tower::`)},
	{"hyper", regexp.MustCompile(`\buse\s+hyper\b|hyper::`)},
	{"gotham", regexp.MustCompile(`\buse\s+gotham\b|gotham::`)},
}

func detectRustObsFramework(src string) string {
	for _, m := range rustObsFrameworkMarkers {
		if m.re.MatchString(src) {
			return m.name
		}
	}
	return ""
}

// ---------------------------------------------------------------------------
// Signal catalogs
// ---------------------------------------------------------------------------

type rustObsSignal struct {
	re      *regexp.Regexp
	otype   string // logging | metrics | tracing
	subtype string // e.g. tracing_macro | instrument | prometheus | otel_span
	// nameGroup: which submatch carries the distinguishing detail (0=whole match)
	nameGroup int
}

var rustObsSignals = []rustObsSignal{
	// --- logging: tracing crate macros ------------------------------------
	{regexp.MustCompile(`\btracing::(info|warn|error|debug|trace)!\s*\(`), "logging", "tracing_macro", 1},
	{regexp.MustCompile(`\btracing::span!\s*\(`), "logging", "tracing_span", 0},
	{regexp.MustCompile(`\btracing::event!\s*\(`), "logging", "tracing_event", 0},
	// #[instrument] attribute — serves both logging and tracing
	{regexp.MustCompile(`#\[(?:tracing::)?instrument(?:\s*\([^)]*\))?\]`), "logging", "instrument", 0},

	// --- metrics: metrics crate macros -----------------------------------
	{regexp.MustCompile(`\bmetrics::(counter|histogram|gauge)!\s*\(`), "metrics", "metrics_macro", 1},
	// prometheus crate: register_counter / register_histogram / register_gauge
	{regexp.MustCompile(`\bprometheus::(?:IntCounter|Counter|Histogram|Gauge|CounterVec|HistogramVec|GaugeVec)\b`), "metrics", "prometheus", 0},
	// prometheus register_* helpers
	{regexp.MustCompile(`\b(?:register_counter|register_histogram|register_gauge)(?:_with_registry)?\s*!\s*\(`), "metrics", "prometheus_macro", 0},

	// --- tracing: opentelemetry crate ------------------------------------
	{regexp.MustCompile(`\bopentelemetry(?:_sdk)?::global::tracer\s*\(`), "tracing", "otel_tracer", 0},
	{regexp.MustCompile(`\bopentelemetry(?:_sdk)?::[a-z_]+::tracer\s*\(`), "tracing", "otel_tracer", 0},
	// tracer.start() / span builder
	{regexp.MustCompile(`\btracer\.start(?:_with_context)?\s*\(\s*(?:[^,)]+,\s*)?"([^"]+)"`), "tracing", "otel_span_start", 1},
	// #[instrument] again — also a tracing signal
	{regexp.MustCompile(`#\[(?:tracing::)?instrument(?:\s*\([^)]*\))?\]`), "tracing", "instrument", 0},
}

func (e *rustObsExtractor) Extract(ctx context.Context, file extractor.FileInput) ([]types.EntityRecord, error) {
	tracer := otel.Tracer("archigraph/custom/rust")
	_, span := tracer.Start(ctx, "indexer.rust_obs_extractor.extract",
		trace.WithAttributes(
			attribute.String("language", file.Language),
			attribute.String("file_path", file.Path),
		),
	)
	defer span.End()

	if len(file.Content) == 0 {
		return nil, nil
	}
	if file.Language != "rust" {
		return nil, nil
	}

	src := string(file.Content)
	framework := detectRustObsFramework(src)

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

	for _, sig := range rustObsSignals {
		for _, m := range sig.re.FindAllStringSubmatchIndex(src, -1) {
			detail := ""
			if sig.nameGroup > 0 && len(m) >= (sig.nameGroup+1)*2 {
				start := m[sig.nameGroup*2]
				end := m[sig.nameGroup*2+1]
				if start >= 0 && end >= 0 {
					detail = src[start:end]
				}
			}
			if detail == "" {
				detail = src[m[0]:m[1]]
			}
			detail = strings.TrimSpace(detail)

			name := "obs:" + sig.otype + ":" + sig.subtype + ":" + detail
			ent := makeEntity(name, "SCOPE.Pattern", "", file.Path, file.Language, lineOf(src, m[0]))
			setProps(&ent,
				"framework", framework,
				"provenance", rustObsProvenance(framework, sig.otype),
				"pattern_kind", "observability",
				"observability_type", sig.otype,
				"observability_subtype", sig.subtype,
			)
			add(ent)
		}
	}

	span.SetAttributes(
		attribute.String("framework", framework),
		attribute.Int("entity_count", len(entities)),
	)
	return entities, nil
}

func rustObsProvenance(framework, otype string) string {
	fw := strings.ToUpper(framework)
	if fw == "" {
		fw = "RUST"
	}
	return "INFERRED_FROM_" + fw + "_" + strings.ToUpper(otype)
}
