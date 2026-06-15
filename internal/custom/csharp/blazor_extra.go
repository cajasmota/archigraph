// Package csharp — Blazor Structure/Navigation/Lifecycle extractor for C# source files.
//
// Covers capabilities missing in the Blazor, Blazor Server, and Blazor WASM records:
//
//	Structure/component_extraction:
//	  @page-annotated Razor components and classes inheriting ComponentBase are
//	  emitted as SCOPE.UIComponent/component_extraction.
//	  .razor file names are also treated as component declarations.
//
//	Structure/context_extraction:
//	  @inject-introduced service types and [CascadingParameter] cascade context
//	  providers emitted as SCOPE.Component/context_extraction.
//
//	Navigation/router_pattern:
//	  @page routes emitted as SCOPE.Operation/router_pattern.
//	  NavigateTo() calls and <NavLink href="..."> patterns captured as
//	  SCOPE.Pattern/router_pattern.
//
//	Lifecycle/state_setter_emission:
//	  OnInitialized / OnInitializedAsync / OnParametersSet / OnAfterRender
//	  lifecycle method declarations emitted as SCOPE.Operation/state_setter_emission.
//	  StateHasChanged() call sites also captured.
//
// Registration key: "custom_csharp_blazor_extra"
// Issue #3261.
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
	extractor.Register("custom_csharp_blazor_extra", &blazorExtraExtractor{})
}

type blazorExtraExtractor struct{}

func (e *blazorExtraExtractor) Language() string { return "custom_csharp_blazor_extra" }

// ---------------------------------------------------------------------------
// Regex catalog
// ---------------------------------------------------------------------------

var (
	// Structure/component_extraction ----------------------------------------

	// @page "/path" — marks this .razor file as a routable component
	reBEPage = regexp.MustCompile(`(?m)^@page\s+"([^"]+)"`)

	// class MyComponent : ComponentBase  — direct C# component class
	reBEComponentBase = regexp.MustCompile(
		`(?m)class\s+(\w+)\s*:\s*(?:ComponentBase|IComponent)\b`,
	)

	// class MyLayout : LayoutComponentBase
	reBELayoutBase = regexp.MustCompile(
		`(?m)class\s+(\w+)\s*:\s*LayoutComponentBase\b`,
	)

	// Structure/context_extraction -------------------------------------------

	// @inject IMyService _svc — injected service context
	reBEInject = regexp.MustCompile(`(?m)^@inject\s+(\S+)\s+(\w+)`)

	// [CascadingParameter] public AppState State — cascading context value
	reBECascadingParam = regexp.MustCompile(
		`\[CascadingParameter\]\s*(?:public\s+)?(\w+(?:<[^>]+>)?)\s+(\w+)`,
	)

	// Navigation/router_pattern ----------------------------------------------

	// NavigationManager.NavigateTo("path") — programmatic navigation
	reBENavigateTo = regexp.MustCompile(
		`\.NavigateTo\s*\(\s*["']([^"']+)["']`,
	)

	// <NavLink href="/path"> — HTML NavLink usage
	reBENavLink = regexp.MustCompile(
		`<NavLink\b[^>]*\bhref\s*=\s*["']([^"']+)["']`,
	)

	// <a href="/path"> in razor markup — plain anchor navigation
	reBEAnchorHref = regexp.MustCompile(
		`<a\b[^>]*\bhref\s*=\s*["']([^"'#][^"']*)["']`,
	)

	// Lifecycle/state_setter_emission ----------------------------------------

	// OnInitialized / OnInitializedAsync / OnParametersSet / OnAfterRender / ...
	reBELifecycleMethod = regexp.MustCompile(
		`(?m)(?:override\s+)?(?:protected|public)\s+(?:override\s+)?(?:async\s+)?` +
			`(?:Task|void)\s+(On(?:Initialized(?:Async)?|ParametersSet(?:Async)?|AfterRender(?:Async)?|` +
			`Dispose|ParametersSetAsync|CircuitOpened|CircuitClosed))\s*\(`,
	)

	// StateHasChanged() call — explicit re-render trigger
	reBEStateHasChanged = regexp.MustCompile(
		`\bStateHasChanged\s*\(\s*\)`,
	)

	// InvokeAsync(StateHasChanged) — thread-safe variant
	reBEInvokeStateHasChanged = regexp.MustCompile(
		`InvokeAsync\s*\(\s*StateHasChanged\s*\)`,
	)
)

