<!-- DO NOT EDIT — generated from docs/coverage/registry.json by 'go run ./tools/coverage gen' -->
# `lang.rust.orm.sqlx` — sqlx (Rust)

Auto-generated. Back to [summary](../summary.md).

- **Language:** [rust](../by-language/rust.md)
- **Category:** [orm](../by-category/orm.md)
- **Subcategory:** ORM / Data Mapper
- **Capability cells:** 8

## Capabilities


### Models

| Capability | Status | Verified at | Issue | Cites | Notes |
|------------|--------|-------------|-------|-------|-------|
| Model extraction | 🟢 `partial` | `2026-05-28` | — | `internal/engine/rules/rust/orms/sqlx.yaml` | — |
| Schema extraction | 🔴 `missing` | — | backfill:dictionary-completeness | — | — |

### Relationships

| Capability | Status | Verified at | Issue | Cites | Notes |
|------------|--------|-------------|-------|-------|-------|
| Association extraction | — `not_applicable` | — | — | — | sqlx is a compile-checked query layer; no relationship/association DSL |
| Foreign key extraction | — `not_applicable` | — | — | — | sqlx is a compile-checked query layer; no relationship/association DSL |
| Lazy loading recognition | — `not_applicable` | — | — | — | sqlx is a compile-checked query layer; no relationship/association DSL |
| Relationship extraction | — `not_applicable` | — | — | — | sqlx is a compile-checked query layer; no relationship/association DSL |

### Queries

| Capability | Status | Verified at | Issue | Cites | Notes |
|------------|--------|-------------|-------|-------|-------|
| Query attribution | 🟢 `partial` | `2026-05-28` | — | `internal/engine/rules/rust/orms/sqlx.yaml` | — |

### Migrations

| Capability | Status | Verified at | Issue | Cites | Notes |
|------------|--------|-------------|-------|-------|-------|
| Migration parsing | 🔴 `missing` | — | — | — | — |

## Provenance

This record is sourced from `docs/coverage/registry.json`. To update it, edit the JSON
(or use `go run ./tools/coverage update lang.rust.orm.sqlx ...`) then regenerate:

```
go run ./tools/coverage validate
go run ./tools/coverage gen
```
