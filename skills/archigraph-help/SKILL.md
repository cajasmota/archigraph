---
name: archigraph-help
description: Overview of the archigraph skill family. Lists all skills with one-line purposes, shows the canonical execution chain, and suggests which skill to invoke first based on your goal. Does NOT run any analysis — purely informational.
when-to-use: User asks "what archigraph skills are there", "how do I use archigraph", "where do I start", "what order should I run the skills", "which skill should I use", or invokes /archigraph-help explicitly. Also useful as a first step when returning to a group after time away.
---

# archigraph-help

Overview of the archigraph skill family. Use this to orient yourself or to suggest the right starting point to a new team member.

## Skill family

| Skill | One-line purpose |
|-------|-----------------|
| `/archigraph-resolve` | Surface and resolve residual edges (runtime dispatch, dynamic URLs). Run first. |
| `/archigraph-graph-quality` | Benchmark MCP vs grep+read on ~10–15 questions. Run to confirm graph health before spending tokens. |
| `/archigraph-graph-enrich` | Emit YAML frontmatter for endpoints/flows/topics so the dashboard Paths, Flows, Topology panels display data. |
| `/archigraph-tech-docs` | Generate per-module technical documentation for engineers. The big one — 13 passes, 25 min – 4 h. |
| `/archigraph-business-docs` | Generate PM-facing capabilities, user journeys, business rules synthesised across the group. Independent of tech docs. |
| `/archigraph-security-audit` | Two-phase security audit: static analysis (free) + LLM confirmation (interactive). |
| `/archigraph-consult` | Panel of 5 specialist personas (architect, security auditor, business analyst, performance reviewer, refactor critic). Requires tech docs. |
| `/archigraph-patterns-discover` | Discover recurring structural patterns across the group. Standalone. |
| `/archigraph-patterns-sync` | Bidirectional sync of pattern markers with CLAUDE.md files. |
| `/archigraph-aware-review` | PR-review-time skill that uses the graph to add context to code reviews. |
| `/archigraph-test-page` | Single-entity smoke test of the LLM docgen loop. Debugging tool. |
| `/extend-convention` | Generate a stack convention file for a new language/framework. |
| `/using-archigraph` | Orientation skill explaining how to use archigraph day-to-day. |
| `/archigraph-help` | This skill. |

## Canonical execution chains

### Minimum useful (first time on a new repo, ~30 min, ~$5–$20)
```
/archigraph-resolve
/archigraph-graph-quality     (optional but recommended)
/archigraph-graph-enrich
```
Result: a queryable, dashboard-rich graph. No prose docs yet.

### Technical documentation (adds 25 min – 4 h)
```
/archigraph-resolve
/archigraph-graph-enrich      (optional)
/archigraph-tech-docs
```

### Business documentation (independent, adds 25 min – 1 h)
```
/archigraph-resolve
/archigraph-business-docs
```
Does NOT require tech docs. Graph-only fallback is built in.

### Full pipeline (pre-release / pre-audit, 1–6 h)
```
/archigraph-resolve
/archigraph-graph-quality
/archigraph-graph-enrich
/archigraph-tech-docs
/archigraph-business-docs
/archigraph-security-audit
/archigraph-consult
```

### Daily maintenance (after a commit, < 10 min)
```
/archigraph-resolve --delta-only
/archigraph-graph-enrich --delta-only
/archigraph-tech-docs --delta-only
```

## Which skill should I start with?

| Your goal | Start here |
|-----------|-----------|
| "Is the graph trustworthy?" | `/archigraph-graph-quality` |
| "Fix dangling edges / residuals" | `/archigraph-resolve` |
| "Make dashboard panels show data" | `/archigraph-graph-enrich` |
| "Document the code for engineers" | `/archigraph-tech-docs` |
| "Document the product for PMs" | `/archigraph-business-docs` |
| "Find security issues" | `/archigraph-security-audit` |
| "Get an expert second opinion" | `/archigraph-consult` |
| "Find recurring patterns" | `/archigraph-patterns-discover` |
| "Review a PR with graph context" | `/archigraph-aware-review` |
| "Set up a new stack" | `/extend-convention` |
| "Just got started" | `/using-archigraph` |

## Dependency quick-reference

```
archigraph-resolve
  └─(soft)─> archigraph-graph-quality
  └─(soft)─> archigraph-graph-enrich
  ├─(hard)─> archigraph-tech-docs
  │             └─(hard)─> archigraph-consult
  ├─(hard)─> archigraph-business-docs
  │             └─(soft)─> archigraph-consult
  └─(hard)─> archigraph-security-audit
                └─(soft)─> archigraph-consult
```

Legend: `hard` = must complete before consumer starts. `soft` = improves quality but not required.

## Install

All skills ship with the archigraph binary and are installed by `archigraph install` or `archigraph install --dev`. To check which skills are installed and up-to-date:

```bash
archigraph doctor
```

To refresh all skills after an archigraph upgrade:

```bash
archigraph install --skills
```
