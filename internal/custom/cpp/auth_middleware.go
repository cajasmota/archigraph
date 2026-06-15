package cpp

// auth_middleware.go — auth + middleware scanner for C++ HTTP services.
//
// Depth target (TS/JS bar): capture the *specific* filter / interceptor /
// middleware name, the auth *method* (jwt | bearer | basic | api_key |
// session), and — where the framework expresses it inline — the middleware
// *order*. Names and methods are stamped as properties so downstream tools and
// the value-asserting fixtures can attribute auth to a concrete symbol rather
// than asserting len>0.
//
// Covered surfaces:
//
//   - auth_coverage:
//   - Drogon: `class X : public HttpFilter<X>` subclasses — captured as the
//     filter name; auth method classified from the class name (Jwt/Bearer/
//     Basic/ApiKey) and from jwt-cpp usage in the same file.
//   - oatpp: `BearerAuthorizationHandler` / `BasicAuthorizationHandler` /
//     `AuthorizationHandler` subclasses → auth method bearer/basic; the
//     handler class name is captured.
//   - Crow: middleware structs with auth keywords (before_handle) → captured
//     struct name; CROW_MIDDLEWARES registration list.
//   - jwt-cpp: `jwt::verify` / `jwt::decode` / `jwt::create` → method jwt.
//   - libjwt: `jwt_decode` / `jwt_add_grant` → method jwt.
//   - Generic: `Authorization: Bearer` header extraction → method bearer.
//   - api_key / session header & cookie patterns.
//
//   - middleware_coverage:
//   - Drogon: `void doFilter(const HttpRequestPtr&, ...)` filter body;
//     `registerFilter<X>()` registration (captures filter type X);
//     `FILTER_ADD(X)` per-route filter binding (captures filter X).
//   - oatpp: `RequestInterceptor` / `ResponseInterceptor` subclasses →
//     captured name + phase (request|response).
//   - Crow: `struct X { before_handle/after_handle }` middleware structs →
//     captured name + hook; `crow::App<Mw1, Mw2, ...>` template → ordered
//     middleware list (order captured as 0-based index).
//   - Pistache / cpprestsdk / poco / restbed: handler-chain markers.
//
// Honesty: partial — heuristic regex/substring match on source text. Captures
// concrete names/methods/order *within a file*, but does NOT resolve filter
// class definitions across files or bind a middleware to a specific route's
// runtime chain. Fixtures prove the detection + attribution surface.

import (
	"context"
	"regexp"
	"strconv"
	"strings"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/trace"

	"github.com/cajasmota/grafel/internal/extractor"
	"github.com/cajasmota/grafel/internal/types"
)

func init() {
	extractor.Register("custom_cpp_auth_middleware", &cppAuthMwExtractor{})
}

type cppAuthMwExtractor struct{}

func (e *cppAuthMwExtractor) Language() string { return "custom_cpp_auth_middleware" }

// ---------------------------------------------------------------------------
// Framework detection
// ---------------------------------------------------------------------------

var cppFrameworkMarkers = []struct {
	name string
	re   *regexp.Regexp
}{
	{"drogon", regexp.MustCompile(`(?:#include\s*<drogon/|drogon::|\bHttpFilter\b|\bDrogon\b)`)},
	{"oatpp", regexp.MustCompile(`(?:#include\s*<oatpp|oatpp::|\bDTO_FIELD\b|AuthorizationHandler|RequestInterceptor|ResponseInterceptor)`)},
	{"crow", regexp.MustCompile(`(?:#include\s*<crow|crow::|\bCROW_ROUTE\b|\bCROW_MIDDLEWARES\b)`)},
	{"pistache", regexp.MustCompile(`(?:#include\s*<pistache/|Pistache::|Routes::)`)},
	{"cpprestsdk", regexp.MustCompile(`(?:#include\s*<cpprest/|web::http::|http_listener)`)},
}

func detectCPPFramework(src string) string {
	for _, m := range cppFrameworkMarkers {
		if m.re.MatchString(src) {
			return m.name
		}
	}
	return ""
}

// ---------------------------------------------------------------------------
// Auth method classification
// ---------------------------------------------------------------------------

