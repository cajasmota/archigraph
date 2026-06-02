package ruby_test

// activerecord_lifecycle_test.go — value-asserting tests for the data-lifecycle
// traits (#3628 child) stamped on the SCOPE.Schema/model entity: soft-delete
// (acts_as_paranoid / default_scope deleted_at), the require-convention honesty
// boundary, and audit columns. Timestamps stay honest-partial for Rails (they
// live in the schema, not the model body), so they are never asserted here.

import (
	"testing"
)

func TestLifecycle_ActsAsParanoidSoftDelete(t *testing.T) {
	src := `class User < ApplicationRecord
  acts_as_paranoid
  has_many :orders
end`
	ents := arExtractRaw(t, "app/models/user.rb", src)
	m := findEnt(ents, "model:User")
	if m == nil {
		t.Fatal("expected model:User entity")
	}
	mustProp(t, m, "soft_delete", "true")
	mustProp(t, m, "soft_delete_column", "deleted_at")
}

func TestLifecycle_DefaultScopeSoftDelete(t *testing.T) {
	src := `class Post < ApplicationRecord
  default_scope { where(deleted_at: nil) }
end`
	ents := arExtractRaw(t, "app/models/post.rb", src)
	m := findEnt(ents, "model:Post")
	if m == nil {
		t.Fatal("expected model:Post entity")
	}
	mustProp(t, m, "soft_delete", "true")
	mustProp(t, m, "soft_delete_column", "deleted_at")
}

// Honesty boundary: a plain `deleted` boolean / unrelated default_scope must
// NOT be classified as soft-delete.
func TestLifecycle_PlainDeletedNotSoftDelete(t *testing.T) {
	src := `class Widget < ApplicationRecord
  default_scope { order(created_at: :desc) }
  attribute :deleted, :boolean
end`
	ents := arExtractRaw(t, "app/models/widget.rb", src)
	m := findEnt(ents, "model:Widget")
	if m == nil {
		t.Fatal("expected model:Widget entity")
	}
	if _, ok := m.Properties["soft_delete"]; ok {
		t.Errorf("Widget must NOT be soft_delete (plain deleted bool / ordering scope)")
	}
}

func TestLifecycle_AuditColumns(t *testing.T) {
	src := `class Document < ApplicationRecord
  acts_as_paranoid
  belongs_to :created_by, class_name: "User"
  belongs_to :updated_by, class_name: "User"
end`
	ents := arExtractRaw(t, "app/models/document.rb", src)
	m := findEnt(ents, "model:Document")
	if m == nil {
		t.Fatal("expected model:Document entity")
	}
	mustProp(t, m, "soft_delete", "true")
	mustProp(t, m, "audit_columns", "created_by,updated_by")
}

// A model with no lifecycle markers carries no lifecycle props (honest absence).
func TestLifecycle_NoTraits(t *testing.T) {
	src := `class Tag < ApplicationRecord
  has_many :taggings
end`
	ents := arExtractRaw(t, "app/models/tag.rb", src)
	m := findEnt(ents, "model:Tag")
	if m == nil {
		t.Fatal("expected model:Tag entity")
	}
	if _, ok := m.Properties["soft_delete"]; ok {
		t.Error("Tag must NOT be soft_delete")
	}
	if _, ok := m.Properties["timestamps"]; ok {
		t.Error("Tag must NOT have timestamps (honest-partial for Rails)")
	}
	if _, ok := m.Properties["audit_columns"]; ok {
		t.Error("Tag must NOT have audit_columns")
	}
}
