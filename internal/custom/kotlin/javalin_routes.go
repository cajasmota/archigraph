// Package kotlin — javalin_routes.go: regex/scan-based route extractor for the
// Javalin (Kotlin/JVM) web framework.
//
// Routing.route_extraction + endpoint_synthesis + handler_attribution coverage
// for:
//   - lang.kotlin.framework.javalin
//
// Javalin exposes two route-registration idioms, both of which Kotlin code uses:
//
//  1. Direct fluent DSL — a verb method on the app handle takes a path string
//     and a handler (trailing lambda or method reference):
//
//     app.get("/users") { ctx -> ctx.json(users) }     → GET  /users  (lambda)
//     app.post("/users", ::createUser)                  → POST /users  (::createUser)
//     Javalin.create().get("/x", UserController::getAll) → GET /x      (UserController::getAll)
//
//  2. ApiBuilder DSL — a `routes { … }` block (app.routes { … } or a bare
//     `routes { … }`) containing nested `path("prefix") { … }` scopes whose
//     inner verb calls inherit the composed prefix:
//
//     app.routes {
//     path("users") {
//     get(UserController::getAll)   → GET  /users  (UserController::getAll)
//     post(::create)                → POST /users  (::create)
//     path("{id}") {
//     delete(::remove)          → DELETE /users/{id}  (::remove)
//     }
//     }
//     }
//
// Both forms emit a SCOPE.Operation `endpoint` entity named "<VERB> <path>" with
// http_method / path / framework / handler properties — the same entity shape
// the http4k / spring / micronaut / quarkus Kotlin route extractors emit, which
// the graph treats as the synthesized endpoint for these frameworks.
//
// Honest-partial boundaries:
//   - before/after are middleware, NOT routes — they are deliberately skipped
//     here (Routing scope only). app.config(...) and other non-verb calls emit
//     nothing.
//   - ApiBuilder verb calls without an explicit sub-path bind to the enclosing
//     path() prefix. A bare `get(handler)` at the top of a routes{} block with no
//     enclosing path() maps to "/".
//
// Issue #4017 (epic #3872) — Kotlin Javalin Routing REAL-GAP.
package kotlin

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
	extractor.Register("custom_kotlin_javalin_routes", &kotlinJavalinRoutesExtractor{})
}

// kotlinJavalinRoutesExtractor emits SCOPE.Operation endpoint entities for
// Javalin Kotlin route registrations (direct fluent DSL + ApiBuilder DSL).
type kotlinJavalinRoutesExtractor struct{}

func (e *kotlinJavalinRoutesExtractor) Language() string { return "custom_kotlin_javalin_routes" }

var (
	// reJavalinDirect matches a direct fluent-DSL route registration:
	//   <receiver>.get("/path"   …  (lambda or 2nd arg handler follows)
	// The receiver is any identifier or a chained call tail (e.g. `app`,
	// `Javalin.create()`). We anchor on `.<verb>(` followed by a quoted path so
	// `app.config(...)`, `app.start(7000)` and similar non-route calls never
	// match. Capture: (verb, path). The leading `\.` requires a receiver, so a
	// bare ApiBuilder `get("/x")` (no receiver) is NOT matched here — those are
	// handled by the ApiBuilder scanner.
	reJavalinDirect = regexp.MustCompile(
		`\.\s*(get|post|put|delete|patch|head|options)\s*\(\s*"([^"]+)"`)

	// reJavalinApiBuilderBlock matches the opening of an ApiBuilder routes block:
	//   app.routes {     or     routes {
	// Capture group 0 only; we use the match offset to locate the block.
	reJavalinApiBuilderBlock = regexp.MustCompile(`(?:\.\s*routes|\broutes)\s*\{`)

	// reJavalinPathScope matches an ApiBuilder path("prefix") { scope opener.
	// Capture: (prefix).
	reJavalinPathScope = regexp.MustCompile(`\bpath\s*\(\s*"([^"]*)"\s*\)\s*\{`)

	// reJavalinApiVerb matches an ApiBuilder bare verb call (no receiver):
	//   get(::handler)            → verb, "", handler
	//   post("sub", ::create)     → verb, "sub", handler
	//   get(UserController::all)  → verb, "", handler
	// The verb must be preceded by a non-identifier, non-`.` char so that a
	// direct `app.get(` (receiver form) is excluded — those are claimed by
	// reJavalinDirect. Capture groups: (verb, optPath, handler).
	reJavalinApiVerb = regexp.MustCompile(
		`(?:^|[^\w.])(get|post|put|delete|patch|head|options)\s*\(\s*` +
			`(?:"([^"]*)"\s*,\s*)?` +
			`([^)\n]*?)\s*\)`)
)

