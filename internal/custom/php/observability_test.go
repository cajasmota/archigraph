// observability_test.go — value-asserting tests for the phpObs_laravel_symfony
// extractor. Each test verifies that a concrete call-site pattern emits the
// expected entity (Kind + Name). Tests intentionally use realistic PHP
// snippets; no test forces a "full" status — detection is call-site heuristic.
//
// Part of issue #3400.
package php_test

import "testing"

// NOTE: fi(), extract(), containsEntity(), entitySummary are defined in
// extractors_test.go (same package php_test).

// ---------------------------------------------------------------------------
// log_extraction — Laravel Log facade
// ---------------------------------------------------------------------------

// TestPHPObsLaravelLogFacadeInfo verifies that Log::info(...) emits a
// per-call-site log_statement entity with name "Log::info".
func TestPHPObsLaravelLogFacadeInfo(t *testing.T) {
	src := `<?php
use Illuminate\Support\Facades\Log;

class UserController extends Controller
{
    public function login(Request $request): Response
    {
        Log::info('User logged in', ['user_id' => $request->user()->id]);
        return response()->json(['status' => 'ok']);
    }
}
`
	ents := extract(t, "phpObs_laravel_symfony", fi("app/Http/Controllers/UserController.php", "php", src))
	if !containsEntity(ents, "SCOPE.Pattern", "Log::info") {
		t.Error("expected Log::info log_statement entity")
	}
}

// TestPHPObsLaravelLogFacadeError verifies Log::error emits its own entity.
func TestPHPObsLaravelLogFacadeError(t *testing.T) {
	src := `<?php
Log::error('Payment failed', ['order_id' => $orderId, 'reason' => $e->getMessage()]);
`
	ents := extract(t, "phpObs_laravel_symfony", fi("app/Services/PaymentService.php", "php", src))
	if !containsEntity(ents, "SCOPE.Pattern", "Log::error") {
		t.Error("expected Log::error log_statement entity")
	}
}

// TestPHPObsLaravelLogFacadeMultiLevel verifies multiple log levels in one file
// each produce their own entity.
func TestPHPObsLaravelLogFacadeMultiLevel(t *testing.T) {
	src := `<?php
Log::debug('Processing item', ['id' => $id]);
Log::warning('Item is deprecated', ['item' => $name]);
Log::critical('Database connection failed');
`
	ents := extract(t, "phpObs_laravel_symfony", fi("app/Jobs/ProcessItem.php", "php", src))
	if !containsEntity(ents, "SCOPE.Pattern", "Log::debug") {
		t.Error("expected Log::debug entity")
	}
	if !containsEntity(ents, "SCOPE.Pattern", "Log::warning") {
		t.Error("expected Log::warning entity")
	}
	if !containsEntity(ents, "SCOPE.Pattern", "Log::critical") {
		t.Error("expected Log::critical entity")
	}
}

// TestPHPObsLaravelLogChannel verifies Log::channel('name') emits a channel
// call entity with the channel name embedded.
func TestPHPObsLaravelLogChannel(t *testing.T) {
	src := `<?php
Log::channel('slack')->critical('Server is down', ['host' => $host]);
Log::channel('daily')->info('Scheduled job completed');
`
	ents := extract(t, "phpObs_laravel_symfony", fi("app/Console/Commands/HealthCheck.php", "php", src))
	if !containsEntity(ents, "SCOPE.Pattern", "Log::channel(slack)") {
		t.Error("expected Log::channel(slack) entity")
	}
	if !containsEntity(ents, "SCOPE.Pattern", "Log::channel(daily)") {
		t.Error("expected Log::channel(daily) entity")
	}
}

// TestPHPObsLaravelLoggerHelper verifies logger()->info() and logger()->error()
// helper calls are extracted.
func TestPHPObsLaravelLoggerHelper(t *testing.T) {
	src := `<?php
public function store(Request $request): JsonResponse
{
    logger()->info('New order created', ['order' => $order->id]);
    logger()->error('Validation failed', $request->all());
    return response()->json($order);
}
`
	ents := extract(t, "phpObs_laravel_symfony", fi("app/Http/Controllers/OrderController.php", "php", src))
	if !containsEntity(ents, "SCOPE.Pattern", "logger()->info") {
		t.Error("expected logger()->info entity")
	}
	if !containsEntity(ents, "SCOPE.Pattern", "logger()->error") {
		t.Error("expected logger()->error entity")
	}
}

