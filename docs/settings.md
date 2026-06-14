# Settings file reference

archigraph persists user preferences in `~/.archigraph/settings.json`.
The file is created automatically on first write (via the Settings surface
or the `PUT /api/settings` endpoint). All fields are optional; missing keys
fall back to the defaults below.

## Schema

```json
{
  "theme": "light",
  "default_group": "",
  "auto_check_updates": true,
  "update_channel": "stable",
  "refresh_schedule": "",
  "telemetry_enabled": false,
  "daemon_rss_budget_mb": 512,
  "watcher_debounce_secs": 2,
  "indexer_parallelism": 4,
  "log_level": "info"
}
```

## Field reference

| Field | Type | Default | Allowed values | Notes |
|-------|------|---------|----------------|-------|
| `theme` | string | `"light"` | `"light"` \| `"dark"` \| `"auto"` | `"auto"` follows the OS dark-mode preference |
| `default_group` | string | `""` | Any registered group slug | Group shown on first dashboard load; empty = most recently used |
| `auto_check_updates` | bool | `true` | `true` \| `false` | Polls GitHub releases on daemon start |
| `update_channel` | string | `"stable"` | `"stable"` \| `"dev"` | `"dev"` includes release candidates |
| `refresh_schedule` | string | `""` | cron expression or `""` | Empty = manual-only refresh; e.g. `"0 3 * * *"` for 03:00 daily |
| `telemetry_enabled` | bool | `false` | `true` \| `false` | Opt-in anonymous usage metrics |
| `daemon_rss_budget_mb` | int | `512` | 100–2000 | Maximum RSS the daemon may use before it sheds loaded graphs. **Requires daemon restart.** |
| `watcher_debounce_secs` | int | `2` | 1–60 | How long to wait after a file change before triggering a re-index |
| `indexer_parallelism` | int | `4` | 1–32 | Number of parallel goroutines used during indexing. **Requires daemon restart.** |
| `log_level` | string | `"info"` | `"debug"` \| `"info"` \| `"warn"` \| `"error"` | Daemon log verbosity |

## CPU / concurrency / reindex environment variables

