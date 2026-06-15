package engine

import (
	"strings"
	"testing"

	"github.com/cajasmota/grafel/internal/types"

	// Register the Rust route-hit extractor so the test_suite (carrying
	// e2e_route_calls) comes from the REAL extractor, not a hand-built fixture.
	_ "github.com/cajasmota/grafel/internal/custom/rust"
)

// Issue #4749 LIVE-REPRO (resolve side) — Rust Actix / Axum / Rocket route-hit
// tests. Proves end-to-end that a Rust test calling a route by string
// (`test::TestRequest::get().uri("/p")` + call_service, `app.oneshot(Request::
// get("/p"))`, `client.get("/p").dispatch()`) links to the
// http_endpoint_definition it exercises — the Rust slice of the all-language
// program (#4615/#4749). The shared linkE2ERouteTestsToEndpoints pass is
// language-agnostic; only the Rust route capture is new.

const rustActixTestSrc4749 = `
#[actix_web::test]
async fn get_counts() {
    let app = test::init_service(App::new().configure(routes)).await;
    let req = test::TestRequest::get().uri("/api/v1/x/get_counts").to_request();
    let _ = test::call_service(&app, req).await;
}
#[actix_web::test]
async fn create_item() {
    let app = test::init_service(App::new().configure(routes)).await;
    let req = test::TestRequest::post().uri("/api/v1/x/items").to_request();
    let _ = test::call_service(&app, req).await;
}
`

const rustAxumTestSrc4749 = `
#[tokio::test]
async fn get_counts() {
    let resp = app.oneshot(Request::get("/api/v1/x/get_counts").body(Body::empty()).unwrap()).await.unwrap();
    let _ = resp;
}
#[tokio::test]
async fn create_item() {
    let req = Request::builder().method(Method::POST).uri("/api/v1/x/items").body(Body::empty()).unwrap();
    let _ = app.oneshot(req).await.unwrap();
}
`

func TestIssue4749_RustActixE2ERouteTestsLinkToEndpoints(t *testing.T) {
	defs := []types.EntityRecord{
		def("GET", "/api/v1/x/get_counts"),
		def("POST", "/api/v1/x/items"),
	}
	suite := realSuite(t, "custom_rust_tests_route_e2e",
		"tests/x_controller_test.rs", "rust", rustActixTestSrc4749)

	afterOut, edges := runE2ERouteResolve(t, defs, suite)
	if edges < 2 {
		t.Fatalf("expected >=2 e2e route TESTS edges (GET + POST), got %d", edges)
	}
	assertRustRouteEdges(t, edgeTargets(afterOut))
}

func TestIssue4749_RustAxumE2ERouteTestsLinkToEndpoints(t *testing.T) {
	defs := []types.EntityRecord{
		def("GET", "/api/v1/x/get_counts"),
		def("POST", "/api/v1/x/items"),
	}
	suite := realSuite(t, "custom_rust_tests_route_e2e",
		"tests/x_routes_test.rs", "rust", rustAxumTestSrc4749)

	afterOut, edges := runE2ERouteResolve(t, defs, suite)
	if edges < 2 {
		t.Fatalf("expected >=2 e2e route TESTS edges (GET + POST), got %d", edges)
	}
	assertRustRouteEdges(t, edgeTargets(afterOut))
}

func assertRustRouteEdges(t *testing.T, targets map[string]bool) {
	t.Helper()
	wantGet, wantPost := false, false
	for to := range targets {
		if strings.Contains(to, "GET:/api/v1/x/get_counts") {
			wantGet = true
		}
		if strings.Contains(to, "POST:/api/v1/x/items") {
			wantPost = true
		}
	}
	if !wantGet {
		t.Errorf("expected a TESTS edge to GET /api/v1/x/get_counts; targets=%v", targets)
	}
	if !wantPost {
		t.Errorf("expected a TESTS edge to POST /api/v1/x/items; targets=%v", targets)
	}
}
