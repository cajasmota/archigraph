package golang

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
	extractor.Register("custom_go_fiber", &fiberExtractor{})
}

type fiberExtractor struct{}

func (e *fiberExtractor) Language() string { return "custom_go_fiber" }

var (
	reFiberEngine = regexp.MustCompile(
		`(?m)(\w+)\s*:?=\s*fiber\.New\s*\(\s*\)`,
	)
	reFiberGroup = regexp.MustCompile(
		`(?m)(\w+)\s*:?=\s*(\w+)\.Group\s*\(\s*"([^"]+)"`,
	)
	reFiberRoute = regexp.MustCompile(
		`(?m)(\w+)\.(Get|Post|Put|Delete|Patch|Head|Options|All|Connect|Trace)\s*\(\s*"([^"]+)"`,
	)
	reFiberBind = regexp.MustCompile(
		`(?m)c\.(BodyParser|QueryParser|ParamsParser|ReqHeaderParser)\s*\(\s*&?(\w+)`,
	)
	reFiberStatic = regexp.MustCompile(
		`(?m)(\w+)\.Static\s*\(\s*"([^"]+)"\s*,\s*"([^"]+)"`,
	)
	reFiberWebSocket = regexp.MustCompile(
		`(?m)websocket\.New\s*\(\s*(\w+)\s*\)`,
	)
)

func (e *fiberExtractor) Extract(ctx context.Context, file extractor.FileInput) ([]types.EntityRecord, error) {
	tracer := otel.Tracer("grafel/custom/golang")
	_, span := tracer.Start(ctx, "indexer.fiber_extractor.extract",
		trace.WithAttributes(
			attribute.String("language", file.Language),
			attribute.String("framework", "fiber"),
			attribute.String("file_path", file.Path),
		),
	)
	defer span.End()

	if len(file.Content) == 0 {
		return nil, nil
	}
	if file.Language != "go" {
		return nil, nil
	}

	src := string(file.Content)
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

	// #3628 rate-limit child — resolve route/group/engine rate-limit middleware
	// once so each route op carries the flat rate_limited / rate_limit /
	// rate_limit_scope / rate_limit_source contract. fiber supports all three
	// scopes: inline `app.Get("/x", limiterMw, h)` (route), `api := app.Group(
	// "/x", limiterMw)` (group), and `app.Use(limiterMw)` (engine).
	rlIdx := buildGoRouteRateLimitIndex(src)

	// 1. fiber.New() engine -> SCOPE.Service
	for _, m := range reFiberEngine.FindAllStringSubmatchIndex(src, -1) {
		varName := src[m[2]:m[3]]
		ent := makeEntity(varName, "SCOPE.Service", "", file.Path, file.Language, lineOf(src, m[0]))
		setProps(&ent, "framework", "fiber", "provenance", "INFERRED_FROM_FIBER_ENGINE")
		add(ent)
	}

	// 2. .Group() -> SCOPE.Component
	groupPaths := make(map[string]string)
	for _, m := range reFiberGroup.FindAllStringSubmatchIndex(src, -1) {
		varName := src[m[2]:m[3]]
		path := src[m[6]:m[7]]
		groupPaths[varName] = path
		ent := makeEntity(path, "SCOPE.Component", "", file.Path, file.Language, lineOf(src, m[0]))
		setProps(&ent, "framework", "fiber", "provenance", "INFERRED_FROM_FIBER_GROUP",
			"group_path", path)
		add(ent)
	}

	// 3. HTTP routes -> SCOPE.Operation/endpoint
	for _, m := range reFiberRoute.FindAllStringSubmatchIndex(src, -1) {
		routerVar := src[m[2]:m[3]]
		method := strings.ToUpper(src[m[4]:m[5]])
		ownPath := src[m[6]:m[7]]
		path := ownPath
		if gp, ok := groupPaths[routerVar]; ok {
			path = gp + path
		}
		name := method + " " + path
		ent := makeEntity(name, "SCOPE.Operation", "endpoint", file.Path, file.Language, lineOf(src, m[0]))
		setProps(&ent, "framework", "fiber", "provenance", "INFERRED_FROM_FIBER_ROUTE",
			"http_method", method, "route_path", path)
		// #3628 — stamp endpoint rate-limit posture (inline > group > engine). The
		// inline index is keyed by the route's own (un-prefixed) path.
		rlIdx.resolve(routerVar, method, ownPath).stamp(ent.Properties)
		add(ent)
	}

	// 4. .Use(middleware, …) -> ordered SCOPE.Pattern middleware (+ auth)
	for _, uc := range findUseCalls(src) {
		chain := parseMiddlewareChain(uc.Args)
		emitMiddlewareChain(add, chain, "fiber",
			"INFERRED_FROM_FIBER_MIDDLEWARE", "INFERRED_FROM_FIBER_AUTH",
			file.Path, file.Language, uc.Line)
	}

	// 5. c.BodyParser etc -> SCOPE.Schema
	for _, m := range reFiberBind.FindAllStringSubmatchIndex(src, -1) {
		bindMethod := src[m[2]:m[3]]
		bindType := src[m[4]:m[5]]
		name := "bind:" + bindMethod + ":" + bindType
		ent := makeEntity(name, "SCOPE.Schema", "", file.Path, file.Language, lineOf(src, m[0]))
		setProps(&ent, "framework", "fiber", "provenance", "INFERRED_FROM_FIBER_BINDING",
			"bind_method", bindMethod, "ctx_var", "c")
		add(ent)
	}

	// 6. Static -> SCOPE.Pattern
	for _, m := range reFiberStatic.FindAllStringSubmatchIndex(src, -1) {
		urlPrefix := src[m[4]:m[5]]
		dirPath := src[m[6]:m[7]]
		name := "static:" + urlPrefix
		ent := makeEntity(name, "SCOPE.Pattern", "", file.Path, file.Language, lineOf(src, m[0]))
		setProps(&ent, "framework", "fiber", "provenance", "INFERRED_FROM_FIBER_STATIC",
			"url_prefix", urlPrefix, "dir_path", dirPath)
		add(ent)
	}

	// 7. WebSocket -> SCOPE.Operation/endpoint
	for _, m := range reFiberWebSocket.FindAllStringSubmatchIndex(src, -1) {
		handler := src[m[2]:m[3]]
		ent := makeEntity("websocket:"+handler, "SCOPE.Operation", "endpoint", file.Path, file.Language, lineOf(src, m[0]))
		setProps(&ent, "framework", "fiber", "provenance", "INFERRED_FROM_FIBER_WEBSOCKET",
			"handler_name", handler)
		add(ent)
	}

	span.SetAttributes(attribute.Int("entity_count", len(entities)))
	return entities, nil
}
