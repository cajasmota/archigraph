package cpp_test

import (
	"testing"

	"github.com/cajasmota/grafel/internal/extractor"
	"github.com/cajasmota/grafel/internal/types"
)

// cppExcEdge asserts a THROWS / CATCHES edge from fromName to the shared
// exception-type node for typeName (matching the flagship edge shape: ToID ==
// ExceptionTypeTargetID, Kind == THROWS/CATCHES).
func cppExcEdge(recs []types.EntityRecord, fromName, kind, typeName string) bool {
	want := extractor.ExceptionTypeTargetID(typeName)
	for i := range recs {
		if recs[i].Name != fromName {
			continue
		}
		for _, r := range recs[i].Relationships {
			if r.Kind == kind && r.ToID == want {
				return true
			}
		}
	}
	return false
}

// cppExcNodeCount counts SCOPE.ExceptionType nodes named exception:<typeName>.
func cppExcNodeCount(recs []types.EntityRecord, typeName string) int {
	want := extractor.ExceptionTypeName(typeName)
	c := 0
	for i := range recs {
		if recs[i].Kind == string(types.EntityKindExceptionType) && recs[i].Name == want {
			c++
		}
	}
	return c
}

// cppExcNode returns the shared exception-type entity for typeName, or nil.
func cppExcNode(recs []types.EntityRecord, typeName string) *types.EntityRecord {
	want := extractor.ExceptionTypeName(typeName)
	for i := range recs {
		if recs[i].Kind == string(types.EntityKindExceptionType) && recs[i].Name == want {
			return &recs[i]
		}
	}
	return nil
}

// TestCppExceptionFlowConvergence is the flagship value-assertion: a type thrown
// in one method and caught in another converges on ONE exception-type node, and
// both edges target that node's ExceptionTypeTargetID.
func TestCppExceptionFlowConvergence(t *testing.T) {
	src := `
class NotFoundException {};
class UserService {
  User find(int id) {
    throw NotFoundException("missing");
  }
  void handle() {
    try {
      find(1);
    } catch (const NotFoundException& e) {
    }
  }
};
`
	recs, err := extractCPP(src, "svc.cpp")
	if err != nil {
		t.Fatalf("extract: %v", err)
	}

	if !cppExcEdge(recs, "find", "THROWS", "NotFoundException") {
		t.Errorf("expected find --THROWS--> NotFoundException")
	}
	if !cppExcEdge(recs, "handle", "CATCHES", "NotFoundException") {
		t.Errorf("expected handle --CATCHES--> NotFoundException")
	}
	// Convergence: exactly ONE shared node for the type.
	if n := cppExcNodeCount(recs, "NotFoundException"); n != 1 {
		t.Errorf("expected exactly 1 NotFoundException exception node, got %d", n)
	}
	// The node's structural ToID is the THROWS/CATCHES edge target.
	node := cppExcNode(recs, "NotFoundException")
	if node == nil {
		t.Fatal("missing NotFoundException exception node")
	}
	if node.QualifiedName != extractor.ExceptionTypeTargetID("NotFoundException") {
		t.Errorf("node QualifiedName=%q, want %q",
			node.QualifiedName, extractor.ExceptionTypeTargetID("NotFoundException"))
	}
}

// TestCppThrowVariants asserts each throw-by-value / brace-init / qualified /
// std:: / new shape yields the correct bare exception type.
func TestCppThrowVariants(t *testing.T) {
	src := `
namespace MyNs { struct Boom {}; }
void a() { throw NotFoundException("x"); }
void b() { throw MyNs::Boom{}; }
void c() { throw std::runtime_error("oops"); }
void d() { throw BoomLit{}; }
void e() { throw new HeapErr(); }
`
	recs, err := extractCPP(src, "t.cpp")
	if err != nil {
		t.Fatalf("extract: %v", err)
	}
	cases := []struct{ fn, typ string }{
		{"a", "NotFoundException"},
		{"b", "Boom"},          // qualified MyNs::Boom -> bare last segment
		{"c", "runtime_error"}, // std::runtime_error -> bare last segment
		{"d", "BoomLit"},       // compound literal
		{"e", "HeapErr"},       // new_expression
	}
	for _, tc := range cases {
		if !cppExcEdge(recs, tc.fn, "THROWS", tc.typ) {
			t.Errorf("fn %s: expected THROWS %s", tc.fn, tc.typ)
		}
	}
}

