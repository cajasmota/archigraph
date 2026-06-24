<!-- DO NOT EDIT — generated from docs/coverage/registry.json by 'go run ./tools/coverage gen' -->
# `lang.dart.orm.isar` — Isar (Dart NoSQL embedded DB)

Auto-generated. Back to [summary](../summary.md).

- **Language:** [dart](../by-language/dart.md)
- **Category:** [orm](../by-category/orm.md)
- **Subcategory:** ORM / Data Mapper
- **Capability cells:** 11

## Capabilities


### Models

| Capability | Status | Verified at | Issue | Cites | Notes |
|------------|--------|-------------|-------|-------|-------|
| Model extraction | ✅ `full` | `2026-06-24` | 5361 | `internal/custom/dart/persistence_test.go` | #5361: @collection / @Collection() class (isarCollectionRe) -> SCOPE.Schema/collection stamped framework=isar, store_kind=nosql_embedded. Proven by TestDartIsar (User collection). |
| Model lifecycle extraction | — `not_applicable` | — | 5361 | — | Isar has no static lifecycle-hook / lazy-loading proxy declaration to recognise; .filter() queries are a deferred query-DSL concern. |
| Schema extraction | ✅ `full` | `2026-06-24` | 5361 | `internal/custom/dart/persistence_test.go` | #5361: the collection's fields (classFieldColumns) become SCOPE.Schema/column; the Id-typed / id field is flagged primary_key. Proven by TestDartIsar (id pk, name, email). |

### Relationships

| Capability | Status | Verified at | Issue | Cites | Notes |
|------------|--------|-------------|-------|-------|-------|
| Association extraction | 🔴 `missing` | — | 5361 | — | — |
| Foreign key extraction | 🔴 `missing` | — | 5361 | — | — |
| Lazy loading recognition | — `not_applicable` | — | 5361 | — | Isar has no static lifecycle-hook / lazy-loading proxy declaration to recognise; .filter() queries are a deferred query-DSL concern. |
| Relationship extraction | 🔴 `missing` | — | 5361 | — | — |

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
(or use `go run ./tools/coverage update lang.dart.orm.isar ...`) then regenerate:

```
go run ./tools/coverage validate
go run ./tools/coverage gen
```
