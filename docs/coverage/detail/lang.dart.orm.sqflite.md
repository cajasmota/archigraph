<!-- DO NOT EDIT — generated from docs/coverage/registry.json by 'go run ./tools/coverage gen' -->
# `lang.dart.orm.sqflite` — sqflite (Dart raw SQLite)

Auto-generated. Back to [summary](../summary.md).

- **Language:** [dart](../by-language/dart.md)
- **Category:** [orm](../by-category/orm.md)
- **Subcategory:** ORM / Data Mapper
- **Capability cells:** 11

## Capabilities


### Models

| Capability | Status | Verified at | Issue | Cites | Notes |
|------------|--------|-------------|-------|-------|-------|
| Model extraction | 🟢 `partial` | `2026-06-24` | 5361 | `internal/custom/dart/persistence_test.go` | #5361 honest-partial: sqflite is a raw-SQL driver, not an annotation ORM. CREATE TABLE statements inside db.execute('...') (sqfliteCreateRe) emit a SCOPE.Schema/table stamped raw_sql=true. Only CREATE TABLE DDL is modelled (no model classes exist). Proven by TestDartSqfliteRawSQL (products table). |
| Model lifecycle extraction | — `not_applicable` | — | 5361 | — | sqflite is raw SQL with no ORM model/association layer to recognise. |
| Schema extraction | 🟢 `partial` | `2026-06-24` | 5361 | `internal/custom/dart/persistence_test.go` | #5361 honest-partial: columns are parsed from the CREATE TABLE column body (parseSQLColumns, top-level comma split; table-level PRIMARY/FOREIGN/UNIQUE/CHECK/CONSTRAINT clauses skipped) into SCOPE.Schema/column carrying sql_type + primary_key. ALTER TABLE / dynamic SQL not modelled. Proven by TestDartSqfliteRawSQL (id INTEGER pk, name TEXT, price REAL). |

### Relationships

| Capability | Status | Verified at | Issue | Cites | Notes |
|------------|--------|-------------|-------|-------|-------|
| Association extraction | 🔴 `missing` | — | 5361 | — | — |
| Foreign key extraction | — `not_applicable` | — | 5361 | — | sqflite is raw SQL with no ORM model/association layer to recognise. |
| Lazy loading recognition | — `not_applicable` | — | 5361 | — | sqflite is raw SQL with no ORM model/association layer to recognise. |
| Relationship extraction | — `not_applicable` | — | 5361 | — | sqflite is raw SQL with no ORM model/association layer to recognise. |

### Queries

| Capability | Status | Verified at | Issue | Cites | Notes |
|------------|--------|-------------|-------|-------|-------|
| Query attribution | 🔴 `missing` | — | 5361 | — | sqflite query attribution (db.query/insert/update/delete call -> table) is a deferred follow-up; this PR (#5361) parses CREATE TABLE DDL schema only. |

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
(or use `go run ./tools/coverage update lang.dart.orm.sqflite ...`) then regenerate:

```
go run ./tools/coverage validate
go run ./tools/coverage gen
```
