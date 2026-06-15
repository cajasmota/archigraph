// Additional JS/TS producer-side synthesizers added by the #2678 audit:
//
//   - Fastify (fastify.get / server.post / instance.<verb>): the Express
//     synthesizer's receiver allowlist does not include "fastify" — the
//     receiver name does not match the `app|router|server|*Router|*App`
//     allowlist regex, so Fastify endpoints were not emitted at all.
//
//   - Next.js API routes: file-path-based routing. The handler lives in
//     `pages/api/<segments>.{ts,js}` (pages router) or
//     `app/api/<segments>/route.{ts,js}` (app router). The verb is either
//     the file's default export (pages) or one of the named verb exports
//     (`export async function GET`, etc.).
//
// Both synthesizers re-use the shared `emit` closure from
// applyHTTPEndpointSynthesis so the existing http_endpoint_resolve.go
// rewrite (handler→file/line) applies uniformly.
package engine

import (
	"path/filepath"
	"regexp"
	"sort"
	"strings"

	"github.com/cajasmota/grafel/internal/engine/httproutes"
)

// ---------------------------------------------------------------------------
// Fastify
// ---------------------------------------------------------------------------

// fastifyAllowedReceiverRe matches receiver names that are conventional for
// Fastify instances. The Express synthesizer's allowlist does NOT include
// "fastify", so Fastify routes were silently dropped before #2678.
//
// Accepted receivers: `fastify`, `instance`, `f`, `srv`, `server` (also matched
// by Express; Express's blocklist defers to us when the verb regex fires from
// either side — duplicates collapse via the side-scoped seen-map in
// http_endpoint_synthesis.go).
var fastifyAllowedReceiverRe = regexp.MustCompile(
	`^(?:fastify|instance|f|srv|server|\w+[Ff]astify|\w+[Ii]nstance)$`,
)

// fastifyVerbRe captures `<recv>.<verb>('/path', handler)` for the canonical
// handler-named form. Verbs are the Fastify-supported set.
// Groups: 1=receiver, 2=verb, 3=path, 4=handler.
var fastifyVerbRe = regexp.MustCompile(
	`([$\w][\w$]*)\.(get|post|put|patch|delete|head|options|all)\s*\(` +
		`\s*['"` + "`" + `]([^'"` + "`" + `\n\r]+)['"` + "`" + `]` +
		`\s*(?:,[^,)]*)*?,\s*([\w$.]+)\s*[\),]`,
)

// fastifyVerbPathOnlyRe captures the inline-handler form
// `<recv>.<verb>('/path', async (req, reply) => {...})` where the handler is
// an inline function expression. We still emit, leaving source_handler empty.
// Groups: 1=receiver, 2=verb, 3=path.
var fastifyVerbPathOnlyRe = regexp.MustCompile(
	`([$\w][\w$]*)\.(get|post|put|patch|delete|head|options|all)\s*\(` +
		`\s*['"` + "`" + `]([^'"` + "`" + `\n\r]+)['"` + "`" + `]`,
)

// fastifyRouteRe captures `<recv>.route({ method: 'GET', url: '/path', handler })`
// — Fastify's structured registration form.
// Groups: 1=receiver, 2=method, 3=url, 4=handler (optional).
var fastifyRouteRe = regexp.MustCompile(
	`([$\w][\w$]*)\.route\s*\(\s*\{[^}]*?` +
		`method\s*:\s*['"` + "`" + `](GET|POST|PUT|PATCH|DELETE|HEAD|OPTIONS)['"` + "`" + `]` +
		`[^}]*?url\s*:\s*['"` + "`" + `]([^'"` + "`" + `\n\r]+)['"` + "`" + `]` +
		`(?:[^}]*?handler\s*:\s*([\w$.]+))?` +
		`[^}]*?\}`,
)

// fastifyRegisterRe captures the opening of a `<recv>.register(` call so #2934
// can locate plugin-registration spans and their `{ prefix: '/x' }` option.
// Group 1 = receiver (gated by fastifyAllowedReceiverRe).
var fastifyRegisterRe = regexp.MustCompile(`([$\w][\w$]*)\.register\s*\(`)

