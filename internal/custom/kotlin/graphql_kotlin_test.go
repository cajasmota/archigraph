package kotlin_test

import "testing"

// graphql_kotlin_test.go — value-asserting tests for the graphql-kotlin
// (Expedia Group) extractor. Record lang.kotlin.framework.graphql-kotlin;
// cells endpoint_synthesis / handler_attribution / route_extraction /
// dto_extraction → full. Issue #3537 (epic #3505).

const gqlkSchemaSrc = `
package com.example.graphql

import com.expediagroup.graphql.server.operations.Query
import com.expediagroup.graphql.server.operations.Mutation
import com.expediagroup.graphql.generator.annotations.GraphQLName
import com.expediagroup.graphql.generator.annotations.GraphQLIgnore
import com.expediagroup.graphql.generator.annotations.GraphQLDescription

data class User(val id: ID, val name: String)

data class NewUser(val name: String)

class UserQuery : Query {
    @GraphQLDescription("Fetch a single user by id")
    fun user(id: ID): User = repo.find(id)

    fun users(): List<User> = repo.all()

    @GraphQLIgnore
    fun internalCache(): Map<String, User> = cache

    private fun helper(): Int = 0
}

class UserMutation : Mutation {
    @GraphQLName("createUser")
    fun addUser(input: NewUser): User = repo.save(input)
}
`

func TestGraphQLKotlin_ResolverFields(t *testing.T) {
	ents := extract(t, "custom_kotlin_graphql_kotlin", fi("Schema.kt", "kotlin", gqlkSchemaSrc))
	if len(ents) == 0 {
		t.Fatal("[graphql-kotlin] expected entities, got none")
	}

	// Each public resolver fun on a Query/Mutation root is an addressable
	// GRAPHQL endpoint at /graphql/<Operation>/<field>.
	for _, want := range []string{
		"GRAPHQL /graphql/Query/user",
		"GRAPHQL /graphql/Query/users",
	} {
		e := findEntity(ents, "SCOPE.Operation", want)
		if e == nil {
			t.Errorf("expected operation %q", want)
			continue
		}
		if e.Props["verb"] != "GRAPHQL" {
			t.Errorf("%s: verb = %q, want GRAPHQL", want, e.Props["verb"])
		}
		if e.Props["graphql_operation"] != "Query" {
			t.Errorf("%s: graphql_operation = %q, want Query", want, e.Props["graphql_operation"])
		}
	}

	// Specific field + handler attribution.
	if e := findEntity(ents, "SCOPE.Operation", "GRAPHQL /graphql/Query/user"); e != nil {
		if e.Props["graphql_field"] != "user" {
			t.Errorf("user: graphql_field = %q, want user", e.Props["graphql_field"])
		}
		if e.Props["handler_name"] != "UserQuery.user" {
			t.Errorf("user: handler_name = %q, want UserQuery.user", e.Props["handler_name"])
		}
		if e.Props["graphql_root"] != "UserQuery" {
			t.Errorf("user: graphql_root = %q, want UserQuery", e.Props["graphql_root"])
		}
		if e.Props["graphql_description"] != "Fetch a single user by id" {
			t.Errorf("user: graphql_description = %q", e.Props["graphql_description"])
		}
	}
}

func TestGraphQLKotlin_NameRename(t *testing.T) {
	ents := extract(t, "custom_kotlin_graphql_kotlin", fi("Schema.kt", "kotlin", gqlkSchemaSrc))

	// @GraphQLName("createUser") on fun addUser → field is "createUser".
	e := findEntity(ents, "SCOPE.Operation", "GRAPHQL /graphql/Mutation/createUser")
	if e == nil {
		t.Fatal("expected renamed mutation field createUser")
	}
	if e.Props["graphql_field"] != "createUser" {
		t.Errorf("graphql_field = %q, want createUser", e.Props["graphql_field"])
	}
	// The underlying Kotlin fun name is preserved for handler attribution.
	if e.Props["resolver_fun"] != "addUser" {
		t.Errorf("resolver_fun = %q, want addUser", e.Props["resolver_fun"])
	}
	if e.Props["handler_name"] != "UserMutation.addUser" {
		t.Errorf("handler_name = %q, want UserMutation.addUser", e.Props["handler_name"])
	}
	// The un-renamed name must NOT exist.
	if findEntity(ents, "SCOPE.Operation", "GRAPHQL /graphql/Mutation/addUser") != nil {
		t.Error("un-renamed addUser endpoint should not exist after @GraphQLName")
	}
}

