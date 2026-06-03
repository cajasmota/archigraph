<!-- DO NOT EDIT — generated from docs/coverage/registry.json by 'go run ./tools/coverage gen' -->
# `lang.php.framework.symfony` — Symfony

Auto-generated. Back to [summary](../summary.md).

- **Language:** [php](../by-language/php.md)
- **Category:** [http_framework](../by-category/http_framework.md)
- **Subcategory:** Backend HTTP
- **Capability cells:** 49

## Capabilities


### Routing

| Capability | Status | Verified at | Issue | Cites | Notes |
|------------|--------|-------------|-------|-------|-------|
| Endpoint deprecation versioning | 🟢 `partial` | `2026-06-03` | 3628 | `internal/custom/php/apiplatform.go`<br>`internal/custom/php/deprecation_test.go`<br>`internal/custom/php/helpers.go`<br>`internal/engine/http_endpoint_deprecation.go` | #3628 PHP port: deprecation is FULL — API Platform `#[ApiResource(deprecationReason: '...')]` (resource-wide, applies to every generated CRUD op) and per-operation `new Get(deprecationReason: '...')` stamp deprecated/deprecation_source(+deprecated_replacement) on the SCOPE.Operation endpoints at the source (operations-list scoping prevents a per-op reason leaking resource-wide). api_version is honest-partial: default resource paths (/books, /books/{id}) carry no /vN segment, so api_version only lights up when an operation's uriTemplate names one. Identical property contract to the flagship. |
| Endpoint pagination posture | 🔴 `missing` | `2026-06-02` | 3628 | `internal/engine/http_endpoint_pagination.go`<br>`internal/engine/http_endpoint_pagination_patterns.go`<br>`internal/engine/http_endpoint_pagination_test.go`<br>`internal/engine/http_endpoint_synthesis.go` | #3628: applyEndpointPagination stamps paginated/pagination_style/pagination_params via the cross-language parameters/parameter_schema fallback (limit+offset/page/cursor shape). No framework-specific pagination-class/ORM signal yet for this framework. |
| Endpoint response codes | 🔴 `missing` | — | 3818 | — | — |
| Endpoint synthesis | ✅ `full` | `2026-05-28` | — | `internal/engine/http_endpoint_php_producer.go`<br>`internal/engine/rules/php/frameworks/symfony.yaml` | — |
| Handler attribution | ✅ `full` | `2026-05-28` | — | `internal/engine/http_endpoint_php_producer.go` | — |
| Route extraction | ✅ `full` | `2026-05-30` | — | `internal/custom/php/symfony.go` | PHP8 attribute routes with methods/name, @Route annotations, YAML routes (config/routes.yaml) with path+method extraction; class-level prefix support |

### View

| Capability | Status | Verified at | Issue | Cites | Notes |
|------------|--------|-------------|-------|-------|-------|
| View rendering | 🔴 `missing` | — | view_rendering:#3628-not-yet-extracted | — | — |

### Auth

| Capability | Status | Verified at | Issue | Cites | Notes |
|------------|--------|-------------|-------|-------|-------|
| Auth coverage | ✅ `full` | `2026-05-30` | — | `internal/custom/php/symfony.go` | #[IsGranted] attribute, denyAccessUnlessGranted(), Voter/VoterInterface classes with voter attribute extraction, security.yaml access_control entries |

### Validation

| Capability | Status | Verified at | Issue | Cites | Notes |
|------------|--------|-------------|-------|-------|-------|
| DTO extraction | ✅ `full` | `2026-05-30` | — | `internal/custom/php/symfony.go` | #[Assert\NotBlank]/#[Assert\Length]/#[Assert\Email] PHP8 attribute constraints on entity/DTO properties; DTO class detection (Request/DTO/Input/Data suffix) |
| Request validation | ✅ `full` | `2026-05-30` | — | `internal/custom/php/symfony.go` | Assert constraint extraction (PHP8 attr + annotation), ->validate($obj) programmatic call detection |

