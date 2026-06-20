package engine

import "testing"

// TestPyGraphQLClient_QueryConstRef covers the canonical gql-package idiom:
// a gql doc bound to a module-level const (triple-quoted), referenced by a
// client.execute() call inside a function. Asserts the SPECIFIC operation
// root field (`user`) mapped to the server endpoint shape
// http:GRAPHQL:/graphql/Query/user, plus the FETCHES edge attributed to the
// referencing function.
func TestPyGraphQLClient_QueryConstRef(t *testing.T) {
	src := `
from gql import gql, Client

GET_USER = gql("""
  query GetUser {
    user { id name }
  }
""")

def load_user(client):
    return client.execute(GET_USER)
`
	ids, rels := runDetectWithRels(t, "python", "app/users.py", src)
	requireContains(t, ids, []string{
		"http:GRAPHQL:/graphql/Query/user",
	}, "py-gql-query-const")
	requireFetches(t, rels, "http:GRAPHQL:/graphql/Query/user", "py-gql-query-const")
}

// TestPyGraphQLClient_InlineMutation covers an inline gql mutation passed
// straight into session.execute(gql(”'...”')), asserting the Mutation root
// field `createUser` → http:GRAPHQL:/graphql/Mutation/createUser, with the
// FETCHES edge attributed to the enclosing async function.
func TestPyGraphQLClient_InlineMutation(t *testing.T) {
	src := `
from gql import gql

async def submit(session):
    await session.execute(gql('''
        mutation AddUser {
            createUser(input: {name: "x"}) { id }
        }
    '''))
`
	ids, rels := runDetectWithRels(t, "python", "app/add_user.py", src)
	requireContains(t, ids, []string{
		"http:GRAPHQL:/graphql/Mutation/createUser",
	}, "py-gql-inline-mutation")
	requireFetches(t, rels, "http:GRAPHQL:/graphql/Mutation/createUser", "py-gql-inline-mutation")
}

// TestPyGraphQLClient_MultiField covers a single-quoted inline gql doc with
// MULTIPLE root fields, asserting BOTH server endpoints are emitted.
func TestPyGraphQLClient_MultiField(t *testing.T) {
	src := `
from gql import gql

def dashboard(client):
    return client.execute(gql('query Dashboard { stats { count } alerts { id } }'))
`
	ids, _ := runDetectWithRels(t, "python", "app/dashboard.py", src)
	requireContains(t, ids, []string{
		"http:GRAPHQL:/graphql/Query/stats",
		"http:GRAPHQL:/graphql/Query/alerts",
	}, "py-gql-multi-field")
}

// TestPyGraphQLClient_Subscription covers a subscription gql doc, asserting
// the Subscription root-type mapping.
func TestPyGraphQLClient_Subscription(t *testing.T) {
	src := `
from gql import gql

SUB = gql("""
  subscription OnMessage {
    messageAdded { id body }
  }
""")

def watch(client):
    return client.subscribe(SUB)
`
	ids, _ := runDetectWithRels(t, "python", "app/watch.py", src)
	requireContains(t, ids, []string{
		"http:GRAPHQL:/graphql/Subscription/messageAdded",
	}, "py-gql-subscription")
}

// TestPyGraphQLClient_AnonShorthand covers the anonymous shorthand
// `{ users { id } }` (no operation keyword) → Query root type.
func TestPyGraphQLClient_AnonShorthand(t *testing.T) {
	src := `
from gql import gql

def list_users(client):
    return client.execute(gql("{ users { id } }"))
`
	ids, _ := runDetectWithRels(t, "python", "app/list.py", src)
	requireContains(t, ids, []string{
		"http:GRAPHQL:/graphql/Query/users",
	}, "py-gql-anon-shorthand")
}

// TestPyGraphQLClient_NegativeNoGql asserts a plain Python file with no gql(...)
// parser call emits NO GraphQL operation endpoint (the pass is a no-op).
func TestPyGraphQLClient_NegativeNoGql(t *testing.T) {
	src := `
import requests

def fetch():
    return requests.get("https://api.example.com/users")
`
	ids, _ := runDetectWithRels(t, "python", "app/rest.py", src)
	for _, id := range ids {
		if len(id) >= 13 && id[:13] == "http:GRAPHQL:/" {
			t.Fatalf("py-gql-negative: unexpected GraphQL endpoint emitted: %s", id)
		}
	}
}
