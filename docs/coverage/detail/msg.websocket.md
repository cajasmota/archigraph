<!-- DO NOT EDIT — generated from docs/coverage/registry.json by 'go run ./tools/coverage gen' -->
# `msg.websocket` — WebSocket

Auto-generated. Back to [summary](../summary.md).

- **Language:** [multi](../by-language/multi.md)
- **Category:** [message_broker](../by-category/message_broker.md)
- **Capability cells:** 3

## Capabilities

| Capability | Status | Verified at | Issue | Cites | Notes |
|------------|--------|-------------|-------|-------|-------|
| Consumer extraction | ✅ `full` | `2026-06-02` | — | `internal/engine/http_endpoint_ws_client.go`<br>`internal/engine/websocket_edges.go` | (epic #3628 realtime cross-link): in addition to the connection-level WS_CONNECTS edge (ws:<channel> identity), the socket.io CLIENT side now ALSO emits event-level consumer-side http_endpoint_call entities keyed BYTE-IDENTICALLY to the realtime producer endpoint shape http:WS:/<canonical event> — socket.emit('chat:message') and socket.on('notify') run through the SAME canonicalizeRealtimePath('websocket',...) as the producer so 'chat:message' collapses to http:WS:/chat{message} on both sides and the Name-based cross-repo HTTP linker joins client emit/subscribe to the server socket.on handler with no new linker code. ws_role (emit|subscribe) folded into the framework label; source_caller drives the FETCHES edge. Honest-partial: dynamic event names, native ws.send (no event), and SERVER files are skipped. |
| Producer extraction | ✅ `full` | `2026-06-02` | — | `internal/engine/realtime_endpoint_synthesis.go`<br>`internal/engine/websocket_edges.go` | #3682 (epic #3628 area #7): in addition to ChannelEvent + WS_SUBSCRIBES_TO/WS_EMITS edges, the producer side now ALSO emits endpoint-shaped http_endpoint_definition entities (verb=WS, route_path, realtime=true, transport=websocket) with a HANDLES edge to the handler, so the endpoints/find tools surface WS endpoints alongside REST and they cross-link on the shared http:WS:<path> synthetic ID. Frameworks: NestJS @WebSocketGateway+@SubscribeMessage, socket.io socket.on, bare ws WebSocketServer, FastAPI @app.websocket. Honest-partial where the channel/path is fully dynamic. |
| Topic attribution | 🟢 `partial` | `2026-05-28` | — | `internal/engine/websocket_edges.go` | — |

## Provenance

This record is sourced from `docs/coverage/registry.json`. To update it, edit the JSON
(or use `go run ./tools/coverage update msg.websocket ...`) then regenerate:

```
go run ./tools/coverage validate
go run ./tools/coverage gen
```
