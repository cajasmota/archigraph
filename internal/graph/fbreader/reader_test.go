package fbreader_test

import (
	"path/filepath"
	"testing"
	"time"

	"github.com/cajasmota/archigraph/internal/graph"
	"github.com/cajasmota/archigraph/internal/graph/fbreader"
	"github.com/cajasmota/archigraph/internal/graph/fbwriter"
)

func writeAndOpen(t *testing.T, doc *graph.Document) *fbreader.Reader {
	t.Helper()
	path := filepath.Join(t.TempDir(), "g.fb")
	if err := fbwriter.WriteAtomic(path, doc); err != nil {
		t.Fatalf("write: %v", err)
	}
	r, err := fbreader.Open(path)
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	t.Cleanup(func() { _ = r.Close() })
	return r
}

func TestPhaseDReaderMethods(t *testing.T) {
	doc := &graph.Document{
		Repo:        "demo",
		GeneratedAt: time.Date(2026, 5, 19, 12, 0, 0, 0, time.UTC),
		Entities: []graph.Entity{
			{ID: "a", QualifiedName: "pkg.A", Kind: "function", Name: "A"},
			{ID: "b", QualifiedName: "pkg.B", Kind: "struct", Name: "B"},
			{ID: "c", QualifiedName: "pkg.C", Kind: "function", Name: "C"},
		},
		Relationships: []graph.Relationship{
			{FromID: "a", ToID: "b", Kind: "CALLS"},
			{FromID: "c", ToID: "b", Kind: "CALLS"},
			{FromID: "a", ToID: "c", Kind: "REFERENCES"},
		},
	}
	r := writeAndOpen(t, doc)

	if r.LookupEntityByID("b") == nil {
		t.Errorf("expected entity b")
	}

	fromA := r.IterateRelationshipsFromID("a")
	if len(fromA) != 2 {
		t.Errorf("from a: got %d, want 2", len(fromA))
	}

	toB := r.IterateRelationshipsToID("b")
	if len(toB) != 2 {
		t.Errorf("to b: got %d, want 2", len(toB))
	}

	funcs := r.FilterEntitiesByKind("function")
	if len(funcs) != 2 {
		t.Errorf("functions: got %d, want 2", len(funcs))
	}

	meta := r.LoadGraphMeta()
	if meta.Version == 0 {
		t.Errorf("expected non-zero version")
	}
	if meta.RepoTag != "demo" {
		t.Errorf("repo tag = %q, want demo", meta.RepoTag)
	}
	if meta.ComputedAt == "" {
		t.Errorf("expected non-empty computed_at")
	}
}
