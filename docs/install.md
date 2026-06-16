# Install

Full install matrix for grafel. For the five-command path see [quickstart.md](quickstart.md).

---

## Prerequisites

| Requirement | Notes |
|-------------|-------|
| Go 1.26+ | CGO must be enabled (tree-sitter uses a C library) |
| C compiler | macOS: `xcode-select --install` / Debian-Ubuntu: `apt install build-essential` / Windows: MinGW via MSYS2 |
| Node.js 20+ + npm | Dashboard build only — not needed at runtime after `make build` |
| git | Required for indexing |

---

## macOS / Linux — installer script

> **Not yet published.** The script below will work after the first release ships. During the preview phase, build from source (see below).

```bash
curl -fsSL https://raw.githubusercontent.com/cajasmota/grafel/main/install.sh | bash
```

---

## Windows — PowerShell installer

> **Not yet published.** During the preview phase, build from source with MinGW.

```powershell
irm https://raw.githubusercontent.com/cajasmota/grafel/main/install.ps1 | iex
```

Windows builds require MSYS2/MinGW64. The installer handles this. Shipped binaries link statically against MinGW — users do not need MinGW installed at runtime.

---

## Pre-built binary (manual download)

> **Not yet published.** Will be at https://github.com/cajasmota/grafel/releases after the first release.

Archives per platform: `linux_x86_64`, `linux_arm64`, `macos_x86_64`, `macos_arm64`, `windows_x86_64`.

Extract the archive and move the `grafel` binary to a directory on your `PATH`.

---

## Build from source

This is the correct path during the preview phase.

```sh
git clone https://github.com/cajasmota/grafel.git
cd grafel
make build           # builds dashboard (npm ci + vite build) + Go binary
./grafel --version
```

`make build` runs `dashboard-build` (npm) then `go build`. If you only want the Go binary and the dashboard is already built:

```sh
make build-go-only
```

To install the binary system-wide:

```sh
go install -ldflags="-X main.commit=$(git rev-parse --short HEAD)" ./cmd/grafel
# installs to ~/go/bin -- ensure ~/go/bin is on PATH
```

---

## Dev mode (symlinked skills)

For contributors who want to edit skills in-place and see changes without reinstalling:

```sh
grafel install --dev
```

This creates symlinks in `~/.claude/skills/` pointing back to `skills/` in the source checkout instead of copying files.

---

## Post-install steps

```sh
grafel doctor               # smoke-check install + tools
grafel wizard               # create a group (interactive)
grafel install              # start daemon, register MCP, install skills
grafel status               # confirm MCP: connected
```

---

## Upgrade

```sh
grafel update               # fetch latest stable, atomic replace, re-run install
grafel update --pre         # latest pre-release
grafel update --tag v1.2.3  # pin a specific version
```

If the upgrade fails, the binary is automatically restored from the pre-update stash.

---

## Uninstall

```sh
grafel uninstall            # removes skills, MCP entries, stops daemon; leaves graphs
grafel uninstall --purge    # also removes ~/.grafel/store/ and docs/
```

Uninstall reads `install.json` to remove only what grafel owns — it will not touch other tools' MCP registrations.

---

## Configuration file

User preferences are persisted to `~/.grafel/settings.json`. See [settings.md](settings.md) for the full field reference.

---

## Troubleshooting

See [troubleshooting.md](troubleshooting.md).
