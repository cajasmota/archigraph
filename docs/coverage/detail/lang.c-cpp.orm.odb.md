<!-- DO NOT EDIT — generated from docs/coverage/registry.json by 'go run ./tools/coverage gen' -->
# `lang.c-cpp.orm.odb` — ODB

Auto-generated. Back to [summary](../summary.md).

- **Language:** [C/C++](../by-language/c-cpp.md)
- **Category:** [orm](../by-category/orm.md)
- **Subcategory:** ORM / Data Mapper
- **Capability cells:** 8

## Capabilities


### Models

| Capability | Status | Verified at | Issue | Cites | Notes |
|------------|--------|-------------|-------|-------|-------|
| Model extraction | 🟢 `partial` | `2026-05-30` | — | `internal/custom/cpp/orm.go` | Regex: #pragma db object → class/struct name |
| Schema extraction | 🟢 `partial` | `2026-05-30` | backfill:dictionary-completeness | `internal/custom/cpp/orm.go` | Regex: #pragma db member(field) → column entity |

### Relationships

| Capability | Status | Verified at | Issue | Cites | Notes |
|------------|--------|-------------|-------|-------|-------|
| Association extraction | 🟢 `partial` | `2026-05-30` | backfill:dictionary-completeness | `internal/custom/cpp/orm.go` | Regex: one_to_one/many_to_many in #pragma db member annotations |
| Foreign key extraction | 🟢 `partial` | `2026-05-30` | backfill:dictionary-completeness | `internal/custom/cpp/orm.go` | Regex: inverse() in pragma member annotations |
| Lazy loading recognition | 🟢 `partial` | `2026-05-30` | backfill:dictionary-completeness | `internal/custom/cpp/orm.go` | Regex: odb::lazy_ptr<T> / lazy_shared_ptr<T> |
| Relationship extraction | 🟢 `partial` | `2026-05-30` | backfill:dictionary-completeness | `internal/custom/cpp/orm.go` | Regex: relationship kind from #pragma db member annotations |

### Queries

| Capability | Status | Verified at | Issue | Cites | Notes |
|------------|--------|-------------|-------|-------|-------|
| Query attribution | 🟢 `partial` | `2026-05-30` | — | `internal/custom/cpp/orm.go` | Regex: odb::query<T> / odb::result<T> |

### Migrations

| Capability | Status | Verified at | Issue | Cites | Notes |
|------------|--------|-------------|-------|-------|-------|
| Migration parsing | — `not_applicable` | `2026-05-30` | — | — | ODB has no built-in migration system |

## Provenance

This record is sourced from `docs/coverage/registry.json`. To update it, edit the JSON
(or use `go run ./tools/coverage update lang.c-cpp.orm.odb ...`) then regenerate:

```
go run ./tools/coverage validate
go run ./tools/coverage gen
```