// TestPHPObsLaravelBackslashLogFacade verifies the fully-qualified \Log::info
// form is detected (common outside the Illuminate namespace).
func TestPHPObsLaravelBackslashLogFacade(t *testing.T) {
	src := `<?php
\Log::info('Starting import job');
\Log::warning('Import row skipped', ['row' => $rowNumber]);
`
	ents := extract(t, "phpObs_laravel_symfony", fi("app/Jobs/ImportJob.php", "php", src))
	if !containsEntity(ents, "SCOPE.Pattern", "Log::info") {
		t.Error("expected Log::info entity for \\Log::info call")
	}
	if !containsEntity(ents, "SCOPE.Pattern", "Log::warning") {
		t.Error("expected Log::warning entity for \\Log::warning call")
	}
}

// ---------------------------------------------------------------------------
// log_extraction — Monolog / PSR-3 injected logger
// ---------------------------------------------------------------------------

// TestPHPObsMonologInjectedLogger verifies $this->logger->info/error/warning
// PSR-3 call-sites are extracted.
func TestPHPObsMonologInjectedLogger(t *testing.T) {
	src := `<?php
use Monolog\Logger;
use Monolog\Handler\StreamHandler;

class OrderService
{
    public function __construct(private Logger $logger) {}

    public function process(Order $order): void
    {
        $this->logger->info('Processing order', ['id' => $order->id]);
        $this->logger->error('Order failed', ['reason' => $e->getMessage()]);
    }
}
`
	ents := extract(t, "phpObs_laravel_symfony", fi("app/Services/OrderService.php", "php", src))
	if !containsEntity(ents, "SCOPE.Pattern", "$logger->info") {
		t.Error("expected $logger->info Monolog call-site entity")
	}
	if !containsEntity(ents, "SCOPE.Pattern", "$logger->error") {
		t.Error("expected $logger->error Monolog call-site entity")
	}
}

// TestPHPObsMonologUseDeclaration verifies that a file with a Monolog use
// statement but no call-sites emits a logger use-declaration entity.
func TestPHPObsMonologUseDeclaration(t *testing.T) {
	src := `<?php
use Monolog\Logger;

class AppBootstrap
{
    private Logger $logger;
}
`
	ents := extract(t, "phpObs_laravel_symfony", fi("bootstrap/app.php", "php", src))
	if !containsEntity(ents, "SCOPE.Pattern", "Monolog\\Logger") {
		t.Error("expected Monolog\\Logger use-declaration entity")
	}
}

// TestPHPObsSymfonyInjectedLogger verifies $this->logger->info call-sites inside
// a Symfony-style service (LoggerInterface injected via constructor).
func TestPHPObsSymfonyInjectedLogger(t *testing.T) {
	src := `<?php
use Psr\Log\LoggerInterface;

class NotificationService
{
    public function __construct(private LoggerInterface $logger) {}

    public function send(Notification $n): void
    {
        $this->logger->info('Notification sent', ['id' => $n->id]);
        $this->logger->warning('Recipient not found', ['email' => $n->email]);
    }
}
`
	ents := extract(t, "phpObs_laravel_symfony", fi("src/Service/NotificationService.php", "php", src))
	if !containsEntity(ents, "SCOPE.Pattern", "$logger->info") {
		t.Error("expected $logger->info PSR-3 entity for Symfony injected logger")
	}
	if !containsEntity(ents, "SCOPE.Pattern", "$logger->warning") {
		t.Error("expected $logger->warning PSR-3 entity")
	}
}

// ---------------------------------------------------------------------------
// log_extraction — no false positives
// ---------------------------------------------------------------------------

// TestPHPObsLogNoFalsePositiveOnPlainPHP verifies files with no log calls
// produce no observability entities.
func TestPHPObsLogNoFalsePositiveOnPlainPHP(t *testing.T) {
	src := `<?php
class Calculator
{
    public function add(int $a, int $b): int { return $a + $b; }
}
`
	ents := extract(t, "phpObs_laravel_symfony", fi("src/Calculator.php", "php", src))
	if len(ents) != 0 {
		t.Errorf("expected no observability entities for plain PHP, got %d", len(ents))
	}
}

// ---------------------------------------------------------------------------
// metric_extraction — prometheus_client_php
// ---------------------------------------------------------------------------

