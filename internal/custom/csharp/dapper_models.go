// Package csharp — ORM model extractor for Dapper, LINQ-to-SQL, LinqToDB,
// and NHibernate/FluentNHibernate C# source files.
//
// Patterns extracted:
//
//	Dapper (POCO + attribute annotations):
//	  - POCO class with [Table("name")] / [Column("name")] → Models/schema
//	  - sql.Query<T>(...) / sql.Execute(...) → query_attribution
//
//	LINQ-to-SQL / LinqToDB:
//	  - [Table] / [Table(Name="...")] class attribute → Models
//	  - [Column] / [Column(Name="...")] property attribute → schema
//	  - [Association(...)] property attribute → relationship_extraction
//
//	NHibernate / FluentNHibernate:
//	  - ClassMap<T> subclass (FluentNHibernate mapping) → Models/schema
//	  - Map(x => x.Prop) / References(x => x.Nav) fluent calls → relationships
//	  - ISession.Query<T> / ISession.Get<T> → query_attribution
//
// Emitted entity kinds:
//
//	SCOPE.Component   — model/table/mapping containers
//	SCOPE.Operation   — queries
//	SCOPE.Pattern     — attribute-annotated schema, relationship markers
package csharp

import (
	"context"
	"regexp"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/trace"

	"github.com/cajasmota/archigraph/internal/extractor"
	"github.com/cajasmota/archigraph/internal/types"
)

func init() {
	extractor.Register("custom_csharp_orm_models", &ormModelsExtractor{})
}

type ormModelsExtractor struct{}

func (e *ormModelsExtractor) Language() string { return "custom_csharp_orm_models" }

// ---------------------------------------------------------------------------
// Regexes — Dapper
// ---------------------------------------------------------------------------

var (
	// [Table("users")] or [Table] on a class
	reDapperTable = regexp.MustCompile(
		`\[Table(?:\s*\(\s*(?:Name\s*=\s*)?["']([^"']+)["']\s*\))?\s*\]`,
	)
	// [Column("col_name")] or [Column] on a property
	reDapperColumn = regexp.MustCompile(
		`\[Column(?:\s*\(\s*(?:Name\s*=\s*)?["']([^"']+)["']\s*\))?\s*\]`,
	)
	// Dapper query: conn.Query<T>("sql") / conn.QueryAsync<T>("sql") / db.Execute("sql")
	reDapperQuery = regexp.MustCompile(
		`\.(?:Query|QueryAsync|QueryFirst|QueryFirstOrDefault|QuerySingle|QuerySingleOrDefault|Execute|ExecuteAsync|ExecuteScalar)\s*(?:<\s*(\w+)\s*>)?\s*\(`,
	)
	// POCO class with [Table] — class declaration following the attribute
	reDapperClass = regexp.MustCompile(
		`(?m)class\s+(\w+)\s*(?::\s*[\w\s,<>]+)?\s*\{`,
	)
)

// ---------------------------------------------------------------------------
// Regexes — LINQ-to-SQL / LinqToDB
// ---------------------------------------------------------------------------

var (
	// [Table] or [Table(Name="tableName")] — shared with Dapper but presence
	// of LinqToDB / L2S namespace usage discriminates the framework.
	// We re-use reDapperTable above and tag by framework via caller context.

	// [Association(ThisKey="...", OtherKey="...")] property attribute
	reLinqAssociation = regexp.MustCompile(
		`\[Association\s*\([^)]*\)\s*\]`,
	)
	// DataContext / DataConnection subclass (L2S / LinqToDB context)
	reLinqContext = regexp.MustCompile(
		`(?m)class\s+(\w+)\s*:\s*(?:DataContext|DataConnection)\b`,
	)
	// Table<T> property on a DataContext (L2S table declaration)
	reLinqTable = regexp.MustCompile(
		`Table\s*<\s*(\w+)\s*>`,
	)
)

// ---------------------------------------------------------------------------
// Regexes — NHibernate / FluentNHibernate
// ---------------------------------------------------------------------------

