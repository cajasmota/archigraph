// tRPC procedure resolver endpoint synthesis (#2693, follow-up to #2678
// audit #2687). tRPC composes routers via an object-literal DSL:
//
//	const userRouter = router({
//	  list: publicProcedure.query(({ ctx }) => { ... }),
//	  byId: publicProcedure.input(z.object({ id: z.string() })).query(({ input }) => { ... }),
//	  create: protectedProcedure.input(...).mutation(({ input }) => { ... }),
//	});
//
//	const appRouter = router({ users: userRouter, posts: postsRouter });
//
// Each leaf procedure becomes an addressable endpoint identified by its
// dotted path (`users.list`, `users.byId`, `users.create`). Verb mapping:
//
//	.query(...)        → GET
//	.mutation(...)     → POST
//	.subscription(...) → SUBSCRIBE
//
// This synthesizer is intentionally same-file only. If `userRouter` is
// imported from another module the dotted-path composition is not
// performed in v1 — we emit endpoints only for procedures whose router
// definition lives in the file we are scanning. Each router defined
// locally is treated as a root unless another local router references it
// as a child (in which case it composes with the parent's key prefix).
//
// The source_line stamped on each synthetic is the 1-based line of the
// `.query(` / `.mutation(` / `.subscription(` call — i.e. the resolver
// arrow function's def line. We do not emit a `source_handler` because
// the leaf is an inline arrow expression with no addressable symbol;
// without a handler ref the shared resolver leaves source_file /
// source_line untouched (it short-circuits on the empty ref), preserving
// the precise attribution this synthesizer produces.
package engine

import (
	"regexp"
	"strings"
)

// trpcRouterDeclRe matches a top-level router declaration:
//
//	const <name> = router({ ... })
//	const <name> = t.router({ ... })
//	export const <name> = createTRPCRouter({ ... })
//
// Group 1 = router variable name. The opening `{` of the object literal
// is located after the match end so the body span can be extracted with
// balanced-brace walking (`trpcRouterBody`).
var trpcRouterDeclRe = regexp.MustCompile(
	`(?:^|\n)\s*(?:export\s+)?(?:const|let|var)\s+([A-Za-z_$][\w$]*)\s*` +
		`(?::\s*[^=]+?)?` + // optional TS type annotation
		`=\s*(?:[A-Za-z_$][\w$]*\s*\.\s*)?(?:router|createTRPCRouter)\s*\(\s*\{`,
)

// trpcVerbRe locates a `.query(` / `.mutation(` / `.subscription(` call
// inside a property value so the verb and the call-site line can be
// extracted. The leading `.` anchors the match so a bare `query` /
// `mutation` identifier in another context does not false-fire.
var trpcVerbRe = regexp.MustCompile(`\.\s*(query|mutation|subscription)\s*\(`)

// trpcIdentRe matches a bare identifier — used to detect a nested-router
// reference (value of a property is just another router variable name).
var trpcIdentRe = regexp.MustCompile(`^[A-Za-z_$][\w$]*$`)

// trpcVerbFor maps a tRPC builder method name to the canonical grafel
// HTTP verb used in the synthetic ID. The `.subscription` case maps to
// `SUBSCRIBE` rather than reusing an HTTP verb, mirroring the convention
// used for WebSocket-style transports elsewhere in the synthesis layer.
func trpcVerbFor(method string) string {
	switch method {
	case "query":
		return "GET"
	case "mutation":
		return "POST"
	case "subscription":
		return "SUBSCRIBE"
	}
	return ""
}

// trpcRouter is the in-file representation of a parsed router definition.
// `body` is the raw object-literal text (excluding the outer braces).
// `bodyOffset` is the absolute offset of the first byte of `body` inside
// the original file content, needed so per-property line numbers can be
// computed against the file.
type trpcRouter struct {
	name       string
	body       string
	bodyOffset int
}

// trpcProperty is one top-level entry in a router's object literal.
// `key` is the property name, `value` is the raw RHS text, and
// `valueOffset` is the absolute offset of the first non-space byte of the
// value inside the original file content (used to locate `.query(` lines).
type trpcProperty struct {
	key         string
	value       string
	valueOffset int
}

