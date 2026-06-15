package php_test

import (
	"context"
	"testing"

	extreg "github.com/cajasmota/grafel/internal/extractor"
	"github.com/cajasmota/grafel/internal/types"
)

// graphql_parity_test.go — value-asserting tests for the PHP GraphQL parity
// credit (epic #3872): Lighthouse / webonyx auth + request/response shapes, and
// the new API Platform GraphQL endpoint synthesis + auth + shapes.

// extractRecords runs an extractor and returns the full entity records (with
// Properties), unlike the summary-only `extract` helper.
func extractRecords(t *testing.T, name string, file extreg.FileInput) []types.EntityRecord {
	t.Helper()
	e, ok := extreg.Get(name)
	if !ok {
		t.Fatalf("extractor %q not registered", name)
	}
	ents, err := e.Extract(context.Background(), file)
	if err != nil {
		t.Fatalf("extract error: %v", err)
	}
	return ents
}

// findRecord returns the first record matching kind+name, or nil.
func findRecord(ents []types.EntityRecord, kind, name string) *types.EntityRecord {
	for i := range ents {
		if ents[i].Kind == kind && ents[i].Name == name {
			return &ents[i]
		}
	}
	return nil
}

func prop(t *testing.T, e *types.EntityRecord, key string) string {
	t.Helper()
	if e == nil {
		t.Fatalf("record is nil, cannot read %q", key)
	}
	return e.Properties[key]
}

// ---------------------------------------------------------------------------
// Lighthouse: auth (@guard / @can) + request/response shapes
// ---------------------------------------------------------------------------

const lhAuthSchema = `
type Query {
    me: User @field(resolver: "App\\GraphQL\\Queries\\Me") @guard
    secret(id: ID!): User @field(resolver: "App\\GraphQL\\Queries\\Secret") @can(ability: "viewAny", model: "App\\User")
    users(page: Int, search: String): [User!]! @paginate @guard(with: ["api"])
    public(id: ID!): User @find
}

type User {
    id: ID!
    name: String!
}
`

func TestLighthouse_Auth_Guard(t *testing.T) {
	ents := extractRecords(t, "custom_php_lighthouse", fi("schema.graphql", "graphql", lhAuthSchema))
	me := findRecord(ents, "SCOPE.Operation", "GRAPHQL /graphql/Query/me")
	if me == nil {
		t.Fatal("expected GRAPHQL /graphql/Query/me endpoint")
	}
	if got := prop(t, me, "auth_required"); got != "true" {
		t.Errorf("@guard field: auth_required = %q, want true", got)
	}
	if got := prop(t, me, "auth_method"); got != "guard" {
		t.Errorf("@guard field: auth_method = %q, want guard", got)
	}
	if got := prop(t, me, "auth_directive"); got != "@guard" {
		t.Errorf("@guard field: auth_directive = %q, want @guard", got)
	}
}

func TestLighthouse_Auth_Can_Permission(t *testing.T) {
	ents := extractRecords(t, "custom_php_lighthouse", fi("schema.graphql", "graphql", lhAuthSchema))
	sec := findRecord(ents, "SCOPE.Operation", "GRAPHQL /graphql/Query/secret")
	if sec == nil {
		t.Fatal("expected GRAPHQL /graphql/Query/secret endpoint")
	}
	if got := prop(t, sec, "auth_required"); got != "true" {
		t.Errorf("@can field: auth_required = %q, want true", got)
	}
	if got := prop(t, sec, "auth_method"); got != "policy" {
		t.Errorf("@can field: auth_method = %q, want policy", got)
	}
	if got := prop(t, sec, "auth_permissions"); got != "viewAny" {
		t.Errorf("@can field: auth_permissions = %q, want viewAny", got)
	}
}

func TestLighthouse_Auth_GuardWith(t *testing.T) {
	ents := extractRecords(t, "custom_php_lighthouse", fi("schema.graphql", "graphql", lhAuthSchema))
	u := findRecord(ents, "SCOPE.Operation", "GRAPHQL /graphql/Query/users")
	if u == nil {
		t.Fatal("expected GRAPHQL /graphql/Query/users endpoint")
	}
	if got := prop(t, u, "auth_guards"); got != "api" {
		t.Errorf("@guard(with:[\"api\"]): auth_guards = %q, want api", got)
	}
}

