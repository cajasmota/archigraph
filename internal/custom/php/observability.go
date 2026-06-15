// observability.go — PHP observability extractor (log/metric/trace).
//
// Covers the Observability lane for lang.php.framework.laravel and
// lang.php.framework.symfony. Replaces the coarse one-entity-per-file
// stub that was previously in frameworks.go (custom_php_observability)
// with per-call-site extraction that matches the bar set by the Java,
// Python, and Ruby observability extractors.
//
// log_extraction
//
//	Laravel:  Log::info/warning/error/debug/critical/alert/emergency/notice,
//	          logger()->info (and all other log levels),
//	          \Log::channel('name')->info,
//	          Monolog \$logger->info (and all PSR-3 levels).
//	Symfony:  PSR-3 $this->logger->info (injected LoggerInterface),
//	          $logger->info, monolog.channel.X calls detected via use-site.
//
// metric_extraction
//
//	Laravel + Symfony: prometheus_client_php Counter/Gauge/Histogram/Summary
//	                   call-sites via new Counter/Gauge/Histogram/Summary pattern,
//	                   StatsD (League\StatsD or Domnikl\Statsd) $statsd->increment etc.
//	                   Laravel-Metrics facade (if present).
//
// trace_extraction
//
//	Laravel + Symfony: OpenTelemetry PHP SDK — CachedInstrumentation / Globals::tracerProvider,
//	                   $tracer->spanBuilder/startSpan call-sites, Span creation,
//	                   Symfony Stopwatch component — $stopwatch->start/stop/lap.
//
// Detection is import/use-statement + call-site heuristic. The extractor
// identifies library use declarations and canonical call-site patterns but
// does NOT perform cross-file dataflow (e.g. does not follow the injected
// LoggerInterface instance from DI container to handler). All cells
// therefore remain `partial`, matching the honesty discipline established
// by the Java, Python, and Ruby extractors.
//
// Extractor key: "custom_php_obs_laravel_symfony" — registered as a dedicated
// extractor. The coarse custom_php_observability stub in frameworks.go
// continues to run for the remaining PHP frameworks; this extractor
// deepens detection specifically for Laravel + Symfony call-sites.
//
// Part of issue #3400.
package php

import (
	"context"
	"regexp"
	"strings"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/trace"

	"github.com/cajasmota/grafel/internal/extractor"
	"github.com/cajasmota/grafel/internal/types"
)

func init() {
	extractor.Register("custom_php_obs_laravel_symfony", &phpObsExtractor{})
}

type phpObsExtractor struct{}

func (e *phpObsExtractor) Language() string { return "custom_php_obs_laravel_symfony" }

// ---------------------------------------------------------------------------
// Compiled regexes — log_extraction
// ---------------------------------------------------------------------------

var (
	// Laravel Log facade: Log::info(...), \Log::error(...), Log::channel('name')->debug(...)
	phpLaravelLogFacadeRe = regexp.MustCompile(
		`(?m)\\?Log::(debug|info|notice|warning|error|critical|alert|emergency)\s*\(`)

	// Log::channel('channel-name')  — captures the channel name
	phpLaravelLogChannelRe = regexp.MustCompile(
		`(?m)\\?Log::channel\s*\(\s*['"]([^'"]+)['"]\s*\)`)

	// logger() helper: logger()->info(...), logger('message') standalone
	phpLaravelLoggerHelperRe = regexp.MustCompile(
		`(?m)\blogger\s*\(\s*\)\s*->(debug|info|notice|warning|error|critical|alert|emergency)\s*\(`)

	// logger('message') as a standalone call (shorthand for Log::info)
	phpLaravelLoggerShorthandRe = regexp.MustCompile(
		`(?m)\blogger\s*\(\s*['"][^'"]+['"]\s*(?:,\s*\[[^\]]*\])?\s*\)`)

	// PSR-3 / Monolog injected logger: $logger->info(...), $this->logger->error(...)
	// Covers: $logger->, $this->logger->, $this->loggerInterface->
	phpMonologCallRe = regexp.MustCompile(
		`(?m)\$(?:this->)?(?:logger|log|monolog)\s*->(debug|info|notice|warning|error|critical|alert|emergency)\s*\(`)

	// use Monolog\Logger or use Psr\Log\LoggerInterface — file-level signal
	phpMonologUseRe = regexp.MustCompile(
		`(?m)\buse\s+(?:Monolog\\(?:Logger|Handler\\[A-Za-z]+)|Psr\\Log\\(?:LoggerInterface|AbstractLogger|NullLogger))\s*;`)

	// Symfony: $this->logger->info injected via LoggerInterface (same pattern as Monolog above)
	// Additional Symfony-specific: $this->logger->log(LogLevel::WARNING, ...)
	phpSymfonyLogLevelRe = regexp.MustCompile(
		`(?m)\$(?:this->)?logger\s*->\s*log\s*\(\s*(?:LogLevel::)?(\w+)\s*,`)

	// use Symfony\Component\HttpKernel\Log\Logger — Symfony native logger
	phpSymfonyLoggerUseRe = regexp.MustCompile(
		`(?m)\buse\s+Symfony\\Component\\(?:HttpKernel\\Log\\Logger|HttpFoundation\\[A-Za-z]+)\s*;`)
)

