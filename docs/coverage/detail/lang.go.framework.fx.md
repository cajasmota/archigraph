<!-- DO NOT EDIT — generated from docs/coverage/registry.json by 'go run ./tools/coverage gen' -->
# `lang.go.framework.fx` — uber/fx (DI)

Auto-generated. Back to [summary](../summary.md).

- **Language:** [go](../by-language/go.md)
- **Category:** [http_framework](../by-category/http_framework.md)
- **Subcategory:** Backend HTTP
- **Capability cells:** 49

## Capabilities


### Routing

| Capability | Status | Verified at | Issue | Cites | Notes |
|------------|--------|-------------|-------|-------|-------|
| Endpoint deprecation versioning | 🔴 `missing` | — | 3628 | — | — |
| Endpoint pagination posture | 🔴 `missing` | `2026-06-02` | 3628 | `internal/engine/http_endpoint_pagination.go`<br>`internal/engine/http_endpoint_pagination_patterns.go`<br>`internal/engine/http_endpoint_pagination_test.go`<br>`internal/engine/http_endpoint_synthesis.go` | #3628: applyEndpointPagination stamps paginated/pagination_style/pagination_params via the cross-language parameters/parameter_schema fallback (limit+offset/page/cursor shape). No framework-specific pagination-class/ORM signal yet for this framework. |
| Endpoint response codes | 🔴 `missing` | — | 3818 | — | — |
| Endpoint synthesis | 🔴 `missing` | — | 3628 | — | — |
| Handler attribution | 🔴 `missing` | — | 3628 | — | — |
| Route extraction | 🔴 `missing` | — | 3628 | — | — |

### View

| Capability | Status | Verified at | Issue | Cites | Notes |
|------------|--------|-------------|-------|-------|-------|
| View rendering | 🔴 `missing` | — | view_rendering:#3628-not-yet-extracted | — | — |

### Auth

| Capability | Status | Verified at | Issue | Cites | Notes |
|------------|--------|-------------|-------|-------|-------|
| Auth coverage | 🔴 `missing` | — | 3628 | — | — |

### Validation

| Capability | Status | Verified at | Issue | Cites | Notes |
|------------|--------|-------------|-------|-------|-------|
| DTO extraction | 🔴 `missing` | — | 3628 | — | — |
| Request validation | 🔴 `missing` | — | 3628 | — | — |

### Middleware

