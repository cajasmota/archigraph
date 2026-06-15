package engine

import (
	"sort"
	"testing"

	"github.com/cajasmota/grafel/internal/types"
)

// findGQLClientEntity returns the http_endpoint_call entity with the given ID,
// or nil. Used to assert the client-call ID matches the server endpoint shape.
func findGQLClientEntity(res *DetectResult, id string) *entityForTest {
	for i := range res.Entities {
		e := res.Entities[i]
		if e.ID == id && e.Kind == httpEndpointCallKind {
			return &entityForTest{
				id:           e.ID,
				name:         e.Name,
				qn:           e.QualifiedName,
				patternType:  e.Properties["pattern_type"],
				sourceCaller: e.Properties["source_caller"],
				verb:         e.Properties["verb"],
				framework:    e.Properties["framework"],
			}
		}
	}
	return nil
}

type entityForTest struct {
	id, name, qn, patternType, sourceCaller, verb, framework string
}

// TestSynth_GraphQLClient_ApolloUseQuery is the core parity-oracle test: a
// client `useQuery(gql\`query GetUsers { users { id } }\`)` must emit a
// client-call whose ID EXACTLY matches the server-side resolver endpoint
// (http:GRAPHQL:/graphql/Query/users) so the cross-repo linker joins them, plus
// a source_caller carrying the enclosing component for the FETCHES edge.
func TestSynth_GraphQLClient_ApolloUseQuery(t *testing.T) {
	src := `
import { gql, useQuery } from '@apollo/client';

const GET_USERS = gql` + "`" + `
  query GetUsers {
    users {
      id
      name
    }
  }
` + "`" + `;

function UserList() {
  const { data } = useQuery(GET_USERS);
  return data;
}
`
	_, res := runDetect(t, "typescript", "UserList.tsx", src)

	// The server side (synthesizeGraphQLResolvers) emits exactly this ID for the
	// `users` field under the Query root. The client MUST match it.
	const wantID = "http:GRAPHQL:/graphql/Query/users"
	e := findGQLClientEntity(res, wantID)
	if e == nil {
		var got []string
		for _, x := range res.Entities {
			if x.Kind == httpEndpointCallKind {
				got = append(got, x.ID)
			}
		}
		sort.Strings(got)
		t.Fatalf("missing client-call %q matching server endpoint shape (got calls: %v)", wantID, got)
	}
	if e.verb != "GRAPHQL" {
		t.Errorf("verb = %q, want GRAPHQL (must match server synthetic verb)", e.verb)
	}
	if e.patternType != "http_endpoint_client_synthesis" {
		t.Errorf("pattern_type = %q, want http_endpoint_client_synthesis", e.patternType)
	}
	// FETCHES edge precursor: source_caller is the enclosing component.
	if e.sourceCaller != "Function:UserList" {
		t.Errorf("source_caller = %q, want Function:UserList (drives FETCHES edge)", e.sourceCaller)
	}
	// QN must equal the ID so cross-repo joins land (issue #1725).
	if e.qn != wantID {
		t.Errorf("qualified_name = %q, want %q", e.qn, wantID)
	}
}

