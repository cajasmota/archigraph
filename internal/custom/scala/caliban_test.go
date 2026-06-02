package scala_test

import "testing"

// ---------------------------------------------------------------------------
// Caliban (Scala GraphQL)
// ---------------------------------------------------------------------------

func TestCalibanResolverFields(t *testing.T) {
	src := `
import caliban._
import caliban.schema.Schema

case class UserArgs(id: String)

case class Queries(
  user: UserArgs => URIO[Any, User],
  users: () => List[User],
)

case class Mutations(
  createUser: NewUser => Task[User],
)

object Api {
  val api = graphQL(RootResolver(Queries(resolveUser, resolveUsers), Mutations(resolveCreate)))
}
`
	ents := extract(t, "custom_scala_caliban", fi("Api.scala", "scala", src))

	// Each resolver case-class field becomes an addressable GRAPHQL endpoint,
	// rooted by the positional RootResolver argument (Queries=Query,
	// Mutations=Mutation).
	for _, want := range []string{
		"GRAPHQL /graphql/Queries/user",
		"GRAPHQL /graphql/Queries/users",
		"GRAPHQL /graphql/Mutations/createUser",
	} {
		e := findEntity(ents, "SCOPE.Operation", want)
		if e == nil {
			t.Fatalf("expected resolver endpoint %q", want)
		}
		if e.Props["verb"] != "GRAPHQL" {
			t.Errorf("%s: verb = %q, want GRAPHQL", want, e.Props["verb"])
		}
		if e.Props["framework"] != "caliban" {
			t.Errorf("%s: framework = %q, want caliban", want, e.Props["framework"])
		}
	}

	// Query-root field carries the right operation kind + handler.
	if e := findEntity(ents, "SCOPE.Operation", "GRAPHQL /graphql/Queries/user"); e != nil {
		if e.Props["graphql_operation"] != "Query" {
			t.Errorf("user: graphql_operation = %q, want Query", e.Props["graphql_operation"])
		}
		if e.Props["graphql_field"] != "user" {
			t.Errorf("user: graphql_field = %q, want user", e.Props["graphql_field"])
		}
		if e.Props["handler_name"] != "Queries.user" {
			t.Errorf("user: handler_name = %q, want Queries.user", e.Props["handler_name"])
		}
	}

	// Mutation-root field carries the Mutation operation kind.
	if e := findEntity(ents, "SCOPE.Operation", "GRAPHQL /graphql/Mutations/createUser"); e != nil {
		if e.Props["graphql_operation"] != "Mutation" {
			t.Errorf("createUser: graphql_operation = %q, want Mutation", e.Props["graphql_operation"])
		}
	}

	// A schema-root entity captures the wired roots positionally.
	if e := findEntity(ents, "SCOPE.Service", "graphql_schema:Queries,Mutations"); e == nil {
		t.Fatalf("expected graphql_schema root entity for Queries,Mutations")
	} else {
		if e.Props["query_root"] != "Queries" {
			t.Errorf("schema: query_root = %q, want Queries", e.Props["query_root"])
		}
		if e.Props["mutation_root"] != "Mutations" {
			t.Errorf("schema: mutation_root = %q, want Mutations", e.Props["mutation_root"])
		}
	}
}

func TestCalibanSchemaAdapter(t *testing.T) {
	src := `
import caliban._
import caliban.interop.http4s.Http4sAdapter

case class Queries(users: () => List[User])

object Server {
  val api = graphQL(RootResolver(Queries(resolveUsers)))
  val routes = http4sAdapter.makeHttpService(interpreter)
}
`
	ents := extract(t, "custom_scala_caliban", fi("Server.scala", "scala", src))

	e := findEntity(ents, "SCOPE.Service", "graphql_schema:Queries")
	if e == nil {
		t.Fatalf("expected graphql_schema:Queries entity")
	}
	if e.Props["http_adapter"] != "http4sAdapter" {
		t.Errorf("schema: http_adapter = %q, want http4sAdapter", e.Props["http_adapter"])
	}
}

func TestCalibanDTOs(t *testing.T) {
	src := `
import caliban.schema.Annotations._
import caliban.schema.Schema

@GQLDescription("A registered user")
case class User(id: String, name: String)

@GQLInputName("NewUserInput")
case class NewUser(name: String)

@GQLName("Role")
enum Role:
  case Admin, Member

object Schemas {
  implicit val userSchema = Schema.gen[Any, User]
}
`
	ents := extract(t, "custom_scala_caliban", fi("Schemas.scala", "scala", src))

	// Annotated case classes become schema DTOs.
	if e := findEntity(ents, "SCOPE.Schema", "graphql_dto:User"); e == nil {
		t.Fatalf("expected graphql_dto:User")
	} else if e.Props["graphql_dto_role"] != "object" {
		t.Errorf("User: graphql_dto_role = %q, want object", e.Props["graphql_dto_role"])
	}

	if e := findEntity(ents, "SCOPE.Schema", "graphql_dto:NewUser"); e == nil {
		t.Fatalf("expected graphql_dto:NewUser")
	}

	// An annotated enum becomes an enum-role DTO.
	if e := findEntity(ents, "SCOPE.Schema", "graphql_dto:Role"); e == nil {
		t.Fatalf("expected graphql_dto:Role")
	} else if e.Props["graphql_dto_role"] != "enum" {
		t.Errorf("Role: graphql_dto_role = %q, want enum", e.Props["graphql_dto_role"])
	}
}

