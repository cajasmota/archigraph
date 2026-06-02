package java

import "testing"

// Helpers ------------------------------------------------------------------

// findGQLEndpoint returns the SCOPE.Operation endpoint entity whose Name equals
// want (the canonical "GRAPHQL /graphql/<Op>/<field>" shape), or nil.
func findGQLEndpoint(r PatternResult, want string) *SecondaryEntity {
	for i := range r.Entities {
		e := &r.Entities[i]
		if e.Kind == "SCOPE.Operation" && e.Subtype == "endpoint" && e.Name == want {
			return e
		}
	}
	return nil
}

// hasHandlesEdge reports whether the result carries a HANDLES edge from the
// endpoint with route name endpointName to a resolver whose handler_name equals
// resolverHandler.
func hasHandlesEdge(r PatternResult, endpointName, resolverHandler string) bool {
	ep := findGQLEndpoint(r, endpointName)
	if ep == nil {
		return false
	}
	// Locate the resolver entity ref by handler name.
	var resolverRef string
	for i := range r.Entities {
		e := &r.Entities[i]
		if e.Subtype == "graphql_resolver" && e.Name == resolverHandler {
			resolverRef = e.Ref
			break
		}
	}
	if resolverRef == "" {
		return false
	}
	for _, rel := range r.Relationships {
		if rel.RelationshipType == "HANDLES" && rel.SourceRef == ep.Ref && rel.TargetRef == resolverRef {
			return true
		}
	}
	return false
}

// Spring for GraphQL -------------------------------------------------------

const springGraphQLSrc = `package com.example;

import org.springframework.graphql.data.method.annotation.QueryMapping;
import org.springframework.graphql.data.method.annotation.MutationMapping;
import org.springframework.graphql.data.method.annotation.SubscriptionMapping;
import org.springframework.graphql.data.method.annotation.SchemaMapping;
import org.springframework.stereotype.Controller;

@Controller
public class UserController {

    @QueryMapping
    public List<User> users() { return repo.all(); }

    @MutationMapping
    public User createUser(@Argument NewUser input) { return repo.save(input); }

    @SubscriptionMapping
    public Flux<Event> events() { return bus.stream(); }

    @QueryMapping(name = "allUsers")
    public List<User> usersAlias() { return repo.all(); }

    @SchemaMapping(typeName = "Query", field = "node")
    public Node node(@Argument String id) { return repo.node(id); }

    // Per-type field resolver — NOT a root operation. Must be skipped.
    @SchemaMapping(typeName = "User", field = "orders")
    public List<Order> userOrders(User user) { return user.getOrders(); }
}
`

func TestSpringGraphQL_QueryMapping(t *testing.T) {
	r := ExtractSpringGraphQL(PatternContext{Source: springGraphQLSrc, Language: "java", Framework: "spring_graphql", FilePath: "UserController.java"})

	e := findGQLEndpoint(r, "GRAPHQL /graphql/Query/users")
	if e == nil {
		t.Fatal("expected canonical endpoint GRAPHQL /graphql/Query/users")
	}
	if got := e.Properties["route_path"]; got != "/graphql/Query/users" {
		t.Errorf("route_path = %v, want /graphql/Query/users", got)
	}
	if got := e.Properties["verb"]; got != "GRAPHQL" {
		t.Errorf("verb = %v, want GRAPHQL", got)
	}
	if got := e.Properties["http_method"]; got != "GRAPHQL" {
		t.Errorf("http_method = %v, want GRAPHQL", got)
	}
	if got := e.Properties["graphql_operation"]; got != "Query" {
		t.Errorf("graphql_operation = %v, want Query", got)
	}
	if got := e.Properties["graphql_field"]; got != "users" {
		t.Errorf("graphql_field = %v, want users", got)
	}
	if got := e.Properties["handler_name"]; got != "UserController.users" {
		t.Errorf("handler_name = %v, want UserController.users", got)
	}
	if !hasHandlesEdge(r, "GRAPHQL /graphql/Query/users", "UserController.users") {
		t.Error("expected HANDLES edge endpoint -> UserController.users")
	}
}

