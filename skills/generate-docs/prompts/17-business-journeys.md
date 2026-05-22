# Pass 17 — User journeys as business narrative (BUSINESS tier)

Produce end-to-end user journeys written as PLAIN-LANGUAGE narratives — a user
accomplishing a goal across the whole product. This pass exists specifically to
fix the audit finding that the old `user-journeys.md` was a 60-step mermaid
sequence diagram naming internal symbols. That artifact belongs in the technical
tier; here we write the business version.

> **READ FIRST:** `snippets/business-voice.md`. Binding. The hard rule: NO code
> sequence diagrams, NO internal symbols, NO API paths. A simple business-step
> `flowchart` (≤ 8 business-labelled boxes) is the ONLY diagram allowed, and only
> if it adds something the prose doesn't.

Synthesised across the whole group — a journey typically crosses repos (mobile
app → backend → office web).

## Inputs

- `<primary-repo>/docs/business/domain-glossary.md` (Pass 15).
- `<primary-repo>/docs/business/capabilities/*.md` (Pass 16) — link into them.
- Process flows / call chains and cross-repo links from the graph.
- Any technical-tier journey/flow pages — translate to business voice, do not
  copy. Demote their mermaid sequence diagrams; they stay in the technical tier.

## Output

```
<primary-repo>/docs/business/journeys/<journey-slug>.md   # one per journey
```

Use `output-templates/business-journey.md`. A journey-slug is a goal in
kebab-case (e.g. `field-inspection-day`, `customer-places-order`,
`onboard-new-building`).

## Procedure

### Step 1 — Identify the goals

A journey is a goal a real user accomplishes, end to end. Find them from:

```
archigraph_flows(repo_filter=null, limit=200)
archigraph_traces(action=list, repo_filter=null, limit=50)
archigraph_cross_links(action=list, limit=200)   # cross-repo legs of a journey
```

Plus the capability set from Pass 16 (a journey usually chains several
capabilities). Pick the handful of journeys that matter to the business
(typically 3–8). Do not enumerate every code path.

### Step 2 — Write the narrative

A NUMBERED list of plain-language steps: what the user does, sees, decides; what
the system does for them — in business terms. Cross-repo legs become natural
sentences ("the inspection syncs to the office when the device is back online"),
never "`core-mobile` calls `upvate_core` `/api/v1/sync`".

Then "What can go wrong" (business exceptions: offline, rejected, missing data)
and "Where it touches the business" (link to the capabilities and domain terms
it exercises).

### Step 3 — Optional business diagram

If a ≤8-box business `flowchart` clarifies the sequence, add it with
business-only labels. Otherwise omit. NEVER a `sequenceDiagram`.

### Step 4 — Anchors + provenance

Headings first, derive `anchors:` per `snippets/anchor-contract.md`. Symbols and
file paths ONLY in the collapsed `<details>` block.

### Step 5 — Verify + save

Run `snippets/verification-checklist.md`. Then:

```
archigraph_save_finding(
  question="What are the business user journeys for the <group> group?",
  answer="<files: <primary-repo>/docs/business/journeys/*.md>",
  type="business_journeys",
)
```

Hand back; report the list of journey slugs for Pass 19's index.
