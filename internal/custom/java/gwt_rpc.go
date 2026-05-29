package java

import (
	"regexp"
	"strconv"
)

// GWT RPC / RequestFactory data_fetching custom extractor (#3190).
//
// GWT (Google Web Toolkit) compiles Java to client-side JavaScript. Its two
// canonical client→server data-fetching mechanisms are:
//
//   1. GWT-RPC — a service interface annotated with
//      @RemoteServiceRelativePath("...") plus an async mirror interface; the
//      client obtains a proxy via GWT.create(MyService.class) and issues calls
//      that complete through an AsyncCallback<T>.
//
//   2. RequestFactory — a RequestFactory subinterface exposing
//      RequestContext factories; data is fetched by building a Request<T> /
//      InstanceRequest and invoking .fire(Receiver<T>). Domain objects cross
//      the wire as EntityProxy / ValueProxy beans.
//
// The pre-existing vaadin_gwt.go extractor only records the server-side
// RemoteServiceServlet taint marker (component/router cells). It does NOT
// record the client-side data-fetch surface, so the gwt.data_fetching cell
// was "missing". This file flips it to "partial": it is heuristic regex
// detection of the well-known RPC / RequestFactory shapes, not a full
// data-flow proof, hence partial (mirroring Vaadin's data_fetching).
//
// Coverage cell delivered (#3190):
//   - data_fetching → partial  (RPC service ifaces, GWT.create proxies,
//                               AsyncCallback sites, RequestFactory contexts,
//                               EntityProxy beans, Request.fire receivers)
//
// Entities emitted are SCOPE.Operation / SCOPE.Service / SCOPE.DataModel
// markers gated to the gwt framework; they never fire on non-GWT sources.

// ─── GWT data-fetching regexps ──────────────────────────────────────────────

var (
	// @RemoteServiceRelativePath("greet") interface Foo extends RemoteService —
	// the RPC service-endpoint declaration (client-visible binding path).
	gwtRemoteServicePathRE = regexp.MustCompile(
		`(?s)@RemoteServiceRelativePath\s*\(\s*"([^"]*)"\s*\)` +
			`[^{;]*?interface\s+(\w+)`)

	// GWT.create(MyService.class) — RPC (or RequestFactory) proxy creation site.
	gwtCreateProxyRE = regexp.MustCompile(
		`(?m)\bGWT\s*\.\s*create\s*\(\s*(\w+)\s*\.\s*class\s*\)`)

	// new AsyncCallback<Foo>() / AsyncCallback<Foo> cb — RPC async completion.
	gwtAsyncCallbackRE = regexp.MustCompile(
		`(?m)\bAsyncCallback\s*<\s*([\w.<>\[\]?, ]*?)\s*>`)

	// interface AppRequestFactory extends RequestFactory — RF root factory.
	gwtRequestFactoryIfaceRE = regexp.MustCompile(
		`(?m)interface\s+(\w+)\s+extends\s+(?:[\w.]+\s*,\s*)*RequestFactory\b`)

	// interface FooRequest extends RequestContext — RF request context (the
	// unit that carries one or more server method invocations).
	gwtRequestContextIfaceRE = regexp.MustCompile(
		`(?m)interface\s+(\w+)\s+extends\s+(?:[\w.]+\s*,\s*)*RequestContext\b`)

	// @ProxyFor(Foo.class) / @ProxyForName("...") interface FooProxy
	// extends EntityProxy|ValueProxy — RF domain proxy bean.
	gwtProxyForRE = regexp.MustCompile(
		`(?s)@ProxyFor(?:Name)?\s*\(\s*"?([\w.$]+?)"?(?:\s*\.\s*class)?\s*\)` +
			`[^{;]*?interface\s+(\w+)`)

	// interface FooProxy extends EntityProxy|ValueProxy (no @ProxyFor) — RF bean.
	gwtProxyIfaceRE = regexp.MustCompile(
		`(?m)interface\s+(\w+)\s+extends\s+(?:[\w.]+\s*,\s*)*(?:EntityProxy|ValueProxy)\b`)

	// someRequest.fire(new Receiver<Foo>() ...) / .fire(receiver) — RF fetch site.
	gwtRequestFireRE = regexp.MustCompile(
		`(?m)\.\s*fire\s*\(\s*(?:new\s+Receiver\b|[\w.]+\s*\))`)
)