// cppClassifyAuthMethod infers the concrete auth method (jwt|bearer|basic|
// api_key|session) from a symbol name (filter / handler / struct). Returns ""
// when the name carries no auth signal so callers can fall back to context.
func cppClassifyAuthMethod(name string) string {
	l := strings.ToLower(name)
	switch {
	case strings.Contains(l, "jwt"):
		return "jwt"
	case strings.Contains(l, "bearer"):
		return "bearer"
	case strings.Contains(l, "basic"):
		return "basic"
	case strings.Contains(l, "apikey") || strings.Contains(l, "api_key"):
		return "api_key"
	case strings.Contains(l, "session") || strings.Contains(l, "cookie"):
		return "session"
	case strings.Contains(l, "oauth"):
		return "oauth"
	case strings.Contains(l, "token"):
		return "bearer"
	case strings.Contains(l, "auth"):
		return "auth"
	}
	return ""
}

// ---------------------------------------------------------------------------
// Name-capturing auth signals
// ---------------------------------------------------------------------------

var (
	// Drogon filter subclass: `class AuthFilter : public HttpFilter<AuthFilter>`
	reDrogonFilterClass = regexp.MustCompile(
		`(?:class|struct)\s+([A-Za-z_]\w*)\s*:\s*(?:public\s+)?(?:drogon::)?HttpFilter\b`)

	// oatpp authorization handler subclass:
	// `class BearerAuth : public oatpp::web::server::handler::BearerAuthorizationHandler`
	reOatppAuthHandler = regexp.MustCompile(
		`(?:class|struct)\s+([A-Za-z_]\w*)\s*:\s*(?:public\s+)?(?:[\w:]*::)?(Bearer|Basic|)AuthorizationHandler\b`)

	// jwt-cpp call sites: jwt::verify / jwt::decode / jwt::create
	reJwtCppCall = regexp.MustCompile(`\bjwt::(decode|create|verify)\s*\(`)

	// libjwt: jwt_decode / jwt_add_grant
	reLibjwtCall = regexp.MustCompile(`\b(jwt_decode|jwt_add_grant)\s*\(`)

	// Crow middleware struct carrying before_handle, captured name.
	reCrowMwStruct = regexp.MustCompile(
		`(?s)struct\s+([A-Za-z_]\w*)\s*\{.{0,400}?\bbefore_handle\s*\(`)

	// Generic Authorization: Bearer header extraction.
	reBearerHeader = regexp.MustCompile(`(?i)"Authorization".{0,40}?Bearer`)

	// api_key header / param.
	reApiKey = regexp.MustCompile(`(?i)"X-Api-Key"|"api[_-]?key"|\bapi_key\b`)

	// session / cookie auth.
	reSession = regexp.MustCompile(`(?i)\b(?:session_id|session_token|session_key|SessionManager|sessionCookie|set_session)\b`)
)

// ---------------------------------------------------------------------------
// Name-capturing middleware signals
// ---------------------------------------------------------------------------

