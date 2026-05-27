<!-- DO NOT EDIT — generated from docs/coverage/registry.json by 'go run ./tools/coverage gen' -->
# `lang.python.orm.tortoise` — Tortoise ORM

Auto-generated. Back to [summary](../summary.md).

- **Language:** [python](../by-language/python.md)
- **Category:** [orm](../by-category/orm.md)
- **Capability cells:** 3

## Capabilities

| Capability | Status | Verified at | Verified SHA | Issue | Cites | Notes |
|------------|--------|-------------|--------------|-------|-------|-------|
| `migration_parsing` | ❌ `missing` | — | — | — | — | — |
| `model_extraction` | ✅ `full` | `2026-05-28` | — | — | `internal/engine/rules/python/orms/tortoise_orm.yaml` | — |
| `query_attribution` | ⚠️ `partial` | `2026-05-28` | — | — | `internal/engine/orm_queries_python.go` | — |

## Provenance

This record is sourced from `docs/coverage/registry.json`. To update it, edit the JSON
(or use `go run ./tools/coverage update lang.python.orm.tortoise ...`) then regenerate:

```
go run ./tools/coverage validate
go run ./tools/coverage gen
```
