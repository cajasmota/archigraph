<!-- DO NOT EDIT — generated from docs/coverage/registry.json by 'go run ./tools/coverage gen' -->
# `lang.go.orm.pgx` — pgx (PostgreSQL driver)

Auto-generated. Back to [summary](../summary.md).

- **Language:** [go](../by-language/go.md)
- **Category:** [orm](../by-category/orm.md)
- **Subcategory:** ORM / Data Mapper
- **Capability cells:** 8

## Capabilities


### Models

| Capability | Status | Verified at | Issue | Cites | Notes |
|------------|--------|-------------|-------|-------|-------|
| Model extraction | 🟢 `partial` | `2026-05-30` | 3214 | `internal/custom/golang/sql_drivers.go`<br>`internal/custom/golang/sql_drivers_test.go` | structs carrying db:"col" field tags are treated as schemas (heuristic — db: tag does not prove the struct is a DB table); fixture-tested (TestSqlxModelsAndQueries / TestPgxModelsAndQueries) |
| Schema extraction | 🟢 `partial` | `2026-05-30` | 3214 | `internal/custom/golang/sql_drivers.go`<br>`internal/custom/golang/sql_drivers_test.go` | columns from db: struct tags + CREATE TABLE SQL literals in double/backquoted strings; schema inferred from strings only, not from runtime DDL or migrate files (separate migration_parsing cell) |

### Relationships

| Capability | Status | Verified at | Issue | Cites | Notes |
|------------|--------|-------------|-------|-------|-------|
| Association extraction | — `not_applicable` | — | — | — | — |
| Foreign key extraction | 🟢 `partial` | `2026-05-30` | 3214 | `internal/custom/golang/sql_drivers.go`<br>`internal/custom/golang/sql_drivers_test.go` | FOREIGN KEY clauses parsed from CREATE TABLE SQL string literals; only catches inline DDL, not FK declarations in external migration files or ALTER TABLE statements |
| Lazy loading recognition | — `not_applicable` | — | — | — | — |
| Relationship extraction | — `not_applicable` | — | — | — | — |

### Queries

| Capability | Status | Verified at | Issue | Cites | Notes |
|------------|--------|-------------|-------|-------|-------|
| Query attribution | 🟢 `partial` | `2026-05-30` | 3214 | `internal/custom/golang/sql_drivers.go`<br>`internal/custom/golang/sql_drivers_test.go` | Exec/Query/QueryRow/Get/Select/NamedExec call sites + SQL literal content captured; cannot bind a call site to a specific model without data-flow analysis |

### Migrations

| Capability | Status | Verified at | Issue | Cites | Notes |
|------------|--------|-------------|-------|-------|-------|
| Migration parsing | — `not_applicable` | — | — | — | — |

## Provenance

This record is sourced from `docs/coverage/registry.json`. To update it, edit the JSON
(or use `go run ./tools/coverage update lang.go.orm.pgx ...`) then regenerate:

```
go run ./tools/coverage validate
go run ./tools/coverage gen
```