var (
	// Drogon doFilter override.
	reDrogonDoFilter = regexp.MustCompile(
		`void\s+doFilter\s*\(\s*const\s+(?:drogon::)?HttpRequestPtr`)

	// Drogon registerFilter<X>() — captures the filter type X.
	reDrogonRegisterFilter = regexp.MustCompile(
		`\.registerFilter\s*<\s*([A-Za-z_]\w*(?:::[A-Za-z_]\w*)*)\s*>`)

	// Drogon FILTER_ADD(X) — per-controller route filter binding.
	reDrogonFilterAdd = regexp.MustCompile(
		`\bFILTER_ADD\s*\(\s*"?([A-Za-z_]\w*(?:::[A-Za-z_]\w*)*)"?\s*\)`)

	// oatpp request/response interceptor subclass.
	reOatppInterceptor = regexp.MustCompile(
		`(?:class|struct)\s+([A-Za-z_]\w*)\s*:\s*(?:public\s+)?(?:[\w:]*::)?(Request|Response)Interceptor\b`)

	// Crow before_handle / after_handle hooks.
	reCrowBeforeHandle = regexp.MustCompile(`\bbefore_handle\s*\(`)
	reCrowAfterHandle  = regexp.MustCompile(`\bafter_handle\s*\(`)

	// Crow App<Mw1, Mw2, ...> template — ordered middleware list.
	reCrowApp = regexp.MustCompile(`crow::App\s*<([^>]+)>`)

	// CROW_MIDDLEWARES(app, Mw1, Mw2, ...)
	reCrowMiddlewares = regexp.MustCompile(`\bCROW_MIDDLEWARES\s*\(\s*\w+\s*,\s*([^)]+)\)`)

	// Pistache handler chain class.
	rePistacheHandler = regexp.MustCompile(
		`(?:class|struct)\s+([A-Za-z_]\w*)\s*:\s*(?:public\s+)?(?:Pistache::)?(?:Http::)?Handler\b`)

	// cpprestsdk listener pipeline.
	reCpprestSupport = regexp.MustCompile(`\blistener\s*\.\s*support\s*\(`)

	// poco request handler factory.
	rePocoHandler = regexp.MustCompile(`\baddRequestHandler\s*\(`)

	// restbed filter handler.
	reRestbedFilter = regexp.MustCompile(`resource->set_(?:failed_)?filter_handler\s*\(`)
)

// splitTypeList splits a comma-separated template-arg / macro-arg list into
// trimmed, non-empty tokens (the leading template/whitespace and any trailing
// qualifiers are stripped to the bare type name).
func splitTypeList(raw string) []string {
	var out []string
	for _, p := range strings.Split(raw, ",") {
		p = strings.TrimSpace(p)
		if p == "" {
			continue
		}
		out = append(out, p)
	}
	return out
}

