package rust_test

import (
	"testing"
)

// --- axum: StatusCode enum + tuple return in handler body --------------------

func TestRustRespCodes_AxumStatusCodeEnum(t *testing.T) {
	src := `
async fn create_user() -> impl IntoResponse {
    StatusCode::CREATED
}

async fn get_user() -> impl IntoResponse {
    if missing {
        return (StatusCode::NOT_FOUND, "nope");
    }
    (StatusCode::OK, "ok")
}

fn app() -> Router {
    Router::new()
        .route("/users", post(create_user))
        .route("/users/:id", get(get_user))
}
`
	ents := extract(t, "custom_rust_endpoint_response_codes", fi("routes.rs", "rust", src))

	create := findRustDep(ents, "SCOPE.Operation", "POST /users")
	if create == nil {
		t.Fatalf("expected POST /users, got %+v", ents)
	}
	propEq(t, create, "response_codes", "201")
	propEq(t, create, "success_code", "201")
	propEq(t, create, "framework", "axum")

	get := findRustDep(ents, "SCOPE.Operation", "GET /users/{id}")
	if get == nil {
		t.Fatalf("expected GET /users/{id}, got %+v", ents)
	}
	// Both 200 and 404 returned; success_code is the lone 2xx (200).
	propEq(t, get, "response_codes", "200,404")
	propEq(t, get, "success_code", "200")
}

func TestRustRespCodes_AxumNumericConstructor(t *testing.T) {
	src := `
async fn handler() -> impl IntoResponse {
    StatusCode::from_u16(202).unwrap()
}
fn app() -> Router { Router::new().route("/jobs", post(handler)) }
`
	ents := extract(t, "custom_rust_endpoint_response_codes", fi("num.rs", "rust", src))
	op := findRustDep(ents, "SCOPE.Operation", "POST /jobs")
	if op == nil {
		t.Fatalf("expected POST /jobs, got %+v", ents)
	}
	propEq(t, op, "response_codes", "202")
	propEq(t, op, "success_code", "202")
	propEq(t, op, "response_codes_source", "StatusCode::from_u16")
}

func TestRustRespCodes_AxumNestComposes(t *testing.T) {
	src := `
async fn old() -> impl IntoResponse { (StatusCode::GONE, "gone") }
fn build() -> Router {
    let api = Router::new().route("/items", get(old));
    Router::new().nest("/api/v1", api)
}
`
	ents := extract(t, "custom_rust_endpoint_response_codes", fi("nest.rs", "rust", src))
	op := findRustDep(ents, "SCOPE.Operation", "GET /api/v1/items")
	if op == nil {
		t.Fatalf("expected nest-composed GET /api/v1/items, got %+v", ents)
	}
	propEq(t, op, "response_codes", "410")
	propEq(t, op, "nest_prefix", "/api/v1")
	// 410 is not 2xx → no success_code.
	propAbsent(t, op, "success_code")
}

// --- actix: HttpResponse builders + .status() --------------------------------

func TestRustRespCodes_ActixBuilders(t *testing.T) {
	src := `
#[post("/items")]
async fn create() -> impl Responder {
    HttpResponse::Created().json(item)
}
`
	ents := extract(t, "custom_rust_endpoint_response_codes", fi("actix.rs", "rust", src))
	op := findRustDep(ents, "SCOPE.Operation", "POST /items")
	if op == nil {
		t.Fatalf("expected POST /items, got %+v", ents)
	}
	propEq(t, op, "response_codes", "201")
	propEq(t, op, "success_code", "201")
	propEq(t, op, "framework", "actix_web")
	propEq(t, op, "response_codes_source", "HttpResponse::Created()")
}

func TestRustRespCodes_ActixMultiCode(t *testing.T) {
	src := `
#[get("/items/{id}")]
async fn get_item() -> impl Responder {
    if missing {
        return HttpResponse::NotFound().finish();
    }
    HttpResponse::Ok().json(item)
}
`
	ents := extract(t, "custom_rust_endpoint_response_codes", fi("actix2.rs", "rust", src))
	op := findRustDep(ents, "SCOPE.Operation", "GET /items/{id}")
	if op == nil {
		t.Fatalf("expected GET /items/{id}, got %+v", ents)
	}
	propEq(t, op, "response_codes", "200,404")
	propEq(t, op, "success_code", "200")
}

