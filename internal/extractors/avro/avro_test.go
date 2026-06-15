package avro_test

import (
	"context"
	"testing"

	"github.com/cajasmota/grafel/internal/extractor"
	_ "github.com/cajasmota/grafel/internal/extractors/avro"
	"github.com/cajasmota/grafel/internal/types"
)

func extract(t *testing.T, path, src string) []types.EntityRecord {
	t.Helper()
	ext, ok := extractor.Get("avro")
	if !ok {
		t.Fatal("avro extractor not registered")
	}
	ents, err := ext.Extract(context.Background(), extractor.FileInput{
		Path:     path,
		Content:  []byte(src),
		Language: "avro",
	})
	if err != nil {
		t.Fatalf("extract error: %v", err)
	}
	return ents
}

func find(ents []types.EntityRecord, subtype, name string) *types.EntityRecord {
	for i := range ents {
		if ents[i].Subtype == subtype && ents[i].Name == name {
			return &ents[i]
		}
	}
	return nil
}

// TestAvro_RecordFieldType asserts an Avro record User with field id:long.
func TestAvro_RecordFieldType(t *testing.T) {
	src := `{
  "type": "record",
  "name": "User",
  "namespace": "com.example",
  "fields": [
    {"name": "id", "type": "long"},
    {"name": "name", "type": "string"}
  ]
}`
	ents := extract(t, "user.avsc", src)

	rec := find(ents, "record", "User")
	if rec == nil {
		t.Fatal("expected record entity User")
	}
	if rec.Kind != "SCOPE.Schema" {
		t.Errorf("User kind = %q, want SCOPE.Schema", rec.Kind)
	}

	idField := find(ents, "field", "User.id")
	if idField == nil {
		t.Fatal("expected field entity User.id")
	}
	if got := idField.Properties["type"]; got != "long" {
		t.Errorf("User.id type = %q, want long", got)
	}

	// CONTAINS edge record → id field.
	wantRef := "scope:schema:column:avro:user.avsc:User#id"
	found := false
	for _, r := range rec.Relationships {
		if r.Kind == "CONTAINS" && r.ToID == wantRef {
			found = true
		}
	}
	if !found {
		t.Errorf("expected CONTAINS edge User → id (%q)", wantRef)
	}
}

// TestAvro_NamedTypeReference asserts a record field referencing another named
// record yields a REFERENCES edge.
func TestAvro_NamedTypeReference(t *testing.T) {
	src := `{
  "type": "record",
  "name": "User",
  "fields": [
    {"name": "id", "type": "long"},
    {"name": "address", "type": "Address"},
    {"name": "orders", "type": {"type": "array", "items": "Order"}}
  ]
}`
	ents := extract(t, "user.avsc", src)
	rec := find(ents, "record", "User")
	if rec == nil {
		t.Fatal("expected record User")
	}
	wantAddress := extractor.BuildOperationStructuralRef("avro", "user.avsc", "Address")
	wantOrder := extractor.BuildOperationStructuralRef("avro", "user.avsc", "Order")
	gotAddress, gotOrder := false, false
	for _, r := range rec.Relationships {
		if r.Kind != "REFERENCES" {
			continue
		}
		if r.ToID == wantAddress {
			gotAddress = true
		}
		if r.ToID == wantOrder {
			gotOrder = true
		}
	}
	if !gotAddress {
		t.Error("expected REFERENCES edge User → Address")
	}
	if !gotOrder {
		t.Error("expected REFERENCES edge User → Order (array items)")
	}

	// array<Order> field type rendered.
	orders := find(ents, "field", "User.orders")
	if orders == nil || orders.Properties["type"] != "array<Order>" {
		t.Errorf("User.orders type = %v, want array<Order>", orders)
	}
}

// TestAvro_Enum asserts an Avro enum becomes a schema with symbol children.
func TestAvro_Enum(t *testing.T) {
	src := `{"type":"enum","name":"Suit","symbols":["SPADES","HEARTS"]}`
	ents := extract(t, "suit.avsc", src)
	e := find(ents, "enum", "Suit")
	if e == nil {
		t.Fatal("expected enum entity Suit")
	}
	if find(ents, "field", "Suit.SPADES") == nil {
		t.Error("expected symbol field Suit.SPADES")
	}
}

// TestAvro_NonJSON is a no-op.
func TestAvro_NonJSON(t *testing.T) {
	ents := extract(t, "x.avsc", "not json {{{")
	if len(ents) != 0 {
		t.Errorf("expected 0 entities for non-JSON, got %d", len(ents))
	}
}
