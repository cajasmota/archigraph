<!-- DO NOT EDIT — generated from docs/coverage/registry.json by 'go run ./tools/coverage gen' -->
# `infra.resource.kubernetes` — Kubernetes manifests

Auto-generated. Back to [summary](../summary.md).

- **Language:** [multi](../by-language/multi.md)
- **Category:** [platform](../by-category/platform.md)
- **Subcategory:** App Topology & Integration
- **Capability cells:** 2

## Capabilities

| Capability | Status | Verified at | Issue | Cites | Notes |
|------------|--------|-------------|-------|-------|-------|
| Dependency attribution | 🟢 `partial` | `2026-06-04` | 4202 | `internal/engine/kubernetes_edges.go`<br>`internal/engine/kubernetes_edges_test.go` | #4202 (epic #4194): Kubernetes app_topology DEPENDS_ON-class attribution. applyKubernetesEdges emits resource→resource DEPENDS_ON edges (types.RelationshipKindDependsOn) over parsed manifests, namespace-scoped (#3551), via the shared emit closure. Attribution bases (all DEPENDS_ON): HPA spec.scaleTargetRef → workload (k8s_edge=hpa_target); NetworkPolicy spec.podSelector → matched same-ns workloads (networkpolicy_podselector); PodDisruptionBudget spec.selector → workload (pdb_selector); ServiceMonitor/PodMonitor spec.selector → Service (servicemonitor_selector); ownerReferences → parent resource (owner_reference). (Service selector→workload and Ingress→Service are ROUTES_TO, not counted here.) Every DEPENDS_ON edge carries attribution props k8s_edge=<basis> + synthesis=kubernetes_edges + language=yaml. Value-asserting tests pin exact From/To/Kind+props: TestKubernetesEdges_DependencyAttribution_Cell (HPA api-hpa→Deployment api, k8s_edge=hpa_target, synthesis=kubernetes_edges), plus TestKubernetesEdges_HPATarget / _OwnerReference / _NetworkPolicy / _PDB / _ServiceMonitor bases. PARTIAL: selector/ref-driven dependency shapes over single-file manifest sets; CRD-custom dependency semantics + cross-file Helm-rendered graphs deferred. DEPLOY-DEFERRED. |
| Resource extraction | ✅ `full` | `2026-05-28` | — | `internal/extractors/yaml/extractor.go` | — |

## Provenance

This record is sourced from `docs/coverage/registry.json`. To update it, edit the JSON
(or use `go run ./tools/coverage update infra.resource.kubernetes ...`) then regenerate:

```
go run ./tools/coverage validate
go run ./tools/coverage gen
```
