<!-- DO NOT EDIT — generated from docs/coverage/registry.json by 'go run ./tools/coverage gen' -->
# `infra.observability.slf4j` — SLF4J / Logback (structured logging)

Auto-generated. Back to [summary](../summary.md).

- **Language:** [multi](../by-language/multi.md)
- **Category:** [observability](../by-category/observability.md)
- **Capability cells:** 3

## Capabilities

| Capability | Status | Verified at | Issue | Cites | Notes |
|------------|--------|-------------|-------|-------|-------|
| Log extraction | 🟢 `partial` | `2026-06-02` | [link](https://github.com/cajasmota/archigraph/issues/3856) | `internal/extractors/java/observability.go` | #3854 area #11: SLF4J 2.x fluent structured logging emits INSTRUMENTS edges (enclosing method -> log:<event> stub) in Java. Anchored on the terminal .log("event") of a fluent chain that includes a level entry (atInfo/atDebug/atWarn/atError/atTrace/atLevel) AND at least one structured key (addKeyValue/addMarker/addArgument), so only keyed/structured events count. The log_name is the literal message passed to .log(...); honest-partial: a non-literal message on a recognised structured chain yields traced=true+dynamic=true. Plain free-text logging (logger.info("text")) is intentionally NOT emitted (no structured key). |
| Metric extraction | — `not_applicable` | — | — | — | — |
| Trace extraction | — `not_applicable` | — | — | — | — |

## Provenance

This record is sourced from `docs/coverage/registry.json`. To update it, edit the JSON
(or use `go run ./tools/coverage update infra.observability.slf4j ...`) then regenerate:

```
go run ./tools/coverage validate
go run ./tools/coverage gen
```