// ---------------------------------------------------------------------------
// Compiled regexes — metric_extraction
// ---------------------------------------------------------------------------

var (
	// prometheus_client_php: new Counter(...), new Gauge(...), new Histogram(...), new Summary(...)
	phpPrometheusNewRe = regexp.MustCompile(
		`(?m)\bnew\s+(Counter|Gauge|Histogram|Summary)\s*\(`)

	// use Prometheus\...: use Prometheus\Counter, use Prometheus\Gauge, etc.
	phpPrometheusUseRe = regexp.MustCompile(
		`(?m)\buse\s+Prometheus\\(Counter|Gauge|Histogram|Summary|CollectorRegistry|RenderTextFormat)\s*;`)

	// $registry->registerCounter / $registry->registerGauge / etc.
	phpPrometheusRegistryRe = regexp.MustCompile(
		`(?m)\$\w+\s*->\s*register(Counter|Gauge|Histogram|Summary)\s*\(`)

	// StatsD: League\StatsD or Domnikl\Statsd
	// $statsd->increment / $statsd->gauge / $statsd->timing etc.
	phpStatsdCallRe = regexp.MustCompile(
		`(?m)\$\w+\s*->\s*(increment|decrement|gauge|timing|histogram|set|event|updateStats)\s*\(\s*['"]([^'"]+)['"]`)

	// use League\StatsD or use Domnikl\Statsd (with optional \Client alias)
	phpStatsdUseRe = regexp.MustCompile(
		`(?m)\buse\s+(?:League\\StatsD(?:\\Client)?|Domnikl\\Statsd(?:\\Client)?)\s*(?:as\s+\w+\s*)?;`)

	// Laravel Metrics facade (optional package illuminate/metrics or spatie/laravel-metrics)
	phpLaravelMetricsRe = regexp.MustCompile(
		`(?m)\bMetrics::(counter|gauge|histogram|summary)\s*\(`)
)

// ---------------------------------------------------------------------------
// Compiled regexes — trace_extraction
// ---------------------------------------------------------------------------

var (
	// OpenTelemetry PHP SDK use statements
	phpOtelUseRe = regexp.MustCompile(
		`(?m)\buse\s+OpenTelemetry\\(?:API|SDK|Contrib)\\[A-Za-z\\]+\s*;`)

	// CachedInstrumentation / Globals::tracerProvider() — SDK bootstrap patterns
	// Note: no trailing \b — ) is not a word character, so \b after ) never matches
	phpOtelBootstrapRe = regexp.MustCompile(
		`(?m)(?:CachedInstrumentation|Globals::tracerProvider\s*\(\s*\)|SDK::builder\s*\(\s*\))`)

	// $tracer->spanBuilder('name') or $tracer->startSpan('name')
	// Also handles chained access: $this->tracer->spanBuilder('name')
	phpOtelSpanBuilderRe = regexp.MustCompile(
		`(?m)\$(?:\w+|this->\w+)\s*->\s*(?:spanBuilder|startSpan)\s*\(\s*['"]([^'"]+)['"]`)

	// $span->setStatus / $span->addEvent / $span->setAttribute — span lifecycle
	phpOtelSpanLifecycleRe = regexp.MustCompile(
		`(?m)\$\w+\s*->\s*(setStatus|addEvent|setAttribute|recordException|end)\s*\(`)

	// use OpenTelemetry\API\Trace\SpanKind etc.
	phpOtelTraceUseRe = regexp.MustCompile(
		`(?m)\buse\s+OpenTelemetry\\API\\Trace\\(Tracer|Span|SpanInterface|SpanKind|StatusCode|TracerInterface)\s*;`)

	// Symfony Stopwatch: $stopwatch->start('name') / $stopwatch->stop('name')
	phpSymfonyStopwatchRe = regexp.MustCompile(
		`(?m)\$\w+\s*->\s*(start|stop|lap)\s*\(\s*['"]([^'"]+)['"]`)

	// use Symfony\Component\Stopwatch\Stopwatch
	phpSymfonyStopwatchUseRe = regexp.MustCompile(
		`(?m)\buse\s+Symfony\\Component\\Stopwatch\\Stopwatch\s*;`)

	// DDTrace / Datadog APM for PHP
	// \DDTrace\trace_function / \DDTrace\trace_method
	phpDDTraceRe = regexp.MustCompile(
		`(?m)\\?DDTrace\\(trace_function|trace_method|start_span|active_span)\s*\(`)
)

