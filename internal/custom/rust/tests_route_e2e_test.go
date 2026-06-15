package rust

import (
	"context"
	"strings"
	"testing"

	"github.com/cajasmota/grafel/internal/extractor"
)

// runRustRouteE2E runs the extractor and returns the captured e2e_route_calls
// property (empty string when no suite emitted).
func runRustRouteE2E(t *testing.T, path, src string) string {
	t.Helper()
	e := &rustTestRouteE2EExtractor{}
	ents, err := e.Extract(context.Background(), extractor.FileInput{
		Path: path, Language: "rust", Content: []byte(src),
	})
	if err != nil {
		t.Fatalf("extract: %v", err)
	}
	if len(ents) == 0 {
		return ""
	}
	if ents[0].Subtype != "test_suite" {
		t.Fatalf("expected test_suite, got %q", ents[0].Subtype)
	}
	return ents[0].Properties["e2e_route_calls"]
}

func TestRustRouteE2E_ActixTestRequest(t *testing.T) {
	src := `
#[actix_web::test]
async fn test_get_counts() {
    let app = test::init_service(App::new().configure(routes)).await;
    let req = test::TestRequest::get().uri("/api/v1/x/get_counts").to_request();
    let resp = test::call_service(&app, req).await;
    assert!(resp.status().is_success());
}

#[actix_web::test]
async fn test_create_item() {
    let app = test::init_service(App::new().configure(routes)).await;
    let req = test::TestRequest::post().uri("/api/v1/x/items").to_request();
    let resp = test::call_service(&app, req).await;
    assert!(resp.status().is_success());
}
`
	got := runRustRouteE2E(t, "tests/x_api_test.rs", src)
	if !strings.Contains(got, "GET /api/v1/x/get_counts") || !strings.Contains(got, "POST /api/v1/x/items") {
		t.Fatalf("Actix TestRequest routes not captured: %q", got)
	}
}

func TestRustRouteE2E_ActixWithURIMethod(t *testing.T) {
	src := `
#[actix_web::test]
async fn test_delete() {
    let req = test::TestRequest::with_uri("/api/v1/x/1").method(Method::DELETE).to_request();
    let _ = test::call_service(&app, req).await;
}
`
	got := runRustRouteE2E(t, "tests/x_api_test.rs", src)
	if !strings.Contains(got, "DELETE /api/v1/x/1") {
		t.Fatalf("Actix with_uri/method route not captured: %q", got)
	}
}

func TestRustRouteE2E_AxumOneshot(t *testing.T) {
	src := `
#[tokio::test]
async fn test_axum() {
    let app = router();
    let resp = app
        .oneshot(Request::get("/api/v1/users/1").body(Body::empty()).unwrap())
        .await
        .unwrap();
    assert_eq!(resp.status(), StatusCode::OK);
}

#[tokio::test]
async fn test_axum_post() {
    let req = Request::builder()
        .method(Method::POST)
        .uri("/api/v1/users")
        .body(Body::from(payload))
        .unwrap();
    let _ = app.oneshot(req).await.unwrap();
}
`
	got := runRustRouteE2E(t, "tests/users_test.rs", src)
	if !strings.Contains(got, "GET /api/v1/users/1") || !strings.Contains(got, "POST /api/v1/users") {
		t.Fatalf("Axum oneshot routes not captured: %q", got)
	}
}

func TestRustRouteE2E_RocketDispatch(t *testing.T) {
	src := `
#[test]
fn test_index() {
    let client = Client::tracked(rocket()).unwrap();
    let response = client.get("/api/v1/index").dispatch();
    assert_eq!(response.status(), Status::Ok);
}

#[test]
fn test_create() {
    let client = Client::tracked(rocket()).unwrap();
    let response = client.post("/api/v1/items").dispatch();
    assert_eq!(response.status(), Status::Created);
}
`
	got := runRustRouteE2E(t, "tests/rocket_test.rs", src)
	if !strings.Contains(got, "GET /api/v1/index") || !strings.Contains(got, "POST /api/v1/items") {
		t.Fatalf("Rocket dispatch routes not captured: %q", got)
	}
}

func TestRustRouteE2E_ReqwestTestServer(t *testing.T) {
	src := `
#[tokio::test]
async fn test_reqwest() {
    let addr = spawn_app().await;
    let client = reqwest::Client::new();
    let resp = client.get(format!("{}/api/v1/health", addr)).send().await.unwrap();
    assert!(resp.status().is_success());
}
`
	got := runRustRouteE2E(t, "tests/health_test.rs", src)
	if !strings.Contains(got, "GET /api/v1/health") {
		t.Fatalf("reqwest test-server route not captured: %q", got)
	}
}

// Negative: a production handler file (not a test, no test attribute) that
// merely mentions a route string must NOT emit a suite.
func TestRustRouteE2E_NonTestFileNoSuite(t *testing.T) {
	src := `
async fn handler() -> impl Responder {
    HttpResponse::Ok().json("/api/v1/x")
}
`
	got := runRustRouteE2E(t, "src/handlers.rs", src)
	if got != "" {
		t.Fatalf("expected no suite for production file, got %q", got)
	}
}

// Negative: variable-only routes (no literal path) are dropped.
func TestRustRouteE2E_VariableRouteDropped(t *testing.T) {
	src := `
#[tokio::test]
async fn test_dynamic() {
    let path = build_path();
    let resp = client.get(path).send().await.unwrap();
    let _ = resp;
}
`
	got := runRustRouteE2E(t, "tests/dyn_test.rs", src)
	if got != "" {
		t.Fatalf("expected no routes for variable-only URL, got %q", got)
	}
}
