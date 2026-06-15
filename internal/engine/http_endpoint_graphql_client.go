// GraphQL client-side (consumer) synthetic http_endpoint_call emission for
// cross-repo matching (epic #3607, issue #3608).
//
// The GraphQL SERVER side (synthesizeGraphQLResolvers / synthesizeStrawberry /
// the Elixir/PHP/Kotlin/Rust/Scala custom passes) emits one synthetic per
// resolver field under the Query / Mutation / Subscription roots:
//
//	http:GRAPHQL:/graphql/<RootType>/<field>     e.g. http:GRAPHQL:/graphql/Query/users
//
// using the synthetic verb GRAPHQL and a canonical path of
// `/graphql/<Operation>/<field>` (see http_endpoint_synthesis.go:3768 and
// :3114). The REST consumer pass (synthesizeFetchAxios) only handles REST
// clients (fetch/axios/wrappers), so a GraphQL CLIENT — Apollo, urql,
// graphql-request, raw gql template literals — emitted NOTHING at the
// operation level. The only thing recorded was the single `/graphql` mount
// from the ApolloClient URI (#1483), which can never link to the per-field
// server endpoints above.
//
// This pass closes that gap. For every GraphQL client operation it recognizes
// the operation type (query / mutation / subscription) and its ROOT FIELD(s),
// then emits a consumer-side http_endpoint_call whose ID EXACTLY matches the
// server endpoint shape:
//
//	useQuery(gql`query { users { id } }`)  →  http_endpoint_call  http:GRAPHQL:/graphql/Query/users
//
// Because the ID is identical to the server-side definition's ID, the existing
// Name-based cross-repo HTTP linker (links/http_pass.go) joins them on reindex
// with NO new linker code — exactly like the REST consumer pass.
//
// Recognized client idioms (JS/TS only — the dominant case):
//
//   - gql template literals bound to a const:
//     const GET_USERS = gql`query GetUsers { users { id } }`
//     → operation type Query, root field `users`.
//
//   - Apollo Client: useQuery(GET_USERS), useMutation(CREATE_USER),
//     useSubscription(SUB), client.query({ query: GET_USERS }),
//     client.mutate({ mutation: CREATE_USER }) → resolved via the gql const.
//
//   - graphql-request: request(endpoint, `{ users { id } }`),
//     gqlClient.request(QUERY).
//
//   - urql: useQuery({ query: GET_USERS }).
//
//   - Relay (react-relay): useLazyLoadQuery(graphql`query …`, vars),
//     usePreloadedQuery(QUERY, ref), useClientQuery(QUERY) — the operation
//     document is the FIRST positional argument (inline graphql`` template or a
//     const reference), resolved the same way as Apollo's bare-arg form. Relay
//     FRAGMENT hooks (useFragment / usePaginationFragment) take a fragment
//     document with no operation root field and emit nothing.
//
//   - Inline gql docs passed directly to a hook / client call.
//
// FETCHES edge: the enclosing function at the call site is recorded as
// `source_caller` (Function:<name>); ResolveHTTPEndpointHandlers turns that
// into the FETCHES edge — same mechanism as every other consumer pass.
//
// Honest-partial: when the gql document is imported from another file (the
// const is referenced but its body is not in this file), we still emit a
// client-call keyed on the OPERATION NAME with `unresolved=true`, so the
// operation is at least discoverable even though the root field could not be
// resolved here.

package engine

import (
	"regexp"
	"strings"

	"github.com/cajasmota/grafel/internal/engine/httproutes"
)

// gqlOpType maps a parsed GraphQL operation keyword to the server root-type
// name used in the synthetic path (`/graphql/<RootType>/<field>`).
func gqlOpType(keyword string) string {
	switch strings.ToLower(keyword) {
	case "mutation":
		return "Mutation"
	case "subscription":
		return "Subscription"
	default:
		// Anonymous shorthand `{ users { id } }` and `query …` both map to Query.
		return "Query"
	}
}

