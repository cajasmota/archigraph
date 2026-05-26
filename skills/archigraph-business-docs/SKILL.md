---
name: archigraph-business-docs
description: Generate group-synthesised, PM-facing documentation — product capabilities, business domain glossary, user journeys as plain-language narratives, business rules reverse-engineered from code, and a business overview landing page. Zero internal symbols. Does NOT require tech docs to have been run first — graph-only fallback is built into every pass.
when-to-use: User asks to "write business docs", "generate PM-facing docs", "document capabilities", "write user journeys", "produce the business overview", or invokes /archigraph-business-docs explicitly. Also invoked standalone when a non-engineer stakeholder needs to understand the product without reading code.
---

# archigraph-business-docs

Generate the business-tier documentation set for a registered archigraph group. All output is synthesised across every repo in the group and written in plain business language for PMs, designers, and non-engineers.

## Key design point: independent of tech docs

This skill does **NOT** hard-depend on `/archigraph-tech-docs` having been run. Every business pass has an explicit graph-only fallback: when technical-tier module READMEs are absent, the pass derives the same content directly from the graph via MCP queries. Running `/archigraph-tech-docs` first improves fidelity (the business writers can reference engineer-written module names already translated to business voice), but it is not required.

**Soft dependency path:** `archigraph-resolve` → `archigraph-business-docs` (minimum viable).
**Enhanced path:** `archigraph-resolve` → `archigraph-tech-docs` → `archigraph-business-docs` (higher fidelity).

## CRITICAL TOOL DISCIPLINE

For ANY structural question about the codebase, use archigraph MCP tools: `archigraph_whoami`, `archigraph_find`, `archigraph_inspect`, `archigraph_expand`, `archigraph_traces`, `archigraph_clusters`, `archigraph_stats`. Do NOT grep or read source files for entity discovery.

### Pre-flight assertion

Call `archigraph_whoami` before doing anything. If it errors: ABORT with "archigraph MCP not configured for this directory. Run `/mcp` to fix, then re-invoke `/archigraph-business-docs`."

## When to use this skill

- "Write business docs / PM docs for this group."
- "Generate product capabilities and user journeys."
- "Produce the business overview."
- "Document what the product does in plain English."
- `/archigraph-business-docs` (slash command).

Do **not** invoke for technical module documentation (that is `/archigraph-tech-docs`).

## Business voice contract

Every business pass reads `snippets/business-voice.md` first (if present). The contract: PM audience, zero internal symbol names, zero code-style mermaid diagrams. All pages carry a collapsed `<details>` provenance block at the bottom (the ONLY place a file path or symbol may appear) so an engineer can audit without polluting the PM reading experience.

## Staging-dir + atomic promote architecture

Same pattern as `/archigraph-tech-docs`:
1. Pass 15 (first pass) calls `archigraph_docgen_start_run(group="<group>")` → receives `run_id` and `staging_path`.
2. Passes 15–19 write all files into `<staging_path>/<relative-path>`.
3. Pass 19 (last pass) calls `archigraph_docgen_validate(run_id)` then `archigraph_docgen_promote(run_id)`.

## Pass chain

Run in this order (Pass 15 first, Pass 19 last — Pass 19 indexes the others):

| Pass | Prompt | Purpose | Est. time |
|------|--------|---------|-----------|
| 15 | `prompts/15-business-domain.md` | Business domain model + glossary: business nouns defined in plain language | 5–10 min |
| 16 | `prompts/16-business-capabilities.md` | Product capabilities: what the system does + why, grouped by business outcome | 8–20 min |
| 17 | `prompts/17-business-journeys.md` | User journeys as plain-language narratives | 5–15 min |
| 18 | `prompts/18-business-rules.md` | Business rules reverse-engineered from validation/permission/conditional logic | 5–15 min |
| 19 | `prompts/19-business-overview.md` | Business landing page: pitch + indexes into capabilities, journeys, glossary, rules. Runs last. | 3–5 min |

**Total wall time:** 25 min – 1 h depending on group size.

## Output layout

```
~/.archigraph/docs/<group>/business/
  overview.md                   # Pass 19 — landing page
  capabilities/
    <capability-slug>.md        # Pass 16 — one per product capability
  domain-glossary.md            # Pass 15 — business vocabulary
  journeys/
    <journey-slug>.md           # Pass 17 — plain-language user journeys
  rules/
    index.md                    # Pass 18 — business rules
    <area>.md                   # optional — if one area's rules outgrow index.md
```

The business tier is **not per-repo**. It is synthesised across every repo in the group and written to the single group-level `business/` directory. The webui surfaces this under the Business chooser tab.

## archigraph MCP tool surface

- `archigraph_whoami` — group/repo resolution.
- `archigraph_find`, `archigraph_inspect`, `archigraph_expand` — entity navigation.
- `archigraph_traces`, `archigraph_clusters` — flow and cluster navigation.
- `archigraph_enrichments` — read enriched frontmatter for endpoint/flow summaries.
- `archigraph_save_finding` — persist domain interview answers.
- `archigraph_docgen_start_run`, `archigraph_docgen_validate`, `archigraph_docgen_promote` — staging lifecycle.

## Related

- `skills/archigraph-resolve/SKILL.md` — recommended prerequisite.
- `skills/archigraph-tech-docs/SKILL.md` — optional soft dependency; improves fidelity.
- `skills/archigraph-graph-enrich/SKILL.md` — enriched endpoint/flow summaries feed business capability descriptions.
- `skills/generate-docs/SKILL.md` — the monolith this skill was extracted from.

## Read next

After generating business docs, run deeper analysis or present findings to stakeholders:
→ `/archigraph-security-audit` — two-phase security audit; the business-analyst persona reads these docs.
→ `/archigraph-consult` — panel of specialist personas including business analyst and architect; hard-depends on tech docs but soft-depends on business docs.
