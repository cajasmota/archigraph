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
	tracer := otel.Tracer("archigraph/custom/golang")
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
	hasMarker := strings.Contains(src, "danielgtaylor/huma") || strings.Contains(src, "huma.Register") ||
		strings.Contains(src, "huma.API") || strings.Contains(src, "UseMiddleware")
	if !hasMarker {
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

	// middleware / auth / request_validation surfaces (issue #3255).
	e.extractMiddlewareAuth(src, file, add)
	e.extractRequestValidation(src, file, add)

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
	}

	span.SetAttributes(attribute.Int("entity_count", len(entities)))
	return entities, nil
}

// ---------------------------------------------------------------------------
// Middleware + auth + request validation (issue #3255).
//
// huma's routing extractor above models huma.Register endpoints. The three
// capabilities below model huma's middleware/security/validation surface, which
// the routing pass does not touch:
//
//   - middleware_coverage : middleware is installed on a huma.API via
//       api.UseMiddleware(mw, …) (and the huma.Middlewares{…} slice). Each
//       middleware expression is one ordered SCOPE.Pattern.
//   - auth_coverage      : huma is OpenAPI-first — auth is declared by a
//       Security field on a huma.Operation literal naming security schemes, and
//       the schemes themselves are registered under Components.SecuritySchemes
//       (often huma.SecurityScheme / type: "http", scheme: "bearer"). Both the
//       per-operation Security requirement and the scheme registration are auth
//       SCOPE.Pattern entities.
//   - request_validation : huma auto-validates every input struct from its
//       OpenAPI-schema field tags (required/minimum/maximum/maxLength/format/
//       enum/pattern). Each validated field tag is a validation rule
//       SCOPE.Pattern; the huma.Register call is itself the binding site.
//
// Honesty: heuristic identifier/substring/tag matches on source text with no
// data-flow proof, so each capability is reported `partial`. huma genuinely has
// all three concepts (not NA); the proving fixture exercises each.
// ---------------------------------------------------------------------------

var (
	// api.UseMiddleware( — huma middleware install on an API value.
	reHumaUseMiddleware = regexp.MustCompile(`\.UseMiddleware\s*\(`)
	// huma.Middlewares{ … } slice literal of middleware values.
	reHumaMiddlewaresHead = regexp.MustCompile(`huma\.Middlewares\s*\{`)
	// Security: field on a huma.Operation literal. The value is a
	// []map[string][]string{ {"<scheme>": {…}} } composite literal; the scheme
	// names live in the trailing {…} block, so the match anchors on the field
	// and the block is scanned forward from the first following '{'.
	reHumaSecurityField = regexp.MustCompile(`\bSecurity\s*:`)
	// A security-scheme name key inside a Security requirement / scheme registry:
	// "bearer": { … } | "ApiKeyAuth": … . Captures the scheme name.
	reHumaSchemeKey = regexp.MustCompile(`"([A-Za-z][\w-]*)"\s*:`)
	// Components.SecuritySchemes registration site.
	reHumaSecuritySchemes = regexp.MustCompile(`SecuritySchemes\b`)
	// huma input-struct validation tags (OpenAPI-schema auto-validation). Each
	// captures the field name + the tag key that drives validation.
	reHumaValidatedField = regexp.MustCompile(
		"(?m)^\\s*(\\w+)\\s+[^`\\n]*`[^`]*\\b(required|minimum|maximum|minLength|maxLength|pattern|format|enum|exclusiveMinimum|exclusiveMaximum):\"[^\"]*\"[^`]*`")
)

var reHumaCallHead = regexp.MustCompile(`^[A-Za-z_][\w.]*`)

// humaClassifyAuth — file-local auth classifier for a scheme name / middleware
// expression (shared helpers untouched).
func humaClassifyAuth(s string) string {
	low := strings.ToLower(s)
	switch {
	case strings.Contains(low, "jwt"), strings.Contains(low, "bearer"):
		return "jwt"
	case strings.Contains(low, "oauth"):
		return "oauth"
	case strings.Contains(low, "apikey"), strings.Contains(low, "api_key"), strings.Contains(low, "api-key"):
		return "api_key"
	case strings.Contains(low, "basic"):
		return "basic"
	default:
		return "auth"
	}
}