// ---------------------------------------------------------------------------
// Extract
// ---------------------------------------------------------------------------

func (e *phpObsExtractor) Extract(ctx context.Context, file extractor.FileInput) ([]types.EntityRecord, error) {
	tracer := otel.Tracer("grafel/custom/php")
	_, span := tracer.Start(ctx, "indexer.phpObs_extractor.extract",
		trace.WithAttributes(
			attribute.String("language", file.Language),
			attribute.String("file_path", file.Path),
		),
	)
	defer span.End()

	if len(file.Content) == 0 || file.Language != "php" {
		return nil, nil
	}
	src := string(file.Content)

	// Fast guard: skip files with none of the known observability tokens.
	hasLog := strings.ContainsAny(src, "Ll") && (strings.Contains(src, "Log::") ||
		strings.Contains(src, "logger()") ||
		strings.Contains(src, "->logger") ||
		strings.Contains(src, "$logger->") ||
		strings.Contains(src, "Monolog") ||
		strings.Contains(src, "LoggerInterface") ||
		strings.Contains(src, "LogLevel"))

	hasMetric := strings.Contains(src, "Prometheus") ||
		strings.Contains(src, "StatsD") ||
		strings.Contains(src, "statsd") ||
		strings.Contains(src, "Metrics::") ||
		strings.Contains(src, "Counter") ||
		strings.Contains(src, "Gauge") ||
		strings.Contains(src, "Histogram")

	hasTrace := strings.Contains(src, "OpenTelemetry") ||
		strings.Contains(src, "Stopwatch") ||
		strings.Contains(src, "DDTrace") ||
		strings.Contains(src, "spanBuilder") ||
		strings.Contains(src, "startSpan")

	if !hasLog && !hasMetric && !hasTrace {
		return nil, nil
	}

	var out []types.EntityRecord
	seen := make(map[string]bool)
	add := func(ent types.EntityRecord) {
		key := ent.Kind + ":" + ent.Subtype + ":" + ent.Name + ":" + ent.SourceFile
		if !seen[key] {
			seen[key] = true
			out = append(out, ent)
		}
	}

	if hasLog {
		for _, ent := range phpObsExtractLog(src, file.Path) {
			add(ent)
		}
	}
	if hasMetric {
		for _, ent := range phpObsExtractMetric(src, file.Path) {
			add(ent)
		}
	}
	if hasTrace {
		for _, ent := range phpObsExtractTrace(src, file.Path) {
			add(ent)
		}
	}

	span.SetAttributes(attribute.Int("entity_count", len(out)))
	return out, nil
}

// ---------------------------------------------------------------------------
// log_extraction
// ---------------------------------------------------------------------------

