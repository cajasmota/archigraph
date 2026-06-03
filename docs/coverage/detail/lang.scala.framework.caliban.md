<!-- DO NOT EDIT — generated from docs/coverage/registry.json by 'go run ./tools/coverage gen' -->
# `lang.scala.framework.caliban` — Caliban

Auto-generated. Back to [summary](../summary.md).

- **Language:** [scala](../by-language/scala.md)
- **Category:** [http_framework](../by-category/http_framework.md)
- **Subcategory:** JVM Backend
- **Capability cells:** 55

## Capabilities


### Routing

| Capability | Status | Verified at | Issue | Cites | Notes |
|------------|--------|-------------|-------|-------|-------|
| Endpoint deprecation versioning | 🔴 `missing` | — | 3628 | — | — |
| Endpoint pagination posture | 🔴 `missing` | `2026-06-02` | 3628 | `internal/engine/http_endpoint_pagination.go`<br>`internal/engine/http_endpoint_pagination_patterns.go`<br>`internal/engine/http_endpoint_pagination_test.go`<br>`internal/engine/http_endpoint_synthesis.go` | #3628: applyEndpointPagination stamps paginated/pagination_style/pagination_params via the cross-language parameters/parameter_schema fallback (limit+offset/page/cursor shape). No framework-specific pagination-class/ORM signal yet for this framework. |
| Endpoint response codes | 🔴 `missing` | — | 3818 | — | — |
| Endpoint synthesis | ✅ `full` | `2026-05-30` | — | `internal/custom/scala/caliban.go`<br>`internal/custom/scala/caliban_test.go` | graphQL(RootResolver(...)) synthesises one GRAPHQL endpoint per resolver case-class field, path /graphql/<Root>/<field>, positional Query/Mutation/Subscription root. TestCalibanResolverFields. |
| Handler attribution | ✅ `full` | `2026-05-30` | — | `internal/custom/scala/caliban.go`<br>`internal/custom/scala/caliban_test.go` | handler_name = <Root>.<field> bound from resolver case-class field. TestCalibanResolverFields. |
| Route extraction | ✅ `full` | `2026-05-30` | — | `internal/custom/scala/caliban.go`<br>`internal/custom/scala/caliban_test.go` | GraphQL field path /graphql/<Root>/<field> recorded as route_path. TestCalibanResolverFields. |

### View

| Capability | Status | Verified at | Issue | Cites | Notes |
|------------|--------|-------------|-------|-------|-------|
| View rendering | 🔴 `missing` | — | view_rendering:#3628-not-yet-extracted | — | — |

### Auth

| Capability | Status | Verified at | Issue | Cites | Notes |
|------------|--------|-------------|-------|-------|-------|
| Auth coverage | 🟢 `partial` | `2026-06-03` | 3992 | `internal/custom/scala/caliban.go`<br>`internal/custom/scala/caliban_test.go` | #3992: a resolver-field @GQLDirective(Authenticated) (or Authorized/Secured/RequiresAuth) gates the synthesised GRAPHQL endpoint — stamps the flat shared auth contract auth_required=true / auth_method=directive / auth_confidence=high / auth_directive=<name> / auth_guard=<name>. A custom role directive @GQLDirective(HasRole("admin")) / RequireRole/HasPermission/HasScope parses quoted role tokens into auth_roles (sorted). Non-auth directives (@GQLDeprecated, @GQLDirective(deprecated)) and directive-free fields stay unauthenticated. TestCalibanAuthDirective. PARTIAL honest limit: only field-level @GQLDirective auth is recovered (positional + intra-file, same binding limit as endpoint synthesis); Caliban Wrapper-based auth middleware and ZIO auth-environment requirements are NOT statically chased, and custom role directives with non-literal (computed/enum) role args stamp auth_required+auth_directive but leave auth_roles absent. |

### Validation

| Capability | Status | Verified at | Issue | Cites | Notes |
|------------|--------|-------------|-------|-------|-------|
| DTO extraction | ✅ `full` | `2026-05-30` | — | `internal/custom/scala/caliban.go`<br>`internal/custom/scala/caliban_test.go` | @GQLDescription/@GQLName/@GQLInputName case classes + Schema.gen[T]/deriveSchema[T] become SCOPE.Schema DTOs. TestCalibanDTOs. |
| Request validation | 🔴 `missing` | — | backfill:dictionary-completeness | — | — |

