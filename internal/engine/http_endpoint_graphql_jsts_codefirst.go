package engine

import (
	"regexp"
	"strings"

	"github.com/cajasmota/grafel/internal/engine/httproutes"
)

// ---------------------------------------------------------------------------
// Pothos + TypeGraphQL (JS/TS code-first GraphQL servers) — #3619 (epic #3607)
// ---------------------------------------------------------------------------
//
// This file completes the JS/TS GraphQL-server family beyond Apollo / graphql-
// tools resolver maps (synthesizeGraphQLResolvers, #1422) and NestJS GraphQL.
// Both Pothos and TypeGraphQL are *code-first*: the schema is built from
// TypeScript code rather than an SDL string, so the resolver-map regexes in
// synthesizeGraphQLResolvers never fire on them.
//
// Both synthesizers emit the SAME canonical operation-endpoint shape used by
// gqlgen (Go), Apollo (JS), Strawberry (Python), HotChocolate (C#), and the
// Java spring-graphql / DGS servers:
//
//	http:GRAPHQL:/graphql/<RootType>/<field>
//
// where RootType is Query / Mutation / Subscription. Emitting the identical id
// shape is what lets the GraphQL client-link synthesizer (#3667) and the
// cross-repo linker join these servers to their consumers.

// ---------------------------------------------------------------------------
// Pothos — builder.queryField / mutationField / subscriptionField and the
// queryType / mutationType / subscriptionType field-map forms.
// ---------------------------------------------------------------------------
//
// Pothos (https://pothos-graphql.dev) builds a schema from a `builder` object:
//
//	const builder = new SchemaBuilder<...>({})
//
//	builder.queryField('users', (t) => t.field({ ... resolve: () => ... }))
//	builder.mutationField('createUser', (t) => ...)
//	builder.subscriptionField('userAdded', (t) => ...)
//
//	builder.queryType({
//	  fields: (t) => ({
//	    me: t.field({ ... }),
//	    users: t.field({ ... }),
//	  }),
//	})
//
// The resolver in every form is an inline arrow with no addressable handler
// symbol, so — exactly like the Apollo resolver-map synthesizer — we emit the
// operation endpoints with NO source_handler. That lands them on the resolver's
// NoHandlerProp keep-path so they survive Phase-2 resolution instead of being
// dropped as handler_dropped.

// pothosRootFieldRe matches the single-field registration form
// `builder.queryField('users', ...)` / `builder.mutationField("createUser", ...)`
// / `builder.subscriptionField('userAdded', ...)`.
//
// Capture groups:
//
//	1 = root verb (query | mutation | subscription)
//	2 = field name (string-literal first argument)
var pothosRootFieldRe = regexp.MustCompile(
	`\bbuilder\s*\.\s*(query|mutation|subscription)Field\s*\(\s*['"` + "`" + `]([A-Za-z_$][\w$]*)['"` + "`" + `]`,
)

// pothosRootTypeRe matches the field-map registration form
// `builder.queryType({` / `builder.mutationType({` / `builder.subscriptionType({`,
// capturing the root verb. The field names are recovered by scanning the
// `fields: (t) => ({ ... })` object that follows (see pothosFieldEntryRe).
//
// Capture groups:
//
//	1 = root verb (query | mutation | subscription)
var pothosRootTypeRe = regexp.MustCompile(
	`\bbuilder\s*\.\s*(query|mutation|subscription)Type\s*\(\s*\{`,
)

// pothosFieldEntryRe matches each `<field>: t.field(` / `<field>: t.<helper>(`
// entry inside a Pothos `fields: (t) => ({ ... })` object. Pothos field helpers
// are all `t.<something>(` (t.field, t.string, t.int, t.boolean, t.id,
// t.stringList, t.expose*, t.prismaField, t.relation, etc.), so requiring the
// value to start with `t.` and an open paren both scopes the match to real
// field entries and excludes non-field config keys (e.g. `description:` /
// `nullable:`).
//
// Capture group 1 = field name.
var pothosFieldEntryRe = regexp.MustCompile(
	`(?m)^[ \t]*([A-Za-z_$][\w$]*)\s*:\s*t\s*\.\s*[A-Za-z_$][\w$]*\s*\(`,
)

