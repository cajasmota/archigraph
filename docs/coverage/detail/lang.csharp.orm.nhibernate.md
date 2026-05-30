<!-- DO NOT EDIT — generated from docs/coverage/registry.json by 'go run ./tools/coverage gen' -->
# `lang.csharp.orm.nhibernate` — NHibernate

Auto-generated. Back to [summary](../summary.md).

- **Language:** [C#](../by-language/csharp.md)
- **Category:** [orm](../by-category/orm.md)
- **Subcategory:** ORM / Data Mapper
- **Capability cells:** 8

## Capabilities


### Models

| Capability | Status | Verified at | Issue | Cites | Notes |
|------------|--------|-------------|-------|-------|-------|
| Model extraction | 🟢 `partial` | `2026-05-30` | 3263 | `internal/custom/csharp/dapper_models.go` | FluentNHibernate ClassMap<T> subclass declarations detected via regex; heuristic |
| Schema extraction | 🟢 `partial` | `2026-05-30` | 3263 | `internal/custom/csharp/dapper_models.go` | FluentNHibernate Map(x => x.Prop) column mapping detected via regex; heuristic |

### Relationships

| Capability | Status | Verified at | Issue | Cites | Notes |
|------------|--------|-------------|-------|-------|-------|
| Association extraction | 🔴 `missing` | — | backfill:dictionary-completeness | — | — |
| Foreign key extraction | 🔴 `missing` | — | backfill:dictionary-completeness | — | — |
| Lazy loading recognition | 🔴 `missing` | — | backfill:dictionary-completeness | — | — |
| Relationship extraction | 🟢 `partial` | `2026-05-30` | 3263 | `internal/custom/csharp/dapper_models.go` | FluentNHibernate References/HasMany fluent relationship calls detected via regex; heuristic |

### Queries

| Capability | Status | Verified at | Issue | Cites | Notes |
|------------|--------|-------------|-------|-------|-------|
| Query attribution | 🟢 `partial` | `2026-05-30` | 3263 | `internal/custom/csharp/dapper_models.go` | ISession.Query<T>/Get<T>/Load<T> calls detected via regex; heuristic |

### Migrations

| Capability | Status | Verified at | Issue | Cites | Notes |
|------------|--------|-------------|-------|-------|-------|
| Migration parsing | — `not_applicable` | — | — | — | micro-ORM/query-lib — no built-in migration system |

## Provenance

This record is sourced from `docs/coverage/registry.json`. To update it, edit the JSON
(or use `go run ./tools/coverage update lang.csharp.orm.nhibernate ...`) then regenerate:

```
go run ./tools/coverage validate
go run ./tools/coverage gen
```
