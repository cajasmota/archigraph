<!-- DO NOT EDIT — generated from docs/coverage/registry.json by 'go run ./tools/coverage gen' -->
# `lang.elixir.framework.bandit` — Bandit

Auto-generated. Back to [summary](../summary.md).

- **Language:** [elixir](../by-language/elixir.md)
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
| Endpoint synthesis | — `not_applicable` | — | — | — | Bandit has no endpoint/route concept; it is a transport adapter |
| Handler attribution | 🟢 `partial` | — | — | `internal/engine/rules/elixir/frameworks/absinthe.yaml`<br>`internal/substrate/entry_points_elixir.go` | Bandit delegates to Plug-compatible handlers; Plug.call/2 attribution tracked via entry-point sniffer |
| Route extraction | — `not_applicable` | — | — | — | Bandit is a low-level HTTP server (replaces Cowboy); routing belongs to Phoenix/Plug layer above it |

### View

| Capability | Status | Verified at | Issue | Cites | Notes |
|------------|--------|-------------|-------|-------|-------|
| View rendering | 🔴 `missing` | — | view_rendering:#3628-not-yet-extracted | — | — |

### Auth

| Capability | Status | Verified at | Issue | Cites | Notes |
|------------|--------|-------------|-------|-------|-------|
| Auth coverage | — `not_applicable` | — | — | — | Bandit is a transport layer with no auth concept; auth handled by Plug/Phoenix above |

### Validation

| Capability | Status | Verified at | Issue | Cites | Notes |
|------------|--------|-------------|-------|-------|-------|
| DTO extraction | — `not_applicable` | — | — | — | Bandit has no DTO layer; data handling is in the Plug/Phoenix application above |
| Request validation | — `not_applicable` | — | — | — | Bandit has no request validation; that layer is above in Plug/Phoenix |

### Middleware