func TestSpringGraphQL_MutationAndSubscription(t *testing.T) {
	r := ExtractSpringGraphQL(PatternContext{Source: springGraphQLSrc, Language: "java", Framework: "spring_graphql", FilePath: "UserController.java"})

	if findGQLEndpoint(r, "GRAPHQL /graphql/Mutation/createUser") == nil {
		t.Error("expected GRAPHQL /graphql/Mutation/createUser")
	}
	if findGQLEndpoint(r, "GRAPHQL /graphql/Subscription/events") == nil {
		t.Error("expected GRAPHQL /graphql/Subscription/events")
	}
	if !hasHandlesEdge(r, "GRAPHQL /graphql/Mutation/createUser", "UserController.createUser") {
		t.Error("expected HANDLES edge for createUser")
	}
}

func TestSpringGraphQL_NameOverride(t *testing.T) {
	r := ExtractSpringGraphQL(PatternContext{Source: springGraphQLSrc, Language: "java", Framework: "spring_graphql", FilePath: "UserController.java"})

	// @QueryMapping(name="allUsers") on method usersAlias → field is allUsers.
	e := findGQLEndpoint(r, "GRAPHQL /graphql/Query/allUsers")
	if e == nil {
		t.Fatal("expected renamed field GRAPHQL /graphql/Query/allUsers")
	}
	if got := e.Properties["resolver_method"]; got != "usersAlias" {
		t.Errorf("resolver_method = %v, want usersAlias", got)
	}
	// The un-renamed method-named endpoint must NOT exist.
	if findGQLEndpoint(r, "GRAPHQL /graphql/Query/usersAlias") != nil {
		t.Error("un-renamed usersAlias endpoint should not exist after name override")
	}
}

func TestSpringGraphQL_SchemaMappingExplicitRoot(t *testing.T) {
	r := ExtractSpringGraphQL(PatternContext{Source: springGraphQLSrc, Language: "java", Framework: "spring_graphql", FilePath: "UserController.java"})

	// @SchemaMapping(typeName="Query", field="node") → root op endpoint.
	if findGQLEndpoint(r, "GRAPHQL /graphql/Query/node") == nil {
		t.Error("expected GRAPHQL /graphql/Query/node from @SchemaMapping root")
	}
	// @SchemaMapping(typeName="User", ...) is a field resolver — must be absent.
	if findGQLEndpoint(r, "GRAPHQL /graphql/User/orders") != nil {
		t.Error("per-type field resolver @SchemaMapping(typeName=User) must not emit a root endpoint")
	}
	if findGQLEndpoint(r, "GRAPHQL /graphql/Query/orders") != nil {
		t.Error("field resolver must not be misattributed to Query")
	}
}

// Netflix DGS --------------------------------------------------------------

const dgsSrc = `package com.example;

import com.netflix.graphql.dgs.DgsComponent;
import com.netflix.graphql.dgs.DgsQuery;
import com.netflix.graphql.dgs.DgsMutation;
import com.netflix.graphql.dgs.DgsSubscription;
import com.netflix.graphql.dgs.DgsData;

@DgsComponent
public class UserDataFetcher {

    @DgsQuery
    public User user(@InputArgument String id) { return svc.get(id); }

    @DgsMutation
    public User addUser(@InputArgument NewUser in) { return svc.add(in); }

    @DgsSubscription
    public Publisher<Event> events() { return svc.stream(); }

    @DgsQuery(field = "allUsers")
    public List<User> users() { return svc.all(); }

    @DgsData(parentType = "Query", field = "search")
    public List<User> searchUsers(@InputArgument String q) { return svc.search(q); }

    // Field resolver on a non-root type — must be skipped.
    @DgsData(parentType = "User", field = "orders")
    public List<Order> orders(DgsDataFetchingEnvironment env) { return svc.orders(); }
}
`

