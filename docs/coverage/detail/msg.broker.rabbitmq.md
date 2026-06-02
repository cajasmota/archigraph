<!-- DO NOT EDIT — generated from docs/coverage/registry.json by 'go run ./tools/coverage gen' -->
# `msg.broker.rabbitmq` — RabbitMQ

Auto-generated. Back to [summary](../summary.md).

- **Language:** [multi](../by-language/multi.md)
- **Category:** [message_broker](../by-category/message_broker.md)
- **Capability cells:** 3

## Capabilities

| Capability | Status | Verified at | Issue | Cites | Notes |
|------------|--------|-------------|-------|-------|-------|
| Consumer extraction | ✅ `full` | `2026-05-28` | — | `internal/engine/rabbitmq_edges.go` | — |
| Producer extraction | ✅ `full` | `2026-05-28` | — | `internal/engine/rabbitmq_edges.go` | — |
| Topic attribution | ✅ `full` | `2026-06-02` | — | `internal/engine/rabbitmq_edges.go`<br>`internal/links/topic_pass.go` | rabbitmq:<queue/exchange> SCOPE.Queue node; topic_pass.go joins a PUBLISHES_TO producer (exchange 'orders') to a SUBSCRIBES_TO consumer of queue/exchange 'orders' sharing the node Name into a cross-repo producer->consumer topology edge (channel=rabbitmq). |

## Related extraction records

This hub record tracks the technology at a high level. The deep, code-level
coverage for this technology lives in the per-language records below — each
one is a separate detail page.

| Record | Language | Kind | Status |
|--------|----------|------|--------|
| [`lang.rust.framework.lapin`](./lang.rust.framework.lapin.md) | rust | framework | 3 partial |

## Provenance

This record is sourced from `docs/coverage/registry.json`. To update it, edit the JSON
(or use `go run ./tools/coverage update msg.broker.rabbitmq ...`) then regenerate:

```
go run ./tools/coverage validate
go run ./tools/coverage gen
```
