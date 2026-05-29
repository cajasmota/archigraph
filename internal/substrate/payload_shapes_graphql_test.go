// Tests for the GraphQL SDL payload-shape sniffer (#3076).
//
// These tests prove schema_drift_detection for the GraphQL-resolvers record:
// the sniffer correctly extracts request shapes from input types and response
// shapes from object types declared in SDL files.  The test fixture lives at
// testdata/fixtures/graphql/schema.graphql.
package substrate

import (
	"os"
	"path/filepath"
	"reflect"
	"testing"
)

// graphqlFixtureContent reads a fixture file from testdata/fixtures/graphql/
// relative to the module root (two levels up from internal/substrate/).
func graphqlFixtureContent(t *testing.T, relPath string) string {
	t.Helper()
	root := filepath.Join("..", "..", "testdata", "fixtures", "graphql")
	full := filepath.Join(root, relPath)
	b, err := os.ReadFile(full)
	if err != nil {
		t.Fatalf("graphql fixture %q not found: %v", full, err)
	}
	return string(b)
}

// TestPayloadShapesGraphQL_InputType proves that an SDL input type definition
// is extracted as a ProducerRequest shape with all declared field names.
func TestPayloadShapesGraphQL_InputType(t *testing.T) {
	src := graphqlFixtureContent(t, "schema.graphql")
	shapes := sniffPayloadShapesGraphQL(src)

	s := findShape(shapes, "CreateUserInput", PayloadDirectionRequest, PayloadSideProducer)
	if s == nil {
		t.Fatalf("expected ProducerRequest shape for CreateUserInput; got shapes: %+v", shapes)
	}
	want := []string{"age", "email", "name", "role"}
	if got := sortedNames(s.Fields); !reflect.DeepEqual(got, want) {
		t.Errorf("CreateUserInput fields: want %v got %v", want, got)
	}
	if s.Confidence != 1.0 {
		t.Errorf("input type confidence: want 1.0 got %v", s.Confidence)
	}
}

// TestPayloadShapesGraphQL_UpdateInputType proves that a second input type is
// also extracted correctly with its own partial field set.
func TestPayloadShapesGraphQL_UpdateInputType(t *testing.T) {
	src := graphqlFixtureContent(t, "schema.graphql")
	shapes := sniffPayloadShapesGraphQL(src)

	s := findShape(shapes, "UpdateUserInput", PayloadDirectionRequest, PayloadSideProducer)
	if s == nil {
		t.Fatalf("expected ProducerRequest shape for UpdateUserInput; got shapes: %+v", shapes)
	}
	want := []string{"age", "email", "name"}
	if got := sortedNames(s.Fields); !reflect.DeepEqual(got, want) {
		t.Errorf("UpdateUserInput fields: want %v got %v", want, got)
	}
}

// TestPayloadShapesGraphQL_ObjectType proves that an SDL object type definition
// is extracted as a ProducerResponse shape with all declared field names.
func TestPayloadShapesGraphQL_ObjectType(t *testing.T) {
	src := graphqlFixtureContent(t, "schema.graphql")
	shapes := sniffPayloadShapesGraphQL(src)

	s := findShape(shapes, "User", PayloadDirectionResponse, PayloadSideProducer)
	if s == nil {
		t.Fatalf("expected ProducerResponse shape for User; got shapes: %+v", shapes)
	}
	want := []string{"age", "email", "id", "name", "role"}
	if got := sortedNames(s.Fields); !reflect.DeepEqual(got, want) {
		t.Errorf("User response fields: want %v got %v", want, got)
	}
	if s.Confidence != 1.0 {
		t.Errorf("object type confidence: want 1.0 got %v", s.Confidence)
	}
}

// TestPayloadShapesGraphQL_DeleteResultObjectType proves a smaller object type
// is extracted correctly.
func TestPayloadShapesGraphQL_DeleteResultObjectType(t *testing.T) {
	src := graphqlFixtureContent(t, "schema.graphql")
	shapes := sniffPayloadShapesGraphQL(src)

	s := findShape(shapes, "DeleteResult", PayloadDirectionResponse, PayloadSideProducer)
	if s == nil {
		t.Fatalf("expected ProducerResponse shape for DeleteResult; got shapes: %+v", shapes)
	}
	want := []string{"deletedId", "ok"}
	if got := sortedNames(s.Fields); !reflect.DeepEqual(got, want) {
		t.Errorf("DeleteResult response fields: want %v got %v", want, got)
	}
}