// fastifyPrefixOptRe captures the `prefix: '/v1'` option inside a register
// call's options object. Group 1 = prefix string.
var fastifyPrefixOptRe = regexp.MustCompile(
	`\bprefix\s*:\s*['"` + "`" + `]([^'"` + "`" + `\n\r]*)['"` + "`" + `]`,
)

// fastifyPluginSpan describes one `fastify.register(plugin, { prefix: '/x' })`
// registration: the byte range [bodyStart, bodyEnd) covering the inline plugin
// function body and the normalized prefix declared in the options object.
// Routes registered on the plugin instance inside that body compose the prefix
// (#2934). Only inline plugin functions (with a `{ … }` body in the same file)
// are span-resolvable; imported plugins compose nothing here (the cross-repo
// matcher's prefix-stripping still links those).
type fastifyPluginSpan struct {
	bodyStart int
	bodyEnd   int
	prefix    string
}

// buildFastifyPluginSpans locates each `<recv>.register(` call, brace-matches
// the FIRST `{ … }` after the opening paren (the inline plugin function body),
// and pairs it with the `prefix:` option found in the register call's argument
// region (#2934). Registrations with no inline body or no prefix are skipped.
func buildFastifyPluginSpans(content string) []fastifyPluginSpan {
	if !strings.Contains(content, ".register") {
		return nil
	}
	var spans []fastifyPluginSpan
	for _, loc := range fastifyRegisterRe.FindAllStringSubmatchIndex(content, -1) {
		recv := content[loc[2]:loc[3]]
		if !fastifyAllowedReceiverRe.MatchString(recv) {
			continue
		}
		openParen := loc[1] - 1 // index of `(`
		// Plugin body: the first `{` after the open paren that is part of the
		// callback (inline async (instance) => { … } or function (f) { … }).
		bodyOpen := strings.IndexByte(content[openParen:], '{')
		if bodyOpen < 0 {
			continue
		}
		bodyOpen += openParen
		bodyEnd := matchBrace(content, bodyOpen)
		if bodyEnd < 0 {
			continue
		}
		// The options object with `prefix:` follows the plugin body, before the
		// register call's closing paren. Scan the window after the body.
		tail := content[bodyEnd+1:]
		if len(tail) > 128 {
			tail = tail[:128]
		}
		m := fastifyPrefixOptRe.FindStringSubmatch(tail)
		if m == nil {
			continue
		}
		spans = append(spans, fastifyPluginSpan{
			bodyStart: bodyOpen,
			bodyEnd:   bodyEnd,
			prefix:    normalizeMountPrefix(m[1]),
		})
	}
	return spans
}

// fastifyComposedPrefix returns the joined prefix of every plugin span whose
// body encloses offset, outermost first (#2934) — supporting nested
// register(...) plugins. Empty when no enclosing plugin declares a prefix.
func fastifyComposedPrefix(spans []fastifyPluginSpan, offset int) string {
	var enclosing []fastifyPluginSpan
	for _, s := range spans {
		if offset > s.bodyStart && offset < s.bodyEnd && s.prefix != "" {
			enclosing = append(enclosing, s)
		}
	}
	sort.Slice(enclosing, func(i, j int) bool { return enclosing[i].bodyStart < enclosing[j].bodyStart })
	composed := ""
	for _, s := range enclosing {
		composed = joinPathFragments(composed, s.prefix)
	}
	return composed
}

