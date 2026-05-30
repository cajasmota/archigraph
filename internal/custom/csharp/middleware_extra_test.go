package csharp_test

// ---------------------------------------------------------------------------
// Middleware Extra — Carter / FastEndpoints / NancyFX / ServiceStack / ASP.NET MVC
// ---------------------------------------------------------------------------

import "testing"

func TestMiddlewareExtraUseMiddleware(t *testing.T) {
	src := `
app.UseMiddleware<AuthenticationMiddleware>();
app.UseMiddleware<RateLimitingMiddleware>();
`
	ents := extract(t, "custom_csharp_middleware_extra", fi("Program.cs", "csharp", src))
	if !containsEntity(ents, "SCOPE.Component", "middleware:AuthenticationMiddleware") {
		t.Error("expected middleware:AuthenticationMiddleware from UseMiddleware<T>")
	}
	if !containsEntity(ents, "SCOPE.Component", "middleware:RateLimitingMiddleware") {
		t.Error("expected middleware:RateLimitingMiddleware from UseMiddleware<T>")
	}
}

func TestMiddlewareExtraMiddlewareClass(t *testing.T) {
	src := `
public class RequestLoggingMiddleware : IMiddleware
{
    public async Task InvokeAsync(HttpContext context, RequestDelegate next)
    {
        await next(context);
    }
}
`
	ents := extract(t, "custom_csharp_middleware_extra", fi("Middleware.cs", "csharp", src))
	foundClass := false
	foundInvoke := false
	for _, e := range ents {
		if e.Subtype == "middleware_coverage" && e.Name == "middleware:class:RequestLoggingMiddleware" {
			foundClass = true
		}
		if e.Subtype == "middleware_coverage" && e.Kind == "SCOPE.Component" {
			foundInvoke = true
		}
	}
	if !foundClass {
		t.Error("expected middleware:class:RequestLoggingMiddleware")
	}
	if !foundInvoke {
		t.Error("expected middleware_coverage from InvokeAsync signature")
	}
}

func TestMiddlewareExtraMVCFilter(t *testing.T) {
	src := `
[ServiceFilter(typeof(AuthorizationFilter))]
public class OrdersController : ControllerBase
{
    [TypeFilter(typeof(RateLimitFilter))]
    public IActionResult GetOrders() => Ok();
}
`
	ents := extract(t, "custom_csharp_middleware_extra", fi("OrdersController.cs", "csharp", src))
	foundFilter := false
	for _, e := range ents {
		if e.Subtype == "middleware_coverage" {
			foundFilter = true
			break
		}
	}
	if !foundFilter {
		t.Error("expected middleware_coverage from [ServiceFilter]/[TypeFilter]")
	}
}

func TestMiddlewareExtraCartermiddleware(t *testing.T) {
	src := `
builder.Services.AddCarter();
var app = builder.Build();
app.MapCarter();
`
	ents := extract(t, "custom_csharp_middleware_extra", fi("Program.cs", "csharp", src))
	foundMapCarter := false
	foundAddCarter := false
	for _, e := range ents {
		if e.Subtype == "middleware_coverage" && e.Kind == "SCOPE.Pattern" {
			if e.Name != "" {
				foundMapCarter = true
				foundAddCarter = true
			}
		}
	}
	if !foundMapCarter || !foundAddCarter {
		t.Error("expected middleware_coverage from MapCarter/AddCarter")
	}
}

func TestMiddlewareExtraFastEndpointsmiddleware(t *testing.T) {
	src := `
builder.Services.AddFastEndpoints();
var app = builder.Build();
app.UseFastEndpoints();
`
	ents := extract(t, "custom_csharp_middleware_extra", fi("Program.cs", "csharp", src))
	foundUse := false
	foundAdd := false
	for _, e := range ents {
		if e.Subtype == "middleware_coverage" {
			if e.Kind == "SCOPE.Pattern" {
				foundUse = true
				foundAdd = true
			}
		}
	}
	if !foundUse || !foundAdd {
		t.Error("expected middleware_coverage from UseFastEndpoints/AddFastEndpoints")
	}
}

