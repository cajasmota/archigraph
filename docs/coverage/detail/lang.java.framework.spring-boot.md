<!-- DO NOT EDIT — generated from docs/coverage/registry.json by 'go run ./tools/coverage gen' -->
# `lang.java.framework.spring-boot` — Spring Boot / Spring MVC

Auto-generated. Back to [summary](../summary.md).

- **Language:** [java](../by-language/java.md)
- **Category:** [http_framework](../by-category/http_framework.md)
- **Subcategory:** JVM Backend
- **Capability cells:** 45

## Capabilities


### Routing

| Capability | Status | Verified at | Verified SHA | Issue | Cites | Notes |
|------------|--------|-------------|--------------|-------|-------|-------|
| `endpoint_synthesis` | ✅ `full` | `2026-05-28` | — | — | `internal/engine/http_endpoint_synthesis.go`<br>`internal/engine/rules/java/frameworks/spring_boot.yaml`<br>`internal/engine/rules/java/frameworks/spring_mvc.yaml`<br>`internal/engine/spring_routes.go` | — |
| `handler_attribution` | ✅ `full` | `2026-05-28` | — | — | `internal/engine/java_annotation_routes.go`<br>`internal/engine/spring_routes.go` | — |
| `route_extraction` | ❌ `missing` | — | — | [link](backfill:dictionary-completeness) | — | — |

### Auth

| Capability | Status | Verified at | Verified SHA | Issue | Cites | Notes |
|------------|--------|-------------|--------------|-------|-------|-------|
| `auth_coverage` | ✅ `full` | `2026-05-28` | — | — | `internal/engine/java_auth_policy.go` | — |

### Validation

| Capability | Status | Verified at | Verified SHA | Issue | Cites | Notes |
|------------|--------|-------------|--------------|-------|-------|-------|
| `dto_extraction` | ❌ `missing` | — | — | [link](backfill:dictionary-completeness) | — | — |
| `request_validation` | ❌ `missing` | — | — | [link](backfill:dictionary-completeness) | — | — |

### Middleware

| Capability | Status | Verified at | Verified SHA | Issue | Cites | Notes |
|------------|--------|-------------|--------------|-------|-------|-------|
| `middleware_coverage` | ⚠️ `partial` | `2026-05-28` | — | — | `internal/engine/java_annotation_params.go` | — |

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
| `di_binding_extraction` | ⚠️ `partial` | `2026-05-28` | — | [link](backfill:dictionary-completeness) | `internal/custom/java/spring_boot.go` | — |
| `di_injection_point` | ⚠️ `partial` | `2026-05-28` | — | [link](backfill:dictionary-completeness) | `internal/custom/java/spring_boot.go` | — |
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

### Substrate

