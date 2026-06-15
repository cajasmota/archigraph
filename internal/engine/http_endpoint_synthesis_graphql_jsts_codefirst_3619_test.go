package engine

import (
	"testing"

	"github.com/cajasmota/grafel/internal/types"
)

// The code-first GraphQL tests assert the EXACT operation-endpoint id shape
// shared with gqlgen / Apollo / Strawberry (via findSynthDef), then probe per-
// endpoint properties (framework, source_handler, start line).

// ---------------------------------------------------------------------------
// Pothos — #3619
// ---------------------------------------------------------------------------

// TestSynth_Pothos_RootFields covers builder.queryField / mutationField /
// subscriptionField single-field registrations. Asserts the EXACT operation-
// endpoint shape shared with gqlgen / Apollo / Strawberry, the framework label,
// and that the inline-arrow resolver yields NO source_handler (NoHandlerProp
// keep-path) so the synthetic survives Phase-2 resolution. #3619.
func TestSynth_Pothos_RootFields(t *testing.T) {
	src := `import SchemaBuilder from '@pothos/core';

const builder = new SchemaBuilder({});

builder.queryField('users', (t) =>
  t.field({ type: [User], resolve: () => getUsers() }),
);

builder.mutationField('createUser', (t) =>
  t.field({ type: User, resolve: (_, args) => create(args) }),
);

builder.subscriptionField('userAdded', (t) =>
  t.field({ type: User, subscribe: () => pubsub.asyncIterator('USER_ADDED') }),
);
`
	got, res := runDetect(t, "typescript", "schema/queries.ts", src)
	want := []string{
		"http:GRAPHQL:/graphql/Query/users",
		"http:GRAPHQL:/graphql/Mutation/createUser",
		"http:GRAPHQL:/graphql/Subscription/userAdded",
	}
	requireContains(t, got, want, "Pothos root fields")

	// EXACT shape + framework + no-handler keep-path.
	e := findSynthDef(res, "http:GRAPHQL:/graphql/Query/users")
	if e == nil {
		t.Fatalf("Pothos: missing http:GRAPHQL:/graphql/Query/users")
	}
	if e.Properties["framework"] != "pothos" {
		t.Errorf("Pothos: framework = %q, want pothos", e.Properties["framework"])
	}
	if e.Properties["source_handler"] != "" {
		t.Errorf("Pothos: source_handler = %q, want empty (inline-arrow resolver)", e.Properties["source_handler"])
	}
	if e.StartLine == 0 {
		t.Errorf("Pothos: StartLine not stamped on builder.queryField('users')")
	}
}

// TestSynth_Pothos_FieldMap covers the builder.queryType({ fields: (t) => ({...}) })
// field-map registration form. Asserts each `<field>: t.field(` entry under the
// root becomes an operation endpoint with the EXACT canonical shape. #3619.
func TestSynth_Pothos_FieldMap(t *testing.T) {
	src := `import SchemaBuilder from '@pothos/core';
const builder = new SchemaBuilder({});

builder.queryType({
  description: 'root query',
  fields: (t) => ({
    me: t.field({ type: User, resolve: () => currentUser() }),
    users: t.field({ type: [User], resolve: () => getUsers() }),
  }),
});

builder.mutationType({
  fields: (t) => ({
    login: t.field({ type: Session, resolve: (_, a) => doLogin(a) }),
  }),
});
`
	got, res := runDetect(t, "typescript", "schema/root.ts", src)
	want := []string{
		"http:GRAPHQL:/graphql/Query/me",
		"http:GRAPHQL:/graphql/Query/users",
		"http:GRAPHQL:/graphql/Mutation/login",
	}
	requireContains(t, got, want, "Pothos field map")

	// The non-field config key `description:` must NOT produce an endpoint.
	if e := findSynthDef(res, "http:GRAPHQL:/graphql/Query/description"); e != nil {
		t.Errorf("Pothos field map: emitted a bogus endpoint for the `description` config key")
	}

	e := findSynthDef(res, "http:GRAPHQL:/graphql/Query/me")
	if e == nil {
		t.Fatalf("Pothos field map: missing http:GRAPHQL:/graphql/Query/me")
	}
	if e.Properties["framework"] != "pothos" {
		t.Errorf("Pothos field map: framework = %q, want pothos", e.Properties["framework"])
	}
}

