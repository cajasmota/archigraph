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
	extractor.Register("custom_go_go_zero", &goZeroExtractor{})
}

// goZeroExtractor extracts routing structure from go-zero
// (github.com/zeromicro/go-zero) REST services. go-zero is codegen-driven: the
// `goctl` tool generates `internal/handler/routes.go` from an `.api`
// descriptor. The generated file registers routes by passing
// `[]rest.Route{...}` slices to `server.AddRoutes(...)` —
//
//	server.AddRoutes(
//		[]rest.Route{
//			{
//				Method:  http.MethodGet,
//				Path:    "/users/:id",
//				Handler: user.GetUserHandler(serverCtx),
//			},
//		},
//		rest.WithPrefix("/api/v1"),
//	)
//
// Each `rest.Route{Method, Path, Handler}` struct literal yields an endpoint
// (Method + Path) with the Handler expression attributed as the handler. A
// `rest.WithPrefix("/p")` option on the same AddRoutes call prefixes every
// route in that group.
//
// Honesty note: this targets the *generated* `routes.go` output (the committed
// goctl artifact), which is a stable statically-analysable struct-literal
// shape — the proving fixture exercises exactly this. When only the `.api`
// descriptor is present and `routes.go` has not been generated, there are no
// `rest.Route` registration sites to detect; that is an inherent limit of the
// descriptor-only layout, not a heuristic gap.
type goZeroExtractor struct{}

func (e *goZeroExtractor) Language() string { return "custom_go_go_zero" }

var (
	// server.AddRoutes( — start token. The balanced argument span is scanned
	// forward so each []rest.Route{...} slice (with nested braces) and any
	// trailing rest.WithPrefix(...) option are captured whole.
	reGoZeroAddRoutesHead = regexp.MustCompile(`(\w+)\.AddRoutes\s*\(`)
	// Method field of a rest.Route literal: Method: http.MethodGet | "GET".
	reGoZeroMethodField = regexp.MustCompile(
		`Method\s*:\s*(?:http\.Method(\w+)|"([A-Za-z]+)")`,
	)
	// Path field of a rest.Route literal: Path: "/users/:id".
	reGoZeroPathField = regexp.MustCompile(`Path\s*:\s*"([^"]+)"`)
	// Handler field of a rest.Route literal: Handler: user.GetUserHandler(ctx).
	// Captures the leading identifier/selector before the call parens.
	reGoZeroHandlerField = regexp.MustCompile(
		`Handler\s*:\s*([A-Za-z_][\w.]*)`,
	)
	// rest.WithPrefix("/api/v1") — group prefix option on an AddRoutes call.
	reGoZeroWithPrefix = regexp.MustCompile(`rest\.WithPrefix\s*\(\s*"([^"]+)"`)

	// --- middleware (#3255) -------------------------------------------------
	// rest.WithMiddleware(mw, routes…) / rest.WithMiddlewares([]rest.Middleware{…}, …)
	// — the goctl/go-zero idiom for wrapping a route group in a middleware. The
	// first balanced argument is the middleware expression.
	reGoZeroWithMiddleware = regexp.MustCompile(`rest\.WithMiddlewares?\s*\(`)
	// server.Use(mw) — global middleware registration on a rest.Server.
	reGoZeroUse = regexp.MustCompile(`(\w+)\.Use\s*\(`)

	// --- auth (#3255) -------------------------------------------------------
	// rest.WithJwt(secret) / rest.WithJwtTransition(secret, prevSecret) — the
	// built-in go-zero JWT auth option attached to a route group in the
	// generated routes.go. Marks the group as JWT-protected.
	reGoZeroWithJwt = regexp.MustCompile(`rest\.WithJwt(?:Transition)?\s*\(`)

	// --- request validation (#3255) ----------------------------------------
	// httpx.Parse(r, &req) / httpx.ParseJsonBody(r, &req) — the go-zero request
	// binding call that triggers validation of the typed request struct (its
	// `validate:` / go-zero `,optional`/range tags). Captures the bound var.
	reGoZeroHttpxParse = regexp.MustCompile(
		`httpx\.Parse(?:JsonBody|Form|Header|Path)?\s*\(\s*[\w.]+\s*,\s*&(\w+)`,
	)
)

// goZeroVerb resolves the HTTP verb from a Method-field match, normalising both
// the http.Method<Verb> constant form and the bare string-literal form.
func goZeroVerb(src string, m []int) string {
	if v := submatch(src, m, 2); v != "" { // http.MethodGet -> GET
		return strings.ToUpper(v)
	}
	if v := submatch(src, m, 4); v != "" { // "GET"
		return strings.ToUpper(v)
	}
	return ""
}