func TestDGS_ShorthandOperations(t *testing.T) {
	r := ExtractSpringGraphQL(PatternContext{Source: dgsSrc, Language: "java", Framework: "dgs", FilePath: "UserDataFetcher.java"})

	e := findGQLEndpoint(r, "GRAPHQL /graphql/Query/user")
	if e == nil {
		t.Fatal("expected GRAPHQL /graphql/Query/user from @DgsQuery")
	}
	if got := e.Properties["route_path"]; got != "/graphql/Query/user" {
		t.Errorf("route_path = %v, want /graphql/Query/user", got)
	}
	if got := e.Properties["framework"]; got != "dgs" {
		t.Errorf("framework = %v, want dgs", got)
	}
	if got := e.Properties["handler_name"]; got != "UserDataFetcher.user" {
		t.Errorf("handler_name = %v, want UserDataFetcher.user", got)
	}
	if !hasHandlesEdge(r, "GRAPHQL /graphql/Query/user", "UserDataFetcher.user") {
		t.Error("expected HANDLES edge endpoint -> UserDataFetcher.user")
	}
	if findGQLEndpoint(r, "GRAPHQL /graphql/Mutation/addUser") == nil {
		t.Error("expected GRAPHQL /graphql/Mutation/addUser from @DgsMutation")
	}
	if findGQLEndpoint(r, "GRAPHQL /graphql/Subscription/events") == nil {
		t.Error("expected GRAPHQL /graphql/Subscription/events from @DgsSubscription")
	}
}

func TestDGS_FieldOverrideAndDgsData(t *testing.T) {
	r := ExtractSpringGraphQL(PatternContext{Source: dgsSrc, Language: "java", Framework: "dgs", FilePath: "UserDataFetcher.java"})

	// @DgsQuery(field="allUsers") on method users → field allUsers.
	if findGQLEndpoint(r, "GRAPHQL /graphql/Query/allUsers") == nil {
		t.Error("expected GRAPHQL /graphql/Query/allUsers from @DgsQuery(field=...)")
	}
	if findGQLEndpoint(r, "GRAPHQL /graphql/Query/users") != nil {
		t.Error("un-renamed users endpoint should not exist after field override")
	}
	// @DgsData(parentType="Query", field="search") → root endpoint.
	if findGQLEndpoint(r, "GRAPHQL /graphql/Query/search") == nil {
		t.Error("expected GRAPHQL /graphql/Query/search from @DgsData root")
	}
}

func TestDGS_FieldResolverSkipped(t *testing.T) {
	r := ExtractSpringGraphQL(PatternContext{Source: dgsSrc, Language: "java", Framework: "dgs", FilePath: "UserDataFetcher.java"})

	// @DgsData(parentType="User", field="orders") is a field resolver, not a root.
	if findGQLEndpoint(r, "GRAPHQL /graphql/User/orders") != nil {
		t.Error("@DgsData(parentType=User) field resolver must not emit a root endpoint")
	}
	if findGQLEndpoint(r, "GRAPHQL /graphql/Query/orders") != nil {
		t.Error("field resolver must not be misattributed to Query")
	}
}

// Gating -------------------------------------------------------------------

func TestSpringGraphQL_GatesOffNonGraphQL(t *testing.T) {
	plain := `package com.example;
@RestController
public class PlainController {
    @GetMapping("/users")
    public List<User> users() { return repo.all(); }
}`
	r := ExtractSpringGraphQL(PatternContext{Source: plain, Language: "java", Framework: "spring_boot", FilePath: "PlainController.java"})
	if len(r.Entities) != 0 {
		t.Errorf("expected no GraphQL entities for plain Spring MVC controller, got %d", len(r.Entities))
	}
}

func TestSpringGraphQL_GatesOffWrongFramework(t *testing.T) {
	r := ExtractSpringGraphQL(PatternContext{Source: springGraphQLSrc, Language: "java", Framework: "django", FilePath: "X.java"})
	if len(r.Entities) != 0 {
		t.Errorf("expected no entities when framework gate rejects, got %d", len(r.Entities))
	}
}

