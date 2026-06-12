package rust

// seaorm.go — custom extractor for the SeaORM async ORM (Rust).
//
// Detects and emits entities for:
//
//   - #[derive(Clone, Debug, PartialEq, DeriveEntityModel)] + #[sea_orm(table_name = "...")] →
//     SCOPE.Component (subtype="orm_model")
//   - DeriveRelation enum variants with #[sea_orm(has_many / belongs_to = "...")] →
//     SCOPE.Pattern (subtype="orm_relationship")
//   - sea-orm-migration MigrationTrait impl blocks → SCOPE.Component (subtype="migration")
//   - Foreign-key columns detected via #[sea_orm(belongs_to)] with from/to column refs →
//     SCOPE.Pattern (subtype="foreign_key")   [foreign_key_extraction]
//   - find_related() / find_linked() / LoaderTrait usage →
//     SCOPE.Pattern (subtype="lazy_load")     [lazy_loading_recognition]
//
// Honesty:
//
//	partial — heuristic regex match on source text. Does NOT perform
//	full semantic analysis, import-resolution, or macro expansion.
//	Fixtures prove the detection surface; full cross-entity linking
//	requires import-graph analysis beyond this scanner.
//
// Issue #3267 — lang.rust.orm.seaorm Relationships cells.

import (
	"context"
	"regexp"
	"strings"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/trace"

	"github.com/cajasmota/archigraph/internal/extractor"
	"github.com/cajasmota/archigraph/internal/types"
)

func init() {
	extractor.Register("custom_rust_seaorm", &rustSeaORMExtractor{})
}

type rustSeaORMExtractor struct{}

func (e *rustSeaORMExtractor) Language() string { return "custom_rust_seaorm" }

// ---------------------------------------------------------------------------
// Regex catalog
// ---------------------------------------------------------------------------

