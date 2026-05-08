package resolve

import (
	"testing"

	"github.com/cajasmota/archigraph/internal/types"
)

func ent(id, kind, name string) types.EntityRecord {
	return types.EntityRecord{ID: id, Kind: kind, Name: name, SourceFile: "x.go"}
}

func entAt(id, kind, name, file string) types.EntityRecord {
	return types.EntityRecord{ID: id, Kind: kind, Name: name, SourceFile: file}
}

func TestReferences_Unambiguous(t *testing.T) {
	entities := []types.EntityRecord{ent("aaaaaaaaaaaaaaaa", "Function", "Hello")}
	rels := []types.RelationshipRecord{{FromID: "0000000000000000", ToID: "Function:Hello", Kind: "CALLS"}}
	idx := BuildIndex(entities)
	stats := References(rels, idx)
	if rels[0].ToID != "aaaaaaaaaaaaaaaa" {
		t.Fatalf("unambiguous: ToID not rewritten: %s", rels[0].ToID)
	}
	if stats.Rewritten != 1 {
		t.Fatalf("expected 1 rewrite, got %d", stats.Rewritten)
	}
}

func TestReferences_Ambiguous(t *testing.T) {
	entities := []types.EntityRecord{
		ent("aaaaaaaaaaaaaaaa", "Function", "Foo"),
		ent("bbbbbbbbbbbbbbbb", "Function", "Foo"),
	}
	rels := []types.RelationshipRecord{{FromID: "0000000000000000", ToID: "Function:Foo", Kind: "CALLS"}}
	idx := BuildIndex(entities)
	stats := References(rels, idx)
	if rels[0].ToID != "Function:Foo" {
		t.Fatalf("ambiguous: ToID was rewritten to %s, expected stub preserved", rels[0].ToID)
	}
	if stats.Ambiguous != 1 {
		t.Fatalf("expected 1 ambiguous, got %d", stats.Ambiguous)
	}
}

func TestReferences_Unmatched(t *testing.T) {
	entities := []types.EntityRecord{ent("aaaaaaaaaaaaaaaa", "Function", "Hello")}
	rels := []types.RelationshipRecord{{FromID: "0000000000000000", ToID: "Function:Missing", Kind: "CALLS"}}
	idx := BuildIndex(entities)
	stats := References(rels, idx)
	if rels[0].ToID != "Function:Missing" {
		t.Fatalf("unmatched: ToID was rewritten: %s", rels[0].ToID)
	}
	if stats.Unmatched != 1 {
		t.Fatalf("expected 1 unmatched, got %d", stats.Unmatched)
	}
}

func TestReferences_KindAware(t *testing.T) {
	entities := []types.EntityRecord{
		ent("aaaaaaaaaaaaaaaa", "Function", "User"),
		ent("bbbbbbbbbbbbbbbb", "View", "User"),
	}
	rels := []types.RelationshipRecord{{FromID: "0000000000000000", ToID: "View:User", Kind: "USES"}}
	idx := BuildIndex(entities)
	stats := References(rels, idx)
	if rels[0].ToID != "bbbbbbbbbbbbbbbb" {
		t.Fatalf("kind-aware: ToID resolved to wrong entity: %s", rels[0].ToID)
	}
	if stats.Rewritten != 1 {
		t.Fatalf("expected 1 rewrite, got %d", stats.Rewritten)
	}
}

func TestReferences_StubMissingPrefix(t *testing.T) {
	entities := []types.EntityRecord{ent("aaaaaaaaaaaaaaaa", "Function", "Hello")}
	rels := []types.RelationshipRecord{{FromID: "0000000000000000", ToID: "Hello", Kind: "CALLS"}}
	idx := BuildIndex(entities)
	stats := References(rels, idx)
	if rels[0].ToID != "aaaaaaaaaaaaaaaa" {
		t.Fatalf("bare-name fallback: ToID not rewritten: %s", rels[0].ToID)
	}
	if stats.Rewritten != 1 {
		t.Fatalf("expected 1 rewrite, got %d", stats.Rewritten)
	}
}

func TestReferences_ScopePrefixedKind(t *testing.T) {
	// Pass 3 cross-language extractors emit kinds like "SCOPE.View". A stub
	// "View:User" must still resolve to that entity.
	entities := []types.EntityRecord{ent("cccccccccccccccc", "SCOPE.View", "Dashboard")}
	rels := []types.RelationshipRecord{{FromID: "0000000000000000", ToID: "View:Dashboard", Kind: "USES"}}
	idx := BuildIndex(entities)
	stats := References(rels, idx)
	if rels[0].ToID != "cccccccccccccccc" {
		t.Fatalf("scope-prefixed kind: ToID=%s", rels[0].ToID)
	}
	if stats.Rewritten != 1 {
		t.Fatalf("expected 1 rewrite, got %d", stats.Rewritten)
	}
}

