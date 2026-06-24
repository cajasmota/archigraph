<!-- DO NOT EDIT — generated from docs/coverage/registry.json by 'go run ./tools/coverage gen' -->
# `lang.haskell.orm.persistent` — persistent (Haskell ORM)

Auto-generated. Back to [summary](../summary.md).

- **Language:** [haskell](../by-language/haskell.md)
- **Category:** [orm](../by-category/orm.md)
- **Subcategory:** ORM / Data Mapper
- **Capability cells:** 11

## Capabilities


### Models

| Capability | Status | Verified at | Issue | Cites | Notes |
|------------|--------|-------------|-------|-------|-------|
| Model extraction | ✅ `full` | `2026-06-24` | 5373 | `internal/extractors/haskell/depth.go`<br>`internal/extractors/haskell/depth_test.go`<br>`internal/extractors/haskell/extractor.go` | extractPersistentEntities parses the [persistLowerCase| ... |] / [persistUpperCase| ... |] QuasiQuote schema blocks: each column-0 capitalised header is an entity, emitted as one SCOPE.Component(subtype=orm_model) carrying orm=persistent, orm_model, table_name and a MAPS_TO edge to the derived table (snake_case of the entity, e.g. BlogPost→blog_post), in the same (orm_model,table_name) contract the cross-language ormlink sentinel uses. Proven by TestPersistent_EntityBlock. |
| Model lifecycle extraction | 🔴 `missing` | — | backfill:dictionary-completeness | — | — |
| Schema extraction | 🟢 `partial` | `2026-06-24` | 5373 | `internal/extractors/haskell/depth.go`<br>`internal/extractors/haskell/depth_test.go`<br>`internal/extractors/haskell/extractor.go` | Entity field names are recovered from indented 'fieldName Type ...' lines (persistFieldRE) into a fields/field_count Property; deriving/primary/foreign/unique clause lines are excluded. Honest partial: field SQL types, Maybe-nullability, sql= overrides and !force attributes are not modelled as separate schema columns — follow-up. |

### Relationships

| Capability | Status | Verified at | Issue | Cites | Notes |
|------------|--------|-------------|-------|-------|-------|
| Association extraction | 🔴 `missing` | — | backfill:dictionary-completeness | — | — |
| Foreign key extraction | 🟢 `partial` | `2026-06-24` | 5373 | `internal/extractors/haskell/depth.go`<br>`internal/extractors/haskell/depth_test.go`<br>`internal/extractors/haskell/extractor.go` | A persistent foreign key is the conventional 'authorId UserId' field referencing another entity's auto Id; it is currently recorded as a plain field on the owning entity (no dedicated REFERENCES edge to the target entity/table yet). Honest partial — FK edge synthesis is a follow-up. |
| Lazy loading recognition | 🔴 `missing` | — | backfill:dictionary-completeness | — | — |
| Relationship extraction | 🔴 `missing` | — | backfill:dictionary-completeness | — | — |

### Queries

| Capability | Status | Verified at | Issue | Cites | Notes |
|------------|--------|-------------|-------|-------|-------|
| Query attribution | 🔴 `missing` | — | backfill:dictionary-completeness | — | — |

### Migrations

| Capability | Status | Verified at | Issue | Cites | Notes |
|------------|--------|-------------|-------|-------|-------|
| Migration parsing | 🔴 `missing` | — | backfill:dictionary-completeness | — | — |
| Migration schema ops | 🔴 `missing` | — | backfill:dictionary-completeness | — | — |

### Transactions

| Capability | Status | Verified at | Issue | Cites | Notes |
|------------|--------|-------------|-------|-------|-------|
| Transaction function stamping | 🔴 `missing` | — | backfill:dictionary-completeness | — | — |

## Provenance

This record is sourced from `docs/coverage/registry.json`. To update it, edit the JSON
(or use `go run ./tools/coverage update lang.haskell.orm.persistent ...`) then regenerate:

```
go run ./tools/coverage validate
go run ./tools/coverage gen
```