var (
	// #[derive(...DeriveEntityModel...)]
	reSeaOrmEntityDerive = regexp.MustCompile(
		`#\[derive\([^)]*\bDeriveEntityModel\b[^)]*\)\]`,
	)

	// #[sea_orm(table_name = "users")]
	reSeaOrmTableName = regexp.MustCompile(
		`#\[sea_orm\([^)]*table_name\s*=\s*"([^"]+)"[^)]*\)\]`,
	)

	// Entity Model struct name (pub struct Model)
	reSeaOrmModel = regexp.MustCompile(`\bpub\s+struct\s+(\w+)`)

	// DeriveRelation enum
	reSeaOrmRelationEnum = regexp.MustCompile(
		`#\[derive\([^)]*\bDeriveRelation\b[^)]*\)\]\s*(?:pub\s+)?enum\s+(\w+)`,
	)

	// Relation variant annotations:
	// #[sea_orm(has_many = "super::post::Entity")]
	// #[sea_orm(belongs_to = "super::user::Entity", from = "Column::UserId", to = "super::user::Column::Id")]
	reSeaOrmRelationAttr = regexp.MustCompile(
		`#\[sea_orm\([^)]*\b(has_many|belongs_to|has_one)\s*=\s*"([^"]+)"[^)]*\)\]`,
	)

	// sea-orm-migration: impl MigrationTrait for MigrationName
	reSeaOrmMigration = regexp.MustCompile(
		`\bimpl\s+MigrationTrait\s+for\s+(\w+)`,
	)

	// MigrationName::name impl returning the migration identifier string.
	// fn name(&self) -> &str { "m20220101_000001_create_users_table" }
	reSeaOrmMigrationName = regexp.MustCompile(
		`fn\s+name\s*\(\s*&self\s*\)\s*->\s*&\s*str\s*\{\s*"([^"]+)"`,
	)

	// migration_schema_ops — schema-builder calls inside up()/down() bodies:
	//   manager.create_table( Table::create().table(Users::Table) ... )
	//   manager.alter_table(  Table::alter().table(Users::Table)  ... )
	//   manager.drop_table(   Table::drop().table(Users::Table)   ... )
	//   manager.create_index( Index::create().table(Users::Table) ... )
	//   manager.drop_index(   Index::drop().name("idx_...")        ... )
	// We anchor on the manager.<op>( call and then resolve the .table(<Iden>)
	// or .name("<idx>") that names the affected object. The Iden enum's
	// `::Table` variant maps the Rust enum name → logical table name.
	reSeaOrmManagerOp = regexp.MustCompile(
		`\bmanager\s*\.\s*(create_table|alter_table|drop_table|truncate_table|rename_table|create_index|drop_index)\s*\(`,
	)

	// .table(Users::Table) — names the target table via its Iden enum.
	reSeaOrmTableIden = regexp.MustCompile(
		`\.\s*table\s*\(\s*(\w+)\s*::\s*Table\s*\)`,
	)

	// .name("idx_users_name") — names an index target.
	reSeaOrmIndexName = regexp.MustCompile(
		`\.\s*name\s*\(\s*"([^"]+)"\s*\)`,
	)

	// ColumnDef::new(Users::Id) — a column definition inside a create/alter op.
	// Captures the optional Iden enum and the column variant.
	reSeaOrmColumnDef = regexp.MustCompile(
		`ColumnDef::new\s*\(\s*(?:(\w+)\s*::\s*)?(\w+)\s*\)`,
	)

	// belongs_to with from/to column references → FK extraction
	// #[sea_orm(belongs_to = "...", from = "Column::FieldId", to = "super::user::Column::Id")]
	reSeaOrmBelongsToFK = regexp.MustCompile(
		`#\[sea_orm\([^)]*\bbelongs_to\s*=\s*"([^"]+)"[^)]*\bfrom\s*=\s*"([^"]+)"[^)]*\bto\s*=\s*"([^"]+)"`,
	)

	// find_related(T) / find_linked(T) — lazy/eager load signals
	// Note: these take a type argument, so do NOT require empty parens.
	reSeaOrmFindRelated = regexp.MustCompile(
		`\.find_related\s*\(|\.find_linked\s*\(`,
	)

	// LoaderTrait usage — batch loading (lazy loading pattern)
	reSeaOrmLoaderTrait = regexp.MustCompile(
		`LoaderTrait|load_many\s*\(|load_one\s*\(|load_many_to_many\s*\(`,
	)

	// impl Related<T> for Entity — relation type implementation
	reSeaOrmRelated = regexp.MustCompile(
		`\bimpl\s+(?:<[^>]*>\s+)?Related\s*<\s*([^>]+)>\s+for\s+(\w+)`,
	)

	// Struct field inside an Entity Model body: `pub col_name: RustType,`
	// Captures the field name and its Rust type for schema_column emission.
	reSeaOrmField = regexp.MustCompile(
		`(?m)^\s*pub\s+(\w+)\s*:\s*([A-Za-z_][\w:<>, ]*?)\s*,`,
	)
)

// ---------------------------------------------------------------------------
// Extract
// ---------------------------------------------------------------------------

