// Package csharp — minor-framework routing extractor for C# source files.
//
// Covers four minor HTTP frameworks:
//
//   - Carter:         app.MapGet/MapPost/...; ICarterModule.AddRoutes
//   - FastEndpoints:  Endpoint<TReq> subclass; Get("/path") / Post("/path") / ...
//   - NancyFX:        Get["/path"] / Post["/path"]; class : NancyModule
//   - ServiceStack:   [Route("/path")] attribute; class : Service
//
// Each detected route is emitted as SCOPE.Operation with subtype "endpoint"
// so the Routing group cells (endpoint_synthesis, handler_attribution,
// route_extraction) light up for the four framework records.
package csharp

import (
	"context"
	"regexp"
	"strings"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/trace"

	"github.com/cajasmota/grafel/internal/extractor"
	"github.com/cajasmota/grafel/internal/types"
)

func init() {
	extractor.Register("custom_csharp_minor_routes", &minorRoutesExtractor{})
}

type minorRoutesExtractor struct{}

func (e *minorRoutesExtractor) Language() string { return "custom_csharp_minor_routes" }

// ---------------------------------------------------------------------------
// Regexes — Carter
// ---------------------------------------------------------------------------

var (
	// app.MapGet("/path", ...) — Carter / minimal-API style inside ICarterModule
	reCarterMapRoute = regexp.MustCompile(
		`(?m)app\.Map(Get|Post|Put|Delete|Patch)\s*\(\s*["']([^"']+)["']`,
	)
	// class MyModule : ICarterModule
	reCarterModule = regexp.MustCompile(
		`(?m)class\s+(\w+)\s*:\s*(?:\w+,\s*)*ICarterModule\b`,
	)
	// void AddRoutes(IEndpointRouteBuilder app) — module marker
	reCarterAddRoutes = regexp.MustCompile(
		`(?m)(?:public\s+)?void\s+AddRoutes\s*\(\s*IEndpointRouteBuilder`,
	)
)

// ---------------------------------------------------------------------------
// Regexes — FastEndpoints
// ---------------------------------------------------------------------------

var (
	// class MyEndpoint : Endpoint<TReq> or Endpoint<TReq, TRes>
	reFastEndpoint = regexp.MustCompile(
		`(?m)class\s+(\w+)\s*:\s*Endpoint\s*<`,
	)
	// Get("/path") / Post("/path") / ... inside Configure() or directly
	reFastRoute = regexp.MustCompile(
		`(?m)\b(Get|Post|Put|Delete|Patch|Head|Options)\s*\(\s*["']([^"']+)["']`,
	)
)

// ---------------------------------------------------------------------------
// Regexes — NancyFX
// ---------------------------------------------------------------------------

var (
	// Get["/path"] = ... (Nancy route registration with index syntax)
	reNancyIndexRoute = regexp.MustCompile(
		`(?m)(Get|Post|Put|Delete|Patch|Head|Options)\s*\[\s*["']([^"']+)["']\s*\]`,
	)
	// Get("/path", ...) or Get("/path") — method-call style (Nancy 2.x)
	reNancyCallRoute = regexp.MustCompile(
		`(?m)(Get|Post|Put|Delete|Patch|Head|Options)\s*\(\s*["']([^"']+)["']`,
	)
	// class MyModule : NancyModule
	reNancyModule = regexp.MustCompile(
		`(?m)class\s+(\w+)\s*:\s*NancyModule\b`,
	)
)

// ---------------------------------------------------------------------------
// Regexes — ServiceStack
// ---------------------------------------------------------------------------

var (
	// [Route("/path")] or [Route("/path", "GET POST")]
	reSSRoute = regexp.MustCompile(
		`\[Route\s*\(\s*["']([^"']+)["'](?:\s*,\s*["']([^"']*)["'])?\s*\)`,
	)
	// class MyService : Service
	reSSService = regexp.MustCompile(
		`(?m)class\s+(\w+)\s*:\s*(?:\w+,\s*)*Service\b`,
	)
	// Any(dto) / Get(dto) / Post(dto) — handler methods
	reSSHandler = regexp.MustCompile(
		`(?m)public\s+\S+\s+(Any|Get|Post|Put|Delete|Patch)\s*\(`,
	)
)