// pothosVerbToRoot maps the Pothos builder verb to its GraphQL root type.
var pothosVerbToRoot = map[string]string{
	"query":        "Query",
	"mutation":     "Mutation",
	"subscription": "Subscription",
}

func synthesizePothos(content string, emit emitDefFn) {
	// File-signal gate: require a Pothos marker so this no-ops on every other
	// JS/TS file. `@pothos/core` is the canonical import; the `builder.<verb>`
	// registration surface is the structural signal.
	if !strings.Contains(content, "@pothos/") &&
		!strings.Contains(content, "SchemaBuilder") &&
		!strings.Contains(content, "builder.queryField") &&
		!strings.Contains(content, "builder.mutationField") &&
		!strings.Contains(content, "builder.subscriptionField") &&
		!strings.Contains(content, "builder.queryType") &&
		!strings.Contains(content, "builder.mutationType") &&
		!strings.Contains(content, "builder.subscriptionType") {
		return
	}

	// Dedup across BOTH registration forms (a field could in principle be
	// declared in queryType and re-added via queryField) keyed on Root.field.
	seen := map[string]bool{}
	emitField := func(root, field string, defLine int) {
		if field == "" {
			return
		}
		key := root + "." + field
		if seen[key] {
			return
		}
		seen[key] = true
		path := "/graphql/" + root + "/" + field
		canonical := httproutes.Canonicalize(httproutes.FrameworkPothos, path)
		// Inline arrow resolver — no addressable handler symbol. Emit with no
		// handler so the synthetic lands on the NoHandlerProp keep-path
		// (mirrors synthesizeGraphQLResolvers / Apollo).
		emit("GRAPHQL", canonical, "pothos", "", "", defLine)
	}

	// (1) Single-field registrations: builder.queryField('users', ...).
	for _, m := range pothosRootFieldRe.FindAllStringSubmatchIndex(content, -1) {
		if len(m) < 6 {
			continue
		}
		root, ok := pothosVerbToRoot[content[m[2]:m[3]]]
		if !ok {
			continue
		}
		field := content[m[4]:m[5]]
		defLine := lineOfOffset(content, m[0])
		emitField(root, field, defLine)
	}

	// (2) Field-map registrations: builder.queryType({ fields: (t) => ({ ... }) }).
	for _, m := range pothosRootTypeRe.FindAllStringSubmatchIndex(content, -1) {
		if len(m) < 4 {
			continue
		}
		root, ok := pothosVerbToRoot[content[m[2]:m[3]]]
		if !ok {
			continue
		}
		// The `{` of the builder.<verb>Type({ ... }) argument object is the
		// last byte consumed by the regex. Scan to its matching close brace
		// so we only read field entries inside THIS registration.
		blockOpen := m[1] - 1
		blockClose := findMatchingBrace(content, blockOpen)
		if blockClose < 0 {
			continue
		}
		body := content[blockOpen+1 : blockClose]
		for _, fm := range pothosFieldEntryRe.FindAllStringSubmatchIndex(body, -1) {
			if len(fm) < 4 {
				continue
			}
			field := body[fm[2]:fm[3]]
			defLine := lineOfOffset(content, blockOpen+1+fm[2])
			emitField(root, field, defLine)
		}
	}
}

// ---------------------------------------------------------------------------
// TypeGraphQL — @Resolver classes with @Query / @Mutation / @Subscription
// decorated methods.
// ---------------------------------------------------------------------------
//
// TypeGraphQL (https://typegraphql.com) is class-and-decorator based:
//
//	@Resolver(() => User)
//	class UserResolver {
//	  @Query(() => [User])
//	  users() { ... }
//
//	  @Query(() => User, { name: 'currentUser' })
//	  me() { ... }
//
//	@Mutation(() => User)
//	  createUser(@Arg('data') data: NewUserInput) { ... }
//
//	  @FieldResolver(() => [Post])   // NON-root — skipped
//	  posts(@Root() user: User) { ... }
//	}
//
// The GraphQL field name defaults to the method name, but a `{ name: '...' }`
// option in the decorator overrides it (TypeGraphQL's `name` option). Only the
// three ROOT operation decorators (@Query / @Mutation / @Subscription) become
// operation endpoints; @FieldResolver methods resolve a field on a non-root
// object type and are intentionally skipped (matching the gqlgen / spring-
// graphql convention of root-only synthesis).
//
// Handler attribution: the decorated method IS an addressable symbol — the
// JS/TS extractor lands it as a SCOPE.Operation entity named by the method.
// We emit source_handler = `SCOPE.Operation:<method>` so the Phase-2 resolver
// rebinds it into a HANDLES edge against the extracted method symbol (the
// resolver's same-file / cross-kind bare-name match, #3426).

