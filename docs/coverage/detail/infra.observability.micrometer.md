<!-- DO NOT EDIT — generated from docs/coverage/registry.json by 'go run ./tools/coverage gen' -->
# `infra.observability.micrometer` — Micrometer (JVM metrics facade)

Auto-generated. Back to [summary](../summary.md).

- **Language:** [multi](../by-language/multi.md)
- **Category:** [observability](../by-category/observability.md)
- **Capability cells:** 3

## Capabilities

| Capability | Status | Verified at | Issue | Cites | Notes |
|------------|--------|-------------|-------|-------|-------|
| Log extraction | — `not_applicable` | — | — | — | — |
| Metric extraction | 🟢 `partial` | `2026-06-02` | [link](https://github.com/cajasmota/archigraph/issues/3856) | `internal/extractors/java/observability.go` | #3854 area #11: Micrometer metrics emit INSTRUMENTS edges (enclosing method -> metric:<name> stub) in Java. Registry-method form on a registry-like receiver: <reg>.counter/timer/gauge/summary/longTaskTimer/distributionSummary("name") -> metric:name. Builder form: Counter|Timer|Gauge|DistributionSummary|LongTaskTimer.builder("name") -> metric:name. Annotation form: @Timed("name") / @Counted("name") on a method -> metric:name. Honest-partial: a non-literal name (or a bare @Timed with no name) yields traced=true+dynamic=true keyed on the method without fabrication; a metric method on a receiver that does not look like a meter registry is NOT emitted (precision). |
| Trace extraction | — `not_applicable` | — | — | — | — |

## Related extraction records

This record provides code-level coverage for the
[`infra.observability.prometheus`](./infra.observability.prometheus.md) hub record (Prometheus),
which tracks the same technology at a higher level.

## Provenance

This record is sourced from `docs/coverage/registry.json`. To update it, edit the JSON
(or use `go run ./tools/coverage update infra.observability.micrometer ...`) then regenerate:

```
go run ./tools/coverage validate
go run ./tools/coverage gen
```
