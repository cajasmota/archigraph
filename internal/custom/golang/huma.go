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
	extractor.Register("custom_go_huma", &humaExtractor{})
}

// humaExtractor extracts routing structure from Huma
// (github.com/danielgtaylor/huma) servers. Huma is OpenAPI-first: routes are
// declared by registering an Operation against an API value —
//
//	huma.Register(api, huma.Operation{Method: "GET", Path: "/users/{id}"}, handler)
//
// The Method + Path fields of the Operation literal yield an endpoint, and the
// final argument of huma.Register is the handler (handler attribution). Both
// v1 (danielgtaylor/huma) and v2 (danielgtaylor/huma/v2) share the same
// huma.Register entry point and Operation struct shape.
type humaExtractor struct{}

func (e *humaExtractor) Language() string { return "custom_go_huma" }

var (
	// huma.Register( — start token; the balanced argument span is scanned
	// forward so the Operation struct literal (with its own braces/commas)
	// is captured whole.
	reHumaRegisterHead = regexp.MustCompile(`huma\s*\.\s*Register\s*\(`)
	// Method field of an Operation literal: Method: http.MethodGet | "POST".
	reHumaMethodField = regexp.MustCompile(
		`Method\s*:\s*(?:http\.Method(\w+)|"([A-Za-z]+)")`,
	)
	// Path field of an Operation literal: Path: "/users/{id}".
	reHumaPathField = regexp.MustCompile(`Path\s*:\s*"([^"]+)"`)

	// --- middleware (#3255) -------------------------------------------------
	// api.UseMiddleware(mw1, mw2, …) — huma's API-level middleware registration.
	// The balanced argument span is captured so each middleware is one entry.
	reHumaUseMiddleware = regexp.MustCompile(`(\w+)\.UseMiddleware\s*\(`)
	// huma.Middlewares{mw1, mw2} — the Operation.Middlewares slice literal form
	// (per-operation middleware). Captured by balanced brace scan.
	reHumaMiddlewares = regexp.MustCompile(`huma\.Middlewares\s*\{`)

	// --- auth (#3255) -------------------------------------------------------
	// Security field of an Operation literal:
	//   Security: []map[string][]string{{"bearer": {"read"}}}
	// Marks the operation as auth-protected. We detect the presence of the
	// Security key and capture the first scheme name referenced.
	reHumaOpSecurity = regexp.MustCompile(`Security\s*:\s*\[\]map\[string\]\[\]string\{`)
	// SecuritySchemes registration on the OpenAPI config / components:
	//   SecuritySchemes: map[string]*huma.SecurityScheme{"bearer": {Type:"http", Scheme:"bearer"}}
	// and the scheme-name keys inside it.
	reHumaSecuritySchemes = regexp.MustCompile(`SecuritySchemes\s*[:=]\s*map\[string\]\*?huma\.SecurityScheme\{`)
	// A "name": { ... } scheme entry key inside a SecuritySchemes / Security map.
	reHumaSchemeKey = regexp.MustCompile(`"([A-Za-z_][\w-]*)"\s*:`)

	// --- request validation (#3255) ----------------------------------------
	// huma derives request validation from the input struct's schema tags
	// (minimum/maxLength/pattern/enum/format/required) declared on fields of the
	// *Input struct bound as the handler's second parameter. Each field carrying
	// such a tag is one validation rule. The field name + the constraint tag are
	// captured.
	reHumaSchemaField = regexp.MustCompile(
		"(?m)^\\s*(\\w+)\\s+[^`\\n]*`[^`]*?\\b(minimum|maximum|minLength|maxLength|pattern|enum|format|required|exclusiveMinimum|exclusiveMaximum|multipleOf|minItems|maxItems|uniqueItems):\"([^\"]*)\"[^`]*`")
)

// humaVerb resolves the HTTP verb from a Method-field match, normalising both
// the http.Method<Verb> constant form and the bare string-literal form.
func humaVerb(src string, m []int) string {
	if v := submatch(src, m, 2); v != "" { // http.MethodGet -> GET
		return strings.ToUpper(v)
	}
	if v := submatch(src, m, 4); v != "" { // "POST"
		return strings.ToUpper(v)
	}
	return ""
}

