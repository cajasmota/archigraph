package golang

import (
	"context"
	"regexp"
	"strings"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/trace"

	"github.com/cajasmota/archigraph/internal/extractor"
	"github.com/cajasmota/archigraph/internal/types"
)

func init() {
	extractor.Register("custom_go_kratos", &kratosExtractor{})
}

// kratosExtractor extracts routing structure from go-kratos
// (github.com/go-kratos/kratos/v2) services. Kratos is proto/codegen-driven:
// the protoc-gen-go-http plugin generates a `*_http.pb.go` file per service
// containing a `RegisterXxxHTTPServer(s, srv)` function whose body wires the
// transport verb calls against a router obtained from `s.Route(...)` —
//
//	func RegisterGreeterHTTPServer(s *http.Server, srv GreeterHTTPServer) {
//		r := s.Route("/")
//		r.GET("/helloworld/{name}", _Greeter_SayHello0_HTTP_Handler(srv))
//	}
//
// Each `r.GET/POST/...("/path", _Svc_Method0_HTTP_Handler(srv))` registration
// yields an endpoint. The handler is the generated `_Svc_Method_HTTP_Handler`
// wrapper, from which the underlying service method name is recovered for
// handler attribution. The `RegisterXxxHTTPServer` function itself is recorded
// as the service-registration scope.
//
// Honesty note: this targets the *generated* `*_http.pb.go` output. When that
// file is present in the repo (the common committed-codegen case) routes and
// handlers resolve fully from a single statically-analysable AST shape — the
// proving fixture exercises exactly this. When only the `.proto` source is
// present and the generated file is absent, no registration sites exist to
// detect; that is an inherent limit of the proto-only layout, not a heuristic
// gap.
type kratosExtractor struct{}

func (e *kratosExtractor) Language() string { return "custom_go_kratos" }

var (
	// func RegisterGreeterHTTPServer(s *http.Server, srv GreeterHTTPServer) {
	// Captures the service token (e.g. "Greeter") from the generated
	// registration entry point.
	reKratosRegister = regexp.MustCompile(
		`(?m)func\s+Register(\w+?)HTTPServer\s*\(`,
	)
	// r := s.Route("/") — router handle obtained from the *http.Server inside a
	// RegisterXxxHTTPServer body. The optional prefix becomes a route prefix.
	reKratosRoute = regexp.MustCompile(
		`(?m)(\w+)\s*:?=\s*(\w+)\.Route\s*\(\s*"([^"]*)"`,
	)
	// r.GET("/helloworld/{name}", _Greeter_SayHello0_HTTP_Handler(srv))
	// verb registration with a generated `_Svc_Method<idx>_HTTP_Handler`
	// handler. The handler identifier is captured whole for attribution.
	reKratosVerb = regexp.MustCompile(
		`(?m)(\w+)\.(GET|POST|PUT|DELETE|PATCH|HEAD|OPTIONS)\s*\(\s*"([^"]+)"\s*,\s*([A-Za-z_]\w*)`,
	)
	// _Greeter_SayHello0_HTTP_Handler -> service "Greeter", method "SayHello".
	// The trailing numeric suffix is protoc-gen-go-http's per-binding index.
	reKratosHandler = regexp.MustCompile(
		`^_(\w+?)_(\w+?)(\d+)_HTTP_Handler$`,
	)
)

// kratosMethodFromHandler recovers the underlying service method name from a
// generated `_Svc_Method<idx>_HTTP_Handler` wrapper identifier. Returns "" when
// the identifier is not a generated kratos handler wrapper.
func kratosMethodFromHandler(handler string) string {
	m := reKratosHandler.FindStringSubmatch(handler)
	if m == nil {
		return ""
	}
	return m[2]
}

