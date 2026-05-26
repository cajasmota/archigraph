# Phase 3 - With-MCP run

Answer every question from `questions.json` using **only archigraph MCP tools**. Record full per-question metrics. Your output is `with-mcp.json` in the run directory.

> **Context isolation:** This phase runs in a FRESH subagent context. You have NOT seen the grep-only results from Phase 2. Do not open `without-mcp.json` during this phase. This ordering is intentional: grep-only runs first (Phase 2) so that run cannot be contaminated by MCP results; you run second and must not look back.

## Allowed tools

You may call any archigraph MCP tool: `archigraph_whoami`, `archigraph_search`, `archigraph_describe`, `archigraph_related`, `archigraph_trace`, `archigraph_list_clusters`, `archigraph_get_source`, `archigraph_recent_activity`, `archigraph_list_link_candidates`, `archigraph_list_enrichment_candidates`, `archigraph_graph_stats`, `archigraph_patterns`, `archigraph_get_telemetry`, etc.

You may **not** call `rg`, `grep`, `Read` on source files, or any other non-MCP file-inspection tool. If a question is unanswerable through MCP alone, record `confidence: 0.0` and mark `unknown: true` - that is a valid result.

## Per-question protocol

For each question in `questions.json`:

1. Note the host's `usage_info` snapshot at question start (input/output/cache tokens emitted so far in this session).
2. Note `wall_clock_start` (monotonic time, RFC3339 nanos).
3. **Snapshot the daemon log byte-offset.** Read the current size of `~/.archigraph/logs/daemon.log` (`stat -f%z` on macOS, `stat -c%s` on Linux) and remember it as `log_start_offset` (= `$START`). If the file does not exist (rare; daemon may have just started and not yet flushed), set `log_start_offset = 0` and proceed.
4. Answer the question using archigraph MCP tools. Take as many tool calls as needed but stop when you reach a defensible answer.
5. Note `wall_clock_end`.
6. Note the host's `usage_info` at question end.
7. **Capture daemon RPC metrics.** Note the current log size as `$END`, then run:
   ```
   archigraph bench-capture rpc --start $START --end $END
   ```
   Merge the resulting JSON directly into this question's `metrics` block (`mcp_rpc_count`, `mcp_rpc_handler_ms_sum`, `mcp_rpc_handler_ms_p50`, `mcp_rpc_handler_ms_p99`, `mcp_rpc_per_tool`). If the command fails (missing binary, permissions, sandbox), set `mcp_rpc_count: null` and add `"mcp_rpc_capture_error": "<reason>"` to the artifact. See "Daemon RPC capture" below for the field semantics.
8. Compute the delta and record the metrics below.

## Daemon RPC capture (handler vs transport split)

**Why:** the daemon logs `[mcp-rpc] tool=<name> elapsed=<N>ms repo=<repo>` at `internal/daemon/mcp_rpc.go:193` for every RPC call dispatched. The `elapsed` value is the **handler** time — what the daemon spent actually computing the answer, exclusive of the JSON-RPC bridge transport. Wall-clock per-question time minus the sum of `elapsed` values is the **transport** time (bridge serialization, stdio pipe, host overhead).

This split is the whole point of the capture: a question with `wall=8000ms, handler_sum=7500ms` says the handler is the lever; a question with `wall=8000ms, handler_sum=400ms` says the transport is the lever.

**How to capture (step 7 above):** use the deterministic CLI helper:

```
archigraph bench-capture rpc \
  --log ~/.archigraph/logs/daemon.log \
  --start-offset $START \
  --end-offset $END
```

