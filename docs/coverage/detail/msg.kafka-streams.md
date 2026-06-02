<!-- DO NOT EDIT — generated from docs/coverage/registry.json by 'go run ./tools/coverage gen' -->
# `msg.kafka-streams` — Kafka Streams / Faust

Auto-generated. Back to [summary](../summary.md).

- **Language:** [multi](../by-language/multi.md)
- **Category:** [message_broker](../by-category/message_broker.md)
- **Capability cells:** 2

## Capabilities

| Capability | Status | Verified at | Issue | Cites | Notes |
|------------|--------|-------------|-------|-------|-------|
| Consumer extraction | 🔴 `missing` | — | 3828 | — | No producer/consumer extraction yet for this broker variant; tracked in #3828. |
| Producer extraction | 🔴 `missing` | — | 3828 | — | No producer/consumer extraction yet for this broker variant; tracked in #3828. |

## Related extraction records

This record provides code-level coverage for the
[`msg.broker.kafka`](./msg.broker.kafka.md) hub record (Apache Kafka),
which tracks the same technology at a higher level.

## Provenance

This record is sourced from `docs/coverage/registry.json`. To update it, edit the JSON
(or use `go run ./tools/coverage update msg.kafka-streams ...`) then regenerate:

```
go run ./tools/coverage validate
go run ./tools/coverage gen
```