// TestPHPObsPrometheusCounter verifies that new Counter() with a Prometheus
// use-statement emits a metric entity.
func TestPHPObsPrometheusCounter(t *testing.T) {
	src := `<?php
use Prometheus\Counter;
use Prometheus\CollectorRegistry;

class MetricsService
{
    public function registerMetrics(CollectorRegistry $registry): void
    {
        $counter = new Counter('http_requests_total', 'Total HTTP requests', ['method', 'status']);
        $registry->registerCounter('app', 'errors_total', 'Total errors');
    }
}
`
	ents := extract(t, "phpObs_laravel_symfony", fi("src/Service/MetricsService.php", "php", src))
	if !containsEntity(ents, "SCOPE.Pattern", "new Counter") {
		t.Error("expected new Counter metric entity")
	}
	if !containsEntity(ents, "SCOPE.Pattern", "registerCounter") {
		t.Error("expected registerCounter metric entity")
	}
}

// TestPHPObsPrometheusGaugeHistogram verifies Gauge and Histogram entities
// are extracted per-instantiation.
func TestPHPObsPrometheusGaugeHistogram(t *testing.T) {
	src := `<?php
use Prometheus\Gauge;
use Prometheus\Histogram;

$gauge = new Gauge('memory_usage_bytes', 'Current memory usage');
$histogram = new Histogram('request_duration_seconds', 'Request duration', ['endpoint']);
`
	ents := extract(t, "phpObs_laravel_symfony", fi("src/Metrics/AppMetrics.php", "php", src))
	if !containsEntity(ents, "SCOPE.Pattern", "new Gauge") {
		t.Error("expected new Gauge metric entity")
	}
	if !containsEntity(ents, "SCOPE.Pattern", "new Histogram") {
		t.Error("expected new Histogram metric entity")
	}
}

// TestPHPObsPrometheusUseDeclarationOnly verifies that a file with only a
// Prometheus use-statement (no instantiation) emits a use-declaration entity.
func TestPHPObsPrometheusUseDeclarationOnly(t *testing.T) {
	src := `<?php
use Prometheus\CollectorRegistry;

class PrometheusBootstrap
{
    public function getRegistry(): CollectorRegistry
    {
        return new CollectorRegistry(new InMemory());
    }
}
`
	ents := extract(t, "phpObs_laravel_symfony", fi("bootstrap/metrics.php", "php", src))
	if !containsEntity(ents, "SCOPE.Pattern", "Prometheus") {
		t.Error("expected Prometheus use-declaration entity")
	}
}

// ---------------------------------------------------------------------------
// metric_extraction — StatsD
// ---------------------------------------------------------------------------

// TestPHPObsStatsdIncrement verifies $statsd->increment('metric.name') emits
// a named metric entity.
func TestPHPObsStatsdIncrement(t *testing.T) {
	src := `<?php
use League\StatsD\Client as StatsD;

$statsd = new StatsD();
$statsd->increment('page.views');
$statsd->gauge('queue.depth', $depth);
`
	ents := extract(t, "phpObs_laravel_symfony", fi("src/Tracking/PageTracker.php", "php", src))
	if !containsEntity(ents, "SCOPE.Pattern", "page.views") {
		t.Error("expected page.views StatsD metric entity")
	}
	if !containsEntity(ents, "SCOPE.Pattern", "queue.depth") {
		t.Error("expected queue.depth StatsD gauge entity")
	}
}

// TestPHPObsStatsdUseDeclaration verifies League\StatsD use without call-sites
// emits a use-declaration entity.
func TestPHPObsStatsdUseDeclaration(t *testing.T) {
	src := `<?php
use League\StatsD\Client as StatsD;

class StatsHelper
{
    public function __construct(private StatsD $statsd) {}
}
`
	ents := extract(t, "phpObs_laravel_symfony", fi("src/StatsHelper.php", "php", src))
	if !containsEntity(ents, "SCOPE.Pattern", "StatsD") {
		t.Error("expected StatsD use-declaration entity")
	}
}

// TestPHPObsLaravelMetrics verifies the Laravel Metrics facade call-sites are
// extracted (optional package).
func TestPHPObsLaravelMetrics(t *testing.T) {
	src := `<?php
Metrics::counter('orders_placed');
Metrics::gauge('active_users', $count);
`
	ents := extract(t, "phpObs_laravel_symfony", fi("app/Observers/OrderObserver.php", "php", src))
	if !containsEntity(ents, "SCOPE.Pattern", "Metrics::counter") {
		t.Error("expected Metrics::counter entity")
	}
	if !containsEntity(ents, "SCOPE.Pattern", "Metrics::gauge") {
		t.Error("expected Metrics::gauge entity")
	}
}

