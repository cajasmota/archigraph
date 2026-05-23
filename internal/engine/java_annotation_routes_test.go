package engine

import (
	"sort"
	"strings"
	"testing"
)

// helper: build a reader from a path->src map.
func mapReader(m map[string]string) JavaAnnotationFileReader {
	return func(p string) []byte {
		s, ok := m[p]
		if !ok {
			return nil
		}
		return []byte(s)
	}
}

func endpointIDs(records []recordLike) []string {
	out := make([]string, 0, len(records))
	for _, r := range records {
		out = append(out, r.ID)
	}
	sort.Strings(out)
	return out
}

type recordLike struct {
	ID         string
	Props      map[string]string
	SourceFile string
}

func collect(t *testing.T, files map[string]string) []recordLike {
	t.Helper()
	paths := make([]string, 0, len(files))
	for p := range files {
		paths = append(paths, p)
	}
	sort.Strings(paths)
	got := ApplyJavaAnnotationRoutes(paths, mapReader(files))
	out := make([]recordLike, 0, len(got))
	for _, e := range got {
		out = append(out, recordLike{ID: e.ID, Props: e.Properties, SourceFile: e.SourceFile})
	}
	return out
}

func TestApplyJavaAnnotationRoutes_JAXRSClassPlusMethodPath(t *testing.T) {
	src := `package com.example;
import jakarta.ws.rs.*;

@Path("/products")
public class ProductsController {
    @GET
    public Object list() { return null; }

    @GET
    @Path("/{id}")
    public Object get() { return null; }

    @POST
    @Path("/upload")
    public Object upload() { return null; }
}
`
	got := collect(t, map[string]string{"Products.java": src})
	ids := endpointIDs(got)
	want := []string{
		"http:GET:/products",
		"http:GET:/products/{id}",
		"http:POST:/products/upload",
	}
	sort.Strings(want)
	if strings.Join(ids, "|") != strings.Join(want, "|") {
		t.Fatalf("ids = %v\nwant %v", ids, want)
	}
	for _, r := range got {
		if r.Props["framework"] != "jaxrs" {
			t.Errorf("expected framework=jaxrs, got %q for %s", r.Props["framework"], r.ID)
		}
		if !strings.HasPrefix(r.Props["source_handler"], "SCOPE.Operation:ProductsController.") {
			t.Errorf("bad source_handler %q on %s", r.Props["source_handler"], r.ID)
		}
	}
}

func TestApplyJavaAnnotationRoutes_JAXRSMethodOnlyNoClassPrefix(t *testing.T) {
	src := `package com.example;
import jakarta.ws.rs.*;

public class StatusResource {
    @GET
    @Path("/status")
    public Object status() { return null; }
}
`
	got := collect(t, map[string]string{"Status.java": src})
	ids := endpointIDs(got)
	if len(ids) != 1 || ids[0] != "http:GET:/status" {
		t.Fatalf("ids = %v, want [http:GET:/status]", ids)
	}
}

func TestApplyJavaAnnotationRoutes_SpringRequestMappingClassGetMappingMethod(t *testing.T) {
	src := `package com.example;
import org.springframework.web.bind.annotation.*;

@RequestMapping("/api/users")
@RestController
public class UserController {
    @GetMapping("/{id}")
    public Object get() { return null; }

    @PostMapping
    public Object create() { return null; }
}
`
	got := collect(t, map[string]string{"User.java": src})
	ids := endpointIDs(got)
	want := []string{
		"http:GET:/api/users/{id}",
		"http:POST:/api/users",
	}
	sort.Strings(want)
	if strings.Join(ids, "|") != strings.Join(want, "|") {
		t.Fatalf("ids = %v\nwant %v", ids, want)
	}
	for _, r := range got {
		if r.Props["framework"] != "spring" {
			t.Errorf("expected framework=spring, got %q for %s", r.Props["framework"], r.ID)
		}
	}
}

