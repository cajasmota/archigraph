// Package csharp — Middleware extractor for minor C# web frameworks.
//
// Covers the Middleware/middleware_coverage cells missing for:
//   - lang.csharp.framework.aspnet-mvc  (Middleware/middleware_coverage)
//   - lang.csharp.framework.carter      (Middleware/middleware_coverage)
//   - lang.csharp.framework.fastendpoints (Middleware/middleware_coverage)
//   - lang.csharp.framework.nancyfx     (Middleware/middleware_coverage)
//   - lang.csharp.framework.servicestack (Middleware/middleware_coverage)
//
// Detection surface:
//
//	app.UseMiddleware<T>() / app.Use(next => ...) / app.UseWhen(...)
//	  → SCOPE.Component/middleware_coverage
//
//	Carter: app.MapCarter() / ICarterModule pipeline hooks
//	  → SCOPE.Pattern/middleware_coverage
//
//	FastEndpoints: AddFastEndpoints() / UseFastEndpoints()
//	  → SCOPE.Pattern/middleware_coverage
//
//	NancyFX: NancyModule RequestStartup / BeforeRequest / AfterRequest hooks
//	  → SCOPE.Component/middleware_coverage
//
//	ServiceStack: Plugins.Add<T>() / RequestFilters / ResponseFilters
//	  → SCOPE.Pattern/middleware_coverage
//
//	ASP.NET MVC: app.Use*()/[Filters] attribute middleware detection
//	  → SCOPE.Component/middleware_coverage
//
// Registration key: "custom_csharp_middleware_extra"
// Issue #3261.
package csharp

import (
	"context"
	"regexp"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/trace"

	"github.com/cajasmota/archigraph/internal/extractor"
	"github.com/cajasmota/archigraph/internal/types"
)

func init() {
	extractor.Register("custom_csharp_middleware_extra", &middlewareExtraExtractor{})
}

type middlewareExtraExtractor struct{}

func (e *middlewareExtraExtractor) Language() string { return "custom_csharp_middleware_extra" }

// ---------------------------------------------------------------------------
// Regex catalog
// ---------------------------------------------------------------------------