func (e *humaExtractor) Extract(ctx context.Context, file extractor.FileInput) ([]types.EntityRecord, error) {
	tracer := otel.Tracer("grafel/custom/golang")
	_, span := tracer.Start(ctx, "indexer.huma_extractor.extract",
		trace.WithAttributes(
			attribute.String("language", file.Language),
			attribute.String("framework", "huma"),
			attribute.String("file_path", file.Path),
		),
	)
	defer span.End()

	if len(file.Content) == 0 || file.Language != "go" {
		return nil, nil
	}

	src := string(file.Content)
	if !strings.Contains(src, "danielgtaylor/huma") && !strings.Contains(src, "huma.Register") {
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

	for _, loc := range reHumaRegisterHead.FindAllStringIndex(src, -1) {
		open := loc[1] - 1 // index of the '(' that reHumaRegisterHead ends at
		args, end := balancedArgs(src, open)
		if end < 0 {
			continue // unbalanced; skip
		}
		parts := splitTopLevelArgs(args)
		if len(parts) < 3 {
			continue // need (api, Operation{...}, handler)
		}
		opLit := parts[1]
		handler := strings.TrimSpace(parts[len(parts)-1])

		verb := humaVerb(opLit, reHumaMethodField.FindStringSubmatchIndex(opLit))
		pathM := reHumaPathField.FindStringSubmatch(opLit)
		if verb == "" || pathM == nil {
			continue // incomplete Operation — would fail at huma runtime too
		}
		path := pathM[1]
		line := lineOf(src, loc[0])

		name := verb + " " + path
		ent := makeEntity(name, "SCOPE.Operation", "endpoint", file.Path, file.Language, line)
		setProps(&ent, "framework", "huma", "provenance", "INFERRED_FROM_HUMA_OPERATION",
			"http_method", verb, "route_path", path)
		if handler != "" {
			ent.Properties["handler"] = handler
		}
		add(ent)

		// Per-operation Security: an Operation literal carrying a Security field
		// is auth-protected. Emit a dedicated auth SCOPE.Pattern bound to this
		// endpoint, capturing the first referenced scheme name.
		if reHumaOpSecurity.MatchString(opLit) {
			scheme := ""
			if km := reHumaSchemeKey.FindStringSubmatch(
				opLit[strings.Index(opLit, "Security"):]); km != nil {
				scheme = km[1]
			}
			au := makeEntity("auth:"+verb+" "+path, "SCOPE.Pattern", "", file.Path, file.Language, line)
			setProps(&au, "framework", "huma", "provenance", "INFERRED_FROM_HUMA_AUTH",
				"pattern_kind", "auth", "auth_kind", humaAuthKind(scheme),
				"route_path", path, "http_method", verb, "security_scheme", scheme)
			add(au)
		}
	}

	emitHumaMiddleware(add, src, file.Path, file.Language)
	emitHumaSecuritySchemes(add, src, file.Path, file.Language)
	emitHumaValidation(add, src, file.Path, file.Language)

	span.SetAttributes(attribute.Int("entity_count", len(entities)))
	return entities, nil
}

// humaAuthKind maps a huma security-scheme name to a coarse auth kind via the
// shared classifier, falling back to a name-shape heuristic and finally to the
// generic "auth" kind so a Security-protected operation is never left unkinded.
func humaAuthKind(scheme string) string {
	if k := classifyAuthMiddleware(scheme); k != "" {
		return k
	}
	low := strings.ToLower(scheme)
	switch {
	case strings.Contains(low, "bearer") || strings.Contains(low, "jwt"):
		return "jwt"
	case strings.Contains(low, "oauth"):
		return "oauth"
	case strings.Contains(low, "apikey") || strings.Contains(low, "api_key") || strings.Contains(low, "key"):
		return "api_key"
	case strings.Contains(low, "basic"):
		return "basic"
	default:
		return "auth"
	}
}

// emitHumaMiddleware detects huma middleware registration:
//
//	api.UseMiddleware(mw1, mw2, …)   — API-level middleware chain
//	huma.Middlewares{mw1, mw2}       — per-operation Middlewares slice literal
//
// Each middleware expression becomes an ordered SCOPE.Pattern
// (pattern_kind=middleware), auth-classified via the shared catalog.
//
// Honesty: heuristic source match, no enforcement proof. Reported `partial`.
func emitHumaMiddleware(add func(types.EntityRecord), src, filePath, language string) {
	const mwProv = "INFERRED_FROM_HUMA_MIDDLEWARE"
	const authProv = "INFERRED_FROM_HUMA_AUTH"

	emit := func(chain []middlewareArg, form string, line int) {
		for _, a := range chain {
			mw := makeEntity(a.Expr, "SCOPE.Pattern", "", filePath, language, line)
			setProps(&mw, "framework", "huma", "provenance", mwProv,
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
				setProps(&au, "framework", "huma", "provenance", authProv,
					"pattern_kind", "auth", "auth_kind", a.AuthKind,
					"middleware_name", a.Name, "middleware_expr", a.Expr)
				add(au)
			}
		}
	}

	// api.UseMiddleware(...) — balanced paren args.
	for _, loc := range reHumaUseMiddleware.FindAllStringSubmatchIndex(src, -1) {
		open := loc[1] - 1
		args, end := balancedArgs(src, open)
		if end < 0 {
			continue
		}
		emit(parseMiddlewareChain(args), "use_middleware", lineOf(src, loc[0]))
	}

	// huma.Middlewares{...} — balanced brace literal.
	for _, loc := range reHumaMiddlewares.FindAllStringIndex(src, -1) {
		open := loc[1] - 1 // index of the '{'
		end := matchBrace(src, open)
		if end < 0 {
			continue
		}
		emit(parseMiddlewareChain(src[open+1:end]), "middlewares_slice", lineOf(src, loc[0]))
	}
}

// emitHumaSecuritySchemes detects the OpenAPI SecuritySchemes registration on
// the huma config/components and emits one auth SCOPE.Pattern per declared
// scheme name (subtype scheme), describing the available auth schemes.
func emitHumaSecuritySchemes(add func(types.EntityRecord), src, filePath, language string) {
	const authProv = "INFERRED_FROM_HUMA_AUTH"
	for _, loc := range reHumaSecuritySchemes.FindAllStringIndex(src, -1) {
		open := loc[1] - 1 // the '{'
		end := matchBrace(src, open)
		if end < 0 {
			continue
		}
		body := src[open+1 : end]
		line := lineOf(src, loc[0])
		for _, km := range reHumaSchemeKey.FindAllStringSubmatch(body, -1) {
			scheme := km[1]
			au := makeEntity("auth:scheme:"+scheme, "SCOPE.Pattern", "", filePath, language, line)
			setProps(&au, "framework", "huma", "provenance", authProv,
				"pattern_kind", "auth", "auth_kind", humaAuthKind(scheme),
				"auth_form", "security_scheme", "security_scheme", scheme)
			add(au)
		}
	}
}

// emitHumaValidation detects huma request validation derived from OpenAPI schema
// tags on input-struct fields (minimum/maxLength/pattern/enum/format/required/…).
// Each constrained field is one validation rule attributed to its enclosing
// struct (resolved via the shared findValidationRules struct-head index logic,
// reimplemented here against the huma constraint-tag set).
//
// Honesty: heuristic source match; no proof the struct is actually bound as a
// handler input. Reported `partial`.
func emitHumaValidation(add func(types.EntityRecord), src, filePath, language string) {
	const prov = "INFERRED_FROM_HUMA_VALIDATION"

	// Index struct heads by offset so each constrained field finds its struct.
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

	for _, m := range reHumaSchemaField.FindAllStringSubmatchIndex(src, -1) {
		field := submatch(src, m, 2)
		tag := submatch(src, m, 4)
		value := submatch(src, m, 6)
		st := structAt(m[0])
		name := "validation:rule:" + st + "." + field + ":" + tag
		ent := makeEntity(name, "SCOPE.Pattern", "", filePath, language, lineOf(src, m[0]))
		setProps(&ent, "framework", "huma", "provenance", prov,
			"pattern_kind", "validation", "validation_kind", "rule",
			"validation_subtype", "openapi_schema_tag",
			"struct_name", st, "field_name", field,
			"constraint", tag, "constraint_value", value)
		add(ent)
	}
}

// balancedArgs returns the argument text between the paren at index open and
// its matching close paren, plus the index of that close paren. Quoted strings
// are skipped so parens inside string literals do not affect the depth count.
// Returns ("", -1) when the parens are unbalanced.
func balancedArgs(src string, open int) (string, int) {
	depth := 0
	var quote rune
	for i := open; i < len(src); i++ {
		r := rune(src[i])
		if quote != 0 {
			if r == quote {
				quote = 0
			}
			continue
		}
		switch r {
		case '"', '\'', '`':
			quote = r
		case '(':
			depth++
		case ')':
			depth--
			if depth == 0 {
				return strings.TrimSpace(src[open+1 : i]), i
			}
		}
	}
	return "", -1
}