func TestGraphQLKotlin_IgnoreAndPrivate(t *testing.T) {
	ents := extract(t, "custom_kotlin_graphql_kotlin", fi("Schema.kt", "kotlin", gqlkSchemaSrc))

	// @GraphQLIgnore fun is excluded from the schema.
	if findEntity(ents, "SCOPE.Operation", "GRAPHQL /graphql/Query/internalCache") != nil {
		t.Error("@GraphQLIgnore fun internalCache should be excluded")
	}
	// private fun is not a GraphQL field.
	if findEntity(ents, "SCOPE.Operation", "GRAPHQL /graphql/Query/helper") != nil {
		t.Error("private fun helper should be excluded")
	}
}

func TestGraphQLKotlin_DTOs(t *testing.T) {
	ents := extract(t, "custom_kotlin_graphql_kotlin", fi("Schema.kt", "kotlin", gqlkSchemaSrc))

	for _, want := range []string{"User", "NewUser"} {
		e := findEntity(ents, "SCOPE.Schema", "graphql_dto:"+want)
		if e == nil {
			t.Errorf("expected DTO %q", want)
			continue
		}
		if e.Props["dto_name"] != want {
			t.Errorf("dto_name = %q, want %q", e.Props["dto_name"], want)
		}
		if e.Props["framework"] != "graphql-kotlin" {
			t.Errorf("%s: framework = %q", want, e.Props["framework"])
		}
	}
}

func TestGraphQLKotlin_NoMatch(t *testing.T) {
	// Plain Kotlin with no graphql-kotlin signal → no entities.
	src := `
package com.example
data class Plain(val x: Int)
fun helper(): Int = 0
`
	ents := extract(t, "custom_kotlin_graphql_kotlin", fi("Plain.kt", "kotlin", src))
	if len(ents) != 0 {
		t.Errorf("expected no entities, got %d", len(ents))
	}
}

func TestGraphQLKotlin_WrongLanguage(t *testing.T) {
	ents := extract(t, "custom_kotlin_graphql_kotlin", fi("Schema.java", "java", gqlkSchemaSrc))
	if len(ents) != 0 {
		t.Errorf("wrong language should return no entities, got %d", len(ents))
	}
}

// gqlkRelaySrc — a graphql-kotlin schema carrying GraphQL-native directives:
// a @Deprecated resolver field and a Relay/connection pagination field, plus the
// Connection/Edge/PageInfo wire types. Exercises #5010.
const gqlkRelaySrc = `
package com.example.graphql

import com.expediagroup.graphql.server.operations.Query
import com.expediagroup.graphql.generator.annotations.GraphQLDescription

data class User(val id: ID, val name: String)
data class UserEdge(val node: User, val cursor: String)
data class PageInfo(val hasNextPage: Boolean, val endCursor: String?)
data class UserConnection(val edges: List<UserEdge>, val pageInfo: PageInfo)

class UserQuery : Query {
    @Deprecated("use users instead, since 2.0")
    fun legacyUsers(): List<User> = repo.all()

    @GraphQLDescription("Relay-paginated users")
    fun users(first: Int, after: String?): UserConnection = repo.page(first, after)

    fun plain(id: ID): User = repo.find(id)
}
`

