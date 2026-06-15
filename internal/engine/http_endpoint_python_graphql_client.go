// Python GraphQL client-side (consumer) operation synthesis
// (epic #3607, issue #3608; mirrors the JS/TS pass in
// http_endpoint_graphql_client.go and the Dart pass in
// http_endpoint_dart_graphql_client.go).
//
// The dominant GraphQL client for Python is the `gql` package
// (graphql-python/gql). A GraphQL operation document is built by wrapping a
// document string in the `gql(...)` parser function, then executed against a
// transport-backed client/session:
//
//	from gql import gql, Client
//	GET_USER = gql("""query GetUser { user { id name } }""")
//	result = client.execute(GET_USER)
//
//	await session.execute(
//	    gql('''mutation AddUser { createUser(name: "x") { id } }''')
//	)
//
//	client.execute(gql("{ users { id } }"))   # anonymous shorthand
//
// Before this pass these produced no operation entity and no cross-repo link:
// the Python REST client pass (http_endpoint_python_client.go) only recognises
// requests / httpx / aiohttp / urllib verb calls, so a Python service or script
// talking to a GraphQL backend was invisible at the operation level. (Some gql
// clients POST to `/graphql` via httpx under the hood, but that single mount can
// never link to the per-field server endpoints.)
//
// This pass recognises the `gql(<string-literal>)` parser call — whether bound
// to a module-level/local constant and referenced by name, or inlined directly
// at the `execute(...)` call site — parses its operation type
// (query/mutation/subscription) and ROOT FIELD(s) via the shared
// gqlRootFieldsFromDoc parser, and emits one synthetic http_endpoint_call per
// (operation, root field) keyed to EXACTLY match the GraphQL server endpoint
// shape produced by every grafel GraphQL server synthesizer
// (http_endpoint_synthesis.go, the Strawberry / Graphene / Ariadne / gqlgen /
// graphql-ruby / HotChocolate / Spring-GraphQL / caliban / async-graphql / …
// passes):
//
//	gql("query { users { id } }")  →  http_endpoint_call  http:GRAPHQL:/graphql/Query/users
//
// Because the id is identical to the server-side definition's id, the existing
// Name-based cross-repo HTTP linker joins them on reindex with NO new linker
// code — exactly like every other consumer pass (REST, Dart-GraphQL,
// Swift-GraphQL).
//
// FETCHES edge: the enclosing function at the gql(...) site is recorded as the
// `source_caller` (Function:<name>). For a TOP-LEVEL constant (whose gql site
// has no enclosing function) the caller is resolved from the function that
// REFERENCES the const (`client.execute(GET_USER)`) — the real consumer of the
// operation — mirroring the Dart pass.
//
// Honest-partial: a gql doc whose body cannot be parsed into a top-level
// selection set (e.g. fragment-only documents, or dynamically-composed query
// strings) emits nothing rather than a wrong endpoint. Documents imported from
// another module and referenced only by name (no gql(...) body in this file)
// are not resolved here — they are picked up in the file that defines them.

package engine

import (
	"regexp"

	"github.com/cajasmota/grafel/internal/engine/httproutes"
)

// pyGqlDocRe matches a `gql(<string-literal>)` call in Python and captures the
// document body. Python string literals come in several flavours, all handled
// by the alternation below (optional `r`/`f`/`b` string prefix in every case,
// case-insensitive, so `r"""..."""` and `f'...'` are tolerated):
//
//	gql("""query { ... }""")   triple-quoted double (the dominant gql form)
//	gql('''query { ... }''')   triple-quoted single
//	gql("query { ... }")       double-quoted
//	gql('query { ... }')       single-quoted
//
// Capture groups (mutually exclusive, exactly one is non-empty):
//
//	1 = triple-double body, 2 = triple-single body,
//	3 = double-quote body,  4 = single-quote body.
//
// (?s) so the `.`-equivalents span newlines inside a multi-line document.
var pyGqlDocRe = regexp.MustCompile(
	`(?s)\bgql\s*\(\s*[rRfFbB]?(?:"""(.*?)"""|'''(.*?)'''|"((?:[^"\\]|\\.)*)"|'((?:[^'\\]|\\.)*)')`,
)

// pyGqlConstDefRe captures a Python assignment whose initializer is a gql(...)
// document, capturing the constant NAME (group 1). The gql body is re-parsed
// from the same span via pyGqlDocRe; this regex only anchors the name so a
// TOP-LEVEL const (whose gql site has no enclosing function) can have its
// FETCHES caller resolved from the REFERENCE site instead. A leading type
// annotation (`GET_USER: DocumentNode = gql(...)`) is tolerated.
var pyGqlConstDefRe = regexp.MustCompile(
	`(?m)^[ \t]*([A-Za-z_]\w*)(?:[ \t]*:[ \t]*[\w.\[\], ]+)?[ \t]*=[ \t]*gql\s*\(`,
)