var ktJavalinVerbMap = map[string]string{
	"get": "GET", "post": "POST", "put": "PUT", "delete": "DELETE",
	"patch": "PATCH", "head": "HEAD", "options": "OPTIONS",
}

func (e *kotlinJavalinRoutesExtractor) Extract(ctx context.Context, file extractor.FileInput) ([]types.EntityRecord, error) {
	tracer := otel.Tracer("grafel/custom/kotlin")
	_, span := tracer.Start(ctx, "indexer.kotlin_javalin_routes.extract",
		trace.WithAttributes(
			attribute.String("language", file.Language),
			attribute.String("framework", "javalin"),
			attribute.String("file_path", file.Path),
		),
	)
	defer span.End()

	if len(file.Content) == 0 || file.Language != "kotlin" {
		return nil, nil
	}
	src := string(file.Content)
	// File-signal gate: require a Javalin reference so this never fires on
	// http4k / ktor / spring Kotlin files (which also use get/post tokens).
	if !strings.Contains(src, "javalin") && !strings.Contains(src, "Javalin") {
		return nil, nil
	}

	var entities []types.EntityRecord
	seen := make(map[string]bool)

	add := func(ent types.EntityRecord) {
		key := ent.Kind + ":" + ent.Name + ":" + ent.Properties["handler"]
		if seen[key] {
			return
		}
		seen[key] = true
		entities = append(entities, ent)
	}

	emitRoute := func(verb, path, handler string, off int) {
		if path == "" {
			path = "/"
		}
		path = ensureLeadingSlash(path)
		name := verb + " " + path
		ent := makeEntity(name, "SCOPE.Operation", "endpoint", file.Path, "kotlin", lineOf(src, off))
		props := []string{
			"framework", "javalin",
			"http_method", verb,
			"path", path,
			"provenance", "INFERRED_FROM_JAVALIN_ROUTE",
		}
		if handler != "" {
			props = append(props, "handler", handler)
		}
		setProps(&ent, props...)
		add(ent)
	}

	// -----------------------------------------------------------------------
	// ApiBuilder DSL: app.routes { path("p") { get(::h); … } } with nesting.
	// Resolve these FIRST and record the byte ranges they cover so the direct
	// scanner does not also claim the verb calls inside a routes{} block (a
	// bare `get("sub", ::h)` has no receiver so reJavalinDirect won't match it,
	// but `path("p")` and chained forms could overlap — record ranges to be
	// safe and to keep direct-vs-ApiBuilder attribution clean).
	// -----------------------------------------------------------------------
	apiBuilderRanges := javalinApiBuilderRoutes(src, emitRoute)

	// -----------------------------------------------------------------------
	// Direct fluent DSL: app.get("/path") { ctx -> } / app.post("/p", ::h).
	// Skip any match that falls inside an ApiBuilder routes{} block.
	// -----------------------------------------------------------------------
	for _, m := range reJavalinDirect.FindAllStringSubmatchIndex(src, -1) {
		if offsetInAnyRange(m[0], apiBuilderRanges) {
			continue
		}
		verb := ktJavalinVerbMap[src[m[2]:m[3]]]
		path := src[m[4]:m[5]]
		handler := javalinHandlerAfter(src, m[1])
		emitRoute(verb, path, handler, m[0])
	}

	span.SetAttributes(attribute.Int("entity_count", len(entities)))
	return entities, nil
}

