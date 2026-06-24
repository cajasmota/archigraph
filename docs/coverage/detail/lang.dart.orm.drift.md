<!-- DO NOT EDIT — generated from docs/coverage/registry.json by 'go run ./tools/coverage gen' -->
# `lang.dart.orm.drift` — drift (Dart SQLite ORM)

Auto-generated. Back to [summary](../summary.md).

- **Language:** [dart](../by-language/dart.md)
- **Category:** [orm](../by-category/orm.md)
- **Subcategory:** ORM / Data Mapper
- **Capability cells:** 11

## Capabilities


### Models

| Capability | Status | Verified at | Issue | Cites | Notes |
|------------|--------|-------------|-------|-------|-------|
| Model extraction | ✅ `full` | `2026-06-24` | 5361 | `internal/custom/dart/persistence_test.go` | #5361: a class extending Table (driftTableRe) emits a SCOPE.Schema/table stamped framework=drift; @DriftDatabase(tables:[...]) (driftDatabaseRe) emits a SCOPE.Schema/database node. File pre-filtered by hasPersistenceSignal. Proven by TestDartDrift (TodoItems table + AppDatabase database). |
| Model lifecycle extraction | — `not_applicable` | — | 5361 | — | drift has no static lifecycle-hook / lazy-loading proxy declaration to recognise; reactive .watch() streams are a query-DSL concern, deferred. |
| Schema extraction | ✅ `full` | `2026-06-24` | 5361 | `internal/custom/dart/persistence_test.go` | #5361: each typed column getter IntColumn/TextColumn/BoolColumn get <name> => <builder>()() (driftColumnRe) becomes a SCOPE.Schema/column carrying column_type + sql_type (integer->INTEGER, text->TEXT, boolean->BOOLEAN, real->REAL, dateTime->DATETIME, blob->BLOB); autoIncrement columns flagged primary_key. Proven by TestDartDrift (id INTEGER pk, title TEXT, done BOOLEAN). |

### Relationships

| Capability | Status | Verified at | Issue | Cites | Notes |
|------------|--------|-------------|-------|-------|-------|
| Association extraction | 🔴 `missing` | — | 5361 | — | — |
| Foreign key extraction | ✅ `full` | `2026-06-24` | 5361 | `internal/custom/dart/persistence_test.go` | #5361: the @DriftDatabase tables list yields REFERENCES database->table edges (to_model prop). Inter-table FK references() columns are a deferred follow-up. Proven by TestDartDrift. |
| Lazy loading recognition | — `not_applicable` | — | 5361 | — | drift has no static lifecycle-hook / lazy-loading proxy declaration to recognise; reactive .watch() streams are a query-DSL concern, deferred. |
| Relationship extraction | ✅ `full` | `2026-06-24` | 5361 | `internal/custom/dart/persistence_test.go` | #5361: @DriftDatabase(tables:[Foo,Bar]) emits one REFERENCES edge database->table per listed table (ref_kind=drift_table). Proven by TestDartDrift (AppDatabase REFERENCES TodoItems). |

### Queries

| Capability | Status | Verified at | Issue | Cites | Notes |
|------------|--------|-------------|-------|-------|-------|
| Query attribution | 🔴 `missing` | — | 5361 | — | — |

### Migrations

| Capability | Status | Verified at | Issue | Cites | Notes |
|------------|--------|-------------|-------|-------|-------|
| Migration parsing | 🔴 `missing` | — | 5361 | — | — |
| Migration schema ops | 🔴 `missing` | — | 5361 | — | — |

### Transactions

| Capability | Status | Verified at | Issue | Cites | Notes |
|------------|--------|-------------|-------|-------|-------|
| Transaction function stamping | 🔴 `missing` | — | 5361 | — | — |

## Provenance

This record is sourced from `docs/coverage/registry.json`. To update it, edit the JSON
(or use `go run ./tools/coverage update lang.dart.orm.drift ...`) then regenerate:

```
go run ./tools/coverage validate
go run ./tools/coverage gen
```
