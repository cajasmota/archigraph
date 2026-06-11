package nim_test

import (
	"context"
	"strings"
	"testing"

	extreg "github.com/cajasmota/archigraph/internal/extractor"

	_ "github.com/cajasmota/archigraph/internal/custom/nim"
)

func fi(path, lang, src string) extreg.FileInput {
	return extreg.FileInput{Path: path, Language: lang, Content: []byte(src)}
}

// TestNimRouteE2E_Capture proves the std/httpclient route helpers are captured
// onto a single test_suite's e2e_route_calls property.
func TestNimRouteE2E_Capture(t *testing.T) {
	src := `
import std/unittest
import std/httpclient

suite "Todos":
  test "lists":
    let client = newHttpClient()
    discard client.get("http://localhost:8080/todos")
  test "shows one":
    let client = newHttpClient()
    discard client.get(baseUrl & "/todos/42")
  test "creates":
    let client = newHttpClient()
    discard client.post("http://localhost:8080/todos", body = "{}")
  test "replaces":
    let client = newHttpClient()
    discard client.request("http://localhost:8080/todos/42", httpMethod = HttpPut)
`
	e, ok := extreg.Get("custom_nim_tests_route_e2e")
	if !ok {
		t.Fatal("custom_nim_tests_route_e2e not registered")
	}
	ents, err := e.Extract(context.Background(), fi("tests/tTodos.nim", "nim", src))
	if err != nil {
		t.Fatalf("extract: %v", err)
	}
	if len(ents) != 1 {
		t.Fatalf("expected exactly 1 test_suite, got %d", len(ents))
	}
	rec := ents[0]
	if rec.Subtype != "test_suite" {
		t.Errorf("expected test_suite, got %q", rec.Subtype)
	}
	calls := rec.Properties["e2e_route_calls"]
	for _, want := range []string{"GET /todos", "GET /todos/42", "POST /todos", "PUT /todos/42"} {
		if !strings.Contains(calls, want) {
			t.Errorf("expected route call %q in %q", want, calls)
		}
	}
}

// TestNimRouteE2E_NonTestExcluded proves a non-test file (production route
// registration) is NOT captured as a test_suite.
func TestNimRouteE2E_NonTestExcluded(t *testing.T) {
	src := `
import jester
routes:
  get "/todos":
    resp "ok"
`
	e, _ := extreg.Get("custom_nim_tests_route_e2e")
	ents, _ := e.Extract(context.Background(), fi("src/routes.nim", "nim", src))
	if len(ents) != 0 {
		t.Fatalf("expected no test_suite for a non-test file, got %d", len(ents))
	}
}

// TestNimRouteE2E_ShapeOnlyTestExcluded proves a unit test that never hits a
// route emits no suite.
func TestNimRouteE2E_ShapeOnlyTestExcluded(t *testing.T) {
	src := `
import std/unittest

suite "Todo":
  test "validates title":
    let t = newTodo("")
    check(not t.valid())
`
	e, _ := extreg.Get("custom_nim_tests_route_e2e")
	ents, _ := e.Extract(context.Background(), fi("tests/tTodo.nim", "nim", src))
	if len(ents) != 0 {
		t.Fatalf("expected no test_suite for a shape-only test, got %d", len(ents))
	}
}

// TestNimRouteE2E_WrongLanguageNoop proves the extractor gates on
// language=="nim".
func TestNimRouteE2E_WrongLanguageNoop(t *testing.T) {
	src := `discard client.get("http://localhost/todos")`
	e, _ := extreg.Get("custom_nim_tests_route_e2e")
	ents, _ := e.Extract(context.Background(), fi("tests/tTodos.nim", "go", src))
	if len(ents) != 0 {
		t.Fatalf("expected no entities for non-nim language, got %d", len(ents))
	}
}