func TestApplyJavaAnnotationRoutes_SpringRequestMappingWithMethodKeyword(t *testing.T) {
	src := `package com.example;
import org.springframework.web.bind.annotation.*;

@RequestMapping("/api/items")
@RestController
public class ItemController {
    @RequestMapping(value = "/{id}", method = RequestMethod.POST)
    public Object update() { return null; }
}
`
	got := collect(t, map[string]string{"Item.java": src})
	ids := endpointIDs(got)
	if len(ids) != 1 || ids[0] != "http:POST:/api/items/{id}" {
		t.Fatalf("ids = %v, want [http:POST:/api/items/{id}]", ids)
	}
}

func TestApplyJavaAnnotationRoutes_PathParamRegexStripped(t *testing.T) {
	// Spring + JAX-RS both allow regex constraints inside {name:regex}.
	// The canonicalizer should drop the constraint.
	src := `package com.example;
import jakarta.ws.rs.*;

@Path("/things")
public class ThingsResource {
    @GET
    @Path("/{name:[a-z]+}")
    public Object byName() { return null; }
}
`
	got := collect(t, map[string]string{"Things.java": src})
	ids := endpointIDs(got)
	if len(ids) != 1 || ids[0] != "http:GET:/things/{name}" {
		t.Fatalf("ids = %v, want [http:GET:/things/{name}]", ids)
	}
}

func TestApplyJavaAnnotationRoutes_MultipleVerbsOnSamePath(t *testing.T) {
	src := `package com.example;
import jakarta.ws.rs.*;

@Path("/widgets")
public class WidgetController {
    @GET
    @Path("/{id}")
    public Object get() { return null; }

    @DELETE
    @Path("/{id}")
    public Object remove() { return null; }
}
`
	got := collect(t, map[string]string{"Widget.java": src})
	ids := endpointIDs(got)
	want := []string{"http:DELETE:/widgets/{id}", "http:GET:/widgets/{id}"}
	if strings.Join(ids, "|") != strings.Join(want, "|") {
		t.Fatalf("ids = %v\nwant %v", ids, want)
	}
}

func TestApplyJavaAnnotationRoutes_ConsumesProducesSurfaced(t *testing.T) {
	src := `package com.example;
import jakarta.ws.rs.*;
import jakarta.ws.rs.core.MediaType;

@Path("/files")
@Consumes(MediaType.APPLICATION_JSON)
@Produces(MediaType.APPLICATION_JSON)
public class FilesController {
    @POST
    @Path("/upload")
    @Consumes(MediaType.MULTIPART_FORM_DATA)
    public Object upload() { return null; }
}
`
	got := collect(t, map[string]string{"Files.java": src})
	if len(got) != 1 {
		t.Fatalf("expected 1 endpoint, got %d", len(got))
	}
	r := got[0]
	if !strings.Contains(r.Props["consumes"], "MULTIPART_FORM_DATA") {
		t.Errorf("expected method-level consumes override, got %q", r.Props["consumes"])
	}
	if !strings.Contains(r.Props["produces"], "APPLICATION_JSON") {
		t.Errorf("expected class-level produces inherited, got %q", r.Props["produces"])
	}
}

func TestApplyJavaAnnotationRoutes_NonRouteFileSkipped(t *testing.T) {
	src := `package com.example;
public class PojoBag {
    private int x;
    public int getX() { return x; }
}
`
	got := collect(t, map[string]string{"Pojo.java": src})
	if len(got) != 0 {
		t.Fatalf("expected 0 endpoints for non-route file, got %d", len(got))
	}
}

func TestApplyJavaAnnotationRoutes_SpringSpecialisedMappingInlinePath(t *testing.T) {
	// No class-level @RequestMapping — only specialised method mappings.
	src := `package com.example;
import org.springframework.web.bind.annotation.*;

@RestController
public class PingController {
    @GetMapping("/ping")
    public String ping() { return "pong"; }
}
`
	got := collect(t, map[string]string{"Ping.java": src})
	ids := endpointIDs(got)
	if len(ids) != 1 || ids[0] != "http:GET:/ping" {
		t.Fatalf("ids = %v, want [http:GET:/ping]", ids)
	}
}