| Capability | Status | Verified at | Issue | Cites | Notes |
|------------|--------|-------------|-------|-------|-------|
| Middleware coverage | — `not_applicable` | — | — | — | Bandit is a raw HTTP/2 server with no middleware pipeline; pipeline is Plug-layer concern |
| Rate limit stamping | 🔴 `missing` | — | [link](https://github.com/cajasmota/archigraph/issues/3778) | — | endpoint rate-limit / throttle stamping not yet implemented for this framework; the #3628 child shipped express-rate-limit (JS/TS) + slowapi/django-ratelimit/flask-limiter/DRF (Python). express-slow-down-compatible / framework-native limiters for this framework are future work. |

### Schema

| Capability | Status | Verified at | Issue | Cites | Notes |
|------------|--------|-------------|-------|-------|-------|
| Type graph extraction | — `not_applicable` | — | — | — | GraphQL schema type→type graph (object-typed field -> referenced object type with list/nullable cardinality) is a GraphQL-only concept; this framework is not a GraphQL server, so it has no GraphQL object-type relationship graph. |

### Type System

| Capability | Status | Verified at | Issue | Cites | Notes |
|------------|--------|-------------|-------|-------|-------|
| Enum extraction | 🟢 `partial` | — | [link](https://github.com/cajasmota/archigraph/issues/3471) | `internal/custom/elixir/typespec.go` | Elixir has no enum keyword. Literal atom-union typespecs (@type role :: :admin | :member | :guest) are the idiomatic enum analogue and ARE captured as SCOPE.Schema/enum with enum_members/member_count props, value-asserted (TestTypespecAtomUnionEnum). Partial (not full): only literal-union typespecs qualify; runtime atom sets / Ecto.Enum field options not statically resolved. |
| Interface extraction | 🟢 `partial` | — | [link](https://github.com/cajasmota/archigraph/issues/3471) | `internal/custom/elixir/typespec.go` | @callback declarations + @behaviour attrs extracted; defprotocol -> SCOPE.Component/interface. Partial: callback arities and per-argument typespecs not parsed into structured signatures. |
| Type alias extraction | 🟢 `partial` | — | [link](https://github.com/cajasmota/archigraph/issues/3471) | `internal/custom/elixir/typespec.go` | @type Name :: OtherType simple alias forms extracted as SCOPE.Schema/type_alias with alias_target. Partial: parametric/compound RHS (unions, maps, tuples) not decomposed. |
| Type extraction | ✅ `full` | — | — | `internal/custom/elixir/typespec.go` | @type/@typep/@opaque declarations + defstruct fields (with @enforce_keys required-key subset) extracted as SCOPE.Schema/struct carrying literal struct_fields/field_count props; @spec annotations captured. defstruct field sets are fully static and value-asserted (TestTypespecDefStructFields). |

### DI

| Capability | Status | Verified at | Issue | Cites | Notes |
|------------|--------|-------------|-------|-------|-------|
| DI binding extraction | 🔴 `missing` | — | 3628 | — | — |
| DI injection point | 🔴 `missing` | — | 3628 | — | — |
| DI scope resolution | 🔴 `missing` | — | 3628 | — | — |

### Testing

| Capability | Status | Verified at | Issue | Cites | Notes |
|------------|--------|-------------|-------|-------|-------|
| Tests linkage | ✅ `full` | `2026-05-30` | — | `internal/extractors/cross/testmap/frameworks.go` | Deep testmap Elixir TESTS linkage: ExUnit (test "..." do leaves, describe groups via balanced do/end body walk) + StreamData (property "..." do) with subject-from-module-name (MyApp.UserServiceTest->UserService) + body call resolution (Foo.bar(...) promoted high); Elixir assertion stopwords (assert/refute/assert_raise/assert_received/assert_in_delta/catch_throw) + check-all generator DSL. Value-asserting tests in extractor_test.go assert specific test->target edges (UserService.register, Accounts.create_user, Serializer.encode, Guard.parse). Closes #3473. |

### Observability

| Capability | Status | Verified at | Issue | Cites | Notes |
|------------|--------|-------------|-------|-------|-------|
| Log extraction | 🟢 `partial` | `2026-05-30` | backfill:dictionary-completeness | `internal/custom/elixir/observability.go`<br>`internal/custom/elixir/observability_test.go` | Logger.debug/info/warning/error(...) statements captured by dedicated observabilityExtractor as SCOPE.Pattern/log_statement with log_level + leading string-literal message; Logger.metadata(...) captured. PARTIAL: logger require/import and message binding not correlated cross-file; interpolated/concatenated message tails not resolved. |
| Metric extraction | 🟢 `partial` | `2026-05-30` | [link](https://github.com/cajasmota/archigraph/issues/3474) | `internal/custom/elixir/observability.go`<br>`internal/custom/elixir/observability_test.go` | :telemetry.execute([:a,:b],...) event names and Telemetry.Metrics counter/summary/last_value/distribution/sum("name") captured by observabilityExtractor as SCOPE.Pattern/metric (metric_name + telemetry_event) when literal at call site. Value-asserting tests prove exact names. PARTIAL: metric/event name -> :telemetry.attach handler -> reporter/exporter wiring spans multiple files and is not resolved. |
| Trace extraction | 🟢 `partial` | `2026-05-30` | [link](https://github.com/cajasmota/archigraph/issues/3474) | `internal/custom/elixir/observability.go`<br>`internal/custom/elixir/observability_test.go` | :telemetry.span([:a,:b],...) event-prefix captured by observabilityExtractor as SCOPE.Pattern/trace_span (span_name + telemetry_event) when literal at call site. Value-asserting test proves exact name. PARTIAL: idiomatic Elixir has no static OTel span/exporter binding; spans are bridged from :telemetry events by a handler attached at runtime (cross-file). |

### Data

| Capability | Status | Verified at | Issue | Cites | Notes |
|------------|--------|-------------|-------|-------|-------|

### Substrate

| Capability | Status | Verified at | Issue | Cites | Notes |
|------------|--------|-------------|-------|-------|-------|
| Confidence overlay | ✅ `full` | `2026-05-28` | — | `internal/graph/graph.go`<br>`internal/mcp/tools.go`<br>`internal/types/confidence.go` | — |
| Config consumption | 🔴 `missing` | — | 3641 | — | — |
| Constant propagation | ✅ `full` | `2026-05-27` | — | `internal/links/constant_propagation.go`<br>`internal/substrate/elixir.go`<br>`internal/substrate/substrate.go` | — |
| DB effect | 🟢 `partial` | `2026-05-28` | — | `internal/links/effect_propagation.go`<br>`internal/substrate/effect_sinks_elixir.go` | — |
| Dead code detection | 🟢 `partial` | `2026-05-28` | — | `internal/links/reachability.go`<br>`internal/mcp/dead_code.go`<br>`internal/substrate/entry_points.go`<br>`internal/substrate/entry_points_elixir.go` | — |
| Def use chain extraction | 🟢 `partial` | — | backfill:dictionary-completeness | `internal/links/def_use_pass.go`<br>`internal/substrate/def_use.go`<br>`internal/substrate/def_use_elixir.go` | Elixir def-use sniffer registered; intra-procedural def-use chains over .ex/.exs |
| Env fallback recognition | ✅ `full` | `2026-05-27` | — | `internal/links/constant_propagation.go`<br>`internal/substrate/elixir.go`<br>`internal/substrate/substrate.go` | — |
| Error flow | ✅ `full` | `2026-06-03` | — | `internal/extractor/exception_flow.go`<br>`internal/extractors/elixir/exception_flow.go`<br>`internal/extractors/elixir/exception_flow_test.go` | raise X / raise mod.X -> THROWS; rescue e in [A,B] / unbound typed rescue -> CATCHES; bare rescue + string raise + reraise + catch :throw + {:error,_} tuple dropped (#3628) |
| Feature flag gating | 🔴 `missing` | — | feature_flag_gating:#3706-not-yet-extracted | — | — |
| Fs effect | 🟢 `partial` | `2026-05-28` | — | `internal/links/effect_propagation.go`<br>`internal/substrate/effect_sinks_elixir.go` | — |
| HTTP effect | 🟢 `partial` | `2026-05-28` | — | `internal/links/effect_propagation.go`<br>`internal/substrate/effect_sinks_elixir.go` | — |
| Import resolution quality | 🟢 `partial` | `2026-05-27` | — | `internal/links/constant_propagation.go`<br>`internal/substrate/elixir.go`<br>`internal/substrate/substrate.go` | — |
| Module cycle detection | 🟢 `partial` | — | backfill:dictionary-completeness | `internal/links/module_cycle_pass.go` | Language-agnostic Tarjan SCC over IMPORTS edges; Elixir use/alias/import edges flow through extractor |
| Mutation effect | 🟢 `partial` | `2026-05-28` | — | `internal/links/effect_propagation.go`<br>`internal/substrate/effect_sinks_elixir.go` | — |
| Pure function tagging | 🟢 `partial` | — | backfill:dictionary-completeness | `internal/links/effect_propagation.go`<br>`internal/links/pure_function_pass.go`<br>`internal/substrate/effect_sinks_elixir.go` | Elixir effect sniffer registered; functions with no elixir effect matches tagged pure=true; immutable semantics make Elixir especially suitable |
| Reachability analysis | 🟢 `partial` | `2026-05-28` | — | `internal/links/reachability.go`<br>`internal/substrate/entry_points.go`<br>`internal/substrate/entry_points_elixir.go` | — |
| Request shape extraction | 🟢 `partial` | `2026-05-28` | [link](https://github.com/cajasmota/archigraph/issues/2771) | `internal/links/payload_drift.go`<br>`internal/mcp/payload_drift_tool.go`<br>`internal/substrate/payload_shapes.go`<br>`internal/substrate/payload_shapes_elixir.go` | — |
| Request sink dataflow | 🔴 `missing` | — | 3740 | — | — |
| Response shape extraction | 🟢 `partial` | `2026-05-28` | [link](https://github.com/cajasmota/archigraph/issues/2771) | `internal/links/payload_drift.go`<br>`internal/mcp/payload_drift_tool.go`<br>`internal/substrate/payload_shapes.go`<br>`internal/substrate/payload_shapes_elixir.go` | — |
| Sanitizer recognition | 🟢 `partial` | `2026-05-28` | — | `internal/links/taint_flow.go`<br>`internal/substrate/taint_sites_elixir.go` | — |
| Schema drift detection | 🟢 `partial` | `2026-05-28` | [link](https://github.com/cajasmota/archigraph/issues/2771) | `internal/links/payload_drift.go`<br>`internal/mcp/payload_drift_tool.go`<br>`internal/substrate/payload_shapes.go`<br>`internal/substrate/payload_shapes_elixir.go` | — |
| Taint sink detection | 🟢 `partial` | `2026-05-28` | — | `internal/links/taint_flow.go`<br>`internal/substrate/taint_sites_elixir.go` | — |
| Taint source detection | 🟢 `partial` | `2026-05-28` | — | `internal/links/taint_flow.go`<br>`internal/substrate/taint_sites_elixir.go` | — |
| Template pattern catalog | 🟢 `partial` | — | backfill:dictionary-completeness | `internal/links/template_pattern_pass.go`<br>`internal/substrate/template_pattern.go`<br>`internal/substrate/template_pattern_elixir.go` | Elixir template-pattern sniffer registered: i18n (gettext/dgettext), log_format (Logger.*), SQL literals via Ecto.Adapters.SQL |
| Vulnerability finding | 🟢 `partial` | `2026-05-28` | — | `internal/links/taint_flow.go`<br>`internal/substrate/taint_sites_elixir.go` | — |

## Provenance

This record is sourced from `docs/coverage/registry.json`. To update it, edit the JSON
(or use `go run ./tools/coverage update lang.elixir.framework.bandit ...`) then regenerate:

```
go run ./tools/coverage validate
go run ./tools/coverage gen
```