// leafBraceBlocks returns the text inside every innermost (leaf) `{...}` block
// in s — i.e. brace pairs that contain no nested brace pair. For a go-zero
// `[]rest.Route{ {Method:…, Path:…}, {…} }` argument this yields one entry per
// individual route struct literal, ignoring the enclosing slice braces. Quoted
// strings are skipped so braces inside string literals do not affect nesting.
func leafBraceBlocks(s string) []string {
	var blocks []string
	var stack []int // start indices (after '{') of currently-open blocks
	hasChild := map[int]bool{}
	var quote rune
	for i := 0; i < len(s); i++ {
		r := rune(s[i])
		if quote != 0 {
			if r == quote {
				quote = 0
			}
			continue
		}
		switch r {
		case '"', '\'', '`':
			quote = r
		case '{':
			if n := len(stack); n > 0 {
				hasChild[stack[n-1]] = true
			}
			stack = append(stack, i+1)
		case '}':
			if n := len(stack); n > 0 {
				start := stack[n-1]
				stack = stack[:n-1]
				if !hasChild[start] {
					blocks = append(blocks, s[start:i])
				}
				delete(hasChild, start)
			}
		}
	}
	return blocks
}

func (e *goZeroExtractor) Extract(ctx context.Context, file extractor.FileInput) ([]types.EntityRecord, error) {
	tracer := otel.Tracer("grafel/custom/golang")
	_, span := tracer.Start(ctx, "indexer.go_zero_extractor.extract",
		trace.WithAttributes(
			attribute.String("language", file.Language),
			attribute.String("framework", "go_zero"),
			attribute.String("file_path", file.Path),
		),
	)
	defer span.End()

	if len(file.Content) == 0 || file.Language != "go" {
		return nil, nil
	}

	src := string(file.Content)
	// Routing is gated on the generated routes-registration signature; the
	// middleware/auth/validation passes run on any go-zero file (gated on the
	// broader zeromicro/go-zero import or rest/httpx markers) so they also cover
	// the handler/logic files where httpx.Parse(...) binds request structs and
	// the types file where `validate:` tags live.
	hasRouting := strings.Contains(src, "rest.Route") || strings.Contains(src, "AddRoutes")
	if !hasRouting && !isGoZeroFile(src) {
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
		emitGoZeroMiddlewareAndAuth(add, src, file.Path, file.Language)
		emitGoZeroValidation(add, src, file.Path, file.Language)
		span.SetAttributes(attribute.Int("entity_count", len(entities)))
		return entities, nil
	}

	for _, loc := range reGoZeroAddRoutesHead.FindAllStringSubmatchIndex(src, -1) {
		serverVar := submatch(src, loc, 2)
		open := loc[1] - 1 // index of the '(' that the head ends at
		args, end := balancedArgs(src, open)
		if end < 0 {
			continue // unbalanced; skip
		}
		callLine := lineOf(src, loc[0])

		// Group-level server -> SCOPE.Service.
		svc := makeEntity(serverVar, "SCOPE.Service", "", file.Path, file.Language, callLine)
		setProps(&svc, "framework", "go_zero", "provenance", "INFERRED_FROM_GOZERO_SERVER",
			"server_var", serverVar)
		add(svc)

		// Optional rest.WithPrefix(...) applies to every route in this call.
		prefix := ""
		if pm := reGoZeroWithPrefix.FindStringSubmatch(args); pm != nil {
			prefix = pm[1]
			pent := makeEntity(prefix, "SCOPE.Component", "", file.Path, file.Language, callLine)
			setProps(&pent, "framework", "go_zero", "provenance", "INFERRED_FROM_GOZERO_PREFIX",
				"group_path", prefix)
			add(pent)
		}

		// Each rest.Route{...} literal in the slice is one endpoint. The route
		// literals are nested inside the []rest.Route{ ... } slice braces, so
		// scan the argument text for the innermost (leaf) brace blocks — those
		// are the individual struct literals — and parse each whose fields
		// include Method + Path.
		for _, lit := range leafBraceBlocks(args) {
			mM := reGoZeroMethodField.FindStringSubmatchIndex(lit)
			verb := goZeroVerb(lit, mM)
			pathM := reGoZeroPathField.FindStringSubmatch(lit)
			if verb == "" || pathM == nil {
				continue // not a complete route literal
			}
			path := pathM[1]
			if prefix != "" {
				path = prefix + path
			}
			name := verb + " " + path
			ent := makeEntity(name, "SCOPE.Operation", "endpoint", file.Path, file.Language, callLine)
			setProps(&ent, "framework", "go_zero", "provenance", "INFERRED_FROM_GOZERO_ROUTE",
				"http_method", verb, "route_path", path)
			if hM := reGoZeroHandlerField.FindStringSubmatch(lit); hM != nil {
				ent.Properties["handler"] = hM[1]
			}
			add(ent)
		}
	}

	// middleware / auth / request-validation surfaces (#3255) — also run on the
	// routes.go file, where rest.WithMiddleware(...)/rest.WithJwt(...) options
	// commonly sit alongside the AddRoutes(...) registrations.
	emitGoZeroMiddlewareAndAuth(add, src, file.Path, file.Language)
	emitGoZeroValidation(add, src, file.Path, file.Language)

	span.SetAttributes(attribute.Int("entity_count", len(entities)))
	return entities, nil
}