func TestGraphQLKotlin_FieldDeprecation(t *testing.T) {
	ents := extract(t, "custom_kotlin_graphql_kotlin", fi("Relay.kt", "kotlin", gqlkRelaySrc))

	e := findEntity(ents, "SCOPE.Operation", "GRAPHQL /graphql/Query/legacyUsers")
	if e == nil {
		t.Fatal("expected legacyUsers endpoint")
	}
	if e.Props["graphql_deprecated"] != "true" {
		t.Errorf("graphql_deprecated = %q, want true", e.Props["graphql_deprecated"])
	}
	if e.Props["deprecated"] != "true" {
		t.Errorf("deprecated = %q, want true", e.Props["deprecated"])
	}
	if e.Props["deprecation_source"] != "@Deprecated" {
		t.Errorf("deprecation_source = %q, want @Deprecated", e.Props["deprecation_source"])
	}
	if e.Props["deprecated_since"] != "2.0" {
		t.Errorf("deprecated_since = %q, want 2.0", e.Props["deprecated_since"])
	}
	if e.Props["deprecated_replacement"] != "users" {
		t.Errorf("deprecated_replacement = %q, want users", e.Props["deprecated_replacement"])
	}

	// A non-deprecated field must NOT carry the deprecation marker.
	if p := findEntity(ents, "SCOPE.Operation", "GRAPHQL /graphql/Query/plain"); p != nil {
		if p.Props["graphql_deprecated"] != "" {
			t.Errorf("plain: graphql_deprecated = %q, want empty", p.Props["graphql_deprecated"])
		}
	}
}

func TestGraphQLKotlin_RelayPagination(t *testing.T) {
	ents := extract(t, "custom_kotlin_graphql_kotlin", fi("Relay.kt", "kotlin", gqlkRelaySrc))

	// The users(first, after): UserConnection field is Relay-paginated.
	e := findEntity(ents, "SCOPE.Operation", "GRAPHQL /graphql/Query/users")
	if e == nil {
		t.Fatal("expected users endpoint")
	}
	if e.Props["graphql_pagination"] != "relay_connection" {
		t.Errorf("graphql_pagination = %q, want relay_connection", e.Props["graphql_pagination"])
	}
	if e.Props["graphql_pagination_args"] != "first,after" {
		t.Errorf("graphql_pagination_args = %q, want first,after", e.Props["graphql_pagination_args"])
	}

	// A non-paginated field must NOT carry the posture.
	if p := findEntity(ents, "SCOPE.Operation", "GRAPHQL /graphql/Query/plain"); p != nil {
		if p.Props["graphql_pagination"] != "" {
			t.Errorf("plain: graphql_pagination = %q, want empty", p.Props["graphql_pagination"])
		}
	}
}

func TestGraphQLKotlin_RelayConnectionTypes(t *testing.T) {
	ents := extract(t, "custom_kotlin_graphql_kotlin", fi("Relay.kt", "kotlin", gqlkRelaySrc))

	cases := []struct {
		dto, role string
	}{
		{"UserConnection", "connection"},
		{"UserEdge", "edge"},
		{"PageInfo", "page_info"},
	}
	for _, c := range cases {
		e := findEntity(ents, "SCOPE.Schema", "graphql_dto:"+c.dto)
		if e == nil {
			t.Errorf("expected DTO %q", c.dto)
			continue
		}
		if e.Props["graphql_dto_role"] != c.role {
			t.Errorf("%s: graphql_dto_role = %q, want %q", c.dto, e.Props["graphql_dto_role"], c.role)
		}
		if e.Props["graphql_pagination"] != "relay_connection" {
			t.Errorf("%s: graphql_pagination = %q, want relay_connection", c.dto, e.Props["graphql_pagination"])
		}
		if e.Props["graphql_pagination_role"] != c.role {
			t.Errorf("%s: graphql_pagination_role = %q, want %q", c.dto, e.Props["graphql_pagination_role"], c.role)
		}
	}

	// A plain DTO keeps role=object and no pagination.
	if u := findEntity(ents, "SCOPE.Schema", "graphql_dto:User"); u != nil {
		if u.Props["graphql_dto_role"] != "object" {
			t.Errorf("User: graphql_dto_role = %q, want object", u.Props["graphql_dto_role"])
		}
		if u.Props["graphql_pagination"] != "" {
			t.Errorf("User: graphql_pagination = %q, want empty", u.Props["graphql_pagination"])
		}
	}
}