// --- rocket: Status enum + mount prefix --------------------------------------

func TestRustRespCodes_RocketStatusMounted(t *testing.T) {
	src := `
#[post("/posts")]
fn create_post() -> Status {
    Status::Created
}

#[launch]
fn rocket() -> _ {
    rocket::build().mount("/api/v1", routes![create_post])
}
`
	ents := extract(t, "custom_rust_endpoint_response_codes", fi("rocket.rs", "rust", src))
	op := findRustDep(ents, "SCOPE.Operation", "POST /api/v1/posts")
	if op == nil {
		t.Fatalf("expected mount-composed POST /api/v1/posts, got %+v", ents)
	}
	propEq(t, op, "response_codes", "201")
	propEq(t, op, "success_code", "201")
	propEq(t, op, "framework", "rocket")
	propEq(t, op, "mount_prefix", "/api/v1")
}

// --- negatives ---------------------------------------------------------------

func TestRustRespCodes_NoStatusNotStamped(t *testing.T) {
	// A plain handler with no resolvable literal status is NOT re-emitted (the
	// framework default 200 is never fabricated).
	src := `
async fn list() -> impl IntoResponse { "ok" }
fn app() -> Router { Router::new().route("/users", get(list)) }
`
	ents := extract(t, "custom_rust_endpoint_response_codes", fi("plain.rs", "rust", src))
	if findRustDep(ents, "SCOPE.Operation", "GET /users") != nil {
		t.Errorf("handler with no literal status must NOT be stamped, got %+v", ents)
	}
}

func TestRustRespCodes_DynamicStatusHonestPartial(t *testing.T) {
	// A dynamic status (a variable) is skipped, but a sibling literal in the same
	// body is still recorded.
	src := `
async fn handler() -> impl IntoResponse {
    let code = compute();
    if bad {
        return StatusCode::from_u16(code).unwrap(); // dynamic — skipped
    }
    StatusCode::CREATED
}
fn app() -> Router { Router::new().route("/x", post(handler)) }
`
	ents := extract(t, "custom_rust_endpoint_response_codes", fi("dyn.rs", "rust", src))
	op := findRustDep(ents, "SCOPE.Operation", "POST /x")
	if op == nil {
		t.Fatalf("expected POST /x, got %+v", ents)
	}
	// Only the literal 201 is recorded; the dynamic from_u16(code) is not.
	propEq(t, op, "response_codes", "201")
}

func TestRustRespCodes_NonRouteHelperUnaffected(t *testing.T) {
	// A StatusCode reference in a non-route helper fn that no route names must
	// NOT emit any endpoint.
	src := `
fn map_err() -> StatusCode { StatusCode::INTERNAL_SERVER_ERROR }

async fn list() -> impl IntoResponse { "ok" }
fn app() -> Router { Router::new().route("/users", get(list)) }
`
	ents := extract(t, "custom_rust_endpoint_response_codes", fi("helper.rs", "rust", src))
	if findRustDep(ents, "SCOPE.Operation", "GET /users") != nil {
		t.Errorf("route naming a status-less handler must NOT be stamped, got %+v", ents)
	}
}

// --- #5018: remaining handler-named frameworks -------------------------------

// poem — `.at("/path", post(handler))`; handler returns a poem StatusCode.
func TestRustRespCodes_PoemHandlerStatus(t *testing.T) {
	src := `
async fn create(req: Request) -> Response {
    Response::builder().status(StatusCode::CREATED).body("ok")
}
fn route() -> Route {
    Route::new().at("/items", post(create))
}
`
	ents := extract(t, "custom_rust_endpoint_response_codes", fi("poem.rs", "rust", src))
	op := findRustDep(ents, "SCOPE.Operation", "POST /items")
	if op == nil {
		t.Fatalf("expected POST /items, got %+v", ents)
	}
	propEq(t, op, "response_codes", "201")
	propEq(t, op, "success_code", "201")
	propEq(t, op, "framework", "poem")
}

