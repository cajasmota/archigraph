<!-- DO NOT EDIT — generated from docs/coverage/registry.json by 'go run ./tools/coverage gen' -->
# `msg.django-channels` — Django Channels

Auto-generated. Back to [summary](../summary.md).

- **Language:** [python](../by-language/python.md)
- **Category:** [message_broker](../by-category/message_broker.md)
- **Capability cells:** 1

## Capabilities

| Capability | Status | Verified at | Issue | Cites | Notes |
|------------|--------|-------------|-------|-------|-------|
| Room channel grouping | ✅ `full` | `2026-06-02` | 3628 | `internal/engine/ws_channel_grouping.go`<br>`internal/engine/ws_channel_grouping_test.go` | [realtime] Django Channels group grouping (#3628 child): applyWSChannelGrouping (python) emits JOINS_CHANNEL(method -> SCOPE.Channel:<group>) for channel_layer.group_add('chat', ...) (and group_discard) and BROADCASTS_TO(method -> SCOPE.Channel:<group>) for group_send('chat', ...) with LITERAL group names; group_add + group_send on the same group converge on one node. Anchored on the group_ call so it is independent of how the channel layer is reached. Honest-partial: dynamic group names skipped. |

## Provenance

This record is sourced from `docs/coverage/registry.json`. To update it, edit the JSON
(or use `go run ./tools/coverage update msg.django-channels ...`) then regenerate:

```
go run ./tools/coverage validate
go run ./tools/coverage gen
```
