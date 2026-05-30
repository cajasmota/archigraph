// Fixture: actix-web application with JWT auth + middleware
use actix_web::{web, App, HttpServer, middleware};
use actix_web_httpauth::middleware::HttpAuthentication;
use jsonwebtoken::{decode, encode, DecodingKey, EncodingKey, Validation};

async fn index() -> &'static str {
    "Hello, World!"
}

#[actix_web::main]
async fn main() -> std::io::Result<()> {
    HttpServer::new(|| {
        let bearer_middleware = HttpAuthentication::bearer(validator);
        App::new()
            .wrap(bearer_middleware)
            .wrap(middleware::Logger::default())
            .service(web::resource("/api").route(web::get().to(index)))
    })
    .bind("127.0.0.1:8080")?
    .run()
    .await
}

async fn validator(req: ServiceRequest, credentials: BearerCredentials) -> Result<ServiceRequest, Error> {
    let token = credentials.token();
    let key = DecodingKey::from_secret(b"secret");
    let validation = Validation::default();
    decode::<Claims>(token, &key, &validation)?;
    Ok(req)
}
