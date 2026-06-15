package golang_test

import (
	"context"
	"testing"

	extreg "github.com/cajasmota/grafel/internal/extractor"
	"github.com/cajasmota/grafel/internal/types"
)

// gormExtractFull runs the GORM extractor and returns full EntityRecords so
// data-lifecycle trait properties can be value-asserted (the entitySummary
// helper drops Properties).
func gormExtractFull(t *testing.T, file extreg.FileInput) []types.EntityRecord {
	t.Helper()
	e, ok := extreg.Get("custom_go_gorm")
	if !ok {
		t.Fatal("custom_go_gorm not registered")
	}
	ents, err := e.Extract(context.Background(), file)
	if err != nil {
		t.Fatalf("extract: %v", err)
	}
	return ents
}

// schemaByName returns the SCOPE.Schema model entity named name.
func schemaByName(t *testing.T, ents []types.EntityRecord, name string) types.EntityRecord {
	t.Helper()
	for _, e := range ents {
		if e.Kind == "SCOPE.Schema" && e.Name == name {
			return e
		}
	}
	t.Fatalf("no SCOPE.Schema entity named %q", name)
	return types.EntityRecord{}
}

// TestGORMLifecycleTraits asserts the exact soft_delete / timestamps /
// audit_columns flat props stamped on specific GORM model entities, and the
// honesty boundary (a plain `deleted` bool is NOT soft-delete).
func TestGORMLifecycleTraits(t *testing.T) {
	ents := gormExtractFull(t, fixtureInput(t, "gorm_lifecycle.go", "go"))

	// Account embeds gorm.Model -> soft_delete (deleted_at) + timestamps, and
	// declares created_by/updated_by audit columns.
	acct := schemaByName(t, ents, "Account")
	if acct.Properties["soft_delete"] != "true" {
		t.Errorf("Account soft_delete: want true, got %q", acct.Properties["soft_delete"])
	}
	if acct.Properties["soft_delete_column"] != "deleted_at" {
		t.Errorf("Account soft_delete_column: want deleted_at, got %q", acct.Properties["soft_delete_column"])
	}
	if acct.Properties["timestamps"] != "true" {
		t.Errorf("Account timestamps: want true, got %q", acct.Properties["timestamps"])
	}
	if got := acct.Properties["audit_columns"]; got != "created_by,updated_by" {
		t.Errorf("Account audit_columns: want created_by,updated_by, got %q", got)
	}

	// Invoice uses an explicit gorm.DeletedAt field with a custom column, plus
	// explicit CreatedAt+UpdatedAt -> soft_delete (archived_at) + timestamps.
	inv := schemaByName(t, ents, "Invoice")
	if inv.Properties["soft_delete"] != "true" {
		t.Errorf("Invoice soft_delete: want true, got %q", inv.Properties["soft_delete"])
	}
	if inv.Properties["soft_delete_column"] != "archived_at" {
		t.Errorf("Invoice soft_delete_column: want archived_at, got %q", inv.Properties["soft_delete_column"])
	}
	if inv.Properties["timestamps"] != "true" {
		t.Errorf("Invoice timestamps: want true, got %q", inv.Properties["timestamps"])
	}
	if _, ok := inv.Properties["audit_columns"]; ok {
		t.Errorf("Invoice should have no audit_columns, got %q", inv.Properties["audit_columns"])
	}

	// Ledger: plain `deleted` bool, no lib/DeletedAt/deleted_at column, no
	// timestamp columns -> NO lifecycle traits at all (honesty boundary).
	ledger := schemaByName(t, ents, "Ledger")
	if _, ok := ledger.Properties["soft_delete"]; ok {
		t.Errorf("Ledger must NOT be soft_delete (plain deleted bool)")
	}
	if _, ok := ledger.Properties["timestamps"]; ok {
		t.Errorf("Ledger must NOT have timestamps")
	}
	if _, ok := ledger.Properties["audit_columns"]; ok {
		t.Errorf("Ledger must NOT have audit_columns")
	}
}

// TestGORMLifecycleExistingFixture re-asserts against the shared gorm_models.go
// fixture: User embeds gorm.Model (soft_delete + timestamps); Company is a
// plain field-tag model with no lifecycle markers (no traits).
func TestGORMLifecycleExistingFixture(t *testing.T) {
	ents := gormExtractFull(t, fixtureInput(t, "gorm_models.go", "go"))

	user := schemaByName(t, ents, "User")
	if user.Properties["soft_delete"] != "true" || user.Properties["soft_delete_column"] != "deleted_at" {
		t.Errorf("User: want soft_delete=true/deleted_at, got %q/%q",
			user.Properties["soft_delete"], user.Properties["soft_delete_column"])
	}
	if user.Properties["timestamps"] != "true" {
		t.Errorf("User timestamps: want true, got %q", user.Properties["timestamps"])
	}

	company := schemaByName(t, ents, "Company")
	if _, ok := company.Properties["soft_delete"]; ok {
		t.Errorf("Company must NOT be soft_delete")
	}
	if _, ok := company.Properties["timestamps"]; ok {
		t.Errorf("Company must NOT have timestamps")
	}
}