// gqlVerb is the synthetic HTTP verb used for all GraphQL operations, matching
// the server side (http:GRAPHQL:/graphql/...).
const gqlVerb = "GRAPHQL"

// gqlDocConstRe matches a `gql`-tagged (or `graphql`-tagged) template literal
// bound to a const/let/var:
//
//	const GET_USERS = gql`query GetUsers { users { id } }`;
//	export const CREATE = /* GraphQL */ graphql`mutation { createUser { id } }`;
//
// Capture groups: 1 = const name, 2 = template-literal body (without backticks).
var gqlDocConstRe = regexp.MustCompile(
	"(?s)(?:const|let|var)\\s+([A-Za-z_$][\\w$]*)\\s*=\\s*(?:gql|graphql)\\s*`([^`]*)`",
)

// gqlRootFieldsFromDoc parses a GraphQL document body and returns the operation
// root-type name (Query/Mutation/Subscription) plus the list of root field
// names selected directly under the top-level operation selection set.
//
// It handles:
//   - named & anonymous operations: `query Foo { … }`, `mutation { … }`,
//     `subscription { … }`, and the shorthand `{ users { id } }`.
//   - aliases (`alias: realField`) → the REAL field name (post-colon) is used,
//     since the server endpoint is keyed on the schema field, not the alias.
//   - arguments on the root field: `user(id: 5) { … }` → `user`.
//   - leading operation-level arguments / variable defs:
//     `query Foo($id: ID!) { user(id: $id) { … } }`.
//
// Returns (rootType, fields, ok). ok is false when no top-level selection set
// can be located.
func gqlRootFieldsFromDoc(doc string) (string, []string, bool) {
	doc = stripGraphQLComments(doc)

	// Find the operation keyword (query/mutation/subscription) that PRECEDES the
	// first top-level `{`. Shorthand `{ … }` with no keyword is an anonymous
	// query.
	open := strings.IndexByte(doc, '{')
	if open < 0 {
		return "", nil, false
	}
	header := strings.TrimSpace(doc[:open])
	rootType := "Query"
	if header != "" {
		// header looks like `query` / `query GetUsers` / `mutation Foo($x: Int)`.
		fields := strings.FieldsFunc(header, func(r rune) bool {
			return r == ' ' || r == '\t' || r == '\n' || r == '\r' || r == '(' || r == '{'
		})
		if len(fields) > 0 {
			rootType = gqlOpType(fields[0])
		}
	}

	// Extract the balanced top-level selection set starting at `open`.
	body, ok := gqlBalancedBlock(doc, open)
	if !ok {
		return rootType, nil, false
	}

	return rootType, gqlTopLevelFields(body), true
}

// gqlBalancedBlock returns the content between the `{` at openIdx and its
// matching `}` (exclusive of the braces). ok is false if unbalanced.
func gqlBalancedBlock(s string, openIdx int) (string, bool) {
	depth := 0
	for i := openIdx; i < len(s); i++ {
		switch s[i] {
		case '{':
			depth++
		case '}':
			depth--
			if depth == 0 {
				return s[openIdx+1 : i], true
			}
		}
	}
	return "", false
}

