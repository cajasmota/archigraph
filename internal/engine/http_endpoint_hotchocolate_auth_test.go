package engine

import (
	"testing"

	"github.com/cajasmota/grafel/internal/types"
)

// assertEndpointPropEmpty asserts the synthetic endpoint `wantID` does NOT
// carry a non-empty Properties[key].
func assertEndpointPropEmpty(t *testing.T, records []types.EntityRecord, wantID, key string) {
	t.Helper()
	for _, e := range records {
		if e.ID != wantID {
			continue
		}
		if got := e.Properties[key]; got != "" {
			t.Errorf("%s: %s=%q, want empty", wantID, key, got)
		}
		return
	}
	t.Errorf("missing synthetic endpoint %q", wantID)
}

// TestHotChocolate_MethodLevelAuthorize asserts that a HotChocolate resolver
// method decorated with [Authorize] / [Authorize(Roles=...)] /
// [Authorize(Policy=...)] has the auth verdict stamped on its GRAPHQL endpoint
// (auth_required + signal-1 auth_decorator + roles/permissions), while an
// undecorated resolver in the same class stays unauthenticated (#3961).
func TestHotChocolate_MethodLevelAuthorize(t *testing.T) {
	src := `
using HotChocolate;
using HotChocolate.Types;
using Microsoft.AspNetCore.Authorization;

[QueryType]
public class Query
{
    [Authorize]
    public User GetUser(int id) => _repo.Find(id);

    [Authorize(Roles="Admin,Owner")]
    public User GetAdmin(int id) => _repo.Find(id);

    [Authorize(Policy="CanReadReports")]
    public Report GetReport(int id) => _repo.Report(id);

    public User GetPublic(int id) => _repo.Find(id);
}
`
	_, res := runDetect(t, "csharp", "GraphQL/Query.cs", src)

	// [Authorize] → protected, signal-1 auth_decorator fires.
	assertEndpointProp(t, res.Entities, "http:GRAPHQL:/graphql/Query/user", "auth_required", "true")
	assertEndpointProp(t, res.Entities, "http:GRAPHQL:/graphql/Query/user", "auth_decorator", "Authorize")

	// [Authorize(Roles="Admin,Owner")] → auth_roles.
	assertEndpointProp(t, res.Entities, "http:GRAPHQL:/graphql/Query/admin", "auth_required", "true")
	assertEndpointProp(t, res.Entities, "http:GRAPHQL:/graphql/Query/admin", "auth_roles", "Admin,Owner")

	// [Authorize(Policy="CanReadReports")] → auth_permissions.
	assertEndpointProp(t, res.Entities, "http:GRAPHQL:/graphql/Query/report", "auth_required", "true")
	assertEndpointProp(t, res.Entities, "http:GRAPHQL:/graphql/Query/report", "auth_permissions", "CanReadReports")

	// Undecorated resolver → NO auth stamped (negative).
	assertEndpointPropEmpty(t, res.Entities, "http:GRAPHQL:/graphql/Query/public", "auth_required")
	assertEndpointPropEmpty(t, res.Entities, "http:GRAPHQL:/graphql/Query/public", "auth_decorator")
}

// TestHotChocolate_ClassLevelAuthorize asserts a type-level [Authorize] on the
// resolver class protects every field, and a method-level [AllowAnonymous]
// overrides it to public (explicit auth_required=false).
func TestHotChocolate_ClassLevelAuthorize(t *testing.T) {
	src := `
using HotChocolate.Types;
using Microsoft.AspNetCore.Authorization;

[QueryType]
[Authorize]
public class Query
{
    public User GetUser(int id) => _repo.Find(id);

    [AllowAnonymous]
    public Health GetHealth() => Health.Ok();
}
`
	_, res := runDetect(t, "csharp", "GraphQL/Secured.cs", src)

	// Inherits the class-level [Authorize].
	assertEndpointProp(t, res.Entities, "http:GRAPHQL:/graphql/Query/user", "auth_required", "true")
	assertEndpointProp(t, res.Entities, "http:GRAPHQL:/graphql/Query/user", "auth_decorator", "Authorize")

	// [AllowAnonymous] overrides the class-level protection → explicit public.
	assertEndpointProp(t, res.Entities, "http:GRAPHQL:/graphql/Query/health", "auth_required", "false")
	assertEndpointPropEmpty(t, res.Entities, "http:GRAPHQL:/graphql/Query/health", "auth_decorator")
}

// TestHotChocolate_NoAuthNoStamp asserts a HotChocolate file with NO authorize
// attributes leaves every endpoint unauthenticated (no false positives).
func TestHotChocolate_NoAuthNoStamp(t *testing.T) {
	src := `
using HotChocolate.Types;

[QueryType]
public class Query
{
    public User GetUser(int id) => _repo.Find(id);
}
`
	_, res := runDetect(t, "csharp", "GraphQL/Open.cs", src)
	assertEndpointPropEmpty(t, res.Entities, "http:GRAPHQL:/graphql/Query/user", "auth_required")
	assertEndpointPropEmpty(t, res.Entities, "http:GRAPHQL:/graphql/Query/user", "auth_decorator")
}
