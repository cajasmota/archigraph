package python

// django_migration.go — extract one Migration entity per auto-generated
// Django migration file.
//
// # Background (Issue #2283)
//
// Before #1617 the Python extractor walked the full AST of migration files
// and emitted one entity per operation (AddField, RemoveField, AlterField,
// CreateModel, …) plus the Migration class itself. On the Acme corpus that
// produced ≈100 entities for 43 actual files (~2.3× over).
//
// #1617 pruned migration files entirely — keeping only the per-file
// SCOPE.Component/file entity so import resolution stayed intact.
//
// #2283 restores exactly one architectural Migration entity per file.
// Per-operation details are stored as properties on that entity instead of
// being emitted as separate graph nodes, keeping the entity count at 1:1
// with files while preserving all schema-change metadata for downstream
// consumers (enrichment, sequence annotation, MCP inspect).
//
// # Property shape
//
// The entity carries the following properties:
//
//	operations  — JSON-encoded array of operation objects, each with:
//	              { "type": "<OpClass>", "model": "<model_name>", "field": "<field_name>" }
//	              "model" and "field" are omitted when the operation has no matching kwarg.
//
//	op_count    — total number of operations (decimal string) for quick filtering.
//
//	dependencies — comma-separated list of "<app>/<migration>" pairs the
//	               migration depends on; empty string when none.
//
// # Entity identity
//
//	Kind:       "Migration"
//	Subtype:    "django"
//	Name:       the filename without extension (e.g. "0042_device_serial_number")
//	SourceFile: the migration file path
//	Language:   "python"

import (
	"fmt"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"

	"github.com/cajasmota/grafel/internal/types"
)

// ---- regex patterns -------------------------------------------------------

// migrationOpRE matches `migrations.AddField(`, `migrations.RemoveField(` etc.
// inside the operations list.  Capture group 1 = operation class name.
var migrationOpRE = regexp.MustCompile(
	`\bmigrations\.([A-Za-z][A-Za-z0-9_]*)\s*\(`,
)

// migrationModelNameRE captures the model_name kwarg value.
var migrationModelNameRE = regexp.MustCompile(
	`model_name\s*=\s*["']([^"']+)["']`,
)

// migrationFieldNameRE captures the standalone `name=` kwarg value (field
// name).  Uses `\bname\b` anchored so it does NOT match `model_name=`;
// RE2 lacks lookbehind so we match the literal token boundary via a
// non-capturing alternative that requires name to start after a comma,
// open-paren, or whitespace — which is always the case for Python kwargs.
var migrationFieldNameRE = regexp.MustCompile(
	`(?:^|[\s,\(])name\s*=\s*["']([^"']+)["']`,
)

// migrationDepsRE captures dependency tuples: ("app", "migration_name").
var migrationDepsRE = regexp.MustCompile(
	`\(\s*["']([^"']+)["']\s*,\s*["']([^"']+)["']\s*\)`,
)

// dependenciesBlock finds the dependencies list assignment so we only
// match dependency tuples within that block.
var dependenciesBlockRE = regexp.MustCompile(
	`(?s)dependencies\s*=\s*\[(.*?)\]`,
)

// operationsBlock finds the operations list assignment so we only match
// operation calls within that block.
var operationsBlockRE = regexp.MustCompile(
	`(?s)operations\s*=\s*\[(.*?)\](?:\s*\n\s*[^\s#])`,
)

// ---- public API -----------------------------------------------------------

// migrationOp holds the extracted metadata for a single Django migration
// operation, ready for serialisation into the entity's properties.
type migrationOp struct {
	Type  string `json:"type"`
	Model string `json:"model,omitempty"`
	Field string `json:"field,omitempty"`
}

// extractMigrationEntity parses a Django migration source file (already
// confirmed to be under a migrations/ directory) and returns one
// kind="Migration" EntityRecord with operation metadata encoded in its
// Properties.  The file-level SCOPE.Component/file entity is emitted by the
// caller and is NOT duplicated here.
func extractMigrationEntity(file djangoMigrationFile) types.EntityRecord {
	src := file.source
	name := migrationFileName(file.path)

	ops := parseMigrationOps(src)
	deps := parseMigrationDeps(src)

	props := map[string]string{
		"op_count":     strconv.Itoa(len(ops)),
		"dependencies": deps,
		"operations":   encodeMigrationOps(ops),
	}

	qn := ""
	if mod := filePathToModule(file.path); mod != "" {
		qn = mod
	}

	return types.EntityRecord{
		Name:             name,
		QualifiedName:    qn,
		Kind:             "Migration",
		Subtype:          "django",
		SourceFile:       file.path,
		Language:         file.language,
		StartLine:        1,
		Properties:       props,
		QualityScore:     0.8,
		EnrichmentStatus: types.StatusPending,
	}
}

