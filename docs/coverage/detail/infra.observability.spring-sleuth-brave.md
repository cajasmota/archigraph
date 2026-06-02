<!-- DO NOT EDIT — generated from docs/coverage/registry.json by 'go run ./tools/coverage gen' -->
# `infra.observability.spring-sleuth-brave` — Spring Cloud Sleuth / Brave

Auto-generated. Back to [summary](../summary.md).

- **Language:** [multi](../by-language/multi.md)
- **Category:** [observability](../by-category/observability.md)
- **Capability cells:** 3

## Capabilities

| Capability | Status | Verified at | Issue | Cites | Notes |
|------------|--------|-------------|-------|-------|-------|
| Log extraction | — `not_applicable` | — | — | — | — |
| Metric extraction | — `not_applicable` | — | — | — | — |
| Trace extraction | 🟢 `partial` | `2026-06-02` | [link](https://github.com/cajasmota/archigraph/issues/3856) | `internal/extractors/java/observability.go` | #3854 area #11: Spring Sleuth / Brave manual spans emit INSTRUMENTS edges (enclosing method -> span:<name> stub) in Java via the fluent form tracer.nextSpan().name("op").start(), anchored on the .name("op") call whose receiver chain includes nextSpan(). Honest-partial: a non-literal span name yields traced=true+dynamic=true keyed on the method without fabrication. |

## Provenance

This record is sourced from `docs/coverage/registry.json`. To update it, edit the JSON
(or use `go run ./tools/coverage update infra.observability.spring-sleuth-brave ...`) then regenerate:

```
go run ./tools/coverage validate
go run ./tools/coverage gen
```
