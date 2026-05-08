package hierarchy

import (
	"context"
	"testing"

	"github.com/cajasmota/archigraph/internal/extractor"
	"github.com/cajasmota/archigraph/internal/types"
)

func runExtract(t *testing.T, lang, path, source string) []types.EntityRecord {
	t.Helper()
	e := &Extractor{}
	records, err := e.Extract(context.Background(), extractor.FileInput{
		Path:     path,
		Content:  []byte(source),
		Language: lang,
	})
	if err != nil {
		t.Fatalf("Extract returned error: %v", err)
	}
	return records
}

// TestPython_DeclaredClassHasSubtypeClass asserts the declared (child) class
// in `class Foo(Bar):` is emitted with Subtype="class" — preserves the
// behavior tightened by issue #46.
func TestPython_DeclaredClassHasSubtypeClass(t *testing.T) {
	src := "class UserSerializer(serializers.ModelSerializer):\n    pass\n"
	got := runExtract(t, "python", "app/serializers.py", src)

	var foundChild bool
	for _, e := range got {
		if e.Kind == "SCOPE.Component" && e.Name == "UserSerializer" {
			foundChild = true
			if e.Subtype != "class" {
				t.Errorf("UserSerializer Subtype=%q, want %q", e.Subtype, "class")
			}
		}
	}
	if !foundChild {
		t.Fatalf("expected UserSerializer entity, got entities=%v", entityNames(got))
	}
}

// TestPython_ExternalBaseDoesNotGetClassSubtype is the regression test for
// issue #74. External base references (e.g. `serializers.ModelSerializer`)
// must NOT be emitted as Subtype="class" entities — that conflated external
// references with declared classes. Per issue #74 we drop the placeholder
// entirely and let internal/external/synth.go (Pass 4.5) handle the
// unresolved EXTENDS endpoint.
func TestPython_ExternalBaseDoesNotGetClassSubtype(t *testing.T) {
	src := "class UserSerializer(serializers.ModelSerializer):\n    pass\n"
	got := runExtract(t, "python", "app/serializers.py", src)

	forbidden := map[string]bool{
		"ModelSerializer":             true,
		"serializers.ModelSerializer": true,
		"serializers":                 true,
	}
	for _, e := range got {
		if e.Kind != "SCOPE.Component" {
			continue
		}
		if e.Subtype != "class" {
			continue
		}
		if forbidden[e.Name] {
			t.Errorf("forbidden external-base entity emitted as Subtype=class: name=%q", e.Name)
		}
	}
}

// TestPython_ExtendsRelationshipStillEmitted ensures dropping the placeholder
// did NOT drop the EXTENDS relationship — the resolver and external
// synthesis pass still need it to wire up the graph edge.
func TestPython_ExtendsRelationshipStillEmitted(t *testing.T) {
	src := "class UserSerializer(serializers.ModelSerializer):\n    pass\n"
	got := runExtract(t, "python", "app/serializers.py", src)

	var hasExtends bool
	for _, e := range got {
		for _, rel := range e.Relationships {
			if rel.Kind == "EXTENDS" {
				hasExtends = true
			}
		}
	}
	if !hasExtends {
		t.Errorf("expected an EXTENDS relationship, got entities=%v", entityNames(got))
	}
}

// TestPython_DeclaredParentStillResolvable confirms that a base class
// declared in the same file (Child extends Base) still produces a valid
// EXTENDS edge even though we no longer synthesise a placeholder for the
// parent. The actual `Base` declaration comes from the python extractor;
// the cross-hierarchy extractor only emits the relationship.
func TestPython_DeclaredParentStillResolvable(t *testing.T) {
	src := "class Base:\n    pass\n\nclass Child(Base):\n    pass\n"
	got := runExtract(t, "python", "app/models.py", src)

	// Child must be emitted (it has a non-empty base list).
	var hasChild bool
	for _, e := range got {
		if e.Kind == "SCOPE.Component" && e.Name == "Child" && e.Subtype == "class" {
			hasChild = true
		}
	}
	if !hasChild {
		t.Errorf("expected Child class entity, got entities=%v", entityNames(got))
	}

	// EXTENDS relationship from Child -> Base must still be emitted.
	var hasExtends bool
	for _, e := range got {
		for _, rel := range e.Relationships {
			if rel.Kind == "EXTENDS" {
				hasExtends = true
			}
		}
	}
	if !hasExtends {
		t.Errorf("expected EXTENDS relationship Child->Base, got entities=%v", entityNames(got))
	}
}

func entityNames(records []types.EntityRecord) []string {
	out := make([]string, 0, len(records))
	for _, r := range records {
		out = append(out, r.Name+"["+r.Subtype+"]")
	}
	return out
}
