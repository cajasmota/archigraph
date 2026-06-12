package rust_test

import "testing"

// juniper_test.go — value-asserting tests for the custom_rust_juniper extractor
// (#4964). juniper is the GraphQL-server sibling of async-graphql with a distinct
// macro vocabulary (#[graphql_object], #[derive(GraphQLObject/GraphQLInputObject/
// GraphQLEnum)], RootNode::new). Asserts the EXACT canonical GRAPHQL endpoint
// shape /graphql/<Root>/<field>, DTO roles, schema-root capture, the file-signal
// gate (no-op on non-juniper Rust), and that juniper does not collide with the
// async-graphql extractor.

func TestJuniperResolverFields(t *testing.T) {
	src := `
struct Query;

#[graphql_object]
impl Query {
    fn user(&self, id: i32) -> User {
        User::default()
    }
    fn users(&self) -> Vec<User> {
        vec![]
    }
}

struct Mutation;

#[graphql_object(context = Ctx)]
impl Mutation {
    fn create_user(&self, new: NewUser) -> User {
        User::default()
    }
}

struct Subscription;

#[graphql_subscription]
impl Subscription {
    async fn user_added(&self) -> UserStream {
        unimplemented!()
    }
}
`
	ents := extract(t, "custom_rust_juniper", fi("schema.rs", "rust", src))

	for _, want := range []string{
		"GRAPHQL /graphql/Query/user",
		"GRAPHQL /graphql/Query/users",
		"GRAPHQL /graphql/Mutation/create_user",
		"GRAPHQL /graphql/Subscription/user_added",
	} {
		e := findEnt(ents, "SCOPE.Operation", want)
		if e == nil {
			t.Fatalf("expected juniper resolver endpoint %q", want)
		}
		if e.Props["verb"] != "GRAPHQL" {
			t.Errorf("%s: verb = %q, want GRAPHQL", want, e.Props["verb"])
		}
		if e.Props["framework"] != "juniper" {
			t.Errorf("%s: framework = %q, want juniper", want, e.Props["framework"])
		}
	}

	// Operation kind derived from impl root.
	if e := findEnt(ents, "SCOPE.Operation", "GRAPHQL /graphql/Query/user"); e != nil {
		if e.Props["graphql_operation"] != "Query" {
			t.Errorf("Query.user: graphql_operation = %q, want Query", e.Props["graphql_operation"])
		}
		if e.Props["handler_name"] != "Query.user" {
			t.Errorf("Query.user: handler_name = %q, want Query.user", e.Props["handler_name"])
		}
	}
	if e := findEnt(ents, "SCOPE.Operation", "GRAPHQL /graphql/Mutation/create_user"); e != nil {
		if e.Props["graphql_operation"] != "Mutation" {
			t.Errorf("create_user: graphql_operation = %q, want Mutation", e.Props["graphql_operation"])
		}
	}
	if e := findEnt(ents, "SCOPE.Operation", "GRAPHQL /graphql/Subscription/user_added"); e != nil {
		if e.Props["graphql_operation"] != "Subscription" {
			t.Errorf("user_added: graphql_operation = %q, want Subscription", e.Props["graphql_operation"])
		}
	}
}

func TestJuniperDTOsAndEnum(t *testing.T) {
	src := `
#[derive(GraphQLObject)]
struct User {
    id: i32,
    name: String,
}

#[derive(GraphQLInputObject)]
struct NewUser {
    name: String,
}

#[derive(GraphQLEnum)]
enum Episode {
    NewHope,
    Empire,
}
`
	ents := extract(t, "custom_rust_juniper", fi("types.rs", "rust", src))

	cases := []struct {
		name string
		role string
	}{
		{"graphql_dto:User", "object"},
		{"graphql_dto:NewUser", "input"},
		{"graphql_dto:Episode", "enum"},
	}
	for _, c := range cases {
		e := findEnt(ents, "SCOPE.Schema", c.name)
		if e == nil {
			t.Fatalf("expected DTO %q", c.name)
		}
		if e.Props["graphql_dto_role"] != c.role {
			t.Errorf("%s: role = %q, want %q", c.name, e.Props["graphql_dto_role"], c.role)
		}
		if e.Props["framework"] != "juniper" {
			t.Errorf("%s: framework = %q, want juniper", c.name, e.Props["framework"])
		}
	}
}

func TestJuniperSchemaRoot(t *testing.T) {
	src := `
#[derive(GraphQLObject)]
struct User { id: i32 }

fn make_schema() -> Schema {
    RootNode::new(Query, Mutation, Subscription)
}
`
	ents := extract(t, "custom_rust_juniper", fi("schema.rs", "rust", src))

	e := findEnt(ents, "SCOPE.Service", "graphql_schema:Query,Mutation,Subscription")
	if e == nil {
		t.Fatalf("expected juniper schema-root SCOPE.Service")
	}
	if e.Props["query_root"] != "Query" || e.Props["mutation_root"] != "Mutation" || e.Props["subscription_root"] != "Subscription" {
		t.Errorf("schema roots = %v", e.Props)
	}
}

func TestJuniperSchemaRoot_EmptyConstructors(t *testing.T) {
	src := `
#[derive(GraphQLObject)]
struct User { id: i32 }

fn make_schema() -> Schema {
    Schema::new(Query, EmptyMutation::new(), EmptySubscription::new())
}
`
	ents := extract(t, "custom_rust_juniper", fi("schema.rs", "rust", src))

	e := findEnt(ents, "SCOPE.Service", "graphql_schema:Query,EmptyMutation,EmptySubscription")
	if e == nil {
		t.Fatalf("expected juniper schema-root with stripped constructor tails")
	}
}

// TestJuniperGate asserts the file-signal gate: a plain Rust file with no juniper
// macro emits nothing, and an async-graphql file is NOT misattributed to juniper.
func TestJuniperGate(t *testing.T) {
	plain := `
struct Query;
impl Query {
    fn user(&self) -> User { User::default() }
}
fn main() {}
`
	if ents := extract(t, "custom_rust_juniper", fi("plain.rs", "rust", plain)); len(ents) != 0 {
		t.Fatalf("plain Rust should emit nothing, got %d", len(ents))
	}

	// async-graphql idiom (#[Object], SimpleObject) must not be picked up by the
	// juniper extractor.
	agql := `
#[derive(SimpleObject)]
struct User { id: ID }

struct Query;
#[Object]
impl Query {
    async fn user(&self) -> Result<User> { Ok(User::default()) }
}
`
	if ents := extract(t, "custom_rust_juniper", fi("agql.rs", "rust", agql)); len(ents) != 0 {
		t.Fatalf("async-graphql file should not be claimed by juniper, got %d", len(ents))
	}
}

// TestJuniperNonRust asserts the language gate.
func TestJuniperNonRust(t *testing.T) {
	src := `#[graphql_object] impl Query { fn x() -> i32 { 1 } }`
	if ents := extract(t, "custom_rust_juniper", fi("x.go", "go", src)); len(ents) != 0 {
		t.Fatalf("non-rust language should emit nothing, got %d", len(ents))
	}
}
