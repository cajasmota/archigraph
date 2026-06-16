# Troubleshooting

Symptoms, likely causes, and fixes. Run `grafel doctor` first — it catches the most common issues automatically.

---

## Daemon issues

### `grafel status` shows the daemon is not running

```sh
grafel start             # start the daemon
grafel doctor            # check for port conflicts, permission errors
```

The daemon listens on `http://127.0.0.1:47274`. If that port is occupied by another process, override it with the `GRAFEL_DASHBOARD_PORT` environment variable before starting the daemon (e.g. `GRAFEL_DASHBOARD_PORT=48000 grafel start`), then restart. For a standalone dev server you can also pass `grafel dashboard serve --port N`. There is no `daemon_port` field in `settings.json`.

### Dashboard shows a blank page or fails to load

The dashboard is embedded in the binary. If it shows a blank page:
1. Check the daemon is running: `grafel status`
2. Rebuild after an upgrade: `make build` then `grafel restart`
3. Check the daemon logs: `grafel doctor` prints the log path

---

## MCP issues

### Agent shows "grafel MCP not found" or no `grafel_*` tools

1. Confirm the daemon is running: `grafel status`
2. Confirm the MCP entry exists: `grafel status` should show `MCP: connected`
3. If not connected, re-run: `grafel install`
4. Restart your agent session — MCP servers are loaded at session start

### Agent is in the wrong group or returns `source: "none"` from `grafel_whoami`

The daemon resolves the group from the agent's working directory. If your CWD is not inside a registered repo:
- Run `grafel wizard` to register the repo
- Or pass `group=` explicitly in tool calls

### Tools return "tool not found" errors with old names

Tool names changed in #668 and #1281. There is no backwards-compatible fallback (ADR-0017). See the renamed-tools table in [mcp-tools.md](mcp-tools.md) or `internal/mcp/SCHEMA.md`.

---

## Indexing issues

### `grafel rebuild` completes but entity count is much lower than expected

1. Check language support: `grafel doctor` lists extractors and their status
2. Check the `.grafel/` directory: `ls .grafel/` should contain `graph.fb` and optionally `graph.json`
3. Verify CGO is enabled: `go env CGO_ENABLED` should print `1`. Without CGO, tree-sitter cannot compile and falls back to a reduced extractor set.

### Index hangs or the daemon uses too much RAM

- Adjust parallelism: `daemon_rss_budget_mb` and `indexer_parallelism` in `~/.grafel/settings.json`
- For large repos, increase `daemon_rss_budget_mb` beyond the default 512 MB
- Check for cycles in the graph that inflate BFS depth: run `/grafel-graph-quality` and inspect the `orphan_audit` output

### Multi-branch: wrong graph loaded after branch switch

grafel watches `.git/HEAD` for branch changes. If the reload did not happen:

```sh
grafel rebuild <group>    # force a reload
grafel branches           # list all indexed refs with HOT/WARM/COLD tier
```

See [user-guide/multi-branch.md](user-guide/multi-branch.md) for the full guide.

---

## Build issues

### `make build` fails on `dashboard-build`

```sh
node --version    # must be 20+
npm --version     # 8+ recommended
cd webui-v2 && npm ci && npx vite build    # run the dashboard build manually to see errors
```

### CGO errors on Linux

```sh
apt install build-essential    # install gcc and libc dev headers
CGO_ENABLED=1 go build ./...   # verify CGO is active
```

### CGO errors on Windows

Install MSYS2/MinGW64 and set `CC=x86_64-w64-mingw32-gcc` in your environment. The CI workflow in `.github/workflows/test.yml` shows the exact setup steps.

---

## Skills

### Slash commands not available in Claude Code

```sh
grafel doctor              # checks skill install status
grafel install --skills    # reinstall skills
```

Skills land in `~/.claude/skills/`. Confirm that directory is not excluded by your Claude Code config.

### Skill runs but produces no output or errors on MCP tools

Ensure the daemon is running and the group is registered before invoking any skill. Start with `/grafel-help` to get an orientation and confirm the MCP connection.
