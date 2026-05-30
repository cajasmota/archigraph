// Fixture: axum application with tracing + metrics + opentelemetry
use axum::{Router, routing::get};
use tracing::{info, warn, instrument};
use metrics::{counter, histogram};
use opentelemetry::global;

#[instrument]
async fn get_users() -> &'static str {
    tracing::info!("fetching users");
    metrics::counter!("requests_total", 1);
    "users"
}

async fn create_user() -> &'static str {
    tracing::warn!("creating user");
    metrics::histogram!("request_duration_seconds", 0.1);
    "created"
}

fn setup_telemetry() {
    let tracer = opentelemetry::global::tracer("my_service");
}