// ─── ExtractGWTDataFetching ──────────────────────────────────────────────────

// ExtractGWTDataFetching runs the GWT RPC / RequestFactory data_fetching
// extractor. Delivers partial coverage for: data_fetching.
//
// It is gated to language=java and the gwt framework (reusing the
// gwtFrameworkMatches gate from vaadin_gwt.go) and is a no-op otherwise.
func ExtractGWTDataFetching(ctx PatternContext) PatternResult {
	var result PatternResult
	if ctx.Language != "java" || !gwtFrameworkMatches(ctx.Framework) {
		return result
	}

	source := ctx.Source
	fp := ctx.FilePath
	seenRefs := make(map[string]bool)
	seenRels := make(map[relKey]bool)

	// --- @RemoteServiceRelativePath RPC service interfaces ---
	for _, m := range gwtRemoteServicePathRE.FindAllStringSubmatchIndex(source, -1) {
		path := source[m[2]:m[3]]
		svc := source[m[4]:m[5]]
		ref := "scope:service:gwt_rpc_service:" + fp + ":" + svc
		addEntity(&result, seenRefs, SecondaryEntity{
			Name: svc, Kind: "SCOPE.Service", Subtype: "rpc_service",
			SourceFile: fp,
			LineStart:  lineOf(source, m[0]), LineEnd: lineOf(source, m[0]),
			Provenance: "INFERRED_FROM_GWT_RPC_SERVICE", Ref: ref,
			Properties: map[string]any{
				"data_fetch_kind": "rpc_service", "relative_path": path,
				"framework": "gwt",
			},
		})
	}

	// --- GWT.create(Service.class) RPC/RF proxy creation sites ---
	for _, m := range gwtCreateProxyRE.FindAllStringSubmatchIndex(source, -1) {
		target := source[m[2]:m[3]]
		host := findEnclosingClass(source, m[0])
		if host == "" {
			host = "unknown"
		}
		ref := "scope:operation:gwt_create_proxy:" + fp + ":" + host + ":" + target
		svcRef := "scope:service:gwt_rpc_service:" + fp + ":" + target
		if addEntity(&result, seenRefs, SecondaryEntity{
			Name: host + "::GWT.create(" + target + ")", Kind: "SCOPE.Operation", Subtype: "data_fetch",
			SourceFile: fp,
			LineStart:  lineOf(source, m[0]), LineEnd: lineOf(source, m[0]),
			Provenance: "INFERRED_FROM_GWT_CREATE_PROXY", Ref: ref,
			Properties: map[string]any{
				"data_fetch_kind": "rpc_proxy_create", "service_type": target,
				"host_class": host, "framework": "gwt",
			},
		}) {
			// Link the proxy-create site to the service interface it targets.
			addRel(&result, seenRels, Relationship{
				SourceRef: ref, TargetRef: svcRef, RelationshipType: "FETCHES_FROM",
				Properties: map[string]string{"service_type": target},
			})
		}
	}

	// --- AsyncCallback<T> RPC completion sites (data_fetching presence) ---
	for _, m := range gwtAsyncCallbackRE.FindAllStringSubmatchIndex(source, -1) {
		payload := source[m[2]:m[3]]
		host := findEnclosingClass(source, m[0])
		if host == "" {
			host = "unknown"
		}
		ref := "scope:operation:gwt_async_callback:" + fp + ":" + host + ":" + payload
		addEntity(&result, seenRefs, SecondaryEntity{
			Name: host + "::AsyncCallback<" + payload + ">", Kind: "SCOPE.Operation", Subtype: "data_fetch",
			SourceFile: fp,
			LineStart:  lineOf(source, m[0]), LineEnd: lineOf(source, m[0]),
			Provenance: "INFERRED_FROM_GWT_ASYNC_CALLBACK", Ref: ref,
			Properties: map[string]any{
				"data_fetch_kind": "rpc_async_callback", "payload_type": payload,
				"host_class": host, "framework": "gwt",
			},
		})
	}

	// --- RequestFactory root interfaces ---
	for _, m := range gwtRequestFactoryIfaceRE.FindAllStringSubmatchIndex(source, -1) {
		name := source[m[2]:m[3]]
		ref := "scope:service:gwt_request_factory:" + fp + ":" + name
		addEntity(&result, seenRefs, SecondaryEntity{
			Name: name, Kind: "SCOPE.Service", Subtype: "request_factory",
			SourceFile: fp,
			LineStart:  lineOf(source, m[0]), LineEnd: lineOf(source, m[0]),
			Provenance: "INFERRED_FROM_GWT_REQUEST_FACTORY", Ref: ref,
			Properties: map[string]any{
				"data_fetch_kind": "request_factory", "framework": "gwt",
			},
		})
	}

	// --- RequestContext interfaces (RF request units) ---
	for _, m := range gwtRequestContextIfaceRE.FindAllStringSubmatchIndex(source, -1) {
		name := source[m[2]:m[3]]
		ref := "scope:service:gwt_request_context:" + fp + ":" + name
		addEntity(&result, seenRefs, SecondaryEntity{
			Name: name, Kind: "SCOPE.Service", Subtype: "request_context",
			SourceFile: fp,
			LineStart:  lineOf(source, m[0]), LineEnd: lineOf(source, m[0]),
			Provenance: "INFERRED_FROM_GWT_REQUEST_CONTEXT", Ref: ref,
			Properties: map[string]any{
				"data_fetch_kind": "request_context", "framework": "gwt",
			},
		})
	}

	// --- @ProxyFor/@ProxyForName domain proxy beans ---
	for _, m := range gwtProxyForRE.FindAllStringSubmatchIndex(source, -1) {
		domain := source[m[2]:m[3]]
		proxy := source[m[4]:m[5]]
		ref := "scope:datamodel:gwt_proxy:" + fp + ":" + proxy
		addEntity(&result, seenRefs, SecondaryEntity{
			Name: proxy, Kind: "SCOPE.DataModel", Subtype: "entity_proxy",
			SourceFile: fp,
			LineStart:  lineOf(source, m[0]), LineEnd: lineOf(source, m[0]),
			Provenance: "INFERRED_FROM_GWT_PROXY", Ref: ref,
			Properties: map[string]any{
				"data_fetch_kind": "request_factory_proxy", "domain_type": domain,
				"framework": "gwt",
			},
		})
	}

	// --- bare proxy interfaces (extends EntityProxy/ValueProxy, no @ProxyFor) ---
	for _, m := range gwtProxyIfaceRE.FindAllStringSubmatchIndex(source, -1) {
		proxy := source[m[2]:m[3]]
		ref := "scope:datamodel:gwt_proxy:" + fp + ":" + proxy
		addEntity(&result, seenRefs, SecondaryEntity{
			Name: proxy, Kind: "SCOPE.DataModel", Subtype: "entity_proxy",
			SourceFile: fp,
			LineStart:  lineOf(source, m[0]), LineEnd: lineOf(source, m[0]),
			Provenance: "INFERRED_FROM_GWT_PROXY", Ref: ref,
			Properties: map[string]any{
				"data_fetch_kind": "request_factory_proxy", "framework": "gwt",
			},
		})
	}

	// --- request.fire(Receiver) RequestFactory fetch sites ---
	for _, m := range gwtRequestFireRE.FindAllStringSubmatchIndex(source, -1) {
		host := findEnclosingClass(source, m[0])
		if host == "" {
			host = "unknown"
		}
		ref := "scope:operation:gwt_request_fire:" + fp + ":" + host + ":" + strconv.Itoa(lineOf(source, m[0]))
		addEntity(&result, seenRefs, SecondaryEntity{
			Name: host + "::Request.fire", Kind: "SCOPE.Operation", Subtype: "data_fetch",
			SourceFile: fp,
			LineStart:  lineOf(source, m[0]), LineEnd: lineOf(source, m[0]),
			Provenance: "INFERRED_FROM_GWT_REQUEST_FIRE", Ref: ref,
			Properties: map[string]any{
				"data_fetch_kind": "request_factory_fire", "host_class": host,
				"framework": "gwt",
			},
		})
	}

	return result
}
