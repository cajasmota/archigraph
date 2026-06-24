package engine

import "testing"

// Next.js App Router Route Handlers (#5486): every exported HTTP-method handler
// in an `app/.../route.{ts,js,tsx}` file becomes one http_endpoint_definition,
// with method = the export name and path = the `app/`-relative directory
// (route groups `(group)` stripped, dynamic `[seg]` → `{seg}`).

// TestNextRouteHandler_FunctionForm covers the `export async function GET/POST`
// declaration form in app/api/users/route.ts → one endpoint per verb.
func TestNextRouteHandler_FunctionForm(t *testing.T) {
	src := `
export async function GET(request: Request) { return Response.json([]) }
export async function POST(request: Request) { return Response.json({}) }
`
	ids, _ := runDetect(t, "typescript", "app/api/users/route.ts", src)
	want := []string{
		"http:GET:/api/users",
		"http:POST:/api/users",
	}
	requireContains(t, ids, want, "next-route-handler-function-form")
}

// TestNextRouteHandler_DynamicSegment covers a dynamic `[id]` segment in
// app/api/users/[id]/route.ts → path normalised to `{id}`.
func TestNextRouteHandler_DynamicSegment(t *testing.T) {
	src := `export async function GET(req: Request, ctx: { params: { id: string } }) { return Response.json({}) }`
	ids, _ := runDetect(t, "typescript", "app/api/users/[id]/route.ts", src)
	want := []string{"http:GET:/api/users/{id}"}
	requireContains(t, ids, want, "next-route-handler-dynamic-segment")
}

// TestNextRouteHandler_RouteGroup covers a route group `(admin)` — invisible to
// routing — in app/(admin)/api/x/route.ts → group stripped from the path.
func TestNextRouteHandler_RouteGroup(t *testing.T) {
	src := `export async function GET(req: Request) { return Response.json({}) }`
	ids, _ := runDetect(t, "typescript", "app/(admin)/api/x/route.ts", src)
	want := []string{"http:GET:/api/x"}
	requireContains(t, ids, want, "next-route-handler-route-group")
}

// TestNextRouteHandler_ConstForm covers the `export const GET = (...) => {}`
// arrow/function-expression export form.
func TestNextRouteHandler_ConstForm(t *testing.T) {
	src := `
export const GET = async (req: Request) => Response.json([])
export const DELETE = async (req: Request) => new Response(null, { status: 204 })
`
	ids, _ := runDetect(t, "typescript", "app/api/items/route.ts", src)
	want := []string{
		"http:GET:/api/items",
		"http:DELETE:/api/items",
	}
	requireContains(t, ids, want, "next-route-handler-const-form")
}

// TestNextRouteHandler_NonApiRoute covers a Route Handler NOT under `api/` —
// App Router permits `route.ts` anywhere under `app/` (#5486).
func TestNextRouteHandler_NonApiRoute(t *testing.T) {
	src := `export async function GET(req: Request) { return Response.json({}) }`
	ids, _ := runDetect(t, "typescript", "app/feed/route.ts", src)
	want := []string{"http:GET:/feed"}
	requireContains(t, ids, want, "next-route-handler-non-api")
}

// TestNextRouteHandler_PageNotEndpoint proves a `page.tsx` exporting a `GET`
// function is NOT a Route Handler and produces no http_endpoint_definition —
// the gating is on the `route.*` basename, not arbitrary verb exports.
func TestNextRouteHandler_PageNotEndpoint(t *testing.T) {
	src := `export async function GET(req: Request) { return Response.json({}) }`
	ids, _ := runDetect(t, "typescript", "app/dashboard/page.tsx", src)
	requireNotContains(t, ids, []string{
		"http:GET:/dashboard",
		"http:GET:/dashboard/page",
	}, "next-page-not-endpoint")
}