// gqlTopLevelFields returns the field names selected at depth 0 of a selection
// set body (the text between the operation's outermost braces). Nested
// selection sets, argument parens, and fragment spreads are skipped.
func gqlTopLevelFields(body string) []string {
	var out []string
	seen := map[string]bool{}
	i := 0
	n := len(body)
	for i < n {
		c := body[i]
		switch {
		case c == '{':
			// Skip a nested selection set entirely.
			if sub, ok := gqlBalancedBlock(body, i); ok {
				i += len(sub) + 2
				continue
			}
			i++
		case c == '(':
			// Skip an argument list.
			depth := 0
			for i < n {
				if body[i] == '(' {
					depth++
				} else if body[i] == ')' {
					depth--
					if depth == 0 {
						i++
						break
					}
				}
				i++
			}
		case c == '.':
			// Fragment spread `...Frag` — skip the dots; the following ident is a
			// fragment name, not a root field. We skip until whitespace.
			for i < n && (body[i] == '.' || isGQLNameByte(body[i])) {
				i++
			}
		case isGQLNameStart(c):
			// Read an identifier (possibly `alias: field`).
			start := i
			for i < n && isGQLNameByte(body[i]) {
				i++
			}
			name := body[start:i]
			// Look ahead for an alias colon: `alias : field`.
			j := i
			for j < n && (body[j] == ' ' || body[j] == '\t' || body[j] == '\n' || body[j] == '\r') {
				j++
			}
			if j < n && body[j] == ':' {
				// `name` is an alias; the real field is the next identifier.
				j++
				for j < n && (body[j] == ' ' || body[j] == '\t' || body[j] == '\n' || body[j] == '\r') {
					j++
				}
				rs := j
				for j < n && isGQLNameByte(body[j]) {
					j++
				}
				if j > rs {
					name = body[rs:j]
				}
				i = j
			}
			// Skip fragment-definition keyword bodies and the `on` keyword.
			if name == "fragment" || name == "on" {
				continue
			}
			if name != "" && !seen[name] {
				seen[name] = true
				out = append(out, name)
			}
		default:
			i++
		}
	}
	return out
}

// gqlDocIsConstDefinition reports whether the gql/graphql tag at tagStart is the
// right-hand side of a const/let/var assignment (a document DEFINITION) rather
// than an argument at a call site. It walks backward over whitespace from the
// tag; if the previous non-space char is `=`, it's a definition.
func gqlDocIsConstDefinition(content string, tagStart int) bool {
	i := tagStart - 1
	for i >= 0 && (content[i] == ' ' || content[i] == '\t' || content[i] == '\n' || content[i] == '\r') {
		i--
	}
	return i >= 0 && content[i] == '='
}

func isGQLNameStart(c byte) bool {
	return c == '_' || (c >= 'a' && c <= 'z') || (c >= 'A' && c <= 'Z')
}

func isGQLNameByte(c byte) bool {
	return isGQLNameStart(c) || (c >= '0' && c <= '9')
}

// stripGraphQLComments removes `#`-to-end-of-line comments from a GraphQL doc
// so they don't confuse the field walker.
func stripGraphQLComments(doc string) string {
	if !strings.Contains(doc, "#") {
		return doc
	}
	var b strings.Builder
	for _, line := range strings.Split(doc, "\n") {
		if idx := strings.IndexByte(line, '#'); idx >= 0 {
			line = line[:idx]
		}
		b.WriteString(line)
		b.WriteByte('\n')
	}
	return b.String()
}

// gqlRawTemplateRe matches a RAW (untagged) backtick template literal whose
// content begins (after optional whitespace) with a GraphQL operation keyword
// or the anonymous-shorthand `{`. Used for graphql-request `request(endpoint,
// `{ users { id } }`)`. Capture group 1 = the template-literal body.
var gqlRawTemplateRe = regexp.MustCompile(
	"(?s)`(\\s*(?:query|mutation|subscription)\\b[^`]*|\\s*\\{[^`]*)`",
)

// gqlInlineDocRe matches an inline gql/graphql-tagged template literal passed
// directly to a hook or client call, e.g. `useQuery(gql\`query { users {…} }\`)`
// or `request(endpoint, \`{ users { id } }\`)`. Capture group 1 = doc body.
var gqlInlineDocRe = regexp.MustCompile("(?s)(?:gql|graphql)\\s*`([^`]*)`")