### Middleware

| Capability | Status | Verified at | Issue | Cites | Notes |
|------------|--------|-------------|-------|-------|-------|
| Middleware coverage | 🔴 `missing` | — | backfill:dictionary-completeness | — | — |
| Rate limit stamping | 🔴 `missing` | — | backfill:dictionary-completeness | — | — |

### Testing

| Capability | Status | Verified at | Issue | Cites | Notes |
|------------|--------|-------------|-------|-------|-------|
| Tests linkage | ✅ `full` | `2026-06-03` | — | `internal/extractors/cross/testmap/frameworks.go` | Deep testmap Scala TESTS linkage (testmap/frameworks.go): scalatest/specs2/MUnit/ZIO leaf cases with subject-from-spec-name + body-call resolution; assertion/matcher stopwords. Framework-agnostic (operates on test source). Value-asserting test pins a specific test->target edge for this framework. Fixture: TestScalaTrailing_Caliban_TestsLinkage. |

### Type System

| Capability | Status | Verified at | Issue | Cites | Notes |
|------------|--------|-------------|-------|-------|-------|
| Enum extraction | ✅ `full` | `2026-05-30` | — | `internal/custom/scala/caliban.go`<br>`internal/custom/scala/caliban_test.go` | @GQL-annotated enum extracted with graphql_dto_role=enum. TestCalibanDTOs. |
| Interface extraction | ✅ `full` | `2026-06-03` | — | `internal/custom/scala/type_system.go` | custom_scala_type_system extractor (type_system.go): trait -> SCOPE.Interface/trait, abstract class -> abstract_class. Framework-agnostic. Fixture: TestTrailing_Caliban_TypeSystem. |
| Type alias extraction | ✅ `full` | `2026-06-03` | — | `internal/custom/scala/type_system.go` | custom_scala_type_system extractor (type_system.go): type Alias = T and opaque type -> SCOPE.Type/type_alias. Framework-agnostic. Fixture: TestTrailing_Caliban_TypeSystem. |
| Type extraction | ✅ `full` | `2026-05-30` | — | `internal/custom/scala/caliban.go`<br>`internal/custom/scala/caliban_test.go` | Caliban schema types extracted as DTO entities. TestCalibanDTOs. |

### DI

| Capability | Status | Verified at | Issue | Cites | Notes |
|------------|--------|-------------|-------|-------|-------|
| DI binding extraction | 🔴 `missing` | — | backfill:dictionary-completeness | — | — |
| DI injection point | 🔴 `missing` | — | backfill:dictionary-completeness | — | — |
| DI scope resolution | 🔴 `missing` | — | backfill:dictionary-completeness | — | — |

### Transactions

| Capability | Status | Verified at | Issue | Cites | Notes |
|------------|--------|-------------|-------|-------|-------|
| Transaction boundary extraction | 🔴 `missing` | — | backfill:dictionary-completeness | — | — |
| Transaction function stamping | 🔴 `missing` | — | 3628-transaction-function-stamping | — | — |
| Transaction propagation | 🔴 `missing` | — | backfill:dictionary-completeness | — | — |
| Transaction rollback rules | 🔴 `missing` | — | backfill:dictionary-completeness | — | — |

### AOP

| Capability | Status | Verified at | Issue | Cites | Notes |
|------------|--------|-------------|-------|-------|-------|
| Advice attribution | 🔴 `missing` | — | backfill:dictionary-completeness | — | — |
| Aspect extraction | 🔴 `missing` | — | backfill:dictionary-completeness | — | — |
| Pointcut resolution | 🔴 `missing` | — | backfill:dictionary-completeness | — | — |

### Observability