// warp — `warp::reply::with_status(reply, StatusCode::CREATED)` in handler body.
func TestRustRespCodes_WarpWithStatus(t *testing.T) {
	src := `
async fn create_item(body: Item) -> Result<impl warp::Reply, warp::Rejection> {
    Ok(warp::reply::with_status(warp::reply::json(&body), StatusCode::CREATED))
}
fn routes() -> impl Filter {
    warp::path!("items").and(warp::post()).and_then(create_item)
}
`
	ents := extract(t, "custom_rust_endpoint_response_codes", fi("warp.rs", "rust", src))
	op := findRustDep(ents, "SCOPE.Operation", "POST /items")
	if op == nil {
		t.Fatalf("expected POST /items, got %+v", ents)
	}
	propEq(t, op, "response_codes", "201")
	propEq(t, op, "success_code", "201")
	propEq(t, op, "framework", "warp")
}

// tide — `Response::builder(201)` positional status in handler body.
func TestRustRespCodes_TideBuilderStatus(t *testing.T) {
	src := `
async fn create(_req: Request<()>) -> tide::Result {
    Ok(Response::builder(201).body("created").build())
}
fn app() {
    let mut app = tide::new();
    app.at("/items").post(create);
}
`
	ents := extract(t, "custom_rust_endpoint_response_codes", fi("tide.rs", "rust", src))
	op := findRustDep(ents, "SCOPE.Operation", "POST /items")
	if op == nil {
		t.Fatalf("expected POST /items, got %+v", ents)
	}
	propEq(t, op, "response_codes", "201")
	propEq(t, op, "success_code", "201")
	propEq(t, op, "framework", "tide")
	propEq(t, op, "response_codes_source", "Response::builder()")
}

// gotham — `route.get("/path").to(handler)`; handler builds a status response.
func TestRustRespCodes_GothamRouteStatus(t *testing.T) {
	src := `
fn get_item(state: State) -> (State, Response<Body>) {
    let res = create_response(&state, StatusCode::NOT_FOUND, mime::TEXT_PLAIN, "nope");
    (state, res)
}
fn router() -> Router {
    build_simple_router(|route| {
        route.get("/items/:id").to(get_item);
    })
}
`
	ents := extract(t, "custom_rust_endpoint_response_codes", fi("gotham.rs", "rust", src))
	op := findRustDep(ents, "SCOPE.Operation", "GET /items/{id}")
	if op == nil {
		t.Fatalf("expected GET /items/{id}, got %+v", ents)
	}
	propEq(t, op, "response_codes", "404")
	propAbsent(t, op, "success_code") // 404 is not 2xx
	propEq(t, op, "framework", "gotham")
}

// salvo — `res.status_code(StatusCode::CREATED)` in handler body.
func TestRustRespCodes_SalvoStatusCode(t *testing.T) {
	src := `
#[handler]
async fn create(res: &mut Response) {
    res.status_code(StatusCode::CREATED);
}
fn route() -> Router {
    Router::with_path("items").post(create)
}
`
	ents := extract(t, "custom_rust_endpoint_response_codes", fi("salvo.rs", "rust", src))
	op := findRustDep(ents, "SCOPE.Operation", "POST /items")
	if op == nil {
		t.Fatalf("expected POST /items, got %+v", ents)
	}
	propEq(t, op, "response_codes", "201")
	propEq(t, op, "success_code", "201")
	propEq(t, op, "framework", "salvo")
}

// hyper — `match (req.method(), path) { (&Method::GET, "/p") => handler(req) }`.
// The arm RHS names a handler whose body carries the status idiom.
func TestRustRespCodes_HyperNamedHandler(t *testing.T) {
	src := `
async fn get_users(req: Request<Body>) -> Result<Response<Body>, hyper::Error> {
    Ok(Response::builder().status(StatusCode::OK).body(Body::empty()).unwrap())
}
async fn create_user(req: Request<Body>) -> Result<Response<Body>, hyper::Error> {
    Ok(Response::builder().status(StatusCode::CREATED).body(Body::empty()).unwrap())
}
async fn router(req: Request<Body>) -> Result<Response<Body>, hyper::Error> {
    match (req.method(), req.uri().path()) {
        (&Method::GET, "/users") => get_users(req).await,
        (&Method::POST, "/users") => create_user(req).await,
        _ => not_found(),
    }
}
`
	ents := extract(t, "custom_rust_endpoint_response_codes", fi("hyper.rs", "rust", src))

	get := findRustDep(ents, "SCOPE.Operation", "GET /users")
	if get == nil {
		t.Fatalf("expected GET /users, got %+v", ents)
	}
	propEq(t, get, "response_codes", "200")
	propEq(t, get, "success_code", "200")
	propEq(t, get, "framework", "hyper")

	post := findRustDep(ents, "SCOPE.Operation", "POST /users")
	if post == nil {
		t.Fatalf("expected POST /users, got %+v", ents)
	}
	propEq(t, post, "response_codes", "201")
	propEq(t, post, "success_code", "201")
	propEq(t, post, "framework", "hyper")
}

