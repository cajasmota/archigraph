<!-- DO NOT EDIT — generated from docs/coverage/registry.json by 'go run ./tools/coverage gen' -->
# `lang.c-cpp.orm.soci` — SOCI

Auto-generated. Back to [summary](../summary.md).

- **Language:** [C/C++](../by-language/c-cpp.md)
- **Category:** [orm](../by-category/orm.md)
- **Subcategory:** ORM / Data Mapper
- **Capability cells:** 8

## Capabilities


### Models

| Capability | Status | Verified at | Issue | Cites | Notes |
|------------|--------|-------------|-------|-------|-------|
| Model extraction | 🟢 `partial` | `2026-05-30` | — | `internal/custom/cpp/orm.go` | Regex: type_conversion<T> specialization → model entity |
| Schema extraction | 🟢 `partial` | `2026-05-30` | backfill:dictionary-completeness | `internal/custom/cpp/orm.go` | Regex: into(var)/use(var) column bindings |

### Relationships

| Capability | Status | Verified at | Issue | Cites | Notes |
|------------|--------|-------------|-------|-------|-------|
| Association extraction | — `not_applicable` | `2026-05-30` | — | — | SOCI is a raw SQL library; no ORM-level association/relationship layer |
| Foreign key extraction | — `not_applicable` | `2026-05-30` | — | — | SOCI is a raw SQL library; no ORM FK layer |
| Lazy loading recognition | — `not_applicable` | `2026-05-30` | — | — | SOCI is a raw SQL library; no lazy-loading concept |
| Relationship extraction | — `not_applicable` | `2026-05-30` | — | — | SOCI is a raw SQL library; no relationship layer |

### Queries

| Capability | Status | Verified at | Issue | Cites | Notes |
|------------|--------|-------------|-------|-------|-------|
| Query attribution | 🟢 `partial` | `2026-05-30` | — | `internal/custom/cpp/orm.go` | Regex: sql << "SQL" string literal → verb + query entity |

### Migrations

| Capability | Status | Verified at | Issue | Cites | Notes |
|------------|--------|-------------|-------|-------|-------|
| Migration parsing | — `not_applicable` | `2026-05-30` | — | — | SOCI has no built-in migration system |

## Provenance

This record is sourced from `docs/coverage/registry.json`. To update it, edit the JSON
(or use `go run ./tools/coverage update lang.c-cpp.orm.soci ...`) then regenerate:

```
go run ./tools/coverage validate
go run ./tools/coverage gen
```
