package swift

import (
	"context"
	"strings"
	"testing"

	"github.com/cajasmota/archigraph/internal/extractor"
	"github.com/cajasmota/archigraph/internal/types"
)

// #4749 — XCTVapor route-hit capture unit tests (extractor side).

func extractSwiftRouteSuite(t *testing.T, path, src string) (types.EntityRecord, bool) {
	t.Helper()
	ex := &swiftTestRouteE2EExtractor{}
	ents, err := ex.Extract(context.Background(), extractor.FileInput{
		Path: path, Language: "swift", Content: []byte(src),
	})
	if err != nil {
		t.Fatalf("extract: %v", err)
	}
	for _, e := range ents {
		if e.Subtype == "test_suite" {
			return e, true
		}
	}
	return types.EntityRecord{}, false
}

func swiftRouteCallSet(e types.EntityRecord) map[string]bool {
	out := map[string]bool{}
	for _, l := range strings.Split(e.Properties["e2e_route_calls"], "\n") {
		if l != "" {
			out[l] = true
		}
	}
	return out
}

// TestSwiftRouteE2E_TestAndTestable covers app.test(.VERB, "route") and the
// app.testable().test(.VERB, "route") form, across verbs.
func TestSwiftRouteE2E_TestAndTestable(t *testing.T) {
	src := `
import XCTVapor
@testable import App

final class TodoControllerTests: XCTestCase {
    func testListTodos() throws {
        let app = Application(.testing)
        defer { app.shutdown() }
        try app.test(.GET, "todos") { res in
            XCTAssertEqual(res.status, .ok)
        }
        try app.test(.POST, "todos") { res in
            XCTAssertEqual(res.status, .ok)
        }
        try app.testable().test(.DELETE, "todos/1") { res in
            XCTAssertEqual(res.status, .ok)
        }
    }
}
`
	e, ok := extractSwiftRouteSuite(t, "Tests/AppTests/TodoControllerTests.swift", src)
	if !ok {
		t.Fatal("expected a test_suite")
	}
	got := swiftRouteCallSet(e)
	for _, want := range []string{
		"GET /todos",
		"POST /todos",
		"DELETE /todos/1",
	} {
		if !got[want] {
			t.Errorf("missing route call %q; got %v", want, got)
		}
	}
	if e.Properties["framework"] != "xctvapor" {
		t.Errorf("framework = %q, want xctvapor", e.Properties["framework"])
	}
}

// TestSwiftRouteE2E_InterpolatedRoutePrefix captures the static prefix of an
// interpolated route (`"todos/\(id)"` → /todos), dropping the dynamic tail.
func TestSwiftRouteE2E_InterpolatedRoutePrefix(t *testing.T) {
	src := `
import XCTVapor
final class TodoControllerTests: XCTestCase {
    func testGet() throws {
        let app = Application(.testing)
        let id = UUID()
        try app.test(.GET, "todos/\(id)") { res in }
    }
}
`
	e, ok := extractSwiftRouteSuite(t, "Tests/AppTests/TodoControllerTests.swift", src)
	if !ok {
		t.Fatal("expected a test_suite")
	}
	got := swiftRouteCallSet(e)
	if !got["GET /todos"] {
		t.Errorf("expected interpolated route to yield static prefix GET /todos; got %v", got)
	}
}

// TestSwiftRouteE2E_ShapeOnlyNoSuite is the honest exclusion: a test that never
// drives a route (asserts on a model only) emits NO suite.
func TestSwiftRouteE2E_ShapeOnlyNoSuite(t *testing.T) {
	src := `
import XCTest
@testable import App

final class TodoModelTests: XCTestCase {
    func testTitle() {
        let todo = Todo(title: "x")
        XCTAssertEqual(todo.title, "x")
    }
}
`
	if _, ok := extractSwiftRouteSuite(t, "Tests/AppTests/TodoModelTests.swift", src); ok {
		t.Fatal("shape-only test must not emit a route-hit suite")
	}
}

// TestSwiftRouteE2E_NonTestFileIgnored guards that a production controller that
// happens to call `.test(` is not mined (only *Tests.swift / /Tests/ files).
func TestSwiftRouteE2E_NonTestFileIgnored(t *testing.T) {
	src := `
import Vapor
func boot(routes: RoutesBuilder) throws {
    routes.get("todos") { req in "[]" }
}
`
	if _, ok := extractSwiftRouteSuite(t, "Sources/App/Controllers/TodoController.swift", src); ok {
		t.Fatal("non-test file must not emit a route-hit suite")
	}
}
