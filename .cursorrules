<!-- archigraph:mcp-usage:start v=1 -->

## archigraph MCP

This repo is part of archigraph group **archigraph**. archigraph is an architecture knowledge graph available via MCP. When you (an AI coding agent) need to understand how this codebase fits together, prefer the archigraph MCP tools over `grep` + reading files.

### When to use archigraph instead of grep

| Question shape | Prefer |
|---|---|
| "Where is `X` defined?" | `archigraph_find` |
| "What does `X` look like + its neighbors?" | `archigraph_inspect` |
| "Who calls `X`?" | `archigraph_expand` / `archigraph_find_callers` |
| "End-to-end flow when user does X?" | `archigraph_traces` |
| "How does the frontend talk to the backend?" | `archigraph_cross_links` |
| "Show me the source of `X`" | `archigraph_get_source` |

### When grep IS still better

- Substring search across all files for non-entity strings (comments, TODOs).
- Anything where you need raw file contents in bulk.

### Anti-patterns

- Don't read an entire file to find one function — `archigraph_inspect` returns it directly.
- Don't glob for a class name across the repo — `archigraph_find` indexes it.
- Don't traverse imports manually — `archigraph_expand` does it via the IMPORTS edge.

The full agent guide is delivered automatically in the MCP `instructions` handshake when you connect.

_Do not edit between the markers — this block is auto-updated by `archigraph install`._

<!-- archigraph:mcp-usage:end -->