// typeGraphQLOpRe matches a @Query / @Mutation / @Subscription decorator and
// the method declaration that follows it (possibly across intervening lines
// carrying other decorators such as @Authorized() or @UseMiddleware()).
//
// Capture groups:
//
//	1 = operation decorator (Query | Mutation | Subscription)
//	2 = the decorator's argument list (between the outer parens), used to
//	    recover an explicit `{ name: '...' }` field-name override; may be empty
//	3 = the method name
//
// The method-name capture allows optional `async` and an optional access
// modifier (public/private/protected) before the identifier, and requires the
// identifier to be immediately followed by `(` so a property is not mistaken
// for a method.
var typeGraphQLOpRe = regexp.MustCompile(
	`@(Query|Mutation|Subscription)\s*\(([^)]*(?:\([^)]*\)[^)]*)*)\)` +
		`(?:\s*@[A-Za-z_$][\w$]*\s*(?:\([^)]*(?:\([^)]*\)[^)]*)*\))?)*` +
		`\s*(?:public\s+|private\s+|protected\s+)?(?:async\s+)?([A-Za-z_$][\w$]*)\s*\(`,
)

// typeGraphQLNameOptRe extracts an explicit `name: '<field>'` from a decorator
// argument list, e.g. `@Query(() => User, { name: 'currentUser' })`.
//
// Capture group 1 = the overridden field name.
var typeGraphQLNameOptRe = regexp.MustCompile(
	`\bname\s*:\s*['"` + "`" + `]([A-Za-z_$][\w$]*)['"` + "`" + `]`,
)

// typeGraphQLDecoToRoot maps the TypeGraphQL operation decorator to its
// GraphQL root type.
var typeGraphQLDecoToRoot = map[string]string{
	"Query":        "Query",
	"Mutation":     "Mutation",
	"Subscription": "Subscription",
}

func synthesizeTypeGraphQL(content string, emit emitDefFn) {
	// File-signal gate: require a TypeGraphQL marker. `type-graphql` is the
	// canonical import; `@Resolver` is the structural class marker. The bare
	// @Query/@Mutation gate below is intentionally NOT used as the sole signal
	// because those identifiers also appear in NestJS GraphQL (handled
	// elsewhere) — requiring type-graphql / @Resolver scopes this synthesizer.
	if !strings.Contains(content, "type-graphql") &&
		!strings.Contains(content, "@Resolver") {
		return
	}

	seen := map[string]bool{}
	for _, m := range typeGraphQLOpRe.FindAllStringSubmatchIndex(content, -1) {
		if len(m) < 8 {
			continue
		}
		root, ok := typeGraphQLDecoToRoot[content[m[2]:m[3]]]
		if !ok {
			continue
		}
		method := content[m[6]:m[7]]
		if method == "" {
			continue
		}
		// Field name: explicit `{ name: '...' }` option overrides the method
		// name (TypeGraphQL's name option).
		field := method
		args := content[m[4]:m[5]]
		if nm := typeGraphQLNameOptRe.FindStringSubmatch(args); nm != nil {
			field = nm[1]
		}

		key := root + "." + field
		if seen[key] {
			continue
		}
		seen[key] = true

		defLine := lineOfOffset(content, m[6])
		path := "/graphql/" + root + "/" + field
		canonical := httproutes.Canonicalize(httproutes.FrameworkTypeGraphQL, path)
		// The decorated method is a real symbol (SCOPE.Operation, named by the
		// method). source_handler rebinds to a HANDLES edge via the resolver's
		// same-file / cross-kind bare-name match.
		emit("GRAPHQL", canonical, "type-graphql", "SCOPE.Operation", method, defLine)
	}
}
