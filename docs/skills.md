# Skills

archigraph ships a family of Claude Code slash-command skills. Each skill owns one concern and is idempotent — safe to re-run after any graph change.

The canonical reference is:

**[`skills/README.md`](../skills/README.md)**

This page is a pointer and summary. Do not duplicate the skills/README.md content here.

---

## Install

Skills are installed automatically by `archigraph install`. To refresh after an upgrade:

```sh
archigraph install --skills    # refresh skills only
archigraph doctor              # check which skills are installed and up-to-date
```

Skills land in `~/.claude/skills/` where Claude Code discovers them.

---

## Skill chain (summary)

```
/archigraph-resolve            -- fix residual edges (run first)
    |
    +-- /archigraph-graph-quality   -- benchmark graph health
    +-- /archigraph-graph-enrich    -- populate Paths/Flows/Topology panels
    |
    +-- /archigraph-tech-docs       -- engineer-facing module docs (13 passes)
    +-- /archigraph-business-docs   -- PM-facing capabilities + journeys
    |
    +-- /archigraph-security-audit  -- static + LLM security audit
    +-- /archigraph-consult         -- 5-persona consultant panel
```

For the full chain diagram, hard vs. soft dependencies, all 14 skills, and the decision table ("which skill for my goal?"), see [skills/README.md](../skills/README.md).

---

## Minimum useful run (first time, ~30 min, ~$5-$20)

```
/archigraph-resolve
/archigraph-graph-enrich
```

Result: a queryable, dashboard-rich graph. No prose docs yet.

## Add documentation

```
/archigraph-tech-docs
```

Produces per-module READMEs, API reference, cross-cutting concerns, and group synthesis. Runs in 25 min to 4 h depending on repo size.
