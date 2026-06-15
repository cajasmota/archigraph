package rust

// ntex_loco_shuttle.go — route / deploy-substrate extractors for three more
// Rust frameworks deferred from the #4964 audit (follow-up #5008):
//
//   - ntex      : actix-derived async HTTP framework. Same #[get("/p")] route
//                 macros + manual web::resource("/p").route(web::get().to(h))
//                 + web::scope("/prefix") + ntex::web::HttpServer builder as
//                 actix, so this mirrors actix_web.go's producer surface.
//   - Loco.rs   : batteries-included MVC framework. Routes are declared with
//                 Routes::new().add("/path", get(handler)) (axum-style method
//                 routers under the hood) and prefixed via .prefix("/api").
//   - Shuttle   : a deploy / runtime substrate, NOT an HTTP router. The
//                 #[shuttle_runtime::main] entrypoint + #[shuttle_*::*]
//                 resource annotations (shuttle_shared_db::Postgres, etc.)
//                 declare the managed runtime + provisioned resources. We
//                 emit a SCOPE.Service for the runtime entrypoint and a
//                 SCOPE.Component per declared managed resource so the deploy
//                 substrate is visible in the graph.
//
// All patterns are regex-based, conservative, and fixture-proven. Each
// extractor returns nil for non-rust files (wrong-language no-op) and for
// files with no matching signal (no-match no-op).

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
	extractor.Register("custom_rust_ntex", &ntexExtractor{})
	extractor.Register("custom_rust_loco", &locoExtractor{})
	extractor.Register("custom_rust_shuttle", &shuttleExtractor{})
}

// ---------------------------------------------------------------------------
// ntex (actix-derived)
// ---------------------------------------------------------------------------

type ntexExtractor struct{}

func (e *ntexExtractor) Language() string { return "custom_rust_ntex" }

var (
	// ntex signal — require an ntex import / path so the actix-shaped route
	// regexes below cannot mis-fire on a real actix file (which has its own
	// extractor). ntex code references `ntex::web` / `ntex::` or `use ntex`.
	reNtexSignal = regexp.MustCompile(`\bntex\b`)

	// #[get("/path")] / #[web::get("/path")] async fn handler(...) — attribute
	// route macros (ntex exposes both the bare and web::-qualified forms).
	reNtexRouteMacro = regexp.MustCompile(
		`#\[(?:web::)?(get|post|put|delete|patch|head|options)\s*\(\s*"([^"]+)"\s*\)\][\s\S]*?(?:async\s+)?fn\s+(\w+)\s*\(`,
	)
	// web::scope("/prefix")
	reNtexScope = regexp.MustCompile(
		`web::scope\s*\(\s*"([^"]+)"\s*\)`,
	)
	// Manual route registration. ntex exposes BOTH the actix-style
	//   .route("/p", web::get().to(h))
	// and the resource-builder style
	//   web::resource("/p").route(web::get().to(h))
	// We capture the chained-on-resource verb+handler with the path on the
	// resource() call.
	reNtexRouteManual = regexp.MustCompile(
		`\.route\s*\(\s*"([^"]+)"\s*,\s*web::(get|post|put|delete|patch|head|options)\s*\(\s*\)\.to\s*\(\s*(\w+)\s*\)`,
	)
	reNtexResource = regexp.MustCompile(
		`web::resource\s*\(\s*"([^"]+)"\s*\)\s*\.route\s*\(\s*web::(get|post|put|delete|patch|head|options)\s*\(\s*\)\.to\s*\(\s*(\w+)\s*\)`,
	)
	// .wrap(Middleware)
	reNtexWrap = regexp.MustCompile(
		`\.wrap\s*\(\s*([A-Za-z_]\w*(?:::[A-Za-z_]\w*)*)`,
	)
	// HttpServer::new( — ntex::web::HttpServer / web::server builder.
	reNtexServer = regexp.MustCompile(
		`(?:HttpServer\s*::\s*new|web::server)\s*\(`,
	)
)

// ntexScopePrefix returns the nearest preceding web::scope("/prefix") on the
// same chain (no `;` separator) as the route at routeOff. Mirrors
// actixScopePrefix.
func ntexScopePrefix(src string, routeOff int) string {
	locs := reNtexScope.FindAllStringSubmatchIndex(src, -1)
	best := -1
	var prefix string
	for _, sm := range locs {
		if sm[0] > routeOff {
			break
		}
		if strings.ContainsAny(src[sm[1]:routeOff], ";") {
			continue
		}
		best = sm[0]
		prefix = rustNormalizePath(src[sm[2]:sm[3]])
	}
	if best < 0 {
		return ""
	}
	return prefix
}

