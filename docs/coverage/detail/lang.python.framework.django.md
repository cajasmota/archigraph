<!-- DO NOT EDIT — generated from docs/coverage/registry.json by 'go run ./tools/coverage gen' -->
# `lang.python.framework.django` — Django

Auto-generated. Back to [summary](../summary.md).

- **Language:** [python](../by-language/python.md)
- **Category:** [http_framework](../by-category/http_framework.md)
- **Capability cells:** 4

## Capabilities

| Capability | Status | Verified at | Verified SHA | Issue | Cites | Notes |
|------------|--------|-------------|--------------|-------|-------|-------|
| `auth_coverage` | ⚠️ `partial` | `2026-05-28` | — | — | `internal/engine/django_drf_actions.go` | — |
| `endpoint_synthesis` | ✅ `full` | `2026-05-28` | — | — | `internal/engine/django_routes.go`<br>`internal/engine/django_urlconf_nested.go`<br>`internal/engine/rules/python/frameworks/django.yaml` | — |
| `handler_attribution` | ✅ `full` | `2026-05-28` | — | — | `internal/engine/django_admin_routes.go`<br>`internal/engine/django_routes.go` | — |
| `middleware_coverage` | ⚠️ `partial` | `2026-05-28` | — | — | `internal/engine/django_imports_rewrite.go` | — |

## Provenance

This record is sourced from `docs/coverage/registry.json`. To update it, edit the JSON
(or use `go run ./tools/coverage update lang.python.framework.django ...`) then regenerate:

```
go run ./tools/coverage validate
go run ./tools/coverage gen
```