// ---------------------------------------------------------------------------
// Regression tests for #682 — kind/name mismatch
// ---------------------------------------------------------------------------

// TestApplyJavaAnnotationRoutes_Issue682_SourceHandlerKindAndName verifies
// that the emitted source_handler property uses "SCOPE.Operation" as the
// kind and "ClassName.methodName" as the name format. This is the exact
// format the Java AST extractor emits, so the resolver can wire the
// IMPLEMENTS edge. The old synthesizeJAXRS emitted "Controller:methodName"
// which resolved to nothing and dropped all 60 fixture-d synthetics.
func TestApplyJavaAnnotationRoutes_Issue682_SourceHandlerKindAndName(t *testing.T) {
	src := `package com.example.quarkus;
import jakarta.ws.rs.*;
import jakarta.ws.rs.core.Response;

@Path("/products")
public class ProductsController {
    @GET
    public Response listProducts() { return null; }

    @GET
    @Path("/{id}")
    public Response getProduct() { return null; }

    @POST
    public Response createProduct() { return null; }
}
`
	got := collect(t, map[string]string{"ProductsController.java": src})
	if len(got) == 0 {
		t.Fatal("expected endpoints, got none")
	}
	for _, r := range got {
		handler := r.Props["source_handler"]
		// Must start with "SCOPE.Operation:" (not "Controller:")
		if !strings.HasPrefix(handler, "SCOPE.Operation:") {
			t.Errorf("[#682] source_handler kind wrong: got %q, want prefix SCOPE.Operation:", handler)
		}
		// Must use "ClassName.methodName" format (not bare "methodName")
		// e.g. "SCOPE.Operation:ProductsController.listProducts"
		suffix := strings.TrimPrefix(handler, "SCOPE.Operation:")
		if !strings.HasPrefix(suffix, "ProductsController.") {
			t.Errorf("[#682] source_handler name format wrong: got %q, want ProductsController.<methodName>", handler)
		}
		parts := strings.SplitN(suffix, ".", 2)
		if len(parts) != 2 || parts[1] == "" {
			t.Errorf("[#682] source_handler name must be ClassName.methodName, got %q", handler)
		}
	}

	// Verify exact expected handlers.
	handlerSet := map[string]bool{}
	for _, r := range got {
		handlerSet[r.Props["source_handler"]] = true
	}
	wantHandlers := []string{
		"SCOPE.Operation:ProductsController.listProducts",
		"SCOPE.Operation:ProductsController.getProduct",
		"SCOPE.Operation:ProductsController.createProduct",
	}
	for _, wh := range wantHandlers {
		if !handlerSet[wh] {
			t.Errorf("[#682] missing expected source_handler %q; got set: %v", wh, handlerSet)
		}
	}
}

// TestApplyJavaAnnotationRoutes_Issue682_OldFormatNotEmitted explicitly
// verifies that the OLD broken format "Controller:methodName" is never
// emitted. This test would have caught the regression.
func TestApplyJavaAnnotationRoutes_Issue682_OldFormatNotEmitted(t *testing.T) {
	src := `package com.example;
import jakarta.ws.rs.*;

@Path("/inventory")
public class InventoryController {
    @GET
    @Path("/{id}")
    public Object getItem() { return null; }

    @DELETE
    @Path("/{id}")
    public Object deleteItem() { return null; }
}
`
	got := collect(t, map[string]string{"InventoryController.java": src})
	for _, r := range got {
		handler := r.Props["source_handler"]
		if strings.HasPrefix(handler, "Controller:") {
			t.Errorf("[#682] old broken source_handler format emitted: %q (endpoint %s)", handler, r.ID)
		}
		// Verify it's the correct format
		if !strings.HasPrefix(handler, "SCOPE.Operation:InventoryController.") {
			t.Errorf("[#682] expected SCOPE.Operation:InventoryController.<method>, got %q (endpoint %s)", handler, r.ID)
		}
	}
}

// ---------------------------------------------------------------------------
// Regression tests for #683 — annotation budget exhaustion
// ---------------------------------------------------------------------------

