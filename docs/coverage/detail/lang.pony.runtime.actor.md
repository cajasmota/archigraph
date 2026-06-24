<!-- DO NOT EDIT — generated from docs/coverage/registry.json by 'go run ./tools/coverage gen' -->
# `lang.pony.runtime.actor` — Pony actor model

Auto-generated. Back to [summary](../summary.md).

- **Language:** [pony](../by-language/pony.md)
- **Category:** [language](../by-category/language.md)
- **Capability cells:** 1

## Capabilities

| Capability | Status | Verified at | Issue | Cites | Notes |
|------------|--------|-------------|-------|-------|-------|
| Core extraction | 🟢 `partial` | `2026-06-24` | 5384 | `internal/extractors/pony/actor_topology.go`<br>`internal/extractors/pony/actor_topology_test.go`<br>`internal/extractors/pony/extractor.go` | #5384 (epic #5360): Pony actor-model topology enrichment over the existing regex extractor (no new entity Kind, mirroring the lang.erlang.runtime.otp edge-enrichment model; enrichActorTopology). (1) actor Components are Tagged 'actor'; each behaviour (be name(...), Subtype=behavior) owned by an actor is Tagged 'pony_behaviour' with Properties[actor]=<owning actor>. (2) MESSAGE-SEND RECOVERY: a behaviour call site receiver.behaviour(args) (ponyMsgSendRE) is the actor-model equivalent of a gen_server:cast — async delivery to another actor's mailbox; recovered per-operation body and stamped onto the matching CALLS edge (ToID==behaviour name) with Properties pony_msg_send=true / pony_msg_behaviour / pony_msg_receiver (verbatim receiver expr) / pony_msg_actor (target actor), and the sending operation is Tagged pony_msg_out:<behaviour>. Honest PARTIAL: the receiver's static actor TYPE is not resolved (receiver captured verbatim), a send is only recovered when its behaviour name matches a behaviour declared in the SAME file (regex extractor has no cross-file type table), and synchronous fun/constructor calls are intentionally excluded (only behaviours are asynchronous messages). Proven by TestActorTopology_TagsActorsAndBehaviours, _MessageSendEnrichment, _NoActorsNoOp, _SyncFunNotMessage. |

## Provenance

This record is sourced from `docs/coverage/registry.json`. To update it, edit the JSON
(or use `go run ./tools/coverage update lang.pony.runtime.actor ...`) then regenerate:

```
go run ./tools/coverage validate
go run ./tools/coverage gen
```
