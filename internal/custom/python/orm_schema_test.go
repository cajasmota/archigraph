package python_test

// orm_schema_test.go — field-level (schema) extraction tests for
// Peewee, Pony ORM, Beanie, MongoEngine, and Tortoise ORM.
//
// Issue #3072 — ORM schema extraction for peewee/pony/beanie/mongoengine/tortoise.

import (
	"os"
	"testing"
)

// ============================================================================
// Helpers
// ============================================================================

// fixtureSchema loads a testdata file and returns its content.
func fixtureSchema(t *testing.T, name string) string {
	t.Helper()
	b, err := os.ReadFile("testdata/" + name)
	if err != nil {
		t.Fatalf("fixtureSchema: %v", err)
	}
	return string(b)
}

// hasSchemaEntity returns true if any entity in result matches name+kind+props.
func hasSchemaEntity(result []extractResult, name, kind string, props map[string]string) bool {
	for _, e := range result {
		if e.Name != name || e.Kind != kind {
			continue
		}
		match := true
		for k, v := range props {
			if e.Props[k] != v {
				match = false
				break
			}
		}
		if match {
			return true
		}
	}
	return false
}

// countSchemaEntities returns the count of entities matching kind+props.
func countSchemaEntities(result []extractResult, kind string, props map[string]string) int {
	n := 0
	for _, e := range result {
		if e.Kind != kind {
			continue
		}
		match := true
		for k, v := range props {
			if e.Props[k] != v {
				match = false
				break
			}
		}
		if match {
			n++
		}
	}
	return n
}

// ============================================================================
// Peewee schema extraction tests
// ============================================================================

func TestPeeweeSchema_ModelEmitted(t *testing.T) {
	src := fixtureSchema(t, "peewee_schema.py")
	ents := extract(t, "python_peewee_schema", src)

	if !hasSchemaEntity(ents, "User", "SCOPE.Schema", map[string]string{
		"framework":    "peewee",
		"pattern_type": "model",
		"class_name":   "User",
	}) {
		t.Fatal("expected User model entity")
	}
	if !hasSchemaEntity(ents, "Post", "SCOPE.Schema", map[string]string{
		"framework":    "peewee",
		"pattern_type": "model",
		"class_name":   "Post",
	}) {
		t.Fatal("expected Post model entity")
	}
}

func TestPeeweeSchema_FieldsEmitted(t *testing.T) {
	src := fixtureSchema(t, "peewee_schema.py")
	ents := extract(t, "python_peewee_schema", src)

	expected := []struct {
		name      string
		fieldType string
	}{
		{"User.username", "CharField"},
		{"User.email", "CharField"},
		{"User.age", "IntegerField"},
		{"User.active", "BooleanField"},
		{"User.created_at", "DateTimeField"},
		{"Post.title", "CharField"},
		{"Post.body", "TextField"},
		{"Post.score", "FloatField"},
		{"Post.published", "BooleanField"},
	}

	for _, tc := range expected {
		if !hasSchemaEntity(ents, tc.name, "SCOPE.Schema", map[string]string{
			"framework":    "peewee",
			"pattern_type": "field",
			"field_type":   tc.fieldType,
		}) {
			t.Errorf("expected field entity %q with type %q", tc.name, tc.fieldType)
		}
	}
}

func TestPeeweeSchema_FieldSubtype(t *testing.T) {
	src := fixtureSchema(t, "peewee_schema.py")
	ents := extract(t, "python_peewee_schema", src)

	for _, e := range ents {
		if e.Props["pattern_type"] == "field" && e.Subtype != "column" {
			t.Errorf("field entity %q: expected subtype 'column', got %q", e.Name, e.Subtype)
		}
	}
}

func TestPeeweeSchema_NoFalsePositiveWithoutPeewee(t *testing.T) {
	src := `
class MyClass(object):
    name = CharField(max_length=100)
`
	ents := extract(t, "python_peewee_schema", src)
	if len(ents) != 0 {
		t.Fatalf("expected 0 entities for non-peewee file, got %d", len(ents))
	}
}