func (e *ntexExtractor) Extract(ctx context.Context, file extractor.FileInput) ([]types.EntityRecord, error) {
	tracer := otel.Tracer("grafel/custom/rust")
	_, span := tracer.Start(ctx, "indexer.ntex_extractor.extract",
		trace.WithAttributes(
			attribute.String("language", file.Language),
			attribute.String("framework", "ntex"),
			attribute.String("file_path", file.Path),
		),
	)
	defer span.End()

	if len(file.Content) == 0 || file.Language != "rust" {
		return nil, nil
	}
	src := string(file.Content)
	if !reNtexSignal.MatchString(src) {
		return nil, nil
	}

	var entities []types.EntityRecord
	seen := make(map[string]bool)
	add := func(ent types.EntityRecord) {
		key := ent.Kind + ":" + ent.Name
		if seen[key] {
			return
		}
		seen[key] = true
		entities = append(entities, ent)
	}

	// 1. Attribute route macros -> SCOPE.Operation/endpoint.
	for _, m := range reNtexRouteMacro.FindAllStringSubmatchIndex(src, -1) {
		method := strings.ToUpper(src[m[2]:m[3]])
		path := rustNormalizePath(src[m[4]:m[5]])
		handler := src[m[6]:m[7]]
		name := method + " " + path
		ent := makeEntity(name, "SCOPE.Operation", "endpoint", file.Path, file.Language, lineOf(src, m[0]))
		setProps(&ent, "framework", "ntex", "provenance", "INFERRED_FROM_NTEX_ROUTE",
			"http_method", method, "route_pattern", path, "handler_name", handler)
		add(ent)
	}

	// 2. web::resource("/p").route(web::get().to(h)) -> endpoint (scope-aware).
	for _, m := range reNtexResource.FindAllStringSubmatchIndex(src, -1) {
		path := rustNormalizePath(src[m[2]:m[3]])
		method := strings.ToUpper(src[m[4]:m[5]])
		handler := src[m[6]:m[7]]
		prefix := ntexScopePrefix(src, m[0])
		fullPath := rustJoinPaths(prefix, path)
		name := method + " " + fullPath
		ent := makeEntity(name, "SCOPE.Operation", "endpoint", file.Path, file.Language, lineOf(src, m[0]))
		setProps(&ent, "framework", "ntex", "provenance", "INFERRED_FROM_NTEX_RESOURCE",
			"http_method", method, "route_pattern", fullPath, "handler_name", handler)
		if prefix != "" {
			setProps(&ent, "scope_prefix", prefix)
		}
		add(ent)
	}

	// 3. Manual .route("/p", web::get().to(h)) -> endpoint (scope-aware).
	for _, m := range reNtexRouteManual.FindAllStringSubmatchIndex(src, -1) {
		path := rustNormalizePath(src[m[2]:m[3]])
		method := strings.ToUpper(src[m[4]:m[5]])
		handler := src[m[6]:m[7]]
		prefix := ntexScopePrefix(src, m[0])
		fullPath := rustJoinPaths(prefix, path)
		name := method + " " + fullPath
		ent := makeEntity(name, "SCOPE.Operation", "endpoint", file.Path, file.Language, lineOf(src, m[0]))
		setProps(&ent, "framework", "ntex", "provenance", "INFERRED_FROM_NTEX_ROUTE",
			"http_method", method, "route_pattern", fullPath, "handler_name", handler)
		if prefix != "" {
			setProps(&ent, "scope_prefix", prefix)
		}
		add(ent)
	}

	// 4. web::scope("/prefix") -> SCOPE.Component.
	for _, m := range reNtexScope.FindAllStringSubmatchIndex(src, -1) {
		prefix := src[m[2]:m[3]]
		ent := makeEntity(prefix, "SCOPE.Component", "", file.Path, file.Language, lineOf(src, m[0]))
		setProps(&ent, "framework", "ntex", "provenance", "INFERRED_FROM_NTEX_SCOPE",
			"scope_prefix", prefix)
		add(ent)
	}

	// 5. .wrap(Middleware) -> SCOPE.Pattern.
	for _, m := range reNtexWrap.FindAllStringSubmatchIndex(src, -1) {
		mwType := src[m[2]:m[3]]
		ent := makeEntity("middleware:"+mwType, "SCOPE.Pattern", "", file.Path, file.Language, lineOf(src, m[0]))
		setProps(&ent, "framework", "ntex", "provenance", "INFERRED_FROM_NTEX_MIDDLEWARE",
			"middleware_type", mwType)
		add(ent)
	}

	// 6. HttpServer::new()/web::server() -> SCOPE.Service.
	for _, m := range reNtexServer.FindAllStringIndex(src, -1) {
		ent := makeEntity("ntex::HttpServer", "SCOPE.Service", "", file.Path, file.Language, lineOf(src, m[0]))
		setProps(&ent, "framework", "ntex", "provenance", "INFERRED_FROM_NTEX_SERVER")
		add(ent)
	}

	span.SetAttributes(attribute.Int("entity_count", len(entities)))
	return entities, nil
}