// TestApplyJavaAnnotationRoutes_Issue683_PermitAllBetweenVerbAndPath is
// the concrete reproducer from issue #683. When @PermitAll appears between
// @POST and @Path("/login"), the old {0,3} regex budget was consumed before
// reaching @Path, producing "/auth" instead of "/auth/login".
func TestApplyJavaAnnotationRoutes_Issue683_PermitAllBetweenVerbAndPath(t *testing.T) {
	src := `package com.example.quarkus.auth;
import jakarta.ws.rs.*;
import jakarta.annotation.security.PermitAll;
import org.eclipse.microprofile.openapi.annotations.Operation;

@Path("/auth")
public class AuthController {

    @POST
    @PermitAll
    @Path("/login")
    @Operation(summary = "Login an existing user")
    public LoginResponse login(@Valid LoginRequest request) { return null; }

    @POST
    @PermitAll
    @Path("/register")
    @Operation(summary = "Register a new user")
    public RegisterResponse register(@Valid RegisterRequest request) { return null; }
}
`
	got := collect(t, map[string]string{"AuthController.java": src})
	ids := endpointIDs(got)
	want := []string{
		"http:POST:/auth/login",
		"http:POST:/auth/register",
	}
	sort.Strings(want)
	if strings.Join(ids, "|") != strings.Join(want, "|") {
		t.Fatalf("[#683] ids = %v\nwant %v\n(old bug: @PermitAll between @POST and @Path exhausted {0,3} budget)", ids, want)
	}
	// Also verify the source_handler is correctly formed
	for _, r := range got {
		handler := r.Props["source_handler"]
		if !strings.HasPrefix(handler, "SCOPE.Operation:AuthController.") {
			t.Errorf("[#683] source_handler wrong: got %q for endpoint %s", handler, r.ID)
		}
	}
}

// ---------------------------------------------------------------------------
// Tests for #1909 — JAX-RS request body inference
// ---------------------------------------------------------------------------

// TestApplyJavaAnnotationRoutes_Issue1909_JAXRSImplicitBody verifies that a
// JAX-RS PUT method whose only non-annotated parameter is treated as the
// request body emits request_body_type correctly.
//
// Fixture is a client-fixture-X style transfer confirmation endpoint:
//
//	@PUT
//	@Path("/transfers/confirm/{transferId}")
//	public Response confirm(@PathParam("transferId") String id, ConfirmRequest body) { ... }
//
// Expected: request_body_type = "ConfirmRequest", request_body_param_name = "body"
func TestApplyJavaAnnotationRoutes_Issue1909_JAXRSImplicitBody(t *testing.T) {
	src := `package com.example.banking;
import jakarta.ws.rs.*;
import jakarta.ws.rs.core.Response;

@Path("/transfers")
public class TransferResource {

    @PUT
    @Path("/confirm/{transferId}")
    public Response confirm(@PathParam("transferId") String id, ConfirmRequest body) {
        return Response.ok().build();
    }

    @GET
    @Path("/{id}")
    public Response getTransfer(@PathParam("id") String id) {
        return Response.ok().build();
    }
}
`
	got := collect(t, map[string]string{"TransferResource.java": src})
	// Find the PUT endpoint.
	var putEP *recordLike
	for i := range got {
		if got[i].ID == "http:PUT:/transfers/confirm/{transferId}" {
			putEP = &got[i]
			break
		}
	}
	if putEP == nil {
		ids := endpointIDs(got)
		t.Fatalf("[#1909] PUT endpoint not found; got IDs: %v", ids)
	}
	if putEP.Props["request_body_type"] != "ConfirmRequest" {
		t.Errorf("[#1909] PUT: want request_body_type=ConfirmRequest, got %q", putEP.Props["request_body_type"])
	}
	if putEP.Props["request_body_param_name"] != "body" {
		t.Errorf("[#1909] PUT: want request_body_param_name=body, got %q", putEP.Props["request_body_param_name"])
	}
	// GET endpoint must NOT have request_body_type set.
	for _, r := range got {
		if r.ID == "http:GET:/transfers/{id}" && r.Props["request_body_type"] != "" {
			t.Errorf("[#1909] GET endpoint should not have request_body_type, got %q", r.Props["request_body_type"])
		}
	}
}

