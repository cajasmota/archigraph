// Fixture: rocket with tracing macro usage and prometheus metrics
use rocket::{get, launch, routes};
use prometheus::{IntCounter, Histogram};

#[get("/metrics")]
fn metrics_handler() -> &'static str {
    tracing::info!("metrics requested");
    tracing::debug!("debug info");
    "metrics"
}

#[launch]
fn rocket() -> _ {
    rocket::build().mount("/", routes![metrics_handler])
}
