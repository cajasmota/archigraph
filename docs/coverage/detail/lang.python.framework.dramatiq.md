<!-- DO NOT EDIT — generated from docs/coverage/registry.json by 'go run ./tools/coverage gen' -->
# `lang.python.framework.dramatiq` — Dramatiq (task queue)

Auto-generated. Back to [summary](../summary.md).

- **Language:** [python](../by-language/python.md)
- **Category:** [http_framework](../by-category/http_framework.md)
- **Subcategory:** Task Queue
- **Capability cells:** 31

## Capabilities


### Tasks

| Capability | Status | Verified at | Issue | Cites | Notes |
|------------|--------|-------------|-------|-------|-------|
| Task extraction | 🟢 `partial` | `2026-05-28` | — | `internal/custom/python/dramatiq.go`<br>`internal/engine/rules/python/frameworks/dramatiq.yaml` | — |
| Task routing | 🟢 `partial` | `2026-05-30` | [link](https://github.com/cajasmota/archigraph/issues/3193) | `internal/custom/python/dramatiq.go`<br>`internal/custom/python/dramatiq_routing_test.go` | queue->actor routing extracted from @dramatiq.actor(queue_name=...) decorators and actor.send_with_options(queue_name=...) dispatch overrides. Partial because broker-level routing middleware (broker.add_middleware) and dynamic/computed queue names are not modelled. |

### Schedule

| Capability | Status | Verified at | Issue | Cites | Notes |
|------------|--------|-------------|-------|-------|-------|
| Schedule extraction | 🟢 `partial` | — | [link](https://github.com/cajasmota/archigraph/issues/3074) | — | django-dramatiq periodiq/APScheduler integration: periodic task declarations via @dramatiq.actor + periodiq.cron are detectable via the @periodiq.cron decorator pattern but are not yet extracted by the dramatiq extractor. Partial because basic actor-based scheduling is not modelled; only per-actor retry intervals. |

### Broker

| Capability | Status | Verified at | Issue | Cites | Notes |
|------------|--------|-------------|-------|-------|-------|
| Broker binding | ✅ `full` | `2026-05-29` | [link](https://github.com/cajasmota/archigraph/issues/3074) | `internal/custom/python/broker_binding_test.go`<br>`internal/custom/python/dramatiq.go` | — |
| Result backend binding | — `not_applicable` | — | [link](https://github.com/cajasmota/archigraph/issues/3074) | — | dramatiq has no result_backend concept; task results are stored via middleware (e.g. django-dramatiq result backend), not a single broker URL |

### Reliability

| Capability | Status | Verified at | Issue | Cites | Notes |
|------------|--------|-------------|-------|-------|-------|
| Retry policy extraction | ✅ `full` | `2026-05-29` | [link](https://github.com/cajasmota/archigraph/issues/3074) | `internal/custom/python/broker_binding_test.go`<br>`internal/custom/python/dramatiq.go` | — |

### Testing

| Capability | Status | Verified at | Issue | Cites | Notes |
|------------|--------|-------------|-------|-------|-------|
| Tests linkage | 🟢 `partial` | `2026-05-29` | backfill:dictionary-completeness | `internal/custom/python/pytest.go` | — |

### Substrate

| Capability | Status | Verified at | Issue | Cites | Notes |
|------------|--------|-------------|-------|-------|-------|
| Confidence overlay | 🟢 `partial` | `2026-05-29` | 3068 | `internal/links/effect_propagation.go`<br>`internal/substrate/effect_sinks_python.go`<br>`internal/types/confidence.go` | — |
| Config consumption | ✅ `full` | `2026-06-02` | 3641 | `internal/extractors/python/config_consumer.go`<br>`internal/extractors/python/config_consumer_test.go` | settings.X / os.environ.get(k) -> DEPENDS_ON_CONFIG (live pre-#3641; config-blast-radius) |
| Constant propagation | ✅ `full` | `2026-05-28` | — | `internal/links/constant_propagation.go`<br>`internal/substrate/python.go`<br>`internal/substrate/substrate.go` | — |
| DB effect | 🟢 `partial` | `2026-05-29` | backfill:dictionary-completeness | `internal/substrate/effect_sinks_python.go` | — |
| Dead code detection | 🟢 `partial` | `2026-05-29` | 3047 | `internal/links/reachability.go`<br>`internal/substrate/entry_points.go`<br>`internal/substrate/entry_points_python.go` | dead code derived from reachability + entry-point sniffer; partial because dramatiq @actor entry wiring is not modelled |
| Def use chain extraction | 🟢 `partial` | `2026-05-29` | backfill:dictionary-completeness | `internal/substrate/def_use_python.go` | — |
| Env fallback recognition | ✅ `full` | `2026-05-28` | — | `internal/links/constant_propagation.go`<br>`internal/substrate/python.go`<br>`internal/substrate/substrate.go` | — |
| Error flow | ✅ `full` | `2026-06-02` | 3628 | `internal/extractor/exception_flow.go`<br>`internal/extractors/python/exception_flow.go`<br>`internal/extractors/python/exception_flow_test.go` | raise X / raise mod.X -> THROWS; except (A,B) -> CATCHES; bare except + dynamic raise dropped (#3628) |
| Feature flag gating | 🔴 `missing` | — | feature_flag_gating:#3706-not-yet-extracted | — | — |
| Fs effect | 🟢 `partial` | `2026-05-29` | backfill:dictionary-completeness | `internal/substrate/effect_sinks_python.go` | — |
| HTTP effect | 🟢 `partial` | `2026-05-29` | backfill:dictionary-completeness | `internal/substrate/effect_sinks_python.go` | — |
| Import resolution quality | 🟢 `partial` | `2026-05-28` | — | `internal/links/constant_propagation.go`<br>`internal/substrate/python.go`<br>`internal/substrate/substrate.go` | — |
| Module cycle detection | 🟢 `partial` | `2026-05-29` | 3047 | `internal/links/module_cycle_pass.go` | language-agnostic Tarjan SCC over IMPORTS edges; fires on all Python modules; partial because task/actor module cycles are not distinguished from app-level cycles |
| Mutation effect | 🟢 `partial` | `2026-05-29` | backfill:dictionary-completeness | `internal/substrate/effect_sinks_python.go` | — |
| Pure function tagging | 🟢 `partial` | `2026-05-29` | 3047 | `internal/links/effect_propagation.go`<br>`internal/links/pure_function_pass.go` | derivative of effect propagation; fires on all Python entities; partial because actor functions may have side effects missed by heuristic sniffers |
| Reachability analysis | 🟢 `partial` | `2026-05-29` | 3047 | `internal/links/reachability.go`<br>`internal/substrate/entry_points.go`<br>`internal/substrate/entry_points_python.go` | language-wide Python entry-point sniffer covers module-level __main__/test/lifecycle; partial because @dramatiq.actor entry wiring is not modelled |
| Request shape extraction | ✅ `full` | `2026-05-27` | — | `internal/links/payload_drift.go`<br>`internal/mcp/payload_drift_tool.go`<br>`internal/substrate/payload_shapes.go`<br>`internal/substrate/payload_shapes_python.go` | — |
| Response shape extraction | ✅ `full` | `2026-05-27` | — | `internal/links/payload_drift.go`<br>`internal/mcp/payload_drift_tool.go`<br>`internal/substrate/payload_shapes.go`<br>`internal/substrate/payload_shapes_python.go` | — |
| Sanitizer recognition | 🟢 `partial` | `2026-05-29` | backfill:dictionary-completeness | `internal/substrate/taint_sites_python.go` | — |
| Schema drift detection | ✅ `full` | `2026-05-27` | — | `internal/links/payload_drift.go`<br>`internal/mcp/payload_drift_tool.go`<br>`internal/substrate/payload_shapes.go`<br>`internal/substrate/payload_shapes_python.go` | — |
| Taint sink detection | 🟢 `partial` | `2026-05-29` | backfill:dictionary-completeness | `internal/substrate/taint_sites_python.go` | — |
| Taint source detection | 🟢 `partial` | `2026-05-29` | backfill:dictionary-completeness | `internal/substrate/taint_sites_python.go` | — |
| Template pattern catalog | 🟢 `partial` | `2026-05-29` | backfill:dictionary-completeness | `internal/substrate/template_pattern_python.go` | — |
| Vulnerability finding | 🟢 `partial` | `2026-05-29` | 3047 | `internal/links/taint_flow.go`<br>`internal/substrate/taint_sites_python.go` | derives from taint source+sink co-occurrence; fires on all Python; partial for dramatiq actor context |

## Provenance

This record is sourced from `docs/coverage/registry.json`. To update it, edit the JSON
(or use `go run ./tools/coverage update lang.python.framework.dramatiq ...`) then regenerate:

```
go run ./tools/coverage validate
go run ./tools/coverage gen
```