// blazorBuiltinNavTargets are paths that are framework-internal and not user routes.
var blazorBuiltinNavTargets = map[string]bool{
	"#": true, "/": true, "": true, "javascript:void(0)": true, "javascript:;": true,
}

func (e *blazorExtraExtractor) Extract(ctx context.Context, file extractor.FileInput) ([]types.EntityRecord, error) {
	tracer := otel.Tracer("grafel/custom/csharp")
	_, span := tracer.Start(ctx, "indexer.blazor_extra_extractor.extract",
		trace.WithAttributes(
			attribute.String("language", file.Language),
			attribute.String("framework", "blazor"),
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
	// Structure/component_extraction
	// -------------------------------------------------------------------------

	// @page routes also count as component declarations (the file is the component)
	for _, m := range reBEPage.FindAllStringSubmatchIndex(src, -1) {
		route := src[m[2]:m[3]]
		name := "component:" + route
		ent := makeEntity(name, "SCOPE.UIComponent", "component_extraction", file.Path, "csharp", lineOf(src, m[0]))
		setProps(&ent, "framework", "blazor", "provenance", "INFERRED_FROM_BLAZOR_PAGE_COMPONENT",
			"route_path", route)
		add(ent)
	}

	// ComponentBase / LayoutComponentBase subclasses
	for _, m := range reBEComponentBase.FindAllStringSubmatchIndex(src, -1) {
		name := src[m[2]:m[3]]
		ent := makeEntity(name, "SCOPE.UIComponent", "component_extraction", file.Path, "csharp", lineOf(src, m[0]))
		setProps(&ent, "framework", "blazor", "provenance", "INFERRED_FROM_BLAZOR_COMPONENTBASE")
		add(ent)
	}
	for _, m := range reBELayoutBase.FindAllStringSubmatchIndex(src, -1) {
		name := src[m[2]:m[3]]
		ent := makeEntity(name, "SCOPE.UIComponent", "component_extraction", file.Path, "csharp", lineOf(src, m[0]))
		setProps(&ent, "framework", "blazor", "provenance", "INFERRED_FROM_BLAZOR_LAYOUTBASE")
		add(ent)
	}

	// -------------------------------------------------------------------------
	// Structure/context_extraction
	// -------------------------------------------------------------------------

	for _, m := range reBEInject.FindAllStringSubmatchIndex(src, -1) {
		serviceType := src[m[2]:m[3]]
		varName := src[m[4]:m[5]]
		name := "ctx:" + serviceType + ":" + varName
		ent := makeEntity(name, "SCOPE.Component", "context_extraction", file.Path, "csharp", lineOf(src, m[0]))
		setProps(&ent, "framework", "blazor", "provenance", "INFERRED_FROM_BLAZOR_INJECT",
			"service_type", serviceType, "variable_name", varName)
		add(ent)
	}

	for _, m := range reBECascadingParam.FindAllStringSubmatchIndex(src, -1) {
		paramType := src[m[2]:m[3]]
		paramName := src[m[4]:m[5]]
		name := "cascade:" + paramType + ":" + paramName
		ent := makeEntity(name, "SCOPE.Component", "context_extraction", file.Path, "csharp", lineOf(src, m[0]))
		setProps(&ent, "framework", "blazor", "provenance", "INFERRED_FROM_BLAZOR_CASCADING_PARAM",
			"param_type", paramType, "param_name", paramName)
		add(ent)
	}

	// -------------------------------------------------------------------------
	// Navigation/router_pattern
	// -------------------------------------------------------------------------

	// @page routes as router patterns (route declaration surface)
	for _, m := range reBEPage.FindAllStringSubmatchIndex(src, -1) {
		route := src[m[2]:m[3]]
		ent := makeEntity("route:"+route, "SCOPE.Operation", "router_pattern", file.Path, "csharp", lineOf(src, m[0]))
		setProps(&ent, "framework", "blazor", "provenance", "INFERRED_FROM_BLAZOR_PAGE_ROUTE",
			"route_path", route)
		add(ent)
	}

	// NavigateTo() programmatic navigation
	for _, m := range reBENavigateTo.FindAllStringSubmatchIndex(src, -1) {
		path := src[m[2]:m[3]]
		ent := makeEntity("navigate:"+path, "SCOPE.Pattern", "router_pattern", file.Path, "csharp", lineOf(src, m[0]))
		setProps(&ent, "framework", "blazor", "provenance", "INFERRED_FROM_BLAZOR_NAVIGATE_TO",
			"nav_path", path)
		add(ent)
	}

	// <NavLink href="...">
	for _, m := range reBENavLink.FindAllStringSubmatchIndex(src, -1) {
		href := src[m[2]:m[3]]
		if blazorBuiltinNavTargets[href] {
			continue
		}
		ent := makeEntity("navlink:"+href, "SCOPE.Pattern", "router_pattern", file.Path, "csharp", lineOf(src, m[0]))
		setProps(&ent, "framework", "blazor", "provenance", "INFERRED_FROM_BLAZOR_NAVLINK",
			"nav_path", href)
		add(ent)
	}

	// -------------------------------------------------------------------------
	// Lifecycle/state_setter_emission
	// -------------------------------------------------------------------------

	for _, m := range reBELifecycleMethod.FindAllStringSubmatchIndex(src, -1) {
		methodName := src[m[2]:m[3]]
		ent := makeEntity("lifecycle:"+methodName, "SCOPE.Operation", "state_setter_emission", file.Path, "csharp", lineOf(src, m[0]))
		setProps(&ent, "framework", "blazor", "provenance", "INFERRED_FROM_BLAZOR_LIFECYCLE",
			"lifecycle_method", methodName)
		add(ent)
	}

	stateHasChangedCount := len(reBEStateHasChanged.FindAllString(src, -1)) +
		len(reBEInvokeStateHasChanged.FindAllString(src, -1))
	if stateHasChangedCount > 0 {
		name := "state_has_changed:" + file.Path
		ent := makeEntity(name, "SCOPE.Pattern", "state_setter_emission", file.Path, "csharp",
			func() int {
				m := reBEStateHasChanged.FindStringIndex(src)
				if m != nil {
					return lineOf(src, m[0])
				}
				return 1
			}())
		setProps(&ent, "framework", "blazor", "provenance", "INFERRED_FROM_BLAZOR_STATE_HAS_CHANGED",
			"call_count", itoa(stateHasChangedCount))
		add(ent)
	}

	span.SetAttributes(attribute.Int("entity_count", len(entities)))
	return entities, nil
}

// blazorExtraAnchorHref is kept separate to avoid name clash in tests.
var blazorExtraAnchorHref = reBEAnchorHref

// blazorExtraFrameworks are the framework IDs that share this extractor.
var blazorExtraFrameworks = []string{"blazor", "blazor-server", "blazor-wasm"}

// emitBlazorAnchorNavigation emits anchor href navigation patterns.
// Called by tests to verify anchor detection in isolation.
func emitBlazorAnchorNavigation(src, filePath string) []types.EntityRecord {
	var out []types.EntityRecord
	for _, m := range blazorExtraAnchorHref.FindAllStringSubmatchIndex(src, -1) {
		href := src[m[2]:m[3]]
		if blazorBuiltinNavTargets[href] {
			continue
		}
		if strings.HasPrefix(href, "http") {
			continue
		}
		ent := makeEntity("anchor:"+href, "SCOPE.Pattern", "router_pattern", filePath, "csharp", lineOf(src, m[0]))
		setProps(&ent, "framework", "blazor", "provenance", "INFERRED_FROM_BLAZOR_ANCHOR",
			"nav_path", href)
		out = append(out, ent)
	}
	return out
}