| Capability | Status | Verified at | Issue | Cites | Notes |
|------------|--------|-------------|-------|-------|-------|
| Middleware coverage | 🔴 `missing` | — | 3628 | — | — |
| Rate limit stamping | 🔴 `missing` | — | [link](https://github.com/cajasmota/archigraph/issues/3778) | — | endpoint rate-limit / throttle stamping not yet implemented for this framework; the #3628 child shipped express-rate-limit (JS/TS) + slowapi/django-ratelimit/flask-limiter/DRF (Python). express-slow-down-compatible / framework-native limiters for this framework are future work. |

### Schema

| Capability | Status | Verified at | Issue | Cites | Notes |
|------------|--------|-------------|-------|-------|-------|
| Type graph extraction | — `not_applicable` | — | — | — | GraphQL schema type→type graph (object-typed field -> referenced object type with list/nullable cardinality) is a GraphQL-only concept; this framework is not a GraphQL server, so it has no GraphQL object-type relationship graph. |

### Type System

| Capability | Status | Verified at | Issue | Cites | Notes |
|------------|--------|-------------|-------|-------|-------|
| Enum extraction | 🔴 `missing` | — | 3628 | — | — |
| Interface extraction | 🔴 `missing` | — | 3628 | — | — |
| Type alias extraction | 🟢 `partial` | `2026-06-04` | 3628 | `internal/extractors/golang/extractor.go` | #3872: Go `type X = Y` alias declarations are lifted by the tree-sitter base Go extractor regardless of framework; a fx app's ordinary .go files carry such aliases and are extracted identically to gin/echo. PARTIAL (mirrors all Go siblings): framework runtime aliases are captured but not distinguished from user-defined ones; no fx-specific type-alias test. |
| Type extraction | 🔴 `missing` | — | 3628 | — | — |

### DI

| Capability | Status | Verified at | Issue | Cites | Notes |
|------------|--------|-------------|-------|-------|-------|
| DI binding extraction | 🟢 `partial` | `2026-06-02` | 3628 | `internal/custom/golang/di_graph.go`<br>`internal/custom/golang/di_graph_test.go` | uber/fx: constructors in fx.Provide(...) emit BINDS(constructor -> produced-type) via the shared Go provider pass (func NewService(...) *Service => BINDS NewService->Service). Value-asserted TestGoDI_FxProvide (NewService->Service). Negatives shared with wire (unresolved/unregistered/error-only). PARTIAL: fx.Annotate/ParamTags/ResultTags + value groups not modeled; cross-file return types unresolved (honest-partial). |
| DI injection point | 🟢 `partial` | `2026-06-02` | 3628 | `internal/custom/golang/di_graph.go`<br>`internal/custom/golang/di_graph_test.go` | uber/fx: an fx-provided constructors parameter types are injected into the produced type: func NewService(cfg *Config) *Service emits INJECTED_INTO(Config->Service). Value-asserted TestGoDI_FxProvide (Config->Service). PARTIAL: fx.Invoke target params + fx.In/fx.Out struct-tag injection not yet modeled. |
| DI scope resolution | — `not_applicable` | `2026-06-02` | — | — | uber/fx provides singletons within an App by construction; there are no per-binding scope annotations to resolve. Not_applicable. |

### Testing

| Capability | Status | Verified at | Issue | Cites | Notes |
|------------|--------|-------------|-------|-------|-------|
| Tests linkage | 🔴 `missing` | — | 3628 | — | — |

### Observability

| Capability | Status | Verified at | Issue | Cites | Notes |
|------------|--------|-------------|-------|-------|-------|
| Log extraction | 🔴 `missing` | — | 3628 | — | — |
| Metric extraction | 🔴 `missing` | — | 3628 | — | — |
| Trace extraction | 🔴 `missing` | — | 3628 | — | — |

### Data

| Capability | Status | Verified at | Issue | Cites | Notes |
|------------|--------|-------------|-------|-------|-------|
| DB effect | 🔴 `missing` | — | 3628 | — | — |

### Substrate

| Capability | Status | Verified at | Issue | Cites | Notes |
|------------|--------|-------------|-------|-------|-------|
| Confidence overlay | ✅ `full` | `2026-06-04` | — | `internal/graph/graph.go`<br>`internal/mcp/tools.go`<br>`internal/types/confidence.go` | #3872: the per-LANGUAGE sniffGo sniffer (Register("go")) gates only on file content with zero per-framework branching, so the graph-wide confidence overlay (#2769) consumes the SAME per-Binding Confidence for fx files as flagship siblings. Value-asserting test drives the uber-go/fx DI module (.go) idiom and asserts the EXACT Confidence (literal 1.0 / env-fallback 0.85 / cross-file import 0.6). |
| Config consumption | 🔴 `missing` | — | 3628 | — | — |
| Constant propagation | ✅ `full` | `2026-06-04` | — | `internal/links/constant_propagation.go`<br>`internal/substrate/golang.go`<br>`internal/substrate/substrate.go`<br>`internal/substrate/substrate_cap_gjj_sweep_test.go` | #3872: the framework-blind go sniffGo sniffer extracts top-level string literals regardless of framework; fx dispatches it identically. Test asserts the EXACT literal value (FxModuleName="http-server" literal) + ProvenanceLiteral + Confidence 1.0 on the uber-go/fx DI module (.go) idiom. |
| Dead code detection | 🟢 `partial` | `2026-06-04` | 3628 | `internal/links/reachability.go`<br>`internal/mcp/dead_code.go`<br>`internal/substrate/entry_points.go`<br>`internal/substrate/entry_points_golang.go` | #3872: dead-code identification is the whole-GRAPH Phase-1B reachability pass (reachability.go) with zero per-language code; an fx provider func is an ordinary Go entity, so one never reached from an entry-point is flagged a dead-code candidate exactly as for gin/echo. PARTIAL (mirrors all Go siblings): fx.Provide/fx.Invoke registration is DI-runtime reflection the static entry-point seeder does not model, so a provider/resolver reached only that way can be a false dead-code positive. |
| Def use chain extraction | 🟢 `partial` | `2026-06-04` | 3628 | `internal/links/def_use_pass.go`<br>`internal/substrate/def_use.go`<br>`internal/substrate/def_use_golang.go`<br>`internal/substrate/substrate_structural_gojava_wave1_test.go` | #3872 (verify-first): def_use_golang.go registers per-LANGUAGE via RegisterDefUseSniffer("go", …), .go→go file dispatch, zero framework refs. sniffDefUseGo extracts intra-procedural defs/uses and attributes them to the enclosing fx provider func via scanGoFuncHeaders. Proven by TestStructural_Go_Fx_DefUseAttributes (def+use of local `addr`/`srv` in fx provider NewServer). PARTIAL: standard local-binding chains; inter-procedural reaching-defs across the fx provider/wiring graph not modelled. |
| Env fallback recognition | ✅ `full` | `2026-06-04` | — | `internal/links/constant_propagation.go`<br>`internal/substrate/golang.go`<br>`internal/substrate/substrate.go`<br>`internal/substrate/substrate_cap_gjj_sweep_test.go` | #3872: the framework-blind go substrate sniffer recognises the env-fallback idiom regardless of framework; fx dispatches it identically. Test asserts the EXACT env-var name + default literal (FX_BIND_ADDR+default ":3000") + ProvenanceEnvFallback + Confidence 0.85 on the uber-go/fx DI module (.go) idiom. |
| Error flow | ✅ `full` | `2026-06-02` | 3628 | `internal/extractor/exception_flow.go`<br>`internal/extractors/golang/exception_flow.go`<br>`internal/extractors/golang/exception_flow_test.go` | return ErrX / fmt.Errorf %w -> THROWS; errors.Is/As -> CATCHES; named sentinels only (#3628) |
| Feature flag gating | 🔴 `missing` | — | 3628 | — | — |
| Fs effect | 🔴 `missing` | — | 3628 | — | — |
| HTTP effect | 🔴 `missing` | — | 3628 | — | — |
| Import resolution quality | 🟢 `partial` | `2026-06-04` | — | `internal/links/constant_propagation.go`<br>`internal/substrate/golang.go`<br>`internal/substrate/substrate.go`<br>`internal/substrate/substrate_cap_gjj_sweep_test.go` | #3872: the go cross-file import sniffer is framework-blind; fx dispatches it identically. PARTIAL (mirrors all siblings): single-segment binding, no transitive/re-export graph. Test asserts the EXACT ImportSource (go.uber.org/fx) + ProvenanceCrossFile + Confidence 0.6 on the uber-go/fx DI module (.go) idiom. |
| Module cycle detection | 🟢 `partial` | `2026-06-04` | 3628 | `internal/links/module_cycle_pass.go` | #3872: module-cycle detection is the whole-GRAPH module_cycle_pass over the Go IMPORTS edge graph; a fx app is composed of ≥2 ordinary Go packages with import edges, so import cycles among them are detected exactly as for gin/echo. PARTIAL (mirrors all Go siblings): package-level import cycles only; fx runtime/DI wiring is not an import edge and is out of scope. |
| Mutation effect | 🔴 `missing` | — | 3628 | — | — |
| Pure function tagging | 🟢 `partial` | `2026-06-04` | 3628 | `internal/links/effect_propagation.go`<br>`internal/links/pure_function_pass.go` | #3872: pure-function tagging is the whole-GRAPH Phase-3A pass (pure_function_pass.go) with zero per-language code — it tags any function-like entity the effect pass left effect-free. A fx func/resolver with no stamped effect is tagged a pure candidate exactly as for gin/echo handlers. PARTIAL (mirrors all Go siblings): tagging is absence-of-detected-effect, confidence floor 0.30, not a proof of purity. |
| Reachability analysis | 🟢 `partial` | `2026-06-04` | 3628 | `internal/links/reachability.go`<br>`internal/substrate/entry_points.go`<br>`internal/substrate/entry_points_golang.go` | #3872: reachability is the whole-GRAPH Phase-1B BFS from the Go entry-point set across CALLS/IMPORTS/etc; an fx provider func reached transitively from a Go main is marked reachable exactly as for gin/echo. PARTIAL (mirrors all Go siblings): fx.Provide/fx.Invoke registration is DI-runtime reflection the static seeder does not follow, so entities reached only that way can be under-reached. |
| Request shape extraction | 🔴 `missing` | — | 3628 | — | — |
| Request sink dataflow | 🔴 `missing` | — | 3628 | — | — |
| Response shape extraction | 🔴 `missing` | — | 3628 | — | — |
| Sanitizer recognition | 🔴 `missing` | — | 3628 | — | — |
| Schema drift detection | 🟢 `partial` | `2026-06-04` | — | `internal/links/payload_drift.go`<br>`internal/mcp/payload_drift_tool.go`<br>`internal/substrate/payload_shapes.go`<br>`internal/substrate/payload_shapes_golang.go` | #3872: the framework-agnostic payload-drift pass dispatches sniffPayloadShapesGolang by LANGUAGE slug (LanguageForPath->PayloadShapeSnifferFor), so fx producer/consumer shapes feed the same drift join as siblings. PARTIAL (mirrors siblings): no fx-specific drift fixture asserted end-to-end yet. |
| Taint sink detection | 🔴 `missing` | — | 3628 | — | — |
| Taint source detection | 🔴 `missing` | — | 3628 | — | — |
| Template pattern catalog | 🟢 `partial` | — | — | `internal/links/template_pattern_pass.go`<br>`internal/substrate/template_pattern_golang.go` | #3872: sniffTemplatePatternsGolang is registered on the go language slug and gates only on file content (no per-framework branch), so fx dispatches it identically. PARTIAL: mirrors all siblings. |
| Vulnerability finding | 🔴 `missing` | — | 3628 | — | — |

## Provenance

This record is sourced from `docs/coverage/registry.json`. To update it, edit the JSON
(or use `go run ./tools/coverage update lang.go.framework.fx ...`) then regenerate:

```
go run ./tools/coverage validate
go run ./tools/coverage gen
```