// A field with NO auth directive must NOT be marked auth_required (negative).
func TestLighthouse_Auth_NegativeUnauthenticated(t *testing.T) {
	ents := extractRecords(t, "custom_php_lighthouse", fi("schema.graphql", "graphql", lhAuthSchema))
	pub := findRecord(ents, "SCOPE.Operation", "GRAPHQL /graphql/Query/public")
	if pub == nil {
		t.Fatal("expected GRAPHQL /graphql/Query/public endpoint")
	}
	if got := pub.Properties["auth_required"]; got != "" {
		t.Errorf("directive-free field leaked auth_required = %q, want empty", got)
	}
}

func TestLighthouse_RequestResponseShapes(t *testing.T) {
	ents := extractRecords(t, "custom_php_lighthouse", fi("schema.graphql", "graphql", lhAuthSchema))
	users := findRecord(ents, "SCOPE.Operation", "GRAPHQL /graphql/Query/users")
	if users == nil {
		t.Fatal("expected GRAPHQL /graphql/Query/users endpoint")
	}
	if got := prop(t, users, "request_shape"); got != "page:Int,search:String" {
		t.Errorf("request_shape = %q, want page:Int,search:String", got)
	}
	if got := prop(t, users, "response_shape"); got != "User" {
		t.Errorf("response_shape = %q, want User", got)
	}
	// A field with no args has no request_shape (honest negative).
	me := findRecord(ents, "SCOPE.Operation", "GRAPHQL /graphql/Query/me")
	if got := me.Properties["request_shape"]; got != "" {
		t.Errorf("argument-free field leaked request_shape = %q", got)
	}
}

// ---------------------------------------------------------------------------
// webonyx graphql-php: request/response shapes
// ---------------------------------------------------------------------------

const gqlpShapeSrc = `<?php
$queryType = new ObjectType([
    'name' => 'Query',
    'fields' => [
        'user' => [
            'type' => Type::nonNull($userType),
            'args' => [
                'id'     => Type::nonNull(Type::id()),
                'filter' => ['type' => Type::string()],
            ],
            'resolve' => fn ($root, $args) => $repo->find($args['id']),
        ],
        'ping' => [
            'type' => Type::string(),
            'resolve' => fn () => 'pong',
        ],
    ],
]);
`

func TestGraphQLPHP_RequestResponseShapes(t *testing.T) {
	ents := extractRecords(t, "custom_php_graphql_php", fi("schema.php", "php", gqlpShapeSrc))
	user := findRecord(ents, "SCOPE.Operation", "GRAPHQL /graphql/Query/user")
	if user == nil {
		t.Fatal("expected GRAPHQL /graphql/Query/user endpoint")
	}
	if got := prop(t, user, "request_shape"); got != "id:id,filter:string" {
		t.Errorf("request_shape = %q, want id:id,filter:string", got)
	}
	if got := prop(t, user, "response_shape"); got != "userType" {
		t.Errorf("response_shape = %q, want userType", got)
	}
	// A field with no args map → no request_shape (honest negative).
	ping := findRecord(ents, "SCOPE.Operation", "GRAPHQL /graphql/Query/ping")
	if got := ping.Properties["request_shape"]; got != "" {
		t.Errorf("argument-free field leaked request_shape = %q", got)
	}
	if got := prop(t, ping, "response_shape"); got != "string" {
		t.Errorf("ping response_shape = %q, want string", got)
	}
}

// ---------------------------------------------------------------------------
// API Platform GraphQL: endpoint synthesis + auth + shapes (NEW)
// ---------------------------------------------------------------------------

const apGQLSrc = `<?php
namespace App\Entity;
use ApiPlatform\Metadata\ApiResource;
use ApiPlatform\Metadata\GraphQl\Query;
use ApiPlatform\Metadata\GraphQl\QueryCollection;
use ApiPlatform\Metadata\GraphQl\Mutation;

#[ApiResource(
    graphQlOperations: [
        new Query(security: "is_granted('ROLE_ADMIN')"),
        new QueryCollection(),
        new Mutation(name: 'create', security: "is_granted('ROLE_USER')"),
    ]
)]
class Book
{
    public int $id;
    public string $title;
    public ?Author $author = null;
}
`

func TestAPIPlatformGraphQL_EndpointSynthesis(t *testing.T) {
	ents := extractRecords(t, "custom_php_api_platform_graphql", fi("Book.php", "php", apGQLSrc))
	if len(ents) == 0 {
		t.Fatal("[api-platform-graphql] expected entities, got none")
	}
	for _, want := range []string{
		"GRAPHQL /graphql/Query/book",  // item Query (default name)
		"GRAPHQL /graphql/Query/books", // QueryCollection (pluralised)
		"GRAPHQL /graphql/Mutation/create",
	} {
		if findRecord(ents, "SCOPE.Operation", want) == nil {
			t.Errorf("expected GraphQL endpoint %q", want)
		}
	}
}