// synthesizeFastify emits http_endpoint_definition entities for Fastify
// route registrations. It complements synthesizeExpress, which would miss
// these because its receiver allowlist excludes "fastify".
func synthesizeFastify(content string, emit emitFn) {
	// Fast path: bail unless the file plausibly imports / uses Fastify.
	if !strings.Contains(content, "fastify") && !strings.Contains(content, "Fastify") {
		return
	}
	// #2934 — plugin-registration prefixes: routes registered inside an inline
	// `fastify.register(plugin, { prefix: '/v1' })` body compose `/v1`.
	pluginSpans := buildFastifyPluginSpans(content)
	withHandler := map[string]bool{}
	for _, m := range fastifyVerbRe.FindAllStringSubmatchIndex(content, -1) {
		if len(m) < 10 {
			continue
		}
		receiver := content[m[2]:m[3]]
		if !fastifyAllowedReceiverRe.MatchString(receiver) {
			continue
		}
		raw := content[m[6]:m[7]]
		if !looksLikeExpressPath(raw) {
			continue
		}
		verb := strings.ToUpper(content[m[4]:m[5]])
		handler := content[m[8]:m[9]]
		refKind := "Controller"
		if isInlineExpressHandler(content[m[0]:m[1]], raw) {
			// #4324 — inline arrow/function-expression handler; no symbol.
			handler = ""
			refKind = inlineHandlerRefKind
		}
		full := joinPathFragments(fastifyComposedPrefix(pluginSpans, m[0]), raw)
		canonical := httproutes.Canonicalize(httproutes.FrameworkExpress, full)
		if verb == "ALL" {
			verb = "ANY"
		}
		key := verb + ":" + canonical
		withHandler[key] = true
		emit(verb, canonical, "fastify", refKind, handler)
	}
	for _, m := range fastifyVerbPathOnlyRe.FindAllStringSubmatchIndex(content, -1) {
		if len(m) < 8 {
			continue
		}
		receiver := content[m[2]:m[3]]
		if !fastifyAllowedReceiverRe.MatchString(receiver) {
			continue
		}
		raw := content[m[6]:m[7]]
		if !looksLikeExpressPath(raw) {
			continue
		}
		verb := strings.ToUpper(content[m[4]:m[5]])
		full := joinPathFragments(fastifyComposedPrefix(pluginSpans, m[0]), raw)
		canonical := httproutes.Canonicalize(httproutes.FrameworkExpress, full)
		if verb == "ALL" {
			verb = "ANY"
		}
		key := verb + ":" + canonical
		if withHandler[key] {
			continue
		}
		// #4324 — handler-named pass missed this (verb,path) ⇒ inline handler.
		emit(verb, canonical, "fastify", inlineHandlerRefKind, "")
	}
	// Structured form: fastify.route({ method, url, handler }).
	for _, m := range fastifyRouteRe.FindAllStringSubmatchIndex(content, -1) {
		if len(m) < 8 {
			continue
		}
		receiver := content[m[2]:m[3]]
		if !fastifyAllowedReceiverRe.MatchString(receiver) {
			continue
		}
		verb := strings.ToUpper(content[m[4]:m[5]])
		raw := content[m[6]:m[7]]
		handler := ""
		if len(m) >= 10 && m[8] >= 0 {
			handler = content[m[8]:m[9]]
		}
		if !looksLikeExpressPath(raw) {
			continue
		}
		full := joinPathFragments(fastifyComposedPrefix(pluginSpans, m[0]), raw)
		canonical := httproutes.Canonicalize(httproutes.FrameworkExpress, full)
		emit(verb, canonical, "fastify", "Controller", handler)
	}
}

// ---------------------------------------------------------------------------
// Next.js API routes
// ---------------------------------------------------------------------------

// nextPagesAPIRe matches a `pages/api/...` file path (pages router). The
// handler is the file's `export default` function. Verb is "ANY" unless the
// file body discriminates on `req.method` (we still emit ANY — the verb
// dispatch happens inside the handler at runtime).
var nextPagesAPIRe = regexp.MustCompile(`(?:^|/)pages/api/(.+?)(?:\.(?:ts|tsx|js|jsx|mjs|cjs))$`)

// nextAppRouterRe matches an `app/api/<segments>/route.{ts,js}` file path
// (app router). Each verb is an exported function `export async function GET`
// etc.; we emit one endpoint per exported verb.
var nextAppRouterRe = regexp.MustCompile(`(?:^|/)app/api/(.+?)/route\.(?:ts|tsx|js|jsx|mjs|cjs)$`)