var (
	// class MyMap : ClassMap<Entity>
	reNHClassMap = regexp.MustCompile(
		`(?m)class\s+(\w+)\s*:\s*ClassMap\s*<\s*(\w+)\s*>`,
	)
	// Map(x => x.PropertyName) — column mapping in ClassMap
	reNHMap = regexp.MustCompile(
		`\.Map\s*\(\s*x\s*=>\s*x\.(\w+)`,
	)
	// References(x => x.Nav) — many-to-one relationship (bare call or chained)
	reNHReferences = regexp.MustCompile(
		`(?:^|\.|\s)References\s*\(\s*x\s*=>\s*x\.(\w+)`,
	)
	// HasMany(x => x.Col) — one-to-many relationship (bare call or chained)
	reNHHasMany = regexp.MustCompile(
		`(?:^|\.|\s)HasMany\s*\(\s*x\s*=>\s*x\.(\w+)`,
	)
	// session.Query<T>() / session.Get<T>()
	reNHQuery = regexp.MustCompile(
		`\.(?:Query|Get|Load|Find|QueryOver)\s*<\s*(\w+)\s*>\s*\(`,
	)
	// ISession usage marker — presence means NHibernate
	reNHSession = regexp.MustCompile(
		`\bISession\b`,
	)
)

// ---------------------------------------------------------------------------
// framework detection helpers
// ---------------------------------------------------------------------------

var (
	reDapperNS     = regexp.MustCompile(`using\s+Dapper\b`)
	reLinqToSQLNS  = regexp.MustCompile(`using\s+System\.Data\.Linq\b`)
	reLinqToDBNS   = regexp.MustCompile(`using\s+LinqToDB\b`)
	reNHibernateNS = regexp.MustCompile(`using\s+(?:NHibernate|FluentNHibernate)\b`)
)

// ---------------------------------------------------------------------------
// Extract
// ---------------------------------------------------------------------------