func TestAPIPlatformGraphQL_Auth(t *testing.T) {
	ents := extractRecords(t, "custom_php_api_platform_graphql", fi("Book.php", "php", apGQLSrc))
	q := findRecord(ents, "SCOPE.Operation", "GRAPHQL /graphql/Query/book")
	if q == nil {
		t.Fatal("expected GRAPHQL /graphql/Query/book endpoint")
	}
	if got := prop(t, q, "auth_required"); got != "true" {
		t.Errorf("auth_required = %q, want true", got)
	}
	if got := prop(t, q, "auth_roles"); got != "ROLE_ADMIN" {
		t.Errorf("auth_roles = %q, want ROLE_ADMIN", got)
	}
	if got := prop(t, q, "auth_method"); got != "expression" {
		t.Errorf("auth_method = %q, want expression", got)
	}
	// QueryCollection has no security: → not auth_required (negative).
	coll := findRecord(ents, "SCOPE.Operation", "GRAPHQL /graphql/Query/books")
	if got := coll.Properties["auth_required"]; got != "" {
		t.Errorf("unsecured collection leaked auth_required = %q", got)
	}
}

func TestAPIPlatformGraphQL_Shapes(t *testing.T) {
	ents := extractRecords(t, "custom_php_api_platform_graphql", fi("Book.php", "php", apGQLSrc))
	mut := findRecord(ents, "SCOPE.Operation", "GRAPHQL /graphql/Mutation/create")
	if mut == nil {
		t.Fatal("expected GRAPHQL /graphql/Mutation/create endpoint")
	}
	// Response shape = all typed public props; request (input) shape drops id.
	if got := prop(t, mut, "response_shape"); got != "id:int,title:string,author:Author" {
		t.Errorf("response_shape = %q, want id:int,title:string,author:Author", got)
	}
	if got := prop(t, mut, "request_shape"); got != "title:string,author:Author" {
		t.Errorf("request_shape = %q, want title:string,author:Author", got)
	}
}

// graphql: true shorthand → default operation set.
func TestAPIPlatformGraphQL_Shorthand(t *testing.T) {
	src := `<?php
use ApiPlatform\Metadata\ApiResource;
#[ApiResource(graphql: true)]
class Author { public int $id; public string $name; }
`
	ents := extractRecords(t, "custom_php_api_platform_graphql", fi("Author.php", "php", src))
	if findRecord(ents, "SCOPE.Operation", "GRAPHQL /graphql/Query/author") == nil {
		t.Error("expected default item Query endpoint from graphql: true shorthand")
	}
	if findRecord(ents, "SCOPE.Operation", "GRAPHQL /graphql/Mutation/createAuthor") == nil {
		t.Error("expected default create Mutation from graphql: true shorthand")
	}
}

// A REST-only #[ApiResource] (no GraphQL opt-in) yields NO GraphQL endpoints.
func TestAPIPlatformGraphQL_NegativeRESTOnly(t *testing.T) {
	src := `<?php
use ApiPlatform\Metadata\ApiResource;
#[ApiResource]
class Book { public int $id; public string $title; }
`
	ents := extractRecords(t, "custom_php_api_platform_graphql", fi("Book.php", "php", src))
	if len(ents) != 0 {
		t.Errorf("REST-only resource produced %d GraphQL endpoints, want 0", len(ents))
	}
}

// A plain PHP class with no #[ApiResource] yields nothing (negative).
func TestAPIPlatformGraphQL_NegativePlain(t *testing.T) {
	src := `<?php class Plain { public function go(): int { return 1; } }`
	ents := extractRecords(t, "custom_php_api_platform_graphql", fi("Plain.php", "php", src))
	if len(ents) != 0 {
		t.Errorf("plain PHP produced %d entities, want 0", len(ents))
	}
}

// Language gate: non-php files are ignored.
func TestAPIPlatformGraphQL_WrongLanguage(t *testing.T) {
	ents := extractRecords(t, "custom_php_api_platform_graphql", fi("Book.kt", "kotlin", apGQLSrc))
	if len(ents) != 0 {
		t.Errorf("non-php language produced %d entities, want 0", len(ents))
	}
}
