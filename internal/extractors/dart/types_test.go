package dart_test

import (
	"context"
	"testing"

	"github.com/cajasmota/archigraph/internal/extractor"
	_ "github.com/cajasmota/archigraph/internal/extractors/dart"
	"github.com/cajasmota/archigraph/internal/types"
)

func extractDartFixture(t *testing.T, src string) []types.EntityRecord {
	t.Helper()
	ext, _ := extractor.Get("dart")
	ents, err := ext.Extract(context.Background(), extractor.FileInput{
		Path:     "types.dart",
		Content:  []byte(src),
		Language: "dart",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	return ents
}

func findByKind(ents []types.EntityRecord, kind, name string) *types.EntityRecord {
	for i := range ents {
		if ents[i].Kind == kind && ents[i].Name == name {
			return &ents[i]
		}
	}
	return nil
}

func TestDartTypes_PlainEnum(t *testing.T) {
	src := `enum Color {
  red,
  green,
  blue,
}
`
	ents := extractDartFixture(t, src)
	en := findByKind(ents, "SCOPE.Enum", "Color")
	if en == nil {
		t.Fatal("expected SCOPE.Enum:Color")
	}
	if en.Subtype != "enum" {
		t.Errorf("Subtype = %q, want enum", en.Subtype)
	}
	if en.Properties["kind_hint"] != "dart_enum" {
		t.Errorf("kind_hint = %q, want dart_enum", en.Properties["kind_hint"])
	}
	if got := en.Properties["members"]; got != "red, green, blue" {
		t.Errorf("members = %q, want \"red, green, blue\"", got)
	}
	if en.Language != "dart" {
		t.Errorf("Language = %q, want dart", en.Language)
	}
}

func TestDartTypes_EnhancedEnum(t *testing.T) {
	// Dart 2.17 enhanced enum: constants with ctor args, then fields/methods.
	src := `enum Planet {
  mercury(3.3e23),
  earth(5.97e24);

  const Planet(this.mass);
  final double mass;

  double surfaceGravity() => mass;
}
`
	ents := extractDartFixture(t, src)
	en := findByKind(ents, "SCOPE.Enum", "Planet")
	if en == nil {
		t.Fatal("expected SCOPE.Enum:Planet")
	}
	if got := en.Properties["members"]; got != "mercury, earth" {
		t.Errorf("members = %q, want \"mercury, earth\" (post-`;` noise must be dropped)", got)
	}
	if got := en.Properties["member_count"]; got != "2" {
		t.Errorf("member_count = %q, want 2", got)
	}
}

func TestDartTypes_TypedefAlias(t *testing.T) {
	src := `typedef JsonMap = Map<String, dynamic>;
typedef IntList = List<int>;
`
	ents := extractDartFixture(t, src)
	ta := findByKind(ents, "SCOPE.Schema", "JsonMap")
	if ta == nil {
		t.Fatal("expected SCOPE.Schema:JsonMap")
	}
	if ta.Subtype != "type_alias" {
		t.Errorf("Subtype = %q, want type_alias", ta.Subtype)
	}
	if got := ta.Properties["type_body"]; got != "Map<String, dynamic>" {
		t.Errorf("type_body = %q, want \"Map<String, dynamic>\"", got)
	}
	if findByKind(ents, "SCOPE.Schema", "IntList") == nil {
		t.Error("expected SCOPE.Schema:IntList")
	}
}

func TestDartTypes_TypedefFunc(t *testing.T) {
	// Legacy function-type typedef (no `=`).
	src := `typedef int Comparator(Object a, Object b);
`
	ents := extractDartFixture(t, src)
	ta := findByKind(ents, "SCOPE.Schema", "Comparator")
	if ta == nil {
		t.Fatal("expected SCOPE.Schema:Comparator (function-type typedef)")
	}
	if ta.Subtype != "type_alias" {
		t.Errorf("Subtype = %q, want type_alias", ta.Subtype)
	}
}

func TestDartTypes_SealedClass(t *testing.T) {
	src := `sealed class Shape {}

final class Circle extends Shape {}

interface class Drawable {}
`
	ents := extractDartFixture(t, src)
	shape := findByKind(ents, "SCOPE.Component", "Shape")
	if shape == nil {
		t.Fatal("expected SCOPE.Component:Shape (sealed class)")
	}
	if shape.Subtype != "class" {
		t.Errorf("Subtype = %q, want class", shape.Subtype)
	}
	if shape.Properties["dart_sealed"] != "true" {
		t.Errorf("dart_sealed = %q, want true", shape.Properties["dart_sealed"])
	}
	if shape.Properties["class_modifier"] != "sealed" {
		t.Errorf("class_modifier = %q, want sealed", shape.Properties["class_modifier"])
	}
	if findByKind(ents, "SCOPE.Component", "Circle") == nil {
		t.Error("expected SCOPE.Component:Circle (final class)")
	}
	draw := findByKind(ents, "SCOPE.Component", "Drawable")
	if draw == nil || draw.Properties["dart_interface"] != "true" {
		t.Error("expected SCOPE.Component:Drawable with dart_interface=true")
	}
}

func TestDartTypes_PlainClassNotDoubleEmitted(t *testing.T) {
	// A plain `class` / `abstract class` must NOT be emitted by the modified-
	// class pass (the base walk owns them) — guard against double-emit.
	src := `class Plain {}
abstract class Base {}
`
	ents := extractDartFixture(t, src)
	plainCount := 0
	for _, e := range ents {
		if e.Kind == "SCOPE.Component" && e.Name == "Plain" {
			plainCount++
		}
	}
	if plainCount != 1 {
		t.Errorf("Plain emitted %d times, want exactly 1 (no double-emit)", plainCount)
	}
}