// TestSynth_GraphQLClient_FetchesEdge proves the source_caller resolves into a
// real FETCHES edge from the enclosing component to the GraphQL client-call,
// end-to-end through ResolveHTTPEndpointHandlers.
func TestSynth_GraphQLClient_FetchesEdge(t *testing.T) {
	src := `
import { gql, useMutation } from '@apollo/client';

const CREATE_USER = gql` + "`" + `
  mutation CreateUser($name: String!) {
    createUser(name: $name) {
      id
    }
  }
` + "`" + `;

function CreateForm() {
  const [run] = useMutation(CREATE_USER);
  return run;
}
`
	_, res := runDetect(t, "typescript", "CreateForm.tsx", src)

	// Mutation root field createUser → server shape.
	const wantID = "http:GRAPHQL:/graphql/Mutation/createUser"
	call := findGQLClientEntity(res, wantID)
	if call == nil {
		t.Fatalf("missing client-call %q for mutation root field", wantID)
	}
	if call.sourceCaller != "Function:CreateForm" {
		t.Fatalf("source_caller = %q, want Function:CreateForm", call.sourceCaller)
	}

	// In the full pipeline the JS extractor emits a Function entity for the
	// enclosing component; ResolveHTTPEndpointHandlers then turns the
	// source_caller into a real FETCHES edge. The detector-only harness does
	// not run AST extraction, so we supply the Function entity explicitly (same
	// approach as TestResolveCallers_EmitsFetchesEdge) to prove the edge forms.
	caller := types.EntityRecord{
		Kind:       "Function",
		Name:       "CreateForm",
		SourceFile: "CreateForm.tsx",
		Language:   "typescript",
	}
	merged := append([]types.EntityRecord{caller}, res.Entities...)
	out, stats := ResolveHTTPEndpointHandlers(merged)
	if stats.CallerResolved < 1 {
		t.Errorf("expected >=1 caller_resolved, got %d", stats.CallerResolved)
	}
	foundFetch := false
	for _, e := range out {
		for _, r := range e.Relationships {
			if r.Kind == "FETCHES" && r.ToID == "http_endpoint_call:"+wantID {
				if r.FromID != "Function:CreateForm" {
					t.Errorf("FETCHES from = %q, want Function:CreateForm", r.FromID)
				}
				foundFetch = true
			}
		}
	}
	if !foundFetch {
		t.Errorf("expected FETCHES edge Function:CreateForm → http_endpoint_call:%s", wantID)
	}
}

// TestSynth_GraphQLClient_ClientQueryObjectForm covers the imperative
// `client.query({ query: GET_X })` Apollo form and graphql-request inline docs.
func TestSynth_GraphQLClient_ClientQueryObjectForm(t *testing.T) {
	src := `
import { gql } from 'graphql-request';

const GET_ORDERS = gql` + "`" + `query { orders { id } }` + "`" + `;

async function load(client) {
  const a = await client.query({ query: GET_ORDERS });
  const b = await request('http://api/graphql', ` + "`" + `{ products { id } }` + "`" + `);
  return [a, b];
}
`
	got, _ := runDetect(t, "typescript", "load.ts", src)

	want := []string{
		"http:GRAPHQL:/graphql/Query/orders",   // client.query({ query: GET_ORDERS })
		"http:GRAPHQL:/graphql/Query/products", // request(endpoint, `{ products { id } }`)
	}
	requireContains(t, got, want, "graphql-client-object-and-inline")
}

// TestSynth_GraphQLClient_MultiRootField asserts one client-call per root field
// when a single operation selects several roots, matching the server's
// per-field endpoint emission.
func TestSynth_GraphQLClient_MultiRootField(t *testing.T) {
	src := `
import { gql, useQuery } from 'urql';

const DASH = gql` + "`" + `
  query Dashboard {
    me { id }
    notifications { count }
  }
` + "`" + `;

function Dashboard() {
  const [res] = useQuery({ query: DASH });
  return res;
}
`
	got, _ := runDetect(t, "typescript", "Dashboard.tsx", src)
	want := []string{
		"http:GRAPHQL:/graphql/Query/me",
		"http:GRAPHQL:/graphql/Query/notifications",
	}
	requireContains(t, got, want, "graphql-client-multi-root-field")
}

// TestSynth_GraphQLClient_NoFalsePositive verifies a plain REST file with a
// `query` variable does NOT fabricate GraphQL client-calls.
func TestSynth_GraphQLClient_NoFalsePositive(t *testing.T) {
	src := `
function search(query) {
  return fetch('/api/search?q=' + query);
}
`
	got, _ := runDetect(t, "typescript", "search.ts", src)
	requireNotContains(t, got, []string{
		"http:GRAPHQL:/graphql/Query/query",
		"http:GRAPHQL:/graphql/Query/search",
	}, "graphql-client-no-false-positive")
}

