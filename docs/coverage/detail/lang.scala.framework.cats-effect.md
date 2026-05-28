<!-- DO NOT EDIT — generated from docs/coverage/registry.json by 'go run ./tools/coverage gen' -->
# `lang.scala.framework.cats-effect` — Cats Effect (concurrency runtime)

Auto-generated. Back to [summary](../summary.md).

- **Language:** [scala](../by-language/scala.md)
- **Category:** [http_framework](../by-category/http_framework.md)
- **Subcategory:** JVM Backend
- **Capability cells:** 45

## Capabilities


### Routing

| Capability | Status | Verified at | Verified SHA | Issue | Cites | Notes |
|------------|--------|-------------|--------------|-------|-------|-------|
| `endpoint_synthesis` | — `not_applicable` | — | — | — | — | Cats Effect is an effect-system runtime, not a web backend — no routing/DI/transaction/AOP container. |
| `handler_attribution` | ⚠️ `partial` | `2026-05-28` | — | — | `internal/engine/rules/scala/frameworks/cats_effect.yaml` | — |
| `route_extraction` | — `not_applicable` | — | — | — | — | Cats Effect is an effect-system runtime, not a web backend — no routing/DI/transaction/AOP container. |

### Auth

| Capability | Status | Verified at | Verified SHA | Issue | Cites | Notes |
|------------|--------|-------------|--------------|-------|-------|-------|
| `auth_coverage` | — `not_applicable` | — | — | — | — | Cats Effect is an effect-system runtime, not a web backend — no routing/DI/transaction/AOP container. |

### Validation

| Capability | Status | Verified at | Verified SHA | Issue | Cites | Notes |
|------------|--------|-------------|--------------|-------|-------|-------|
| `dto_extraction` | — `not_applicable` | — | — | — | — | Cats Effect is an effect-system runtime, not a web backend — no routing/DI/transaction/AOP container. |
| `request_validation` | — `not_applicable` | — | — | — | — | Cats Effect is an effect-system runtime, not a web backend — no routing/DI/transaction/AOP container. |

### Middleware

| Capability | Status | Verified at | Verified SHA | Issue | Cites | Notes |
|------------|--------|-------------|--------------|-------|-------|-------|
| `middleware_coverage` | — `not_applicable` | — | — | — | — | Cats Effect is an effect-system runtime, not a web backend — no routing/DI/transaction/AOP container. |

### Testing

| Capability | Status | Verified at | Verified SHA | Issue | Cites | Notes |
|------------|--------|-------------|--------------|-------|-------|-------|
| `tests_linkage` | ❌ `missing` | — | — | [link](backfill:dictionary-completeness) | — | — |

### Type System

| Capability | Status | Verified at | Verified SHA | Issue | Cites | Notes |
|------------|--------|-------------|--------------|-------|-------|-------|
| `enum_extraction` | ❌ `missing` | — | — | [link](backfill:dictionary-completeness) | — | — |
| `interface_extraction` | ❌ `missing` | — | — | [link](backfill:dictionary-completeness) | — | — |
| `type_alias_extraction` | ❌ `missing` | — | — | [link](backfill:dictionary-completeness) | — | — |
| `type_extraction` | ❌ `missing` | — | — | [link](backfill:dictionary-completeness) | — | — |

### DI

| Capability | Status | Verified at | Verified SHA | Issue | Cites | Notes |
|------------|--------|-------------|--------------|-------|-------|-------|
| `di_binding_extraction` | — `not_applicable` | — | — | — | — | Cats Effect is an effect-system runtime, not a web backend — no routing/DI/transaction/AOP container. |
| `di_injection_point` | — `not_applicable` | — | — | — | — | Cats Effect is an effect-system runtime, not a web backend — no routing/DI/transaction/AOP container. |
| `di_scope_resolution` | — `not_applicable` | — | — | — | — | Cats Effect is an effect-system runtime, not a web backend — no routing/DI/transaction/AOP container. |

### Transactions

| Capability | Status | Verified at | Verified SHA | Issue | Cites | Notes |
|------------|--------|-------------|--------------|-------|-------|-------|
| `transaction_boundary_extraction` | — `not_applicable` | — | — | — | — | Cats Effect is an effect-system runtime, not a web backend — no routing/DI/transaction/AOP container. |
| `transaction_propagation` | — `not_applicable` | — | — | — | — | Cats Effect is an effect-system runtime, not a web backend — no routing/DI/transaction/AOP container. |
| `transaction_rollback_rules` | — `not_applicable` | — | — | — | — | Cats Effect is an effect-system runtime, not a web backend — no routing/DI/transaction/AOP container. |

### AOP

| Capability | Status | Verified at | Verified SHA | Issue | Cites | Notes |
|------------|--------|-------------|--------------|-------|-------|-------|
| `advice_attribution` | — `not_applicable` | — | — | — | — | Cats Effect is an effect-system runtime, not a web backend — no routing/DI/transaction/AOP container. |
| `aspect_extraction` | — `not_applicable` | — | — | — | — | Cats Effect is an effect-system runtime, not a web backend — no routing/DI/transaction/AOP container. |
| `pointcut_resolution` | — `not_applicable` | — | — | — | — | Cats Effect is an effect-system runtime, not a web backend — no routing/DI/transaction/AOP container. |

### Observability

