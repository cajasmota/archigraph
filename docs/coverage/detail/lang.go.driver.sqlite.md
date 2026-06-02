<!-- DO NOT EDIT — generated from docs/coverage/registry.json by 'go run ./tools/coverage gen' -->
# `lang.go.driver.sqlite` — mattn/go-sqlite3

Auto-generated. Back to [summary](../summary.md).

- **Language:** [go](../by-language/go.md)
- **Category:** [orm](../by-category/orm.md)
- **Subcategory:** ORM / Data Mapper
- **Capability cells:** 11

## Capabilities


### Models

| Capability | Status | Verified at | Issue | Cites | Notes |
|------------|--------|-------------|-------|-------|-------|
| Model extraction | 🟢 `partial` | `2026-05-30` | 3214 | `internal/custom/golang/sql_drivers.go`<br>`internal/custom/golang/sql_drivers_test.go` | — |
| Model lifecycle extraction | 🔴 `missing` | — | 3628 | — | — |
| Schema extraction | 🟢 `partial` | `2026-05-30` | 3214 | `internal/custom/golang/sql_drivers.go`<br>`internal/custom/golang/sql_drivers_test.go` | — |

### Relationships

| Capability | Status | Verified at | Issue | Cites | Notes |
|------------|--------|-------------|-------|-------|-------|
| Association extraction | — `not_applicable` | — | — | — | — |
| Foreign key extraction | 🟢 `partial` | `2026-05-30` | 3214 | `internal/custom/golang/sql_drivers.go`<br>`internal/custom/golang/sql_drivers_test.go` | — |
| Lazy loading recognition | — `not_applicable` | — | — | — | — |
| Relationship extraction | — `not_applicable` | — | — | — | — |

### Queries

| Capability | Status | Verified at | Issue | Cites | Notes |
|------------|--------|-------------|-------|-------|-------|
| Query attribution | ✅ `full` | `2026-06-02` | — | `internal/custom/golang/sql_drivers.go`<br>`internal/custom/golang/sql_drivers_test.go`<br>`internal/engine/rules/go/orms/sqlite_go.yaml`<br>`internal/extractors/cross/dbmap/orms.go` | Raw db.Query/Exec("…") SQL resolves table topology: dbmap.detectGoSQLDriver (import-gated on github.com/mattn/go-sqlite3) parses FROM/INTO/UPDATE/JOIN and emits SCOPE.DataAccess + ACCESSES_TABLE edges with read/write verb (#3644). |

### Migrations

| Capability | Status | Verified at | Issue | Cites | Notes |
|------------|--------|-------------|-------|-------|-------|
| Migration parsing | — `not_applicable` | — | — | — | — |
| Migration schema ops | 🔴 `missing` | — | 3628 | — | — |

### Transactions

| Capability | Status | Verified at | Issue | Cites | Notes |
|------------|--------|-------------|-------|-------|-------|
| Transaction function stamping | 🔴 `missing` | — | 3628-transaction-function-stamping | — | — |

## Related extraction records

This record provides code-level coverage for the
[`db.sqlite`](./db.sqlite.md) hub record (SQLite (schema)),
which tracks the same technology at a higher level.

## Provenance

This record is sourced from `docs/coverage/registry.json`. To update it, edit the JSON
(or use `go run ./tools/coverage update lang.go.driver.sqlite ...`) then regenerate:

```
go run ./tools/coverage validate
go run ./tools/coverage gen
```