// ---------------------------------------------------------------------------
// trace_extraction — OpenTelemetry PHP SDK
// ---------------------------------------------------------------------------

// TestPHPObsOtelSpanBuilder verifies $tracer->spanBuilder('name') emits a
// trace_span entity with the span name.
func TestPHPObsOtelSpanBuilder(t *testing.T) {
	src := `<?php
use OpenTelemetry\API\Trace\TracerInterface;

class OrderService
{
    public function __construct(private TracerInterface $tracer) {}

    public function createOrder(array $data): Order
    {
        $span = $this->tracer->spanBuilder('order.create')->startSpan();
        try {
            $order = Order::create($data);
            $span->setAttribute('order.id', $order->id);
            return $order;
        } finally {
            $span->end();
        }
    }
}
`
	ents := extract(t, "phpObs_laravel_symfony", fi("app/Services/OrderService.php", "php", src))
	if !containsEntity(ents, "SCOPE.Pattern", "order.create") {
		t.Error("expected order.create span entity from spanBuilder")
	}
}

// TestPHPObsOtelStartSpan verifies $tracer->startSpan('name') is extracted.
func TestPHPObsOtelStartSpan(t *testing.T) {
	src := `<?php
use OpenTelemetry\API\Trace\TracerInterface;

$span = $tracer->startSpan('payment.process');
$span->setAttribute('amount', $amount);
$span->end();
`
	ents := extract(t, "phpObs_laravel_symfony", fi("src/Payment/Processor.php", "php", src))
	if !containsEntity(ents, "SCOPE.Pattern", "payment.process") {
		t.Error("expected payment.process span entity from startSpan")
	}
}

// TestPHPObsOtelSpanLifecycle verifies $span->setAttribute/addEvent/end
// lifecycle methods emit trace_span entities.
func TestPHPObsOtelSpanLifecycle(t *testing.T) {
	src := `<?php
use OpenTelemetry\API\Trace\TracerInterface;

$span = $tracer->spanBuilder('user.auth')->startSpan();
$span->setAttribute('user.id', $userId);
$span->addEvent('auth.success');
$span->setStatus(StatusCode::STATUS_OK);
$span->end();
`
	ents := extract(t, "phpObs_laravel_symfony", fi("src/Auth/AuthService.php", "php", src))
	if !containsEntity(ents, "SCOPE.Pattern", "$span->setAttribute") {
		t.Error("expected $span->setAttribute lifecycle entity")
	}
	if !containsEntity(ents, "SCOPE.Pattern", "$span->addEvent") {
		t.Error("expected $span->addEvent lifecycle entity")
	}
	if !containsEntity(ents, "SCOPE.Pattern", "$span->end") {
		t.Error("expected $span->end lifecycle entity")
	}
}

// TestPHPObsOtelBootstrap verifies CachedInstrumentation and
// Globals::tracerProvider() bootstrap calls emit trace_span entities.
func TestPHPObsOtelBootstrap(t *testing.T) {
	src := `<?php
use OpenTelemetry\API\Globals;

$tracerProvider = Globals::tracerProvider();
$tracer = $tracerProvider->getTracer('my-app');
`
	ents := extract(t, "phpObs_laravel_symfony", fi("bootstrap/otel.php", "php", src))
	if !containsEntity(ents, "SCOPE.Pattern", "Globals::tracerProvider()") {
		t.Error("expected Globals::tracerProvider() bootstrap entity")
	}
}

// TestPHPObsOtelUseDeclaration verifies that a file with only OpenTelemetry
// use-statements (no span calls) emits a use-declaration entity.
func TestPHPObsOtelUseDeclaration(t *testing.T) {
	src := `<?php
use OpenTelemetry\API\Trace\TracerInterface;

class TracingBootstrap
{
    public function __construct(private TracerInterface $tracer) {}
}
`
	ents := extract(t, "phpObs_laravel_symfony", fi("src/Tracing/Bootstrap.php", "php", src))
	if !containsEntity(ents, "SCOPE.Pattern", "OpenTelemetry") {
		t.Error("expected OpenTelemetry use-declaration entity")
	}
}