func TestPeeweeSchema_InlineSource(t *testing.T) {
	src := `import peewee
from peewee import Model, CharField, IntegerField

class Article(Model):
    title = CharField(max_length=200)
    views = IntegerField(default=0)
`
	ents := extract(t, "python_peewee_schema", src)

	if !hasSchemaEntity(ents, "Article", "SCOPE.Schema", map[string]string{"framework": "peewee", "pattern_type": "model"}) {
		t.Fatal("expected Article model entity")
	}
	if !hasSchemaEntity(ents, "Article.title", "SCOPE.Schema", map[string]string{"framework": "peewee", "pattern_type": "field", "field_type": "CharField"}) {
		t.Fatal("expected Article.title field entity")
	}
	if !hasSchemaEntity(ents, "Article.views", "SCOPE.Schema", map[string]string{"framework": "peewee", "pattern_type": "field", "field_type": "IntegerField"}) {
		t.Fatal("expected Article.views field entity")
	}
}

// ============================================================================
// Pony ORM schema extraction tests
// ============================================================================

func TestPonySchema_EntityEmitted(t *testing.T) {
	src := fixtureSchema(t, "pony_schema.py")
	ents := extract(t, "python_pony_schema", src)

	if !hasSchemaEntity(ents, "Author", "SCOPE.Schema", map[string]string{
		"framework":    "pony",
		"pattern_type": "entity",
		"class_name":   "Author",
	}) {
		t.Fatal("expected Author entity")
	}
	if !hasSchemaEntity(ents, "Book", "SCOPE.Schema", map[string]string{
		"framework":    "pony",
		"pattern_type": "entity",
		"class_name":   "Book",
	}) {
		t.Fatal("expected Book entity")
	}
}

func TestPonySchema_AttributesEmitted(t *testing.T) {
	src := fixtureSchema(t, "pony_schema.py")
	ents := extract(t, "python_pony_schema", src)

	expected := []struct {
		name       string
		descriptor string
	}{
		{"Author.name", "Required"},
		{"Author.email", "Optional"},
		{"Book.title", "Required"},
		{"Book.isbn", "PrimaryKey"},
		{"Book.year", "Optional"},
		{"Book.price", "Optional"},
		{"Book.author", "Required"},
	}

	for _, tc := range expected {
		if !hasSchemaEntity(ents, tc.name, "SCOPE.Schema", map[string]string{
			"framework":    "pony",
			"pattern_type": "attribute",
			"descriptor":   tc.descriptor,
		}) {
			t.Errorf("expected attribute entity %q with descriptor %q", tc.name, tc.descriptor)
		}
	}
}

func TestPonySchema_SetRelationNotField(t *testing.T) {
	// Set() is also captured since it is a valid Pony descriptor
	src := `from pony.orm import Database, Required, Set
db = Database()
class Author(db.Entity):
    name = Required(str)
    books = Set("Book")
`
	ents := extract(t, "python_pony_schema", src)
	if !hasSchemaEntity(ents, "Author.books", "SCOPE.Schema", map[string]string{"descriptor": "Set"}) {
		t.Fatal("expected Author.books Set attribute entity")
	}
}

func TestPonySchema_NoFalsePositiveWithoutPony(t *testing.T) {
	src := `
class Author(object):
    name = Required(str)
`
	ents := extract(t, "python_pony_schema", src)
	if len(ents) != 0 {
		t.Fatalf("expected 0 entities for non-pony file, got %d", len(ents))
	}
}

// ============================================================================
// Beanie schema extraction tests
// ============================================================================

func TestBeanieSchema_DocumentEmitted(t *testing.T) {
	src := fixtureSchema(t, "beanie_schema.py")
	ents := extract(t, "python_beanie_schema", src)

	if !hasSchemaEntity(ents, "Category", "SCOPE.Schema", map[string]string{
		"framework":    "beanie",
		"pattern_type": "document",
		"class_name":   "Category",
	}) {
		t.Fatal("expected Category document entity")
	}
	if !hasSchemaEntity(ents, "Product", "SCOPE.Schema", map[string]string{
		"framework":    "beanie",
		"pattern_type": "document",
		"class_name":   "Product",
	}) {
		t.Fatal("expected Product document entity")
	}
}

func TestBeanieSchema_FieldsEmitted(t *testing.T) {
	src := fixtureSchema(t, "beanie_schema.py")
	ents := extract(t, "python_beanie_schema", src)

	expectedFields := []string{
		"Category.name",
		"Category.description",
		"Product.title",
		"Product.price",
		"Product.stock",
		"Product.tags",
		"Product.category",
	}

	for _, name := range expectedFields {
		if !hasSchemaEntity(ents, name, "SCOPE.Schema", map[string]string{
			"framework":    "beanie",
			"pattern_type": "field",
		}) {
			t.Errorf("expected field entity %q", name)
		}
	}
}

