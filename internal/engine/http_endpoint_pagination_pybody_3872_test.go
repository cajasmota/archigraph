// Python separate-handler body locator (#3872).
//
// Out-of-line route registration (aiohttp `app.router.add_get("/x", handler)`,
// starlette `Route("/x", handler)`) anchors the endpoint StartLine at the
// REGISTRATION line, so the StartLine-anchored handler-body window never reaches
// the separately-defined handler. These tests drive the full synthesis +
// pagination pass and assert the EXACT pagination posture now stamps when the
// body is located via the endpoint's source_handler reference.
package engine

import (
	"strings"
	"testing"
)

// aiohttp `app.router.add_get(path, handler)` with the handler defined
// out-of-line above the registration: limit+offset → offset style.
func TestPagination_AiohttpOutOfLineOffset(t *testing.T) {
	src := `
from aiohttp import web

async def list_items(request):
    limit = request.query.get("limit")
    offset = request.query.get("offset")
    return web.json_response([])

app = web.Application()
app.router.add_get("/items", list_items)
`
	eps := deprecProps(t, "python", "app/routes.py", src)
	e := mustEndpoint(t, eps, "GET /items")
	if e.Properties["paginated"] != "true" {
		t.Fatalf("paginated=%q want true (props: %v)", e.Properties["paginated"], e.Properties)
	}
	if e.Properties["pagination_style"] != "offset" {
		t.Fatalf("pagination_style=%q want offset", e.Properties["pagination_style"])
	}
	if got := e.Properties["pagination_params"]; got != "limit,offset" {
		t.Fatalf("pagination_params=%q want limit,offset", got)
	}
	if got := e.Properties["pagination_source"]; got != "request.query" {
		t.Fatalf("pagination_source=%q want request.query", got)
	}
}

// aiohttp `app.router.add_route("GET", path, handler)` generic form with a
// cursor token in the out-of-line handler → cursor style.
func TestPagination_AiohttpAddRouteCursor(t *testing.T) {
	src := `
from aiohttp import web

async def feed(request):
    cursor = request.query.get("cursor")
    return web.json_response([])

app = web.Application()
app.router.add_route("GET", "/feed", feed)
`
	eps := deprecProps(t, "python", "app/routes.py", src)
	e := mustEndpoint(t, eps, "GET /feed")
	if e.Properties["paginated"] != "true" {
		t.Fatalf("paginated=%q want true (props: %v)", e.Properties["paginated"], e.Properties)
	}
	if e.Properties["pagination_style"] != "cursor" {
		t.Fatalf("pagination_style=%q want cursor", e.Properties["pagination_style"])
	}
	if got := e.Properties["pagination_params"]; got != "cursor" {
		t.Fatalf("pagination_params=%q want cursor", got)
	}
}

// starlette `Route(path, handler, methods=[...])` with the handler defined
// out-of-line, reading from request.query_params → offset style. (Starlette
// already anchors at the def line, but the source_handler fallback must also
// resolve it correctly and must not regress the posture.)
func TestPagination_StarletteOutOfLineOffset(t *testing.T) {
	src := `
from starlette.applications import Starlette
from starlette.responses import JSONResponse
from starlette.routing import Route

async def list_users(request):
    limit = request.query_params.get("limit")
    offset = request.query_params.get("offset")
    return JSONResponse([])

routes = [
    Route("/users", list_users, methods=["GET"]),
]
app = Starlette(routes=routes)
`
	eps := deprecProps(t, "python", "app/main.py", src)
	e := mustEndpoint(t, eps, "GET /users")
	if e.Properties["paginated"] != "true" {
		t.Fatalf("paginated=%q want true (props: %v)", e.Properties["paginated"], e.Properties)
	}
	if e.Properties["pagination_style"] != "offset" {
		t.Fatalf("pagination_style=%q want offset", e.Properties["pagination_style"])
	}
	if got := e.Properties["pagination_params"]; got != "limit,offset" {
		t.Fatalf("pagination_params=%q want limit,offset", got)
	}
}

// Negative / honest-partial: an out-of-line aiohttp handler that reads NO
// pagination param yields no posture. Guards against the locator leaking a
// false positive (e.g. by scanning a sibling function's body).
func TestPagination_AiohttpOutOfLineNoPosture(t *testing.T) {
	src := `
from aiohttp import web

async def get_item(request):
    item_id = request.match_info.get("id")
    return web.json_response({})

# A sibling function that DOES paginate — must NOT leak into get_item's verdict.
async def other(request):
    limit = request.query.get("limit")
    offset = request.query.get("offset")
    return web.json_response([])

app = web.Application()
app.router.add_get("/items/{id}", get_item)
`
	eps := deprecProps(t, "python", "app/routes.py", src)
	e := mustEndpoint(t, eps, "GET /items/{id}")
	if got := e.Properties["paginated"]; got != "" {
		t.Fatalf("paginated=%q want unset (sibling pagination leaked? props: %v)", got, e.Properties)
	}
	if got := e.Properties["pagination_style"]; got != "" {
		t.Fatalf("pagination_style=%q want unset", got)
	}
}

// Boundary: a lone `limit` (no offset/page/cursor companion) is ambiguous and
// must NOT stamp, even when reached via the out-of-line locator.
func TestPagination_AiohttpOutOfLineLoneLimitAmbiguous(t *testing.T) {
	src := `
from aiohttp import web

async def search(request):
    limit = request.query.get("limit")
    return web.json_response([])

app = web.Application()
app.router.add_get("/search", search)
`
	eps := deprecProps(t, "python", "app/routes.py", src)
	e := mustEndpoint(t, eps, "GET /search")
	if got := e.Properties["paginated"]; got != "" {
		t.Fatalf("paginated=%q want unset (lone limit is ambiguous; props: %v)", got, e.Properties)
	}
}

// Unit: findPyHandlerBody captures only the named function's own indented block,
// stopping at the dedent — a following sibling def must be excluded.
func TestFindPyHandlerBody_BlockBounded(t *testing.T) {
	src := `async def list_items(request):
    limit = request.query.get("limit")
    offset = request.query.get("offset")
    return []

async def other(request):
    cursor = request.query.get("cursor")
    return []
`
	body := findPyHandlerBody(src, "list_items")
	if body == "" {
		t.Fatal("findPyHandlerBody returned empty")
	}
	if !strings.Contains(body, `request.query.get("limit")`) || !strings.Contains(body, `request.query.get("offset")`) {
		t.Fatalf("body missing list_items reads: %q", body)
	}
	if strings.Contains(body, "cursor") {
		t.Fatalf("body leaked the sibling `other` block: %q", body)
	}
}
