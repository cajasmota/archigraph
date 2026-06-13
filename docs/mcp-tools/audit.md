# MCP tools — Audit

[← Back to the MCP tools index](../mcp-tools.md)

Security, secrets, licenses, test coverage, dead code, and cross-group parity diffs (oracle vs v3).

---

## `archigraph_auth_coverage`

Flag HTTP endpoints missing auth.

Key parameters: `repo_filter[]`, `only_missing` (bool), `format` (`terse`/`full`), `verbose` (bool), `limit` (default 50), `token_budget`.

Output: per-endpoint auth status with severity (error = sensitive/IDOR, warn = unauthenticated public, info = covered).

---

## `archigraph_secrets`

Scan for hardcoded secrets; masked findings by severity.

Key parameters: `severity`, `limit` (default 200).

Output: masked credential findings by severity. Test fixtures and opt-out comments are suppressed.

---

## `archigraph_license_audit`

Audit dependency licenses; flag GPL/AGPL conflicts.

Key parameters: `group`/`cwd`.

Output: dependency records flagged by license kind (GPL/AGPL conflict detection).

---

## `archigraph_test_coverage`

Production entities with no TESTS edge, ranked by severity.

Key parameters: `entity_id` (optional — scoped query), `repo_filter[]`, `severity`, `limit` (default 100), `top_directories` (bool).