// synthesizeTRPC emits one http_endpoint_definition per leaf procedure
// reachable from a router defined in this file. Verb mapping follows
// trpcVerbFor; the canonical path is the dotted procedure path
// (`users.list`, etc.) with no leading slash — tRPC procedures are RPC
// calls keyed by name, not URL routes. The synthetic ID is therefore
// `http:GET:users.list` and friends.
//
// Same-file resolution only: a child router referenced by name is
// composed if and only if its definition lives in this file. Imported
// routers are not chased — v1 of the synthesiser deliberately scopes the
// dotted-path composition to a single translation unit.
func synthesizeTRPC(content string, emit emitDefFn) {
	if !trpcFileLooksLikeTRPC(content) {
		return
	}
	routers := parseTRPCRouters(content)
	if len(routers) == 0 {
		return
	}

	// Identify which routers are referenced as children of another router
	// (by bare identifier on the RHS of a property). Anything not
	// referenced is a root and gets walked with its own variable name
	// elided from the emitted path — i.e. its top-level properties land
	// at the root of the dotted path. Referenced routers are reached
	// transitively during the parent's walk with the correct prefix.
	byName := map[string]*trpcRouter{}
	for i := range routers {
		byName[routers[i].name] = &routers[i]
	}

	// First pass: find every router referenced by name as a child of
	// another router. These are NOT walked as roots — they will be
	// reached during the parent's walk with the correct dotted prefix.
	// Without this guard a fixture with `userRouter` + `appRouter`
	// composing `{ users: userRouter }` would emit BOTH `users.list`
	// (via appRouter) and `list` (via userRouter), and the second
	// emission would collide with the first on a same-file procedure
	// name like `list`.
	referenced := map[string]bool{}
	for i := range routers {
		for _, p := range parseTRPCProperties(routers[i]) {
			trimmed := strings.TrimSpace(p.value)
			if trpcIdentRe.MatchString(trimmed) {
				if _, ok := byName[trimmed]; ok {
					referenced[trimmed] = true
				}
			}
		}
	}

	for i := range routers {
		if referenced[routers[i].name] {
			continue
		}
		walkTRPCRouter(content, &routers[i], "", byName, map[string]bool{}, emit)
	}
}

// trpcFileLooksLikeTRPC is a cheap pre-filter: a file that doesn't mention
// `router(` or `createTRPCRouter(` cannot possibly be a tRPC server module.
// Without this fast path the regex passes would scan every JS/TS file.
func trpcFileLooksLikeTRPC(content string) bool {
	return strings.Contains(content, "router(") ||
		strings.Contains(content, "createTRPCRouter(") ||
		strings.Contains(content, ".router(")
}

// parseTRPCRouters scans the file for router declarations and returns one
// trpcRouter per match, with the object-literal body span captured via
// balanced-brace walking. Declarations whose `{...}` literal cannot be
// closed before EOF are skipped (defensive against malformed input).
func parseTRPCRouters(content string) []trpcRouter {
	var out []trpcRouter
	for _, m := range trpcRouterDeclRe.FindAllStringSubmatchIndex(content, -1) {
		if len(m) < 4 {
			continue
		}
		name := content[m[2]:m[3]]
		// The `{` that the regex matches sits at `m[1]-1` (the last byte
		// of the overall match). Body content starts at `m[1]` and runs
		// to the matching `}`.
		bodyStart := m[1]
		bodyEnd, ok := matchClosingBrace(content, bodyStart)
		if !ok {
			continue
		}
		out = append(out, trpcRouter{
			name:       name,
			body:       content[bodyStart:bodyEnd],
			bodyOffset: bodyStart,
		})
	}
	return out
}