// nextAppRouterVerbRe captures `export (async )?function <VERB>(` for the
// app-router verb exports. We accept GET/POST/PUT/PATCH/DELETE/HEAD/OPTIONS.
// Group 1 = verb.
var nextAppRouterVerbRe = regexp.MustCompile(
	`export\s+(?:async\s+)?function\s+(GET|POST|PUT|PATCH|DELETE|HEAD|OPTIONS)\s*\(`,
)

// nextPagesHandlerNameRe captures the function name on
// `export default function <name>(` so the resolver can stamp the precise
// source_line. Anonymous `export default async function(req,res){}` falls back
// to an empty handler name (which leaves the synthetic at file-level — still
// correct file, since pages/api routes are file-anchored).
var nextPagesHandlerNameRe = regexp.MustCompile(
	`export\s+default\s+(?:async\s+)?function\s+([A-Za-z_$][\w$]*)\s*\(`,
)

// synthesizeNextAPIRoute emits http_endpoint_definition entities for Next.js
// API routes. The synthetic path is the file path's `api/...` suffix with
// dynamic-segment normalisation (`[id]` → `{id}`, `[...slug]` → `{slug}`).
func synthesizeNextAPIRoute(filePath, content string, emit emitFn) {
	if filePath == "" {
		return
	}
	slash := filepath.ToSlash(filePath)

	// App router: app/api/<segments>/route.ts → verbs from named exports.
	if m := nextAppRouterRe.FindStringSubmatch(slash); len(m) == 2 {
		canonical := nextNormalizePath("/api/" + m[1])
		for _, vm := range nextAppRouterVerbRe.FindAllStringSubmatch(content, -1) {
			if len(vm) < 2 {
				continue
			}
			verb := strings.ToUpper(vm[1])
			// The handler IS the verb function — name it by its export
			// (GET/POST/...) so the resolver can bind it to the
			// SCOPE.Operation entity the JS/TS extractor emits.
			emit(verb, canonical, "nextjs", "Controller", verb)
		}
		return
	}

	// Pages router: pages/api/<segments>.ts → single default export, verb ANY.
	if m := nextPagesAPIRe.FindStringSubmatch(slash); len(m) == 2 {
		// Skip the special _middleware / _app / _document conventions.
		base := filepath.Base(m[1])
		if strings.HasPrefix(base, "_") {
			return
		}
		canonical := nextNormalizePath("/api/" + m[1])
		// Try to capture a named default-export so the resolver lands on the
		// function def line; otherwise emit with empty handler (file-anchor).
		handler := ""
		if hm := nextPagesHandlerNameRe.FindStringSubmatch(content); len(hm) >= 2 {
			handler = hm[1]
		}
		emit("ANY", canonical, "nextjs", "Controller", handler)
		return
	}
}

// nextNormalizePath rewrites Next.js dynamic segments to the
// grafel-canonical `{name}` form so the cross-repo linker can match
// frontend calls.
//
//	[id]         → {id}
//	[...slug]    → {slug}
//	[[...slug]]  → {slug}
//
// Index routes (`/api/users/index`) collapse to `/api/users`.
func nextNormalizePath(p string) string {
	// Strip trailing `/index` — Next.js treats `pages/api/users/index.ts`
	// the same as `pages/api/users.ts`.
	if strings.HasSuffix(p, "/index") {
		p = strings.TrimSuffix(p, "/index")
	}
	// Replace `[[...x]]` and `[...x]` first (catch-all) then `[x]`.
	for _, pat := range []struct{ open, close string }{
		{"[[...", "]]"},
		{"[...", "]"},
		{"[", "]"},
	} {
		for {
			i := strings.Index(p, pat.open)
			if i < 0 {
				break
			}
			j := strings.Index(p[i:], pat.close)
			if j < 0 {
				break
			}
			j += i
			name := p[i+len(pat.open) : j]
			p = p[:i] + "{" + name + "}" + p[j+len(pat.close):]
		}
	}
	return p
}