// byteRange is a half-open [start,end) span of source covered by an ApiBuilder
// routes{} block.
type byteRange struct{ start, end int }

func offsetInAnyRange(off int, ranges []byteRange) bool {
	for _, r := range ranges {
		if off >= r.start && off < r.end {
			return true
		}
	}
	return false
}

// javalinApiBuilderRoutes scans every `routes { … }` ApiBuilder block in src,
// emitting one composed route per verb call with its enclosing path() prefixes
// composed in, and returns the byte ranges of the blocks it consumed so the
// caller can exclude them from the direct-DSL scan.
//
// Within a block it maintains a prefix stack keyed by brace depth: a
// `path("p") {` pushes p at the depth of its `{`; the frame is popped when that
// brace closes. Each `verb(...)` call composes every active prefix onto its
// optional sub-path.
func javalinApiBuilderRoutes(src string, emit func(verb, path, handler string, off int)) []byteRange {
	var ranges []byteRange
	for _, bm := range reJavalinApiBuilderBlock.FindAllStringIndex(src, -1) {
		openBrace := bm[1] - 1 // the '{' is the final char of the match
		closeBrace := matchCloseBrace(src, openBrace)
		if closeBrace < 0 {
			continue
		}
		ranges = append(ranges, byteRange{start: bm[0], end: closeBrace + 1})
		javalinScanApiBlock(src, openBrace, closeBrace, emit)
	}
	return ranges
}

// pathFrame is one active ApiBuilder path() prefix scope.
type pathFrame struct {
	prefix   string
	braceOff int // offset of the '{' that opens this path scope
	closeOff int // offset of the matching '}'
}

// javalinScanApiBlock walks the ApiBuilder block bounded by [openBrace,closeBrace]
// and emits composed verb routes. It first locates every path("p") { scope and
// its matching close brace to build a nesting stack, then walks each verb call
// and composes the prefixes of all path scopes that strictly contain it.
func javalinScanApiBlock(src string, openBrace, closeBrace int, emit func(verb, path, handler string, off int)) {
	block := src[openBrace : closeBrace+1]
	base := openBrace

	// Collect all path() scopes inside this block (by absolute offset).
	var scopes []pathFrame
	for _, pm := range reJavalinPathScope.FindAllStringSubmatchIndex(block, -1) {
		prefix := block[pm[2]:pm[3]]
		scopeBrace := base + pm[1] - 1 // the '{' is the final char of the path() match
		scopeClose := matchCloseBrace(src, scopeBrace)
		if scopeClose < 0 {
			continue
		}
		scopes = append(scopes, pathFrame{prefix: prefix, braceOff: scopeBrace, closeOff: scopeClose})
	}

	// Walk each verb call; compose the prefixes of every path scope that
	// contains the call's offset, outermost-first.
	for _, vm := range reJavalinApiVerb.FindAllStringSubmatchIndex(block, -1) {
		verbOff := base + vm[2] // offset of the verb token (group 1 start)
		// The verb call must live inside this routes block (it does, by slice)
		// but skip a verb token that is actually the `path` opener's `{` body —
		// path() itself is not a verb so it can't match reJavalinApiVerb.
		verb := ktJavalinVerbMap[block[vm[2]:vm[3]]]
		if verb == "" {
			continue
		}
		subPath := ""
		if vm[4] >= 0 {
			subPath = block[vm[4]:vm[5]]
		}
		handlerArg := ""
		if vm[6] >= 0 {
			handlerArg = strings.TrimSpace(block[vm[6]:vm[7]])
		}
		handler := javalinNormalizeHandlerArg(handlerArg)

		// Compose enclosing path() prefixes (outermost-first).
		composed := subPath
		for i := len(scopes) - 1; i >= 0; i-- {
			s := scopes[i]
			if verbOff > s.braceOff && verbOff < s.closeOff {
				composed = joinKtRoutePaths(s.prefix, composed)
			}
		}
		emit(verb, composed, handler, verbOff)
	}
}