// ---------------------------------------------------------------------------
// TypeGraphQL — #3619
// ---------------------------------------------------------------------------

// TestSynth_TypeGraphQL_Resolver covers @Query / @Mutation decorated methods in
// a @Resolver class. Asserts the EXACT operation-endpoint shape, the framework
// label, and the SCOPE.Operation handler attribution that the Phase-2 resolver
// rebinds into a HANDLES edge to the method symbol. #3619.
func TestSynth_TypeGraphQL_Resolver(t *testing.T) {
	src := `import { Resolver, Query, Mutation, Arg } from 'type-graphql';

@Resolver(() => User)
export class UserResolver {
  @Query(() => [User])
  users() {
    return this.service.all();
  }

  @Query(() => User)
  me() {
    return this.service.current();
  }

  @Mutation(() => User)
  createUser(@Arg('data') data: NewUserInput) {
    return this.service.create(data);
  }
}
`
	got, res := runDetect(t, "typescript", "resolvers/user.resolver.ts", src)
	want := []string{
		"http:GRAPHQL:/graphql/Query/users",
		"http:GRAPHQL:/graphql/Query/me",
		"http:GRAPHQL:/graphql/Mutation/createUser",
	}
	requireContains(t, got, want, "TypeGraphQL resolver")

	// EXACT shape + framework + handler attribution → HANDLES edge.
	e := findSynthDef(res, "http:GRAPHQL:/graphql/Query/me")
	if e == nil {
		t.Fatalf("TypeGraphQL: missing http:GRAPHQL:/graphql/Query/me")
	}
	if e.Properties["framework"] != "type-graphql" {
		t.Errorf("TypeGraphQL: framework = %q, want type-graphql", e.Properties["framework"])
	}
	if e.StartLine == 0 {
		t.Errorf("TypeGraphQL: StartLine not stamped on me() resolver")
	}
	// The me() method is an extracted SCOPE.Operation symbol. The synthesizer
	// stamps source_handler = SCOPE.Operation:me; the Phase-2 resolver
	// (ResolveHTTPEndpointHandlers, run over merged entities) rebinds this into
	// a HANDLES edge against the method symbol. Phase-1 Detect preserves the
	// ref, so assert the exact attribution shape here (mirrors the gqlgen test).
	if e.Properties["source_handler"] != "SCOPE.Operation:me" {
		t.Errorf("TypeGraphQL: source_handler = %q, want SCOPE.Operation:me", e.Properties["source_handler"])
	}
}

// TestSynth_TypeGraphQL_NameOverride asserts the `{ name: '...' }` decorator
// option overrides the method name for the GraphQL field. #3619.
func TestSynth_TypeGraphQL_NameOverride(t *testing.T) {
	src := `import { Resolver, Query } from 'type-graphql';

@Resolver()
class MeResolver {
  @Query(() => User, { name: 'currentUser' })
  me() {
    return current();
  }
}
`
	got, _ := runDetect(t, "typescript", "resolvers/me.resolver.ts", src)
	want := []string{"http:GRAPHQL:/graphql/Query/currentUser"}
	requireContains(t, got, want, "TypeGraphQL name override")

	// The method name `me` must NOT also be emitted — the option wins.
	for _, id := range got {
		if id == "http:GRAPHQL:/graphql/Query/me" {
			t.Errorf("TypeGraphQL name override: emitted both Query/me and Query/currentUser; the { name } option must win")
		}
	}
}

