# MCP tools

archigraph exposes 29 MCP tools, all prefixed `archigraph_`. The canonical source of truth for inputs, outputs, and response shapes is:

**[`internal/mcp/SCHEMA.md`](../internal/mcp/SCHEMA.md)**

This page is an orientation-level catalogue. For parameter details, response field lists, and deprecation notices, read SCHEMA.md directly.

---

## Setup

After `archigraph install <group>`, the MCP server is registered automatically in your agent's config. The daemon registers one server per machine; multiple groups can be active simultaneously. The server uses stdio transport.

To verify the wiring:

```sh
archigraph status <group>    # shows MCP: connected / disconnected
```

For per-agent config details see [agent-hosts.md](agent-hosts.md).

---

## Tool catalogue

### Orientation

| Tool | One-line purpose |
|------|-----------------|
| `archigraph_whoami` | Resolve group and repo for the agent's current working directory. Call this first. |
| `archigraph_stats` | Entity + relationship counts per repo. Use to scope token budgets. |
| `archigraph_clusters` | Louvain communities with top-ranked entities. Fast module map. |

### Discovery

| Tool | One-line purpose |
|------|-----------------|
| `archigraph_find` | BM25-ranked graph query with optional BFS expansion. Primary discovery tool. |
| `archigraph_inspect` | Look up a single entity by ID, qualified name, or label. Returns full record + attached findings. |

### Traversal

| Tool | One-line purpose |
|------|-----------------|
| `archigraph_expand` | BFS neighbour expansion from a single node. |
| `archigraph_trace` | Confidence-weighted shortest path (Dijkstra) between two nodes. |
| `archigraph_traces` | Pre-computed process flow traces. Actions: `list`, `get`, `follow`. |
| `archigraph_module_analysis` | Module-level cycle detection, centrality, and cluster analysis. |

### Source

| Tool | One-line purpose |
|------|-----------------|
| `archigraph_get_source` | Return actual source lines for a node, by entity ID or label. |
| `archigraph_recent_activity` | Entities whose source files changed after a given RFC3339 timestamp. |

### HTTP endpoints

| Tool | One-line purpose |
|------|-----------------|
| `archigraph_endpoints` | Action-dispatched: `definitions` (server routes), `calls` (client call-sites), `stats`. |

### Topology (message bus)

| Tool | One-line purpose |
|------|-----------------|
| `archigraph_topology` | Action-dispatched: `topics`, `topic_detail`, `orphan_publishers`, `orphan_subscribers`. |

### Flows

| Tool | One-line purpose |
|------|-----------------|
| `archigraph_flows` | Action-dispatched: `list`, `detail`, `dead_ends`, `truncated`. |

### Patterns

| Tool | One-line purpose |
|------|-----------------|
| `archigraph_graph_patterns` | Action-dispatched: `list`, `get`. Agent-learned structural patterns. |

### Queue management

| Tool | One-line purpose |
|------|-----------------|
| `archigraph_enrichments` | List, submit, or reject enrichment candidates (endpoints, flows, topics). |
| `archigraph_cross_links` | List, accept, or reject cross-repo link candidates. |
| `archigraph_repairs` | List or submit residual-edge repair resolutions. |

### Quality

| Tool | One-line purpose |
|------|-----------------|
| `archigraph_quality` | Action-dispatched: `orphan_audit`, `recall_measurement`, `history`. |
| `archigraph_auth_coverage` | Map authentication coverage across HTTP endpoints. |

### Documentation (docgen)

| Tool | One-line purpose |
|------|-----------------|
| `archigraph_docgen_start_run` | Start a local-staging docgen run; returns `run_id` and `staging_path`. |
| `archigraph_docgen_validate` | Validate a staging run (frontmatter errors, broken links). |
| `archigraph_docgen_promote` | Atomically promote staging docs to canonical path. |
| `archigraph_docgen_status` | Status and progress of a docgen run. |
| `archigraph_docgen_list` | List all docgen runs for a group. |
| `archigraph_docgen_cancel` | Cancel a running docgen run. |

### Memory

| Tool | One-line purpose |
|------|-----------------|
| `archigraph_save_finding` | Persist a Q/A pair to the group's memory directory. |
| `archigraph_list_findings` | Read back saved findings, optionally filtered by entity or timestamp. |

---

## Renamed tools (pre-#668)

Tool names changed significantly in #668 and #1281. If you are using a saved configuration with old names, see the deprecation table in `internal/mcp/SCHEMA.md` — old names return a clear "tool not found" error (no silent fallback, per ADR-0017).

---

## Pairing with grep

archigraph MCP and grep are complementary. Use MCP for structural questions (who calls X, trace a flow, find callers). Use grep for raw enumeration (every `if err != nil`, every import line). See [CLAUDE.md](../CLAUDE.md) for the pairing guide with three worked examples.