func TestReferences_SkipsHexIDs(t *testing.T) {
	// Already-resolved IDs (16-char lower hex) must be left untouched.
	entities := []types.EntityRecord{ent("aaaaaaaaaaaaaaaa", "Function", "Hello")}
	rels := []types.RelationshipRecord{{FromID: "0000000000000000", ToID: "ffffffffffffffff", Kind: "CALLS"}}
	idx := BuildIndex(entities)
	stats := References(rels, idx)
	if rels[0].ToID != "ffffffffffffffff" {
		t.Fatalf("hex-ID was modified: %s", rels[0].ToID)
	}
	if stats.Rewritten != 0 || stats.Ambiguous != 0 || stats.Unmatched != 0 {
		t.Fatalf("hex-ID counted in stats: %+v", stats)
	}
}

func TestReferencesEmbedded(t *testing.T) {
	records := []types.EntityRecord{
		{ID: "aaaaaaaaaaaaaaaa", Kind: "Function", Name: "Hello", SourceFile: "a.go"},
		{
			ID:         "bbbbbbbbbbbbbbbb",
			Kind:       "Function",
			Name:       "Greet",
			SourceFile: "b.go",
			Relationships: []types.RelationshipRecord{
				{ToID: "Hello", Kind: "CALLS"},
			},
		},
	}
	idx := BuildIndex(records)
	stats := ReferencesEmbedded(records, idx)
	if records[1].Relationships[0].ToID != "aaaaaaaaaaaaaaaa" {
		t.Fatalf("embedded: ToID not rewritten: %s", records[1].Relationships[0].ToID)
	}
	if stats.Rewritten != 1 {
		t.Fatalf("expected 1 rewrite, got %d", stats.Rewritten)
	}
}

// PORT-2-FIX-3 (issue #31) — structural reference resolution + kind hint.

func TestReferences_StructuralFormatA(t *testing.T) {
	// scope:component:class:python:core/views/orders.py:OrderViewSet
	// resolves to the entity at that file+name.
	entities := []types.EntityRecord{
		entAt("aaaaaaaaaaaaaaaa", "SCOPE.Component", "OrderViewSet", "core/views/orders.py"),
		entAt("dddddddddddddddd", "SCOPE.Component", "Other", "core/views/other.py"),
	}
	rels := []types.RelationshipRecord{{
		FromID: "0000000000000000",
		ToID:   "scope:component:class:python:core/views/orders.py:OrderViewSet",
		Kind:   "USES",
	}}
	idx := BuildIndex(entities)
	stats := References(rels, idx)
	if rels[0].ToID != "aaaaaaaaaaaaaaaa" {
		t.Fatalf("format A: ToID=%s", rels[0].ToID)
	}
	if stats.Rewritten != 1 {
		t.Fatalf("expected 1 rewrite, got %+v", stats)
	}
}

func TestReferences_StructuralFormatB(t *testing.T) {
	// scope:operation:method:python:core/views/orders.py:OrderViewSet#create
	// resolves to the method entity recorded as "OrderViewSet.create".
	entities := []types.EntityRecord{
		entAt("aaaaaaaaaaaaaaaa", "SCOPE.Component", "OrderViewSet", "core/views/orders.py"),
		entAt("bbbbbbbbbbbbbbbb", "SCOPE.Operation", "OrderViewSet.create", "core/views/orders.py"),
	}
	rels := []types.RelationshipRecord{{
		FromID: "0000000000000000",
		ToID:   "scope:operation:method:python:core/views/orders.py:OrderViewSet#create",
		Kind:   "CALLS",
	}}
	idx := BuildIndex(entities)
	stats := References(rels, idx)
	if rels[0].ToID != "bbbbbbbbbbbbbbbb" {
		t.Fatalf("format B: ToID=%s", rels[0].ToID)
	}
	if stats.Rewritten != 1 {
		t.Fatalf("expected 1 rewrite, got %+v", stats)
	}
}

