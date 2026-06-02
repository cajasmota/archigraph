<!-- DO NOT EDIT — generated from docs/coverage/registry.json by 'go run ./tools/coverage gen' -->
# `security.auth.oauth2` — OAuth2

Auto-generated. Back to [summary](../summary.md).

- **Language:** [multi](../by-language/multi.md)
- **Category:** [security](../by-category/security.md)
- **Capability cells:** 3

## Capabilities

| Capability | Status | Verified at | Issue | Cites | Notes |
|------------|--------|-------------|-------|-------|-------|
| Auth policy | 🟢 `partial` | `2026-05-28` | — | `internal/engine/java_auth_policy.go` | — |
| Secret detection | 🔴 `missing` | — | 3828 | — | No extraction yet for this capability on this auth/security record; tracked in #3828 (may be reclassified not_applicable pending owner sign-off). |
| SQL injection | — `not_applicable` | — | — | — | — |

## Provenance

This record is sourced from `docs/coverage/registry.json`. To update it, edit the JSON
(or use `go run ./tools/coverage update security.auth.oauth2 ...`) then regenerate:

```
go run ./tools/coverage validate
go run ./tools/coverage gen
```
