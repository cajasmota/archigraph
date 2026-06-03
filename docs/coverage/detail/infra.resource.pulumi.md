<!-- DO NOT EDIT — generated from docs/coverage/registry.json by 'go run ./tools/coverage gen' -->
# `infra.resource.pulumi` — Pulumi

Auto-generated. Back to [summary](../summary.md).

- **Language:** [multi](../by-language/multi.md)
- **Category:** [platform](../by-category/platform.md)
- **Subcategory:** App Topology & Integration
- **Capability cells:** 2

## Capabilities

| Capability | Status | Verified at | Issue | Cites | Notes |
|------------|--------|-------------|-------|-------|-------|
| Dependency attribution | 🟢 `partial` | `2026-06-04` | 4202 | `internal/engine/pulumi_edges.go`<br>`internal/engine/pulumi_edges_go_net.go`<br>`internal/engine/pulumi_edges_test.go` | #4202 (epic #4194): Pulumi app_topology DEPENDS_ON-class attribution. applyPulumiEdges emits SCOPE.InfraResource→SCOPE.InfraResource DEPENDS_ON edges (kind const pulumiDependsOnEdgeKind="DEPENDS_ON") via the shared emitDependsOn closure, mirroring the hcl/Terraform depends_on→DEPENDS_ON contract. Three attribution bases: output-ref — one resource's output (`data.arn`) feeding another's args → consumer DEPENDS_ON producer (reason=output_ref); explicit `dependsOn`/`depends_on` list → resource DEPENDS_ON each listed dep (reason=depends_on); StackReference cross-stack → per-file ref node DEPENDS_ON canonical pulumi-stack:<ref> node (emitCrossStack). Every edge carries attribution props iac_tool=pulumi + pattern_type=pulumi_program + reason. Value-asserting tests pin exact From/To/Kind+props: TestPulumiTS_DependencyAttribution_Cell (depends_on: worker→queue, iac_tool=pulumi, reason=depends_on), plus TestPulumiTS_MarqueeFixture / _DependsOnAndStackRef / TestPulumiPy_MarqueeFixture. PARTIAL: TS/JS+Python+Go/.NET lanes (Pulumi-Java not implemented; Go/C# StackReference+ComponentResource deferred) — same scope bound as resource_extraction. DEPLOY-DEFERRED. |
| Resource extraction | 🟢 `partial` | `2026-05-31` | [link](https://github.com/cajasmota/archigraph/issues/3528) | `internal/engine/pulumi_edges.go`<br>`internal/engine/pulumi_edges_go_net.go`<br>`internal/engine/rules/javascript_typescript/frameworks/pulumi.yaml`<br>`internal/engine/rules/python/frameworks/pulumi.yaml` | Pulumi-TS + Pulumi-Python (applyPulumiEdges) and Pulumi-Go + Pulumi-C# (applyPulumiEdgesGoNet, #3550): SCOPE.InfraResource per resource constructor named by its logical-name string literal (construct_type + uniform resource_category from the shared types.IaCResourceCategory classifier, #3549; resource_scope kept as an alias). Go covers v, err := pkg.NewType(ctx, "name", ...) (New-strip to pkg.Type) + output-ref/DependsOn edges; C# covers new Provider.Svc.Type("name", ...) + output-ref/DependsOn edges. Pulumi-Java not yet implemented; Go/C# StackReference + ComponentResource deferred. TS/JS+Python depth deferred to follow-up. Previously over-stamped full citing the hcl/Terraform extractor, which cannot parse .ts/.py. |

## Provenance

This record is sourced from `docs/coverage/registry.json`. To update it, edit the JSON
(or use `go run ./tools/coverage update infra.resource.pulumi ...`) then regenerate:

```
go run ./tools/coverage validate
go run ./tools/coverage gen
```
