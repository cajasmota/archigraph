<!-- DO NOT EDIT — generated from docs/coverage/registry.json by 'go run ./tools/coverage gen' -->
# `lang.dart.orm.floor` — floor (Room-style Dart ORM)

Auto-generated. Back to [summary](../summary.md).

- **Language:** [dart](../by-language/dart.md)
- **Category:** [orm](../by-category/orm.md)
- **Subcategory:** ORM / Data Mapper
- **Capability cells:** 11

## Capabilities


### Models

| Capability | Status | Verified at | Issue | Cites | Notes |
|------------|--------|-------------|-------|-------|-------|
| Model extraction | ✅ `full` | `2026-06-24` | 5361 | `internal/custom/dart/persistence_test.go` | #5361: @entity class (floorEntityRe) -> SCOPE.Schema/model; @dao abstract class (floorDaoRe) -> SCOPE.Schema/dao; @Database(...) abstract class (floorDatabaseRe) -> SCOPE.Schema/database. framework=floor. Proven by TestDartFloor (Person model, PersonDao dao, AppDatabase database). |
| Model lifecycle extraction | — `not_applicable` | — | 5361 | — | floor has no static lifecycle-hook / lazy-loading proxy declaration to recognise. |
| Schema extraction | ✅ `full` | `2026-06-24` | 5361 | `internal/custom/dart/persistence_test.go` | #5361: a floor model's final <Type> <name>; instance fields (classFieldColumns over the brace-balanced class body) become SCOPE.Schema/column carrying column_type + owning model. Proven by TestDartFloor (Person.name column). |

### Relationships

| Capability | Status | Verified at | Issue | Cites | Notes |
|------------|--------|-------------|-------|-------|-------|
| Association extraction | 🔴 `missing` | — | 5361 | — | — |
| Foreign key extraction | 🔴 `missing` | — | 5361 | — | — |
| Lazy loading recognition | — `not_applicable` | — | 5361 | — | floor has no static lifecycle-hook / lazy-loading proxy declaration to recognise. |
| Relationship extraction | 🔴 `missing` | — | 5361 | — | floor @ForeignKey relations are a deferred follow-up; this PR (#5361) implements entity/dao/database + column extraction only. |

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
(or use `go run ./tools/coverage update lang.dart.orm.floor ...`) then regenerate:

```
go run ./tools/coverage validate
go run ./tools/coverage gen
```
