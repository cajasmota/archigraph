<!-- DO NOT EDIT — generated from docs/coverage/registry.json by 'go run ./tools/coverage gen' -->
# `lang.java.framework.struts` — Apache Struts

Auto-generated. Back to [summary](../summary.md).

- **Language:** [java](../by-language/java.md)
- **Category:** [http_framework](../by-category/http_framework.md)
- **Subcategory:** JVM Backend
- **Capability cells:** 45

## Capabilities


### Routing

| Capability | Status | Verified at | Verified SHA | Issue | Cites | Notes |
|------------|--------|-------------|--------------|-------|-------|-------|
| `endpoint_synthesis` | ⚠️ `partial` | `2026-05-28` | — | — | `internal/engine/rules/java/frameworks/apache_struts.yaml` | — |
| `handler_attribution` | ⚠️ `partial` | `2026-05-28` | — | — | `internal/engine/rules/java/frameworks/apache_struts.yaml` | — |
| `route_extraction` | ❌ `missing` | — | — | [link](backfill:dictionary-completeness) | — | — |

### Auth

| Capability | Status | Verified at | Verified SHA | Issue | Cites | Notes |
|------------|--------|-------------|--------------|-------|-------|-------|
| `auth_coverage` | ❌ `missing` | — | — | — | — | — |

### Validation

| Capability | Status | Verified at | Verified SHA | Issue | Cites | Notes |
|------------|--------|-------------|--------------|-------|-------|-------|
| `dto_extraction` | ❌ `missing` | — | — | [link](backfill:dictionary-completeness) | — | — |
| `request_validation` | ❌ `missing` | — | — | [link](backfill:dictionary-completeness) | — | — |

### Middleware

| Capability | Status | Verified at | Verified SHA | Issue | Cites | Notes |
|------------|--------|-------------|--------------|-------|-------|-------|
| `middleware_coverage` | ❌ `missing` | — | — | — | — | — |

### Testing

| Capability | Status | Verified at | Verified SHA | Issue | Cites | Notes |
|------------|--------|-------------|--------------|-------|-------|-------|
| `tests_linkage` | ❌ `missing` | — | — | [link](backfill:dictionary-completeness) | — | — |

### Type System

| Capability | Status | Verified at | Verified SHA | Issue | Cites | Notes |
|------------|--------|-------------|--------------|-------|-------|-------|
| `enum_extraction` | ✅ `full` | `2026-05-28` | — | — | `internal/extractors/java/java.go` | — |
| `interface_extraction` | ✅ `full` | `2026-05-28` | — | — | `internal/extractors/java/java.go` | — |
| `type_alias_extraction` | ❌ `missing` | — | — | [link](backfill:dictionary-completeness) | — | — |
| `type_extraction` | ✅ `full` | `2026-05-28` | — | — | `internal/extractors/java/java.go` | — |

### DI

| Capability | Status | Verified at | Verified SHA | Issue | Cites | Notes |
|------------|--------|-------------|--------------|-------|-------|-------|
| `di_binding_extraction` | ❌ `missing` | — | — | [link](backfill:dictionary-completeness) | — | — |
| `di_injection_point` | ❌ `missing` | — | — | [link](backfill:dictionary-completeness) | — | — |
| `di_scope_resolution` | ❌ `missing` | — | — | [link](backfill:dictionary-completeness) | — | — |

### Transactions

| Capability | Status | Verified at | Verified SHA | Issue | Cites | Notes |
|------------|--------|-------------|--------------|-------|-------|-------|
| `transaction_boundary_extraction` | ❌ `missing` | — | — | [link](backfill:dictionary-completeness) | — | — |
| `transaction_propagation` | ❌ `missing` | — | — | [link](backfill:dictionary-completeness) | — | — |
| `transaction_rollback_rules` | ❌ `missing` | — | — | [link](backfill:dictionary-completeness) | — | — |

### AOP

| Capability | Status | Verified at | Verified SHA | Issue | Cites | Notes |
|------------|--------|-------------|--------------|-------|-------|-------|
| `advice_attribution` | ❌ `missing` | — | — | [link](backfill:dictionary-completeness) | — | — |
| `aspect_extraction` | ❌ `missing` | — | — | [link](backfill:dictionary-completeness) | — | — |
| `pointcut_resolution` | ❌ `missing` | — | — | [link](backfill:dictionary-completeness) | — | — |

