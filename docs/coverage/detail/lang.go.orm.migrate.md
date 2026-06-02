<!-- DO NOT EDIT — generated from docs/coverage/registry.json by 'go run ./tools/coverage gen' -->
# `lang.go.orm.migrate` — golang-migrate

Auto-generated. Back to [summary](../summary.md).

- **Language:** [go](../by-language/go.md)
- **Category:** [orm](../by-category/orm.md)
- **Subcategory:** ORM / Data Mapper
- **Capability cells:** 11

## Capabilities


### Models

| Capability | Status | Verified at | Issue | Cites | Notes |
|------------|--------|-------------|-------|-------|-------|
| Model extraction | — `not_applicable` | — | — | — | — |
| Model lifecycle extraction | 🔴 `missing` | — | 3628 | — | — |
| Schema extraction | — `not_applicable` | — | — | — | golang-migrate is a SQL migration runner — no Go ORM model/relationship layer (schema in raw .up/.down.sql; migration_parsing covers it) |

### Relationships

| Capability | Status | Verified at | Issue | Cites | Notes |
|------------|--------|-------------|-------|-------|-------|
| Association extraction | — `not_applicable` | — | — | — | golang-migrate is a SQL migration runner — no Go ORM model/relationship layer (schema in raw .up/.down.sql; migration_parsing covers it) |
| Foreign key extraction | — `not_applicable` | — | — | — | golang-migrate is a SQL migration runner — no Go ORM model/relationship layer (schema in raw .up/.down.sql; migration_parsing covers it) |
| Lazy loading recognition | — `not_applicable` | `2026-05-29` | — | — | golang-migrate is a forward/backward SQL migration runner with no ORM runtime: there is no object-fetch path, so eager/lazy loading is not a modellable concept (consistent with this record's model_extraction=N/A and query_attribution=N/A). |
| Relationship extraction | — `not_applicable` | — | — | — | golang-migrate is a SQL migration runner — no Go ORM model/relationship layer (schema in raw .up/.down.sql; migration_parsing covers it) |

### Queries

| Capability | Status | Verified at | Issue | Cites | Notes |
|------------|--------|-------------|-------|-------|-------|
| Query attribution | — `not_applicable` | — | — | — | — |

### Migrations

| Capability | Status | Verified at | Issue | Cites | Notes |
|------------|--------|-------------|-------|-------|-------|
| Migration parsing | 🟢 `partial` | `2026-05-28` | — | `internal/engine/rules/go/orms/golang_migrate.yaml` | — |
| Migration schema ops | 🔴 `missing` | — | 3628 | — | — |

### Transactions

| Capability | Status | Verified at | Issue | Cites | Notes |
|------------|--------|-------------|-------|-------|-------|
| Transaction function stamping | 🔴 `missing` | — | 3628-transaction-function-stamping | — | — |

## Provenance

This record is sourced from `docs/coverage/registry.json`. To update it, edit the JSON
(or use `go run ./tools/coverage update lang.go.orm.migrate ...`) then regenerate:

```
go run ./tools/coverage validate
go run ./tools/coverage gen
```