func phpObsExtractLog(src, fp string) []types.EntityRecord {
	var out []types.EntityRecord

	// Laravel Log facade: Log::info / Log::error / \Log::warning etc.
	for _, idx := range phpLaravelLogFacadeRe.FindAllStringSubmatchIndex(src, -1) {
		level := src[idx[2]:idx[3]]
		ln := lineOf(src, idx[0])
		e := makeEntity("Log::"+level, string(types.EntityKindPattern), "log_statement", fp, "php", ln)
		setProps(&e, "signal", "log", "library", "laravel_log_facade",
			"log_level", level, "receiver", "Log")
		out = append(out, e)
	}

	// Log::channel('name') — channel configuration call-site
	for _, idx := range phpLaravelLogChannelRe.FindAllStringSubmatchIndex(src, -1) {
		channel := src[idx[2]:idx[3]]
		ln := lineOf(src, idx[0])
		e := makeEntity("Log::channel("+channel+")", string(types.EntityKindPattern), "log_statement", fp, "php", ln)
		setProps(&e, "signal", "log", "library", "laravel_log_facade",
			"kind", "channel_call", "channel", channel)
		out = append(out, e)
	}

	// logger() helper: logger()->info(...)
	for _, idx := range phpLaravelLoggerHelperRe.FindAllStringSubmatchIndex(src, -1) {
		level := src[idx[2]:idx[3]]
		ln := lineOf(src, idx[0])
		e := makeEntity("logger()->"+level, string(types.EntityKindPattern), "log_statement", fp, "php", ln)
		setProps(&e, "signal", "log", "library", "laravel_logger_helper",
			"log_level", level, "receiver", "logger()")
		out = append(out, e)
	}

	// logger('message') shorthand (only when no ->level call follows on same token)
	if phpLaravelLoggerShorthandRe.MatchString(src) && !phpLaravelLoggerHelperRe.MatchString(src) {
		loc := phpLaravelLoggerShorthandRe.FindStringIndex(src)
		ln := lineOf(src, loc[0])
		e := makeEntity("logger()", string(types.EntityKindPattern), "log_statement", fp, "php", ln)
		setProps(&e, "signal", "log", "library", "laravel_logger_helper",
			"kind", "shorthand_call")
		out = append(out, e)
	}

	// PSR-3 / Monolog injected logger: $logger->info, $this->logger->error
	for _, idx := range phpMonologCallRe.FindAllStringSubmatchIndex(src, -1) {
		level := src[idx[2]:idx[3]]
		ln := lineOf(src, idx[0])
		e := makeEntity("$logger->"+level, string(types.EntityKindPattern), "log_statement", fp, "php", ln)
		setProps(&e, "signal", "log", "library", "monolog_psr3",
			"log_level", level, "receiver", "$logger")
		out = append(out, e)
	}

	// use Monolog\... — file-level logger dependency signal
	if phpMonologUseRe.MatchString(src) && !phpMonologCallRe.MatchString(src) {
		loc := phpMonologUseRe.FindStringIndex(src)
		ln := lineOf(src, loc[0])
		e := makeEntity("Monolog\\Logger", string(types.EntityKindPattern), "logger", fp, "php", ln)
		setProps(&e, "signal", "log", "library", "monolog_psr3",
			"kind", "use_declaration")
		out = append(out, e)
	}

	// Symfony: $this->logger->log(LogLevel::WARNING, ...)
	for _, idx := range phpSymfonyLogLevelRe.FindAllStringSubmatchIndex(src, -1) {
		level := strings.ToLower(src[idx[2]:idx[3]])
		ln := lineOf(src, idx[0])
		e := makeEntity("$this->logger->log(LogLevel::"+strings.ToUpper(level)+")", string(types.EntityKindPattern), "log_statement", fp, "php", ln)
		setProps(&e, "signal", "log", "library", "symfony_logger",
			"log_level", level, "receiver", "$this->logger")
		out = append(out, e)
	}

	// use Symfony\Component\HttpKernel\Log\Logger
	if phpSymfonyLoggerUseRe.MatchString(src) && !phpMonologCallRe.MatchString(src) &&
		!phpLaravelLogFacadeRe.MatchString(src) {
		loc := phpSymfonyLoggerUseRe.FindStringIndex(src)
		ln := lineOf(src, loc[0])
		e := makeEntity("Symfony\\Logger", string(types.EntityKindPattern), "logger", fp, "php", ln)
		setProps(&e, "signal", "log", "library", "symfony_logger",
			"kind", "use_declaration")
		out = append(out, e)
	}

	return out
}

// ---------------------------------------------------------------------------
// metric_extraction
// ---------------------------------------------------------------------------