// ---------------------------------------------------------------------------
// Loco.rs
// ---------------------------------------------------------------------------

type locoExtractor struct{}

func (e *locoExtractor) Language() string { return "custom_rust_loco" }

var (
	// Loco signal — `loco_rs::` import path or `use loco_rs`.
	reLocoSignal = regexp.MustCompile(`\bloco_rs\b`)

	// Routes::new().add("/path", get(handler).post(other)) — a Loco controller
	// route. The verb argument is an axum-style method router (one or more
	// verb(handler) chained). We capture the path + the whole method-router
	// argument and re-scan with reLocoVerb. The method-router body excludes
	// parens/semicolons so the match cannot run past the .add(...) call.
	reLocoAdd = regexp.MustCompile(
		`\.add\s*\(\s*"([^"]+)"\s*,\s*((?:get|post|put|delete|patch|head|options)\s*\(\s*\w+\s*\)(?:\s*\.\s*(?:get|post|put|delete|patch|head|options)\s*\(\s*\w+\s*\))*)`,
	)
	// Individual verb(handler) calls inside a Loco method-router argument.
	reLocoVerb = regexp.MustCompile(
		`(get|post|put|delete|patch|head|options)\s*\(\s*(\w+)\s*\)`,
	)
	// Routes::new().prefix("/api") — controller mount prefix.
	reLocoPrefix = regexp.MustCompile(
		`\.prefix\s*\(\s*"([^"]+)"\s*\)`,
	)
)

// locoPrefix returns the nearest preceding .prefix("/p") on the same Routes
// chain (no `;` between it and the route at routeOff).
func locoPrefix(src string, routeOff int) string {
	locs := reLocoPrefix.FindAllStringSubmatchIndex(src, -1)
	best := -1
	var prefix string
	for _, sm := range locs {
		if sm[0] > routeOff {
			break
		}
		if strings.ContainsAny(src[sm[1]:routeOff], ";") {
			continue
		}
		best = sm[0]
		prefix = rustNormalizePath(src[sm[2]:sm[3]])
	}
	if best < 0 {
		return ""
	}
	return prefix
}

func (e *locoExtractor) Extract(ctx context.Context, file extractor.FileInput) ([]types.EntityRecord, error) {
	tracer := otel.Tracer("grafel/custom/rust")
	_, span := tracer.Start(ctx, "indexer.loco_extractor.extract",
		trace.WithAttributes(
			attribute.String("language", file.Language),
			attribute.String("framework", "loco"),
			attribute.String("file_path", file.Path),
		),
	)
	defer span.End()

	if len(file.Content) == 0 || file.Language != "rust" {
		return nil, nil
	}
	src := string(file.Content)
	if !reLocoSignal.MatchString(src) {
		return nil, nil
	}

	var entities []types.EntityRecord
	seen := make(map[string]bool)
	add := func(ent types.EntityRecord) {
		key := ent.Kind + ":" + ent.Name
		if seen[key] {
			return
		}
		seen[key] = true
		entities = append(entities, ent)
	}

	// 1. Routes::new().add("/p", get(h).post(h2)) -> one endpoint per verb.
	for _, m := range reLocoAdd.FindAllStringSubmatchIndex(src, -1) {
		path := rustNormalizePath(src[m[2]:m[3]])
		methodRouter := src[m[4]:m[5]]
		prefix := locoPrefix(src, m[0])
		fullPath := rustJoinPaths(prefix, path)
		for _, vm := range reLocoVerb.FindAllStringSubmatch(methodRouter, -1) {
			method := strings.ToUpper(vm[1])
			handler := vm[2]
			name := method + " " + fullPath
			ent := makeEntity(name, "SCOPE.Operation", "endpoint", file.Path, file.Language, lineOf(src, m[0]))
			setProps(&ent, "framework", "loco", "provenance", "INFERRED_FROM_LOCO_ROUTE",
				"http_method", method, "route_pattern", fullPath, "handler_name", handler)
			if prefix != "" {
				setProps(&ent, "scope_prefix", prefix)
			}
			add(ent)
		}
	}

	// 2. .prefix("/api") -> SCOPE.Component (controller mount).
	for _, m := range reLocoPrefix.FindAllStringSubmatchIndex(src, -1) {
		prefix := rustNormalizePath(src[m[2]:m[3]])
		ent := makeEntity("loco-prefix:"+prefix, "SCOPE.Component", "", file.Path, file.Language, lineOf(src, m[0]))
		setProps(&ent, "framework", "loco", "provenance", "INFERRED_FROM_LOCO_PREFIX",
			"scope_prefix", prefix)
		add(ent)
	}

	span.SetAttributes(attribute.Int("entity_count", len(entities)))
	return entities, nil
}