// djangoMigrationFile is the minimal context extractMigrationEntity needs.
type djangoMigrationFile struct {
	path     string
	language string
	source   string
}

// ---- helpers --------------------------------------------------------------

// migrationFileName returns the migration's identifier: the base filename
// with the .py extension stripped.
// "core/migrations/0042_device_serial_number.py" → "0042_device_serial_number"
func migrationFileName(path string) string {
	base := filepath.Base(filepath.FromSlash(path))
	return strings.TrimSuffix(base, ".py")
}

// parseMigrationOps extracts operations from the operations list.
// It first isolates the operations block so surrounding class attributes
// (e.g. dependencies) don't produce false positives.
func parseMigrationOps(src string) []migrationOp {
	block := extractOperationsBlock(src)
	if block == "" {
		// Fallback: scan the whole file — better than missing ops entirely.
		block = src
	}

	matches := migrationOpRE.FindAllStringSubmatchIndex(block, -1)
	ops := make([]migrationOp, 0, len(matches))
	for _, m := range matches {
		opType := block[m[2]:m[3]]
		// Capture the argument window for this call (next ~400 bytes).
		winStart := m[0]
		winEnd := winStart + 400
		if winEnd > len(block) {
			winEnd = len(block)
		}
		window := block[winStart:winEnd]

		op := migrationOp{Type: opType}
		if mm := migrationModelNameRE.FindStringSubmatch(window); mm != nil {
			op.Model = mm[1]
		}
		if mm := migrationFieldNameRE.FindStringSubmatch(window); mm != nil {
			op.Field = mm[1]
		}
		ops = append(ops, op)
	}
	return ops
}

// extractOperationsBlock isolates the content of the operations list.
// Returns "" when the block cannot be found.
func extractOperationsBlock(src string) string {
	m := operationsBlockRE.FindStringSubmatch(src)
	if m == nil {
		// Try a looser match that accepts EOF after the closing bracket.
		loose := regexp.MustCompile(`(?s)operations\s*=\s*\[(.*?)\]\s*$`)
		m = loose.FindStringSubmatch(src)
	}
	if m == nil {
		return ""
	}
	return m[1]
}

// parseMigrationDeps extracts dependency pairs from the dependencies list.
// Returns a comma-separated string of "app/migration" identifiers.
func parseMigrationDeps(src string) string {
	m := dependenciesBlockRE.FindStringSubmatch(src)
	if m == nil {
		return ""
	}
	block := m[1]
	matches := migrationDepsRE.FindAllStringSubmatch(block, -1)
	if len(matches) == 0 {
		return ""
	}
	parts := make([]string, 0, len(matches))
	for _, mm := range matches {
		parts = append(parts, mm[1]+"/"+mm[2])
	}
	return strings.Join(parts, ",")
}

// encodeMigrationOps serialises the operation list as a compact JSON array.
// Uses a hand-rolled encoder to avoid importing encoding/json (which would
// add unnecessary complexity to this self-contained helper).
func encodeMigrationOps(ops []migrationOp) string {
	if len(ops) == 0 {
		return "[]"
	}
	var sb strings.Builder
	sb.WriteByte('[')
	for i, op := range ops {
		if i > 0 {
			sb.WriteByte(',')
		}
		sb.WriteString(`{"type":`)
		sb.WriteString(jsonString(op.Type))
		if op.Model != "" {
			sb.WriteString(`,"model":`)
			sb.WriteString(jsonString(op.Model))
		}
		if op.Field != "" {
			sb.WriteString(`,"field":`)
			sb.WriteString(jsonString(op.Field))
		}
		sb.WriteByte('}')
	}
	sb.WriteByte(']')
	return sb.String()
}

// jsonString encodes s as a JSON string literal (double-quoted, backslash-
// escaped). Only handles ASCII printable characters safely; non-ASCII is
// left as-is which is valid UTF-8 JSON.
func jsonString(s string) string {
	return fmt.Sprintf("%q", s)
}
