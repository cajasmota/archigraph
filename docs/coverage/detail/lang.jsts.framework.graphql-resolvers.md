<!-- DO NOT EDIT — generated from docs/coverage/registry.json by 'go run ./tools/coverage gen' -->
# `lang.jsts.framework.graphql-resolvers` — GraphQL Resolvers (Apollo Server / GraphQL Yoga / etc.)

Auto-generated. Back to [summary](../summary.md).

- **Language:** [JS/TS](../by-language/jsts.md)
- **Category:** [http_framework](../by-category/http_framework.md)
- **Subcategory:** RPC Framework
- **Capability cells:** 55

## Capabilities


### Schema

| Capability | Status | Verified at | Issue | Cites | Notes |
|------------|--------|-------------|-------|-------|-------|
| Federation extraction | 🟢 `partial` | `2026-06-02` | 3623 | `internal/extractors/graphql/federation_test.go`<br>`internal/extractors/graphql/graphql.go`<br>`internal/types/kinds.go` | Apollo Federation SDL: type Foo @key(fields:"id") -> entity Properties {federated:true, federation:apollo, key_fields:id} (+shareable:true on @shareable); extend type Foo @key(...) { f @external/@requires/@provides } -> FEDERATES edge to owning entity Foo carrying key_fields + external_fields/requires_fields/provides_fields buckets (legacy IMPORTS edge preserved). Value-asserting tests assert exact key_fields and FEDERATES ToID=owning type. PARTIAL: regex SDL only — no @link/@composeDirective import resolution, no interfaceObject, no cross-file/cross-repo subgraph entity merge (gateway-level concern for the downstream linker). |
| Procedure extraction | ✅ `full` | `2026-05-28` | 2932 | `internal/engine/http_endpoint_synthesis.go`<br>`internal/engine/rules/graphql/frameworks/apollo_server.yaml`<br>`internal/engine/rules/graphql/frameworks/graphql_yoga.yaml`<br>`internal/extractors/graphql/graphql.go` | — |
| Schema extraction | ✅ `full` | `2026-05-28` | 2932 | `internal/engine/rules/graphql/frameworks/graphql_schema.yaml`<br>`internal/extractors/graphql/graphql.go` | — |
| Type graph extraction | ✅ `full` | `2026-06-02` | 3804 | `internal/extractors/graphql/graphql.go`<br>`internal/extractors/graphql/type_graph.go`<br>`internal/extractors/graphql/type_graph_test.go`<br>`internal/types/kinds.go` | SDL schema type→type graph: an object-typed field (type User { orders: [Order!]! }) emits a GRAPH_RELATES edge between the EXISTING SCOPE.Schema type nodes (User node -> Order node, addressed via BuildOperationStructuralRef — node reuse, no duplicate), carrying cardinality props {list, nullable, item_nullable, cardinality: to_one|to_many, field_name, self_ref}. Object + interface targets only; scalar/enum/input/custom-scalar and unresolved type names make NO edge. Union-typed fields expand to one edge per concrete member declared in-file (via_union prop). Value-asserting tests assert exact FromID+ToID+cardinality. Reuses the ORM GRAPH_RELATES vocabulary (#3611/#3747). Code-first lanes (TypeGraphQL/Nexus/Strawberry/graphene/Pothos/gqlgen) tracked separately for type-graph backfill. |

### Codegen

| Capability | Status | Verified at | Issue | Cites | Notes |
|------------|--------|-------------|-------|-------|-------|
| Client codegen | — `not_applicable` | — | 2865 | — | Server-side resolver record: client codegen (graphql-codegen/Apollo) generates a typed CLIENT elsewhere, not in resolver source. |

### Transport

| Capability | Status | Verified at | Issue | Cites | Notes |
|------------|--------|-------------|-------|-------|-------|
| Transport binding | ✅ `full` | `2026-05-28` | 2906 | `internal/engine/http_endpoint_synthesis.go`<br>`internal/engine/http_endpoint_transport_binding.go`<br>`internal/engine/http_endpoint_transport_binding_test.go`<br>`testdata/fixtures/typescript/graphql_transport_http.ts`<br>`testdata/fixtures/typescript/graphql_transport_http_ws.ts` | — |

### Routing

| Capability | Status | Verified at | Issue | Cites | Notes |
|------------|--------|-------------|-------|-------|-------|
| Endpoint deprecation versioning | 🔴 `missing` | — | 3963 | — | — |
| Endpoint pagination posture | 🔴 `missing` | — | 3963 | — | — |
| Endpoint response codes | 🔴 `missing` | — | 3963 | — | — |
| Endpoint synthesis | 🔴 `missing` | — | 3963 | — | — |
| Handler attribution | 🔴 `missing` | — | 3963 | — | — |
| Route extraction | 🔴 `missing` | — | 3963 | — | — |

### View

| Capability | Status | Verified at | Issue | Cites | Notes |
|------------|--------|-------------|-------|-------|-------|
| View rendering | 🔴 `missing` | — | 3963 | — | — |

### Auth

| Capability | Status | Verified at | Issue | Cites | Notes |
|------------|--------|-------------|-------|-------|-------|
| Auth coverage | 🔴 `missing` | — | 3963 | — | — |

### Validation

| Capability | Status | Verified at | Issue | Cites | Notes |
|------------|--------|-------------|-------|-------|-------|
| DTO extraction | 🔴 `missing` | — | 3963 | — | — |
| Request validation | 🔴 `missing` | — | 3963 | — | — |

### Middleware

| Capability | Status | Verified at | Issue | Cites | Notes |
|------------|--------|-------------|-------|-------|-------|
| Middleware coverage | 🔴 `missing` | — | 3963 | — | — |
| Rate limit stamping | 🔴 `missing` | — | 3963 | — | — |

### Type System

| Capability | Status | Verified at | Issue | Cites | Notes |
|------------|--------|-------------|-------|-------|-------|
| Enum extraction | 🔴 `missing` | — | 3963 | — | — |
| Interface extraction | 🔴 `missing` | — | 3963 | — | — |
| Type alias extraction | ✅ `full` | `2026-06-03` | 3963 | `internal/extractors/javascript/extractor.go`<br>`internal/patterns/type_alias_extractor.go` | #3963 wave1-structural: TS type_alias extractor (language typescript/javascript, no framework gate) emits alias_name/alias_of for GraphQL resolver context/parent/args type aliases. Covered by the same jsts type-alias idiom proven in TestW1jr_TypeAlias_TypeGraphqlContextAlias (resolver context alias). |
| Type extraction | 🔴 `missing` | — | 3963 | — | — |

### DI

| Capability | Status | Verified at | Issue | Cites | Notes |
|------------|--------|-------------|-------|-------|-------|
| DI binding extraction | 🔴 `missing` | — | 3963 | — | — |
| DI injection point | 🔴 `missing` | — | 3963 | — | — |
| DI scope resolution | 🔴 `missing` | — | 3963 | — | — |

### Testing

| Capability | Status | Verified at | Issue | Cites | Notes |
|------------|--------|-------------|-------|-------|-------|
| Tests linkage | 🔴 `missing` | — | 3963 | — | — |

### Observability

| Capability | Status | Verified at | Issue | Cites | Notes |
|------------|--------|-------------|-------|-------|-------|
| Log extraction | 🔴 `missing` | — | 3963 | — | — |
| Metric extraction | 🔴 `missing` | — | 3963 | — | — |
| Trace extraction | 🔴 `missing` | — | 3963 | — | — |

### Substrate

| Capability | Status | Verified at | Issue | Cites | Notes |
|------------|--------|-------------|-------|-------|-------|
| Confidence overlay | 🟢 `partial` | `2026-05-29` | 3076 | `internal/links/effect_propagation.go`<br>`internal/substrate/jsts.go`<br>`testdata/fixtures/typescript/substrate_graphql/resolver.ts` | — |
| Config consumption | ✅ `full` | `2026-06-02` | 3641 | `internal/extractor/config_key.go`<br>`internal/extractors/javascript/config_consumer.go`<br>`internal/extractors/javascript/config_consumer_test.go` | process.env.X, import.meta.env.X, config.get(k) -> config:<key> DEPENDS_ON_CONFIG (issue #3641) |
| Constant propagation | ✅ `full` | `2026-05-29` | 3076 | `internal/substrate/jsts.go`<br>`internal/substrate/uimm_substrate_test.go`<br>`testdata/fixtures/typescript/substrate_graphql/resolver.ts` | — |
| DB effect | 🟢 `partial` | `2026-05-29` | 3076 | `internal/links/effect_propagation.go`<br>`internal/substrate/effect_sinks_jsts.go`<br>`testdata/fixtures/typescript/substrate_graphql/resolver.ts` | — |
| Dead code detection | 🟢 `partial` | `2026-05-29` | 3076 | `internal/links/reachability.go`<br>`internal/patterns/dead_module_detector.go` | — |
| Def use chain extraction | 🟢 `partial` | `2026-05-29` | 3076 | `internal/substrate/def_use_jsts.go`<br>`testdata/fixtures/typescript/substrate_graphql/resolver.ts` | — |
| Env fallback recognition | ✅ `full` | `2026-05-29` | 3076 | `internal/substrate/jsts.go`<br>`internal/substrate/uimm_substrate_test.go`<br>`testdata/fixtures/typescript/substrate_graphql/resolver.ts` | — |
| Error flow | ✅ `full` | `2026-06-02` | 3628 | `internal/extractor/exception_flow.go`<br>`internal/extractors/javascript/exception_flow.go`<br>`internal/extractors/javascript/exception_flow_test.go` | throw new X -> THROWS; e instanceof X catch-filter -> CATCHES; untyped throw/catch dropped (#3628) |
| Feature flag gating | 🟢 `partial` | `2026-06-03` | 3706 | `internal/engine/feature_flag_edges.go`<br>`internal/engine/feature_flag_edges_test.go`<br>`internal/engine/orm_queries.go` | flag-check call sites -> feature:<key> + GATED_BY (framework-agnostic JS/TS engine pass, fires regardless of framework). Verified to attribute to the enclosing function: LaunchDarkly ldClient.variation/boolVariation/stringVariation, Unleash unleash.isEnabled, OpenFeature client.getBooleanValue, Unleash-React useFlag, Split.io getTreatment, Flagsmith hasFeature, plus GrowthBook gb.isOn/isOff/getFeatureValue and ConfigCat configCatClient.getValue/getValueAsync (receiver-gated). Honest-partial: dynamic keys + non-flag receivers (button.isOn, formData.getValue) emit nothing. |
| Fs effect | 🟢 `partial` | `2026-05-29` | 3076 | `internal/links/effect_propagation.go`<br>`internal/substrate/effect_sinks_jsts.go`<br>`testdata/fixtures/typescript/substrate_graphql/resolver.ts` | — |
| HTTP effect | 🟢 `partial` | `2026-05-29` | 3076 | `internal/links/effect_propagation.go`<br>`internal/substrate/effect_sinks_jsts.go`<br>`testdata/fixtures/typescript/substrate_graphql/resolver.ts` | — |
| Import resolution quality | ✅ `full` | `2026-05-29` | 3076 | `internal/substrate/jsts.go`<br>`internal/substrate/uimm_substrate_test.go`<br>`testdata/fixtures/typescript/substrate_graphql/resolver.ts` | — |
| Module cycle detection | 🟢 `partial` | `2026-05-29` | 3076 | `internal/links/module_cycle_pass.go`<br>`testdata/fixtures/typescript/substrate_graphql/resolver.ts` | — |
| Mutation effect | 🟢 `partial` | `2026-05-29` | 3076 | `internal/links/effect_propagation.go`<br>`internal/substrate/effect_sinks_jsts.go`<br>`testdata/fixtures/typescript/substrate_graphql/resolver.ts` | — |
| Pure function tagging | 🟢 `partial` | `2026-05-29` | 3076 | `internal/links/pure_function_pass.go`<br>`testdata/fixtures/typescript/substrate_graphql/resolver.ts` | — |
| Reachability analysis | 🟢 `partial` | `2026-05-29` | 3076 | `internal/links/reachability.go`<br>`internal/substrate/entry_points_jsts.go`<br>`testdata/fixtures/typescript/substrate_graphql/resolver.ts` | — |
| Request shape extraction | 🟢 `partial` | `2026-05-29` | 3076 | `internal/substrate/payload_shapes_graphql.go`<br>`internal/substrate/payload_shapes_jsts.go`<br>`testdata/fixtures/graphql/schema.graphql` | — |
| Request sink dataflow | 🔴 `missing` | — | 3963 | — | — |
| Response shape extraction | 🟢 `partial` | `2026-05-29` | 3076 | `internal/substrate/payload_shapes_graphql.go`<br>`internal/substrate/payload_shapes_jsts.go`<br>`testdata/fixtures/graphql/schema.graphql` | — |
| Sanitizer recognition | 🟢 `partial` | `2026-05-29` | 3076 | `internal/links/taint_flow.go`<br>`internal/substrate/taint_sites_jsts.go`<br>`internal/substrate/uimm_substrate_test.go`<br>`testdata/fixtures/typescript/substrate_graphql/resolver.ts` | — |
| Schema drift detection | ✅ `full` | `2026-05-29` | 3076 | `internal/links/payload_drift.go`<br>`internal/substrate/payload_shapes_graphql.go`<br>`internal/substrate/payload_shapes_graphql_test.go`<br>`internal/substrate/payload_shapes_jsts.go`<br>`testdata/fixtures/graphql/schema.graphql` | GraphQL SDL sniffing added (#3076 B-part): input types map to request shapes, object types to response shapes, and inline operation args to per-operation request shapes. payload_drift.go picks these up via the generic PayloadShapeSnifferFor dispatch after LanguageForPath returns graphql for .graphql/.gql files. |
| Taint sink detection | 🟢 `partial` | `2026-05-29` | 3076 | `internal/links/taint_flow.go`<br>`internal/substrate/taint_sites_jsts.go`<br>`internal/substrate/uimm_substrate_test.go`<br>`testdata/fixtures/typescript/substrate_graphql/resolver.ts` | — |
| Taint source detection | 🟢 `partial` | `2026-05-29` | 3076 | `internal/links/taint_flow.go`<br>`internal/substrate/taint_sites_jsts.go`<br>`internal/substrate/uimm_substrate_test.go`<br>`testdata/fixtures/typescript/substrate_graphql/resolver.ts` | — |
| Template pattern catalog | 🟢 `partial` | `2026-05-29` | 3076 | `internal/substrate/template_pattern_jsts.go`<br>`testdata/fixtures/typescript/substrate_graphql/resolver.ts` | — |
| Vulnerability finding | 🟢 `partial` | `2026-05-29` | 3076 | `internal/links/taint_flow.go`<br>`internal/substrate/taint_sites_jsts.go`<br>`internal/substrate/uimm_substrate_test.go`<br>`testdata/fixtures/typescript/substrate_graphql/resolver.ts` | — |

## Framework-specific

### DataLoader (N+1 batching)

| Capability | Status | Verified at | Issue | Cites | Notes |
|------------|--------|-------------|-------|-------|-------|
| Dataloader extraction | 🟢 `partial` | `2026-06-02` | 3624 | `internal/extractors/javascript/graphql_dataloader.go`<br>`internal/extractors/javascript/issue3624_dataloader_test.go`<br>`internal/types/kinds.go` | new DataLoader(batchFn) (the 'dataloader' npm pkg) -> SCOPE.DataLoader entity named by the assigned const/field + BATCHES edge to the wrapped batch fn (bare ident or single-call delegating arrow); loader.load(id)/loadMany(ids) in a resolver body -> USES edge resolver->loader, via=graphql_dataloader. Value-asserted: userLoader BATCHES batchUsers + resolveAuthor USES userLoader. PARTIAL (honest): only statically-named loaders; dynamic/factory-built loaders and lambda batch fns get no BATCHES edge. |

## Related extraction records

This record provides code-level coverage for the
[`protocol.graphql`](./protocol.graphql.md) hub record (GraphQL),
which tracks the same technology at a higher level.

## Provenance

This record is sourced from `docs/coverage/registry.json`. To update it, edit the JSON
(or use `go run ./tools/coverage update lang.jsts.framework.graphql-resolvers ...`) then regenerate:

```
go run ./tools/coverage validate
go run ./tools/coverage gen
```