// synthesizePyGraphQLClient scans a Python file for `gql` package GraphQL
// operations and emits one http_endpoint_call per (operation, root field),
// keyed to match the server endpoint shape http:GRAPHQL:/graphql/<RootType>/
// <field>, plus a FETCHES edge from the enclosing Python function.
//
// It is dispatched from the `case "python":` block of applyHTTPEndpointSynthesis
// alongside the REST client pass, using the same runtime-aware emitter so it
// produces the identical http_endpoint_call entity + FETCHES edge as every
// other consumer pass.
func synthesizePyGraphQLClient(content string, emit pyClientEmitFn) {
	// File-signal gate: require the `gql(` parser marker so this is a no-op on
	// ordinary REST/script Python files. (`gql(` is the parser-call form unique
	// to the gql package; the REST Python pass keys off requests/httpx/aiohttp
	// instead, so the two passes never collide.)
	if !regexp.MustCompile(`\bgql\s*\(`).MatchString(content) {
		return
	}

	funcs := indexPyEnclosingFunctions(content)

	// emitDoc emits one client-call per resolved root field for a gql document.
	emitDoc := func(rt string, fields []string, caller string) {
		for _, f := range fields {
			path := "/graphql/" + rt + "/" + f
			canonical := httproutes.Canonicalize(httproutes.FrameworkExpress, path)
			// runtimeDynamic=false: the operation root field is statically
			// resolved from the literal gql document, so the endpoint is concrete.
			emit(gqlVerb, canonical, "graphql_client", "Function", caller, false)
		}
	}

	// pyGqlDocBody returns the document body from a pyGqlDocRe submatch
	// (exactly one of the four quote-flavour groups is populated).
	pyGqlDocBody := func(m []string) string {
		for g := 1; g <= 4; g++ {
			if m[g] != "" {
				return m[g]
			}
		}
		return ""
	}

	// Record each gql const-definition span → the const NAME, so a gql site that
	// resolves to an empty enclosing function (a top-level const) can have its
	// FETCHES caller resolved from a reference site below. pyGqlConstDefRe spans
	// from the line start through the `(` of `gql(`, so the gql token sits inside
	// [start, end).
	type constSpan struct {
		start, end int
		name       string
	}
	var constSpans []constSpan
	for _, dm := range pyGqlConstDefRe.FindAllStringSubmatchIndex(content, -1) {
		constSpans = append(constSpans, constSpan{start: dm[0], end: dm[1], name: content[dm[2]:dm[3]]})
	}
	constNameForGqlSite := func(gqlOff int) string {
		for _, cs := range constSpans {
			if cs.start <= gqlOff && gqlOff < cs.end {
				return cs.name
			}
		}
		return ""
	}

	// refCallerForConst returns the enclosing function of the FIRST reference to
	// `name` (`client.execute(GET_USER)` or a bare `GET_USER` argument) that
	// lands inside some function, so a top-level gql const is attributed to its
	// consumer.
	refCallerForConst := func(name string) string {
		if name == "" {
			return ""
		}
		ref := regexp.MustCompile(`\b` + regexp.QuoteMeta(name) + `\b`)
		for _, loc := range ref.FindAllStringIndex(content, -1) {
			// Skip the definition site itself (it sits inside a const span).
			inDef := false
			for _, cs := range constSpans {
				if cs.start <= loc[0] && loc[0] < cs.end {
					inDef = true
					break
				}
			}
			if inDef {
				continue
			}
			if c := enclosingPyFuncAt(funcs, loc[0]); c != "" {
				return c
			}
		}
		return ""
	}

	// Every gql(...) site, in source order, handled exactly once. The caller is
	// the enclosing Python function; for a top-level const def (no enclosing
	// function) the caller is resolved from the const's reference site.
	for _, m := range pyGqlDocRe.FindAllStringSubmatchIndex(content, -1) {
		bodyGroups := pyGqlDocRe.FindStringSubmatch(content[m[0]:m[1]])
		if bodyGroups == nil {
			continue
		}
		doc := pyGqlDocBody(bodyGroups)
		if doc == "" {
			continue
		}
		rt, fields, ok := gqlRootFieldsFromDoc(doc)
		if !ok || len(fields) == 0 {
			continue
		}
		caller := enclosingPyFuncAt(funcs, m[0])
		if caller == "" {
			// Top-level gql site: if it is a const definition, attribute the
			// operation to the function that references the const.
			caller = refCallerForConst(constNameForGqlSite(m[0]))
		}
		emitDoc(rt, fields, caller)
	}
}
