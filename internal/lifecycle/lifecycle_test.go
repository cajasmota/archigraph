package lifecycle

import (
	"sort"
	"testing"
)

func stamped(t Traits) map[string]string {
	out := map[string]string{}
	t.Stamp(func(kv ...string) {
		for i := 0; i+1 < len(kv); i += 2 {
			out[kv[i]] = kv[i+1]
		}
	})
	return out
}

func TestGORM_Embed(t *testing.T) {
	got := GORM(GORMInput{EmbedsGormModel: true})
	if !got.SoftDelete || got.SoftDeleteColumn != "deleted_at" {
		t.Errorf("embed: want soft_delete deleted_at, got %+v", got)
	}
	if !got.Timestamps {
		t.Error("embed: want timestamps")
	}
}

func TestGORM_ExplicitDeletedAt(t *testing.T) {
	got := GORM(GORMInput{DeletedAtColumn: "archived_at", HasCreatedAt: true, HasUpdatedAt: true})
	if !got.SoftDelete || got.SoftDeleteColumn != "archived_at" {
		t.Errorf("want soft_delete archived_at, got %+v", got)
	}
	if !got.Timestamps {
		t.Error("want timestamps from CreatedAt+UpdatedAt")
	}
}

func TestGORM_CreatedAtOnly_NoTimestamps(t *testing.T) {
	got := GORM(GORMInput{HasCreatedAt: true})
	if got.Timestamps {
		t.Error("CreatedAt without UpdatedAt must NOT assert timestamps")
	}
	if got.SoftDelete {
		t.Error("no DeletedAt/embed must NOT assert soft_delete")
	}
}

func TestGORM_PlainDeletedBool_NotSoftDelete(t *testing.T) {
	got := GORM(GORMInput{Columns: []string{"id", "deleted", "note"}})
	if got.SoftDelete {
		t.Error("plain deleted column must NOT assert soft_delete")
	}
	if len(got.AuditColumns) != 0 {
		t.Error("no audit columns expected")
	}
}

func TestGORM_AuditColumns(t *testing.T) {
	got := GORM(GORMInput{Columns: []string{"id", "created_by", "updated_by", "name"}})
	want := []string{"created_by", "updated_by"}
	if len(got.AuditColumns) != 2 || got.AuditColumns[0] != want[0] || got.AuditColumns[1] != want[1] {
		t.Errorf("audit: want %v got %v", want, got.AuditColumns)
	}
}

func TestRails_ActsAsParanoid(t *testing.T) {
	got := RailsModelTraits("class U < ApplicationRecord\n acts_as_paranoid\nend", nil)
	if !got.SoftDelete || got.SoftDeleteColumn != "deleted_at" {
		t.Errorf("want soft_delete deleted_at, got %+v", got)
	}
	if got.Timestamps {
		t.Error("Rails timestamps must stay honest-partial (omitted)")
	}
}

func TestRails_DefaultScopeSoftDelete(t *testing.T) {
	got := RailsModelTraits("default_scope { where(deleted_at: nil) }", nil)
	if !got.SoftDelete {
		t.Error("default_scope deleted_at: nil must be soft_delete")
	}
}

func TestRails_OrderingScope_NotSoftDelete(t *testing.T) {
	got := RailsModelTraits("default_scope { order(created_at: :desc) }", nil)
	if got.SoftDelete {
		t.Error("ordering default_scope must NOT be soft_delete")
	}
}

func TestStamp_OmitsAbsent(t *testing.T) {
	props := stamped(Traits{})
	if len(props) != 0 {
		t.Errorf("empty traits must stamp nothing, got %v", props)
	}
	props = stamped(Traits{SoftDelete: true, SoftDeleteColumn: "deleted_at", AuditColumns: []string{"created_by"}})
	keys := make([]string, 0, len(props))
	for k := range props {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	if props["soft_delete"] != "true" || props["soft_delete_column"] != "deleted_at" || props["audit_columns"] != "created_by" {
		t.Errorf("unexpected stamp: %v", props)
	}
	if _, ok := props["timestamps"]; ok {
		t.Error("timestamps must be omitted when false")
	}
}