// isGoZeroFile reports whether src looks like a go-zero source file — used to
// gate the middleware/auth/validation passes on non-routing go-zero files
// (handler/logic/types). Markers: the zeromicro import path and the rest/httpx
// packages go-zero services are built on.
func isGoZeroFile(src string) bool {
	return strings.Contains(src, "zeromicro/go-zero") ||
		strings.Contains(src, "go-zero/rest") ||
		strings.Contains(src, "httpx.Parse")
}

// emitGoZeroMiddlewareAndAuth detects go-zero middleware registration + JWT auth.
//
//	rest.WithMiddleware(mw, …) / rest.WithMiddlewares([]rest.Middleware{…}, …)
//	server.Use(mw)                                  — middleware
//	rest.WithJwt(secret) / rest.WithJwtTransition   — built-in JWT auth option
//
// Each middleware expression becomes a SCOPE.Pattern (pattern_kind=middleware),
// auth-classified via the shared catalog; rest.WithJwt yields a dedicated auth
// SCOPE.Pattern (auth_kind=jwt) regardless of the wrapped expression, since the
// option itself is the auth enforcement point.
//
// Honesty: heuristic source match; no data-flow proof of enforcement or
// route-binding. Reported `partial`.
func emitGoZeroMiddlewareAndAuth(add func(types.EntityRecord), src, filePath, language string) {
	const mwProv = "INFERRED_FROM_GOZERO_MIDDLEWARE"
	const authProv = "INFERRED_FROM_GOZERO_AUTH"

	emitChain := func(headRe *regexp.Regexp, form string) {
		for _, loc := range headRe.FindAllStringSubmatchIndex(src, -1) {
			open := loc[1] - 1
			args, end := balancedArgs(src, open)
			if end < 0 {
				continue
			}
			chain := parseMiddlewareChain(args)
			line := lineOf(src, loc[0])
			for _, a := range chain {
				mw := makeEntity(a.Expr, "SCOPE.Pattern", "", filePath, language, line)
				setProps(&mw, "framework", "go_zero", "provenance", mwProv,
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
					setProps(&au, "framework", "go_zero", "provenance", authProv,
						"pattern_kind", "auth", "auth_kind", a.AuthKind,
						"middleware_name", a.Name, "middleware_expr", a.Expr)
					add(au)
				}
			}
		}
	}

	emitChain(reGoZeroWithMiddleware, "with_middleware")
	emitChain(reGoZeroUse, "use")

	// rest.WithJwt(...) — the built-in JWT auth option. One auth SCOPE.Pattern
	// per occurrence (the option IS the enforcement point).
	for _, loc := range reGoZeroWithJwt.FindAllStringIndex(src, -1) {
		line := lineOf(src, loc[0])
		au := makeEntity("auth:rest.WithJwt", "SCOPE.Pattern", "", filePath, language, line)
		setProps(&au, "framework", "go_zero", "provenance", authProv,
			"pattern_kind", "auth", "auth_kind", "jwt",
			"middleware_name", "rest.WithJwt", "auth_form", "with_jwt")
		add(au)
	}
}

// emitGoZeroValidation detects go-zero request validation.
//
//	rule    — struct fields carrying `validate:` (go-playground) tags on a
//	          go-zero typed request struct
//	binding — httpx.Parse(r, &req) / httpx.ParseJsonBody(...) call sites that
//	          decode-and-validate the request struct
//
// go-zero's httpx.Parse runs the field validators after JSON decoding, so the
// Parse call is the validation-enforcement surface. Struct-tag rules reuse the
// shared findValidationRules scanner (validate.go), keeping the property shape
// identical to the gin/echo/fiber/chi validation cells.
//
// Honesty: heuristic source match; no proof every rule fires on a real request
// path. Reported `partial`.
func emitGoZeroValidation(add func(types.EntityRecord), src, filePath, language string) {
	const prov = "INFERRED_FROM_GOZERO_VALIDATION"

	for _, r := range findValidationRules(src) {
		name := "validation:rule:" + r.Struct + "." + r.Field
		ent := makeEntity(name, "SCOPE.Pattern", "", filePath, language, r.Line)
		setProps(&ent, "framework", "go_zero", "provenance", prov,
			"pattern_kind", "validation", "validation_kind", "rule",
			"struct_name", r.Struct, "field_name", r.Field,
			"rules", r.Rules, "rule_source", r.Source)
		add(ent)
	}

	for _, m := range reGoZeroHttpxParse.FindAllStringSubmatchIndex(src, -1) {
		boundVar := submatch(src, m, 2)
		name := "validation:binding:httpx_parse:" + boundVar
		ent := makeEntity(name, "SCOPE.Pattern", "", filePath, language, lineOf(src, m[0]))
		setProps(&ent, "framework", "go_zero", "provenance", prov,
			"pattern_kind", "validation", "validation_kind", "binding",
			"validation_subtype", "httpx_parse", "bound_var", boundVar)
		add(ent)
	}
}
