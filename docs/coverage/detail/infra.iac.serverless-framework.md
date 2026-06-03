<!-- DO NOT EDIT — generated from docs/coverage/registry.json by 'go run ./tools/coverage gen' -->
# `infra.iac.serverless-framework` — Serverless Framework

Auto-generated. Back to [summary](../summary.md).

- **Language:** [multi](../by-language/multi.md)
- **Category:** [platform](../by-category/platform.md)
- **Subcategory:** IaC / Provisioning
- **Capability cells:** 6

## Capabilities

| Capability | Status | Verified at | Issue | Cites | Notes |
|------------|--------|-------------|-------|-------|-------|
| Dependency attribution | ✅ `full` | `2026-05-30` | — | `internal/engine/serverless_edges.go`<br>`internal/engine/serverless_framework_edges.go` | — |
| Iac cross stack reference | — `not_applicable` | — | — | — | The Serverless Framework parser emits no cross-stack / Fn::ImportValue reference. Honest-missing. |
| Iac event source wiring | ✅ `full` | `2026-06-04` | [link](https://github.com/cajasmota/archigraph/issues/4198) | `internal/engine/serverless_framework_parse.go` | The Serverless Framework parser reads each function's `events:` list (parseEventsBlock, serverless_framework_parse.go:247-286) and emits a TRIGGERS edge source→function per non-http trigger carrying event_type=<kind>: sqs (queue entity → fn, serverless_framework_edges.go:244-250), sns (topic → fn, :270-275), stream/kinesis (stream → fn, :296-301) and schedule (ScheduledJob → fn, :321-327). slsTriggersEdgeKind="TRIGGERS" (serverless_framework_edges.go:59). Full for the events-block trigger idiom — each edge wires which event source (queue/topic/stream/schedule) invokes which function, with the trigger type recorded on event_type. |
| Iac iam grant attribution | — `not_applicable` | — | — | — | Serverless Framework IAM is configured declaratively (provider.iam / iamRoleStatements) and via embedded CloudFormation resources; the parser extracts functions/events/handlers and emits no grantee→target grant edge or grant=<method> attribution. Honest-missing. |
| Iac output export extraction | — `not_applicable` | — | — | — | The Serverless Framework parser extracts functions/events/handlers; it does not parse the `resources.Outputs` CloudFormation block into output/export entities (the 'exports' matches in serverless_edges.go are JS handler exports, not stack outputs). Honest-missing. |
| Resource extraction | ✅ `full` | `2026-05-30` | — | `internal/engine/serverless_framework_edges.go`<br>`internal/engine/serverless_framework_parse.go` | — |

## Provenance

This record is sourced from `docs/coverage/registry.json`. To update it, edit the JSON
(or use `go run ./tools/coverage update infra.iac.serverless-framework ...`) then regenerate:

```
go run ./tools/coverage validate
go run ./tools/coverage gen
```