// TestPayloadShapesGraphQL_QueryOperationArgs proves that inline argument lists
// on Query operations are extracted as ProducerRequest shapes attributed to the
// canonical "Query.<opName>" function name.
func TestPayloadShapesGraphQL_QueryOperationArgs(t *testing.T) {
	src := graphqlFixtureContent(t, "schema.graphql")
	shapes := sniffPayloadShapesGraphQL(src)

	// Query.user(id: ID!)
	s := findShape(shapes, "Query.user", PayloadDirectionRequest, PayloadSideProducer)
	if s == nil {
		t.Fatalf("expected ProducerRequest shape for Query.user; got shapes: %+v", shapes)
	}
	want := []string{"id"}
	if got := sortedNames(s.Fields); !reflect.DeepEqual(got, want) {
		t.Errorf("Query.user args: want %v got %v", want, got)
	}

	// Query.users(limit: Int, offset: Int)
	su := findShape(shapes, "Query.users", PayloadDirectionRequest, PayloadSideProducer)
	if su == nil {
		t.Fatalf("expected ProducerRequest shape for Query.users; got shapes: %+v", shapes)
	}
	wantU := []string{"limit", "offset"}
	if got := sortedNames(su.Fields); !reflect.DeepEqual(got, wantU) {
		t.Errorf("Query.users args: want %v got %v", wantU, got)
	}
}

// TestPayloadShapesGraphQL_MutationOperationArgs proves that Mutation operation
// inline args are captured as ProducerRequest shapes.
func TestPayloadShapesGraphQL_MutationOperationArgs(t *testing.T) {
	src := graphqlFixtureContent(t, "schema.graphql")
	shapes := sniffPayloadShapesGraphQL(src)

	// Mutation.updateUser(id: ID!, input: UpdateUserInput!)
	s := findShape(shapes, "Mutation.updateUser", PayloadDirectionRequest, PayloadSideProducer)
	if s == nil {
		t.Fatalf("expected ProducerRequest shape for Mutation.updateUser; got shapes: %+v", shapes)
	}
	want := []string{"id", "input"}
	if got := sortedNames(s.Fields); !reflect.DeepEqual(got, want) {
		t.Errorf("Mutation.updateUser args: want %v got %v", want, got)
	}
}

// TestPayloadShapesGraphQL_InlineSDL proves the sniffer works on inline SDL
// content (not just from a file) for the canonical single-type case.
func TestPayloadShapesGraphQL_InlineSDL(t *testing.T) {
	const src = `
input LoginInput {
  username: String!
  password: String!
}

type AuthResult {
  token: String!
  expiresIn: Int!
}

type Mutation {
  login(input: LoginInput!): AuthResult
}
`
	shapes := sniffPayloadShapesGraphQL(src)

	req := findShape(shapes, "LoginInput", PayloadDirectionRequest, PayloadSideProducer)
	if req == nil {
		t.Fatalf("expected ProducerRequest shape for LoginInput; got %+v", shapes)
	}
	wantReq := []string{"password", "username"}
	if got := sortedNames(req.Fields); !reflect.DeepEqual(got, wantReq) {
		t.Errorf("LoginInput fields: want %v got %v", wantReq, got)
	}

	resp := findShape(shapes, "AuthResult", PayloadDirectionResponse, PayloadSideProducer)
	if resp == nil {
		t.Fatalf("expected ProducerResponse shape for AuthResult; got %+v", shapes)
	}
	wantResp := []string{"expiresIn", "token"}
	if got := sortedNames(resp.Fields); !reflect.DeepEqual(got, wantResp) {
		t.Errorf("AuthResult fields: want %v got %v", wantResp, got)
	}

	// Mutation.login(input: LoginInput!)
	arg := findShape(shapes, "Mutation.login", PayloadDirectionRequest, PayloadSideProducer)
	if arg == nil {
		t.Fatalf("expected ProducerRequest shape for Mutation.login; got %+v", shapes)
	}
	wantArg := []string{"input"}
	if got := sortedNames(arg.Fields); !reflect.DeepEqual(got, wantArg) {
		t.Errorf("Mutation.login args: want %v got %v", wantArg, got)
	}
}

// TestPayloadShapesGraphQL_Empty proves the sniffer is nil-safe on empty input.
func TestPayloadShapesGraphQL_Empty(t *testing.T) {
	shapes := sniffPayloadShapesGraphQL("")
	if len(shapes) != 0 {
		t.Errorf("expected no shapes for empty input; got %d", len(shapes))
	}
}

// TestLanguageForPath_GraphQL proves that .graphql and .gql extensions are
// mapped to the "graphql" sniffer slug by LanguageForPath.
func TestLanguageForPath_GraphQL(t *testing.T) {
	cases := []struct {
		path string
		want string
	}{
		{"schema.graphql", "graphql"},
		{"types.gql", "graphql"},
		{"src/api/schema.graphql", "graphql"},
		{"src/api/types.gql", "graphql"},
		{"src/api/resolver.ts", "jsts"}, // sanity-check: .ts still maps correctly
	}
	for _, c := range cases {
		if got := LanguageForPath(c.path); got != c.want {
			t.Errorf("LanguageForPath(%q) = %q, want %q", c.path, got, c.want)
		}
	}
}
