// norm_migrations.go — Nim Norm schema-migration ops (#4991, follow-up to #4932).
//
// norm_orm.go covers the model→table/column schema synthesis. Norm does NOT
// have a declarative migration DSL like Allographer's schema().alter(...); its
// schema is created and evolved imperatively against a `DbConn` handle:
//
//	import norm/[model, sqlite]
//
//	let db = open(":memory:", "", "", "")
//	db.createTables(User())            # CREATE TABLE for the User model
//	db.createTables(Post())
//	db.dropTables(Post())              # DROP TABLE for the Post model
//	db.exec(sql"CREATE TABLE audit (id INTEGER PRIMARY KEY)")   # raw DDL
//	db.exec(sql"ALTER TABLE user ADD COLUMN bio TEXT")
//	db.exec(sql"DROP TABLE audit")
//
// Two migration shapes are recognised:
//
//   - model-typed schema ops — `<db>.createTables(Model())` /
//     `<db>.dropTables(Model())` (also the bare `createTables(Model())` form).
//     The op targets the MODEL NAME; the shared resolver binds the model name to
//     its table convergence node (same indirection the Crystal Granite
//     `<Model>.migrator.create` path uses), so a {.tableName.} override on the
//     model is honoured downstream.
//   - raw-DDL schema ops — a `db.exec(sql"CREATE/DROP/ALTER TABLE <name>")`
//     string carrying a CREATE/DROP/ALTER TABLE statement. The op targets the
//     literal table name parsed out of the SQL.
//
// Each op is a shared SCOPE.Evolution migration-op entity (framework=norm,
// migration_op, table, provenance) — the SAME Kind the JS knex/typeorm and Nim
// Allographer migration extractors emit, so the engine migration-schema-ops pass
// (internal/engine/migration_schema_ops.go, `case "norm"`) derives a
// MODIFIES_TABLE edge op→table convergence node, unifying migration→table
// evolution with query→table access on one logical table.
//
// Honest exclusions:
//   - a `createTables(handle)` where the argument is a variable handle (not a
//     `Model()` constructor) is resolved when the handle was bound to a model
//     constructor earlier in the file (var/let binding); a truly dynamic handle
//     is skipped (no fabricated op).
//   - raw DDL whose table name is itself an interpolated/dynamic expression is
//     skipped.
//
// Registration key: "custom_nim_norm_migrations".
package nim

import (
	"context"
	"regexp"
	"strings"

	"github.com/cajasmota/grafel/internal/extractor"
	"github.com/cajasmota/grafel/internal/types"
)

func init() {
	extractor.Register("custom_nim_norm_migrations", &nimNormMigrationsExtractor{})
}

type nimNormMigrationsExtractor struct{}

func (e *nimNormMigrationsExtractor) Language() string { return "custom_nim_norm_migrations" }

var (
	// nimNormCreateTablesRe matches a `<db>.createTables(Model())` /
	// `<db>.dropTables(Model())` call, plus the receiver-less
	// `createTables(Model())` form. Group 1 = the op verb (createTables|
	// dropTables), group 2 = the first-argument identifier (a model constructor
	// `User()` is captured as `User`, or a variable handle).
	nimNormCreateTablesRe = regexp.MustCompile(
		`(?m)\b(?:[A-Za-z_][A-Za-z0-9_]*\s*\.\s*)?(createTables|dropTables)\s*\(\s*([A-Za-z_][A-Za-z0-9_]*)`)

	// nimNormRawDDLRe matches a raw schema-op SQL string passed to `db.exec(sql"…")`
	// (or `exec("…")`). Group 1 = the op keyword phrase, group 2 = the table name.
	nimNormRawDDLRe = regexp.MustCompile(
		`(?is)\.\s*exec\s*\(\s*sql?\s*["` + "`" + `]\s*(CREATE\s+TABLE(?:\s+IF\s+NOT\s+EXISTS)?|DROP\s+TABLE(?:\s+IF\s+EXISTS)?|ALTER\s+TABLE)\s+["` + "`" + `]?([A-Za-z_][A-Za-z0-9_]*)`)

	// nimNormHandleBindRe binds a `var/let x = Model()` handle to its model type so
	// a `createTables(x)` referencing the handle resolves to the model. Group 1 =
	// the handle identifier, group 2 = the model constructor type name.
	nimNormHandleBindRe = regexp.MustCompile(
		`(?m)\b(?:let|var|const)\s+([A-Za-z_][A-Za-z0-9_]*)\s*(?::[^\n=]+)?=\s*([A-Z][A-Za-z0-9_]*)\s*\(`)
)

