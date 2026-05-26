---
name: archigraph-resolve
description: Surface and resolve residual edges — runtime dispatch, dynamic URLs, ambiguous bindings — so the graph is fit-for-purpose before any downstream skill consumes it. Lists pending repair candidates, auto-resolves unambiguous ones via templates, and walks the user through the rest interactively.
when-to-use: User asks to "show me archigraph's residuals", "help me annotate runtime-resolved edges", "run the repair flow", "clean up residuals without regenerating docs", or invokes `/archigraph-resolve` explicitly. Also serves as the recommended first step before running `/archigraph-tech-docs` or `/archigraph-business-docs`.
---

# archigraph-resolve

Stand-alone resolution flow for ADR-0015 residual edges. Lists pending resolution candidates surfaced by `archigraph index`, walks the user through resolutions, and submits each via the `archigraph_repairs` MCP tool. Companion to `/generate-docs`, but can be invoked independently when the user just wants to clean up residuals without regenerating docs.

> **Backward-compatibility note:** The MCP tool name `archigraph_repairs` and on-disk file names `repair-history.json` / `repair-templates.json` are preserved for backward compatibility. A separate ticket will address the API rename.

## When to use this skill

Invoke it when the user asks for any of:

- "Show me archigraph's residuals."
- "Help me annotate the runtime-resolved edges."
- "Run the repair flow but don't regenerate docs."
- "I have N minutes — let me chip at the bug-rate."

Run this before `/archigraph-tech-docs` or `/archigraph-business-docs` to ensure the graph has resolved edges. Also run as a standalone step any time you want to chip at the residual count without regenerating docs.

Do **not** invoke it inside `/generate-docs`; that flow has its own integrated resolution passes (Pass 1a, 1b, 3a). Use this skill for ad-hoc cleanup outside the doc-gen pipeline.

## Inputs

- A resolved archigraph group (the skill calls `archigraph_whoami` first).
- Per-repo `<repo>/.archigraph/enrichment-candidates.json` with `kind: "repair_edge"` records (emitted by `archigraph index` per ADR-0015).
- Optional: `~/.archigraph/groups/<group>/repair-history.json` (prior answers).
- Optional: `~/.archigraph/groups/<group>/repair-templates.json` (saved templates).

If `archigraph_repairs(action=list, limit=1)` returns `total == 0` for every repo, the skill exits with "No residuals to resolve." after a one-line summary.

## Procedure

### Step 0 — Confirm scope

`archigraph_whoami` → confirm group + repo set with the user. If the user wants a single repo, capture the slug; otherwise iterate the full group.

### Step 1 — List residuals

For each repo `<r>`:

```
archigraph_repairs(action=list, repo_filter=["<r>"], limit=50, offset=0)
```

Continue paging while `len(residuals) == limit`. Display per-repo counts to the user up front:

> archigraph has **N** residuals across this group:
> - `<repo-a>` — 47 (mostly CALLS to dynamic URLs).
> - `<repo-b>` — 12 (third-party SaaS).
> - `<repo-c>` —  3 (cross-repo HTTP).
>
> Want me to walk through all of them, just the top 10 by centrality, or a specific repo?

### Step 2 — Apply templates and history (silent)

Before prompting the user, auto-resolve anything that:

- Matches a template in `repair-templates.json` with `confidence >= 0.8`, OR
- Has a prior successful resolution in `repair-history.json` keyed by `residual_id`.

Submit those via `archigraph_repairs(action=submit, source="archigraph-resolve/auto")` and tell the user how many were auto-applied:

> Auto-applied 14 from templates / 6 from prior history. 42 left to walk.

### Step 3 — Walk remaining residuals

Same Q-shape as `/generate-docs` Pass 1b — one residual at a time:

> **<repo>** · <from_entity.kind> `<from_entity.name>` <relation> `<original_stub>`
>
> Likely resolutions:
> - **A.** Bind to `<candidate-id>` (suggested target from the local subgraph).
> - **B.** Reclassify as dynamic (runtime URL / dispatch).
> - **C.** Reclassify as external (third-party).
> - **D.** Abandon.
> - **S.** Skip for now.