| Capability | Status | Verified at | Issue | Cites | Notes |
|------------|--------|-------------|-------|-------|-------|
| Log extraction | 🟢 `partial` | `2026-06-03` | — | `internal/custom/scala/frameworks.go` | custom_scala_frameworks shared Observability block (frameworks.go, outside the framework switch) detects log statements (SLF4J/scala-logging/cats-effect/ZIO). HONEST PARTIAL: logger identity + message<->logger binding need cross-file dataflow; same limit as the other Scala frameworks. Fixture: TestTrailing_Caliban_Observability. |
| Metric extraction | ✅ `full` | `2026-06-03` | — | `internal/custom/scala/frameworks.go` | custom_scala_frameworks shared Observability block (frameworks.go): reScalaMetricNamed captures the LITERAL metric name per call site (Kamon/Micrometer/Dropwizard). Fires on any .scala file regardless of framework. Fixture: TestTrailing_Caliban_Observability. |
| Trace extraction | ✅ `full` | `2026-06-03` | — | `internal/custom/scala/frameworks.go` | custom_scala_frameworks shared Observability block (frameworks.go): reScalaTraceNamed captures the LITERAL span name per call site (Kamon/OTel/natchez). Fires on any .scala file regardless of framework. Fixture: TestTrailing_Caliban_Observability. |

### Data

| Capability | Status | Verified at | Issue | Cites | Notes |
|------------|--------|-------------|-------|-------|-------|
| DB effect | 🟢 `partial` | `2026-06-03` | — | `internal/links/effect_propagation.go`<br>`internal/substrate/effect_sinks_scala.go` | Scala effect sniffer (effect_sinks_scala.go, RegisterEffectSniffer('scala')) recognises Slick/Doobie/Quill/JPA read+write primitives; framework-agnostic, fires on any .scala file. Fixture: TestScalaTrailing_Caliban_Effects. |

### Substrate

