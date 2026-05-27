<!-- DO NOT EDIT — generated from docs/coverage/registry.json by 'go run ./tools/coverage gen' -->
# `lang.jsts.framework.graphql-resolvers` — GraphQL Resolvers (Apollo Server / GraphQL Yoga / etc.)

Auto-generated. Back to [summary](../summary.md).

- **Language:** [JS/TS](../by-language/jsts.md)
- **Category:** [http_framework](../by-category/http_framework.md)
- **Subcategory:** RPC Framework
- **Capability cells:** 6

## Capabilities


### Schema

| Capability | Status | Verified at | Verified SHA | Issue | Cites | Notes |
|------------|--------|-------------|--------------|-------|-------|-------|
| `procedure_extraction` | ✅ `full` | `2026-05-28` | — | — | `internal/engine/rules/graphql/frameworks/apollo_server.yaml`<br>`internal/engine/rules/graphql/frameworks/graphql_yoga.yaml` | — |
| `schema_extraction` | ✅ `full` | `2026-05-28` | — | — | `internal/engine/rules/graphql/frameworks/graphql_schema.yaml` | — |

### Codegen

| Capability | Status | Verified at | Verified SHA | Issue | Cites | Notes |
|------------|--------|-------------|--------------|-------|-------|-------|
| `client_codegen` | ❌ `missing` | — | — | [link](https://github.com/cajasmota/archigraph/issues/2735) | — | — |

### Transport

| Capability | Status | Verified at | Verified SHA | Issue | Cites | Notes |
|------------|--------|-------------|--------------|-------|-------|-------|

### Substrate

| Capability | Status | Verified at | Verified SHA | Issue | Cites | Notes |
|------------|--------|-------------|--------------|-------|-------|-------|
| `constant_propagation` | ✅ `full` | `2026-05-28` | — | — | `internal/links/constant_propagation.go`<br>`internal/substrate/jsts.go`<br>`internal/substrate/substrate.go` | — |
| `env_fallback_recognition` | ✅ `full` | `2026-05-28` | — | — | `internal/links/constant_propagation.go`<br>`internal/substrate/jsts.go`<br>`internal/substrate/substrate.go` | — |
| `import_resolution_quality` | ⚠️ `partial` | `2026-05-28` | — | — | `internal/links/constant_propagation.go`<br>`internal/substrate/jsts.go`<br>`internal/substrate/substrate.go` | — |

## Provenance

This record is sourced from `docs/coverage/registry.json`. To update it, edit the JSON
(or use `go run ./tools/coverage update lang.jsts.framework.graphql-resolvers ...`) then regenerate:

```
go run ./tools/coverage validate
go run ./tools/coverage gen
```