// javalinHandlerAfter inspects the source immediately following a direct route's
// closing path quote (offset `after` = just past the `"`) and returns a handler
// descriptor: a method reference (`::h`, `Ctrl::m`), a constructed handler class
// (`HandlerClass(`), or "lambda" for a trailing/inline lambda. Returns "" when no
// handler is discernible on the route statement (honest-partial).
func javalinHandlerAfter(src string, after int) string {
	// Bound the scan to this route statement: up to the matching ')' of the
	// verb call's argument list OR a trailing `{` lambda opener, whichever the
	// statement uses, capped at the end of the line + an optional trailing
	// lambda brace.
	end := after
	for end < len(src) && src[end] != '\n' && src[end] != '{' {
		end++
	}
	// Allow a trailing-lambda `{` right after the `)` on the same/next token.
	stmt := src[after:end]

	// Trailing lambda: app.get("/x") { ctx -> … }  → handler is the lambda.
	if end < len(src) && src[end] == '{' {
		return "lambda"
	}
	// Second-arg handler inside the parens: app.get("/x", ::h) /
	// app.get("/x", Ctrl::m) / app.get("/x", HandlerClass()).
	if i := strings.Index(stmt, ","); i >= 0 {
		arg := stmt[i+1:]
		if j := strings.Index(arg, ")"); j >= 0 {
			arg = arg[:j]
		}
		return javalinNormalizeHandlerArg(strings.TrimSpace(arg))
	}
	return ""
}

// javalinNormalizeHandlerArg turns a raw handler argument into a stable handler
// descriptor. Method references (`::h`, `Ctrl::m`) are returned verbatim; a
// `ClassName(...)` constructed handler returns `ClassName`; an inline lambda
// fragment (`ctx ->`) or an empty/opaque arg returns "lambda"/"" respectively.
func javalinNormalizeHandlerArg(arg string) string {
	arg = strings.TrimSpace(arg)
	if arg == "" {
		return ""
	}
	if strings.Contains(arg, "->") || strings.HasPrefix(arg, "{") {
		return "lambda"
	}
	if strings.Contains(arg, "::") {
		// Trim anything after the reference (e.g. trailing comma artifacts).
		ref := arg
		if k := strings.IndexAny(ref, " ,"); k >= 0 {
			ref = ref[:k]
		}
		return ref
	}
	// Constructed handler: HandlerClass() — keep the class name.
	if i := strings.Index(arg, "("); i >= 0 {
		name := strings.TrimSpace(arg[:i])
		if name != "" {
			return name
		}
	}
	// A bare identifier handler (a val holding a Handler).
	if isJavalinIdentifier(arg) {
		return arg
	}
	return ""
}

func isJavalinIdentifier(s string) bool {
	if s == "" {
		return false
	}
	for i, r := range s {
		isLetter := r == '_' || (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z')
		isDigit := r >= '0' && r <= '9'
		if i == 0 && !isLetter {
			return false
		}
		if i > 0 && !isLetter && !isDigit {
			return false
		}
	}
	return true
}

// matchCloseBrace returns the offset of the '}' that matches the '{' at
// openBrace, or -1 if unbalanced. src[openBrace] must be '{'.
func matchCloseBrace(src string, openBrace int) int {
	if openBrace < 0 || openBrace >= len(src) || src[openBrace] != '{' {
		return -1
	}
	depth := 0
	for i := openBrace; i < len(src); i++ {
		switch src[i] {
		case '{':
			depth++
		case '}':
			depth--
			if depth == 0 {
				return i
			}
		}
	}
	return -1
}
