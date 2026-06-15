// issue4681_localvar_receiver_test.go — Python local-variable receiver typing
// for test→CALLS coverage crediting (issue #4681, part of epic #4615/#4672).
//
// Mirrors the TS/JS local-receiver typing landed in #4671. A unit test binds
// a local from a constructor and then calls a method on it:
//
//	def test_get_counts():
//	    v = ProposalViewSet()
//	    v.get_counts('2025')
//
// Before #4681 the receiver `v` was a lowercase identifier the PascalCase
// heuristic never matched, so `v.get_counts()` emitted a bare, unresolvable
// leaf — no test→handler CALLS edge — and ComputeCoverage saw the endpoint as
// untested. After #4681 the per-body local map types `v → ProposalViewSet`,
// so the edge resolves to `ProposalViewSet.get_counts`.
//
// Route-hit test-client linkage (DRF `self.client.get(url)`, fixture B in the
// epic) is already covered by the e2e_route_calls path —
// see internal/engine/http_endpoint_e2e_testmap_4369_test.go.

package python

import (
	"testing"

	"github.com/cajasmota/grafel/internal/types"
)

// hasCallTo reports whether any CALLS relationship in rels targets toID.
func hasCallTo(rels []types.RelationshipRecord, toID string) bool {
	for _, r := range rels {
		if r.Kind == "CALLS" && r.ToID == toID {
			return true
		}
	}
	return false
}

// FIXTURE A — local var assigned from a ViewSet constructor inside a named
// test function; the subsequent method call must resolve to the class method.
func TestIssue4681_LocalVarReceiver_ViewSetUnitSpec(t *testing.T) {
	src := `
class ProposalViewSet:
    def get_counts(self, year):
        return year


def test_get_counts():
    v = ProposalViewSet()
    v.get_counts('2025')
`
	ents := extractEntities(t, "tests/test_proposals.py", src)
	calls := findCallsFrom(ents, "test_get_counts")
	if !hasCallTo(calls, "ProposalViewSet.get_counts") {
		t.Fatalf("expected CALLS edge to ProposalViewSet.get_counts (local-var receiver typing #4681); got %+v", calls)
	}
}

// Variant: Django CBV and DRF serializer local-var receivers also type.
func TestIssue4681_LocalVarReceiver_CBVAndSerializer(t *testing.T) {
	src := `
class ArticleView:
    def get(self, request):
        return request


class ArticleSerializer:
    def is_valid(self):
        return True


def test_view_and_serializer():
    view = ArticleView()
    view.get(None)
    obj = ArticleSerializer()
    obj.is_valid()
`
	ents := extractEntities(t, "tests/test_articles.py", src)
	calls := findCallsFrom(ents, "test_view_and_serializer")
	if !hasCallTo(calls, "ArticleView.get") {
		t.Fatalf("expected CALLS edge to ArticleView.get; got %+v", calls)
	}
	if !hasCallTo(calls, "ArticleSerializer.is_valid") {
		t.Fatalf("expected CALLS edge to ArticleSerializer.is_valid; got %+v", calls)
	}
}

// Negative control 1: a factory-function (non-PascalCase) RHS must NOT bind —
// `make_thing()` is not a class construction, so `t.run()` stays ambiguous and
// no `<Class>.run` edge is forged.
func TestIssue4681_LocalVarReceiver_NegativeFactory(t *testing.T) {
	src := `
def test_factory():
    obj = make_thing()
    obj.run()
`
	ents := extractEntities(t, "tests/test_factory.py", src)
	calls := findCallsFrom(ents, "test_factory")
	for _, r := range calls {
		if r.Kind == "CALLS" && r.ToID == "make_thing.run" {
			t.Fatalf("factory-function receiver must not type to make_thing.run; got %+v", calls)
		}
	}
}

// FIXTURE C — shape-only assertion test: never constructs a handler class and
// never calls a production method. No `<Class>.method` CALLS edge must form, so
// ComputeCoverage leaves the endpoint uncovered (honest exclusion, #4662/#4671).
func TestIssue4681_ShapeOnlyAssertion_NoEdge(t *testing.T) {
	src := `
def test_shape_only():
    payload = {'count': 0}
    assert 'count' in payload
    assert isinstance(payload['count'], int)
`
	ents := extractEntities(t, "tests/test_shape.py", src)
	calls := findCallsFrom(ents, "test_shape_only")
	for _, r := range calls {
		if r.Kind == "CALLS" && (r.ToID == "ProposalViewSet.get_counts" || r.ToID == "get_counts") {
			t.Fatalf("shape-only assertion must not credit any handler; got %+v", calls)
		}
	}
}
