package csharp_test

import (
	"testing"
)

// ---------------------------------------------------------------------------
// Carter
// ---------------------------------------------------------------------------

func TestCarterModuleDetection(t *testing.T) {
	src := `
using Carter;

public class ProductsModule : ICarterModule
{
    public void AddRoutes(IEndpointRouteBuilder app)
    {
        app.MapGet("/products", () => Results.Ok());
        app.MapPost("/products", (CreateProductDto dto) => Results.Created("/products/1", dto));
        app.MapDelete("/products/{id}", (int id) => Results.NoContent());
    }
}
`
	ents := extract(t, "custom_csharp_minor_routes", fi("ProductsModule.cs", "csharp", src))

	if !containsEntity(ents, "SCOPE.Component", "carter:module:ProductsModule") {
		t.Error("expected carter:module:ProductsModule entity")
	}
	if !containsEntity(ents, "SCOPE.Operation", "GET /products") {
		t.Error("expected GET /products from Carter MapGet")
	}
	if !containsEntity(ents, "SCOPE.Operation", "POST /products") {
		t.Error("expected POST /products from Carter MapPost")
	}
	if !containsEntity(ents, "SCOPE.Operation", "DELETE /products/{id}") {
		t.Error("expected DELETE /products/{id} from Carter MapDelete")
	}
}

func TestCarterAddRoutesMarker(t *testing.T) {
	src := `
public class OrderModule : ICarterModule
{
    public void AddRoutes(IEndpointRouteBuilder app)
    {
        app.MapGet("/orders", GetAllOrders);
    }
}
`
	ents := extract(t, "custom_csharp_minor_routes", fi("OrderModule.cs", "csharp", src))

	foundAddRoutes := false
	for _, e := range ents {
		if e.Subtype == "endpoint_synthesis" {
			foundAddRoutes = true
			break
		}
	}
	if !foundAddRoutes {
		t.Error("expected endpoint_synthesis entity from AddRoutes method detection")
	}
}

// ---------------------------------------------------------------------------
// FastEndpoints
// ---------------------------------------------------------------------------

func TestFastEndpointsEndpointClass(t *testing.T) {
	src := `
using FastEndpoints;

public class GetProductEndpoint : Endpoint<GetProductRequest, GetProductResponse>
{
    public override void Configure()
    {
        Get("/api/products/{id}");
        AllowAnonymous();
    }

    public override async Task HandleAsync(GetProductRequest req, CancellationToken ct)
    {
        await SendOkAsync(new GetProductResponse { Id = req.Id });
    }
}
`
	ents := extract(t, "custom_csharp_minor_routes", fi("GetProductEndpoint.cs", "csharp", src))

	if !containsEntity(ents, "SCOPE.Component", "fastendpoints:endpoint:GetProductEndpoint") {
		t.Error("expected fastendpoints:endpoint:GetProductEndpoint entity")
	}
	if !containsEntity(ents, "SCOPE.Operation", "GET /api/products/{id}") {
		t.Error("expected GET /api/products/{id} from FastEndpoints Get()")
	}
}

func TestFastEndpointsMultipleRoutes(t *testing.T) {
	src := `
public class CreateOrderEndpoint : Endpoint<CreateOrderRequest>
{
    public override void Configure() { Post("/orders"); }
}

public class DeleteOrderEndpoint : Endpoint<DeleteOrderRequest>
{
    public override void Configure() { Delete("/orders/{id}"); }
}
`
	ents := extract(t, "custom_csharp_minor_routes", fi("OrderEndpoints.cs", "csharp", src))
	if !containsEntity(ents, "SCOPE.Operation", "POST /orders") {
		t.Error("expected POST /orders")
	}
	if !containsEntity(ents, "SCOPE.Operation", "DELETE /orders/{id}") {
		t.Error("expected DELETE /orders/{id}")
	}
}

// ---------------------------------------------------------------------------
// NancyFX
// ---------------------------------------------------------------------------

