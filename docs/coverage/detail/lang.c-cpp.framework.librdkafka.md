<!-- DO NOT EDIT — generated from docs/coverage/registry.json by 'go run ./tools/coverage gen' -->
# `lang.c-cpp.framework.librdkafka` — librdkafka (C/C++ Kafka client)

Auto-generated. Back to [summary](../summary.md).

- **Language:** [C/C++](../by-language/c-cpp.md)
- **Category:** [message_broker](../by-category/message_broker.md)
- **Capability cells:** 3

## Capabilities

| Capability | Status | Verified at | Issue | Cites | Notes |
|------------|--------|-------------|-------|-------|-------|
| Consumer extraction | 🟢 `partial` | `2026-05-31` | — | `internal/engine/kafka_edges.go` | Literal topics via rd_kafka_topic_partition_list_add / consumer->subscribe({...}). |
| Producer extraction | 🟢 `partial` | `2026-05-31` | — | `internal/engine/kafka_edges.go` | Literal topics via rd_kafka_topic_new / RdKafka::Topic::create / producer->produce. No const/config resolution. |
| Topic attribution | 🟢 `partial` | `2026-05-31` | — | `internal/engine/kafka_edges.go` | Canonical kafka:<topic> node shared with all other Kafka languages for cross-repo linkage. |

## Related extraction records

This record provides code-level coverage for the
[`msg.broker.kafka`](./msg.broker.kafka.md) hub record (Apache Kafka),
which tracks the same technology at a higher level.

## Provenance

This record is sourced from `docs/coverage/registry.json`. To update it, edit the JSON
(or use `go run ./tools/coverage update lang.c-cpp.framework.librdkafka ...`) then regenerate:

```
go run ./tools/coverage validate
go run ./tools/coverage gen
```
