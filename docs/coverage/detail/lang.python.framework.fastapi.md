<!-- DO NOT EDIT — generated from docs/coverage/registry.json by 'go run ./tools/coverage gen' -->
# `lang.python.framework.fastapi` — FastAPI

Auto-generated. Back to [summary](../summary.md).

- **Language:** [python](../by-language/python.md)
- **Category:** [http_framework](../by-category/http_framework.md)
- **Capability cells:** 4

## Capabilities

| Capability | Status | Verified at | Verified SHA | Issue | Cites | Notes |
|------------|--------|-------------|--------------|-------|-------|-------|
| `auth_coverage` | ⚠️ `partial` | `2026-05-28` | — | — | `internal/engine/rules/python/frameworks/fastapi.yaml` | — |
| `endpoint_synthesis` | ✅ `full` | `2026-05-28` | — | — | `internal/engine/http_endpoint_synthesis.go`<br>`internal/engine/rules/python/frameworks/fastapi.yaml` | — |
| `handler_attribution` | ✅ `full` | `2026-05-28` | — | — | `internal/engine/rules/python/frameworks/fastapi.yaml` | — |
| `middleware_coverage` | ❌ `missing` | — | — | — | — | — |

## Provenance

This record is sourced from `docs/coverage/registry.json`. To update it, edit the JSON
(or use `go run ./tools/coverage update lang.python.framework.fastapi ...`) then regenerate:

```
go run ./tools/coverage validate
go run ./tools/coverage gen
```
