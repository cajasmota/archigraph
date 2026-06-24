<!-- DO NOT EDIT — generated from docs/coverage/registry.json by 'go run ./tools/coverage gen' -->
# `lang.dart.orm.objectbox` — ObjectBox (Dart NoSQL DB)

Auto-generated. Back to [summary](../summary.md).

- **Language:** [dart](../by-language/dart.md)
- **Category:** [orm](../by-category/orm.md)
- **Subcategory:** ORM / Data Mapper
- **Capability cells:** 11

## Capabilities


### Models

| Capability | Status | Verified at | Issue | Cites | Notes |
|------------|--------|-------------|-------|-------|-------|
| Model extraction | ✅ `full` | `2026-06-24` | 5361 | `internal/custom/dart/persistence_test.go` | #5361: @Entity() class (objectboxEntityRe, the parenthesised form distinguishes it from floor's bare @entity) -> SCOPE.Schema/model stamped framework=objectbox, store_kind=nosql_embedded. Proven by TestDartObjectbox (Note model). |
| Model lifecycle extraction | — `not_applicable` | — | 5361 | — | ObjectBox has no static lifecycle-hook / lazy-loading proxy declaration to recognise. |
| Schema extraction | ✅ `full` | `2026-06-24` | 5361 | `internal/custom/dart/persistence_test.go` | #5361: objectbox model fields (classFieldColumns) become SCOPE.Schema/column; the id field is flagged primary_key. Proven by TestDartObjectbox (id pk, text). |

### Relationships

| Capability | Status | Verified at | Issue | Cites | Notes |
|------------|--------|-------------|-------|-------|-------|
| Association extraction | 🔴 `missing` | — | 5361 | — | — |
| Foreign key extraction | 🔴 `missing` | — | 5361 | — | — |
| Lazy loading recognition | — `not_applicable` | — | 5361 | — | ObjectBox has no static lifecycle-hook / lazy-loading proxy declaration to recognise. |
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
(or use `go run ./tools/coverage update lang.dart.orm.objectbox ...`) then regenerate:

```
go run ./tools/coverage validate
go run ./tools/coverage gen
```