func TestReferences_StructuralLocationAmbiguous(t *testing.T) {
	// Same (file, name) recorded twice → ambiguous, stub preserved.
	entities := []types.EntityRecord{
		entAt("aaaaaaaaaaaaaaaa", "SCOPE.Component", "OrderViewSet", "core/views/orders.py"),
		entAt("bbbbbbbbbbbbbbbb", "SCOPE.View", "OrderViewSet", "core/views/orders.py"),
	}
	stub := "scope:component:class:python:core/views/orders.py:OrderViewSet"
	rels := []types.RelationshipRecord{{FromID: "0000000000000000", ToID: stub, Kind: "USES"}}
	idx := BuildIndex(entities)
	stats := References(rels, idx)
	if rels[0].ToID != stub {
		t.Fatalf("location-ambig: ToID was rewritten to %s", rels[0].ToID)
	}
	if stats.Ambiguous != 1 {
		t.Fatalf("expected 1 ambiguous, got %+v", stats)
	}
}

func TestReferences_KindHintCallsPrefersOperation(t *testing.T) {
	entities := []types.EntityRecord{
		entAt("aaaaaaaaaaaaaaaa", "Operation", "Foo", "a.py"),
		entAt("bbbbbbbbbbbbbbbb", "Component", "Foo", "b.py"),
	}
	rels := []types.RelationshipRecord{{FromID: "0000000000000000", ToID: "Foo", Kind: "CALLS"}}
	idx := BuildIndex(entities)
	stats := References(rels, idx)
	if rels[0].ToID != "aaaaaaaaaaaaaaaa" {
		t.Fatalf("kind-hint CALLS: ToID=%s", rels[0].ToID)
	}
	if stats.Rewritten != 1 {
		t.Fatalf("expected 1 rewrite, got %+v", stats)
	}
}

func TestReferences_KindHintExtendsPrefersComponent(t *testing.T) {
	entities := []types.EntityRecord{
		entAt("aaaaaaaaaaaaaaaa", "Function", "Foo", "a.py"),
		entAt("bbbbbbbbbbbbbbbb", "Component", "Foo", "b.py"),
	}
	rels := []types.RelationshipRecord{{FromID: "0000000000000000", ToID: "Foo", Kind: "EXTENDS"}}
	idx := BuildIndex(entities)
	stats := References(rels, idx)
	if rels[0].ToID != "bbbbbbbbbbbbbbbb" {
		t.Fatalf("kind-hint EXTENDS: ToID=%s", rels[0].ToID)
	}
	if stats.Rewritten != 1 {
		t.Fatalf("expected 1 rewrite, got %+v", stats)
	}
}

func TestReferences_KindHintMissingStillAmbiguous(t *testing.T) {
	// Same setup as the disambiguation test, but the relationship's Kind
	// doesn't bias toward any family → still ambiguous, stub preserved.
	entities := []types.EntityRecord{
		entAt("aaaaaaaaaaaaaaaa", "Operation", "Foo", "a.py"),
		entAt("bbbbbbbbbbbbbbbb", "Component", "Foo", "b.py"),
	}
	rels := []types.RelationshipRecord{{FromID: "0000000000000000", ToID: "Foo", Kind: "USES"}}
	idx := BuildIndex(entities)
	stats := References(rels, idx)
	if rels[0].ToID != "Foo" {
		t.Fatalf("hint-missing: ToID was rewritten to %s", rels[0].ToID)
	}
	if stats.Ambiguous != 1 {
		t.Fatalf("expected 1 ambiguous, got %+v", stats)
	}
}

func TestBuildLocationIndex(t *testing.T) {
	entities := []types.EntityRecord{
		entAt("aaaaaaaaaaaaaaaa", "Component", "Foo", "a.py"),
		entAt("bbbbbbbbbbbbbbbb", "Component", "Bar", "a.py"),
		// Duplicate (file, name) → dropped.
		entAt("cccccccccccccccc", "Component", "Foo", "a.py"),
		entAt("dddddddddddddddd", "Component", "Foo", "b.py"),
	}
	loc := BuildLocationIndex(entities)
	if loc["a.py"]["Bar"] != "bbbbbbbbbbbbbbbb" {
		t.Fatalf("a.py/Bar: %s", loc["a.py"]["Bar"])
	}
	if _, dup := loc["a.py"]["Foo"]; dup {
		t.Fatalf("a.py/Foo should be dropped as ambiguous; got %s", loc["a.py"]["Foo"])
	}
	if loc["b.py"]["Foo"] != "dddddddddddddddd" {
		t.Fatalf("b.py/Foo: %s", loc["b.py"]["Foo"])
	}
}