func phpObsExtractMetric(src, fp string) []types.EntityRecord {
	var out []types.EntityRecord

	// prometheus_client_php: new Counter / new Gauge / new Histogram / new Summary
	// Only emit when the Prometheus use-statement is present (reduces false positives)
	hasPrometheusUse := phpPrometheusUseRe.MatchString(src)
	for _, idx := range phpPrometheusNewRe.FindAllStringSubmatchIndex(src, -1) {
		meterType := src[idx[2]:idx[3]]
		ln := lineOf(src, idx[0])
		// Only emit when we have a Prometheus use-statement, or when the string
		// "Prometheus" appears in the file (require or FQCN usage)
		if !hasPrometheusUse && !strings.Contains(src, "Prometheus\\") &&
			!strings.Contains(src, "Prometheus::") {
			continue
		}
		e := makeEntity("new "+meterType, string(types.EntityKindPattern), "metric", fp, "php", ln)
		setProps(&e, "signal", "metric", "library", "prometheus_client_php",
			"metric_type", strings.ToLower(meterType), "kind", "instantiation")
		out = append(out, e)
	}

	// $registry->registerCounter / registerGauge / registerHistogram
	for _, idx := range phpPrometheusRegistryRe.FindAllStringSubmatchIndex(src, -1) {
		meterType := src[idx[2]:idx[3]]
		ln := lineOf(src, idx[0])
		e := makeEntity("register"+meterType, string(types.EntityKindPattern), "metric", fp, "php", ln)
		setProps(&e, "signal", "metric", "library", "prometheus_client_php",
			"metric_type", strings.ToLower(meterType), "kind", "register_call")
		out = append(out, e)
	}

	// use Prometheus\... without any further call-site
	if hasPrometheusUse && !phpPrometheusNewRe.MatchString(src) && !phpPrometheusRegistryRe.MatchString(src) {
		loc := phpPrometheusUseRe.FindStringIndex(src)
		ln := lineOf(src, loc[0])
		e := makeEntity("Prometheus", string(types.EntityKindPattern), "metric", fp, "php", ln)
		setProps(&e, "signal", "metric", "library", "prometheus_client_php",
			"kind", "use_declaration")
		out = append(out, e)
	}

	// StatsD: $statsd->increment/gauge/timing etc.
	hasStatsdUse := phpStatsdUseRe.MatchString(src)
	for _, idx := range phpStatsdCallRe.FindAllStringSubmatchIndex(src, -1) {
		method := src[idx[2]:idx[3]]
		metricName := src[idx[4]:idx[5]]
		ln := lineOf(src, idx[0])
		// Require either a StatsD use-statement or explicit statsd token in file
		if !hasStatsdUse && !strings.Contains(src, "StatsD") && !strings.Contains(src, "statsd") {
			continue
		}
		e := makeEntity(metricName, string(types.EntityKindPattern), "metric", fp, "php", ln)
		setProps(&e, "signal", "metric", "library", "statsd_php",
			"metric_type", phpStatsdMetricType(method), "metric_name", metricName)
		out = append(out, e)
	}

	// use League\StatsD (or Domnikl) without call-sites
	if hasStatsdUse && !phpStatsdCallRe.MatchString(src) {
		loc := phpStatsdUseRe.FindStringIndex(src)
		ln := lineOf(src, loc[0])
		e := makeEntity("StatsD", string(types.EntityKindPattern), "metric", fp, "php", ln)
		setProps(&e, "signal", "metric", "library", "statsd_php",
			"kind", "use_declaration")
		out = append(out, e)
	}

	// Laravel Metrics facade
	for _, idx := range phpLaravelMetricsRe.FindAllStringSubmatchIndex(src, -1) {
		meterType := src[idx[2]:idx[3]]
		ln := lineOf(src, idx[0])
		e := makeEntity("Metrics::"+meterType, string(types.EntityKindPattern), "metric", fp, "php", ln)
		setProps(&e, "signal", "metric", "library", "laravel_metrics",
			"metric_type", meterType)
		out = append(out, e)
	}

	return out
}

// ---------------------------------------------------------------------------
// trace_extraction
// ---------------------------------------------------------------------------

