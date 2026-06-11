package crystal_test

import (
	"context"
	"strings"
	"testing"

	extreg "github.com/cajasmota/archigraph/internal/extractor"

	_ "github.com/cajasmota/archigraph/internal/custom/crystal"
)

func fi(path, lang, src string) extreg.FileInput {
	return extreg.FileInput{Path: path, Language: lang, Content: []byte(src)}
}

// TestCrystalRouteE2E_Capture proves the spec-kemal route helpers are captured
// onto a single test_suite's e2e_route_calls property.
func TestCrystalRouteE2E_Capture(t *testing.T) {
	src := `
require "./spec_helper"

describe "Todos" do
  it "lists" do
    get "/todos"
    response.status_code.should eq 200
  end

  it "shows one" do
    get "/todos/#{id}"
  end

  it "creates" do
    post "/todos"
  end
end
`
	e, ok := extreg.Get("custom_crystal_tests_route_e2e")
	if !ok {
		t.Fatal("custom_crystal_tests_route_e2e not registered")
	}
	ents, err := e.Extract(context.Background(), fi("spec/todos_spec.cr", "crystal", src))
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
	for _, want := range []string{"GET /todos", "POST /todos"} {
		if !strings.Contains(calls, want) {
			t.Errorf("expected route call %q in %q", want, calls)
		}
	}
	// Interpolated route "/todos/#{id}" is truncated to "/todos" (static prefix),
	// which dedupes against the existing "GET /todos" — so no extra GET line.
}

// TestCrystalRouteE2E_NonSpecExcluded proves a non-spec file (production route
// registration) is NOT captured as a test_suite.
func TestCrystalRouteE2E_NonSpecExcluded(t *testing.T) {
	src := `
require "kemal"
get "/todos" do |env|
  Todo.all.to_json
end
`
	e, _ := extreg.Get("custom_crystal_tests_route_e2e")
	ents, _ := e.Extract(context.Background(), fi("src/routes.cr", "crystal", src))
	if len(ents) != 0 {
		t.Fatalf("expected no test_suite for a non-spec file, got %d", len(ents))
	}
}

// TestCrystalRouteE2E_ShapeOnlySpecExcluded proves a unit/model spec that never
// hits a route emits no suite.
func TestCrystalRouteE2E_ShapeOnlySpecExcluded(t *testing.T) {
	src := `
describe Todo do
  it "validates title" do
    todo = Todo.new(title: "")
    todo.valid?.should be_false
  end
end
`
	e, _ := extreg.Get("custom_crystal_tests_route_e2e")
	ents, _ := e.Extract(context.Background(), fi("spec/models/todo_spec.cr", "crystal", src))
	if len(ents) != 0 {
		t.Fatalf("expected no test_suite for a shape-only spec, got %d", len(ents))
	}
}

// TestCrystalRouteE2E_WrongLanguageNoop proves the extractor gates on
// language=="crystal".
func TestCrystalRouteE2E_WrongLanguageNoop(t *testing.T) {
	src := `get "/todos"`
	e, _ := extreg.Get("custom_crystal_tests_route_e2e")
	ents, _ := e.Extract(context.Background(), fi("spec/todos_spec.cr", "ruby", src))
	if len(ents) != 0 {
		t.Fatalf("expected no entities for non-crystal language, got %d", len(ents))
	}
}