// TestCalibanAuthDirective is the VERIFY-FIRST probe (#3992): a resolver field
// carrying @GQLDirective(Authenticated) is auth-gated, and @GQLDirective(
// HasRole("admin")) carries a role. Directive-free fields and non-auth
// directives (@GQLDeprecated / @GQLDirective(deprecated)) carry NO auth.
func TestCalibanAuthDirective(t *testing.T) {
	src := `
import caliban._
import caliban.schema.Annotations._

case class Queries(
  @GQLDirective(Authenticated) me: () => URIO[Auth, User],
  @GQLDirective(HasRole("admin")) adminStats: () => URIO[Auth, Stats],
  @GQLDeprecated("use me") legacy: () => UIO[User],
  health: () => UIO[String],
)

object Api {
  val api = graphQL(RootResolver(Queries(resolveMe, resolveAdmin, resolveLegacy, resolveHealth)))
}
`
	ents := extract(t, "custom_scala_caliban", fi("Api.scala", "scala", src))

	// @GQLDirective(Authenticated) → auth_required=true, method=directive,
	// the directive name is recorded, and auth_guard makes it count as covered.
	me := findEntity(ents, "SCOPE.Operation", "GRAPHQL /graphql/Queries/me")
	if me == nil {
		t.Fatalf("expected endpoint for me")
	}
	if me.Props["auth_required"] != "true" {
		t.Errorf("me: auth_required = %q, want true", me.Props["auth_required"])
	}
	if me.Props["auth_method"] != "directive" {
		t.Errorf("me: auth_method = %q, want directive", me.Props["auth_method"])
	}
	if me.Props["auth_directive"] != "Authenticated" {
		t.Errorf("me: auth_directive = %q, want Authenticated", me.Props["auth_directive"])
	}
	if me.Props["auth_guard"] != "Authenticated" {
		t.Errorf("me: auth_guard = %q, want Authenticated", me.Props["auth_guard"])
	}

	// @GQLDirective(HasRole("admin")) → auth_roles=admin + auth_required=true.
	admin := findEntity(ents, "SCOPE.Operation", "GRAPHQL /graphql/Queries/adminStats")
	if admin == nil {
		t.Fatalf("expected endpoint for adminStats")
	}
	if admin.Props["auth_required"] != "true" {
		t.Errorf("adminStats: auth_required = %q, want true", admin.Props["auth_required"])
	}
	if admin.Props["auth_roles"] != "admin" {
		t.Errorf("adminStats: auth_roles = %q, want admin", admin.Props["auth_roles"])
	}
	if admin.Props["auth_directive"] != "HasRole" {
		t.Errorf("adminStats: auth_directive = %q, want HasRole", admin.Props["auth_directive"])
	}

	// NEGATIVE: @GQLDeprecated is a non-auth directive → no auth_required.
	legacy := findEntity(ents, "SCOPE.Operation", "GRAPHQL /graphql/Queries/legacy")
	if legacy == nil {
		t.Fatalf("expected endpoint for legacy")
	}
	if _, ok := legacy.Props["auth_required"]; ok {
		t.Errorf("legacy: auth_required should be absent, got %q", legacy.Props["auth_required"])
	}

	// NEGATIVE: a directive-free field carries no auth.
	health := findEntity(ents, "SCOPE.Operation", "GRAPHQL /graphql/Queries/health")
	if health == nil {
		t.Fatalf("expected endpoint for health")
	}
	if _, ok := health.Props["auth_required"]; ok {
		t.Errorf("health: auth_required should be absent, got %q", health.Props["auth_required"])
	}
}

func TestCalibanNoFalsePositive(t *testing.T) {
	// A plain http4s/tapir Scala file with no Caliban markers must yield nothing.
	src := `
import org.http4s._
import org.http4s.dsl.io._

object Routes {
  val routes = HttpRoutes.of[IO] {
    case GET -> Root / "users" => Ok("users")
  }
}
`
	ents := extract(t, "custom_scala_caliban", fi("Routes.scala", "scala", src))
	if len(ents) != 0 {
		t.Fatalf("expected no entities for non-Caliban file, got %d", len(ents))
	}
}
