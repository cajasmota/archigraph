<!-- DO NOT EDIT — generated from docs/coverage/registry.json by 'go run ./tools/coverage gen' -->
# `lang.dart.orm.hive` — Hive (Dart key-value DB)

Auto-generated. Back to [summary](../summary.md).

- **Language:** [dart](../by-language/dart.md)
- **Category:** [orm](../by-category/orm.md)
- **Subcategory:** ORM / Data Mapper
- **Capability cells:** 11

## Capabilities


### Models

| Capability | Status | Verified at | Issue | Cites | Notes |
|------------|--------|-------------|-------|-------|-------|
| Model extraction | ✅ `full` | `2026-06-24` | 5361 | `internal/custom/dart/persistence_test.go` | #5361: @HiveType(typeId: n) class (hiveTypeRe) -> SCOPE.Schema/model stamped framework=hive, store_kind=key_value, type_id. Proven by TestDartHive (Contact, type_id=3). |
| Model lifecycle extraction | — `not_applicable` | — | 5361 | — | Hive boxes have no static lifecycle-hook / lazy-loading proxy declaration to recognise. |
| Schema extraction | ✅ `full` | `2026-06-24` | 5361 | `internal/custom/dart/persistence_test.go` | #5361: each @HiveField(k) final <Type> <name>; field becomes a SCOPE.Schema/field carrying column_type + the hive_field ordinal (nearestHiveOrdinal binds the annotation to the field by name offset). Proven by TestDartHive (name ordinal 0, age ordinal 1). |

### Relationships

| Capability | Status | Verified at | Issue | Cites | Notes |
|------------|--------|-------------|-------|-------|-------|
| Association extraction | 🔴 `missing` | — | 5361 | — | — |
| Foreign key extraction | 🔴 `missing` | — | 5361 | — | — |
| Lazy loading recognition | — `not_applicable` | — | 5361 | — | Hive boxes have no static lifecycle-hook / lazy-loading proxy declaration to recognise. |
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
(or use `go run ./tools/coverage update lang.dart.orm.hive ...`) then regenerate:

```
go run ./tools/coverage validate
go run ./tools/coverage gen
```