| Capability | Status | Verified at | Verified SHA | Issue | Cites | Notes |
|------------|--------|-------------|--------------|-------|-------|-------|
| `log_extraction` | ❌ `missing` | — | — | [link](backfill:dictionary-completeness) | — | — |
| `metric_extraction` | ❌ `missing` | — | — | [link](backfill:dictionary-completeness) | — | — |
| `trace_extraction` | ❌ `missing` | — | — | [link](backfill:dictionary-completeness) | — | — |

### Data

| Capability | Status | Verified at | Verified SHA | Issue | Cites | Notes |
|------------|--------|-------------|--------------|-------|-------|-------|

### Substrate

| Capability | Status | Verified at | Verified SHA | Issue | Cites | Notes |
|------------|--------|-------------|--------------|-------|-------|-------|
| `confidence_overlay` | ✅ `full` | `2026-05-28` | — | — | `internal/graph/graph.go`<br>`internal/mcp/tools.go`<br>`internal/types/confidence.go` | — |
| `constant_propagation` | ✅ `full` | `2026-05-27` | — | — | `internal/links/constant_propagation.go`<br>`internal/substrate/scala.go`<br>`internal/substrate/substrate.go` | — |
| `db_effect` | ⚠️ `partial` | `2026-05-28` | — | — | `internal/links/effect_propagation.go`<br>`internal/substrate/effect_sinks_scala.go` | — |
| `dead_code_detection` | ✅ `full` | `2026-05-28` | — | — | `internal/links/reachability.go`<br>`internal/mcp/dead_code.go`<br>`internal/substrate/entry_points.go`<br>`internal/substrate/entry_points_scala.go` | — |
| `def_use_chain_extraction` | ❌ `missing` | — | — | [link](backfill:dictionary-completeness) | — | — |
| `env_fallback_recognition` | ✅ `full` | `2026-05-27` | — | — | `internal/links/constant_propagation.go`<br>`internal/substrate/scala.go`<br>`internal/substrate/substrate.go` | — |
| `fs_effect` | ⚠️ `partial` | `2026-05-28` | — | — | `internal/links/effect_propagation.go`<br>`internal/substrate/effect_sinks_scala.go` | — |
| `http_effect` | ⚠️ `partial` | `2026-05-28` | — | — | `internal/links/effect_propagation.go`<br>`internal/substrate/effect_sinks_scala.go` | — |
| `import_resolution_quality` | ⚠️ `partial` | `2026-05-27` | — | — | `internal/links/constant_propagation.go`<br>`internal/substrate/scala.go`<br>`internal/substrate/substrate.go` | — |
| `module_cycle_detection` | ❌ `missing` | — | — | [link](backfill:dictionary-completeness) | — | — |
| `mutation_effect` | ⚠️ `partial` | `2026-05-28` | — | — | `internal/links/effect_propagation.go`<br>`internal/substrate/effect_sinks_scala.go` | — |
| `pure_function_tagging` | ❌ `missing` | — | — | [link](backfill:dictionary-completeness) | — | — |
| `reachability_analysis` | ✅ `full` | `2026-05-28` | — | — | `internal/links/reachability.go`<br>`internal/substrate/entry_points.go`<br>`internal/substrate/entry_points_scala.go` | — |
| `request_shape_extraction` | ✅ `full` | `2026-05-28` | — | [link](https://github.com/cajasmota/archigraph/issues/2771) | `internal/links/payload_drift.go`<br>`internal/mcp/payload_drift_tool.go`<br>`internal/substrate/payload_shapes.go`<br>`internal/substrate/payload_shapes_scala.go` | — |
| `response_shape_extraction` | ✅ `full` | `2026-05-28` | — | [link](https://github.com/cajasmota/archigraph/issues/2771) | `internal/links/payload_drift.go`<br>`internal/mcp/payload_drift_tool.go`<br>`internal/substrate/payload_shapes.go`<br>`internal/substrate/payload_shapes_scala.go` | — |
| `sanitizer_recognition` | ⚠️ `partial` | `2026-05-28` | — | — | `internal/links/taint_flow.go`<br>`internal/substrate/taint_sites_scala.go` | — |
| `schema_drift_detection` | ✅ `full` | `2026-05-28` | — | [link](https://github.com/cajasmota/archigraph/issues/2771) | `internal/links/payload_drift.go`<br>`internal/mcp/payload_drift_tool.go`<br>`internal/substrate/payload_shapes.go`<br>`internal/substrate/payload_shapes_scala.go` | — |
| `taint_sink_detection` | ⚠️ `partial` | `2026-05-28` | — | — | `internal/links/taint_flow.go`<br>`internal/substrate/taint_sites_scala.go` | — |
| `taint_source_detection` | ⚠️ `partial` | `2026-05-28` | — | — | `internal/links/taint_flow.go`<br>`internal/substrate/taint_sites_scala.go` | — |
| `template_pattern_catalog` | ❌ `missing` | — | — | [link](backfill:dictionary-completeness) | — | — |
| `vulnerability_finding` | ⚠️ `partial` | `2026-05-28` | — | — | `internal/links/taint_flow.go`<br>`internal/substrate/taint_sites_scala.go` | — |

## Provenance

This record is sourced from `docs/coverage/registry.json`. To update it, edit the JSON
(or use `go run ./tools/coverage update lang.scala.framework.cats-effect ...`) then regenerate:

```
go run ./tools/coverage validate
go run ./tools/coverage gen
```