func (e *cppAuthMwExtractor) Extract(ctx context.Context, file extractor.FileInput) ([]types.EntityRecord, error) {
	tracer := otel.Tracer("grafel/custom/cpp")
	_, span := tracer.Start(ctx, "indexer.cpp_auth_mw_extractor.extract",
		trace.WithAttributes(
			attribute.String("language", file.Language),
			attribute.String("file_path", file.Path),
		),
	)
	defer span.End()

	if len(file.Content) == 0 {
		return nil, nil
	}
	if file.Language != "cpp" {
		return nil, nil
	}

	src := string(file.Content)
	framework := detectCPPFramework(src)

	// File-level JWT signal: if the file uses jwt-cpp/libjwt anywhere, auth
	// methods that don't otherwise classify default to jwt.
	fileHasJWT := reJwtCppCall.MatchString(src) || reLibjwtCall.MatchString(src)

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

	// emitAuth stamps a concrete auth entity carrying the captured symbol name,
	// auth method, and framework.
	emitAuth := func(name, symbol, method, fw string, line int) {
		if method == "" {
			if fileHasJWT {
				method = "jwt"
			} else {
				method = "auth"
			}
		}
		ent := makeEntity(name, "SCOPE.Pattern", "", file.Path, file.Language, line)
		setProps(&ent,
			"framework", fw,
			"provenance", "INFERRED_FROM_CPP_AUTH",
			"pattern_kind", "auth",
			"auth_subtype", method,
			"auth_method", method,
			"auth_symbol", symbol,
		)
		add(ent)
	}

	// emitMw stamps a concrete middleware entity carrying captured symbol name,
	// middleware kind, framework, and (optionally) order.
	emitMw := func(name, symbol, kind, fw string, order int, line int) {
		ent := makeEntity(name, "SCOPE.Pattern", "", file.Path, file.Language, line)
		props := []string{
			"framework", fw,
			"provenance", "INFERRED_FROM_CPP_MIDDLEWARE",
			"pattern_kind", "middleware",
			"middleware_kind", kind,
			"middleware_symbol", symbol,
		}
		if order >= 0 {
			props = append(props, "middleware_order", strconv.Itoa(order))
		}
		setProps(&ent, props...)
		add(ent)
	}

	// =====================================================================
	// AUTH
	// =====================================================================

	// Drogon HttpFilter subclasses — capture the filter class name.
	for _, m := range reDrogonFilterClass.FindAllStringSubmatchIndex(src, -1) {
		name := src[m[2]:m[3]]
		method := cppClassifyAuthMethod(name)
		emitAuth("auth:drogon_filter:"+name, name, method, "drogon", lineOf(src, m[0]))
	}

	// oatpp AuthorizationHandler subclasses — capture name + bearer/basic.
	for _, m := range reOatppAuthHandler.FindAllStringSubmatchIndex(src, -1) {
		name := src[m[2]:m[3]]
		flavor := src[m[4]:m[5]] // "Bearer" | "Basic" | ""
		method := strings.ToLower(flavor)
		if method == "" {
			method = cppClassifyAuthMethod(name)
		}
		emitAuth("auth:oatpp_authorization_handler:"+name, name, method, "oatpp", lineOf(src, m[0]))
	}

	// jwt-cpp call sites — concrete jwt operation.
	for _, m := range reJwtCppCall.FindAllStringSubmatchIndex(src, -1) {
		op := src[m[2]:m[3]] // decode | create | verify
		emitAuth("auth:jwt:jwt::"+op, "jwt::"+op, "jwt", framework, lineOf(src, m[0]))
	}

	// libjwt call sites.
	for _, m := range reLibjwtCall.FindAllStringSubmatchIndex(src, -1) {
		fn := src[m[2]:m[3]]
		emitAuth("auth:jwt:"+fn, fn, "jwt", framework, lineOf(src, m[0]))
	}

	// Crow middleware struct with before_handle and an auth-ish name.
	for _, m := range reCrowMwStruct.FindAllStringSubmatchIndex(src, -1) {
		name := src[m[2]:m[3]]
		if method := cppClassifyAuthMethod(name); method != "" {
			emitAuth("auth:crow_middleware:"+name, name, method, "crow", lineOf(src, m[0]))
		}
	}

	// Generic Authorization: Bearer header.
	if loc := reBearerHeader.FindStringIndex(src); loc != nil {
		emitAuth("auth:bearer:authorization_header", "Authorization", "bearer", framework, lineOf(src, loc[0]))
	}

	// api_key.
	if loc := reApiKey.FindStringIndex(src); loc != nil {
		emitAuth("auth:api_key:header", "api_key", "api_key", framework, lineOf(src, loc[0]))
	}

	// session / cookie auth.
	if loc := reSession.FindStringIndex(src); loc != nil {
		sym := strings.TrimSpace(src[loc[0]:loc[1]])
		emitAuth("auth:session:"+sym, sym, "session", framework, lineOf(src, loc[0]))
	}

	// =====================================================================
	// MIDDLEWARE
	// =====================================================================

	// Drogon doFilter body.
	if loc := reDrogonDoFilter.FindStringIndex(src); loc != nil {
		emitMw("middleware:drogon:doFilter", "doFilter", "doFilter", "drogon", -1, lineOf(src, loc[0]))
	}

	// Drogon registerFilter<X>() — capture filter type X (auth cross-emit too).
	for _, m := range reDrogonRegisterFilter.FindAllStringSubmatchIndex(src, -1) {
		x := src[m[2]:m[3]]
		emitMw("middleware:drogon:registerFilter:"+x, x, "registerFilter", "drogon", -1, lineOf(src, m[0]))
		if method := cppClassifyAuthMethod(x); method != "" {
			emitAuth("auth:drogon_filter:"+x, x, method, "drogon", lineOf(src, m[0]))
		}
	}

	// Drogon FILTER_ADD(X) — per-route filter binding.
	for _, m := range reDrogonFilterAdd.FindAllStringSubmatchIndex(src, -1) {
		x := src[m[2]:m[3]]
		emitMw("middleware:drogon:FILTER_ADD:"+x, x, "FILTER_ADD", "drogon", -1, lineOf(src, m[0]))
		if method := cppClassifyAuthMethod(x); method != "" {
			emitAuth("auth:drogon_filter:"+x, x, method, "drogon", lineOf(src, m[0]))
		}
	}

	// oatpp Request/Response interceptors — capture name + phase.
	for _, m := range reOatppInterceptor.FindAllStringSubmatchIndex(src, -1) {
		name := src[m[2]:m[3]]
		phase := strings.ToLower(src[m[4]:m[5]]) // request | response
		ent := makeEntity("middleware:oatpp_interceptor:"+name, "SCOPE.Pattern", "", file.Path, file.Language, lineOf(src, m[0]))
		setProps(&ent,
			"framework", "oatpp",
			"provenance", "INFERRED_FROM_CPP_MIDDLEWARE",
			"pattern_kind", "middleware",
			"middleware_kind", "interceptor",
			"middleware_symbol", name,
			"interceptor_phase", phase,
		)
		add(ent)
		if method := cppClassifyAuthMethod(name); method != "" {
			emitAuth("auth:oatpp_interceptor:"+name, name, method, "oatpp", lineOf(src, m[0]))
		}
	}

	// Crow middleware structs (any before/after handle struct, captured name).
	for _, m := range reCrowMwStruct.FindAllStringSubmatchIndex(src, -1) {
		name := src[m[2]:m[3]]
		emitMw("middleware:crow_struct:"+name, name, "crow_middleware", "crow", -1, lineOf(src, m[0]))
	}

	// Crow before_handle / after_handle hooks.
	if loc := reCrowBeforeHandle.FindStringIndex(src); loc != nil {
		emitMw("middleware:crow:before_handle", "before_handle", "before_handle", "crow", -1, lineOf(src, loc[0]))
	}
	if loc := reCrowAfterHandle.FindStringIndex(src); loc != nil {
		emitMw("middleware:crow:after_handle", "after_handle", "after_handle", "crow", -1, lineOf(src, loc[0]))
	}

	// Crow App<Mw1, Mw2, ...> — ordered middleware list (order = index).
	if m := reCrowApp.FindStringSubmatchIndex(src); m != nil {
		line := lineOf(src, m[0])
		for i, mw := range splitTypeList(src[m[2]:m[3]]) {
			emitMw("middleware:crow_app:"+mw, mw, "crow_app_template", "crow", i, line)
			if method := cppClassifyAuthMethod(mw); method != "" {
				emitAuth("auth:crow_middleware:"+mw, mw, method, "crow", line)
			}
		}
	}

	// CROW_MIDDLEWARES(app, Mw1, Mw2, ...) — ordered registration.
	if m := reCrowMiddlewares.FindStringSubmatchIndex(src); m != nil {
		line := lineOf(src, m[0])
		for i, mw := range splitTypeList(src[m[2]:m[3]]) {
			emitMw("middleware:crow_middlewares:"+mw, mw, "CROW_MIDDLEWARES", "crow", i, line)
			if method := cppClassifyAuthMethod(mw); method != "" {
				emitAuth("auth:crow_middleware:"+mw, mw, method, "crow", line)
			}
		}
	}

	// Pistache handler chain.
	for _, m := range rePistacheHandler.FindAllStringSubmatchIndex(src, -1) {
		name := src[m[2]:m[3]]
		emitMw("middleware:pistache_handler:"+name, name, "Handler", "pistache", -1, lineOf(src, m[0]))
	}

	// cpprestsdk listener.support pipeline.
	if loc := reCpprestSupport.FindStringIndex(src); loc != nil {
		emitMw("middleware:cpprestsdk:listener_support", "listener.support", "listener_support", "cpprestsdk", -1, lineOf(src, loc[0]))
	}

	// poco addRequestHandler.
	if loc := rePocoHandler.FindStringIndex(src); loc != nil {
		emitMw("middleware:poco:addRequestHandler", "addRequestHandler", "addRequestHandler", "poco", -1, lineOf(src, loc[0]))
	}

	// restbed filter handler.
	if loc := reRestbedFilter.FindStringIndex(src); loc != nil {
		emitMw("middleware:restbed:filter_handler", "set_filter_handler", "filter_handler", "restbed", -1, lineOf(src, loc[0]))
	}

	span.SetAttributes(
		attribute.String("framework", framework),
		attribute.Int("entity_count", len(entities)),
	)
	return entities, nil
}
