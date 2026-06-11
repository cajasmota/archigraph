package engine

import (
	"testing"
)

// TestJester_BasicRoute covers the common Jester `routes:`-block shape:
// get "/todos": ... / post "/todos": ...
func TestJester_BasicRoute(t *testing.T) {
	src := `
import jester

routes:
  get "/todos":
    resp Todo.all().toJson()
  post "/todos":
    resp Todo.create(request.body)

runForever()
`
	ids, _ := runDetect(t, "nim", "src/routes.nim", src)
	requireContains(t, ids, []string{
		"http:GET:/todos",
		"http:POST:/todos",
	}, "jester-basic-route")
}

// TestJester_PathParam covers the Jester `@param` AT-prefixed dynamic segment:
// get "/todos/@id": → GET /todos/{id}.
func TestJester_PathParam(t *testing.T) {
	src := `
import jester

routes:
  get "/todos/@id":
    resp Todo.find(@"id").toJson()
  delete "/todos/@id":
    Todo.delete(@"id")
`
	ids, _ := runDetect(t, "nim", "src/todos.nim", src)
	requireContains(t, ids, []string{
		"http:GET:/todos/{id}",
		"http:DELETE:/todos/{id}",
	}, "jester-path-param")
}

// TestPrologue_VerbMethodRoute covers Prologue `app.get("/users/{id}", h)` /
// `app.post("/users", h)` curly-brace registrations.
func TestPrologue_VerbMethodRoute(t *testing.T) {
	src := `
import prologue

let app = newApp()
app.get("/users/{id}", getUser)
app.post("/users", createUser)
app.run()
`
	ids, _ := runDetect(t, "nim", "src/app.nim", src)
	requireContains(t, ids, []string{
		"http:GET:/users/{id}",
		"http:POST:/users",
	}, "prologue-verb-method")
}

// TestPrologue_AddRoute covers Prologue's `addRoute("/x", handler, HttpGet)`
// form where the verb is the third argument as an Http<Verb> enum value.
func TestPrologue_AddRoute(t *testing.T) {
	src := `
import prologue

let app = newApp()
app.addRoute("/health", healthCheck, HttpGet)
app.addRoute("/items", createItem, HttpPost)
`
	ids, _ := runDetect(t, "nim", "src/routes.nim", src)
	requireContains(t, ids, []string{
		"http:GET:/health",
		"http:POST:/items",
	}, "prologue-add-route")
}

// TestJester_InterpolatedRouteDropped asserts a non-static (concatenated /
// interpolated) Jester path is NOT synthesized — no fabricated endpoint.
func TestJester_InterpolatedRouteDropped(t *testing.T) {
	src := `
import jester

const prefix = "/api"
routes:
  get prefix & "/x":
    resp "dynamic"
`
	ids, _ := runDetect(t, "nim", "src/dyn.nim", src)
	for _, id := range ids {
		if id == "http:GET:/x" || id == "http:GET:/api/x" {
			t.Fatalf("interpolated route should not synthesize an endpoint; got %q", id)
		}
	}
}

// TestJester_NonWebFileIgnored asserts a plain Nim file with a `get` proc call
// but no Jester/Prologue web marker produces no endpoints.
func TestJester_NonWebFileIgnored(t *testing.T) {
	src := `
proc handle() =
  let x = cache.get("key")
  echo x
`
	ids, _ := runDetect(t, "nim", "src/util.nim", src)
	for _, id := range ids {
		if len(id) >= 5 && id[:5] == "http:" {
			t.Fatalf("non-web Nim file should synthesize no endpoint; got %q", id)
		}
	}
}