| Capability | Status | Verified at | Verified SHA | Issue | Cites | Notes |
|------------|--------|-------------|--------------|-------|-------|-------|
| `confidence_overlay` | ❌ `missing` | — | — | [link](backfill:dictionary-completeness) | — | — |
| `constant_propagation` | ✅ `full` | `2026-05-28` | — | — | `internal/links/constant_propagation.go`<br>`internal/substrate/java.go`<br>`internal/substrate/substrate.go` | — |
| `db_effect` | ⚠️ `partial` | `2026-05-28` | — | — | `internal/links/effect_propagation.go`<br>`internal/substrate/effect_sinks_java.go` | — |
| `dead_code_detection` | ✅ `full` | `2026-05-28` | — | — | `internal/links/reachability.go`<br>`internal/mcp/dead_code.go`<br>`internal/substrate/entry_points.go`<br>`internal/substrate/entry_points_java.go` | — |
| `def_use_chain_extraction` | ⚠️ `partial` | `2026-05-28` | — | — | `internal/links/def_use_pass.go`<br>`internal/substrate/def_use.go`<br>`internal/substrate/def_use_java.go` | — |
| `env_fallback_recognition` | ✅ `full` | `2026-05-28` | — | — | `internal/links/constant_propagation.go`<br>`internal/substrate/java.go`<br>`internal/substrate/substrate.go` | — |
| `fs_effect` | ⚠️ `partial` | `2026-05-28` | — | — | `internal/links/effect_propagation.go`<br>`internal/substrate/effect_sinks_java.go` | — |
| `http_effect` | ⚠️ `partial` | `2026-05-28` | — | — | `internal/links/effect_propagation.go`<br>`internal/substrate/effect_sinks_java.go` | — |
| `import_resolution_quality` | ⚠️ `partial` | `2026-05-28` | — | — | `internal/links/constant_propagation.go`<br>`internal/substrate/java.go`<br>`internal/substrate/substrate.go` | — |
| `module_cycle_detection` | ⚠️ `partial` | `2026-05-28` | — | — | `internal/links/module_cycle_pass.go` | — |
| `mutation_effect` | ⚠️ `partial` | `2026-05-28` | — | — | `internal/links/effect_propagation.go`<br>`internal/substrate/effect_sinks_java.go` | — |
| `pure_function_tagging` | ⚠️ `partial` | `2026-05-28` | — | — | `internal/links/effect_propagation.go`<br>`internal/links/pure_function_pass.go` | — |
| `reachability_analysis` | ✅ `full` | `2026-05-28` | — | — | `internal/links/reachability.go`<br>`internal/substrate/entry_points.go`<br>`internal/substrate/entry_points_java.go` | — |
| `request_shape_extraction` | ✅ `full` | `2026-05-27` | — | — | `internal/links/payload_drift.go`<br>`internal/mcp/payload_drift_tool.go`<br>`internal/substrate/payload_shapes.go`<br>`internal/substrate/payload_shapes_java.go` | — |
| `response_shape_extraction` | ✅ `full` | `2026-05-27` | — | — | `internal/links/payload_drift.go`<br>`internal/mcp/payload_drift_tool.go`<br>`internal/substrate/payload_shapes.go`<br>`internal/substrate/payload_shapes_java.go` | — |
| `sanitizer_recognition` | ⚠️ `partial` | `2026-05-28` | — | — | `internal/links/taint_flow.go`<br>`internal/substrate/taint_sites_java.go` | — |
| `schema_drift_detection` | ✅ `full` | `2026-05-27` | — | — | `internal/links/payload_drift.go`<br>`internal/mcp/payload_drift_tool.go`<br>`internal/substrate/payload_shapes.go`<br>`internal/substrate/payload_shapes_java.go` | — |
| `taint_sink_detection` | ⚠️ `partial` | `2026-05-28` | — | — | `internal/links/taint_flow.go`<br>`internal/substrate/taint_sites_java.go` | — |
| `taint_source_detection` | ⚠️ `partial` | `2026-05-28` | — | — | `internal/links/taint_flow.go`<br>`internal/substrate/taint_sites_java.go` | — |
| `template_pattern_catalog` | ⚠️ `partial` | `2026-05-28` | — | — | `internal/links/template_pattern_pass.go`<br>`internal/substrate/template_pattern.go`<br>`internal/substrate/template_pattern_java.go` | — |
| `vulnerability_finding` | ⚠️ `partial` | `2026-05-28` | — | — | `internal/links/taint_flow.go`<br>`internal/substrate/taint_sites_java.go` | — |

## Framework-specific

### Spring Boot Internals

| Capability | Status | Verified at | Verified SHA | Issue | Cites | Notes |
|------------|--------|-------------|--------------|-------|-------|-------|
| `actuator_detection` | ❌ `missing` | — | — | [link](https://github.com/cajasmota/archigraph/issues/2739) | — | — |
| `autoconfiguration_detection` | ❌ `missing` | — | — | [link](https://github.com/cajasmota/archigraph/issues/2739) | — | — |
| `profile_detection` | ❌ `missing` | — | — | [link](https://github.com/cajasmota/archigraph/issues/2739) | — | — |

## Provenance

This record is sourced from `docs/coverage/registry.json`. To update it, edit the JSON
(or use `go run ./tools/coverage update lang.java.framework.spring-boot ...`) then regenerate:

```
go run ./tools/coverage validate
go run ./tools/coverage gen
```