// ---------------------------------------------------------------------------
// Extract
// ---------------------------------------------------------------------------

func (e *minorRoutesExtractor) Extract(ctx context.Context, file extractor.FileInput) ([]types.EntityRecord, error) {
	tracer := otel.Tracer("grafel/custom/csharp")
	_, span := tracer.Start(ctx, "indexer.csharp_minor_routes_extractor.extract",
		trace.WithAttributes(
			attribute.String("language", file.Language),
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
	// Carter
	// -------------------------------------------------------------------------

	// Carter: ICarterModule subclass declarations
	for _, m := range reCarterModule.FindAllStringSubmatchIndex(src, -1) {
		name := src[m[2]:m[3]]
		ent := makeEntity("carter:module:"+name, "SCOPE.Component", "handler_attribution", file.Path, "csharp", lineOf(src, m[0]))
		setProps(&ent, "framework", "carter", "provenance", "INFERRED_FROM_CARTER_MODULE")
		add(ent)
	}

	// Carter: AddRoutes method presence
	for _, m := range reCarterAddRoutes.FindAllStringIndex(src, -1) {
		ent := makeEntity("carter:add_routes:"+file.Path+":"+itoa(lineOf(src, m[0])), "SCOPE.Pattern", "endpoint_synthesis", file.Path, "csharp", lineOf(src, m[0]))
		setProps(&ent, "framework", "carter", "provenance", "INFERRED_FROM_CARTER_ADD_ROUTES")
		add(ent)
	}

	// Carter: app.MapGet/Post/... routes
	for _, m := range reCarterMapRoute.FindAllStringSubmatchIndex(src, -1) {
		method := strings.ToUpper(src[m[2]:m[3]])
		path := src[m[4]:m[5]]
		name := method + " " + path
		ent := makeEntity(name, "SCOPE.Operation", "endpoint", file.Path, "csharp", lineOf(src, m[0]))
		setProps(&ent, "framework", "carter", "provenance", "INFERRED_FROM_CARTER_ROUTE",
			"http_method", method, "route_path", path)
		add(ent)
	}

	// -------------------------------------------------------------------------
	// FastEndpoints
	// -------------------------------------------------------------------------

	// FastEndpoints: Endpoint<TReq> subclass declarations
	for _, m := range reFastEndpoint.FindAllStringSubmatchIndex(src, -1) {
		name := src[m[2]:m[3]]
		ent := makeEntity("fastendpoints:endpoint:"+name, "SCOPE.Component", "handler_attribution", file.Path, "csharp", lineOf(src, m[0]))
		setProps(&ent, "framework", "fastendpoints", "provenance", "INFERRED_FROM_FASTENDPOINTS_ENDPOINT")
		add(ent)
	}

	// FastEndpoints: Get("/path") / Post("/path") / ... route registrations
	for _, m := range reFastRoute.FindAllStringSubmatchIndex(src, -1) {
		method := strings.ToUpper(src[m[2]:m[3]])
		path := src[m[4]:m[5]]
		name := method + " " + path
		ent := makeEntity(name, "SCOPE.Operation", "endpoint", file.Path, "csharp", lineOf(src, m[0]))
		setProps(&ent, "framework", "fastendpoints", "provenance", "INFERRED_FROM_FASTENDPOINTS_ROUTE",
			"http_method", method, "route_path", path)
		add(ent)
	}

	// -------------------------------------------------------------------------
	// NancyFX
	// -------------------------------------------------------------------------

	// Nancy: NancyModule subclass declarations
	for _, m := range reNancyModule.FindAllStringSubmatchIndex(src, -1) {
		name := src[m[2]:m[3]]
		ent := makeEntity("nancy:module:"+name, "SCOPE.Component", "handler_attribution", file.Path, "csharp", lineOf(src, m[0]))
		setProps(&ent, "framework", "nancyfx", "provenance", "INFERRED_FROM_NANCY_MODULE")
		add(ent)
	}

	// Nancy: Get["/path"] = ... index-syntax routes (Nancy 1.x)
	for _, m := range reNancyIndexRoute.FindAllStringSubmatchIndex(src, -1) {
		method := strings.ToUpper(src[m[2]:m[3]])
		path := src[m[4]:m[5]]
		name := method + " " + path
		ent := makeEntity(name, "SCOPE.Operation", "endpoint", file.Path, "csharp", lineOf(src, m[0]))
		setProps(&ent, "framework", "nancyfx", "provenance", "INFERRED_FROM_NANCY_ROUTE",
			"http_method", method, "route_path", path)
		add(ent)
	}

	// Nancy: Get("/path", ...) call-syntax routes (Nancy 2.x) — only emit
	// when inside a NancyModule (detected above) to avoid collisions with
	// FastEndpoints which uses the same call pattern.
	if reNancyModule.MatchString(src) {
		for _, m := range reNancyCallRoute.FindAllStringSubmatchIndex(src, -1) {
			method := strings.ToUpper(src[m[2]:m[3]])
			path := src[m[4]:m[5]]
			name := method + " " + path
			ent := makeEntity(name, "SCOPE.Operation", "endpoint", file.Path, "csharp", lineOf(src, m[0]))
			setProps(&ent, "framework", "nancyfx", "provenance", "INFERRED_FROM_NANCY_ROUTE_CALL",
				"http_method", method, "route_path", path)
			add(ent)
		}
	}

	// -------------------------------------------------------------------------
	// ServiceStack
	// -------------------------------------------------------------------------

	// ServiceStack: [Route("/path")] attribute
	for _, m := range reSSRoute.FindAllStringSubmatchIndex(src, -1) {
		path := src[m[2]:m[3]]
		verbStr := ""
		if m[4] >= 0 {
			verbStr = src[m[4]:m[5]]
		}
		verbs := parseSSVerbs(verbStr)
		for _, verb := range verbs {
			name := verb + " " + path
			ent := makeEntity(name, "SCOPE.Operation", "endpoint", file.Path, "csharp", lineOf(src, m[0]))
			setProps(&ent, "framework", "servicestack", "provenance", "INFERRED_FROM_SERVICESTACK_ROUTE",
				"http_method", verb, "route_path", path)
			add(ent)
		}
	}

	// ServiceStack: class MyService : Service declarations
	for _, m := range reSSService.FindAllStringSubmatchIndex(src, -1) {
		name := src[m[2]:m[3]]
		ent := makeEntity("servicestack:service:"+name, "SCOPE.Component", "handler_attribution", file.Path, "csharp", lineOf(src, m[0]))
		setProps(&ent, "framework", "servicestack", "provenance", "INFERRED_FROM_SERVICESTACK_SERVICE")
		add(ent)
	}

	// ServiceStack: Any/Get/Post/... handler methods
	for _, m := range reSSHandler.FindAllStringSubmatchIndex(src, -1) {
		method := strings.ToUpper(src[m[2]:m[3]])
		ent := makeEntity("servicestack:handler:"+method+":"+file.Path+":"+itoa(lineOf(src, m[0])),
			"SCOPE.Pattern", "route_extraction", file.Path, "csharp", lineOf(src, m[0]))
		setProps(&ent, "framework", "servicestack", "provenance", "INFERRED_FROM_SERVICESTACK_HANDLER",
			"http_method", method)
		add(ent)
	}

	span.SetAttributes(attribute.Int("entity_count", len(entities)))
	return entities, nil
}

// parseSSVerbs splits a ServiceStack verb string like "GET POST" into
// individual HTTP method tokens. When the verb string is empty or "ANY",
// it returns ["ANY"].
func parseSSVerbs(verbStr string) []string {
	verbStr = strings.TrimSpace(verbStr)
	if verbStr == "" || strings.EqualFold(verbStr, "ANY") {
		return []string{"ANY"}
	}
	parts := strings.Fields(verbStr)
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		up := strings.ToUpper(p)
		if up != "" {
			out = append(out, up)
		}
	}
	if len(out) == 0 {
		return []string{"ANY"}
	}
	return out
}
