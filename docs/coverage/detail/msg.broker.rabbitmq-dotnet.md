<!-- DO NOT EDIT — generated from docs/coverage/registry.json by 'go run ./tools/coverage gen' -->
# `msg.broker.rabbitmq-dotnet` — RabbitMQ — C# (RabbitMQ.Client)

Auto-generated. Back to [summary](../summary.md).

- **Language:** [C#](../by-language/csharp.md)
- **Category:** [message_broker](../by-category/message_broker.md)
- **Subcategory:** Brokers
- **Capability cells:** 3

## Capabilities

| Capability | Status | Verified at | Issue | Cites | Notes |
|------------|--------|-------------|-------|-------|-------|
| Consumer extraction | 🟢 `partial` | `2026-06-13` | 4996 | `internal/engine/rabbitmq_edges.go`<br>`internal/engine/rabbitmq_edges_test.go` | #4996: synthesizeCSharpRabbitMQ (rabbitmq_edges.go, case "csharp") resolves RabbitMQ.Client channel.BasicConsume(queue: "q" | positional) consumer sites -> SUBSCRIBES_TO rabbitmq:<name> (messaging_layer=rabbitmq-dotnet), and records channel.QueueDeclare(...) queue nodes. Honest-partial: literal queue names only; EventingBasicConsumer.Received handler attribution not modelled. |
| Producer extraction | 🟢 `partial` | `2026-06-13` | 4996 | `internal/engine/rabbitmq_edges.go`<br>`internal/engine/rabbitmq_edges_test.go` | #4996: synthesizeCSharpRabbitMQ resolves channel.BasicPublish(exchange:, routingKey:, ...) (named, either order) and the positional BasicPublish("ex", "rk", ...) producer sites -> PUBLISHES_TO from the enclosing method, keyed by the routing-key literal (rabbitmq:<name>), with the exchange recorded as an edge property. Honest-partial: literal routing-key only; fanout (empty routing key) and dynamic keys not attributed. |
| Topic attribution | 🟢 `partial` | `2026-06-13` | 4996 | `internal/engine/rabbitmq_edges.go`<br>`internal/links/topic_pass.go` | #4996: routing-key/queue literals become rabbitmq:<name> SCOPE.Queue nodes joined producer->consumer cross-repo via topic_pass.go (channel=rabbitmq); exchange recorded as edge prop. Honest-partial: literal keys only; exchange-binding topology not decomposed. |

## Provenance

This record is sourced from `docs/coverage/registry.json`. To update it, edit the JSON
(or use `go run ./tools/coverage update msg.broker.rabbitmq-dotnet ...`) then regenerate:

```
go run ./tools/coverage validate
go run ./tools/coverage gen
```