These knobs are read from the daemon's environment (not `settings.json`) and
let you dial CPU usage without recompiling. They take effect on the next daemon
start — set them in whatever launches `archigraph start` (your shell profile,
a launchd/systemd unit, etc.). Issues [#5134](https://github.com/cajasmota/archigraph/issues/5134)
and [#5135](https://github.com/cajasmota/archigraph/issues/5135).

archigraph distinguishes two reindex paths and caps them independently:

- **Background reindex** — the watcher/scheduler re-indexing a repo after a
  file save, merge, or branch switch. On a high-churn repo this can fire
  continuously, so it is throttled by default.
- **Explicit rebuild** — a user-triggered `archigraph rebuild` / `archigraph index`
  (or a dashboard rebuild). You are waiting on it, so it runs at host speed.

The CPU caps below only bite when the **subprocess extractor** is enabled
(`ARCHIGRAPH_SUBPROC_EXTRACT=1`), which forks one `archigraph extract` child
per file batch. When it is off (the default), the daemon extracts in-process and
`ARCHIGRAPH_DAEMON_GOMAXPROCS` / the generic `GOMAXPROCS` are the relevant knobs.

| Env var | Default | Path | Effect / when to change |
|---------|---------|------|--------------------------|
| `ARCHIGRAPH_EXTRACT_GOMAXPROCS` | `2` | background | `GOMAXPROCS` set on each **background** extract subprocess. Total extract draw ≈ `concurrency × this`. Lower to `1` to throttle a daemon that's burning CPU on a high-churn repo; raise if background reindexes feel slow and the host is idle. |
| `ARCHIGRAPH_REBUILD_GOMAXPROCS` | host cores (`NumCPU`) | explicit | `GOMAXPROCS` set on each **explicit-rebuild** extract subprocess. Defaults to the full host so a user-triggered rebuild runs fast (it is no longer throttled by the background cap — [#5135](https://github.com/cajasmota/archigraph/issues/5135)). Lower it if explicit rebuilds on a shared host need to leave headroom for CI. |
| `ARCHIGRAPH_EXTRACT_CONCURRENCY` | auto (`NumCPU/2`, capped at 4 background; `NumCPU` explicit) | both | Hard ceiling on the number of concurrent extract subprocesses, honored on **both** paths. Set to `1` as an emergency throttle on a contended machine — it caps explicit rebuilds too. |
| `ARCHIGRAPH_DAEMON_GOMAXPROCS` | unset (Go default = host cores) | in-process | Caps the **daemon's own** Go-runtime parallelism (in-process extraction, reindex, GC, algorithm passes) without the generic `GOMAXPROCS`. Use this when in-process indexing (subprocess extractor off) is the CPU source. **Tradeoff:** query handling shares the same process, so lowering this also lowers the ceiling on concurrent query throughput. Ignored when ≥ host cores. |
| `GOMAXPROCS` | host cores | in-process | Standard Go knob. Caps the entire daemon process (queries **and** in-process indexing). Prefer `ARCHIGRAPH_DAEMON_GOMAXPROCS` (same effect, archigraph-scoped name) or the per-subprocess caps above, which don't touch query latency. |
| `ARCHIGRAPH_INCREMENTAL_REINDEX` | `0` (off) | background | `1` switches single-file edits to a diff-aware incremental reindex (only changed files are re-extracted) instead of a full repo reindex on every settle. Recommended on high-churn repos to cut continuous reindex thrash. |
| `ARCHIGRAPH_REBUILD_CONCURRENCY` | auto (memory-tuned, floor 2, cap 16) | explicit | Number of **repos** indexed in parallel during a group rebuild (distinct from subprocess fan-out *within* a repo). Auto-tuned from system RAM; override to bound memory on constrained hosts. (Legacy alias: `ARCHIGRAPH_MAX_CONCURRENT_GROUPS`.) |

### Runtime reload without restart (#5137)

The four CPU/concurrency caps —
`ARCHIGRAPH_EXTRACT_GOMAXPROCS`, `ARCHIGRAPH_REBUILD_GOMAXPROCS`,
`ARCHIGRAPH_EXTRACT_CONCURRENCY`, and `ARCHIGRAPH_DAEMON_GOMAXPROCS` — can be
changed **without restarting the daemon** via a JSON config file at
`~/.archigraph/cpu.json` (under `$ARCHIGRAPH_DAEMON_ROOT` when set):

```json
{
  "extract_gomaxprocs": 1,
  "rebuild_gomaxprocs": 8,
  "extract_concurrency": 2,
  "daemon_gomaxprocs": 4
}
```

Precedence per knob is **env var > `cpu.json` > built-in default** (an explicit
`--flag`/config field still wins over all three). The env vars themselves are
still read once at process start and cannot change in a running daemon —
`cpu.json` is the live-mutable surface.

- The three **per-subprocess** caps (`extract_gomaxprocs`,
  `rebuild_gomaxprocs`, `extract_concurrency`) are re-read cheaply (mtime-cached)
  at the start of **every reindex**: edit `cpu.json` and the new value applies on
  the next reindex, no restart.
- The **in-process** `daemon_gomaxprocs` is applied live on `SIGHUP`
  (`kill -HUP <daemon-pid>`): the daemon re-reads `cpu.json` and calls
  `runtime.GOMAXPROCS` immediately. Clearing the cap restores the host default.

The remaining knobs in this doc (`daemon_rss_budget_mb`, `indexer_parallelism`,
`ARCHIGRAPH_REBUILD_CONCURRENCY`, the `ARCHIGRAPH_MAX_*` budgets) are still
**read at daemon start** and require `archigraph restart` to apply.

## API endpoints

| Method | Path | Description |
|--------|------|-------------|
| `GET` | `/api/settings` | Return current settings + defaults |
| `PUT` | `/api/settings` | Partial or full update; returns which fields require a restart |
| `POST` | `/api/settings/reset` | Restore factory defaults |

### `GET /api/settings` response

```json
{
  "settings": { ... },
  "defaults": { ... }
}
```

### `PUT /api/settings` response

The server merges the request body onto the current settings (partial PUT is
safe). Fields that require a daemon restart are listed in `restart_required`:

```json
{
  "settings": { ... },
  "defaults": { ... },
  "restart_required": ["daemon_rss_budget_mb"]
}
```

## Dashboard

The Settings surface at `/settings` in the dashboard provides a GUI for all
fields above and displays a "restart required" banner when applicable.