Output: production entities lacking TESTS edges, ranked by severity, followed by a **coverage-freshness** block (#5068).

**Coverage freshness (#5068).** Both this tool and `archigraph_test_reachability` append a freshness section that tells an agent whether any *ingested* line-coverage report (LCOV/Cobertura/JaCoCo, #5036) is **stale relative to the latest index**. The verdict mirrors the dashboard provenance banner (#5038): a measurement is **STALE** when its `coverage_measured_at` predates the latest index `generated_at` (the report annotates a graph that has since been re-indexed, so "% covered" may no longer reflect current code) and **FRESH** when at/after it; the delta between the two is surfaced. It degrades honestly: *no* entity carrying `coverage_source` ⇒ "no coverage report ingested" (the counts above are static reach-coverage, not a measured line %); a report with no `coverage_measured_at` stamp, or no index timestamp to compare against ⇒ **UNKNOWN** rather than a fabricated verdict. STALE output tells the agent to re-run tests + reingest the report, then re-index.

---

## `archigraph_test_reachability`

Static test-reachability over the **TESTS + CALLS** call graph: which functions and endpoints are reached by *any* test path, and — the key signal — which are **orphans** with no test path at all.

Unlike `archigraph_test_coverage` (which only checks for a *direct* inbound `TESTS` edge), this tool reflects *transitive* reachability: a function is reachable if a test reaches it through one or more `CALLS` hops. It surfaces the reaching tests and the minimum hop depth per entity.

The signal is computed by the indexer (#5037 / #5061) and **stamped onto entity properties** (`test_reachable`, `reaching_tests`, `reaching_test_count`, `reach_depth`). This tool **reads those properties off the loaded graph — it does not recompute**. If a group was indexed before the reachability pass landed (no entity carries `test_reachable`), the tool says *"reachability not computed — reindex"* rather than returning a misleading empty/all-zero result.

Key parameters: `entity_id` (optional — focus a single entity), `repo_filter[]`, `untested_only` (bool — list only orphans), `endpoints_only` (bool — HTTP endpoints/routes only), `limit` (default 100).

Output: group and per-module reachability roll-ups (% reachable), an endpoint roll-up, and a row listing sorted **orphans-first** (endpoints before plain functions). Each reachable row shows `depth` and reaching-test count; a `[reachable-but-0%-lines]` tag flags entities statically reached by a test yet with 0% measured LCOV line coverage (the candidate-ineffective-test signal, crossing #5036). The canonical orphan query is `untested_only=true` (optionally with `endpoints_only=true`): "which endpoints/handlers have NO test reaching them". The output ends with the same **coverage-freshness** block described under [`archigraph_test_coverage`](#archigraph_test_coverage) (#5068) — relevant here because the `[reachable-but-0%-lines]` cross-signal is only meaningful against a *current* coverage report.

---

## `archigraph_coverage_effectiveness`

The reachability × line-coverage **cross-product** report (#5063): it crosses the static test-reachability signal (#5037) with the ingested LCOV line coverage (#5036) — both **stamped onto entity properties at index time** by #5061 — and classifies every production function/endpoint into a small set of meaningful quadrants:

- **reachable + 0% lines** → candidate **ineffective / tautological test** (the headline signal): a static test path reaches the entity, yet not one of its production lines actually ran. Cross-check with [`archigraph_contract_test_effectiveness`](#archigraph_contract_test_effectiveness) (#4893).
- **reachable + low coverage** (< 50%) → weak coverage.
- **reachable + covered** (≥ 50%) → healthy: tested and run.
- **reachable + no line-coverage measurement** → reachable, but the entity is absent from the LCOV report, so the line-coverage cross is unavailable for it.
- **unreachable** → untested surface (the #5037 orphans).

It reports per-module and group quadrant roll-ups and the headline **ineffective-test list**. Like the sibling reachability tool, it **reads the stamped properties off the loaded graph — it does not recompute** (it runs the pure `coverage.ComputeEffectivenessReport` over them).

**Honest degradation:** when a group/module carries reachability but **no ingested line coverage** (`coverage_pct` absent on every entity), the tool reports the reachability quadrants and states that the line-coverage cross — and therefore the reachable-but-0%-lines signal — is unavailable, rather than fabricating a verdict. When nothing is stamped at all, it says *"reachability not computed — reindex"*.

Key parameters: `repo_filter[]`, `ineffective_only` (bool — show only the reachable-but-0%-lines list + roll-ups), `limit` (default 100).

Output: group + per-module quadrant roll-ups (worst-first by ineffective+untested ratio), the ineffective-test list, and a quadrant-sorted entity listing. Dashboard surfacing of this report is owned by #5062 / #5067.

---

## `archigraph_dead_code`

Reachability dead-code: entities unreached by entry-points.

Key parameters: `repo_filter[]`, `kind_filter`, `from`, `limit` (default 200).

Output: entities not reachable from the entry-point set.

---

## `archigraph_find_dead_code`

Dead/unwired code: isolated, marked-unused, or test-only-referenced symbols.

Key parameters: `repo_filter[]`, `kind_filter`, `limit` (default 100), `min_confidence`.

Output: entities with zero project edges; may be genuine dead code or an extractor gap.

---

## `archigraph_security_findings`

Taint-flow security findings: source → sink paths ranked by confidence.

Key parameters: `category`, `min_confidence`, `limit`, `source_repo`.

Output: ranked taint-flow findings with source → sink paths.

---

## `archigraph_contract_test_effectiveness`

Tautological-spec detector: assertions that can never fail (oracle-blind).

Key parameters: `group`/`cwd`.

Output: contract tests whose assertions cannot fail.

---

## `archigraph_literal_parity`

Cross-group ConstantSet/enum value-set parity diff (oracle vs v3).

Key parameters: `group_oracle` (required), `group_v3` (required), `set` (required).

Output: per-set value parity diff between the two groups.

---

## `archigraph_auth_posture_diff`

Cross-group auth-posture parity diff per linked endpoint (oracle vs v3).

Key parameters: `group_oracle` (required), `group_v3` (required).

Output: per-endpoint auth-posture parity diff.

---

## `archigraph_stub_detector`

Cross-group stub detector: v3 pure where oracle computes (effects).

Key parameters: `group_v3` (required), `group_oracle` (required).

Output: endpoints/functions that are pure in v3 but compute in the oracle.

---

## `archigraph_response_shape_diff`

Cross-group branch-aware response-shape parity diff per endpoint (oracle vs v3).

Key parameters: `group_oracle` (required), `group_v3` (required).

Output: per-endpoint response-shape parity diff.
