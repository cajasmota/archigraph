// Package lifecycle provides shared, pure detection of ORM model
// data-lifecycle traits — soft-delete, created/updated timestamps, and
// created-by/updated-by audit columns — so per-language ORM extractors can
// stamp a uniform set of flat properties onto the model entity they already
// emit. This lets the graph answer "which models soft-delete?" and "which
// track timestamps?" for rewrite data-lifecycle parity.
//
// HONESTY BOUNDARY: detection requires a recognised soft-delete library /
// column convention or an explicit timestamp/audit column name. It never
// guesses soft-delete from an arbitrary "deleted"-named boolean — a plain
// `deleted` flag with no paranoia scope / library / `deleted_at`-style column
// is NOT reported as soft-delete. Ambiguous signals are omitted (honest
// partial) rather than asserted.
package lifecycle

import "strings"

// Traits is the resolved data-lifecycle trait set for one ORM model. Zero
// value means "no recognised lifecycle traits" — every field is then omitted
// from the entity properties by Stamp.
type Traits struct {
	SoftDelete       bool     // model performs soft-deletes
	SoftDeleteColumn string   // the soft-delete marker column, when known
	Timestamps       bool     // model tracks created/updated timestamps
	AuditColumns     []string // created_by / updated_by style audit columns
}

// PropSetter is the minimal interface the per-language extractors expose for
// stamping flat string properties (matches custom.setProps semantics: even
// key/value pairs). Implemented inline by each caller via a closure.
type PropSetter func(kv ...string)

// Stamp writes the recognised traits as flat properties via set. Only
// asserted traits are written, so a model with no recognised lifecycle traits
// gets no lifecycle properties at all (honest absence).
func (t Traits) Stamp(set PropSetter) {
	if t.SoftDelete {
		set("soft_delete", "true")
		if t.SoftDeleteColumn != "" {
			set("soft_delete_column", t.SoftDeleteColumn)
		}
	}
	if t.Timestamps {
		set("timestamps", "true")
	}
	if len(t.AuditColumns) > 0 {
		set("audit_columns", strings.Join(t.AuditColumns, ","))
	}
}

// auditColumnNames is the closed convention set for created-by/updated-by
// audit columns. We require an explicit, conventional name — we do NOT infer
// audit semantics from arbitrary "*_by" identifiers.
var auditColumnNames = []string{
	"created_by", "updated_by", "creator_id", "updater_id",
	"deleted_by", "created_by_id", "updated_by_id",
}

// collectAuditColumns returns the conventional audit columns present in cols,
// preserving auditColumnNames order and de-duplicating. cols are matched
// case-insensitively against the convention set.
func collectAuditColumns(cols []string) []string {
	have := make(map[string]bool, len(cols))
	for _, c := range cols {
		have[strings.ToLower(strings.TrimSpace(c))] = true
	}
	var out []string
	for _, name := range auditColumnNames {
		if have[name] {
			out = append(out, name)
		}
	}
	return out
}

// --- GORM (Go) -------------------------------------------------------------

// GORMInput carries the GORM-model facts a Go extractor has already parsed:
// whether the struct embeds gorm.Model (which contributes CreatedAt /
// UpdatedAt / DeletedAt), the column name of any `gorm.DeletedAt`-typed field,
// and the resolved column names of every field on the struct.
type GORMInput struct {
	EmbedsGormModel bool     // struct embeds gorm.Model
	DeletedAtColumn string   // column of a gorm.DeletedAt-typed field, if any
	HasCreatedAt    bool     // a CreatedAt column/field is present
	HasUpdatedAt    bool     // an UpdatedAt column/field is present
	Columns         []string // all resolved column names on the struct
}

// GORM resolves lifecycle traits for a GORM model.
//
//   - gorm.Model embed contributes DeletedAt (soft-delete, column deleted_at)
//     AND CreatedAt+UpdatedAt (timestamps).
//   - an explicit gorm.DeletedAt field is soft-delete with that field's column.
//   - explicit CreatedAt + UpdatedAt columns (without the embed) are timestamps.
func GORM(in GORMInput) Traits {
	var t Traits
	switch {
	case in.DeletedAtColumn != "":
		t.SoftDelete = true
		t.SoftDeleteColumn = in.DeletedAtColumn
	case in.EmbedsGormModel:
		// gorm.Model embeds `DeletedAt gorm.DeletedAt` → column deleted_at.
		t.SoftDelete = true
		t.SoftDeleteColumn = "deleted_at"
	}
	if in.EmbedsGormModel || (in.HasCreatedAt && in.HasUpdatedAt) {
		t.Timestamps = true
	}
	t.AuditColumns = collectAuditColumns(in.Columns)
	return t
}

// --- ActiveRecord (Ruby) ---------------------------------------------------

// RailsModelTraits resolves lifecycle traits from the body of a Rails model
// class. timestamps live in the migration/schema (not the model body) so this
// stays honest-partial on timestamps — it only asserts soft-delete and audit
// columns, both of which are observable in the model source:
//
//   - `acts_as_paranoid` (paranoia / acts_as_paranoid gems) → soft-delete.
//   - `default_scope { where(deleted_at: nil) }` (and variants) → soft-delete,
//     column deleted_at.
//
// A bare `deleted` boolean with no scope/lib is NOT soft-delete. columns are
// any conventional audit columns referenced in the model body (e.g. via a
// belongs_to or attribute reference); when none are observable the list is
// empty (honest partial — audit columns usually live in the schema).
func RailsModelTraits(body string, columns []string) Traits {
	var t Traits
	low := body

	if strings.Contains(low, "acts_as_paranoid") {
		t.SoftDelete = true
		t.SoftDeleteColumn = "deleted_at"
	}
	// default_scope { where(deleted_at: nil) } / where("deleted_at IS NULL")
	if !t.SoftDelete && railsSoftDeleteScope(low) {
		t.SoftDelete = true
		t.SoftDeleteColumn = "deleted_at"
	}
	t.AuditColumns = collectAuditColumns(columns)
	return t
}

// railsSoftDeleteScope reports whether the model body declares a default scope
// that filters out soft-deleted rows on a deleted_at column. Requires both the
// default_scope macro AND a deleted_at predicate so an unrelated default_scope
// (e.g. ordering) is not mistaken for soft-delete.
func railsSoftDeleteScope(body string) bool {
	if !strings.Contains(body, "default_scope") {
		return false
	}
	return strings.Contains(body, "deleted_at: nil") ||
		strings.Contains(body, "deleted_at IS NULL") ||
		strings.Contains(body, "deleted_at is null")
}