### Middleware

| Capability | Status | Verified at | Issue | Cites | Notes |
|------------|--------|-------------|-------|-------|-------|
| Middleware coverage | ✅ `full` | `2026-05-30` | — | `internal/custom/php/symfony.go` | EventSubscriberInterface implementors with getSubscribedEvents() event name extraction; addListener/addSubscriber kernel event registration; kernel event names emitted as SCOPE.Pattern |
| Rate limit stamping | ✅ `full` | `2026-06-03` | 4073 | `internal/custom/php/symfony.go`<br>`internal/custom/php/symfony_ratelimit_test.go` | #4073: the custom_php_symfony extractor stamps the flat rate_limited/rate_limit_scope/rate_limit_source contract on Symfony route SCOPE.Operation endpoints when a #[RateLimiter('name')] attribute is co-located with the #[Route(...)] action (attribute order within the block is free). Honest-partial: the named limiter's limit/window live in config/packages/rate_limiter.yaml, so the numeric rate is omitted. A sibling action without the attribute is a negative; the limiter never mis-pairs across actions. |

### Schema

| Capability | Status | Verified at | Issue | Cites | Notes |
|------------|--------|-------------|-------|-------|-------|
| Type graph extraction | — `not_applicable` | — | — | — | GraphQL schema type→type graph (object-typed field -> referenced object type with list/nullable cardinality) is a GraphQL-only concept; this framework is not a GraphQL server, so it has no GraphQL object-type relationship graph. |

### Type System

| Capability | Status | Verified at | Issue | Cites | Notes |
|------------|--------|-------------|-------|-------|-------|
| Enum extraction | ✅ `full` | — | — | `internal/extractors/php/php.go` | PHP 8.1+ backed and pure enums: tree-sitter enum_declaration → SCOPE.Schema/enum with case names (enum_members), backed values (enum_member_values), and backing type (enum_backing_type). Full language-level extraction, framework-independent. |
| Interface extraction | ✅ `full` | — | — | `internal/extractors/php/php.go` | interface_declaration → SCOPE.Component/interface with CONTAINS edges to all declared methods (dotted Interface.method naming). Framework-independent language-level extraction. |
| Type alias extraction | — `not_applicable` | — | — | `internal/extractors/php/php.go` | PHP has no native type alias syntax. @phpstan-type/@psalm-type docblock aliases exist as third-party static-analysis conventions only, not a language feature. not_applicable at the language level. |
| Type extraction | ✅ `full` | — | — | `internal/extractors/php/php.go` | class_declaration → SCOPE.Component/class and trait_declaration → SCOPE.Component/trait, both with CONTAINS edges to methods. Framework-independent language-level extraction via tree-sitter. |

### DI