func TestBeanieSchema_FieldSubtype(t *testing.T) {
	src := fixtureSchema(t, "beanie_schema.py")
	ents := extract(t, "python_beanie_schema", src)

	for _, e := range ents {
		if e.Props["pattern_type"] == "field" && e.Subtype != "column" {
			t.Errorf("field entity %q: expected subtype 'column', got %q", e.Name, e.Subtype)
		}
	}
}

func TestBeanieSchema_InlineSource(t *testing.T) {
	src := `from beanie import Document
from typing import Optional

class Item(Document):
    name: str
    price: float
    description: Optional[str] = None
`
	ents := extract(t, "python_beanie_schema", src)

	if !hasSchemaEntity(ents, "Item", "SCOPE.Schema", map[string]string{"framework": "beanie", "pattern_type": "document"}) {
		t.Fatal("expected Item document entity")
	}
	if !hasSchemaEntity(ents, "Item.name", "SCOPE.Schema", map[string]string{"framework": "beanie", "pattern_type": "field"}) {
		t.Fatal("expected Item.name field entity")
	}
	if !hasSchemaEntity(ents, "Item.price", "SCOPE.Schema", map[string]string{"framework": "beanie", "pattern_type": "field"}) {
		t.Fatal("expected Item.price field entity")
	}
}

func TestBeanieSchema_NoFalsePositiveWithoutBeanie(t *testing.T) {
	src := `from pydantic import BaseModel

class Item(BaseModel):
    name: str
    price: float
`
	ents := extract(t, "python_beanie_schema", src)
	if len(ents) != 0 {
		t.Fatalf("expected 0 entities for non-beanie file, got %d", len(ents))
	}
}

// ============================================================================
// MongoEngine schema extraction tests
// ============================================================================

func TestMongoEngineSchema_DocumentEmitted(t *testing.T) {
	src := fixtureSchema(t, "mongoengine_schema.py")
	ents := extract(t, "python_mongoengine_schema", src)

	if !hasSchemaEntity(ents, "Customer", "SCOPE.Schema", map[string]string{
		"framework":    "mongoengine",
		"pattern_type": "document",
		"class_name":   "Customer",
	}) {
		t.Fatal("expected Customer document entity")
	}
	if !hasSchemaEntity(ents, "Address", "SCOPE.Schema", map[string]string{
		"framework":    "mongoengine",
		"pattern_type": "document",
		"class_name":   "Address",
	}) {
		t.Fatal("expected Address embedded document entity")
	}
}

func TestMongoEngineSchema_FieldsEmitted(t *testing.T) {
	src := fixtureSchema(t, "mongoengine_schema.py")
	ents := extract(t, "python_mongoengine_schema", src)

	expected := []struct {
		name      string
		fieldType string
	}{
		{"Address.street", "StringField"},
		{"Address.city", "StringField"},
		{"Address.zipcode", "StringField"},
		{"Customer.name", "StringField"},
		{"Customer.email", "StringField"},
		{"Customer.age", "IntField"},
		{"Customer.balance", "FloatField"},
		{"Customer.active", "BooleanField"},
		{"Customer.address", "EmbeddedDocumentField"},
		{"Customer.tags", "ListField"},
	}

	for _, tc := range expected {
		if !hasSchemaEntity(ents, tc.name, "SCOPE.Schema", map[string]string{
			"framework":    "mongoengine",
			"pattern_type": "field",
			"field_type":   tc.fieldType,
		}) {
			t.Errorf("expected field entity %q with type %q", tc.name, tc.fieldType)
		}
	}
}

func TestMongoEngineSchema_NoFalsePositiveWithoutMongoEngine(t *testing.T) {
	src := `
class Customer(object):
    name = StringField(required=True)
`
	ents := extract(t, "python_mongoengine_schema", src)
	if len(ents) != 0 {
		t.Fatalf("expected 0 entities for non-mongoengine file, got %d", len(ents))
	}
}