Translate the answer to `archigraph_repairs(action=submit, ..., source="archigraph-resolve")` using the same translation table as Pass 1b.

### Step 4 — Handle rejections

Same retry loop as Pass 1b. Rejection reason codes (`target_entity_not_found`, `self_loop_disallowed`, `contradicts_contains_hierarchy`, `invalid_module_identifier`, `missing_required_field`, `reasoning_too_short`) are surfaced to the user in plain language and re-asked.

### Step 5 — Promote templates

When the user answers ≥3 residuals with the same `resolution` + matching shape (same `relation` + similar `original_stub`), prompt:

> You've classified 3 calls to `/${tenantId}/<path>` as `reclassify_as_dynamic`. Want me to save this as a template so I auto-apply on the rest?

If yes, append to `repair-templates.json` (schema in Pass 1a). The template applies on the next sweep and on the remaining residuals in this run.

### Step 6 — Update history

After every submit, append the Q/A pair to `repair-history.json`. Same schema as Pass 1b Step 6.

### Step 7 — Summary

End with:

> Submitted **K** resolutions (`A` auto from templates, `H` auto from history, `U` from your answers). Run `archigraph index <repo>` to apply them.

Optionally offer to invoke `archigraph index` for the affected repos. The skill does **not** invoke it automatically — re-indexing has side effects the user should consent to.

## Outputs

- Side effects: zero or more `archigraph_repairs(action=submit)` calls.
- `~/.archigraph/groups/<group>/repair-history.json` — appended.
- `~/.archigraph/groups/<group>/repair-templates.json` — possibly extended with new templates.
- `~/.archigraph/groups/<group>/repair-session-<rfc3339>.md` — human-readable transcript of this session (count, applied list, deferred list).

## archigraph MCP tool surface

- `archigraph_whoami` — group/repo resolution.
- `archigraph_repairs(action=list|submit)` — primary tool.
- `archigraph_describe`, `archigraph_related`, `archigraph_search` — for inspecting candidate targets when the user asks "what would that bind to?".

## Quality gates

Before exit, the skill verifies:

- Every `submit` returned without a `rejected_reason`, OR the rejection was surfaced and either re-resolved or recorded as deferred.
- `repair-history.json` writes are atomic (write-temp-then-rename) so a Ctrl-C mid-session does not corrupt prior history.
- No template was promoted with `applied_count < 3` (guards against single-example over-generalisation).

## Passes absorbed from generate-docs

This skill absorbs and supersedes two passes that previously lived inside
`/generate-docs`. Run this skill standalone to get the same resolution
behaviour without triggering a full doc-gen pipeline.

- **Pass 1a** (`prompts/01a-residual-repair-sweep.md`) — pre-Q&A sweep: auto-resolves residuals matching saved templates (confidence ≥ 0.8) and known third-party stubs. Does NOT attempt `bind_to_entity` resolutions — those go to Pass 1b or the 3a hook inside tech-docs.
- **Pass 1b** (`prompts/01b-repair-aware-qa.md`) — interactive Q&A: walks the user through residuals 1a could not auto-resolve; each answer becomes an `archigraph_repairs(action=submit)` call.

## Related

- `skills/generate-docs/SKILL.md` — for the integrated resolution flow inside doc generation (Pass 1a, 1b, 3a).
- ADR-0015 (`docs/adrs/0015-residual-repair-agent-enrichment.md`) — design rationale.
- `docs/specs/repair-trust-model.md` — allowlist + verification rules enforced by the MCP tool.
- `internal/mcp/SCHEMA.md` §`archigraph_repairs` — tool reference.

## Read next

After resolving residuals, check graph health before spending tokens on documentation:
→ `/archigraph-graph-quality` — benchmark MCP vs grep+read to confirm the foundation is solid.
