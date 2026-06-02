<!-- DO NOT EDIT — generated from docs/coverage/registry.json by 'go run ./tools/coverage gen' -->
# `msg.actioncable` — Rails ActionCable

Auto-generated. Back to [summary](../summary.md).

- **Language:** [ruby](../by-language/ruby.md)
- **Category:** [message_broker](../by-category/message_broker.md)
- **Capability cells:** 1

## Capabilities

| Capability | Status | Verified at | Issue | Cites | Notes |
|------------|--------|-------------|-------|-------|-------|
| Room channel grouping | ✅ `full` | `2026-06-02` | 3628 | `internal/engine/ws_channel_grouping.go`<br>`internal/engine/ws_channel_grouping_test.go` | [realtime] ActionCable room grouping (#3628 child): applyWSChannelGrouping (ruby) emits JOINS_CHANNEL(method -> SCOPE.Channel:<name>) for stream_from 'chat_1' and BROADCASTS_TO(method -> SCOPE.Channel:<name>) for ActionCable.server.broadcast('chat_1', ...) with LITERAL channel names; a stream_from + server.broadcast on the same channel converge on one node. Enclosing def attributes the caller. Honest-partial: stream_for <var> / broadcast_to(<var>) dynamic targets skipped. |

## Provenance

This record is sourced from `docs/coverage/registry.json`. To update it, edit the JSON
(or use `go run ./tools/coverage update msg.actioncable ...`) then regenerate:

```
go run ./tools/coverage validate
go run ./tools/coverage gen
```