// gqlHookCallRe matches Apollo/urql/Relay hook + client method call sites that
// take a gql document (by const reference or inline). Capture group 1 = the
// hook / method keyword, the remainder of the call is scanned for the doc
// reference.
//
//	Apollo / urql:
//	  useQuery( / useMutation( / useSubscription( / useLazyQuery( /
//	  useSuspenseQuery( / client.query( / client.mutate( / client.subscribe( /
//	  .request(
//	Relay (react-relay / relay-runtime): the query DOCUMENT is the FIRST
//	positional argument — `useLazyLoadQuery(graphql\`…\`, vars)` inline, or
//	`const Q = graphql\`…\`; usePreloadedQuery(Q, ref)` by const reference —
//	so it is resolved identically to Apollo's bare-arg `useQuery(GET_USERS)`
//	form via gqlConstRefRe. Only the OPERATION-bearing Relay hooks are listed;
//	the FRAGMENT hooks (useFragment / usePaginationFragment /
//	useRefetchableFragment) take a fragment document (no operation root field)
//	and are deliberately EXCLUDED so they emit nothing (matching the
//	fragment-only negative).
//	  useLazyLoadQuery( / usePreloadedQuery( / useClientQuery(
var gqlHookCallRe = regexp.MustCompile(
	`\b(useQuery|useMutation|useSubscription|useLazyQuery|useSuspenseQuery|useSubscriptionQuery|useLazyLoadQuery|usePreloadedQuery|useClientQuery|query|mutate|subscribe|request)\s*\(`,
)

// gqlConstRefRe extracts the first plausible const identifier referenced inside
// a hook/client call's argument list, looking at both bare-arg form
// (`useQuery(GET_USERS)`) and object form (`useQuery({ query: GET_USERS })` /
// `{ mutation: CREATE }`). Capture group 1 = the identifier.
var gqlConstRefRe = regexp.MustCompile(
	`(?:query|mutation|subscription)\s*:\s*([A-Za-z_$][\w$]*)|^\s*([A-Za-z_$][\w$]*)`,
)

