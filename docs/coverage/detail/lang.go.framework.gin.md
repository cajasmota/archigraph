<!-- DO NOT EDIT — generated from docs/coverage/registry.json by 'go run ./tools/coverage gen' -->
# `lang.go.framework.gin` — Gin

Auto-generated. Back to [summary](../summary.md).

- **Language:** [go](../by-language/go.md)
- **Category:** [http_framework](../by-category/http_framework.md)
- **Subcategory:** Backend HTTP
- **Capability cells:** 43

## Capabilities


### Routing

| Capability | Status | Verified at | Issue | Cites | Notes |
|------------|--------|-------------|-------|-------|-------|
| Endpoint synthesis | ✅ `full` | `2026-05-28` | — | `internal/engine/go_routes.go`<br>`internal/engine/rules/go/frameworks/gin.yaml` | — |
| Handler attribution | ✅ `full` | `2026-05-28` | — | `internal/engine/go_routes.go` | — |
| Route extraction | 🟢 `partial` | `2026-05-29` | backfill:dictionary-completeness | `internal/custom/golang/extractors_test.go`<br>`internal/custom/golang/gin.go` | regex-based: direct .GET/.POST/.DELETE/.PATCH etc + .Group()/.Route() prefix resolution tested; misses cross-file route splits, dynamic path construction, indirect router variable aliasing, and conditional registration |

### Auth

| Capability | Status | Verified at | Issue | Cites | Notes |
|------------|--------|-------------|-------|-------|-------|
| Auth coverage | ✅ `full` | `2026-06-02` | — | `internal/custom/golang/helpers.go`<br>`internal/custom/golang/middleware_auth_test.go`<br>`internal/custom/golang/route_auth.go`<br>`internal/custom/golang/route_auth_test.go` | Two layers. (1) #3734 endpoint-protection stamping (route_auth.go): binds auth middleware to the route SCOPE.Operation op and stamps the #3696 flat contract (auth_required/auth_method=middleware/auth_guard/auth_kind/auth_confidence). Group-level (authorized := r.Group('/', AuthRequired()) -> routes on the group var auth_required, HIGH), inline route middleware (r.GET('/admin', JWTAuth(), h) -> that route auth_required, HIGH, kind from classifyAuthMiddleware), engine-wide .Use(jwt.New()) -> MEDIUM inheritance. Value-asserting tests TestGinGroupAuth/TestGinInlineRouteAuth/TestGinEngineWideAuth + negatives (unprotected /health,/public not stamped). (2) #3213 .Use() chain pattern catalog (jwt/oauth/basic/session/rbac/api_key/auth) for middleware Pattern entities. Honest-partial: dynamic/conditional middleware and roles/scopes from opaque guards not modelled. |

### Validation

