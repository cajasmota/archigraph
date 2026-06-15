<!-- grafel:mcp-usage:start v=1 -->

## grafel MCP

This repo is part of grafel group **grafel**. grafel is an architecture knowledge graph available via MCP. When you (an AI coding agent) need to understand how this codebase fits together, prefer the grafel MCP tools over `grep` + reading files.

### When to use grafel instead of grep

| Question shape | Prefer |
|---|---|
| "Where is `X` defined?" | `grafel_find` |
| "What does `X` look like + its neighbors?" | `grafel_inspect` |
| "Who calls `X`?" | `grafel_expand` / `grafel_find_callers` |
| "End-to-end flow when user does X?" | `grafel_traces` |
| "How does the frontend talk to the backend?" | `grafel_cross_links` |
| "Show me the source of `X`" | `grafel_get_source` |

### When grep IS still better

- Substring search across all files for non-entity strings (comments, TODOs).
- Anything where you need raw file contents in bulk.

### Anti-patterns

- Don't read an entire file to find one function — `grafel_inspect` returns it directly.
- Don't glob for a class name across the repo — `grafel_find` indexes it.
- Don't traverse imports manually — `grafel_expand` does it via the IMPORTS edge.

The full agent guide is delivered automatically in the MCP `instructions` handshake when you connect.

_Do not edit between the markers — this block is auto-updated by `grafel install`._

<!-- grafel:mcp-usage:end -->