// synthesizeGraphQLClientCalls scans a JS/TS file for GraphQL client operations
// and emits one http_endpoint_call per (operation, root field), keyed to match
// the server endpoint shape http:GRAPHQL:/graphql/<RootType>/<field>, plus a
// source_caller for the FETCHES edge.
//
// It is invoked from synthesizeFetchAxios so it shares the JS/TS dispatch.
func synthesizeGraphQLClientCalls(content string, funcs []jsFuncSpan, emit emitFn) {
	// File-signal gate: require a GraphQL client marker so this is a no-op on
	// ordinary REST/React files.
	if !strings.Contains(content, "gql`") && !strings.Contains(content, "graphql`") &&
		!strings.Contains(content, "useQuery") && !strings.Contains(content, "useMutation") &&
		!strings.Contains(content, "useSubscription") &&
		!strings.Contains(content, ".request(") && !strings.Contains(content, "request(") {
		return
	}

	// Build a symbol table of gql-doc consts → parsed (rootType, fields).
	type gqlDoc struct {
		rootType string
		fields   []string
		ok       bool
	}
	docs := map[string]gqlDoc{}
	for _, m := range gqlDocConstRe.FindAllStringSubmatch(content, -1) {
		name := m[1]
		rt, fields, ok := gqlRootFieldsFromDoc(m[2])
		docs[name] = gqlDoc{rootType: rt, fields: fields, ok: ok && len(fields) > 0}
	}

	// emitDoc emits one client-call per root field for a resolved gql doc.
	emitDoc := func(rt string, fields []string, caller string) {
		for _, f := range fields {
			path := "/graphql/" + rt + "/" + f
			canonical := httproutes.Canonicalize(httproutes.FrameworkExpress, path)
			emit(gqlVerb, canonical, "graphql_client", "Function", caller)
		}
	}

	// Track which call-site offsets we've already attributed to a doc, so the
	// inline-doc scan below doesn't double-count an inline gql passed to a hook.
	handledInline := map[int]bool{}

	// 1) Hook / client-method call sites referencing a gql const (or inline doc).
	for _, hm := range gqlHookCallRe.FindAllStringSubmatchIndex(content, -1) {
		callStart := hm[0]
		parenOpen := hm[1] - 1 // the `(` consumed by the regex
		// findMatchingParenAfter assumes depth 1 (parens already open), so start
		// scanning at the byte AFTER the opening `(`.
		parenClose := findMatchingParenAfter(content, parenOpen+1, 8000)
		if parenClose < 0 {
			continue
		}
		args := content[parenOpen+1 : parenClose]
		caller := enclosingJSFuncAt(funcs, callStart)
		keyword := content[hm[2]:hm[3]]

		// Inline gql doc inside the call?
		if im := gqlInlineDocRe.FindStringSubmatchIndex(args); im != nil {
			handledInline[parenOpen+1+im[0]] = true
			rt, fields, ok := gqlRootFieldsFromDoc(args[im[2]:im[3]])
			if ok && len(fields) > 0 {
				emitDoc(rt, fields, caller)
				continue
			}
		}

		// graphql-request `request(endpoint, `{ users { id } }`)` passes the
		// document as a RAW (untagged) template literal — recognise it by the
		// `request` keyword + a backtick template-literal argument whose content
		// parses as a GraphQL operation.
		if keyword == "request" {
			if rm := gqlRawTemplateRe.FindStringSubmatchIndex(args); rm != nil {
				rt, fields, ok := gqlRootFieldsFromDoc(args[rm[2]:rm[3]])
				if ok && len(fields) > 0 {
					emitDoc(rt, fields, caller)
					continue
				}
			}
		}

		// Const reference inside the call args?
		if cm := gqlConstRefRe.FindStringSubmatch(args); cm != nil {
			ref := cm[1]
			if ref == "" {
				ref = cm[2]
			}
			if ref == "" {
				continue
			}
			if d, found := docs[ref]; found && d.ok {
				emitDoc(d.rootType, d.fields, caller)
				continue
			}
			// Honest-partial: the const is referenced but its gql body is not in
			// this file (imported cross-file, so not in `docs`). Emit a
			// discoverable, unresolved client-call keyed on the operation NAME so
			// the operation is at least visible; the root field cannot be
			// resolved here.
			if _, isLocal := docs[ref]; !isLocal && looksLikeGQLOperationConst(ref) {
				path := "/graphql/Query/" + ref
				canonical := httproutes.Canonicalize(httproutes.FrameworkExpress, path)
				emit(gqlVerb, canonical, "graphql_client_unresolved", "Function", caller)
			}
		}
	}

	// 2) Standalone inline gql docs not already attributed to a hook call
	//    (e.g. `const r = await request(endpoint, gql\`{ users { id } }\`)` whose
	//    outer call keyword isn't one we matched, or top-level doc usage).
	//
	//    Skip gql docs that are merely a const DEFINITION
	//    (`const GET_USERS = gql\`…\``). Those are not call sites — they are
	//    consumed via their const reference in section 1, where the caller is
	//    the enclosing component. Emitting here would mis-attribute the call to
	//    the top-level (empty caller) and double-count the operation.
	for _, im := range gqlInlineDocRe.FindAllStringSubmatchIndex(content, -1) {
		if handledInline[im[0]] {
			continue
		}
		if gqlDocIsConstDefinition(content, im[0]) {
			continue
		}
		rt, fields, ok := gqlRootFieldsFromDoc(content[im[2]:im[3]])
		if !ok || len(fields) == 0 {
			continue
		}
		caller := enclosingJSFuncAt(funcs, im[0])
		emitDoc(rt, fields, caller)
	}
}

// looksLikeGQLOperationConst heuristically decides whether a referenced
// identifier is plausibly a gql-document constant (UPPER_SNAKE or a name
// containing Query/Mutation/Subscription), used only on the honest-partial
// cross-file path to avoid attributing arbitrary identifiers.
func looksLikeGQLOperationConst(name string) bool {
	if name == "" {
		return false
	}
	upper := strings.ToUpper(name) == name && strings.ContainsAny(name, "ABCDEFGHIJKLMNOPQRSTUVWXYZ")
	if upper {
		return true
	}
	return strings.Contains(name, "Query") || strings.Contains(name, "Mutation") ||
		strings.Contains(name, "Subscription")
}