func (e *rustSeaORMExtractor) Extract(ctx context.Context, file extractor.FileInput) ([]types.EntityRecord, error) {
	tracer := otel.Tracer("archigraph/custom/rust")
	_, span := tracer.Start(ctx, "indexer.rust_seaorm_extractor.extract",
		trace.WithAttributes(
			attribute.String("language", file.Language),
			attribute.String("file_path", file.Path),
		),
	)
	defer span.End()

	if len(file.Content) == 0 {
		return nil, nil
	}
	if file.Language != "rust" {
		return nil, nil
	}

	src := string(file.Content)
	var entities []types.EntityRecord
	seen := make(map[string]bool)

	add := func(ent types.EntityRecord) {
		key := ent.Kind + ":" + ent.Subtype + ":" + ent.Name
		if seen[key] {
			return
		}
		seen[key] = true
		entities = append(entities, ent)
	}

	// 1. DeriveEntityModel struct → orm_model entity.
	//    Scan for the derive attribute, then look forward for:
	//    (a) an optional #[sea_orm(table_name = "...")] attribute,
	//    (b) the struct declaration.
	entityDeriveMatches := reSeaOrmEntityDerive.FindAllStringIndex(src, -1)
	for _, dm := range entityDeriveMatches {
		// Look ahead ~600 chars for table_name attr and struct name.
		tail := src[dm[1]:]
		if len(tail) > 600 {
			tail = tail[:600]
		}

		tableName := ""
		if tnMatch := reSeaOrmTableName.FindStringSubmatch(tail); tnMatch != nil {
			tableName = tnMatch[1]
		}

		structMatch := reSeaOrmModel.FindStringSubmatchIndex(tail)
		if structMatch == nil {
			continue
		}
		structName := tail[structMatch[2]:structMatch[3]]
		if structName == "" {
			continue
		}

		modelKey := structName
		if tableName != "" {
			modelKey = tableName
		}

		line := lineOf(src, dm[0])
		ent := makeEntity("seaorm:model:"+modelKey, "SCOPE.Component", "orm_model",
			file.Path, file.Language, line)
		setProps(&ent,
			"framework", "seaorm",
			"struct_name", structName,
			"table_name", tableName,
			"provenance", "INFERRED_FROM_SEAORM_DERIVE_ENTITY_MODEL",
		)
		add(ent)

		// schema_extraction (columns) — parse the Model struct body for
		// `pub field: Type,` declarations and emit a schema_column per field.
		// The body begins at the struct's opening brace within the lookahead.
		colKey := modelKey
		if bodyOpen := strings.Index(tail[structMatch[3]:], "{"); bodyOpen >= 0 {
			bodyStart := structMatch[3] + bodyOpen
			bodyEnd := strings.Index(tail[bodyStart:], "}")
			if bodyEnd < 0 {
				bodyEnd = len(tail) - bodyStart
			}
			body := tail[bodyStart : bodyStart+bodyEnd]
			for _, fm := range reSeaOrmField.FindAllStringSubmatchIndex(body, -1) {
				colName := body[fm[2]:fm[3]]
				colType := strings.TrimSpace(body[fm[4]:fm[5]])
				colEnt := makeEntity("seaorm:column:"+colKey+"."+colName,
					"SCOPE.Component", "schema_column",
					file.Path, file.Language, line)
				setProps(&colEnt,
					"framework", "seaorm",
					"table_name", tableName,
					"column_name", colName,
					"rust_type", colType,
					"provenance", "INFERRED_FROM_SEAORM_MODEL_FIELD",
				)
				add(colEnt)
			}
		}
	}

	// 2. DeriveRelation enum → parse each variant's sea_orm attribute
	//    to emit orm_relationship patterns.
	for _, m := range reSeaOrmRelationEnum.FindAllStringSubmatchIndex(src, -1) {
		enumName := src[m[2]:m[3]]

		// Find the enum body: scan forward for { ... }
		bodyStart := strings.Index(src[m[1]:], "{")
		if bodyStart < 0 {
			continue
		}
		bodyStart += m[1]
		// Find matching closing brace (shallow: assume no nested braces in enum body).
		bodyEnd := strings.Index(src[bodyStart:], "}")
		if bodyEnd < 0 {
			continue
		}
		bodyEnd += bodyStart
		body := src[bodyStart : bodyEnd+1]

		// Within the enum body, find each sea_orm relation attribute.
		for _, rm := range reSeaOrmRelationAttr.FindAllStringSubmatchIndex(body, -1) {
			relType := body[rm[2]:rm[3]]      // has_many | belongs_to | has_one
			targetEntity := body[rm[4]:rm[5]] // e.g. "super::post::Entity"
			// Extract the short entity name (last segment before ::Entity or ::Model).
			targetShort := targetEntity
			if idx := strings.LastIndex(targetEntity, "::"); idx >= 0 {
				targetShort = targetEntity[idx+2:]
			}
			name := "seaorm:relation:" + enumName + ":" + relType + ":" + targetShort
			ent := makeEntity(name, "SCOPE.Pattern", "orm_relationship",
				file.Path, file.Language, lineOf(src, bodyStart+rm[0]))
			setProps(&ent,
				"framework", "seaorm",
				"enum_name", enumName,
				"relation_type", relType,
				"target_entity", targetEntity,
				"provenance", "INFERRED_FROM_SEAORM_DERIVE_RELATION",
			)
			add(ent)
		}
	}

	// 3. impl MigrationTrait for M → migration entity
	for _, m := range reSeaOrmMigration.FindAllStringSubmatchIndex(src, -1) {
		migName := src[m[2]:m[3]]
		// migration_parsing — resolve the human migration id from the
		// adjacent MigrationName::name impl when present.
		migID := ""
		if nm := reSeaOrmMigrationName.FindStringSubmatch(src); nm != nil {
			migID = nm[1]
		}
		ent := makeEntity("seaorm:migration:"+migName, "SCOPE.Component", "migration",
			file.Path, file.Language, lineOf(src, m[0]))
		props := []string{
			"framework", "seaorm",
			"migration_name", migName,
			"provenance", "INFERRED_FROM_SEAORM_MIGRATION_TRAIT",
		}
		if migID != "" {
			props = append(props, "migration_id", migID)
		}
		setProps(&ent, props...)
		add(ent)
	}

	// 3b. migration_schema_ops — parse schema-builder calls in up()/down()
	//     bodies. Each manager.<op>( ... ) call becomes a migration component
	//     carrying migration_op + the resolved target table/index. For
	//     create/alter ops we additionally emit a schema_column per
	//     ColumnDef::new(...) found inside the call's balanced argument span.
	for _, m := range reSeaOrmManagerOp.FindAllStringSubmatchIndex(src, -1) {
		op := src[m[2]:m[3]]
		// The opening paren of the manager.<op>( call is the last byte of the
		// match. Find its balanced close to bound the call argument.
		openParen := m[1] - 1
		argSpan := src[m[1]:seaOrmBalancedClose(src, openParen)]

		// Resolve the target object: a .table(<Iden>::Table) reference, or an
		// index .name("...") for create_index/drop_index.
		target := ""
		targetKind := "table"
		if tm := reSeaOrmTableIden.FindStringSubmatch(argSpan); tm != nil {
			target = tm[1]
		} else if strings.Contains(op, "index") {
			if im := reSeaOrmIndexName.FindStringSubmatch(argSpan); im != nil {
				target = im[1]
				targetKind = "index"
			}
		}

		name := "seaorm:migration:" + op
		if target != "" {
			name += ":" + target
		}
		opEnt := makeEntity(name, "SCOPE.Component", "migration",
			file.Path, file.Language, lineOf(src, m[0]))
		opProps := []string{
			"framework", "seaorm",
			"migration_op", op,
			"provenance", "INFERRED_FROM_SEAORM_SCHEMA_MANAGER_OP",
		}
		if target != "" {
			if targetKind == "index" {
				opProps = append(opProps, "index_name", target)
			} else {
				opProps = append(opProps, "table_name", target)
			}
		}
		setProps(&opEnt, opProps...)
		add(opEnt)

		// Columns are only meaningful for create/alter table ops.
		if op == "create_table" || op == "alter_table" {
			for _, cm := range reSeaOrmColumnDef.FindAllStringSubmatchIndex(argSpan, -1) {
				colName := argSpan[cm[4]:cm[5]]
				// Skip the table marker variant if it ever appears here.
				if colName == "Table" {
					continue
				}
				colKey := colName
				if target != "" {
					colKey = target + "." + colName
				}
				colEnt := makeEntity("seaorm:migration:column:"+colKey,
					"SCOPE.Component", "schema_column",
					file.Path, file.Language, lineOf(src, m[0]))
				colProps := []string{
					"framework", "seaorm",
					"column_name", colName,
					"migration_op", op,
					"provenance", "INFERRED_FROM_SEAORM_COLUMN_DEF",
				}
				if target != "" {
					colProps = append(colProps, "table_name", target)
				}
				setProps(&colEnt, colProps...)
				add(colEnt)
			}
		}
	}

	// 4. foreign_key_extraction — belongs_to with explicit from/to column refs
	for _, m := range reSeaOrmBelongsToFK.FindAllStringSubmatchIndex(src, -1) {
		targetEntity := src[m[2]:m[3]]
		fromCol := src[m[4]:m[5]]
		toCol := src[m[6]:m[7]]
		// Shorten target entity path to last segment
		targetShort := targetEntity
		if idx := strings.LastIndex(targetEntity, "::"); idx >= 0 {
			targetShort = targetEntity[idx+2:]
		}
		name := "seaorm:fk:" + fromCol + "->" + targetShort
		ent := makeEntity(name, "SCOPE.Pattern", "foreign_key",
			file.Path, file.Language, lineOf(src, m[0]))
		setProps(&ent,
			"framework", "seaorm",
			"target_entity", targetEntity,
			"from_column", fromCol,
			"to_column", toCol,
			"provenance", "INFERRED_FROM_SEAORM_BELONGS_TO_FK",
		)
		add(ent)
	}

	// 5. impl Related<T> for Entity → relationship implementation
	for _, m := range reSeaOrmRelated.FindAllStringSubmatchIndex(src, -1) {
		targetType := strings.TrimSpace(src[m[2]:m[3]])
		entityType := src[m[4]:m[5]]
		name := "seaorm:related:" + entityType + "->" + targetType
		ent := makeEntity(name, "SCOPE.Pattern", "orm_relationship",
			file.Path, file.Language, lineOf(src, m[0]))
		setProps(&ent,
			"framework", "seaorm",
			"entity_type", entityType,
			"related_to", targetType,
			"provenance", "INFERRED_FROM_SEAORM_IMPL_RELATED",
		)
		add(ent)
	}

	// 6. lazy_loading_recognition — find_related() / find_linked()
	for _, m := range reSeaOrmFindRelated.FindAllStringIndex(src, -1) {
		ent := makeEntity("seaorm:find_related", "SCOPE.Pattern", "lazy_load",
			file.Path, file.Language, lineOf(src, m[0]))
		setProps(&ent,
			"framework", "seaorm",
			"provenance", "INFERRED_FROM_SEAORM_FIND_RELATED",
		)
		add(ent)
	}

	// 7. lazy_loading_recognition — LoaderTrait batch loading
	for _, m := range reSeaOrmLoaderTrait.FindAllStringIndex(src, -1) {
		ent := makeEntity("seaorm:loader_trait", "SCOPE.Pattern", "lazy_load",
			file.Path, file.Language, lineOf(src, m[0]))
		setProps(&ent,
			"framework", "seaorm",
			"provenance", "INFERRED_FROM_SEAORM_LOADER_TRAIT",
		)
		add(ent)
	}

	span.SetAttributes(attribute.Int("entity_count", len(entities)))
	return entities, nil
}

// seaOrmBalancedClose returns the index just past the byte at openIdx (which
// must be an opening '(') up to and including its matching ')'. If the parens
// are unbalanced (truncated source), it returns len(src). Parens inside string
// literals are ignored. The returned index is suitable as an exclusive upper
// bound: src[openIdx+1:seaOrmBalancedClose(src, openIdx)] is the call argument.
func seaOrmBalancedClose(src string, openIdx int) int {
	depth := 0
	inStr := false
	for i := openIdx; i < len(src); i++ {
		c := src[i]
		if inStr {
			switch c {
			case '\\':
				i++ // skip escaped char
			case '"':
				inStr = false
			}
			continue
		}
		switch c {
		case '"':
			inStr = true
		case '(':
			depth++
		case ')':
			depth--
			if depth == 0 {
				return i
			}
		}
	}
	return len(src)
}