// TestSynth_TypeGraphQL_SkipsFieldResolver asserts that @FieldResolver methods
// (which resolve a field on a NON-root object type) are intentionally skipped —
// only the three ROOT operation decorators become operation endpoints, matching
// the gqlgen / spring-graphql root-only synthesis convention. #3619.
func TestSynth_TypeGraphQL_SkipsFieldResolver(t *testing.T) {
	src := `import { Resolver, Query, FieldResolver, Root } from 'type-graphql';

@Resolver(() => User)
class UserResolver {
  @Query(() => [User])
  users() {
    return all();
  }

  @FieldResolver(() => [Post])
  posts(@Root() user: User) {
    return user.posts;
  }
}
`
	got, _ := runDetect(t, "typescript", "resolvers/user.resolver.ts", src)
	requireContains(t, got, []string{"http:GRAPHQL:/graphql/Query/users"}, "TypeGraphQL root query")

	for _, id := range got {
		if id == "http:GRAPHQL:/graphql/Query/posts" ||
			id == "http:GRAPHQL:/graphql/Mutation/posts" ||
			id == "http:GRAPHQL:/graphql/Subscription/posts" {
			t.Errorf("TypeGraphQL: @FieldResolver method posts() must not become a root operation endpoint (got %q)", id)
		}
	}
}

// TestSynth_GraphQL_CodeFirst_ShapeParity locks the EXACT operation-endpoint id
// shape parity between Pothos, TypeGraphQL, and the gqlgen reference. A Pothos
// queryField('users') and a TypeGraphQL @Query users() must both produce the
// IDENTICAL id `http:GRAPHQL:/graphql/Query/users` — the join key for the
// GraphQL client-link synthesizer (#3667). #3619.
func TestSynth_GraphQL_CodeFirst_ShapeParity(t *testing.T) {
	pothosSrc := `import SchemaBuilder from '@pothos/core';
const builder = new SchemaBuilder({});
builder.queryField('users', (t) => t.field({ resolve: () => all() }));
`
	tgSrc := `import { Resolver, Query } from 'type-graphql';
@Resolver()
class R { @Query(() => [User]) users() { return all(); } }
`
	const want = "http:GRAPHQL:/graphql/Query/users"

	pGot, _ := runDetect(t, "typescript", "p.ts", pothosSrc)
	requireContains(t, pGot, []string{want}, "Pothos parity")

	tGot, _ := runDetect(t, "typescript", "t.ts", tgSrc)
	requireContains(t, tGot, []string{want}, "TypeGraphQL parity")
}

// TestResolve_TypeGraphQL_HandlesEdge drives the Phase-2 resolver over a merged
// set containing a TypeGraphQL operation synthetic (source_handler =
// SCOPE.Operation:me) and the extracted SCOPE.Operation:me method symbol. It
// proves the handler ref rebinds into an IMPLEMENTS (HANDLES) edge on the
// method and clears the source_handler property — the same rebind gqlgen relies
// on. #3619.
func TestResolve_TypeGraphQL_HandlesEdge(t *testing.T) {
	method := types.EntityRecord{
		Kind:       "SCOPE.Operation",
		Name:       "me",
		SourceFile: "resolvers/me.resolver.ts",
		Language:   "typescript",
	}
	synth := types.EntityRecord{
		Kind:       httpEndpointKind,
		Name:       "http:GRAPHQL:/graphql/Query/me",
		SourceFile: "resolvers/me.resolver.ts",
		Language:   "typescript",
		Properties: map[string]string{
			"source_handler": "SCOPE.Operation:me",
			"framework":      "type-graphql",
		},
	}
	merged := []types.EntityRecord{method, synth}
	out, stats := ResolveHTTPEndpointHandlers(merged)

	if stats.HandlerResolved != 1 || stats.HandlerDropped != 0 {
		t.Errorf("TypeGraphQL resolve: stats unexpected: %+v", stats)
	}
	if len(out[0].Relationships) != 1 {
		t.Fatalf("TypeGraphQL resolve: expected 1 edge on the me() method, got %d", len(out[0].Relationships))
	}
	rel := out[0].Relationships[0]
	if rel.Kind != implementsEdgeKind {
		t.Errorf("TypeGraphQL resolve: edge kind = %s, want %s", rel.Kind, implementsEdgeKind)
	}
	if rel.FromID != "SCOPE.Operation:me" ||
		rel.ToID != "http_endpoint_definition:http:GRAPHQL:/graphql/Query/me" {
		t.Errorf("TypeGraphQL resolve: edge ids wrong: from=%s to=%s", rel.FromID, rel.ToID)
	}
	if _, ok := out[1].Properties["source_handler"]; ok {
		t.Errorf("TypeGraphQL resolve: source_handler should be cleared after rebind")
	}
}
