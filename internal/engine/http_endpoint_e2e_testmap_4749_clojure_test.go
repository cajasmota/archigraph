package engine

import (
	"strings"
	"testing"

	"github.com/cajasmota/grafel/internal/types"

	// Register the Clojure route-hit extractor so the test_suite (carrying
	// e2e_route_calls) comes from the REAL extractor, not a hand-built fixture.
	_ "github.com/cajasmota/grafel/internal/custom/clojure"
)

// Issue #4749 LIVE-REPRO (resolve side) — Clojure clojure.test / Ring tests.
//
// Proves end-to-end that a Clojure clojure.test test calling a route by string
// via ring-mock — `(app (mock/request :get "/todos"))` — links to the
// http_endpoint_definition it exercises. The Clojure slice of the all-language
// program (#4615 tail #4749), generalizing the shared
// linkE2ERouteTestsToEndpoints pass (#4351). The pass is language-agnostic; only
// the Clojure route capture (custom_clojure_tests_route_e2e) and the Clojure
// producer (synthesizeClojureRoutes) are new. Clojure is functional (no OO
// receiver objects) so receiver typing does not apply; the route-string →
// endpoint linkage is the coverage mechanism.

const cljRingMockTestSrc4749 = `(ns myapp.todo-handler-test
  (:require [clojure.test :refer [deftest testing is]]
            [ring.mock.request :as mock]
            [myapp.handler :refer [app]]))

(deftest todo-routes
  (testing "lists todos"
    (let [resp (app (mock/request :get "/todos"))]
      (is (= 200 (:status resp)))))
  (testing "creates a todo"
    (let [resp (app (mock/request :post "/todos"))]
      (is (= 201 (:status resp))))))
`

func TestIssue4749_ClojureRingMockE2ERouteTestsLinkToEndpoints(t *testing.T) {
	defs := []types.EntityRecord{
		def("GET", "/todos"),
		def("POST", "/todos"),
	}
	suite := realSuite(t, "custom_clojure_tests_route_e2e",
		"test/myapp/todo_handler_test.clj", "clojure", cljRingMockTestSrc4749)

	afterOut, edges := runE2ERouteResolve(t, defs, suite)
	if edges < 2 {
		t.Fatalf("expected >=2 e2e route TESTS edges (GET + POST), got %d", edges)
	}
	assertCljRouteEdges(t, edgeTargets(afterOut))
}

// Path-param variant: `(app (mock/request :get "/todos/1"))` matches the
// templated GET /todos/{id} definition via the resolver's concrete-vs-template
// segment matcher.
const cljRingMockParamTestSrc4749 = `(ns myapp.todo-handler-test
  (:require [clojure.test :refer [deftest is]]
            [ring.mock.request :as mock]
            [myapp.handler :refer [app]]))

(deftest get-one
  (let [resp (app (mock/request :get "/todos/1"))]
    (is (= 200 (:status resp)))))
`

func TestIssue4749_ClojureRingMockPathParamLinks(t *testing.T) {
	defs := []types.EntityRecord{
		def("GET", "/todos/{id}"),
	}
	suite := realSuite(t, "custom_clojure_tests_route_e2e",
		"test/myapp/todo_handler_test.clj", "clojure", cljRingMockParamTestSrc4749)

	_, edges := runE2ERouteResolve(t, defs, suite)
	if edges < 1 {
		t.Fatalf("expected a TESTS edge to GET /todos/{id}, got %d", edges)
	}
}

func assertCljRouteEdges(t *testing.T, targets map[string]bool) {
	t.Helper()
	wantGet, wantPost := false, false
	for to := range targets {
		if strings.Contains(to, "GET:/todos") {
			wantGet = true
		}
		if strings.Contains(to, "POST:/todos") {
			wantPost = true
		}
	}
	if !wantGet {
		t.Errorf("expected a TESTS edge to GET /todos; targets=%v", targets)
	}
	if !wantPost {
		t.Errorf("expected a TESTS edge to POST /todos; targets=%v", targets)
	}
}
