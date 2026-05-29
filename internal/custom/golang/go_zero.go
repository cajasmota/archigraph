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
	tracer := otel.Tracer("archigraph/custom/golang")
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
	// Two gates. The routing pass is gated on the generated routes-registration
	// signature (rest.Route / AddRoutes). The middleware/auth/validation passes
	// model the hand-written server-bootstrap + logic-layer wiring, so they run
	// on any file carrying a go-zero marker. A file with neither emits nothing.
	hasRoutes := strings.Contains(src, "rest.Route") || strings.Contains(src, "AddRoutes")
	hasGoZeroMarker := strings.Contains(src, "zeromicro/go-zero") ||
		strings.Contains(src, "rest.With") ||
		strings.Contains(src, "httpx.Parse")
	if !hasRoutes && !hasGoZeroMarker {
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

	// middleware / auth / request_validation surfaces (issue #3255). Run on
	// every go-zero file (the wiring may live apart from the generated routes).
	e.extractMiddlewareAuth(src, file, add)
	e.extractRequestValidation(src, file, add)

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

	span.SetAttributes(attribute.Int("entity_count", len(entities)))
	return entities, nil
}

// ---------------------------------------------------------------------------
// Middleware + auth + request validation (issue #3255).
//
// go-zero's routing extractor above models the generated handler/routes.go.
// The three capabilities below model the surrounding go-zero wiring, which the
// routing pass does not touch:
//
//   - middleware_coverage : middleware is registered server-wide via the
//       rest.WithMiddleware(mw, …) server option, and per-route via
//       server.Use(mw) / a Middleware field on a rest.Route. Each middleware
//       expression is one ordered SCOPE.Pattern (pattern_kind=middleware).
//   - auth_coverage      : go-zero ships first-class JWT auth via the
//       rest.WithJwt(secret) route option (and the Authorization config block);
//       any middleware or WithJwt option that classifies as auth is flagged
//       is_auth + auth_kind and re-emitted as a dedicated auth SCOPE.Pattern.
//   - request_validation : request DTOs carry `validate:"…"` struct tags and an
//       optional generated `func (r *Req) Validate() error`; the handler binds
//       + validates them via httpx.Parse(r, &req). Both the tag rules and the
//       httpx.Parse binding call site are validation surfaces.
//
// Honesty: heuristic identifier/substring/tag matches on source text with no
// data-flow proof, so each capability is reported `partial`. go-zero genuinely
// has all three concepts (not NA); the proving fixture exercises each.
// ---------------------------------------------------------------------------

var (
	// rest.WithMiddleware( server-option call — balanced args hold a middleware
	// chain. server.Use( / engine.Use( per-server middleware registration.
	reGoZeroMWHead = regexp.MustCompile(`rest\.WithMiddleware\s*\(|\b\w+\.Use\s*\(`)
	// rest.WithJwt("secret") — go-zero's first-class JWT route option.
	reGoZeroWithJwt = regexp.MustCompile(`rest\.WithJwt\s*\(`)
	// validate:"..." struct-field tags (go-zero embeds go-playground/validator).
	reGoZeroValidateField = regexp.MustCompile(
		"(?m)^\\s*(\\w+)\\s+[^`\\n]*`[^`]*\\bvalidate:\"([^\"]+)\"[^`]*`")
	// httpx.Parse(r, &req) — the go-zero request bind+validate call site.
	reGoZeroHttpxParse = regexp.MustCompile(`httpx\.Parse(?:JsonBody|Form|Header|Path)?\s*\(`)
)