// TestSynth_GraphQLClient_RelayInline asserts Relay's primary hook idiom — an
// inline `graphql\`…\“ operation passed as the FIRST positional argument to
// useLazyLoadQuery — emits the per-root-field client-call keyed to the server
// endpoint shape, plus the FETCHES source_caller. (graphql-tagged inline docs
// were already recognised; this pins the Relay hook + caller attribution.)
func TestSynth_GraphQLClient_RelayInline(t *testing.T) {
	src := `
import { graphql, useLazyLoadQuery } from 'react-relay';

function UserScreen() {
  const data = useLazyLoadQuery(graphql` + "`" + `
    query UserScreenQuery {
      user { id name }
    }
  ` + "`" + `, {});
  return data;
}
`
	_, res := runDetect(t, "typescript", "UserScreen.tsx", src)
	const wantID = "http:GRAPHQL:/graphql/Query/user"
	e := findGQLClientEntity(res, wantID)
	if e == nil {
		t.Fatalf("missing Relay client-call %q", wantID)
	}
	if e.verb != "GRAPHQL" {
		t.Errorf("verb = %q, want GRAPHQL", e.verb)
	}
	if e.sourceCaller != "Function:UserScreen" {
		t.Errorf("source_caller = %q, want Function:UserScreen (drives FETCHES)", e.sourceCaller)
	}
}

// TestSynth_GraphQLClient_RelayConstRef is the regression that motivated this
// change: a Relay query bound to a `graphql\`…\“ const and consumed by a
// FIRST-positional-arg hook (usePreloadedQuery(Q, ref)) previously emitted
// NOTHING because usePreloadedQuery was not a recognised hook keyword — the
// const→server-field link was lost. It must now resolve to the per-root-field
// endpoint via the gql-const symbol table.
func TestSynth_GraphQLClient_RelayConstRef(t *testing.T) {
	src := `
import { graphql, usePreloadedQuery } from 'react-relay';

const AppQuery = graphql` + "`" + `
  query AppQuery {
    viewer { id }
    notifications { count }
  }
` + "`" + `;

function App(props) {
  const data = usePreloadedQuery(AppQuery, props.queryRef);
  return data;
}
`
	got, res := runDetect(t, "typescript", "App.tsx", src)
	want := []string{
		"http:GRAPHQL:/graphql/Query/viewer",
		"http:GRAPHQL:/graphql/Query/notifications",
	}
	requireContains(t, got, want, "graphql-client-relay-const-ref")
	// FETCHES caller must be the consuming component, not the top-level const def.
	if e := findGQLClientEntity(res, "http:GRAPHQL:/graphql/Query/viewer"); e != nil &&
		e.sourceCaller != "Function:App" {
		t.Errorf("source_caller = %q, want Function:App", e.sourceCaller)
	}
}

// TestSynth_GraphQLClient_RelayFragmentNoOp asserts a Relay FRAGMENT hook
// (useFragment) over a `graphql\`fragment …\“ document emits NOTHING: a
// fragment has no operation root field, so there is no server endpoint to link
// to. This guards against the Relay-hook widening fabricating a phantom field.
func TestSynth_GraphQLClient_RelayFragmentNoOp(t *testing.T) {
	src := `
import { graphql, useFragment } from 'react-relay';

const UserFrag = graphql` + "`" + `
  fragment UserCard_user on User { id name }
` + "`" + `;

function UserCard(props) {
  const user = useFragment(UserFrag, props.user);
  return user;
}
`
	got, _ := runDetect(t, "typescript", "UserCard.tsx", src)
	requireNotContains(t, got, []string{
		"http:GRAPHQL:/graphql/Query/UserCard_user",
		"http:GRAPHQL:/graphql/Query/id",
		"http:GRAPHQL:/graphql/Query/name",
	}, "graphql-client-relay-fragment-no-op")
}