| Capability | Status | Verified at | Issue | Cites | Notes |
|------------|--------|-------------|-------|-------|-------|
| DTO extraction | ✅ `full` | `2026-06-02` | — | `internal/custom/golang/dto.go`<br>`internal/custom/golang/dto_edges_test.go` | dto.go now emits traversable endpoint→DTO edges (#3629/#3607): each resolved request-bind site (c.ShouldBindJSON/Bind/BindJSON(&x)) emits ACCEPTS_INPUT → Class:<Struct> and each resolved response serialise (c.JSON(code, x)) emits RETURNS → Class:<Struct>, where the bound var resolves to a file-local struct. Unresolved bind targets still emit a DTO entity but NO edge (honest-partial — never point at an unknown type). Previously gin/echo emitted DTO struct entities but no endpoint→DTO edges; now expand/traces/payload_drift can follow them. Tests: TestGoDTOEdge_GinAcceptsInput (ShouldBindJSON(&LoginReq) → ACCEPTS_INPUT Class:LoginReq), TestGoDTOEdge_GinReturns (c.JSON(200,resp) → RETURNS Class:UserResp), TestGoDTOEdge_EchoAcceptsInput, negatives TestGoDTOEdge_UnresolvedNoEdge / TestGoDTOEdge_NoFrameworkNoEdge. |
| Request validation | 🟢 `partial` | `2026-05-29` | [link](https://github.com/cajasmota/archigraph/issues/3213) | `internal/custom/golang/helpers.go` | binding call sites captured (c.ShouldBindJSON/BindJSON/Bind etc); struct-tag validation chain (go-playground/validator binding:"required" tags) not analyzed; no data-flow tracing of validated vs unvalidated paths |

### Middleware

| Capability | Status | Verified at | Issue | Cites | Notes |
|------------|--------|-------------|-------|-------|-------|
| Middleware coverage | ✅ `full` | `2026-05-30` | — | `internal/custom/golang/helpers.go`<br>`internal/custom/golang/middleware_auth_test.go` | balanced .Use(...) chain parser with paren-depth tracking; one SCOPE.Pattern per middleware in registration order (mw_order 0,1,2,...); string-literal mount prefix skipped; fixture-driven tests per framework |

### Type System

| Capability | Status | Verified at | Issue | Cites | Notes |
|------------|--------|-------------|-------|-------|-------|
| Enum extraction | — `not_applicable` | `2026-05-29` | — | — | Go has no first-class enum keyword; the idiom is const(...iota). The Go extractor extracts no const/iota enum constructs, so this capability is not applicable. |
| Interface extraction | ✅ `full` | `2026-05-29` | — | `internal/extractors/golang/extractor.go`<br>`internal/extractors/golang/extractor_test.go` | — |
| Type alias extraction | 🟢 `partial` | `2026-05-29` | — | `internal/extractors/golang/extractor.go` | type X = Y alias declarations via tree-sitter base extractor; framework-specific type aliases (e.g. gin.HandlerFunc, echo.HandlerFunc) captured but not distinguished from user-defined aliases; no value-asserting framework-specific tests |
| Type extraction | ✅ `full` | `2026-05-29` | — | `internal/extractors/golang/extractor.go`<br>`internal/extractors/golang/extractor_test.go` | — |

### DI

| Capability | Status | Verified at | Issue | Cites | Notes |
|------------|--------|-------------|-------|-------|-------|
| DI binding extraction | 🔴 `missing` | — | 3628 | — | — |
| DI injection point | 🔴 `missing` | — | 3628 | — | — |
| DI scope resolution | 🔴 `missing` | — | 3628 | — | — |

### Testing

| Capability | Status | Verified at | Issue | Cites | Notes |
|------------|--------|-------------|-------|-------|-------|
| Tests linkage | 🟢 `partial` | `2026-05-29` | backfill:dictionary-completeness | `internal/extractors/cross/testmap/extractor.go`<br>`internal/extractors/cross/testmap/frameworks.go`<br>`internal/extractors/cross/testmap/resolver.go` | Go test file + Test* func detection via tree-sitter base extractor; handler→test edge resolution and functional coverage mapping not implemented |

### Observability

| Capability | Status | Verified at | Issue | Cites | Notes |
|------------|--------|-------------|-------|-------|-------|
| Log extraction | 🟢 `partial` | `2026-05-30` | — | `internal/custom/golang/observability.go`<br>`internal/custom/golang/observability_test.go` | heuristic: logrus.New/WithFields, zap.NewProduction/New, slog.New/With, zerolog.New setup calls detected; does not trace log fields to handler context or correlate log entries to specific routes |
| Metric extraction | 🟢 `partial` | `2026-05-30` | — | `internal/custom/golang/observability.go`<br>`internal/custom/golang/observability_test.go` | prometheus.NewCounter(Vec)/NewHistogram(Vec)/NewGauge(Vec)/NewSummary(Vec) + promauto.NewXxx declarations detected; metric Name: field extracted when adjacent; does not track Observe/Add/Inc call sites or bind metrics to routes |
| Trace extraction | ✅ `full` | `2026-05-30` | — | `internal/custom/golang/observability.go`<br>`internal/custom/golang/observability_test.go` | — |

### Data

| Capability | Status | Verified at | Issue | Cites | Notes |
|------------|--------|-------------|-------|-------|-------|

### Substrate

| Capability | Status | Verified at | Issue | Cites | Notes |
|------------|--------|-------------|-------|-------|-------|
| Confidence overlay | ✅ `full` | `2026-05-29` | — | `internal/graph/graph.go`<br>`internal/mcp/tools.go`<br>`internal/types/confidence.go` | — |
| Config consumption | ✅ `full` | `2026-06-02` | 3641 | `internal/extractor/config_key.go`<br>`internal/extractors/golang/config_consumer.go`<br>`internal/extractors/golang/config_consumer_test.go` | os.Getenv/viper.GetString -> config:<key> DEPENDS_ON_CONFIG edges (issue #3641) |
| Constant propagation | ✅ `full` | `2026-05-28` | — | `internal/links/constant_propagation.go`<br>`internal/substrate/golang.go`<br>`internal/substrate/substrate.go` | — |
| DB effect | 🟢 `partial` | `2026-05-28` | — | `internal/links/effect_propagation.go`<br>`internal/substrate/effect_sinks_golang.go` | — |
| Dead code detection | 🟢 `partial` | `2026-05-28` | — | `internal/links/reachability.go`<br>`internal/mcp/dead_code.go`<br>`internal/substrate/entry_points.go`<br>`internal/substrate/entry_points_golang.go` | — |
| Def use chain extraction | 🟢 `partial` | `2026-05-28` | — | `internal/links/def_use_pass.go`<br>`internal/substrate/def_use.go`<br>`internal/substrate/def_use_golang.go` | — |
| Env fallback recognition | ✅ `full` | `2026-05-28` | — | `internal/links/constant_propagation.go`<br>`internal/substrate/golang.go`<br>`internal/substrate/substrate.go` | — |
| Error flow | ✅ `full` | `2026-06-02` | 3628 | `internal/extractor/exception_flow.go`<br>`internal/extractors/golang/exception_flow.go`<br>`internal/extractors/golang/exception_flow_test.go` | return ErrX / fmt.Errorf %w -> THROWS; errors.Is/As -> CATCHES; named sentinels only (#3628) |
| Feature flag gating | ✅ `full` | `2026-06-02` | 3706 | `internal/engine/feature_flag_edges.go`<br>`internal/engine/feature_flag_edges_test.go`<br>`internal/engine/orm_queries.go` | flag-check call sites -> feature:<key> + GATED_BY edge (LaunchDarkly/Unleash/Unleash-React/OpenFeature/Flipper/Flagsmith/Split.io/generic) |
| Fs effect | 🟢 `partial` | `2026-05-28` | — | `internal/links/effect_propagation.go`<br>`internal/substrate/effect_sinks_golang.go` | — |
| HTTP effect | 🟢 `partial` | `2026-05-28` | — | `internal/links/effect_propagation.go`<br>`internal/substrate/effect_sinks_golang.go` | — |
| Import resolution quality | 🟢 `partial` | `2026-05-28` | — | `internal/links/constant_propagation.go`<br>`internal/substrate/golang.go`<br>`internal/substrate/substrate.go` | — |
| Module cycle detection | 🟢 `partial` | `2026-05-28` | — | `internal/links/module_cycle_pass.go` | — |
| Mutation effect | 🟢 `partial` | `2026-05-28` | — | `internal/links/effect_propagation.go`<br>`internal/substrate/effect_sinks_golang.go` | — |
| Pure function tagging | 🟢 `partial` | `2026-05-28` | — | `internal/links/effect_propagation.go`<br>`internal/links/pure_function_pass.go` | — |
| Reachability analysis | 🟢 `partial` | `2026-05-28` | — | `internal/links/reachability.go`<br>`internal/substrate/entry_points.go`<br>`internal/substrate/entry_points_golang.go` | — |
| Request shape extraction | ✅ `full` | `2026-05-27` | — | `internal/links/payload_drift.go`<br>`internal/mcp/payload_drift_tool.go`<br>`internal/substrate/payload_shapes.go`<br>`internal/substrate/payload_shapes_golang.go` | — |
| Request sink dataflow | 🔴 `missing` | — | 3740 | — | — |
| Response shape extraction | ✅ `full` | `2026-05-27` | — | `internal/links/payload_drift.go`<br>`internal/mcp/payload_drift_tool.go`<br>`internal/substrate/payload_shapes.go`<br>`internal/substrate/payload_shapes_golang.go` | — |
| Sanitizer recognition | 🟢 `partial` | `2026-05-28` | — | `internal/links/taint_flow.go`<br>`internal/substrate/taint_sites_golang.go` | — |
| Schema drift detection | ✅ `full` | `2026-05-27` | — | `internal/links/payload_drift.go`<br>`internal/mcp/payload_drift_tool.go`<br>`internal/substrate/payload_shapes.go`<br>`internal/substrate/payload_shapes_golang.go` | — |
| Taint sink detection | 🟢 `partial` | `2026-05-28` | — | `internal/links/taint_flow.go`<br>`internal/substrate/taint_sites_golang.go` | — |
| Taint source detection | 🟢 `partial` | `2026-05-28` | — | `internal/links/taint_flow.go`<br>`internal/substrate/taint_sites_golang.go` | — |
| Template pattern catalog | 🟢 `partial` | `2026-05-28` | — | `internal/links/template_pattern_pass.go`<br>`internal/substrate/template_pattern.go`<br>`internal/substrate/template_pattern_golang.go` | — |
| Vulnerability finding | 🟢 `partial` | `2026-05-28` | — | `internal/links/taint_flow.go`<br>`internal/substrate/taint_sites_golang.go` | — |

## Provenance

This record is sourced from `docs/coverage/registry.json`. To update it, edit the JSON
(or use `go run ./tools/coverage update lang.go.framework.gin ...`) then regenerate:

```
go run ./tools/coverage validate
go run ./tools/coverage gen
```
