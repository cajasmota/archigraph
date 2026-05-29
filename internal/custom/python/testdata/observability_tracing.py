"""Fixture: Python tracing with opentelemetry, ddtrace, and jaeger-client."""

# --- opentelemetry ---
from opentelemetry import trace
from opentelemetry.trace import SpanKind

tracer = trace.get_tracer(__name__)


@tracer.start_as_current_span("process_request")
def handle_request(request):
    pass


def process_order(order_id):
    with tracer.start_as_current_span("process_order") as span:
        span.set_attribute("order.id", order_id)
        return _fetch(order_id)


def _fetch(order_id):
    span = tracer.start_span("db_fetch")
    try:
        return db.get(order_id)
    finally:
        span.end()


# --- ddtrace ---
from ddtrace import tracer as dd_tracer
from ddtrace import patch_all

patch_all()


@dd_tracer.wrap("order_service.place")
def place_order(order):
    with dd_tracer.trace("order_service.validate") as span:
        span.set_tag("order.id", order.id)
        return validate(order)


# --- jaeger-client ---
import jaeger_client
from opentracing import tracer as opentracing_tracer

config = jaeger_client.Config(
    config={"sampler": {"type": "const", "param": 1}},
    service_name="order-service",
    validate=True,
)
jaeger_tracer = config.initialize_tracer()

with opentracing_tracer.start_span("order_lookup") as span:
    span.set_tag("order.id", "12345")
    result = db.lookup(span)
