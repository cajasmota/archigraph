<!-- DO NOT EDIT — generated from docs/coverage/registry.json by 'go run ./tools/coverage gen' -->
# `msg.broker.kafka` — Apache Kafka

Auto-generated. Back to [summary](../summary.md).

- **Language:** [multi](../by-language/multi.md)
- **Category:** [message_broker](../by-category/message_broker.md)
- **Capability cells:** 3

## Capabilities

| Capability | Status | Verified at | Issue | Cites | Notes |
|------------|--------|-------------|-------|-------|-------|
| Consumer extraction | ✅ `full` | `2026-05-28` | — | `internal/engine/kafka_edges.go` | — |
| Producer extraction | ✅ `full` | `2026-05-28` | — | `internal/engine/kafka_edges.go`<br>`internal/engine/kafka_wrapper_edges.go` | — |
| Topic attribution | ✅ `full` | `2026-05-28` | — | `internal/engine/kafka_edges.go` | — |

## Related extraction records

This hub record tracks the technology at a high level. The deep, code-level
coverage for this technology lives in the per-language records below — each
one is a separate detail page.

| Record | Language | Kind | Status |
|--------|----------|------|--------|
| [`lang.c-cpp.framework.librdkafka`](./lang.c-cpp.framework.librdkafka.md) | C/C++ | framework | 3 partial |
| [`msg.kafka-streams`](./msg.kafka-streams.md) | multi |  | 2 missing |
| [`lang.rust.framework.rdkafka`](./lang.rust.framework.rdkafka.md) | rust | framework | 3 partial |

## Provenance

This record is sourced from `docs/coverage/registry.json`. To update it, edit the JSON
(or use `go run ./tools/coverage update msg.broker.kafka ...`) then regenerate:

```
go run ./tools/coverage validate
go run ./tools/coverage gen
```
