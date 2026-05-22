# Pass 15 — Business domain model + glossary (BUSINESS tier)

The first business pass. Produce the product's business vocabulary: the nouns a
PM uses (Inspection, Deficiency, Jurisdiction, Customer, Order), each defined in
plain language. Later business passes (capabilities, journeys, rules) link into
this glossary, so it runs first.

> **READ FIRST:** `snippets/business-voice.md`. Every rule there is binding for
> this pass. Zero internal symbol names; PM audience; provenance only in the
> collapsed `<details>` block.

The business tier is **synthesised across the whole group**, not per repo. Even
though the file is written under one repo's `docs/business/` (see Output), its
content spans every repo's domain.

## Inputs

- `~/.archigraph/groups/<group>/domain.md` — owner-supplied domain framing (Pass 0).
- Every `~/.archigraph/docs/<group>/<repo-slug>/overview.md` and `~/.archigraph/docs/<group>/<repo-slug>/glossary.md` from the
  technical tier, if it was generated (the technical glossary is symbol-anchored;
  translate it to business voice — do not copy it).
- The graph (entities / data model) via the MCP tools below.

## Output

Write a single group-synthesised file:

```
~/.archigraph/docs/<group>/business/domain-glossary.md
```

`<primary-repo>` is the group's anchor repo — the one the orchestrator selected
in the tier-selection step (default: the backend/service repo with the most
entities; the webui aggregates all repos' `business/` trees into one Business
view, so a single location reads as one set). Use
`output-templates/business-domain-glossary.md`.

## Procedure

### Step 1 — Gather candidate domain concepts

Find the business entities (data model), not the code structure:

```
archigraph_find(question="core data model and domain entities", repo_filter=null, depth=2, token_budget=1500)
archigraph_find(question="what are the main business records the system stores", repo_filter=null, depth=2, token_budget=1200)
```

Also read each technical-tier `glossary.md` and `overview.md`. These already
name the domain; your job is to lift the **business** terms and drop the code.

### Step 2 — Group by business meaning

Multiple classes/tables often back ONE business concept (e.g. `Inspection`,
`InspectionItem`, `InspectionSerializer`, `inspection` table → the single
business term **Inspection**). Collapse them. The reader sees one term, not
four code artifacts.

Discard pure-plumbing entities (serializers, view-sets, repositories, DTOs) —
they are not domain concepts.

### Step 3 — Define each term in business language

For each business term: a plain-language definition, what it relates to (linked
to other glossary terms), and its lifecycle in business words if it has states.
Order alphabetically. Follow `output-templates/business-domain-glossary.md`.

### Step 4 — Anchors + provenance

Write headings first, then derive the `anchors:` frontmatter list per
`snippets/anchor-contract.md`. Put any code/file references ONLY inside the
collapsed `<details>` provenance block at the bottom.

### Step 5 — Verify + save

Run `snippets/verification-checklist.md` (business-voice section applies).

```
archigraph_save_finding(
  question="What is the business domain glossary for the <group> group?",
  answer="<file: ~/.archigraph/docs/<group>/business/domain-glossary.md>",
  type="business_domain",
)
```

Hand back to the orchestrator. Pass 16 (capabilities) and Pass 17 (journeys)
both link into this glossary, so they run after it.
