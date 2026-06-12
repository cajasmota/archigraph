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