// goZeroClassifyAuth — file-local auth classifier (shared helpers untouched).
func goZeroClassifyAuth(expr string) string {
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

var reGoZeroCallHead = regexp.MustCompile(`^[A-Za-z_][\w.]*`)

// extractMiddlewareAuth scans rest.WithMiddleware(...) / .Use(...) chains and
// rest.WithJwt(...) options, emitting ordered middleware SCOPE.Pattern entities
// and dedicated auth SCOPE.Pattern entities for auth-classified middleware.
func (e *goZeroExtractor) extractMiddlewareAuth(src string, file extractor.FileInput, add func(types.EntityRecord)) {
	addAuth := func(name, expr string, line int) {
		au := makeEntity("auth:"+name, "SCOPE.Pattern", "", file.Path, file.Language, line)
		setProps(&au, "framework", "go_zero",
			"provenance", "INFERRED_FROM_GOZERO_AUTH",
			"pattern_kind", "auth",
			"auth_kind", goZeroClassifyAuth(expr),
			"middleware_name", name,
			"middleware_expr", expr)
		add(au)
	}

	for _, head := range reGoZeroMWHead.FindAllStringIndex(src, -1) {
		open := head[1] - 1
		args, end := balancedArgs(src, open)
		if end < 0 {
			continue
		}
		line := lineOf(src, head[0])
		order := 0
		for _, part := range splitTopLevelArgs(args) {
			if part == "" || isStringLiteral(part) {
				continue
			}
			name := reGoZeroCallHead.FindString(part)
			if name == "" {
				name = part
			}
			mw := makeEntity(part, "SCOPE.Pattern", "", file.Path, file.Language, line)
			setProps(&mw, "framework", "go_zero",
				"provenance", "INFERRED_FROM_GOZERO_MIDDLEWARE",
				"pattern_kind", "middleware",
				"middleware_name", name,
				"mw_order", itoa(order))
			if ak := goZeroClassifyAuth(part); ak != "" {
				setProps(&mw, "is_auth", "true", "auth_kind", ak)
				add(mw)
				addAuth(name, part, line)
			} else {
				add(mw)
			}
			order++
		}
	}

	// rest.WithJwt(secret) is a first-class JWT enforcement option, not a
	// middleware-chain entry — emit it directly as an auth SCOPE.Pattern.
	for _, m := range reGoZeroWithJwt.FindAllStringIndex(src, -1) {
		line := lineOf(src, m[0])
		mw := makeEntity("rest.WithJwt", "SCOPE.Pattern", "", file.Path, file.Language, line)
		setProps(&mw, "framework", "go_zero",
			"provenance", "INFERRED_FROM_GOZERO_MIDDLEWARE",
			"pattern_kind", "middleware",
			"middleware_name", "rest.WithJwt",
			"is_auth", "true", "auth_kind", "jwt")
		add(mw)
		addAuth("rest.WithJwt", "rest.WithJwt(...)", line)
	}
}

// extractRequestValidation emits a validation SCOPE.Pattern per `validate:`
// struct-tag field and per httpx.Parse(...) bind call site.
func (e *goZeroExtractor) extractRequestValidation(src string, file extractor.FileInput, add func(types.EntityRecord)) {
	// Index struct heads so each tagged field is attributed to its struct.
	type head struct {
		name string
		off  int
	}
	var heads []head
	for _, m := range reGoStructHead.FindAllStringSubmatchIndex(src, -1) {
		heads = append(heads, head{name: src[m[2]:m[3]], off: m[0]})
	}
	structAt := func(off int) string {
		name := ""
		for _, h := range heads {
			if h.off <= off {
				name = h.name
			} else {
				break
			}
		}
		return name
	}

	for _, m := range reGoZeroValidateField.FindAllStringSubmatchIndex(src, -1) {
		field := src[m[2]:m[3]]
		rules := src[m[4]:m[5]]
		if rules == "" || rules == "-" {
			continue
		}
		st := structAt(m[0])
		ent := makeEntity("validation:rule:"+st+"."+field, "SCOPE.Pattern", "", file.Path, file.Language, lineOf(src, m[0]))
		setProps(&ent, "framework", "go_zero",
			"provenance", "INFERRED_FROM_GOZERO_VALIDATION",
			"pattern_kind", "validation",
			"validation_kind", "rule",
			"struct_name", st,
			"field_name", field,
			"rules", rules,
			"rule_source", "validate")
		add(ent)
	}

	for _, m := range reGoZeroHttpxParse.FindAllStringIndex(src, -1) {
		ent := makeEntity("validation:binding:bind_call", "SCOPE.Pattern", "", file.Path, file.Language, lineOf(src, m[0]))
		setProps(&ent, "framework", "go_zero",
			"provenance", "INFERRED_FROM_GOZERO_VALIDATION",
			"pattern_kind", "validation",
			"validation_kind", "binding",
			"validation_subtype", "httpx_parse")
		add(ent)
	}
}
