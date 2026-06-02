<!-- DO NOT EDIT — generated from docs/coverage/registry.json by 'go run ./tools/coverage gen' -->
# `msg.resque` — Resque (Ruby task queue)

Auto-generated. Back to [summary](../summary.md).

- **Language:** [ruby](../by-language/ruby.md)
- **Category:** [message_broker](../by-category/message_broker.md)
- **Capability cells:** 3

## Capabilities

| Capability | Status | Verified at | Issue | Cites | Notes |
|------------|--------|-------------|-------|-------|-------|
| Consumer extraction | 🟢 `partial` | — | 3628 | `internal/engine/scheduled_jobs_edges.go`<br>`internal/engine/scheduled_jobs_edges_test.go` | epic #3628 jobs cross-link: a Ruby class declaring @queue = :name AND def self.perform becomes a SCOPE.ScheduledJob (resque:<Job>) with a TRIGGERS edge to perform (the consumer). Honest-partial: requires both @queue and self.perform in one file; dynamic class refs not resolved. |
| Producer extraction | 🟢 `partial` | — | 3628 | `internal/engine/scheduled_jobs_edges.go`<br>`internal/engine/scheduled_jobs_edges_test.go` | epic #3628 jobs cross-link: Resque.enqueue/enqueue_to/enqueue_in dispatch sites emit an ENQUEUES edge from the enclosing Ruby method to the job entity (SCOPE.ScheduledJob resque:<Job>), joined on the shared resque:<Job> id with the consumer side. Honest-partial: enqueue target resolves only when the job class def is in the indexed group; dynamic class names not resolved. |
| Topic attribution | 🟢 `partial` | — | 3628 | `internal/engine/scheduled_jobs_edges.go`<br>`internal/engine/scheduled_jobs_edges_test.go` | epic #3628 jobs cross-link: producer ENQUEUES and consumer TRIGGERS both reference the same resque:<Job> SCOPE.ScheduledJob node, so a Resque.enqueue(Job) caller and the Job's self.perform join on one node (same shape as Sidekiq). Honest-partial: cross-file join requires the job class def to be indexed. |

## Provenance

This record is sourced from `docs/coverage/registry.json`. To update it, edit the JSON
(or use `go run ./tools/coverage update msg.resque ...`) then regenerate:

```
go run ./tools/coverage validate
go run ./tools/coverage gen
```
