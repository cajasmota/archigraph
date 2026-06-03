<!-- DO NOT EDIT — generated from docs/coverage/registry.json by 'go run ./tools/coverage gen' -->
# `infra.resource.aws-cdk` — AWS CDK

Auto-generated. Back to [summary](../summary.md).

- **Language:** [multi](../by-language/multi.md)
- **Category:** [platform](../by-category/platform.md)
- **Subcategory:** App Topology & Integration
- **Capability cells:** 2

## Capabilities

| Capability | Status | Verified at | Issue | Cites | Notes |
|------------|--------|-------------|-------|-------|-------|
| Dependency attribution | 🟢 `partial` | `2026-06-04` | 4202 | `internal/engine/cdk_edges.go`<br>`internal/engine/cdk_edges_jvm_go_net.go`<br>`internal/engine/cdk_edges_test.go` | #4202 (epic #4194): CDK app_topology DEPENDS_ON-class attribution. applyCDKEdges emits SCOPE.InfraResource→SCOPE.InfraResource DEPENDS_ON edges (kind const cdkDependsOnEdgeKind="DEPENDS_ON") via the shared emitDependsOn closure, mirroring the hcl/Terraform depends_on→DEPENDS_ON contract. Three attribution bases: Pass 3 grant — `res.grant*(grantee)` → grantee DEPENDS_ON res (reason=grant, grant=<method>); Pass 4 event-source — `fn.addEventSource(new Src(res))` → fn DEPENDS_ON res (reason=event_source); Pass 5 props-ref — a construct var passed into another construct's props → enclosing DEPENDS_ON passed (reason=props_ref). Every edge carries attribution props iac_tool=aws-cdk + pattern_type=cdk_synthesis + reason, with the per-reason detail under props[reason]. Value-asserting tests pin exact From/To/Kind+props: TestCDK_DependencyAttribution_Cell (grant: Reader→DataBucket, iac_tool=aws-cdk, reason=grant, grant=grantReadWrite), plus TestCDK_MarqueeFixture_BucketLambdaGrant / TestCDK_AddEventSource / TestCDK_PropsRef. PARTIAL: single-file regex over JS/TS+Python+JVM/Go/.NET idioms; cross-file/L1-escape-hatch dependency wiring deferred (same scope bound as resource_extraction). DEPLOY-DEFERRED: live-daemon reindex is a separate coordinated step. |
| Resource extraction | 🟢 `partial` | `2026-05-31` | [link](https://github.com/cajasmota/archigraph/issues/3512) | `internal/engine/cdk_edges.go`<br>`internal/engine/cdk_edges_jvm_go_net.go`<br>`internal/engine/rules/javascript_typescript/frameworks/aws_cdk.yaml`<br>`internal/engine/rules/python/frameworks/aws_cdk.yaml` | CDK-TS + CDK-Python (applyCDKEdges / applyCDKEdgesPython) and CDK-Java + CDK-Go + CDK-C# (applyCDKEdgesJVMGoNet, #3550): SCOPE.InfraResource per construct named by its 'LogicalId' literal (construct_type + uniform resource_category from the shared types.IaCResourceCategory classifier, #3549; resource_scope kept as an alias). Java covers new-form + Type.Builder.create(...) fluent form; Go covers pkg.NewType(scope, jsii.String("Id"), ...) (New-strip to pkg.Type); C# covers new Type(this, "Id", ...). Java/C# bare construct types (Bucket/Table/Function) aliased to provider-qualified forms (s3.Bucket/dynamodb.Table/lambda.Function) so the classifier fires. grant*/Grant* calls -> grantee DEPENDS_ON resource. All five JSII bindings now extract; remains partial (single-file regex; event-source/L1-escape-hatch/cross-file deferred). TS/JS+Python depth deferred to follow-up. Previously over-stamped full citing the hcl/Terraform extractor, which cannot parse .ts/.py. |

## Provenance

This record is sourced from `docs/coverage/registry.json`. To update it, edit the JSON
(or use `go run ./tools/coverage update infra.resource.aws-cdk ...`) then regenerate:

```
go run ./tools/coverage validate
go run ./tools/coverage gen
```
