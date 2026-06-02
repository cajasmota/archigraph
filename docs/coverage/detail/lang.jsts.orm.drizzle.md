<!-- DO NOT EDIT — generated from docs/coverage/registry.json by 'go run ./tools/coverage gen' -->
# `lang.jsts.orm.drizzle` — Drizzle

Auto-generated. Back to [summary](../summary.md).

- **Language:** [JS/TS](../by-language/jsts.md)
- **Category:** [orm](../by-category/orm.md)
- **Subcategory:** ORM / Data Mapper
- **Capability cells:** 8

## Capabilities


### Models

| Capability | Status | Verified at | Issue | Cites | Notes |
|------------|--------|-------------|-------|-------|-------|
| Model extraction | ✅ `full` | `2026-05-28` | — | `internal/engine/rules/javascript_typescript/orms/drizzle.yaml` | — |
| Schema extraction | ✅ `full` | `2026-05-29` | 3067 | `internal/custom/javascript/drizzle.go`<br>`internal/custom/javascript/orm_build_3067_test.go` | — |

### Relationships

| Capability | Status | Verified at | Issue | Cites | Notes |
|------------|--------|-------------|-------|-------|-------|
| Association extraction | ✅ `full` | `2026-05-29` | 3067 | `internal/custom/javascript/drizzle.go`<br>`internal/custom/javascript/orm_build_3067_test.go` | — |
| Foreign key extraction | ✅ `full` | `2026-05-29` | 3067 | `internal/custom/javascript/drizzle.go`<br>`internal/custom/javascript/orm_build_3067_test.go` | — |
| Lazy loading recognition | — `not_applicable` | — | — | — | Drizzle is a query builder; no lazy loading model — all queries are explicit (#3184) |
| Relationship extraction | ✅ `full` | `2026-05-29` | 3067 | `internal/custom/javascript/drizzle.go`<br>`internal/custom/javascript/orm_build_3067_test.go` | — |

### Queries

| Capability | Status | Verified at | Issue | Cites | Notes |
|------------|--------|-------------|-------|-------|-------|
| Query attribution | 🟢 `partial` | `2026-06-02` | — | `internal/engine/orm_queries_jsts.go`<br>`internal/extractors/cross/dbmap/query_builders.go`<br>`internal/extractors/cross/dbmap/query_builders_test.go` | #3628 area #3: Drizzle db.select().from(users)/db.insert(users) resolve the pgTable/mysqlTable/sqliteTable OBJECT to its declared name literal → ACCESSES_TABLE edges. Partial: cross-file table objects unresolved (skipped, no fabricated edge). Proven by TestDrizzle* in query_builders_test.go. |

### Migrations

| Capability | Status | Verified at | Issue | Cites | Notes |
|------------|--------|-------------|-------|-------|-------|
| Migration parsing | ✅ `full` | `2026-05-28` | — | `internal/custom/javascript/drizzle.go`<br>`internal/custom/javascript/extractors_coverage_test.go` | — |

## Provenance

This record is sourced from `docs/coverage/registry.json`. To update it, edit the JSON
(or use `go run ./tools/coverage update lang.jsts.orm.drizzle ...`) then regenerate:

```
go run ./tools/coverage validate
go run ./tools/coverage gen
```
