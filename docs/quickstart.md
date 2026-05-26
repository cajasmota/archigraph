# Quickstart

This page gets you from zero to a running graph in five commands. For a full install matrix (pre-built binaries, dev mode, Windows) see [install.md](install.md).

---

## Prerequisites

- **Go 1.25.5+** with CGO enabled. CGO is required because the tree-sitter extractor uses a C library.
  - macOS: `xcode-select --install` (Xcode Command Line Tools)
  - Debian/Ubuntu: `apt install build-essential`
- **Node.js 20+** and npm (used to build the embedded dashboard)
- **git**

> **Preview note:** Pre-built binaries and the `curl | bash` installer are not yet published. Build from source as shown below.

---

## 1. Build

```sh
git clone https://github.com/cajasmota/archigraph.git
cd archigraph
make build          # builds dashboard + binary -> ./archigraph
./archigraph --version
```

Optional — add to `PATH`:

```sh
go install -ldflags="-X main.commit=$(git rev-parse --short HEAD)" ./cmd/archigraph
# installs to ~/go/bin -- make sure ~/go/bin is on your PATH
```

---

## 2. Index your code

```sh
./archigraph wizard
```

The wizard asks you to point at a folder. It accepts:
- A single git repo
- A folder containing several git repos (they become one multi-repo group)
- A monorepo (auto-split into modules)

You can also use the **Add group** button in the dashboard after step 3.

---

## 3. Start the daemon + register MCP

```sh
./archigraph install <group>
```

This starts the daemon as a background service, registers the MCP server in your AI agent's config (Claude Code's `~/.claude/claude.json`, or equivalent for other clients), and installs the skill family to `~/.claude/skills/`.

To verify:

```sh
./archigraph status <group>
```

Output shows `MCP: connected` when the wiring is complete.

---

## 4. Open the dashboard

```sh
./archigraph dashboard    # opens http://127.0.0.1:47274 in your browser
```

The dashboard is embedded in the daemon binary — no separate server needed. Deep links and browser reloads work on every surface.

---

## 5. Query from your agent

Open a new Claude Code session in one of your indexed repos. The MCP server is auto-registered, so you can call archigraph tools immediately:

```
archigraph_whoami()      -- confirm group + repo
archigraph_stats()       -- entity counts, any unavailable repos
archigraph_clusters()    -- module map (Louvain communities)
```

For a complete guide to navigating with the MCP tools, see [../skills/using-archigraph/SKILL.md](../skills/using-archigraph/SKILL.md).

---

## Daemon control

```sh
archigraph start          # start daemon in background
archigraph stop           # stop daemon
archigraph restart        # restart daemon
archigraph status         # health check all groups
archigraph doctor         # full install smoke-check
```

After upgrading archigraph:

```sh
archigraph rebuild <group>    # force AST rebuild, no cache
```

---

## Uninstall

```sh
archigraph uninstall          # removes skills, MCP entry, stops daemon
archigraph uninstall --purge  # also removes ~/.archigraph/store/ (your graphs)
```