`$START` = `log_start_offset` snapshotted in step 3 (before the first MCP call).
`$END` = current log file size at step 7 (question end). The CLI reads the exact byte window, parses `[mcp-rpc] … elapsed=<N>ms` lines, and emits JSON ready to merge into `metrics`. Parsing regex, percentile math, and null rules are all encapsulated in the CLI (#2298). Do not re-implement them here.

**Field semantics** (canonical definition in `schema/with-mcp-artifact.schema.json`):

- `mcp_rpc_count` — number of `elapsed=` lines in the window.
- `mcp_rpc_handler_ms_sum` — sum of all elapsed_ms values.
- `mcp_rpc_handler_ms_p50` — median handler duration; `null` when count = 0.
- `mcp_rpc_handler_ms_p99` — 99th-percentile handler duration; `null` when count = 0.
- `mcp_rpc_per_tool` — per-tool `{ "count": N, "sum_ms": M }` map.

**Log rotation:** if the log shrank between step 3 and step 7, pass `--start-offset 0` and add a note in `notes`.

If `archigraph bench-capture rpc` fails (missing binary, permissions, sandbox), record `mcp_rpc_count: null` for every question and add an `"mcp_rpc_capture_error": "<reason>"` field. Phase 5 will surface the failure in the report rather than silently drop the split.

## Output schema (`with-mcp.json`)

```json
{
  "version": 1,
  "method": "with-mcp",
  "iteration": 1,
  "started_at": "<RFC3339>",
  "ended_at": "<RFC3339>",
  "results": [
    {
      "id": "q01",
      "answer": "<the agent's prose answer>",
      "confidence": 0.85,
      "unknown": false,
      "tool_calls": [
        {"tool": "archigraph_search", "args_digest": "sha256:...", "ok": true, "ms": 142}
      ],
      "tool_call_count": 4,
      "tools_used": ["archigraph_search", "archigraph_describe", "archigraph_related"],
      "metrics": {
        "input_tokens": 12345,
        "output_tokens": 678,
        "cache_read_tokens": 5000,
        "cache_creation_tokens": 0,
        "wall_clock_ms": 8421,
        "mcp_rpc_count": 4,
        "mcp_rpc_handler_ms_sum": 7980,
        "mcp_rpc_handler_ms_p50": 1850,
        "mcp_rpc_handler_ms_p99": 2410,
        "mcp_rpc_per_tool": {
          "archigraph_search": {"count": 2, "sum_ms": 3600},
          "archigraph_describe": {"count": 1, "sum_ms": 2120},
          "archigraph_related": {"count": 1, "sum_ms": 2260}
        }
      },
      "notes": "Mentioned archigraph_search returned 0 hits initially; widened query."
    }
  ]
}
```

## Token accounting

The host (Claude Code) provides `usage_info` per message. The total tokens for a question are the sums **across all agent messages emitted while answering it**:

- `input_tokens` += `usage_info.input_tokens + usage_info.cache_creation_input_tokens`
- `output_tokens` += `usage_info.output_tokens`
- `cache_read_tokens` += `usage_info.cache_read_input_tokens`

If the host does not surface `usage_info`, fall back to `len(text) / 4` for input and output respectively, and set `"estimated": true` on each result. Phase 5 will label the report accordingly.

## Honesty rules

- Do not retry indefinitely to "win" a question. Stop when you have a defensible answer or after a reasonable effort.
- Record tool failures verbatim in `tool_calls[].ok = false` with the error string in `notes`. This data is used to surface MCP failure modes in the report.
- If a tool returned partial/malformed data, note that in `notes`. Phase 5's "Issues encountered" section depends on this.
- Confidence should reflect honest uncertainty, not match the user's expected outcome.

## Privacy

- The `answer` field may include entity names, file paths, line numbers, kinds, and structural facts. It must **not** include source-code content. Reference snippets by `path:line`, not by embedded code.
- `tool_calls[].args_digest` is a SHA-256 of the arguments, not the raw arguments. This protects entity strings the user may consider private.

## Output

Write `with-mcp.json` to the run directory and print a one-line summary: `<n> questions answered, <unknown> unknown, total <tokens> tokens, total <wall_ms>ms`. Return control to the orchestrator. Phase 4 (quality judgment) reads both `without-mcp.json` and `with-mcp.json` after this phase completes.