// hyper — INLINE-block arm: status idiom written directly in the match arm body
// (no separate handler fn), clipped at the next arm.
func TestRustRespCodes_HyperInlineArm(t *testing.T) {
	src := `
async fn router(req: Request<Body>) -> Result<Response<Body>, hyper::Error> {
    match (req.method(), req.uri().path()) {
        (&Method::POST, "/items") => {
            Ok(Response::builder().status(StatusCode::CREATED).body(Body::empty()).unwrap())
        }
        (&Method::GET, "/health") => {
            Ok(Response::builder().status(StatusCode::NO_CONTENT).body(Body::empty()).unwrap())
        }
        _ => Ok(Response::builder().status(StatusCode::NOT_FOUND).body(Body::empty()).unwrap()),
    }
}
`
	ents := extract(t, "custom_rust_endpoint_response_codes", fi("hyper_inline.rs", "rust", src))

	post := findRustDep(ents, "SCOPE.Operation", "POST /items")
	if post == nil {
		t.Fatalf("expected POST /items, got %+v", ents)
	}
	// Clipped at the next arm: only 201, not the sibling 204.
	propEq(t, post, "response_codes", "201")
	propEq(t, post, "success_code", "201")
	propEq(t, post, "framework", "hyper")

	health := findRustDep(ents, "SCOPE.Operation", "GET /health")
	if health == nil {
		t.Fatalf("expected GET /health, got %+v", ents)
	}
	propEq(t, health, "response_codes", "204")
}

// hyper — wrong language is a no-op (the extractor only runs on rust files).
func TestRustRespCodes_HyperWrongLanguageNoOp(t *testing.T) {
	src := `(&Method::GET, "/users") => get_users(req).await,
fn get_users() { StatusCode::OK }`
	ents := extract(t, "custom_rust_endpoint_response_codes", fi("hyper.go", "go", src))
	if len(ents) != 0 {
		t.Errorf("non-rust file must produce no entities, got %+v", ents)
	}
}

// hyper — honest-partial: a match-arm handler with no literal status is NOT
// re-emitted (no fabricated default 200).
func TestRustRespCodes_HyperNoStatusNotStamped(t *testing.T) {
	src := `
async fn get_users(req: Request<Body>) -> Result<Response<Body>, hyper::Error> {
    Ok(Response::new(Body::from("ok")))
}
async fn router(req: Request<Body>) -> Result<Response<Body>, hyper::Error> {
    match (req.method(), req.uri().path()) {
        (&Method::GET, "/users") => get_users(req).await,
        _ => not_found(),
    }
}
`
	ents := extract(t, "custom_rust_endpoint_response_codes", fi("hyper_plain.rs", "rust", src))
	if findRustDep(ents, "SCOPE.Operation", "GET /users") != nil {
		t.Errorf("hyper handler with no literal status must NOT be stamped, got %+v", ents)
	}
}

// honest-partial — a minor-framework route whose handler has no literal status
// is NOT re-emitted.
func TestRustRespCodes_MinorFwNoStatusNotStamped(t *testing.T) {
	src := `
async fn list(req: Request) -> Response { Response::builder().body("ok") }
fn route() -> Route { Route::new().at("/items", get(list)) }
`
	ents := extract(t, "custom_rust_endpoint_response_codes", fi("poem_plain.rs", "rust", src))
	if findRustDep(ents, "SCOPE.Operation", "GET /items") != nil {
		t.Errorf("poem handler with no literal status must NOT be stamped, got %+v", ents)
	}
}