| Capability | Status | Verified at | Issue | Cites | Notes |
|------------|--------|-------------|-------|-------|-------|
| DI binding extraction | 🟢 `partial` | `2026-06-02` | — | `internal/custom/php/di_graph.go`<br>`internal/custom/php/di_graph_test.go` | Symfony services.yaml service aliases (Foo: \"@Bar\") emit BINDS(Foo->Bar). Value-asserted TestPhpDI_SymfonyServicesYAML (TransportInterface->SmtpTransport). PARTIAL: only YAML alias/@ref form; XML/PHP config + factory/decorator services not parsed (honest-partial). |
| DI injection point | 🟢 `partial` | `2026-06-02` | — | `internal/custom/php/di_graph.go`<br>`internal/custom/php/di_graph_test.go` | Symfony autowiring via constructor type-hints emits INJECTED_INTO(type->class) (shared php_constructor pass with Laravel). Value-asserted TestPhpDI_ConstructorInjection; negative TestPhpDI_ScalarParamNoEdge. PARTIAL: #[Required] setter / property injection + #[Autowire] attribute targets not linked. |
| DI scope resolution | 🔴 `missing` | — | 3628 | — | — |

### Testing

| Capability | Status | Verified at | Issue | Cites | Notes |
|------------|--------|-------------|-------|-------|-------|
| Tests linkage | ✅ `full` | — | — | `internal/extractors/cross/testmap/extractor_test.go`<br>`internal/extractors/cross/testmap/frameworks.go`<br>`internal/extractors/cross/testmap/resolver.go` | Deep PHPUnit/Pest linkage (#3399): test* prefix + #[Test] attribute + @test docblock detection; class-name subject derivation (UserServiceTest→UserService); instantiation body hints (new Foo()); Pest it()/test() blocks with uses(Class::class) subject extraction; TESTS edge emitted for all three PHPUnit forms and Pest DSL; Symfony-specific TestCase subclasses covered by phpunit class regex |

### Observability

| Capability | Status | Verified at | Issue | Cites | Notes |
|------------|--------|-------------|-------|-------|-------|
| Log extraction | 🟢 `partial` | `2026-05-30` | backfill:dictionary-completeness | `internal/custom/php/frameworks.go`<br>`internal/custom/php/observability.go` | Per-call-site: PSR-3/Monolog injected $logger->level() / $this->logger->level() (LoggerInterface). Log::level() Laravel facade patterns also captured. Symfony-specific: $this->logger->log(LogLevel::X) calls. Use-declaration fallback for Monolog\Logger and Psr\Log\LoggerInterface use-statements. Import/call-site heuristic; no cross-file dataflow binding logger to handler. Stays partial. |
| Metric extraction | 🟢 `partial` | `2026-05-30` | backfill:dictionary-completeness | `internal/custom/php/frameworks.go`<br>`internal/custom/php/observability.go` | prometheus_client_php: new Counter/Gauge/Histogram/Summary, registerCounter/Gauge/Histogram call-sites. StatsD (League\StatsD, Domnikl\Statsd): $statsd->increment/gauge/timing/histogram with metric names. Import/call-site heuristic; no cross-file dataflow. Stays partial. |
| Trace extraction | 🟢 `partial` | `2026-05-30` | backfill:dictionary-completeness | `internal/custom/php/frameworks.go`<br>`internal/custom/php/observability.go` | OpenTelemetry PHP SDK: CachedInstrumentation/Globals::tracerProvider() bootstrap, $tracer->spanBuilder/startSpan('name') call-sites, $span lifecycle markers (setAttribute/addEvent/setStatus/end). Symfony Stopwatch: $stopwatch->start/stop/lap('name') event names. DDTrace: trace_function/trace_method. Import/call-site heuristic; no cross-file dataflow binding tracer to exporter. Stays partial. |

### Data

| Capability | Status | Verified at | Issue | Cites | Notes |
|------------|--------|-------------|-------|-------|-------|

### Substrate

| Capability | Status | Verified at | Issue | Cites | Notes |
|------------|--------|-------------|-------|-------|-------|
| Confidence overlay | ✅ `full` | `2026-05-28` | — | `internal/graph/graph.go`<br>`internal/mcp/tools.go`<br>`internal/types/confidence.go` | — |
| Config consumption | ✅ `full` | `2026-06-02` | 3641 | `internal/extractor/config_key.go`<br>`internal/extractors/php/config_consumer.go`<br>`internal/extractors/php/config_consumer_test.go` | getenv/$_ENV + Laravel env()/config() -> config:<key> DEPENDS_ON_CONFIG edges (issue #3641) |
| Constant propagation | ✅ `full` | `2026-05-27` | — | `internal/links/constant_propagation.go`<br>`internal/substrate/php.go`<br>`internal/substrate/substrate.go` | — |
| DB effect | 🟢 `partial` | `2026-05-28` | — | `internal/links/effect_propagation.go`<br>`internal/substrate/effect_sinks_php.go` | — |
| Dead code detection | 🟢 `partial` | `2026-05-28` | — | `internal/links/reachability.go`<br>`internal/mcp/dead_code.go`<br>`internal/substrate/entry_points.go`<br>`internal/substrate/entry_points_php.go` | Framework-agnostic reachability pass via php open-tag and public-function entry points; lifecycle hooks matched by name heuristic only — unified status across all PHP frameworks |
| Def use chain extraction | 🟢 `partial` | — | backfill:dictionary-completeness | `internal/links/def_use_pass.go`<br>`internal/substrate/def_use_php.go` | PHP def-use sniffer registered in substrate; flows through language-agnostic def_use_pass |
| Env fallback recognition | ✅ `full` | `2026-05-27` | — | `internal/links/constant_propagation.go`<br>`internal/substrate/php.go`<br>`internal/substrate/substrate.go` | — |
| Error flow | ✅ `full` | `2026-06-03` | — | `internal/extractor/exception_flow.go`<br>`internal/extractors/php/exception_flow.go`<br>`internal/extractors/php/exception_flow_test.go` | throw new X / throw new \Ns\X -> THROWS; catch (X $e) incl PHP8 union A|B -> CATCHES; broad \Throwable/\Exception recorded (typed); re-throw $e / dynamic dropped (#3628) |
| Feature flag gating | ✅ `full` | `2026-06-03` | 4154 | `internal/engine/feature_flag_edges.go`<br>`internal/engine/feature_flag_edges_test.go`<br>`internal/engine/orm_queries.go` | PHP flag-check call sites -> feature:<key> + GATED_BY edge (framework-agnostic engine pass). Generic SDKs verified to fire on PHP: OpenFeature getBooleanValue, Unleash isEnabled, LaunchDarkly variation. Symfony-first idiom added & verified: Flagception $featureManager->isActive('key'), receiver-gated on a flag/feature receiver token. Honest-partial: dynamic keys emit nothing; $user->isActive('x') (no flag/feature receiver) emits nothing. Edges attach to the PHP enclosing function (indexEnclosingFunctions supports php). |
| Fs effect | 🟢 `partial` | `2026-05-28` | — | `internal/links/effect_propagation.go`<br>`internal/substrate/effect_sinks_php.go` | — |
| HTTP effect | 🟢 `partial` | `2026-05-28` | — | `internal/links/effect_propagation.go`<br>`internal/substrate/effect_sinks_php.go` | — |
| Import resolution quality | 🟢 `partial` | `2026-05-27` | — | `internal/links/constant_propagation.go`<br>`internal/substrate/php.go`<br>`internal/substrate/substrate.go` | — |
| Module cycle detection | 🟢 `partial` | — | backfill:dictionary-completeness | `internal/extractors/php/php.go`<br>`internal/links/module_cycle_pass.go` | Language-agnostic Tarjan SCC over IMPORTS edges; PHP namespace_use_declaration edges emitted by tree-sitter extractor |
| Mutation effect | 🟢 `partial` | `2026-05-28` | — | `internal/links/effect_propagation.go`<br>`internal/substrate/effect_sinks_php.go` | — |
| Pure function tagging | 🟢 `partial` | — | backfill:dictionary-completeness | `internal/links/pure_function_pass.go`<br>`internal/substrate/effect_sinks_php.go` | Language-agnostic pure-function tagging pass reads effect stamps from PHP effect_sinks substrate |
| Reachability analysis | 🟢 `partial` | `2026-05-28` | — | `internal/links/reachability.go`<br>`internal/substrate/entry_points.go`<br>`internal/substrate/entry_points_php.go` | — |
| Request shape extraction | 🟢 `partial` | `2026-05-28` | [link](https://github.com/cajasmota/archigraph/issues/2771) | `internal/links/payload_drift.go`<br>`internal/mcp/payload_drift_tool.go`<br>`internal/substrate/payload_shapes.go`<br>`internal/substrate/payload_shapes_php.go` | — |
| Request sink dataflow | 🟢 `partial` | `2026-06-02` | 3966 | `internal/links/dataflow_pass.go`<br>`internal/substrate/dataflow.go`<br>`internal/substrate/dataflow_php.go`<br>`internal/substrate/dataflow_php_test.go` | SCOPED request-input → sink DATA_FLOWS_TO (#3628 area #22, epic #3872, #3966): new PHP sniffer (internal/substrate/dataflow_php.go) mirroring the ruby/python/jsts model — intra-method assignment tracking over `function … { … }` brace bodies + multi-hop (≤DataFlowMaxHops=3) local same-file method-call propagation ($this->m()/self::m()/free fn), each bound by exact positional index, AND cross-file boundary emission continued by the links pass into resolved same-repo callees (continueDataFlowPHP). Sources: $request->input/query/post/get/json/header/cookie(’x’), $request->request->get(’x’)/$request->query->get(’x’) (Symfony), $request->get(’x’), request(’x’), $request->prop, $_POST/$_GET/$_REQUEST[’x’]; whole-request $request->all()/validated()/only/except → field="" (recovered at $data[’x’] index reads). Sinks: Eloquent write (Model::create/update/insert/firstOrCreate/updateOrCreate, $m->save()/update()), Doctrine $em->persist, raw SQL (DB::insert/statement, $pdo->query/prepare/execute), response (response()->json/make/stream, view, json_encode), outbound HTTP (Guzzle $client->post/get/request, Laravel Http::post). HONEST-PARTIAL (precision-first): drops reassignment, dynamic keys ($request->input($k)), embedded-arg, splat (...$args) call sites, recursion/cycle, the 4th hop, external/unresolved imports; whole-array mass-assignment flows with field="". DEPLOY-DEFERRED (daemon not rebuilt). COMPLETES the cross-language dataflow generalization (py/jsts/go/ruby/java/php); C# dataflow is #3960 (separate). Symfony: $request->request->get/$request->query->get/$request->get + Doctrine $em->persist/response sinks. |
| Response shape extraction | 🟢 `partial` | `2026-05-28` | [link](https://github.com/cajasmota/archigraph/issues/2771) | `internal/links/payload_drift.go`<br>`internal/mcp/payload_drift_tool.go`<br>`internal/substrate/payload_shapes.go`<br>`internal/substrate/payload_shapes_php.go` | — |
| Sanitizer recognition | 🟢 `partial` | `2026-05-28` | — | `internal/links/taint_flow.go`<br>`internal/substrate/taint_sites_php.go` | — |
| Schema drift detection | 🟢 `partial` | `2026-05-28` | [link](https://github.com/cajasmota/archigraph/issues/2771) | `internal/links/payload_drift.go`<br>`internal/mcp/payload_drift_tool.go`<br>`internal/substrate/payload_shapes.go`<br>`internal/substrate/payload_shapes_php.go` | — |
| Taint sink detection | 🟢 `partial` | `2026-05-28` | — | `internal/links/taint_flow.go`<br>`internal/substrate/taint_sites_php.go` | — |
| Taint source detection | 🟢 `partial` | `2026-05-28` | — | `internal/links/taint_flow.go`<br>`internal/substrate/taint_sites_php.go` | — |
| Template pattern catalog | 🟢 `partial` | — | backfill:dictionary-completeness | `internal/links/constant_propagation.go`<br>`internal/substrate/template_pattern_php.go` | PHP template-pattern sniffer registered; covers i18n trans(), log literals, SQL strings |
| Vulnerability finding | 🟢 `partial` | `2026-05-28` | — | `internal/links/taint_flow.go`<br>`internal/substrate/taint_sites_php.go` | — |

## Provenance

This record is sourced from `docs/coverage/registry.json`. To update it, edit the JSON
(or use `go run ./tools/coverage update lang.php.framework.symfony ...`) then regenerate:

```
go run ./tools/coverage validate
go run ./tools/coverage gen
```
