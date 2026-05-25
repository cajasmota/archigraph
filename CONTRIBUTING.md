# Contributing to archigraph

## CI overview

CI is **minutes-aware**: workflows run only when they add signal. Full 3-platform tests are opt-in, not the default.

### What runs on every PR

| Workflow | Cost | Always? |
|---|---|---|
| `board-hygiene` (closure-keyword check) | ~5 s | Yes — all PRs |
| `test` (3-platform: ubuntu / macos / windows) | ~30 min | Only when warranted (see below) |
| `linux-smoke` | ~3 min | Post-merge + tag only |

---

### When does `test` run on a PR?

`test` fires automatically when your diff touches any of:

```
cmd/**
internal/**
vendor/**
go.mod
go.sum
Makefile
.github/workflows/test.yml
```

Pure-docs / pure-asset PRs (`.md`, `docs/**`, `*.png`, `skills/**`, `testdata/**`) run **zero** test jobs.

---

### Opt-in: `ci:full` label

Apply the **`ci:full`** label to force full 3-platform CI on any PR, regardless of which files changed.

**When to use it:**

- You changed something outside the auto-trigger paths but still want cross-platform validation (e.g. a Makefile outside the root, a shell script that affects all platforms, a `webui-v2` change you want to confirm doesn't break the Go build).
- You want to verify a docs-only PR's surrounding infrastructure hasn't regressed.
- You're about to merge and want extra confidence.

**How to apply:**

In the GitHub PR sidebar → Labels → select `ci:full`. The `pull_request_target: labeled` trigger will re-run `test` immediately after you apply the label.

---

### Opt-out: `board:exempt` label

Apply **`board:exempt`** to skip the closure-keyword check on chore PRs that legitimately don't map to an open issue:

- Typo fixes in docs
- `.gitignore` / `.editorconfig` tweaks
- CI formatting cleanups
- Emergency hotfixes that predate issue tracking

The label is checked in the `board-hygiene` workflow. Applying `board:exempt` (or removing it) re-triggers the workflow immediately via the `labeled`/`unlabeled` events, so the check resolves without requiring a re-push.

---

### Manual run: `workflow_dispatch`

Any workflow that supports `workflow_dispatch` can be triggered from the **Actions tab** in GitHub:

1. Go to `Actions` → select the workflow (e.g. `test`, `Linux Smoke Test`, `quality`).
2. Click **Run workflow** → choose a branch → click the green button.

Use this when you want to run CI on a branch that wouldn't otherwise trigger it automatically (e.g. a WIP branch, a branch that only touches docs).

---

### Where smoke runs

`linux-smoke` runs **only on push to `main`** and on **tag pushes (`v*`)**. It is not a PR gate — its job is post-merge sanity: confirm the binary builds and indexes a golden fixture before the commit is considered stable.

---

### Release pipeline

Pushing a tag matching `v*` triggers the full release pipeline (`release.yml`):

- 5-platform binary builds (linux amd64/arm64, macos amd64/arm64, windows amd64)
- Checksums + GitHub Release creation
- Smoke also fires on the tag push

Tags should only be pushed from `main` after all CI is green.

---

### Scenario reference

| Scenario | Workflows triggered |
|---|---|
| PR with only `.md` / `docs/**` changes | `board-hygiene` only |
| PR touching `internal/` or `cmd/` | `board-hygiene` + `test` (3 platforms) |
| PR with `board:exempt` label (docs-only) | `board-hygiene` (passes via label), no `test` |
| PR with `ci:full` label (any diff) | `board-hygiene` + `test` (3 platforms) |
| Push to `main` | `linux-smoke` + `test` |
| Tag push (`v1.2.3`) | `linux-smoke` + `release` pipeline |
| `workflow_dispatch` from Actions UI | Whichever workflow you trigger |