// walkTRPCRouter recursively enumerates leaf procedures reachable from
// `r` and emits one synthetic per leaf. `prefix` accumulates the dotted
// path; the first call uses "" so the leaf key lands at the root of the
// emitted path. `seen` guards against accidental cycles in pathological
// fixtures (a router that names itself as a child).
func walkTRPCRouter(
	content string,
	r *trpcRouter,
	prefix string,
	byName map[string]*trpcRouter,
	seen map[string]bool,
	emit emitDefFn,
) {
	if seen[r.name] {
		return
	}
	seen[r.name] = true
	defer delete(seen, r.name)

	for _, p := range parseTRPCProperties(*r) {
		path := joinTRPCPath(prefix, p.key)

		// Nested-router reference: the property value is a bare identifier
		// that names another router defined in this file. Recurse with the
		// current key as the next path segment.
		trimmed := strings.TrimSpace(p.value)
		if trpcIdentRe.MatchString(trimmed) {
			if child, ok := byName[trimmed]; ok {
				walkTRPCRouter(content, child, path, byName, seen, emit)
				continue
			}
			// Bare identifier with no local definition — imported router.
			// Skip per the same-file rule.
			continue
		}

		// Leaf procedure: locate the `.query(` / `.mutation(` /
		// `.subscription(` call inside the property value. The call's
		// `.` is the line we attribute to (arrow function's def line is
		// the same line in idiomatic tRPC code; multi-line procedures
		// still attribute to the verb-call line, which is the most
		// stable navigation target across formatting styles).
		vm := trpcVerbRe.FindStringSubmatchIndex(p.value)
		if vm == nil {
			continue
		}
		verb := trpcVerbFor(p.value[vm[2]:vm[3]])
		if verb == "" {
			continue
		}
		defLine := lineOfOffset(content, p.valueOffset+vm[0])
		// Path is the dotted form — NOT canonicalised through httproutes
		// (which would prepend a `/` and treat the dotted form as a URL).
		// tRPC procedures are RPC identifiers, not URL routes; the
		// synthetic ID `http:GET:users.list` is the keying convention.
		emit(verb, path, "trpc", "", "", defLine)
	}
}

// parseTRPCProperties returns the top-level entries of `r.body` as
// (key, value, value-offset) triples. Nested object literals, arrays,
// parens, template strings, and string literals are tracked so commas
// inside a procedure's `.input(z.object({...}))` chain do not split a
// property prematurely.
func parseTRPCProperties(r trpcRouter) []trpcProperty {
	var out []trpcProperty
	src := r.body
	i := 0
	n := len(src)
	for i < n {
		// Skip whitespace, commas, and line comments / block comments
		// between properties. We treat `//...\n` and `/*...*/` as
		// whitespace so well-commented router definitions still parse.
		for i < n {
			c := src[i]
			if c == ' ' || c == '\t' || c == '\n' || c == '\r' || c == ',' {
				i++
				continue
			}
			if c == '/' && i+1 < n && src[i+1] == '/' {
				for i < n && src[i] != '\n' {
					i++
				}
				continue
			}
			if c == '/' && i+1 < n && src[i+1] == '*' {
				i += 2
				for i+1 < n && !(src[i] == '*' && src[i+1] == '/') {
					i++
				}
				if i+1 < n {
					i += 2
				}
				continue
			}
			break
		}
		if i >= n {
			break
		}
		// Parse the key. tRPC routers accept either bare identifier keys
		// or string-literal keys; both are common in real code. Computed
		// keys (`[expr]: ...`) and spread (`...other`) are intentionally
		// ignored — they cannot resolve to a static dotted path.
		keyStart := i
		var key string
		switch c := src[i]; {
		case c == '"' || c == '\'' || c == '`':
			end, ok := skipString(src, i)
			if !ok {
				return out
			}
			key = src[i+1 : end-1]
			i = end
		case isIdentStart(c):
			j := i + 1
			for j < n && isIdentChar(src[j]) {
				j++
			}
			key = src[i:j]
			i = j
		default:
			// Unexpected token (e.g. `[` for computed key, `.` for
			// spread). Skip until the next comma at depth 0 to resume
			// the next property cleanly.
			i = skipToTopLevelComma(src, keyStart)
			continue
		}
		// Skip whitespace and the colon. A property without a colon
		// (shorthand `{ list }`) is treated as `list: list` — the value
		// is the identifier itself, which is the nested-router shape.
		shorthand := true
		for i < n && (src[i] == ' ' || src[i] == '\t') {
			i++
		}
		if i < n && src[i] == ':' {
			shorthand = false
			i++
			for i < n && (src[i] == ' ' || src[i] == '\t' || src[i] == '\n' || src[i] == '\r') {
				i++
			}
		}
		valueStart := i
		if shorthand {
			// Shorthand: the value is the key. Emit a synthetic value
			// span pointing back at the key text so nested-router
			// detection works uniformly.
			out = append(out, trpcProperty{
				key:         key,
				value:       key,
				valueOffset: r.bodyOffset + keyStart,
			})
			continue
		}
		// Walk to the next top-level comma to capture the value span.
		valueEnd := skipToTopLevelComma(src, i)
		out = append(out, trpcProperty{
			key:         key,
			value:       src[valueStart:valueEnd],
			valueOffset: r.bodyOffset + valueStart,
		})
		i = valueEnd
	}
	return out
}

