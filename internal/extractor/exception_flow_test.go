package extractor

import (
	"testing"

	"github.com/cajasmota/grafel/internal/types"
)

// fileEntity builds a minimal file entity (entities[0] contract) plus the
// named function/method hosts EmitExceptionEdges attaches edges to.
func newRecords(file string, lang string, hosts ...string) []types.EntityRecord {
	recs := []types.EntityRecord{{
		Name: file, QualifiedName: file, Kind: string(types.EntityKindComponent),
		Subtype: "file", Language: lang, SourceFile: file, StartLine: 1, EndLine: 99,
	}}
	for _, h := range hosts {
		recs = append(recs, types.EntityRecord{
			Name: h, QualifiedName: h, Kind: string(types.EntityKindFunction),
			Language: lang, SourceFile: file, StartLine: 1, EndLine: 9,
		})
	}
	return recs
}

func edgeTo(recs []types.EntityRecord, fromName, kind, toID string) bool {
	for i := range recs {
		if recs[i].Name != fromName {
			continue
		}
		for _, r := range recs[i].Relationships {
			if r.Kind == kind && r.ToID == toID {
				return true
			}
		}
	}
	return false
}

func excEntityID(recs []types.EntityRecord, typeName string) string {
	want := ExceptionTypeName(typeName)
	for i := range recs {
		if recs[i].Kind == string(types.EntityKindExceptionType) && recs[i].Name == want {
			return recs[i].ID
		}
	}
	return ""
}

// TestExceptionFlow_ThrowCatchConverge is the value-asserting test: a type
// raised in one function and caught in another must produce a THROWS edge and
// a CATCHES edge that BOTH point at the SAME exception-type node ID. The
// throw/catch mapping converging on one node IS the capability's value.
func TestExceptionFlow_ThrowCatchConverge(t *testing.T) {
	recs := newRecords("svc.py", "python", "get_user", "handler")
	n := EmitExceptionEdges(&recs, "python", []ExceptionEdge{
		{Type: "NotFound", FromName: "get_user", Pattern: "raise"},
		{Type: "NotFound", FromName: "handler", Catch: true, Pattern: "except"},
	})
	if n != 2 {
		t.Fatalf("expected 2 edges, got %d", n)
	}

	target := ExceptionTypeTargetID("NotFound")
	if want := "scope:exceptiontype:NotFound"; target != want {
		t.Fatalf("target id = %q, want %q", target, want)
	}
	if !edgeTo(recs, "get_user", "THROWS", target) {
		t.Errorf("missing THROWS(get_user -> %s)", target)
	}
	if !edgeTo(recs, "handler", "CATCHES", target) {
		t.Errorf("missing CATCHES(handler -> %s)", target)
	}

	// Convergence: exactly ONE exception-type entity, and its QualifiedName is
	// the edge ToID (so byQualifiedName resolves both edges to this node).
	count := 0
	for i := range recs {
		if recs[i].Kind == string(types.EntityKindExceptionType) {
			count++
			if recs[i].QualifiedName != target {
				t.Errorf("exception entity QualifiedName = %q, want %q (ToID)", recs[i].QualifiedName, target)
			}
		}
	}
	if count != 1 {
		t.Fatalf("throw + catch of NotFound must converge on ONE entity, got %d", count)
	}
	if excEntityID(recs, "NotFound") == "" {
		t.Fatalf("no NotFound exception-type entity")
	}
}

// TestExceptionFlow_CrossFileConvergence proves the synthetic SourceFile makes
// the SAME type name in two different files resolve to ONE node ID.
func TestExceptionFlow_CrossFileConvergence(t *testing.T) {
	a := ExceptionTypeEntity("ValidationError", "typescript")
	b := ExceptionTypeEntity("ValidationError", "python")
	if a.ID != b.ID {
		t.Fatalf("same type name must converge: %q (ts) != %q (py)", a.ID, b.ID)
	}
	if a.SourceFile != ExceptionTypeSourceFile {
		t.Errorf("SourceFile = %q, want synthetic %q", a.SourceFile, ExceptionTypeSourceFile)
	}
}

// TestExceptionFlow_TypedDedup: a function raising the same type twice yields
// one THROWS edge; throwing and catching the same type yields one of each.
func TestExceptionFlow_TypedDedup(t *testing.T) {
	recs := newRecords("a.ts", "typescript", "f")
	n := EmitExceptionEdges(&recs, "typescript", []ExceptionEdge{
		{Type: "AuthError", FromName: "f", Pattern: "throw_new"},
		{Type: "AuthError", FromName: "f", Pattern: "throw_new"},
		{Type: "AuthError", FromName: "f", Catch: true, Pattern: "instanceof"},
	})
	if n != 2 {
		t.Fatalf("expected 2 deduped edges (1 throws + 1 catches), got %d", n)
	}
}

// TestExceptionFlow_NormalizeQualified strips package/module qualification.
func TestExceptionFlow_NormalizeQualified(t *testing.T) {
	cases := map[string]string{
		"errors.ValidationError": "ValidationError",
		"java.io.IOException":    "IOException",
		"pkg::ErrNotFound":       "ErrNotFound",
		"NotFound":               "NotFound",
		"e.AuthError":            "AuthError",
	}
	for in, want := range cases {
		if got := NormalizeExceptionType(in); got != want {
			t.Errorf("NormalizeExceptionType(%q) = %q, want %q", in, got, want)
		}
	}
}

// TestExceptionFlow_DynamicDropped: NEGATIVE — dynamic / computed / non-type
// tokens must emit NO edge and NO node (precision over recall).
func TestExceptionFlow_DynamicDropped(t *testing.T) {
	recs := newRecords("a.py", "python", "f")
	n := EmitExceptionEdges(&recs, "python", []ExceptionEdge{
		{Type: "exc_class()", FromName: "f", Pattern: "raise"}, // dynamic
		{Type: "mk(err)", FromName: "f", Pattern: "throw"},     // computed
		{Type: "", FromName: "f", Pattern: "raise"},            // empty
		{Type: "List<Foo>", FromName: "f", Pattern: "throw"},   // generic punctuation
	})
	if n != 0 {
		t.Fatalf("dynamic/computed types must emit no edge, got %d", n)
	}
	for i := range recs {
		if recs[i].Kind == string(types.EntityKindExceptionType) {
			t.Errorf("dynamic type must not create an exception node: %q", recs[i].Name)
		}
	}
}

// TestExceptionFlow_UnknownHostFallsBackToFile: an edge whose FromName has no
// host entity attaches to the file entity (index 0), never dropped.
func TestExceptionFlow_UnknownHostFallsBackToFile(t *testing.T) {
	recs := newRecords("a.go", "go") // no function hosts
	n := EmitExceptionEdges(&recs, "go", []ExceptionEdge{
		{Type: "ErrNotFound", FromName: "ghost", Pattern: "return_named"},
	})
	if n != 1 {
		t.Fatalf("expected 1 edge attached to file fallback, got %d", n)
	}
	if !edgeTo(recs, "a.go", "THROWS", ExceptionTypeTargetID("ErrNotFound")) {
		t.Errorf("edge with unknown host should attach to file entity")
	}
}
