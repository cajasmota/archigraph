"""Fixture: Python metrics with prometheus_client, statsd, and datadog."""

# --- prometheus_client ---
from prometheus_client import Counter, Gauge, Histogram, Summary, push_to_gateway

REQUEST_COUNT = Counter("http_requests_total", "Total HTTP requests", ["method", "endpoint"])
REQUEST_LATENCY = Histogram("http_request_duration_seconds", "HTTP request duration")
IN_PROGRESS = Gauge("http_requests_in_progress", "In-progress HTTP requests")
REQUEST_SUMMARY = Summary("request_processing_seconds", "Time spent processing request")

REQUEST_COUNT.labels(method="GET", endpoint="/api/users").inc()
REQUEST_LATENCY.observe(0.5)
IN_PROGRESS.inc()
IN_PROGRESS.dec()

push_to_gateway("localhost:9091", job="my_app")

# --- statsd ---
import statsd

client = statsd.StatsClient("localhost", 8125)
client.incr("page.views")
client.gauge("queue.size", 42)
client.timing("query.duration", 250)

# --- datadog ---
from datadog import statsd as dog_statsd

dog_statsd.increment("web.page_views", tags=["page:home"])
dog_statsd.gauge("system.cpu.usage", 83.5)
dog_statsd.histogram("api.response.time", 0.12)