var (
	// Generic ASP.NET middleware -----------------------------------------------

	// app.UseMiddleware<T>() — typed middleware registration
	reMWUseMiddleware = regexp.MustCompile(
		`app\.UseMiddleware\s*<\s*(\w+)\s*>`,
	)

	// app.Use(next => ...) — inline middleware lambda
	reMWUseLambda = regexp.MustCompile(
		`app\.Use\s*\(\s*(?:async\s+)?\(`,
	)

	// app.UseWhen(predicate, ...) — conditional middleware
	reMWUseWhen = regexp.MustCompile(
		`app\.UseWhen\s*\(`,
	)

	// class XyzMiddleware : IMiddleware / XyzMiddleware + InvokeAsync
	reMWMiddlewareClass = regexp.MustCompile(
		`(?m)class\s+(\w+Middleware)\s*(?::\s*IMiddleware\b)?`,
	)

	// InvokeAsync(HttpContext context) — middleware invoke signature
	reMWInvokeAsync = regexp.MustCompile(
		`(?m)public\s+(?:async\s+)?Task\s+InvokeAsync\s*\(\s*HttpContext`,
	)

	// [ServiceFilter(...)], [TypeFilter(...)], [ActionFilter] — MVC filter attributes
	// Use a simpler pattern: match opening bracket+keyword+opening paren to avoid
	// nested-paren issues with typeof(T) arguments.
	reMWFilterAttr = regexp.MustCompile(
		`\[(?:ServiceFilter|TypeFilter|ActionFilter|ResultFilter|ExceptionFilter)\s*[\[(]`,
	)

	// IActionFilter / IResultFilter / IExceptionFilter — MVC filter interfaces
	reMWFilterInterface = regexp.MustCompile(
		`(?m):\s*(?:\w+,\s*)*(?:IActionFilter|IResultFilter|IExceptionFilter|` +
			`IAsyncActionFilter|IAsyncResultFilter|IAuthorizationFilter)\b`,
	)

	// Carter / FastEndpoints -------------------------------------------------

	// app.MapCarter() — Carter pipeline entry
	reMWMapCarter = regexp.MustCompile(
		`app\.MapCarter\s*\(`,
	)

	// builder.Services.AddCarter() / services.AddCarter()
	reMWAddCarter = regexp.MustCompile(
		`\.AddCarter\s*\(`,
	)

	// app.UseFastEndpoints() — FastEndpoints pipeline
	reMWUseFastEndpoints = regexp.MustCompile(
		`app\.UseFastEndpoints\s*\(`,
	)

	// builder.Services.AddFastEndpoints()
	reMWAddFastEndpoints = regexp.MustCompile(
		`\.AddFastEndpoints\s*\(`,
	)

	// GlobalPreProcessor / GlobalPostProcessor — FastEndpoints global hooks
	reMWFastGlobalProcessor = regexp.MustCompile(
		`(?m)class\s+(\w+)\s*:\s*(?:IGlobalPreProcessor|IGlobalPostProcessor|` +
			`IPreProcessor|IPostProcessor)\b`,
	)

	// NancyFX ----------------------------------------------------------------

	// RequestStartup / ApplicationStartup — Nancy pipeline hooks
	reMWNancyStartup = regexp.MustCompile(
		`(?m)(?:protected|public)\s+override\s+void\s+` +
			`(RequestStartup|ApplicationStartup|ConfigureRequestContainer)\s*\(`,
	)

	// this.Before += / this.After += — NancyModule filter hooks
	reMWNancyHook = regexp.MustCompile(
		`(?m)this\.(Before|After)\s*\+?=`,
	)

	// class XxxBootstrapper : DefaultNancyBootstrapper
	reMWNancyBootstrapper = regexp.MustCompile(
		`(?m)class\s+(\w+)\s*:\s*(?:DefaultNancyBootstrapper|NancyBootstrapper|INancyBootstrapper)\b`,
	)

	// ServiceStack -----------------------------------------------------------

	// Plugins.Add<T>() — ServiceStack plugin/middleware registration
	reMWSServiceStackPlugin = regexp.MustCompile(
		`Plugins\.Add\s*<\s*(\w+)\s*>|Plugins\.Add\s*\(\s*new\s+(\w+)`,
	)

	// GlobalRequestFilters.Add / GlobalResponseFilters.Add
	reMWServiceStackFilter = regexp.MustCompile(
		`Global(?:Request|Response)Filters\.Add\s*\(`,
	)

	// PreRequestFilters.Add / PostResponseFilters.Add
	reMWServiceStackPrePost = regexp.MustCompile(
		`(?:Pre|Post)(?:Request|Response)Filters\.Add\s*\(`,
	)

	// class XxxAppHost : AppHostBase / AppSelfHostBase
	reMWServiceStackAppHost = regexp.MustCompile(
		`(?m)class\s+(\w+)\s*:\s*(?:AppHostBase|AppHostHttpListenerBase|AppSelfHostBase|` +
			`ServiceStackHost)\b`,
	)
)

// ---------------------------------------------------------------------------
// Extract
// ---------------------------------------------------------------------------