// Auth — nested-paren @PreAuthorize regression (#3873) -----------------------
//
// Before the annArgsRe fix, a SpEL @PreAuthorize interleaved between the
// mapping annotation and the resolver method (the COMMON Spring auth form)
// dropped the WHOLE endpoint, because the interleaved-annotation tolerance used
// `\([^)]*\)` and stopped at the first inner `)` of hasRole('ADMIN').

func TestSpringGraphQLAuth_PreAuthorizeNestedParen_Issue3873(t *testing.T) {
	src := `package com.example;
import org.springframework.graphql.data.method.annotation.QueryMapping;
import org.springframework.graphql.data.method.annotation.MutationMapping;
import org.springframework.security.access.prepost.PreAuthorize;
import org.springframework.stereotype.Controller;
@Controller
public class UserController {
    @QueryMapping
    @PreAuthorize("hasRole('ADMIN')")
    public User user(@Argument Long id) { return repo.get(id); }

    @PreAuthorize("hasRole('MANAGER') and hasAuthority('SCOPE_write')")
    @MutationMapping
    public User createUser(@Argument NewUser input) { return repo.save(input); }
}`
	r := ExtractSpringGraphQL(PatternContext{Source: src, Language: "java", Framework: "spring_graphql", FilePath: "UserController.java"})

	// Endpoint survives the SpEL @PreAuthorize and is stamped with role ADMIN.
	e := findGQLEndpoint(r, "GRAPHQL /graphql/Query/user")
	if e == nil {
		t.Fatal("endpoint dropped by interleaved @PreAuthorize (nested-paren regression)")
	}
	if got := propStr(e, "auth_required"); got != "true" {
		t.Errorf("user auth_required = %q, want \"true\"", got)
	}
	if got := propStr(e, "auth_roles"); got != "ADMIN" {
		t.Errorf("user auth_roles = %q, want \"ADMIN\"", got)
	}

	// @PreAuthorize BEFORE the mapping with role+authority also survives.
	c := findGQLEndpoint(r, "GRAPHQL /graphql/Mutation/createUser")
	if c == nil {
		t.Fatal("createUser endpoint dropped")
	}
	if got := propStr(c, "auth_roles"); got != "MANAGER" {
		t.Errorf("createUser auth_roles = %q, want \"MANAGER\"", got)
	}
	if got := propStr(c, "auth_scopes"); got != "write" {
		t.Errorf("createUser auth_scopes = %q, want \"write\"", got)
	}
}

// Request/response shapes (#3873) ---------------------------------------------
//
// A resolver's DTO-typed @Argument/@InputArgument params are the request shape
// (ACCEPTS_INPUT) and the unwrapped return type is the response shape
// (RETURNS), mirroring the Spring MVC @RequestBody / return-type contract.

// gqlShapeRel reports whether result carries a rel of type relType from the
// endpoint named endpointName to a spring_dto schema for dtoName.
func gqlShapeRel(r PatternResult, endpointName, relType, dtoName string) bool {
	ep := findGQLEndpoint(r, endpointName)
	if ep == nil {
		return false
	}
	for _, rel := range r.Relationships {
		if rel.RelationshipType == relType && rel.SourceRef == ep.Ref &&
			rel.Properties["dto_type"] == dtoName {
			return true
		}
	}
	return false
}