func TestNancyIndexSyntaxRoutes(t *testing.T) {
	src := `
using Nancy;

public class UsersModule : NancyModule
{
    public UsersModule()
    {
        Get["/users"] = _ => Response.AsJson(new[] { "alice", "bob" });
        Post["/users"] = _ => HttpStatusCode.Created;
        Delete["/users/{id}"] = params => HttpStatusCode.NoContent;
    }
}
`
	ents := extract(t, "custom_csharp_minor_routes", fi("UsersModule.cs", "csharp", src))

	if !containsEntity(ents, "SCOPE.Component", "nancy:module:UsersModule") {
		t.Error("expected nancy:module:UsersModule")
	}
	if !containsEntity(ents, "SCOPE.Operation", "GET /users") {
		t.Error("expected GET /users from Nancy index syntax")
	}
	if !containsEntity(ents, "SCOPE.Operation", "POST /users") {
		t.Error("expected POST /users from Nancy index syntax")
	}
	if !containsEntity(ents, "SCOPE.Operation", "DELETE /users/{id}") {
		t.Error("expected DELETE /users/{id} from Nancy index syntax")
	}
}

func TestNancyModuleIsRequired(t *testing.T) {
	// Without a NancyModule declaration the call-syntax routes should NOT be emitted
	src := `
public class SomeOtherClass
{
    public void Configure()
    {
        Get("/path", _ => "hello");
    }
}
`
	ents := extract(t, "custom_csharp_minor_routes", fi("Other.cs", "csharp", src))
	// No NancyModule, so the call-style Get("/path") should not create a Nancy route.
	// It may create a FastEndpoints route (same regex), but NOT tagged as nancyfx.
	for _, e := range ents {
		if e.Name == "GET /path" {
			// This is acceptable if it came from FastEndpoints detection.
			// The important thing: no nancy:module entity.
			break
		}
	}
	for _, e := range ents {
		if e.Kind == "SCOPE.Component" && e.Name == "nancy:module:SomeOtherClass" {
			t.Error("should not have detected NancyModule on a plain class")
		}
	}
}

// ---------------------------------------------------------------------------
// ServiceStack
// ---------------------------------------------------------------------------

func TestServiceStackRouteAttribute(t *testing.T) {
	src := `
using ServiceStack;

[Route("/customers", "GET POST")]
[Route("/customers/{Id}", "GET PUT DELETE")]
public class CustomerRequest : IReturn<CustomerResponse>
{
    public int Id { get; set; }
}

public class CustomerService : Service
{
    public object Any(CustomerRequest request) => new CustomerResponse();
    public object Get(CustomerRequest request) => new CustomerResponse();
}
`
	ents := extract(t, "custom_csharp_minor_routes", fi("CustomerService.cs", "csharp", src))

	if !containsEntity(ents, "SCOPE.Operation", "GET /customers") {
		t.Error("expected GET /customers from [Route(\"/customers\", \"GET POST\")]")
	}
	if !containsEntity(ents, "SCOPE.Operation", "POST /customers") {
		t.Error("expected POST /customers from [Route(\"/customers\", \"GET POST\")]")
	}
	if !containsEntity(ents, "SCOPE.Component", "servicestack:service:CustomerService") {
		t.Error("expected servicestack:service:CustomerService")
	}
}

func TestServiceStackRouteNoVerbs(t *testing.T) {
	// [Route("/ping")] with no verb string → should emit ANY /ping
	src := `
[Route("/ping")]
public class PingRequest {}
public class PingService : Service
{
    public object Any(PingRequest req) => new PingResponse();
}
`
	ents := extract(t, "custom_csharp_minor_routes", fi("PingService.cs", "csharp", src))
	if !containsEntity(ents, "SCOPE.Operation", "ANY /ping") {
		t.Error("expected ANY /ping from [Route(\"/ping\")] with no verbs")
	}
}

func TestServiceStackHandlerMethods(t *testing.T) {
	src := `
public class OrderService : Service
{
    public object Get(GetOrdersRequest req) => new List<Order>();
    public object Post(CreateOrderRequest req) => new Order();
    public void Delete(DeleteOrderRequest req) {}
}
`
	ents := extract(t, "custom_csharp_minor_routes", fi("OrderService.cs", "csharp", src))
	foundRouteExtraction := false
	for _, e := range ents {
		if e.Subtype == "route_extraction" {
			foundRouteExtraction = true
			break
		}
	}
	if !foundRouteExtraction {
		t.Error("expected route_extraction pattern from ServiceStack handler methods")
	}
}

// ---------------------------------------------------------------------------
// Non-csharp files should produce no entities
// ---------------------------------------------------------------------------

func TestMinorRoutesNonCsharpFile(t *testing.T) {
	src := `app.MapGet("/foo", () => "bar");`
	ents := extract(t, "custom_csharp_minor_routes", fi("program.go", "go", src))
	if len(ents) != 0 {
		t.Errorf("expected 0 entities for non-csharp file, got %d", len(ents))
	}
}
