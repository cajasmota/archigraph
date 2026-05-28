<!-- DO NOT EDIT — generated from docs/coverage/registry.json by 'go run ./tools/coverage gen' -->
# `lang.jsts.framework.angular` — Angular

Auto-generated. Back to [summary](../summary.md).

- **Language:** [JS/TS](../by-language/jsts.md)
- **Category:** [http_framework](../by-category/http_framework.md)
- **Subcategory:** UI Frontend
- **Capability cells:** 18

## Capabilities


### Structure

| Capability | Status | Verified at | Verified SHA | Issue | Cites | Notes |
|------------|--------|-------------|--------------|-------|-------|-------|
| `component_extraction` | ✅ `full` | `2026-05-28` | — | — | `internal/extractors/javascript/angular.go`<br>`internal/extractors/javascript/extractor.go`<br>`internal/extractors/javascript/issue2854_angular_test.go` | — |
| `context_extraction` | ✅ `full` | `2026-05-28` | — | [link](https://github.com/cajasmota/archigraph/issues/2751) | `internal/extractors/javascript/angular.go`<br>`internal/extractors/javascript/extractor.go`<br>`internal/extractors/javascript/issue2854_angular_test.go` | — |
| `hoc_wrapper_recognition` | — `not_applicable` | `2026-05-28` | — | [link](https://github.com/cajasmota/archigraph/issues/2854) | — | — |
| `hook_recognition` | — `not_applicable` | — | — | — | — | — |
| `jsx_template` | — `not_applicable` | — | — | — | — | — |

### Data Flow

| Capability | Status | Verified at | Verified SHA | Issue | Cites | Notes |
|------------|--------|-------------|--------------|-------|-------|-------|
| `branch_conditions` | ✅ `full` | `2026-05-28` | — | [link](https://github.com/cajasmota/archigraph/issues/2855) | `internal/extractors/javascript/angular.go`<br>`internal/extractors/javascript/issue2855_angular_dataflow_test.go`<br>`testdata/fixtures/real-world/typescript/angular_dataflow_component.ts` | — |
| `data_fetching` | ✅ `full` | `2026-05-28` | — | [link](https://github.com/cajasmota/archigraph/issues/2855) | `internal/extractors/javascript/angular.go`<br>`internal/extractors/javascript/issue2855_angular_dataflow_test.go`<br>`testdata/fixtures/real-world/typescript/angular_dataflow_component.ts` | — |
| `prop_extraction` | ✅ `full` | `2026-05-28` | — | [link](https://github.com/cajasmota/archigraph/issues/2855) | `internal/extractors/javascript/angular.go`<br>`internal/extractors/javascript/issue2855_angular_dataflow_test.go`<br>`testdata/fixtures/real-world/typescript/angular_dataflow_component.ts` | — |
| `state_management` | ⚠️ `partial` | `2026-05-28` | — | [link](https://github.com/cajasmota/archigraph/issues/2855) | `internal/extractors/javascript/angular.go`<br>`internal/extractors/javascript/issue2855_angular_dataflow_test.go` | AUDIT(#2847) full->partial: angularStateManagement only detects ngrx Store (select/dispatch/pipe/selectSignal). On angular-realworld (gothinkster) all 11 stateful files use Angular signals + RxJS BehaviorSubject and 0 use ngrx, so the cell fired on the ngrx fixture but missed the dominant modern Angular state idiom. Follow-up filed for signal()/computed()/BehaviorSubject support. |

### Navigation

| Capability | Status | Verified at | Verified SHA | Issue | Cites | Notes |
|------------|--------|-------------|--------------|-------|-------|-------|
| `router_pattern` | ✅ `full` | `2026-05-28` | — | — | `internal/extractors/javascript/angular_nav_lifecycle.go`<br>`internal/extractors/javascript/issue2856_angular_test.go`<br>`testdata/fixtures/real-world/typescript/angular_nav_lifecycle_component.ts` | — |

### Type System

| Capability | Status | Verified at | Verified SHA | Issue | Cites | Notes |
|------------|--------|-------------|--------------|-------|-------|-------|
| `enum_extraction` | ✅ `full` | `2026-05-28` | — | — | `internal/extractors/javascript/extractor.go` | — |
| `interface_extraction` | ✅ `full` | `2026-05-28` | — | — | `internal/extractors/javascript/extractor.go` | — |
| `type_alias_extraction` | ✅ `full` | `2026-05-28` | — | — | `internal/extractors/javascript/extractor.go` | — |

### Lifecycle

| Capability | Status | Verified at | Verified SHA | Issue | Cites | Notes |
|------------|--------|-------------|--------------|-------|-------|-------|
| `state_setter_emission` | ✅ `full` | `2026-05-28` | — | [link](https://github.com/cajasmota/archigraph/issues/2751) | `internal/extractors/javascript/angular_nav_lifecycle.go`<br>`internal/extractors/javascript/issue2856_angular_test.go`<br>`testdata/fixtures/real-world/typescript/angular_nav_lifecycle_component.ts` | — |

### Testing

| Capability | Status | Verified at | Verified SHA | Issue | Cites | Notes |
|------------|--------|-------------|--------------|-------|-------|-------|
| `tests_linkage` | ✅ `full` | `2026-05-28` | — | — | `internal/extractors/javascript/tests.go` | — |

### Substrate

| Capability | Status | Verified at | Verified SHA | Issue | Cites | Notes |
|------------|--------|-------------|--------------|-------|-------|-------|
| `constant_propagation` | ✅ `full` | `2026-05-28` | — | — | `internal/links/constant_propagation.go`<br>`internal/substrate/jsts.go`<br>`internal/substrate/substrate.go` | — |
| `env_fallback_recognition` | ✅ `full` | `2026-05-28` | — | — | `internal/links/constant_propagation.go`<br>`internal/substrate/jsts.go`<br>`internal/substrate/substrate.go` | — |
| `import_resolution_quality` | ✅ `full` | `2026-05-28` | — | — | `internal/links/constant_propagation.go`<br>`internal/substrate/jsts.go`<br>`internal/substrate/markup_script.go`<br>`internal/substrate/substrate.go`<br>`internal/substrate/uimm_substrate_test.go`<br>`testdata/fixtures/typescript/substrate_angular/app.component.ts` | — |

## Framework-specific

### Angular Internals

| Capability | Status | Verified at | Verified SHA | Issue | Cites | Notes |
|------------|--------|-------------|--------------|-------|-------|-------|
| `decorator_recognition` | ✅ `full` | `2026-05-28` | — | [link](https://github.com/cajasmota/archigraph/issues/2847) | `internal/extractors/javascript/angular.go`<br>`internal/extractors/javascript/issue2854_angular_test.go` | AUDIT(#2847) taxonomy: angular.go angularClassDecorators emits component/service/directive/pipe/module subtypes. Verified on angular-realworld: angular_component x18, angular_service x6, angular_pipe x2, angular_directive x1. |
| `dependency_injection` | ✅ `full` | `2026-05-28` | — | [link](https://github.com/cajasmota/archigraph/issues/2847) | `internal/extractors/javascript/angular.go`<br>`internal/extractors/javascript/issue2854_angular_test.go` | AUDIT(#2847) taxonomy: constructor-DI -> INJECTED_INTO edges. Verified on angular-realworld (5 INJECTED_INTO->ArticleComponent etc.), incl. modern inject() function-DI. |
| `directive_recognition` | ✅ `full` | `2026-05-28` | — | [link](https://github.com/cajasmota/archigraph/issues/2847) | `internal/extractors/javascript/angular.go`<br>`internal/extractors/javascript/issue2854_angular_test.go` | AUDIT(#2847) NEW idiom cell: @Directive -> angular_directive subtype. Verified on angular-realworld + nativescript-ng. |
| `ngmodule_extraction` | ✅ `full` | `2026-05-28` | — | [link](https://github.com/cajasmota/archigraph/issues/2847) | `internal/extractors/javascript/angular.go`<br>`internal/extractors/javascript/issue2854_angular_test.go` | AUDIT(#2847) taxonomy: @NgModule -> angular_module subtype. Verified on real NativeScript-Angular app (angular_module x47). |
| `pipe_extraction` | ✅ `full` | `2026-05-28` | — | [link](https://github.com/cajasmota/archigraph/issues/2847) | `internal/extractors/javascript/angular.go`<br>`internal/extractors/javascript/issue2854_angular_test.go` | AUDIT(#2847) NEW idiom cell: @Pipe -> angular_pipe subtype. Verified on angular-realworld (angular_pipe x2). |
| `rxjs_pattern_detection` | ❌ `missing` | — | — | [link](https://github.com/cajasmota/archigraph/issues/2739) | — | AUDIT(#2847): genuinely unimplemented — angular.go does not extract Observable/pipe/subscribe operators as entities. Held at missing. |
| `service_extraction` | ✅ `full` | `2026-05-28` | — | [link](https://github.com/cajasmota/archigraph/issues/2847) | `internal/extractors/javascript/angular.go`<br>`internal/extractors/javascript/issue2854_angular_test.go` | AUDIT(#2847) NEW idiom cell: @Injectable -> angular_service subtype. Verified on angular-realworld (angular_service x6). |

## Provenance

This record is sourced from `docs/coverage/registry.json`. To update it, edit the JSON
(or use `go run ./tools/coverage update lang.jsts.framework.angular ...`) then regenerate:

```
go run ./tools/coverage validate
go run ./tools/coverage gen
```