func (e *ormModelsExtractor) Extract(ctx context.Context, file extractor.FileInput) ([]types.EntityRecord, error) {
	tracer := otel.Tracer("archigraph/custom/csharp")
	_, span := tracer.Start(ctx, "indexer.csharp_orm_models_extractor.extract",
		trace.WithAttributes(
			attribute.String("language", file.Language),
			attribute.String("file_path", file.Path),
		),
	)
	defer span.End()

	if len(file.Content) == 0 {
		return nil, nil
	}
	if file.Language != "csharp" {
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

	// Detect which ORM namespaces are present (controls framework tagging).
	isDapper := reDapperNS.MatchString(src)
	isLinqToSQL := reLinqToSQLNS.MatchString(src)
	isLinqToDB := reLinqToDBNS.MatchString(src)
	isNH := reNHibernateNS.MatchString(src) || reNHClassMap.MatchString(src) || reNHSession.MatchString(src)

	// -------------------------------------------------------------------------
	// Dapper POCO models
	// -------------------------------------------------------------------------

	if isDapper || reDapperTable.MatchString(src) {
		// [Table] attribute → model_extraction
		for _, m := range reDapperTable.FindAllStringSubmatchIndex(src, -1) {
			tableName := ""
			if m[2] >= 0 {
				tableName = src[m[2]:m[3]]
			}
			name := "dapper:table:" + tableName
			ent := makeEntity(name, "SCOPE.Component", "model_extraction", file.Path, "csharp", lineOf(src, m[0]))
			setProps(&ent, "framework", "dapper", "provenance", "INFERRED_FROM_DAPPER_TABLE",
				"table_name", tableName)
			add(ent)
		}

		// [Column] attribute → schema_extraction
		for _, m := range reDapperColumn.FindAllStringSubmatchIndex(src, -1) {
			colName := ""
			if m[2] >= 0 {
				colName = src[m[2]:m[3]]
			}
			name := "dapper:column:" + colName + ":" + file.Path + ":" + itoa(lineOf(src, m[0]))
			ent := makeEntity(name, "SCOPE.Pattern", "schema_extraction", file.Path, "csharp", lineOf(src, m[0]))
			setProps(&ent, "framework", "dapper", "provenance", "INFERRED_FROM_DAPPER_COLUMN",
				"column_name", colName)
			add(ent)
		}

		// POCO class declarations in files with [Table] → model_extraction
		if reDapperTable.MatchString(src) {
			for _, m := range reDapperClass.FindAllStringSubmatchIndex(src, -1) {
				name := src[m[2]:m[3]]
				if csharpPrimitives[name] {
					continue
				}
				ent := makeEntity("dapper:poco:"+name, "SCOPE.Component", "model_extraction", file.Path, "csharp", lineOf(src, m[0]))
				setProps(&ent, "framework", "dapper", "provenance", "INFERRED_FROM_DAPPER_POCO")
				add(ent)
			}
		}
	}

	// Dapper query calls → query_attribution
	if isDapper || reDapperQuery.MatchString(src) {
		for _, m := range reDapperQuery.FindAllStringSubmatchIndex(src, -1) {
			entityType := ""
			if m[2] >= 0 {
				entityType = src[m[2]:m[3]]
			}
			name := "dapper:query:" + entityType + ":" + file.Path + ":" + itoa(lineOf(src, m[0]))
			ent := makeEntity(name, "SCOPE.Operation", "query_attribution", file.Path, "csharp", lineOf(src, m[0]))
			setProps(&ent, "framework", "dapper", "provenance", "INFERRED_FROM_DAPPER_QUERY",
				"entity_type", entityType)
			add(ent)
		}
	}

	// -------------------------------------------------------------------------
	// LINQ-to-SQL / LinqToDB
	// -------------------------------------------------------------------------

	if isLinqToSQL || isLinqToDB || reLinqContext.MatchString(src) {
		fwName := "linqtodb"
		if isLinqToSQL {
			fwName = "linq-to-sql"
		}

		// DataContext / DataConnection subclass → model_extraction (context)
		for _, m := range reLinqContext.FindAllStringSubmatchIndex(src, -1) {
			name := src[m[2]:m[3]]
			ent := makeEntity(fwName+":context:"+name, "SCOPE.Component", "model_extraction", file.Path, "csharp", lineOf(src, m[0]))
			setProps(&ent, "framework", fwName, "provenance", "INFERRED_FROM_LINQ_CONTEXT")
			add(ent)
		}

		// [Table] attribute on classes → model_extraction
		for _, m := range reDapperTable.FindAllStringSubmatchIndex(src, -1) {
			tableName := ""
			if m[2] >= 0 {
				tableName = src[m[2]:m[3]]
			}
			name := fwName + ":table:" + tableName
			ent := makeEntity(name, "SCOPE.Component", "model_extraction", file.Path, "csharp", lineOf(src, m[0]))
			setProps(&ent, "framework", fwName, "provenance", "INFERRED_FROM_LINQ_TABLE",
				"table_name", tableName)
			add(ent)
		}

		// [Column] attribute → schema_extraction
		for _, m := range reDapperColumn.FindAllStringSubmatchIndex(src, -1) {
			colName := ""
			if m[2] >= 0 {
				colName = src[m[2]:m[3]]
			}
			name := fwName + ":column:" + colName + ":" + file.Path + ":" + itoa(lineOf(src, m[0]))
			ent := makeEntity(name, "SCOPE.Pattern", "schema_extraction", file.Path, "csharp", lineOf(src, m[0]))
			setProps(&ent, "framework", fwName, "provenance", "INFERRED_FROM_LINQ_COLUMN",
				"column_name", colName)
			add(ent)
		}

		// Table<T> property → schema entity
		for _, m := range reLinqTable.FindAllStringSubmatchIndex(src, -1) {
			entityType := src[m[2]:m[3]]
			if csharpPrimitives[entityType] {
				continue
			}
			name := fwName + ":table_prop:" + entityType
			ent := makeEntity(name, "SCOPE.Component", "schema_extraction", file.Path, "csharp", lineOf(src, m[0]))
			setProps(&ent, "framework", fwName, "provenance", "INFERRED_FROM_LINQ_TABLE_PROP",
				"entity_type", entityType)
			add(ent)
		}

		// [Association] attribute → relationship_extraction
		for _, m := range reLinqAssociation.FindAllStringIndex(src, -1) {
			name := fwName + ":association:" + file.Path + ":" + itoa(lineOf(src, m[0]))
			ent := makeEntity(name, "SCOPE.Pattern", "relationship_extraction", file.Path, "csharp", lineOf(src, m[0]))
			setProps(&ent, "framework", fwName, "provenance", "INFERRED_FROM_LINQ_ASSOCIATION")
			add(ent)
		}
	}

	// -------------------------------------------------------------------------
	// NHibernate / FluentNHibernate
	// -------------------------------------------------------------------------

	if isNH {
		// ClassMap<Entity> subclass → Models / schema
		for _, m := range reNHClassMap.FindAllStringSubmatchIndex(src, -1) {
			mapClass := src[m[2]:m[3]]
			entityClass := src[m[4]:m[5]]
			ent := makeEntity("nhibernate:classmap:"+mapClass, "SCOPE.Component", "model_extraction", file.Path, "csharp", lineOf(src, m[0]))
			setProps(&ent, "framework", "nhibernate", "provenance", "INFERRED_FROM_NH_CLASSMAP",
				"mapping_class", mapClass, "entity_class", entityClass)
			add(ent)

			// Also emit schema_extraction for the mapped entity
			entSchema := makeEntity("nhibernate:schema:"+entityClass, "SCOPE.Component", "schema_extraction", file.Path, "csharp", lineOf(src, m[0]))
			setProps(&entSchema, "framework", "nhibernate", "provenance", "INFERRED_FROM_NH_CLASSMAP",
				"entity_class", entityClass)
			add(entSchema)
		}

		// Map(x => x.Prop) → schema_extraction (column mapping)
		for _, m := range reNHMap.FindAllStringSubmatchIndex(src, -1) {
			prop := src[m[2]:m[3]]
			name := "nhibernate:map:" + prop + ":" + file.Path + ":" + itoa(lineOf(src, m[0]))
			ent := makeEntity(name, "SCOPE.Pattern", "schema_extraction", file.Path, "csharp", lineOf(src, m[0]))
			setProps(&ent, "framework", "nhibernate", "provenance", "INFERRED_FROM_NH_MAP",
				"property", prop)
			add(ent)
		}

		// References(x => x.Nav) → relationship_extraction (many-to-one)
		for _, m := range reNHReferences.FindAllStringSubmatchIndex(src, -1) {
			nav := src[m[2]:m[3]]
			name := "nhibernate:references:" + nav
			ent := makeEntity(name, "SCOPE.Pattern", "relationship_extraction", file.Path, "csharp", lineOf(src, m[0]))
			setProps(&ent, "framework", "nhibernate", "provenance", "INFERRED_FROM_NH_REFERENCES",
				"navigation_property", nav)
			add(ent)
		}

		// HasMany(x => x.Col) → relationship_extraction (one-to-many)
		for _, m := range reNHHasMany.FindAllStringSubmatchIndex(src, -1) {
			nav := src[m[2]:m[3]]
			name := "nhibernate:hasmany:" + nav
			ent := makeEntity(name, "SCOPE.Pattern", "relationship_extraction", file.Path, "csharp", lineOf(src, m[0]))
			setProps(&ent, "framework", "nhibernate", "provenance", "INFERRED_FROM_NH_HAS_MANY",
				"navigation_property", nav)
			add(ent)
		}

		// session.Query<T>() etc → query_attribution
		for _, m := range reNHQuery.FindAllStringSubmatchIndex(src, -1) {
			entityType := src[m[2]:m[3]]
			if csharpPrimitives[entityType] {
				continue
			}
			name := "nhibernate:query:" + entityType + ":" + file.Path + ":" + itoa(lineOf(src, m[0]))
			ent := makeEntity(name, "SCOPE.Operation", "query_attribution", file.Path, "csharp", lineOf(src, m[0]))
			setProps(&ent, "framework", "nhibernate", "provenance", "INFERRED_FROM_NH_QUERY",
				"entity_type", entityType)
			add(ent)
		}
	}

	span.SetAttributes(attribute.Int("entity_count", len(entities)))
	return entities, nil
}