func TestMongoEngineSchema_InlineSource(t *testing.T) {
	src := `import mongoengine
from mongoengine import Document, StringField, IntField

class BlogPost(Document):
    title = StringField(required=True, max_length=200)
    views = IntField(default=0)
`
	ents := extract(t, "python_mongoengine_schema", src)

	if !hasSchemaEntity(ents, "BlogPost", "SCOPE.Schema", map[string]string{"framework": "mongoengine", "pattern_type": "document"}) {
		t.Fatal("expected BlogPost document entity")
	}
	if !hasSchemaEntity(ents, "BlogPost.title", "SCOPE.Schema", map[string]string{"framework": "mongoengine", "pattern_type": "field", "field_type": "StringField"}) {
		t.Fatal("expected BlogPost.title field entity")
	}
	if !hasSchemaEntity(ents, "BlogPost.views", "SCOPE.Schema", map[string]string{"framework": "mongoengine", "pattern_type": "field", "field_type": "IntField"}) {
		t.Fatal("expected BlogPost.views field entity")
	}
}

// ============================================================================
// Tortoise ORM schema extraction tests
// ============================================================================

func TestTortoiseSchema_ModelEmitted(t *testing.T) {
	src := fixtureSchema(t, "tortoise_schema.py")
	ents := extract(t, "python_tortoise_schema", src)

	if !hasSchemaEntity(ents, "Tournament", "SCOPE.Schema", map[string]string{
		"framework":    "tortoise",
		"pattern_type": "model",
		"class_name":   "Tournament",
	}) {
		t.Fatal("expected Tournament model entity")
	}
	if !hasSchemaEntity(ents, "Event", "SCOPE.Schema", map[string]string{
		"framework":    "tortoise",
		"pattern_type": "model",
		"class_name":   "Event",
	}) {
		t.Fatal("expected Event model entity")
	}
}

func TestTortoiseSchema_ColumnsEmitted(t *testing.T) {
	src := fixtureSchema(t, "tortoise_schema.py")
	ents := extract(t, "python_tortoise_schema", src)

	expected := []struct {
		name      string
		fieldType string
	}{
		{"Tournament.id", "fields.IntField"},
		{"Tournament.name", "fields.CharField"},
		{"Tournament.created_at", "fields.DatetimeField"},
		{"Tournament.active", "fields.BooleanField"},
		{"Event.id", "fields.IntField"},
		{"Event.name", "fields.CharField"},
		{"Event.prize", "fields.DecimalField"},
		{"Event.description", "fields.TextField"},
	}

	for _, tc := range expected {
		if !hasSchemaEntity(ents, tc.name, "SCOPE.Schema", map[string]string{
			"framework":    "tortoise",
			"pattern_type": "column",
			"field_type":   tc.fieldType,
		}) {
			t.Errorf("expected column entity %q with type %q", tc.name, tc.fieldType)
		}
	}
}

func TestTortoiseSchema_ColumnSubtype(t *testing.T) {
	src := fixtureSchema(t, "tortoise_schema.py")
	ents := extract(t, "python_tortoise_schema", src)

	for _, e := range ents {
		if e.Props["pattern_type"] == "column" && e.Subtype != "column" {
			t.Errorf("column entity %q: expected subtype 'column', got %q", e.Name, e.Subtype)
		}
	}
}

func TestTortoiseSchema_NoFalsePositiveWithoutTortoise(t *testing.T) {
	src := `
class Event(Model):
    name = CharField(max_length=255)
`
	ents := extract(t, "python_tortoise_schema", src)
	if len(ents) != 0 {
		t.Fatalf("expected 0 entities for non-tortoise file, got %d", len(ents))
	}
}

func TestTortoiseSchema_InlineSource(t *testing.T) {
	src := `from tortoise import fields
from tortoise.models import Model

class User(Model):
    id = fields.IntField(pk=True)
    username = fields.CharField(max_length=50)
    email = fields.CharField(max_length=255)
`
	ents := extract(t, "python_tortoise_schema", src)

	if !hasSchemaEntity(ents, "User", "SCOPE.Schema", map[string]string{"framework": "tortoise", "pattern_type": "model"}) {
		t.Fatal("expected User model entity")
	}
	if !hasSchemaEntity(ents, "User.id", "SCOPE.Schema", map[string]string{"framework": "tortoise", "pattern_type": "column", "field_type": "fields.IntField"}) {
		t.Fatal("expected User.id column entity")
	}
	if !hasSchemaEntity(ents, "User.username", "SCOPE.Schema", map[string]string{"framework": "tortoise", "pattern_type": "column", "field_type": "fields.CharField"}) {
		t.Fatal("expected User.username column entity")
	}
}
