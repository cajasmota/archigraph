# Pass 16 — Product capabilities (BUSINESS tier)

Produce one page per product capability: what the system does and why, in
business language. Capabilities are derived from the system's externally-visible
behaviour — HTTP endpoints, process flows, message topics, scheduled jobs —
grouped by business meaning, NOT one-per-endpoint.

> **READ FIRST:** `snippets/business-voice.md`. Binding. No symbol names, no API
> paths, no code mermaid. PM audience.

Synthesised across the whole group.

## Inputs

- `<primary-repo>/docs/business/domain-glossary.md` (Pass 15) — link into it.
- Technical-tier `reference/api.md`, module `flows.md`, and `overview.md` for
  every repo (where generated) — these enumerate the behaviour; translate it.
- The graph via the MCP tools below.

## Output

```
<primary-repo>/docs/business/capabilities/<capability-slug>.md   # one per capability
```

Use `output-templates/business-capability.md`. `<capability-slug>` is a
kebab-case business name (e.g. `schedule-inspection`, `submit-report`,
`manage-deficiencies`) — not a module or endpoint name.

## Procedure

### Step 1 — Enumerate behaviour

```
archigraph_endpoints(repo_filter=null, limit=500)        # what the product exposes
archigraph_flows(repo_filter=null, limit=200)            # process flows / call chains
archigraph_find(question="scheduled jobs and background processing", repo_filter=null, depth=2, token_budget=1000)
archigraph_find(question="message topics and events the system emits", repo_filter=null, depth=2, token_budget=1000)
```

If `archigraph_endpoints` / `archigraph_flows` are unavailable, fall back to
reading the technical-tier `reference/api.md` and module `flows.md`.

### Step 2 — Cluster into capabilities

Group the raw behaviour by **business outcome**. Many endpoints + a flow + a
topic often constitute ONE capability:

> `POST /inspections`, `PATCH /inspections/{id}`, `POST /inspections/{id}/submit`,
> the `inspection.submitted` topic, and the submit flow →
> capability **"Conduct and submit an inspection."**

Aim for a SMALL number of capabilities (typically 5–15 for a product), each
genuinely distinct. Do not emit a capability page per endpoint — that is the
over-fragmentation the audit warned about, in business clothing.

### Step 3 — Write each capability page

Fill `output-templates/business-capability.md`: what it does, why it exists, who
uses it and when, what it produces, the rules that govern it (forward-link to
`rules/` — Pass 18 fills those), related journeys (forward-link to `journeys/` —
Pass 17). Plain language throughout.

### Step 4 — Anchors + provenance

Headings first, then derive `anchors:` per `snippets/anchor-contract.md`. Code
references only in the collapsed `<details>` block.

### Step 5 — Verify + save

Run `snippets/verification-checklist.md`. Then once, for the capability set:

```
archigraph_save_finding(
  question="What product capabilities does the <group> group provide?",
  answer="<files: <primary-repo>/docs/business/capabilities/*.md>",
  type="business_capabilities",
)
```

Hand back. Pass 19 (business overview) will index every capability page you
wrote, so report the list of capability slugs to the orchestrator.