func (e *kratosExtractor) Extract(ctx context.Context, file extractor.FileInput) ([]types.EntityRecord, error) {
	tracer := otel.Tracer("archigraph/custom/golang")
	_, span := tracer.Start(ctx, "indexer.kratos_extractor.extract",
		trace.WithAttributes(
			attribute.String("language", file.Language),
			attribute.String("framework", "kratos"),
			attribute.String("file_path", file.Path),
		),
	)
	defer span.End()

	if len(file.Content) == 0 || file.Language != "go" {
		return nil, nil
	}

	src := string(file.Content)
	// Two gates. The routing pass (steps 1–3) is gated on the generated-HTTP-
	// transport signature (the RegisterXxxHTTPServer entry point + the generated
	// _HTTP_Handler wrapper suffix), keeping it inert on hand-written kratos
	// code. The middleware/auth/validation passes (step 4) model the hand-written
	// wiring + the protoc-gen-validate output, so they run on any file carrying a
	// kratos marker. A file with neither signature emits nothing.
	hasTransport := strings.Contains(src, "HTTPServer") && strings.Contains(src, "_HTTP_Handler")
	hasKratosMarker := strings.Contains(src, "go-kratos/kratos") ||
		strings.Contains(src, "selector.Server") ||
		strings.Contains(src, "validate.Validator") ||
		reKratosPGVMethod.MatchString(src)
	if !hasTransport && !hasKratosMarker {
		return nil, nil
	}

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

	if !hasTransport {
		// No generated transport in this file — only the wiring/validation
		// passes apply. Skip the routing scans below.
		e.extractMiddlewareAuth(src, file, add)
		e.extractRequestValidation(src, file, add)
		span.SetAttributes(attribute.Int("entity_count", len(entities)))
		return entities, nil
	}

	// 1. RegisterXxxHTTPServer entry points -> SCOPE.Service (one per service).
	for _, m := range reKratosRegister.FindAllStringSubmatchIndex(src, -1) {
		svc := submatch(src, m, 2)
		ent := makeEntity(svc, "SCOPE.Service", "", file.Path, file.Language, lineOf(src, m[0]))
		setProps(&ent, "framework", "kratos", "provenance", "INFERRED_FROM_KRATOS_HTTP_REGISTER",
			"service", svc)
		add(ent)
	}

	// 2. r := s.Route("/prefix") -> router-var prefix map (+ SCOPE.Component
	//    when a non-empty prefix is declared).
	routePrefix := make(map[string]string) // router var -> prefix
	for _, m := range reKratosRoute.FindAllStringSubmatchIndex(src, -1) {
		routerVar := submatch(src, m, 2)
		prefix := submatch(src, m, 6)
		if prefix == "/" {
			prefix = "" // root mount adds no path segment
		}
		routePrefix[routerVar] = prefix
		if prefix != "" {
			ent := makeEntity(prefix, "SCOPE.Component", "", file.Path, file.Language, lineOf(src, m[0]))
			setProps(&ent, "framework", "kratos", "provenance", "INFERRED_FROM_KRATOS_ROUTE",
				"group_path", prefix)
			add(ent)
		}
	}

	// 3. r.GET/POST/...("/path", _Svc_Method0_HTTP_Handler(srv)) verb routes ->
	//    SCOPE.Operation/endpoint, with the underlying service method recovered
	//    from the generated handler wrapper for handler attribution.
	for _, m := range reKratosVerb.FindAllStringSubmatchIndex(src, -1) {
		routerVar := submatch(src, m, 2)
		method := strings.ToUpper(submatch(src, m, 4))
		path := submatch(src, m, 6)
		handler := submatch(src, m, 8)
		if p, ok := routePrefix[routerVar]; ok && p != "" {
			path = p + path
		}
		name := method + " " + path
		ent := makeEntity(name, "SCOPE.Operation", "endpoint", file.Path, file.Language, lineOf(src, m[0]))
		setProps(&ent, "framework", "kratos", "provenance", "INFERRED_FROM_KRATOS_ROUTE",
			"http_method", method, "route_path", path, "router_var", routerVar)
		ent.Properties["handler"] = handler
		if svcMethod := kratosMethodFromHandler(handler); svcMethod != "" {
			ent.Properties["service_method"] = svcMethod
		}
		add(ent)
	}

	// 4. middleware / auth / request_validation surfaces (issue #3255).
	e.extractMiddlewareAuth(src, file, add)
	e.extractRequestValidation(src, file, add)

	span.SetAttributes(attribute.Int("entity_count", len(entities)))
	return entities, nil
}