func phpObsExtractTrace(src, fp string) []types.EntityRecord {
	var out []types.EntityRecord

	hasOtelUse := phpOtelUseRe.MatchString(src) || phpOtelTraceUseRe.MatchString(src)
	hasOtelToken := strings.Contains(src, "OpenTelemetry") || hasOtelUse

	// OpenTelemetry SDK bootstrap: CachedInstrumentation / Globals::tracerProvider()
	if hasOtelToken && phpOtelBootstrapRe.MatchString(src) {
		for _, idx := range phpOtelBootstrapRe.FindAllStringIndex(src, -1) {
			token := src[idx[0]:idx[1]]
			if len(token) > 50 {
				token = token[:50]
			}
			ln := lineOf(src, idx[0])
			e := makeEntity(token, string(types.EntityKindPattern), "trace_span", fp, "php", ln)
			setProps(&e, "signal", "trace", "library", "opentelemetry_php",
				"kind", "bootstrap")
			out = append(out, e)
		}
	}

	// $tracer->spanBuilder('name') or $tracer->startSpan('name')
	if hasOtelToken {
		for _, idx := range phpOtelSpanBuilderRe.FindAllStringSubmatchIndex(src, -1) {
			spanName := src[idx[2]:idx[3]]
			ln := lineOf(src, idx[0])
			e := makeEntity(spanName, string(types.EntityKindPattern), "trace_span", fp, "php", ln)
			setProps(&e, "signal", "trace", "library", "opentelemetry_php",
				"kind", "span_builder", "span_name", spanName)
			out = append(out, e)
		}
	}

	// $span->setAttribute/addEvent/setStatus/recordException — span lifecycle markers
	if hasOtelToken {
		for _, idx := range phpOtelSpanLifecycleRe.FindAllStringSubmatchIndex(src, -1) {
			method := src[idx[2]:idx[3]]
			ln := lineOf(src, idx[0])
			e := makeEntity("$span->"+method, string(types.EntityKindPattern), "trace_span", fp, "php", ln)
			setProps(&e, "signal", "trace", "library", "opentelemetry_php",
				"kind", "span_lifecycle", "method", method)
			out = append(out, e)
		}
	}

	// use OpenTelemetry\... without any span builder call
	if hasOtelUse && !phpOtelSpanBuilderRe.MatchString(src) && !phpOtelBootstrapRe.MatchString(src) {
		loc := phpOtelUseRe.FindStringIndex(src)
		if loc == nil {
			loc = phpOtelTraceUseRe.FindStringIndex(src)
		}
		if loc != nil {
			ln := lineOf(src, loc[0])
			e := makeEntity("OpenTelemetry", string(types.EntityKindPattern), "trace_span", fp, "php", ln)
			setProps(&e, "signal", "trace", "library", "opentelemetry_php",
				"kind", "use_declaration")
			out = append(out, e)
		}
	}

	// Symfony Stopwatch: $stopwatch->start/stop/lap('name')
	hasStopwatchUse := phpSymfonyStopwatchUseRe.MatchString(src)
	hasStopwatchToken := hasStopwatchUse || strings.Contains(src, "Stopwatch")
	if hasStopwatchToken {
		for _, idx := range phpSymfonyStopwatchRe.FindAllStringSubmatchIndex(src, -1) {
			method := src[idx[2]:idx[3]]
			eventName := src[idx[4]:idx[5]]
			// Only emit stop/lap/start events that look like trace events
			if method != "start" && method != "stop" && method != "lap" {
				continue
			}
			ln := lineOf(src, idx[0])
			e := makeEntity(eventName, string(types.EntityKindPattern), "trace_span", fp, "php", ln)
			setProps(&e, "signal", "trace", "library", "symfony_stopwatch",
				"kind", "stopwatch_"+method, "event_name", eventName)
			out = append(out, e)
		}
		// File-level use-declaration when no call-sites found
		if hasStopwatchUse && !phpSymfonyStopwatchRe.MatchString(src) {
			loc := phpSymfonyStopwatchUseRe.FindStringIndex(src)
			ln := lineOf(src, loc[0])
			e := makeEntity("Stopwatch", string(types.EntityKindPattern), "trace_span", fp, "php", ln)
			setProps(&e, "signal", "trace", "library", "symfony_stopwatch",
				"kind", "use_declaration")
			out = append(out, e)
		}
	}

	// DDTrace: \DDTrace\trace_function / \DDTrace\trace_method
	for _, idx := range phpDDTraceRe.FindAllStringSubmatchIndex(src, -1) {
		fn := src[idx[2]:idx[3]]
		ln := lineOf(src, idx[0])
		e := makeEntity("DDTrace\\"+fn, string(types.EntityKindPattern), "trace_span", fp, "php", ln)
		setProps(&e, "signal", "trace", "library", "ddtrace_php",
			"kind", "trace_call")
		out = append(out, e)
	}

	return out
}

// ---------------------------------------------------------------------------
// Helper: StatsD metric-type normalisation
// ---------------------------------------------------------------------------

func phpStatsdMetricType(method string) string {
	switch method {
	case "increment", "decrement", "updateStats":
		return "counter"
	case "gauge":
		return "gauge"
	case "timing":
		return "timer"
	case "histogram":
		return "histogram"
	case "set":
		return "set"
	case "event":
		return "event"
	default:
		return method
	}
}