// extractMiddlewareAuth scans api.UseMiddleware(...) / huma.Middlewares{...}
// chains (middleware), and Security:[...] requirements + SecuritySchemes
// registrations (auth).
func (e *humaExtractor) extractMiddlewareAuth(src string, file extractor.FileInput, add func(types.EntityRecord)) {
	emitChain := func(args string, line int) {
		order := 0
		for _, part := range splitTopLevelArgs(args) {
			if part == "" || isStringLiteral(part) {
				continue
			}
			name := reHumaCallHead.FindString(part)
			if name == "" {
				name = part
			}
			mw := makeEntity(part, "SCOPE.Pattern", "", file.Path, file.Language, line)
			setProps(&mw, "framework", "huma",
				"provenance", "INFERRED_FROM_HUMA_MIDDLEWARE",
				"pattern_kind", "middleware",
				"middleware_name", name,
				"mw_order", itoa(order))
			add(mw)
			order++
		}
	}
	// api.UseMiddleware(mw, …)
	for _, loc := range reHumaUseMiddleware.FindAllStringIndex(src, -1) {
		open := loc[1] - 1
		if args, end := balancedArgs(src, open); end >= 0 {
			emitChain(args, lineOf(src, loc[0]))
		}
	}
	// huma.Middlewares{ mw, … }
	for _, loc := range reHumaMiddlewaresHead.FindAllStringIndex(src, -1) {
		brace := loc[1] - 1
		if args, end := balancedBraces(src, brace); end >= 0 {
			emitChain(args, lineOf(src, loc[0]))
		}
	}

	// Security:[...] requirements on huma.Operation literals — each named scheme
	// is an auth SCOPE.Pattern (auth requirement on the operation).
	for _, loc := range reHumaSecurityField.FindAllStringIndex(src, -1) {
		rest := src[loc[1]:]
		bi := strings.IndexByte(rest, '{')
		if bi < 0 || bi > 60 {
			continue
		}
		req, end := balancedBraces(src, loc[1]+bi)
		if end < 0 {
			continue
		}
		line := lineOf(src, loc[0])
		for _, km := range reHumaSchemeKey.FindAllStringSubmatch(req, -1) {
			scheme := km[1]
			au := makeEntity("auth:requirement:"+scheme, "SCOPE.Pattern", "", file.Path, file.Language, line)
			setProps(&au, "framework", "huma",
				"provenance", "INFERRED_FROM_HUMA_AUTH",
				"pattern_kind", "auth",
				"auth_kind", humaClassifyAuth(scheme),
				"scheme_name", scheme,
				"auth_source", "operation_security")
			add(au)
		}
	}

	// Components.SecuritySchemes registrations — each scheme name keyed in the
	// scheme map is an auth SCOPE.Pattern (scheme definition).
	for _, loc := range reHumaSecuritySchemes.FindAllStringIndex(src, -1) {
		// Scan the brace block following the SecuritySchemes token for scheme keys.
		rest := src[loc[1]:]
		bi := strings.IndexByte(rest, '{')
		if bi < 0 || bi > 80 {
			continue
		}
		block, end := balancedBraces(src, loc[1]+bi)
		if end < 0 {
			continue
		}
		line := lineOf(src, loc[0])
		for _, km := range reHumaSchemeKey.FindAllStringSubmatch(block, -1) {
			scheme := km[1]
			au := makeEntity("auth:scheme:"+scheme, "SCOPE.Pattern", "", file.Path, file.Language, line)
			setProps(&au, "framework", "huma",
				"provenance", "INFERRED_FROM_HUMA_AUTH",
				"pattern_kind", "auth",
				"auth_kind", humaClassifyAuth(scheme),
				"scheme_name", scheme,
				"auth_source", "security_scheme")
			add(au)
		}
	}
}

// extractRequestValidation emits a validation SCOPE.Pattern per huma input-
// struct field carrying an OpenAPI-schema validation tag.
func (e *humaExtractor) extractRequestValidation(src string, file extractor.FileInput, add func(types.EntityRecord)) {
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
	for _, m := range reHumaValidatedField.FindAllStringSubmatchIndex(src, -1) {
		field := src[m[2]:m[3]]
		tagKey := src[m[4]:m[5]]
		st := structAt(m[0])
		ent := makeEntity("validation:rule:"+st+"."+field, "SCOPE.Pattern", "", file.Path, file.Language, lineOf(src, m[0]))
		setProps(&ent, "framework", "huma",
			"provenance", "INFERRED_FROM_HUMA_VALIDATION",
			"pattern_kind", "validation",
			"validation_kind", "rule",
			"struct_name", st,
			"field_name", field,
			"rules", tagKey,
			"rule_source", "openapi_schema_tag")
		add(ent)
	}
}

// balancedBraces returns the text between the brace at index open and its match,
// plus the index of the closing brace. Mirrors balancedArgs for `{}`.
func balancedBraces(src string, open int) (string, int) {
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
		case '{':
			depth++
		case '}':
			depth--
			if depth == 0 {
				return strings.TrimSpace(src[open+1 : i]), i
			}
		}
	}
	return "", -1
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