// ---------------------------------------------------------------------------
// Middleware + auth + request validation (issue #3255).
//
// kratos's routing extractor above models the generated *_http.pb.go transport
// only. The three capabilities below model the hand-written wiring layer of a
// kratos service, which the routing pass does not touch:
//
//   - middleware_coverage : middleware is installed on a transport server via
//       the http.Middleware(...) / grpc.Middleware(...) server option, and
//       scoped per-operation via selector.Server(...).Match(...).Build(). Each
//       middleware constructor in those lists is one ordered SCOPE.Pattern
//       (pattern_kind=middleware).
//   - auth_coverage      : kratos ships jwt.Server(...) / auth middleware; any
//       middleware whose expression classifies as auth (jwt/oauth/…) is flagged
//       is_auth + auth_kind and re-emitted as a dedicated auth SCOPE.Pattern.
//   - request_validation : protoc-gen-validate generates a Validate()/
//       ValidateAll() method on each request message; the kratos validate
//       middleware (github.com/go-kratos/kratos/v2/middleware/validate) invokes
//       it. Both the generated `func (m *X) Validate() error` method and the
//       validate.Validator() middleware install are validation surfaces.
//
// Honesty: all three are heuristic identifier/substring matches on source text
// with no data-flow proof that a middleware is actually wired into a live
// request path, so each capability is reported `partial`. kratos genuinely has
// all three concepts (it is not NA): the proving fixture exercises each.
// ---------------------------------------------------------------------------

var (
	// http.Middleware( | grpc.Middleware( | .Middleware( server-option call —
	// the balanced argument list holds an ordered middleware-constructor chain.
	reKratosMiddlewareHead = regexp.MustCompile(`(?:\bhttp|\bgrpc|\w+)\.Middleware\s*\(`)
	// selector.Server( per-operation middleware scoping: the argument list is a
	// middleware chain applied to the matched operations.
	reKratosSelectorHead = regexp.MustCompile(`selector\.Server\s*\(`)
	// protoc-gen-validate generated method: func (m *HelloRequest) Validate() error
	// (and the ValidateAll variant). Names the message type carrying the rules.
	reKratosPGVMethod = regexp.MustCompile(
		`(?m)func\s*\(\s*\w+\s+\*?(\w+)\s*\)\s*Validate(?:All)?\s*\(\s*\)\s*(?:error|\(\s*error)`,
	)
	// validate.Validator() — the kratos request-validation middleware install.
	reKratosValidateMW = regexp.MustCompile(`validate\.Validator\s*\(`)
)

// kratosClassifyAuth returns a coarse auth kind for a middleware expression, or
// "" if it does not look like an auth middleware. File-local so the shared
// helpers stay untouched (contention rule). Mirrors the catalog used by the
// gin/echo/fiber/chi shared detector, plus kratos-specific spellings.
func kratosClassifyAuth(expr string) string {
	low := strings.ToLower(expr)
	switch {
	case strings.Contains(low, "jwt"), strings.Contains(low, "bearer"):
		return "jwt"
	case strings.Contains(low, "oauth"):
		return "oauth"
	case strings.Contains(low, "apikey"), strings.Contains(low, "api_key"), strings.Contains(low, "keyauth"):
		return "api_key"
	case strings.Contains(low, "basicauth"), strings.Contains(low, "basic_auth"):
		return "basic"
	case strings.Contains(low, "casbin"), strings.Contains(low, "rbac"):
		return "rbac"
	case strings.Contains(low, "authz"), strings.Contains(low, "authorize"):
		return "authz"
	case strings.Contains(low, "auth"):
		return "auth"
	}
	return ""
}