func TestSpringGraphQLShapes_RequestAndResponse_Issue3873(t *testing.T) {
	src := `package com.example;
import org.springframework.graphql.data.method.annotation.QueryMapping;
import org.springframework.graphql.data.method.annotation.MutationMapping;
import org.springframework.stereotype.Controller;
@Controller
public class UserController {
    @QueryMapping
    public User user(@Argument Long id) { return repo.get(id); }

    @MutationMapping
    public User createUser(@Argument NewUser input) { return repo.save(input); }

    @QueryMapping
    public List<User> users() { return repo.all(); }
}`
	r := ExtractSpringGraphQL(PatternContext{Source: src, Language: "java", Framework: "spring_graphql", FilePath: "UserController.java"})

	// Mutation request shape: @Argument NewUser input → ACCEPTS_INPUT NewUser.
	if !gqlShapeRel(r, "GRAPHQL /graphql/Mutation/createUser", "ACCEPTS_INPUT", "NewUser") {
		t.Error("expected ACCEPTS_INPUT createUser -> NewUser")
	}
	// Mutation response shape: returns User → RETURNS User.
	if !gqlShapeRel(r, "GRAPHQL /graphql/Mutation/createUser", "RETURNS", "User") {
		t.Error("expected RETURNS createUser -> User")
	}
	// Query response shape through List<User> → RETURNS User (unwrapped).
	if !gqlShapeRel(r, "GRAPHQL /graphql/Query/users", "RETURNS", "User") {
		t.Error("expected RETURNS users -> User (List<User> unwrapped)")
	}
	// NEGATIVE: a scalar @Argument Long id is NOT a request DTO.
	for _, rel := range r.Relationships {
		if rel.RelationshipType == "ACCEPTS_INPUT" &&
			(rel.Properties["dto_type"] == "Long" || rel.Properties["dto_type"] == "id") {
			t.Errorf("scalar @Argument should not produce ACCEPTS_INPUT, got %v", rel.Properties)
		}
	}
	// user(...) takes only a scalar arg → no ACCEPTS_INPUT on its endpoint.
	uep := findGQLEndpoint(r, "GRAPHQL /graphql/Query/user")
	for _, rel := range r.Relationships {
		if rel.RelationshipType == "ACCEPTS_INPUT" && uep != nil && rel.SourceRef == uep.Ref {
			t.Errorf("scalar-only resolver user should have no ACCEPTS_INPUT, got %v", rel.Properties)
		}
	}
}

func TestDGSShapes_InputArgumentAndReturn_Issue3873(t *testing.T) {
	src := `package com.example;
import com.netflix.graphql.dgs.DgsComponent;
import com.netflix.graphql.dgs.DgsQuery;
import com.netflix.graphql.dgs.DgsMutation;
@DgsComponent
public class UserFetcher {
    @DgsMutation
    public User addUser(@InputArgument NewUser in) { return svc.add(in); }

    @DgsQuery
    public List<User> users() { return svc.all(); }
}`
	r := ExtractSpringGraphQL(PatternContext{Source: src, Language: "java", Framework: "dgs", FilePath: "UserFetcher.java"})

	if !gqlShapeRel(r, "GRAPHQL /graphql/Mutation/addUser", "ACCEPTS_INPUT", "NewUser") {
		t.Error("expected ACCEPTS_INPUT addUser -> NewUser (@InputArgument)")
	}
	if !gqlShapeRel(r, "GRAPHQL /graphql/Mutation/addUser", "RETURNS", "User") {
		t.Error("expected RETURNS addUser -> User")
	}
	if !gqlShapeRel(r, "GRAPHQL /graphql/Query/users", "RETURNS", "User") {
		t.Error("expected RETURNS users -> User")
	}
}

// Endpoint-shape parity: the emitted id/name MUST match the gqlgen / JS / Kotlin
// canonical "GRAPHQL /graphql/<RootType>/<field>" exactly.
func TestSpringGraphQL_CanonicalShapeParity(t *testing.T) {
	r := ExtractSpringGraphQL(PatternContext{Source: springGraphQLSrc, Language: "java", Framework: "spring_graphql", FilePath: "UserController.java"})
	for _, want := range []string{
		"GRAPHQL /graphql/Query/users",
		"GRAPHQL /graphql/Mutation/createUser",
		"GRAPHQL /graphql/Subscription/events",
	} {
		e := findGQLEndpoint(r, want)
		if e == nil {
			t.Errorf("missing canonical endpoint %q", want)
			continue
		}
		// route_path must be exactly the path portion of the name.
		wantPath := want[len("GRAPHQL "):]
		if got := e.Properties["route_path"]; got != wantPath {
			t.Errorf("%s: route_path = %v, want %s", want, got, wantPath)
		}
	}
}