| Capability | Status | Verified at | Issue | Cites | Notes |
|------------|--------|-------------|-------|-------|-------|
| Confidence overlay | 🔴 `missing` | — | backfill:dictionary-completeness | — | — |
| Config consumption | 🔴 `missing` | — | 3641 | — | — |
| Constant propagation | 🔴 `missing` | — | backfill:dictionary-completeness | — | — |
| Dead code detection | 🟢 `partial` | `2026-06-03` | — | `internal/links/reachability.go`<br>`internal/mcp/dead_code.go`<br>`internal/substrate/entry_points.go`<br>`internal/substrate/entry_points_scala.go` | Language-agnostic reachability/dead-code pass over Scala entry points (entry_points_scala.go) + IMPORTS/CALLS edges; framework-agnostic, fires on any .scala file. |
| Def use chain extraction | 🟢 `partial` | `2026-06-03` | — | `internal/links/def_use_pass.go`<br>`internal/substrate/def_use_scala.go` | Scala def-use sniffer (RegisterDefUseSniffer('scala'), def_use_scala.go) fires on any .scala file via LanguageForPath; def_use_pass.go invokes it for all scala entities. File-local val/var/for-generator def->use pairs. Fixture: TestScalaTrailing_Caliban_DefUse. |
| Env fallback recognition | 🔴 `missing` | — | backfill:dictionary-completeness | — | — |
| Error flow | ✅ `full` | `2026-06-03` | 3628 | `internal/extractor/exception_flow.go`<br>`internal/extractors/scala/exception_flow.go`<br>`internal/extractors/scala/exception_flow_test.go` | throw new X -> THROWS; pattern-match catch { case e: X } + .recover/.recoverWith { case e: X } -> CATCHES; qualified normalized to bare; catch-all/NonFatal/re-throw dropped (#3628) |
| Feature flag gating | 🔴 `missing` | — | feature_flag_gating:#3706-not-yet-extracted | — | — |
| Fs effect | 🟢 `partial` | `2026-06-03` | — | `internal/links/effect_propagation.go`<br>`internal/substrate/effect_sinks_scala.go` | Scala effect sniffer (effect_sinks_scala.go) recognises Source.fromFile/Files/os-lib read+write primitives; framework-agnostic. |
| HTTP effect | 🟢 `partial` | `2026-06-03` | — | `internal/links/effect_propagation.go`<br>`internal/substrate/effect_sinks_scala.go` | Scala effect sniffer (effect_sinks_scala.go) recognises outbound HTTP primitives (sttp/akka-pekko/http4s/requests); framework-agnostic. |
| Import resolution quality | 🟢 `partial` | `2026-05-30` | backfill:dictionary-completeness | `internal/custom/scala/caliban.go`<br>`internal/custom/scala/caliban_test.go` | Honest limit: RootResolver root binding is positional + intra-file; cross-file resolver case-class composition is not chased. |
| Module cycle detection | 🟢 `partial` | `2026-06-03` | — | `internal/links/module_cycle_pass.go` | Language-agnostic module-cycle pass uses IMPORTS edges emitted by the Scala extractor pipeline; framework-agnostic. |
| Mutation effect | 🟢 `partial` | `2026-06-03` | — | `internal/links/effect_propagation.go`<br>`internal/substrate/effect_sinks_scala.go` | Scala effect sniffer (effect_sinks_scala.go) recognises this.<field>= mutation; framework-agnostic. |
| Pure function tagging | 🟢 `partial` | `2026-06-03` | — | `internal/links/pure_function_pass.go` | Language-agnostic pure-function pass tags Scala functions with no effect properties; framework-agnostic (esp. apt for effectful/functional Scala idioms). |
| Reachability analysis | 🟢 `partial` | `2026-06-03` | — | `internal/links/reachability.go`<br>`internal/substrate/entry_points.go`<br>`internal/substrate/entry_points_scala.go` | Language-agnostic reachability pass seeded from Scala entry points (entry_points_scala.go); framework-agnostic. |
| Request shape extraction | 🟢 `partial` | `2026-05-30` | backfill:dictionary-completeness | `internal/custom/scala/caliban.go`<br>`internal/custom/scala/caliban_test.go` | Resolver field argument type (e.g. UserArgs in user: UserArgs => ...) is visible in the field declaration but not deeply parsed into a shape. |
| Request sink dataflow | 🔴 `missing` | — | 3958 | — | No dataflow sniffer covers this framework's request-binding forms yet. The Java sniffer (internal/substrate/dataflow_java.go, #3958) targets Spring MVC/WebFlux @RequestBody/@RequestParam/@PathVariable; Kotlin/Scala have no sniffer at all (no "kotlin"/"scala" slug registered). request_sink_dataflow remains a follow-up for these JVM frameworks. |
| Response shape extraction | 🟢 `partial` | `2026-05-30` | backfill:dictionary-completeness | `internal/custom/scala/caliban.go`<br>`internal/custom/scala/caliban_test.go` | Resolver field return type (e.g. List[User]) visible in declaration; not deeply parsed. |
| Sanitizer recognition | 🟢 `partial` | `2026-06-03` | — | `internal/links/taint_flow.go`<br>`internal/substrate/taint_sites_scala.go` | Scala taint sniffer (taint_sites_scala.go) recognises parameterised-SQL/HTML-escape/Form-mapping sanitizers; framework-agnostic. |
| Schema drift detection | 🔴 `missing` | — | backfill:dictionary-completeness | — | — |
| Taint sink detection | 🟢 `partial` | `2026-06-03` | — | `internal/links/taint_flow.go`<br>`internal/substrate/taint_sites_scala.go` | Scala taint sniffer (taint_sites_scala.go) recognises SQL-splice/command/path/XSS/ReDoS sinks; framework-agnostic. Fixture: TestScalaTrailing_Caliban_Taint. |
| Taint source detection | 🟢 `partial` | `2026-06-03` | — | `internal/links/taint_flow.go`<br>`internal/substrate/taint_sites_scala.go` | Scala taint sniffer (taint_sites_scala.go, RegisterTaintSniffer('scala')) recognises request/param/sys.env/decode sources; framework-agnostic, fires on any .scala file. Fixture: TestScalaTrailing_Caliban_Taint. |
| Template pattern catalog | 🔴 `missing` | — | backfill:dictionary-completeness | — | — |
| Vulnerability finding | 🟢 `partial` | `2026-06-03` | — | `internal/links/taint_flow.go`<br>`internal/substrate/taint_sites_scala.go` | Scala taint flow (taint_flow.go over taint_sites_scala.go) reports source->sink findings; framework-agnostic. Fixture: TestScalaTrailing_Caliban_Taint (source+sink in one resolver). |

## Provenance

This record is sourced from `docs/coverage/registry.json`. To update it, edit the JSON
(or use `go run ./tools/coverage update lang.scala.framework.caliban ...`) then regenerate:

```
go run ./tools/coverage validate
go run ./tools/coverage gen
```
