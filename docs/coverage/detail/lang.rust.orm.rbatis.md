<!-- DO NOT EDIT — generated from docs/coverage/registry.json by 'go run ./tools/coverage gen' -->
# `lang.rust.orm.rbatis` — Rbatis

Auto-generated. Back to [summary](../summary.md).

- **Language:** [rust](../by-language/rust.md)
- **Category:** [orm](../by-category/orm.md)
- **Subcategory:** ORM / Data Mapper
- **Capability cells:** 8

## Capabilities


### Models

| Capability | Status | Verified at | Issue | Cites | Notes |
|------------|--------|-------------|-------|-------|-------|
| Model extraction | 🔴 `missing` | — | — | — | — |
| Schema extraction | 🔴 `missing` | — | backfill:dictionary-completeness | — | — |

### Relationships

| Capability | Status | Verified at | Issue | Cites | Notes |
|------------|--------|-------------|-------|-------|-------|
| Association extraction | — `not_applicable` | — | — | — | rbatis is a SQL/XML mapper; no relationship/association DSL |
| Foreign key extraction | — `not_applicable` | — | — | — | rbatis is a SQL/XML mapper; no relationship/association DSL |
| Lazy loading recognition | — `not_applicable` | — | — | — | rbatis is a SQL/XML mapper; no relationship/association DSL |
| Relationship extraction | — `not_applicable` | — | — | — | rbatis is a SQL/XML mapper; no relationship/association DSL |

### Queries

| Capability | Status | Verified at | Issue | Cites | Notes |
|------------|--------|-------------|-------|-------|-------|
| Query attribution | 🔴 `missing` | — | — | — | — |

### Migrations

| Capability | Status | Verified at | Issue | Cites | Notes |
|------------|--------|-------------|-------|-------|-------|
| Migration parsing | 🔴 `missing` | — | — | — | — |

## Provenance

This record is sourced from `docs/coverage/registry.json`. To update it, edit the JSON
(or use `go run ./tools/coverage update lang.rust.orm.rbatis ...`) then regenerate:

```
go run ./tools/coverage validate
go run ./tools/coverage gen
```
