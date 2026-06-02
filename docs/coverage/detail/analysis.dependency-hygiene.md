<!-- DO NOT EDIT — generated from docs/coverage/registry.json by 'go run ./tools/coverage gen' -->
# `analysis.dependency-hygiene` — Dependency hygiene (used / unused / phantom)

Auto-generated. Back to [summary](../summary.md).

- **Language:** [multi](../by-language/multi.md)
- **Category:** [package_manager](../by-category/package_manager.md)
- **Capability cells:** 1

## Capabilities

| Capability | Status | Verified at | Issue | Cites | Notes |
|------------|--------|-------------|-------|-------|-------|
| Dependency usage status | ✅ `full` | `2026-06-02` | — | `cmd/archigraph/index.go`<br>`internal/engine/dependency_hygiene.go`<br>`internal/engine/dependency_hygiene_test.go`<br>`internal/extractors/cross/deplinker/extractor.go` | deplinker used/unused/phantom classification is now PERSISTED into the graph (issue #3640, epic #3625). The Pass 8.7 engine pass engine.ApplyDependencyHygiene reuses deplinker.Analyze over the assembled graph.Document and writes usage_status=used|unused onto each external_dependency entity (manifest extractor) and its DEPENDS_ON(kind=external_dependency) edge, so find/neighbors/agents read dependency hygiene directly. Phantom imports (imported, not declared) are reported as a stat but not synthesised as entities to avoid double-counting the manifest inventory; phantom listing remains the dashboard's job. The dashboard handler keeps calling AnalyzeGroup independently (no double-emit). Honest-partial: classification fidelity tracks deplinker's manifest+import inputs. |

## Provenance

This record is sourced from `docs/coverage/registry.json`. To update it, edit the JSON
(or use `go run ./tools/coverage update analysis.dependency-hygiene ...`) then regenerate:

```
go run ./tools/coverage validate
go run ./tools/coverage gen
```