func (e *middlewareExtraExtractor) Extract(ctx context.Context, file extractor.FileInput) ([]types.EntityRecord, error) {
	tracer := otel.Tracer("archigraph/custom/csharp")
	_, span := tracer.Start(ctx, "indexer.middleware_extra_extractor.extract",
		trace.WithAttributes(
			attribute.String("language", file.Language),
			attribute.String("framework", "middleware"),
			attribute.String("file_path", file.Path),
		),
	)
	defer span.End()

	if len(file.Content) == 0 {
		return nil, nil
	}
	if file.Language != "csharp" {
		return nil, nil
	}

	src := string(file.Content)
	var entities []types.EntityRecord
	seen := make(map[string]bool)

	add := func(ent types.EntityRecord) {
		key := ent.Kind + ":" + ent.Subtype + ":" + ent.Name
		if seen[key] {
			return
		}
		seen[key] = true
		entities = append(entities, ent)
	}

	// -------------------------------------------------------------------------
	// Generic ASP.NET middleware
	// -------------------------------------------------------------------------

	for _, m := range reMWUseMiddleware.FindAllStringSubmatchIndex(src, -1) {
		middlewareType := src[m[2]:m[3]]
		ent := makeEntity("middleware:"+middlewareType, "SCOPE.Component", "middleware_coverage",
			file.Path, "csharp", lineOf(src, m[0]))
		setProps(&ent, "framework", "aspnet", "provenance", "INFERRED_FROM_USE_MIDDLEWARE",
			"middleware_type", middlewareType)
		add(ent)
	}

	for _, m := range reMWUseLambda.FindAllStringIndex(src, -1) {
		ent := makeEntity("middleware:lambda:"+file.Path+":"+itoa(lineOf(src, m[0])),
			"SCOPE.Component", "middleware_coverage", file.Path, "csharp", lineOf(src, m[0]))
		setProps(&ent, "framework", "aspnet", "provenance", "INFERRED_FROM_USE_LAMBDA")
		add(ent)
	}

	for _, m := range reMWUseWhen.FindAllStringIndex(src, -1) {
		ent := makeEntity("middleware:when:"+file.Path+":"+itoa(lineOf(src, m[0])),
			"SCOPE.Component", "middleware_coverage", file.Path, "csharp", lineOf(src, m[0]))
		setProps(&ent, "framework", "aspnet", "provenance", "INFERRED_FROM_USE_WHEN")
		add(ent)
	}

	for _, m := range reMWMiddlewareClass.FindAllStringSubmatchIndex(src, -1) {
		name := src[m[2]:m[3]]
		ent := makeEntity("middleware:class:"+name, "SCOPE.Component", "middleware_coverage",
			file.Path, "csharp", lineOf(src, m[0]))
		setProps(&ent, "framework", "aspnet", "provenance", "INFERRED_FROM_MIDDLEWARE_CLASS",
			"class_name", name)
		add(ent)
	}

	if reMWInvokeAsync.MatchString(src) {
		ent := makeEntity("middleware:invoke_async:"+file.Path,
			"SCOPE.Component", "middleware_coverage", file.Path, "csharp",
			func() int {
				m := reMWInvokeAsync.FindStringIndex(src)
				if m != nil {
					return lineOf(src, m[0])
				}
				return 1
			}())
		setProps(&ent, "framework", "aspnet", "provenance", "INFERRED_FROM_INVOKE_ASYNC")
		add(ent)
	}

	for _, m := range reMWFilterAttr.FindAllStringIndex(src, -1) {
		ent := makeEntity("middleware:filter_attr:"+file.Path+":"+itoa(lineOf(src, m[0])),
			"SCOPE.Component", "middleware_coverage", file.Path, "csharp", lineOf(src, m[0]))
		setProps(&ent, "framework", "aspnet-mvc", "provenance", "INFERRED_FROM_FILTER_ATTR")
		add(ent)
	}

	for _, m := range reMWFilterInterface.FindAllStringIndex(src, -1) {
		ent := makeEntity("middleware:filter_iface:"+file.Path+":"+itoa(lineOf(src, m[0])),
			"SCOPE.Component", "middleware_coverage", file.Path, "csharp", lineOf(src, m[0]))
		setProps(&ent, "framework", "aspnet-mvc", "provenance", "INFERRED_FROM_FILTER_INTERFACE")
		add(ent)
	}

	// -------------------------------------------------------------------------
	// Carter
	// -------------------------------------------------------------------------

	for _, m := range reMWMapCarter.FindAllStringIndex(src, -1) {
		ent := makeEntity("carter:map_carter:"+file.Path+":"+itoa(lineOf(src, m[0])),
			"SCOPE.Pattern", "middleware_coverage", file.Path, "csharp", lineOf(src, m[0]))
		setProps(&ent, "framework", "carter", "provenance", "INFERRED_FROM_MAP_CARTER")
		add(ent)
	}

	for _, m := range reMWAddCarter.FindAllStringIndex(src, -1) {
		ent := makeEntity("carter:add_carter:"+file.Path+":"+itoa(lineOf(src, m[0])),
			"SCOPE.Pattern", "middleware_coverage", file.Path, "csharp", lineOf(src, m[0]))
		setProps(&ent, "framework", "carter", "provenance", "INFERRED_FROM_ADD_CARTER")
		add(ent)
	}

	// -------------------------------------------------------------------------
	// FastEndpoints
	// -------------------------------------------------------------------------

	for _, m := range reMWUseFastEndpoints.FindAllStringIndex(src, -1) {
		ent := makeEntity("fastendpoints:use:"+file.Path+":"+itoa(lineOf(src, m[0])),
			"SCOPE.Pattern", "middleware_coverage", file.Path, "csharp", lineOf(src, m[0]))
		setProps(&ent, "framework", "fastendpoints", "provenance", "INFERRED_FROM_USE_FAST_ENDPOINTS")
		add(ent)
	}

	for _, m := range reMWAddFastEndpoints.FindAllStringIndex(src, -1) {
		ent := makeEntity("fastendpoints:add:"+file.Path+":"+itoa(lineOf(src, m[0])),
			"SCOPE.Pattern", "middleware_coverage", file.Path, "csharp", lineOf(src, m[0]))
		setProps(&ent, "framework", "fastendpoints", "provenance", "INFERRED_FROM_ADD_FAST_ENDPOINTS")
		add(ent)
	}

	for _, m := range reMWFastGlobalProcessor.FindAllStringSubmatchIndex(src, -1) {
		name := src[m[2]:m[3]]
		ent := makeEntity("fastendpoints:processor:"+name, "SCOPE.Component", "middleware_coverage",
			file.Path, "csharp", lineOf(src, m[0]))
		setProps(&ent, "framework", "fastendpoints", "provenance", "INFERRED_FROM_FAST_PROCESSOR",
			"class_name", name)
		add(ent)
	}

	// -------------------------------------------------------------------------
	// NancyFX
	// -------------------------------------------------------------------------

	for _, m := range reMWNancyStartup.FindAllStringSubmatchIndex(src, -1) {
		hookName := src[m[2]:m[3]]
		ent := makeEntity("nancy:startup:"+hookName, "SCOPE.Component", "middleware_coverage",
			file.Path, "csharp", lineOf(src, m[0]))
		setProps(&ent, "framework", "nancyfx", "provenance", "INFERRED_FROM_NANCY_STARTUP",
			"hook_name", hookName)
		add(ent)
	}

	for _, m := range reMWNancyHook.FindAllStringSubmatchIndex(src, -1) {
		hookType := src[m[2]:m[3]]
		ent := makeEntity("nancy:hook:"+hookType+":"+file.Path+":"+itoa(lineOf(src, m[0])),
			"SCOPE.Component", "middleware_coverage", file.Path, "csharp", lineOf(src, m[0]))
		setProps(&ent, "framework", "nancyfx", "provenance", "INFERRED_FROM_NANCY_HOOK",
			"hook_type", hookType)
		add(ent)
	}

	for _, m := range reMWNancyBootstrapper.FindAllStringSubmatchIndex(src, -1) {
		name := src[m[2]:m[3]]
		ent := makeEntity("nancy:bootstrapper:"+name, "SCOPE.Component", "middleware_coverage",
			file.Path, "csharp", lineOf(src, m[0]))
		setProps(&ent, "framework", "nancyfx", "provenance", "INFERRED_FROM_NANCY_BOOTSTRAPPER",
			"class_name", name)
		add(ent)
	}

	// -------------------------------------------------------------------------
	// ServiceStack
	// -------------------------------------------------------------------------

	for _, m := range reMWSServiceStackPlugin.FindAllStringSubmatchIndex(src, -1) {
		pluginType := src[m[2]:m[3]]
		if pluginType == "" {
			pluginType = src[m[4]:m[5]]
		}
		ent := makeEntity("servicestack:plugin:"+pluginType, "SCOPE.Pattern", "middleware_coverage",
			file.Path, "csharp", lineOf(src, m[0]))
		setProps(&ent, "framework", "servicestack", "provenance", "INFERRED_FROM_SS_PLUGIN",
			"plugin_type", pluginType)
		add(ent)
	}

	for _, m := range reMWServiceStackFilter.FindAllStringIndex(src, -1) {
		ent := makeEntity("servicestack:filter:"+file.Path+":"+itoa(lineOf(src, m[0])),
			"SCOPE.Pattern", "middleware_coverage", file.Path, "csharp", lineOf(src, m[0]))
		setProps(&ent, "framework", "servicestack", "provenance", "INFERRED_FROM_SS_FILTER")
		add(ent)
	}

	for _, m := range reMWServiceStackPrePost.FindAllStringIndex(src, -1) {
		ent := makeEntity("servicestack:pre_post_filter:"+file.Path+":"+itoa(lineOf(src, m[0])),
			"SCOPE.Pattern", "middleware_coverage", file.Path, "csharp", lineOf(src, m[0]))
		setProps(&ent, "framework", "servicestack", "provenance", "INFERRED_FROM_SS_PRE_POST_FILTER")
		add(ent)
	}

	for _, m := range reMWServiceStackAppHost.FindAllStringSubmatchIndex(src, -1) {
		name := src[m[2]:m[3]]
		ent := makeEntity("servicestack:apphost:"+name, "SCOPE.Component", "middleware_coverage",
			file.Path, "csharp", lineOf(src, m[0]))
		setProps(&ent, "framework", "servicestack", "provenance", "INFERRED_FROM_SS_APPHOST",
			"class_name", name)
		add(ent)
	}

	span.SetAttributes(attribute.Int("entity_count", len(entities)))
	return entities, nil
}