### Observability

| Capability | Status | Verified at | Verified SHA | Issue | Cites | Notes |
|------------|--------|-------------|--------------|-------|-------|-------|
| `log_extraction` | ❌ `missing` | — | — | [link](backfill:dictionary-completeness) | — | — |
| `metric_extraction` | ❌ `missing` | — | — | [link](backfill:dictionary-completeness) | — | — |
| `trace_extraction` | ❌ `missing` | — | — | [link](backfill:dictionary-completeness) | — | — |

### Data

| Capability | Status | Verified at | Verified SHA | Issue | Cites | Notes |
|------------|--------|-------------|--------------|-------|-------|-------|
| `db_effect` | ❌ `missing` | — | — | [link](backfill:dictionary-completeness) | — | — |

### Substrate

| Capability | Status | Verified at | Verified SHA | Issue | Cites | Notes |
|------------|--------|-------------|--------------|-------|-------|-------|
| `confidence_overlay` | ❌ `missing` | — | — | [link](backfill:dictionary-completeness) | — | — |
| `constant_propagation` | ✅ `full` | `2026-05-28` | — | — | `internal/links/constant_propagation.go`<br>`internal/substrate/java.go`<br>`internal/substrate/substrate.go` | — |
| `dead_code_detection` | ❌ `missing` | — | — | [link](backfill:dictionary-completeness) | — | — |
| `def_use_chain_extraction` | ❌ `missing` | — | — | [link](backfill:dictionary-completeness) | — | — |
| `env_fallback_recognition` | ✅ `full` | `2026-05-28` | — | — | `internal/links/constant_propagation.go`<br>`internal/substrate/java.go`<br>`internal/substrate/substrate.go` | — |
| `fs_effect` | ❌ `missing` | — | — | [link](backfill:dictionary-completeness) | — | — |
| `http_effect` | ❌ `missing` | — | — | [link](backfill:dictionary-completeness) | — | — |
| `import_resolution_quality` | ⚠️ `partial` | `2026-05-28` | — | — | `internal/links/constant_propagation.go`<br>`internal/substrate/java.go`<br>`internal/substrate/substrate.go` | — |
| `module_cycle_detection` | ❌ `missing` | — | — | [link](backfill:dictionary-completeness) | — | — |
| `mutation_effect` | ❌ `missing` | — | — | [link](backfill:dictionary-completeness) | — | — |
| `pure_function_tagging` | ❌ `missing` | — | — | [link](backfill:dictionary-completeness) | — | — |
| `reachability_analysis` | ❌ `missing` | — | — | [link](backfill:dictionary-completeness) | — | — |
| `request_shape_extraction` | ✅ `full` | `2026-05-27` | — | — | `internal/links/payload_drift.go`<br>`internal/mcp/payload_drift_tool.go`<br>`internal/substrate/payload_shapes.go`<br>`internal/substrate/payload_shapes_java.go` | — |
| `response_shape_extraction` | ✅ `full` | `2026-05-27` | — | — | `internal/links/payload_drift.go`<br>`internal/mcp/payload_drift_tool.go`<br>`internal/substrate/payload_shapes.go`<br>`internal/substrate/payload_shapes_java.go` | — |
| `sanitizer_recognition` | ❌ `missing` | — | — | [link](backfill:dictionary-completeness) | — | — |
| `schema_drift_detection` | ✅ `full` | `2026-05-27` | — | — | `internal/links/payload_drift.go`<br>`internal/mcp/payload_drift_tool.go`<br>`internal/substrate/payload_shapes.go`<br>`internal/substrate/payload_shapes_java.go` | — |
| `taint_sink_detection` | ❌ `missing` | — | — | [link](backfill:dictionary-completeness) | — | — |
| `taint_source_detection` | ❌ `missing` | — | — | [link](backfill:dictionary-completeness) | — | — |
| `template_pattern_catalog` | ❌ `missing` | — | — | [link](backfill:dictionary-completeness) | — | — |
| `vulnerability_finding` | ❌ `missing` | — | — | [link](backfill:dictionary-completeness) | — | — |

## Provenance

This record is sourced from `docs/coverage/registry.json`. To update it, edit the JSON
(or use `go run ./tools/coverage update lang.java.framework.struts ...`) then regenerate:

```
go run ./tools/coverage validate
go run ./tools/coverage gen
```