// TestCppCatchVariants asserts const-ref / pointer / by-value / qualified / std::
// typed catches all yield the bare caught type.
func TestCppCatchVariants(t *testing.T) {
	src := `
void h() {
  try { work(); }
  catch (const NotFoundException& e) {}
  catch (std::runtime_error& e) {}
  catch (MyError* p) {}
  catch (ValueErr v) {}
  catch (MyNs::Domain d) {}
}
`
	recs, err := extractCPP(src, "h.cpp")
	if err != nil {
		t.Fatalf("extract: %v", err)
	}
	for _, typ := range []string{"NotFoundException", "runtime_error", "MyError", "ValueErr", "Domain"} {
		if !cppExcEdge(recs, "h", "CATCHES", typ) {
			t.Errorf("expected h CATCHES %s", typ)
		}
	}
}

// TestCppExceptionFlowNegatives asserts catch-all, re-throws, and a plain
// function emit NO typed edges (precision-first, honest-partial).
func TestCppExceptionFlowNegatives(t *testing.T) {
	src := `
void catchAll() {
  try { work(); } catch (...) {}
}
void rethrowBare() {
  try { work(); } catch (const Foo& e) { throw; }
}
void rethrowVar() {
  try { work(); } catch (Foo e) { throw e; }
}
int plain(int x) { return x + 1; }
`
	recs, err := extractCPP(src, "neg.cpp")
	if err != nil {
		t.Fatalf("extract: %v", err)
	}

	// catch (...) is untyped → no CATCHES edge and no fabricated node.
	for i := range recs {
		if recs[i].Name == "catchAll" {
			for _, r := range recs[i].Relationships {
				if r.Kind == "CATCHES" || r.Kind == "THROWS" {
					t.Errorf("catchAll should emit no typed edges, got %s -> %s", r.Kind, r.ToID)
				}
			}
		}
	}

	// `throw;` (bare) and `throw e;` (variable) introduce no new type, so the
	// only THROWS edge anywhere must be absent — the caught Foo is the sole
	// exception node here, from the CATCHES sides.
	for i := range recs {
		for _, r := range recs[i].Relationships {
			if r.Kind == "THROWS" {
				t.Errorf("no THROWS edge expected (only re-throws present), got from %s -> %s",
					recs[i].Name, r.ToID)
			}
		}
	}

	// Plain function with no throw/catch produces no exception edges.
	for i := range recs {
		if recs[i].Name == "plain" {
			for _, r := range recs[i].Relationships {
				if r.Kind == "CATCHES" || r.Kind == "THROWS" {
					t.Errorf("plain function should have no exception edges, got %s", r.Kind)
				}
			}
		}
	}

	// The catch-all-only function fabricates no exception node from `...`.
	if cppExcNodeCount(recs, "...") != 0 {
		t.Error("catch-all must not create an exception node")
	}
}

// TestCExceptionFlowNoEdges asserts plain C (no exceptions) yields no
// exception-flow artifacts — C has no try/catch/throw.
func TestCExceptionFlowNoEdges(t *testing.T) {
	src := `int add(int a, int b){ return a + b; }`
	recs, err := extractC(src, "add.c")
	if err != nil {
		t.Fatalf("extract: %v", err)
	}
	for i := range recs {
		if recs[i].Kind == string(types.EntityKindExceptionType) {
			t.Errorf("C source must produce no exception-type nodes, got %s", recs[i].Name)
		}
	}
}
