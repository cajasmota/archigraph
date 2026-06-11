package clojure

import (
	"context"
	"strings"
	"testing"

	"github.com/cajasmota/archigraph/internal/extractor"
)

// #4749 — the Clojure route-hit extractor emits one test_suite per clojure.test
// file carrying the captured `VERB route` pairs on e2e_route_calls.

func runClojureTestRouteE2E(t *testing.T, path, src string) []string {
	t.Helper()
	ext := &clojureTestRouteE2EExtractor{}
	ents, err := ext.Extract(context.Background(), extractor.FileInput{
		Path:     path,
		Language: "clojure",
		Content:  []byte(src),
	})
	if err != nil {
		t.Fatalf("Extract error: %v", err)
	}
	if len(ents) == 0 {
		return nil
	}
	if len(ents) != 1 {
		t.Fatalf("expected exactly one test_suite, got %d", len(ents))
	}
	e := ents[0]
	if e.Subtype != "test_suite" {
		t.Fatalf("expected Subtype=test_suite, got %q", e.Subtype)
	}
	raw := e.Properties["e2e_route_calls"]
	if raw == "" {
		return nil
	}
	return strings.Split(raw, "\n")
}

func TestClojureTestRouteE2E_RingMock(t *testing.T) {
	src := `(ns myapp.handler-test
  (:require [clojure.test :refer [deftest is]]
            [ring.mock.request :as mock]
            [myapp.handler :refer [app]]))

(deftest routes
  (let [r (app (mock/request :get "/todos"))] (is (= 200 (:status r))))
  (let [r (app (ring.mock.request/request :post "/todos" {:t 1}))] (is (= 201 (:status r))))
  (let [r (app (mock/request :delete "/todos/:id"))] (is (= 204 (:status r)))))
`
	got := runClojureTestRouteE2E(t, "test/myapp/handler_test.clj", src)
	want := map[string]bool{"GET /todos": true, "POST /todos": true, "DELETE /todos/:id": true}
	for _, line := range got {
		delete(want, line)
	}
	if len(want) != 0 {
		t.Fatalf("missing route calls %v; got %v", want, got)
	}
}

func TestClojureTestRouteE2E_Peridot(t *testing.T) {
	src := `(ns myapp.session-test
  (:require [clojure.test :refer [deftest is]]
            [peridot.core :refer [session request]]))

(deftest health
  (-> (session app)
      (request "/health" :request-method :get)))
`
	got := runClojureTestRouteE2E(t, "test/myapp/session_test.clj", src)
	found := false
	for _, line := range got {
		if line == "GET /health" {
			found = true
		}
	}
	if !found {
		t.Fatalf("expected GET /health from peridot form; got %v", got)
	}
}

// Interpolated / built routes are dropped; a non-test file emits nothing.
func TestClojureTestRouteE2E_HonestExclusion(t *testing.T) {
	// Built route via (str ...) — not a string literal, dropped.
	src := `(ns myapp.h-test
  (:require [clojure.test :refer [deftest is]]
            [ring.mock.request :as mock]))
(deftest dyn
  (let [id 1
        r (app (mock/request :get (str "/todos/" id)))]
    (is (= 200 (:status r)))))
`
	if got := runClojureTestRouteE2E(t, "test/myapp/h_test.clj", src); len(got) != 0 {
		t.Fatalf("expected no route calls for built route, got %v", got)
	}
	// Non-test file (production handler) must not emit a suite even if it
	// contains a request literal.
	prod := `(ns myapp.handler
  (:require [ring.mock.request :as mock]))
(def x (mock/request :get "/todos"))
`
	if got := runClojureTestRouteE2E(t, "src/myapp/handler.clj", prod); got != nil {
		t.Fatalf("expected no suite for non-test file, got %v", got)
	}
}