// ---------------------------------------------------------------------------
// Shuttle (deploy / runtime substrate)
// ---------------------------------------------------------------------------

type shuttleExtractor struct{}

func (e *shuttleExtractor) Language() string { return "custom_rust_shuttle" }

var (
	// Shuttle signal — any `shuttle_` crate reference.
	reShuttleSignal = regexp.MustCompile(`\bshuttle_\w+\b`)

	// #[shuttle_runtime::main] — the managed-runtime entrypoint. The annotated
	// async fn is the deploy entrypoint.
	reShuttleMain = regexp.MustCompile(
		`#\[shuttle_runtime::main\][\s\S]*?(?:async\s+)?fn\s+(\w+)\s*\(`,
	)
	// #[shuttle_shared_db::Postgres] / #[shuttle_secrets::Secrets] /
	// #[shuttle_aws_rds::Postgres] / #[shuttle_persist::Persist] — a managed
	// resource provisioned by the platform and injected into main(). Captures
	// the provider crate (group 1) + the resource type (group 2).
	reShuttleResource = regexp.MustCompile(
		`#\[(shuttle_\w+)::(\w+)\b`,
	)
)

func (e *shuttleExtractor) Extract(ctx context.Context, file extractor.FileInput) ([]types.EntityRecord, error) {
	tracer := otel.Tracer("grafel/custom/rust")
	_, span := tracer.Start(ctx, "indexer.shuttle_extractor.extract",
		trace.WithAttributes(
			attribute.String("language", file.Language),
			attribute.String("framework", "shuttle"),
			attribute.String("file_path", file.Path),
		),
	)
	defer span.End()

	if len(file.Content) == 0 || file.Language != "rust" {
		return nil, nil
	}
	src := string(file.Content)
	if !reShuttleSignal.MatchString(src) {
		return nil, nil
	}

	var entities []types.EntityRecord
	seen := make(map[string]bool)
	add := func(ent types.EntityRecord) {
		key := ent.Kind + ":" + ent.Name
		if seen[key] {
			return
		}
		seen[key] = true
		entities = append(entities, ent)
	}

	// 1. #[shuttle_runtime::main] entrypoint -> SCOPE.Service (deploy substrate).
	for _, m := range reShuttleMain.FindAllStringSubmatchIndex(src, -1) {
		fn := src[m[2]:m[3]]
		ent := makeEntity("shuttle::"+fn, "SCOPE.Service", "", file.Path, file.Language, lineOf(src, m[0]))
		setProps(&ent, "framework", "shuttle", "provenance", "INFERRED_FROM_SHUTTLE_MAIN",
			"deploy_runtime", "shuttle", "entrypoint", fn)
		add(ent)
	}

	// 2. #[shuttle_*::Resource] managed resources -> SCOPE.Component (one per
	//    provider::resource declaration). The #[shuttle_runtime::main] macro is
	//    the entrypoint, not a provisioned resource, so it is excluded here.
	for _, m := range reShuttleResource.FindAllStringSubmatchIndex(src, -1) {
		provider := src[m[2]:m[3]]
		resource := src[m[4]:m[5]]
		if provider == "shuttle_runtime" && resource == "main" {
			continue
		}
		name := provider + "::" + resource
		ent := makeEntity("shuttle-resource:"+name, "SCOPE.Component", "", file.Path, file.Language, lineOf(src, m[0]))
		setProps(&ent, "framework", "shuttle", "provenance", "INFERRED_FROM_SHUTTLE_RESOURCE",
			"deploy_runtime", "shuttle", "resource_provider", provider, "resource_type", resource)
		add(ent)
	}

	span.SetAttributes(attribute.Int("entity_count", len(entities)))
	return entities, nil
}