// nimNormHasMigration is a fast pre-filter: the file must reference a Norm
// schema op (createTables/dropTables, or an exec(sql"…CREATE/DROP/ALTER TABLE")).
func nimNormHasMigration(content string) bool {
	if !strings.Contains(content, "norm") && !strings.Contains(content, "Model") {
		return false
	}
	if strings.Contains(content, "createTables") || strings.Contains(content, "dropTables") {
		return true
	}
	return strings.Contains(content, ".exec(") &&
		(strings.Contains(content, "TABLE") || strings.Contains(content, "table"))
}

func (e *nimNormMigrationsExtractor) Extract(
	ctx context.Context,
	file extractor.FileInput,
) ([]types.EntityRecord, error) {
	if len(file.Content) == 0 || file.Language != "nim" {
		return nil, nil
	}
	src := string(file.Content)
	if !nimNormHasMigration(src) {
		return nil, nil
	}

	// handles maps a `let/var x = Model()` identifier to its model type so a
	// `createTables(x)` resolves to the model; a Model() constructor referencing
	// itself (x == its own type) is harmless.
	handles := make(map[string]string)
	for _, m := range nimNormHandleBindRe.FindAllStringSubmatch(src, -1) {
		handles[m[1]] = m[2]
	}

	var out []types.EntityRecord
	seen := make(map[string]bool)
	emit := func(op, table string, line int) {
		if table == "" {
			return
		}
		name := op + ":" + table
		if seen[name] {
			return
		}
		seen[name] = true
		ent := types.EntityRecord{
			Name:       name,
			Kind:       "SCOPE.Evolution",
			Subtype:    op,
			SourceFile: file.Path,
			Language:   "nim",
			StartLine:  line,
			EndLine:    line,
			Properties: map[string]string{
				"framework":    "norm",
				"migration_op": op,
				"table":        table,
				"provenance":   "INFERRED_FROM_NORM_MIGRATION",
			},
		}
		ent.ID = ent.ComputeID()
		out = append(out, ent)
	}

	// --- model-typed createTables/dropTables ---------------------------------
	for _, m := range nimNormCreateTablesRe.FindAllStringSubmatchIndex(src, -1) {
		verb := src[m[2]:m[3]]
		arg := src[m[4]:m[5]]
		// A bare `Model()` constructor (uppercase) is the model directly; a
		// lowercase handle is resolved via its var/let binding.
		target := ""
		if arg != "" && arg[0] >= 'A' && arg[0] <= 'Z' {
			target = arg // Model() constructor
		} else if mt, ok := handles[arg]; ok {
			target = mt // resolved variable handle
		}
		if target == "" {
			continue // truly dynamic handle — no fabricated op
		}
		op := "create_table"
		if verb == "dropTables" {
			op = "drop_table"
		}
		emit(op, target, nimLineOf(src, m[0]))
	}

	// --- raw DDL: db.exec(sql"CREATE/DROP/ALTER TABLE <name>") ----------------
	for _, m := range nimNormRawDDLRe.FindAllStringSubmatchIndex(src, -1) {
		kw := strings.ToUpper(strings.Fields(src[m[2]:m[3]])[0])
		table := src[m[4]:m[5]]
		op := "alter_table"
		switch kw {
		case "CREATE":
			op = "create_table"
		case "DROP":
			op = "drop_table"
		}
		emit(op, table, nimLineOf(src, m[0]))
	}

	return out, nil
}