// joinTRPCPath concatenates two dotted-path segments. An empty prefix or
// segment passes the other through verbatim; otherwise the two are joined
// by a `.`.
func joinTRPCPath(prefix, segment string) string {
	switch {
	case prefix == "" && segment == "":
		return ""
	case prefix == "":
		return segment
	case segment == "":
		return prefix
	}
	return prefix + "." + segment
}

// matchClosingBrace returns the offset of the byte immediately AFTER the
// `}` that closes the `{` at position `openAfter-1` (i.e. body content
// starts at `openAfter`). Tracks string literals (single, double,
// backtick) and nested braces so `.input(z.object({ id: z.string() }))`
// inside a procedure body doesn't terminate the router literal.
//
// Returns ok=false on EOF before a matching brace is found.
func matchClosingBrace(src string, openAfter int) (int, bool) {
	depth := 1
	i := openAfter
	n := len(src)
	for i < n {
		switch src[i] {
		case '"', '\'', '`':
			end, ok := skipString(src, i)
			if !ok {
				return 0, false
			}
			i = end
			continue
		case '/':
			if i+1 < n && src[i+1] == '/' {
				for i < n && src[i] != '\n' {
					i++
				}
				continue
			}
			if i+1 < n && src[i+1] == '*' {
				i += 2
				for i+1 < n && !(src[i] == '*' && src[i+1] == '/') {
					i++
				}
				if i+1 < n {
					i += 2
				}
				continue
			}
		case '{':
			depth++
		case '}':
			depth--
			if depth == 0 {
				return i, true
			}
		}
		i++
	}
	return 0, false
}

// skipString advances past a JS string literal starting at `src[i]` (which
// must be `'`, `"`, or backtick) and returns the offset of the byte
// immediately AFTER the closing quote. Template literals with `${...}`
// expressions are tracked with a nested brace counter; embedded expressions
// can themselves contain strings, recursing through this same routine.
// Backslash escapes are honoured in all quote styles.
//
// Returns ok=false on unterminated strings (defensive — should not happen
// in well-formed source but we don't want to panic on truncated input).
func skipString(src string, i int) (int, bool) {
	quote := src[i]
	n := len(src)
	i++
	for i < n {
		c := src[i]
		if c == '\\' {
			if i+1 >= n {
				return 0, false
			}
			i += 2
			continue
		}
		if c == quote {
			return i + 1, true
		}
		if quote == '`' && c == '$' && i+1 < n && src[i+1] == '{' {
			// Skip a `${ ... }` template expression with its own
			// balanced-brace counter. The expression body can contain
			// strings, comments, and nested braces.
			i += 2
			depth := 1
			for i < n && depth > 0 {
				switch src[i] {
				case '"', '\'', '`':
					end, ok := skipString(src, i)
					if !ok {
						return 0, false
					}
					i = end
					continue
				case '{':
					depth++
				case '}':
					depth--
					if depth == 0 {
						i++
						break
					}
				}
				if depth == 0 {
					break
				}
				i++
			}
			continue
		}
		i++
	}
	return 0, false
}

// skipToTopLevelComma advances from `i` to the offset of the next comma
// that sits at brace/paren/bracket depth 0 inside `src`, or to `len(src)`
// if no such comma exists. Strings and comments are skipped so a comma
// inside `z.object({a, b})` or `'foo, bar'` does not terminate the value.
func skipToTopLevelComma(src string, i int) int {
	n := len(src)
	depth := 0
	for i < n {
		switch src[i] {
		case '"', '\'', '`':
			end, ok := skipString(src, i)
			if !ok {
				return n
			}
			i = end
			continue
		case '/':
			if i+1 < n && src[i+1] == '/' {
				for i < n && src[i] != '\n' {
					i++
				}
				continue
			}
			if i+1 < n && src[i+1] == '*' {
				i += 2
				for i+1 < n && !(src[i] == '*' && src[i+1] == '/') {
					i++
				}
				if i+1 < n {
					i += 2
				}
				continue
			}
		case '(', '[', '{':
			depth++
		case ')', ']', '}':
			depth--
			if depth < 0 {
				return i
			}
		case ',':
			if depth == 0 {
				return i
			}
		}
		i++
	}
	return n
}

// isIdentStart reports whether c can begin a JS identifier (ASCII subset
// — non-ASCII identifiers exist in JS but are vanishingly rare in tRPC
// router keys and not worth the regex compile cost here).
func isIdentStart(c byte) bool {
	return (c >= 'a' && c <= 'z') ||
		(c >= 'A' && c <= 'Z') ||
		c == '_' || c == '$'
}