func TestMiddlewareExtraFastEndpointsGlobalProcessor(t *testing.T) {
	src := `
public class MyGlobalPreProcessor : IGlobalPreProcessor
{
    public Task PreProcessAsync(IPreProcessorContext ctx, CancellationToken ct) => Task.CompletedTask;
}
`
	ents := extract(t, "custom_csharp_middleware_extra", fi("Processors.cs", "csharp", src))
	if !containsEntity(ents, "SCOPE.Component", "fastendpoints:processor:MyGlobalPreProcessor") {
		t.Error("expected middleware:processor:MyGlobalPreProcessor from IGlobalPreProcessor")
	}
}

func TestMiddlewareExtraNancyHooks(t *testing.T) {
	src := `
public class ProductsModule : NancyModule
{
    public ProductsModule()
    {
        this.Before += ctx => {
            if (!ctx.CurrentUser.IsAuthenticated())
                return HttpStatusCode.Unauthorized;
            return null;
        };

        this.After += ctx => {
            ctx.Response.Headers.Add("X-Powered-By", "Nancy");
        };
    }
}
`
	ents := extract(t, "custom_csharp_middleware_extra", fi("ProductsModule.cs", "csharp", src))
	foundHook := false
	for _, e := range ents {
		if e.Subtype == "middleware_coverage" {
			foundHook = true
			break
		}
	}
	if !foundHook {
		t.Error("expected middleware_coverage from NancyModule Before/After hooks")
	}
}

func TestMiddlewareExtraNancyBootstrapper(t *testing.T) {
	src := `
public class CustomBootstrapper : DefaultNancyBootstrapper
{
    protected override void RequestStartup(TinyIoCContainer container, IPipelines pipelines, NancyContext context)
    {
        pipelines.BeforeRequest += ctx => { return null; };
    }
}
`
	ents := extract(t, "custom_csharp_middleware_extra", fi("Bootstrapper.cs", "csharp", src))
	foundBootstrapper := false
	foundStartup := false
	for _, e := range ents {
		if e.Subtype == "middleware_coverage" {
			if e.Name == "nancy:bootstrapper:CustomBootstrapper" {
				foundBootstrapper = true
			}
			if e.Name == "nancy:startup:RequestStartup" {
				foundStartup = true
			}
		}
	}
	if !foundBootstrapper {
		t.Error("expected nancy:bootstrapper:CustomBootstrapper")
	}
	if !foundStartup {
		t.Error("expected nancy:startup:RequestStartup")
	}
}

func TestMiddlewareExtraServiceStackPlugins(t *testing.T) {
	src := `
public class AppHost : AppHostBase
{
    public override void Configure(Container container)
    {
        Plugins.Add<AuthFeature>();
        Plugins.Add<SwaggerFeature>();
        GlobalRequestFilters.Add((req, res, dto) => { });
    }
}
`
	ents := extract(t, "custom_csharp_middleware_extra", fi("AppHost.cs", "csharp", src))
	foundPlugin := false
	foundFilter := false
	foundHost := false
	for _, e := range ents {
		if e.Subtype == "middleware_coverage" {
			if e.Name == "servicestack:plugin:AuthFeature" {
				foundPlugin = true
			}
			if e.Kind == "SCOPE.Pattern" {
				foundFilter = true
			}
			if e.Name == "servicestack:apphost:AppHost" {
				foundHost = true
			}
		}
	}
	if !foundPlugin {
		t.Error("expected servicestack:plugin:AuthFeature from Plugins.Add<T>")
	}
	if !foundFilter {
		t.Error("expected middleware_coverage from GlobalRequestFilters.Add")
	}
	if !foundHost {
		t.Error("expected servicestack:apphost:AppHost from AppHostBase subclass")
	}
}

func TestMiddlewareExtraNoMatch(t *testing.T) {
	src := `namespace MyApp { class Helper { } }`
	ents := extract(t, "custom_csharp_middleware_extra", fi("Helper.cs", "csharp", src))
	if len(ents) != 0 {
		t.Errorf("expected no entities, got %d", len(ents))
	}
}
