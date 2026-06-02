<!-- DO NOT EDIT — generated from docs/coverage/registry.json by 'go run ./tools/coverage gen' -->
# `infra.observability.dropwizard-metrics` — Dropwizard Metrics

Auto-generated. Back to [summary](../summary.md).

- **Language:** [multi](../by-language/multi.md)
- **Category:** [observability](../by-category/observability.md)
- **Capability cells:** 3

## Capabilities

| Capability | Status | Verified at | Issue | Cites | Notes |
|------------|--------|-------------|-------|-------|-------|
| Log extraction | — `not_applicable` | — | — | — | — |
| Metric extraction | 🟢 `partial` | `2026-06-02` | [link](https://github.com/cajasmota/archigraph/issues/3856) | `internal/extractors/java/observability.go` | #3854 area #11: Dropwizard Metrics emit INSTRUMENTS edges (enclosing method -> metric:<name> stub) in Java: <registry>.meter/timer/counter/histogram("name") on a registry-like receiver. Honest-partial: a non-literal metric name yields traced=true+dynamic=true keyed on the method; calls on a receiver that does not look like a MetricRegistry are NOT emitted. |
| Trace extraction | — `not_applicable` | — | — | — | — |

## Provenance

This record is sourced from `docs/coverage/registry.json`. To update it, edit the JSON
(or use `go run ./tools/coverage update infra.observability.dropwizard-metrics ...`) then regenerate:

```
go run ./tools/coverage validate
go run ./tools/coverage gen
```