// reKratosCallHead extracts the leading identifier/selector of a middleware
// expression (the part before any "(" call).
var reKratosCallHead = regexp.MustCompile(`^[A-Za-z_][\w.]*`)

// extractMiddlewareAuth scans http.Middleware(...) / grpc.Middleware(...) and
// selector.Server(...) chains, emitting one ordered middleware SCOPE.Pattern
// per constructor and a dedicated auth SCOPE.Pattern for auth-classified ones.
func (e *kratosExtractor) extractMiddlewareAuth(src string, file extractor.FileInput, add func(types.EntityRecord)) {
	emit := func(head []int, scope string) {
		open := head[1] - 1
		args, end := balancedArgs(src, open)
		if end < 0 {
			return
		}
		line := lineOf(src, head[0])
		order := 0
		for _, part := range splitTopLevelArgs(args) {
			if part == "" || isStringLiteral(part) {
				continue
			}
			name := reKratosCallHead.FindString(part)
			if name == "" {
				name = part
			}
			mw := makeEntity(part, "SCOPE.Pattern", "", file.Path, file.Language, line)
			setProps(&mw, "framework", "kratos",
				"provenance", "INFERRED_FROM_KRATOS_MIDDLEWARE",
				"pattern_kind", "middleware",
				"middleware_name", name,
				"mw_scope", scope,
				"mw_order", itoa(order))
			authKind := kratosClassifyAuth(part)
			if authKind != "" {
				setProps(&mw, "is_auth", "true", "auth_kind", authKind)
			}
			add(mw)
			if authKind != "" {
				au := makeEntity("auth:"+name, "SCOPE.Pattern", "", file.Path, file.Language, line)
				setProps(&au, "framework", "kratos",
					"provenance", "INFERRED_FROM_KRATOS_AUTH",
					"pattern_kind", "auth",
					"auth_kind", authKind,
					"middleware_name", name,
					"middleware_expr", part)
				add(au)
			}
			order++
		}
	}
	for _, m := range reKratosMiddlewareHead.FindAllStringSubmatchIndex(src, -1) {
		emit(m, "server")
	}
	for _, m := range reKratosSelectorHead.FindAllStringSubmatchIndex(src, -1) {
		emit(m, "operation")
	}
}

// extractRequestValidation emits a validation SCOPE.Pattern for each
// protoc-gen-validate generated Validate()/ValidateAll() method (named by its
// message type) and for the validate.Validator() middleware install.
func (e *kratosExtractor) extractRequestValidation(src string, file extractor.FileInput, add func(types.EntityRecord)) {
	for _, m := range reKratosPGVMethod.FindAllStringSubmatchIndex(src, -1) {
		msg := submatch(src, m, 2)
		ent := makeEntity("validation:rule:"+msg, "SCOPE.Pattern", "", file.Path, file.Language, lineOf(src, m[0]))
		setProps(&ent, "framework", "kratos",
			"provenance", "INFERRED_FROM_KRATOS_VALIDATION",
			"pattern_kind", "validation",
			"validation_kind", "rule",
			"validation_subtype", "pgv_message",
			"struct_name", msg,
			"rule_source", "protoc-gen-validate")
		add(ent)
	}
	for _, m := range reKratosValidateMW.FindAllStringIndex(src, -1) {
		ent := makeEntity("validation:binding:validate_call", "SCOPE.Pattern", "", file.Path, file.Language, lineOf(src, m[0]))
		setProps(&ent, "framework", "kratos",
			"provenance", "INFERRED_FROM_KRATOS_VALIDATION",
			"pattern_kind", "validation",
			"validation_kind", "binding",
			"validation_subtype", "validate_middleware")
		add(ent)
	}
}