// ---------------------------------------------------------------------------
// trace_extraction — Symfony Stopwatch
// ---------------------------------------------------------------------------

// TestPHPObsSymfonyStopwatchStartStop verifies $stopwatch->start/stop calls
// emit named trace_span entities.
func TestPHPObsSymfonyStopwatchStartStop(t *testing.T) {
	src := `<?php
use Symfony\Component\Stopwatch\Stopwatch;

class DataImportService
{
    public function import(array $data): void
    {
        $stopwatch = new Stopwatch();
        $stopwatch->start('data.import');
        foreach ($data as $row) {
            $this->processRow($row);
        }
        $event = $stopwatch->stop('data.import');
    }
}
`
	ents := extract(t, "phpObs_laravel_symfony", fi("src/Service/DataImportService.php", "php", src))
	if !containsEntity(ents, "SCOPE.Pattern", "data.import") {
		t.Error("expected data.import Stopwatch trace_span entity")
	}
}

// TestPHPObsSymfonyStopwatchLap verifies $stopwatch->lap('name') emits an entity.
func TestPHPObsSymfonyStopwatchLap(t *testing.T) {
	src := `<?php
use Symfony\Component\Stopwatch\Stopwatch;

$stopwatch = new Stopwatch();
$stopwatch->start('batch.processing');
foreach ($items as $item) {
    $this->process($item);
    $stopwatch->lap('batch.processing');
}
$stopwatch->stop('batch.processing');
`
	ents := extract(t, "phpObs_laravel_symfony", fi("src/Batch/Processor.php", "php", src))
	if !containsEntity(ents, "SCOPE.Pattern", "batch.processing") {
		t.Error("expected batch.processing Stopwatch entity")
	}
}

// TestPHPObsSymfonyStopwatchUseDeclaration verifies that a file importing
// Stopwatch without call-sites emits a use-declaration entity.
func TestPHPObsSymfonyStopwatchUseDeclaration(t *testing.T) {
	src := `<?php
use Symfony\Component\Stopwatch\Stopwatch;

class ProfilerHelper
{
    public function __construct(private Stopwatch $stopwatch) {}
}
`
	ents := extract(t, "phpObs_laravel_symfony", fi("src/Profiler/Helper.php", "php", src))
	if !containsEntity(ents, "SCOPE.Pattern", "Stopwatch") {
		t.Error("expected Stopwatch use-declaration entity")
	}
}

// ---------------------------------------------------------------------------
// trace_extraction — DDTrace
// ---------------------------------------------------------------------------

// TestPHPObsDDTrace verifies \DDTrace\trace_function and \DDTrace\trace_method
// are extracted.
func TestPHPObsDDTrace(t *testing.T) {
	src := `<?php
\DDTrace\trace_function('App\Service\OrderService::create', function(SpanData $span, $args) {
    $span->name = 'order.service.create';
});
\DDTrace\trace_method('App\Repository\UserRepo', 'find', function(SpanData $span, $args) {
    $span->name = 'user.repo.find';
});
`
	ents := extract(t, "phpObs_laravel_symfony", fi("src/Tracing/DatadogTracing.php", "php", src))
	if !containsEntity(ents, "SCOPE.Pattern", "DDTrace\\trace_function") {
		t.Error("expected DDTrace\\trace_function entity")
	}
	if !containsEntity(ents, "SCOPE.Pattern", "DDTrace\\trace_method") {
		t.Error("expected DDTrace\\trace_method entity")
	}
}

// ---------------------------------------------------------------------------
// Guard: non-PHP files are ignored
// ---------------------------------------------------------------------------

// TestPHPObsIgnoresNonPHP verifies the extractor produces no entities for
// non-php language files.
func TestPHPObsIgnoresNonPHP(t *testing.T) {
	src := `Log::info('This is PHP-like code but in a JS file');`
	ents := extract(t, "phpObs_laravel_symfony", fi("frontend/app.js", "javascript", src))
	if len(ents) != 0 {
		t.Errorf("expected 0 entities for non-php file, got %d", len(ents))
	}
}

// TestPHPObsIgnoresEmptyFile verifies empty files return no entities.
func TestPHPObsIgnoresEmptyFile(t *testing.T) {
	ents := extract(t, "phpObs_laravel_symfony", fi("app/Empty.php", "php", ""))
	if len(ents) != 0 {
		t.Errorf("expected 0 entities for empty file, got %d", len(ents))
	}
}
