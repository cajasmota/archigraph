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

	// --- middleware (#3255) -------------------------------------------------
	// http.Middleware(mw1, mw2, …) server option (and the grpc.Middleware twin)
	// that registers a global middleware chain on a kratos transport server:
	//
	//	srv := http.NewServer(http.Middleware(recovery.Recovery(), auth.JWT(...)))
	//
	// The balanced argument span is scanned forward so nested middleware
	// constructors are captured whole. Captures the receiver selector (e.g.
	// "http") so we can stamp the transport on the chain.
	reKratosMiddleware = regexp.MustCompile(`(\w+)\.Middleware\s*\(`)
	// selector.Server(mw…).Match(fn).Build() — per-route middleware selector,
	// a kratos idiom for applying middleware to a subset of operations. The
	// Server(...) argument list is the middleware chain; captured by balanced
	// scan from the opening paren.
	reKratosSelectorServer = regexp.MustCompile(`selector\s*\.\s*Server\s*\(`)

	// --- request validation (#3255) ----------------------------------------
	// protoc-gen-validate (PGV) generates a Validate()/ValidateAll() method per
	// message. kratos's validate middleware calls it on the decoded request.
	// Two surfaces are detected:
	//   1. the generated method definition: func (m *XReq) Validate() error
	//   2. the call site that enforces it:  if err := in.Validate(); err != nil
	reKratosValidateDef = regexp.MustCompile(
		`(?m)func\s*\(\s*\w+\s+\*?(\w+)\s*\)\s*(Validate|ValidateAll)\s*\(\s*\)\s*error`,
	)
	reKratosValidateCall = regexp.MustCompile(
		`(\w+)\.(Validate|ValidateAll)\s*\(\s*\)`,
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
	tracer := otel.Tracer("grafel/custom/golang")
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
	// Routing is gated on the generated-HTTP-transport signature (registration
	// entry point + generated handler wrapper suffix); middleware/auth/request-
	// validation run on any kratos file, gated on the broader kratos import/
	// transport marker so they also cover the hand-written server-wiring file
	// (where http.Middleware(...)/selector.Server(...) live) and the generated
	// *.pb.validate.go file (where the PGV Validate() methods live). A file with
	// neither signature is not kratos — emit nothing.
	hasRouting := strings.Contains(src, "HTTPServer") && strings.Contains(src, "_HTTP_Handler")
	if !hasRouting && !isKratosFile(src) {
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

	if !hasRouting {
		// Non-routing kratos file (server wiring / generated PGV validators):
		// only the middleware/auth/validation passes apply.
		emitKratosMiddlewareAndAuth(add, src, file.Path, file.Language)
		emitKratosValidation(add, src, file.Path, file.Language)
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

	// 4. middleware / auth / request-validation surfaces (#3255). These also run
	//    on the routing file when present (e.g. a hand-edited *_http.pb.go or a
	//    server file that both registers routes and wires middleware).
	emitKratosMiddlewareAndAuth(add, src, file.Path, file.Language)
	emitKratosValidation(add, src, file.Path, file.Language)

	span.SetAttributes(attribute.Int("entity_count", len(entities)))
	return entities, nil
}

// isKratosFile reports whether src looks like a go-kratos source file — used to
// gate the middleware/auth/validation passes on non-routing kratos files
// (server wiring, generated PGV validators). The markers are the kratos import
// path and its transport/middleware constructs.
func isKratosFile(src string) bool {
	return strings.Contains(src, "go-kratos/kratos") ||
		strings.Contains(src, "kratos/v2/middleware") ||
		strings.Contains(src, "transport/http") && strings.Contains(src, ".Middleware(")
}

// emitKratosMiddlewareAndAuth detects kratos middleware registration and
// classifies auth middleware. Two surfaces:
//
//	http.Middleware(mw1, mw2, …)      — global transport middleware chain
//	selector.Server(mw…).Match(…)     — per-route middleware selector
//
// Each middleware expression becomes an ordered SCOPE.Pattern
// (pattern_kind=middleware); any expression that classifies as auth (jwt/…)
// also yields a dedicated auth SCOPE.Pattern. JWT is the canonical kratos auth
// middleware (github.com/go-kratos/kratos/v2/middleware/auth/jwt), so the
// shared classifier already recognises it via the "jwt" needle.
//
// Honesty: heuristic substring/identifier match on source text; no import
// resolution or data-flow proof that the value enforces auth, and no binding to
// a specific route. Reported `partial`.
func emitKratosMiddlewareAndAuth(add func(types.EntityRecord), src, filePath, language string) {
	const mwProv = "INFERRED_FROM_KRATOS_MIDDLEWARE"
	const authProv = "INFERRED_FROM_KRATOS_AUTH"

	emit := func(headRe *regexp.Regexp, form string) {
		for _, loc := range headRe.FindAllStringSubmatchIndex(src, -1) {
			open := loc[1] - 1 // the '(' the head ends at
			args, end := balancedArgs(src, open)
			if end < 0 {
				continue
			}
			chain := parseMiddlewareChain(args)
			line := lineOf(src, loc[0])
			for _, a := range chain {
				mw := makeEntity(a.Expr, "SCOPE.Pattern", "", filePath, language, line)
				setProps(&mw, "framework", "kratos", "provenance", mwProv,
					"pattern_kind", "middleware",
					"middleware_name", a.Name,
					"middleware_form", form,
					"mw_order", itoa(a.Order))
				if a.AuthKind != "" {
					setProps(&mw, "is_auth", "true", "auth_kind", a.AuthKind)
				}
				add(mw)
				if a.AuthKind != "" {
					au := makeEntity("auth:"+a.Name, "SCOPE.Pattern", "", filePath, language, line)
					setProps(&au, "framework", "kratos", "provenance", authProv,
						"pattern_kind", "auth", "auth_kind", a.AuthKind,
						"middleware_name", a.Name, "middleware_expr", a.Expr)
					add(au)
				}
			}
		}
	}

	emit(reKratosMiddleware, "transport_middleware")
	emit(reKratosSelectorServer, "selector_server")
}

// emitKratosValidation detects protoc-gen-validate (PGV) request validation.
// PGV generates a Validate()/ValidateAll() method per request message and the
// kratos validate middleware (or handler code) invokes it on the decoded
// request. Two surfaces:
//
//	rule       — the generated `func (m *XReq) Validate() error` method def
//	binding    — the enforcing call site `in.Validate()` / `req.ValidateAll()`
//
// Honesty: heuristic source match; it does not confirm the method is wired into
// a request path or that every field rule is enforced. Reported `partial`.
func emitKratosValidation(add func(types.EntityRecord), src, filePath, language string) {
	const prov = "INFERRED_FROM_KRATOS_VALIDATION"

	for _, m := range reKratosValidateDef.FindAllStringSubmatchIndex(src, -1) {
		msg := submatch(src, m, 2)
		method := submatch(src, m, 4)
		name := "validation:rule:" + msg + "." + method
		ent := makeEntity(name, "SCOPE.Pattern", "", filePath, language, lineOf(src, m[0]))
		setProps(&ent, "framework", "kratos", "provenance", prov,
			"pattern_kind", "validation", "validation_kind", "rule",
			"validation_subtype", "protoc_gen_validate",
			"message_type", msg, "validate_method", method)
		add(ent)
	}

	for _, m := range reKratosValidateCall.FindAllStringSubmatchIndex(src, -1) {
		recv := submatch(src, m, 2)
		method := submatch(src, m, 4)
		// Skip the method-definition receiver lines (already emitted as rule);
		// those carry the `func (...)` prefix which the call regex won't anchor,
		// but a defensive dedup via the add() key handles any overlap.
		name := "validation:binding:" + method + ":" + recv
		ent := makeEntity(name, "SCOPE.Pattern", "", filePath, language, lineOf(src, m[0]))
		setProps(&ent, "framework", "kratos", "provenance", prov,
			"pattern_kind", "validation", "validation_kind", "binding",
			"validation_subtype", "validate_call",
			"receiver", recv, "validate_method", method)
		add(ent)
	}
}