// TestApplyJavaAnnotationRoutes_Issue1909_SpringExplicitRequestBody verifies
// that a Spring controller with @RequestBody emits request_body_type correctly.
func TestApplyJavaAnnotationRoutes_Issue1909_SpringExplicitRequestBody(t *testing.T) {
	src := `package com.example.api;
import org.springframework.web.bind.annotation.*;

@RestController
@RequestMapping("/api/orders")
public class OrderController {

    @PostMapping
    public OrderResponse create(@RequestBody CreateOrderRequest req) {
        return new OrderResponse();
    }

    @PutMapping("/{id}")
    public OrderResponse update(@PathVariable Long id, @RequestBody UpdateOrderRequest req) {
        return new OrderResponse();
    }
}
`
	got := collect(t, map[string]string{"OrderController.java": src})
	// POST endpoint.
	var postEP *recordLike
	var putEP *recordLike
	for i := range got {
		if got[i].ID == "http:POST:/api/orders" {
			postEP = &got[i]
		}
		if got[i].ID == "http:PUT:/api/orders/{id}" {
			putEP = &got[i]
		}
	}
	if postEP == nil {
		ids := endpointIDs(got)
		t.Fatalf("[#1909] POST endpoint not found; got IDs: %v", ids)
	}
	if postEP.Props["request_body_type"] != "CreateOrderRequest" {
		t.Errorf("[#1909] POST: want request_body_type=CreateOrderRequest, got %q", postEP.Props["request_body_type"])
	}
	if putEP == nil {
		ids := endpointIDs(got)
		t.Fatalf("[#1909] PUT endpoint not found; got IDs: %v", ids)
	}
	if putEP.Props["request_body_type"] != "UpdateOrderRequest" {
		t.Errorf("[#1909] PUT: want request_body_type=UpdateOrderRequest, got %q", putEP.Props["request_body_type"])
	}
}

// TestApplyJavaAnnotationRoutes_Issue683_QuarkusDeepAnnotationStack verifies
// that a realistic Quarkus annotation stack with 5+ annotations between
// @GET and @Path is handled correctly. Covers @RateLimited, @Produces,
// @ApiResponse, @Tag all appearing before @Path.
func TestApplyJavaAnnotationRoutes_Issue683_QuarkusDeepAnnotationStack(t *testing.T) {
	src := `package com.example.quarkus.catalog;
import jakarta.ws.rs.*;
import jakarta.ws.rs.core.MediaType;
import org.eclipse.microprofile.openapi.annotations.*;
import org.eclipse.microprofile.openapi.annotations.responses.APIResponse;
import io.smallrye.common.annotation.Blocking;

@Path("/catalog")
@Produces(MediaType.APPLICATION_JSON)
@Consumes(MediaType.APPLICATION_JSON)
public class CatalogResource {

    @GET
    @Path("/items")
    @Produces(MediaType.APPLICATION_JSON)
    @APIResponse(responseCode = "200", description = "List of catalog items")
    @APIResponse(responseCode = "401", description = "Unauthorized")
    @Blocking
    public CatalogList listItems() { return null; }

    @GET
    @APIResponse(responseCode = "200", description = "Single item")
    @APIResponse(responseCode = "404", description = "Not found")
    @Blocking
    @Produces(MediaType.APPLICATION_JSON)
    @Path("/items/{sku}")
    public CatalogItem getItem() { return null; }
}
`
	got := collect(t, map[string]string{"CatalogResource.java": src})
	ids := endpointIDs(got)
	want := []string{
		"http:GET:/catalog/items",
		"http:GET:/catalog/items/{sku}",
	}
	sort.Strings(want)
	if strings.Join(ids, "|") != strings.Join(want, "|") {
		t.Fatalf("[#683] deep annotation stack: ids = %v\nwant %v", ids, want)
	}
}
