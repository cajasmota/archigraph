<!-- DO NOT EDIT — generated from docs/coverage.json by 'go run ./tools/coverage gen' -->
# Coverage Registry — Summary

Auto-generated from `docs/coverage.json` by `go run ./tools/coverage gen`.
Each capability cell is one of: `full`, `partial`, `missing`, `not_applicable`.

## Totals

- Records: **180**
- Capability cells: **367**
- Full: **216**
- Partial: **46**
- Missing: **105**
- Not applicable: **0**

## By language

| Language | Records | Full | Partial | Missing | N/A | % Full |
|----------|---------|------|---------|---------|-----|--------|
| [clojure](by-language/clojure.md) | 1 | 1 | 1 | 1 | 0 | 33% |
| [cpp](by-language/cpp.md) | 1 | 1 | 1 | 1 | 0 | 33% |
| [crystal](by-language/crystal.md) | 1 | 1 | 1 | 0 | 0 | 50% |
| [csharp](by-language/csharp.md) | 6 | 3 | 4 | 7 | 0 | 21% |
| [dart](by-language/dart.md) | 2 | 2 | 1 | 2 | 0 | 40% |
| [elixir](by-language/elixir.md) | 5 | 4 | 3 | 5 | 0 | 33% |
| [erlang](by-language/erlang.md) | 1 | 1 | 1 | 0 | 0 | 50% |
| [fsharp](by-language/fsharp.md) | 1 | 1 | 1 | 0 | 0 | 50% |
| [go](by-language/go.md) | 12 | 19 | 1 | 6 | 0 | 73% |
| [groovy](by-language/groovy.md) | 1 | 1 | 1 | 0 | 0 | 50% |
| [haskell](by-language/haskell.md) | 1 | 1 | 1 | 0 | 0 | 50% |
| [java](by-language/java.md) | 13 | 18 | 5 | 4 | 0 | 66% |
| [javascript](by-language/javascript.md) | 22 | 25 | 4 | 13 | 0 | 59% |
| [kotlin](by-language/kotlin.md) | 4 | 5 | 0 | 3 | 0 | 62% |
| [lua](by-language/lua.md) | 2 | 0 | 0 | 2 | 0 | 0% |
| [multi](by-language/multi.md) | 50 | 76 | 7 | 17 | 0 | 76% |
| [nim](by-language/nim.md) | 1 | 1 | 1 | 0 | 0 | 50% |
| [ocaml](by-language/ocaml.md) | 1 | 1 | 1 | 0 | 0 | 50% |
| [php](by-language/php.md) | 7 | 3 | 2 | 9 | 0 | 21% |
| [python](by-language/python.md) | 25 | 29 | 5 | 18 | 0 | 55% |
| [ruby](by-language/ruby.md) | 8 | 9 | 0 | 7 | 0 | 56% |
| [rust](by-language/rust.md) | 7 | 7 | 0 | 5 | 0 | 58% |
| [scala](by-language/scala.md) | 2 | 1 | 1 | 2 | 0 | 25% |
| [solidity](by-language/solidity.md) | 1 | 1 | 1 | 0 | 0 | 50% |
| [swift](by-language/swift.md) | 3 | 1 | 1 | 3 | 0 | 20% |
| [typescript](by-language/typescript.md) | 1 | 4 | 0 | 0 | 0 | 100% |
| [zig](by-language/zig.md) | 1 | 0 | 2 | 0 | 0 | 0% |

## By category

| Category | Records |
|----------|---------|
| [build_system](by-category/build_system.md) | 4 |
| [configuration](by-category/configuration.md) | 10 |
| [http_framework](by-category/http_framework.md) | 62 |
| [infrastructure](by-category/infrastructure.md) | 6 |
| [language](by-category/language.md) | 25 |
| [message_broker](by-category/message_broker.md) | 23 |
| [observability](by-category/observability.md) | 6 |
| [orm](by-category/orm.md) | 20 |
| [package_manager](by-category/package_manager.md) | 15 |
| [protocol](by-category/protocol.md) | 4 |
| [security](by-category/security.md) | 5 |

## All records

| ID | Language | Category | Label |
|----|----------|----------|-------|
| [build.bazel](detail/build.bazel.md) | multi | build_system | Bazel / BUCK / WORKSPACE |
| [build.dockerfile](detail/build.dockerfile.md) | multi | build_system | Dockerfile |
| [build.justfile](detail/build.justfile.md) | multi | build_system | Justfile |
| [build.makefile](detail/build.makefile.md) | multi | build_system | Makefile |
| [config.docker-compose](detail/config.docker-compose.md) | multi | configuration | docker-compose.yml |
| [config.dotenv](detail/config.dotenv.md) | multi | configuration | .env (names-only — values stripped at extraction boundary) |
| [config.github-actions](detail/config.github-actions.md) | multi | configuration | GitHub Actions workflows |
| [config.gitlab-ci](detail/config.gitlab-ci.md) | multi | configuration | GitLab CI |
| [config.ini](detail/config.ini.md) | multi | configuration | .ini / setup.cfg / flake8 / mypy / pytest.ini |
| [config.jenkins](detail/config.jenkins.md) | multi | configuration | Jenkinsfile / Jenkins Pipeline DSL |
| [config.properties](detail/config.properties.md) | java | configuration | .properties (application.properties) |
| [config.toml](detail/config.toml.md) | multi | configuration | .toml |
| [config.tsconfig](detail/config.tsconfig.md) | javascript | configuration | tsconfig.json |
| [config.yaml](detail/config.yaml.md) | multi | configuration | .yaml / .yml |
| [infra.observability.datadog](detail/infra.observability.datadog.md) | multi | observability | Datadog APM / StatsD |
| [infra.observability.logging-config](detail/infra.observability.logging-config.md) | multi | observability | Generic logging-config extractor (Python logging, Go slog, Node winston/pino, .NET NLog/Serilog, log4j/logback) |
| [infra.observability.newrelic](detail/infra.observability.newrelic.md) | multi | observability | New Relic |
| [infra.observability.opentelemetry](detail/infra.observability.opentelemetry.md) | multi | observability | OpenTelemetry instrumentation |
| [infra.observability.prometheus](detail/infra.observability.prometheus.md) | multi | observability | Prometheus client libraries |
| [infra.observability.sentry](detail/infra.observability.sentry.md) | multi | observability | Sentry SDK |
| [infra.resource.aws-cdk](detail/infra.resource.aws-cdk.md) | multi | infrastructure | AWS CDK |
| [infra.resource.cloudformation](detail/infra.resource.cloudformation.md) | multi | infrastructure | AWS CloudFormation |
| [infra.resource.helm](detail/infra.resource.helm.md) | multi | infrastructure | Helm charts |
| [infra.resource.kubernetes](detail/infra.resource.kubernetes.md) | multi | infrastructure | Kubernetes manifests |
| [infra.resource.pulumi](detail/infra.resource.pulumi.md) | multi | infrastructure | Pulumi |
| [infra.resource.terraform](detail/infra.resource.terraform.md) | multi | infrastructure | Terraform / OpenTofu / Vault / Nomad / Packer / Waypoint |
| [lang.clojure](detail/lang.clojure.md) | clojure | language | Clojure |
| [lang.cpp](detail/lang.cpp.md) | cpp | language | C++ |
| [lang.crystal](detail/lang.crystal.md) | crystal | language | Crystal |
| [lang.csharp](detail/lang.csharp.md) | csharp | language | C# |
| [lang.csharp.framework.aspnet-core](detail/lang.csharp.framework.aspnet-core.md) | csharp | http_framework | ASP.NET Core |
| [lang.csharp.framework.aspnet-mvc](detail/lang.csharp.framework.aspnet-mvc.md) | csharp | http_framework | ASP.NET MVC (attribute-route subset) |
| [lang.csharp.framework.blazor](detail/lang.csharp.framework.blazor.md) | csharp | http_framework | Blazor Server / WebAssembly |
| [lang.csharp.orm.efcore](detail/lang.csharp.orm.efcore.md) | csharp | orm | Entity Framework Core |
| [lang.dart](detail/lang.dart.md) | dart | language | Dart |
| [lang.elixir](detail/lang.elixir.md) | elixir | language | Elixir |
| [lang.elixir.framework.phoenix](detail/lang.elixir.framework.phoenix.md) | elixir | http_framework | Phoenix |
| [lang.elixir.framework.phoenix-liveview](detail/lang.elixir.framework.phoenix-liveview.md) | elixir | http_framework | Phoenix LiveView (pages indexed; lifecycle hooks not surfaced) |
| [lang.elixir.orm.ecto](detail/lang.elixir.orm.ecto.md) | elixir | orm | Ecto |
| [lang.erlang](detail/lang.erlang.md) | erlang | language | Erlang |
| [lang.fsharp](detail/lang.fsharp.md) | fsharp | language | F# |
| [lang.go](detail/lang.go.md) | go | language | Go |
| [lang.go.framework.beego](detail/lang.go.framework.beego.md) | go | http_framework | Beego |
| [lang.go.framework.chi](detail/lang.go.framework.chi.md) | go | http_framework | go-chi/chi |
| [lang.go.framework.echo](detail/lang.go.framework.echo.md) | go | http_framework | labstack/echo |
| [lang.go.framework.fiber](detail/lang.go.framework.fiber.md) | go | http_framework | gofiber/fiber |
| [lang.go.framework.gin](detail/lang.go.framework.gin.md) | go | http_framework | gin-gonic/gin |
| [lang.go.framework.gorilla-mux](detail/lang.go.framework.gorilla-mux.md) | go | http_framework | gorilla/mux |
| [lang.go.framework.huma](detail/lang.go.framework.huma.md) | go | http_framework | Huma |
| [lang.go.framework.net-http](detail/lang.go.framework.net-http.md) | go | http_framework | Go net/http stdlib |
| [lang.go.orm.ent](detail/lang.go.orm.ent.md) | go | orm | ent |
| [lang.go.orm.gorm](detail/lang.go.orm.gorm.md) | go | orm | GORM |
| [lang.groovy](detail/lang.groovy.md) | groovy | language | Groovy |
| [lang.haskell](detail/lang.haskell.md) | haskell | language | Haskell |
| [lang.java](detail/lang.java.md) | java | language | Java |
| [lang.java.framework.dropwizard](detail/lang.java.framework.dropwizard.md) | java | http_framework | Dropwizard (JAX-RS subset) |
| [lang.java.framework.jaxrs](detail/lang.java.framework.jaxrs.md) | java | http_framework | JAX-RS / Jakarta EE |
| [lang.java.framework.micronaut](detail/lang.java.framework.micronaut.md) | java | http_framework | Micronaut |
| [lang.java.framework.quarkus](detail/lang.java.framework.quarkus.md) | java | http_framework | Quarkus (JAX-RS-backed) |
| [lang.java.framework.spring-boot](detail/lang.java.framework.spring-boot.md) | java | http_framework | Spring Boot / Spring MVC |
| [lang.java.framework.spring-webflux](detail/lang.java.framework.spring-webflux.md) | java | http_framework | Spring WebFlux |
| [lang.java.orm.hibernate](detail/lang.java.orm.hibernate.md) | java | orm | Hibernate / JPA |
| [lang.java.orm.spring-data-jpa](detail/lang.java.orm.spring-data-jpa.md) | java | orm | Spring Data JPA |
| [lang.javascript](detail/lang.javascript.md) | javascript | language | JavaScript |
| [lang.javascript.framework.angular](detail/lang.javascript.framework.angular.md) | javascript | http_framework | Angular |
| [lang.javascript.framework.astro](detail/lang.javascript.framework.astro.md) | javascript | http_framework | Astro |
| [lang.javascript.framework.express](detail/lang.javascript.framework.express.md) | javascript | http_framework | Express.js |
| [lang.javascript.framework.fastify](detail/lang.javascript.framework.fastify.md) | javascript | http_framework | Fastify |
| [lang.javascript.framework.graphql-resolvers](detail/lang.javascript.framework.graphql-resolvers.md) | javascript | http_framework | GraphQL resolvers (Apollo / Yoga) |
| [lang.javascript.framework.hono](detail/lang.javascript.framework.hono.md) | javascript | http_framework | Hono |
| [lang.javascript.framework.koa](detail/lang.javascript.framework.koa.md) | javascript | http_framework | Koa |
| [lang.javascript.framework.nestjs](detail/lang.javascript.framework.nestjs.md) | javascript | http_framework | NestJS |
| [lang.javascript.framework.next-api](detail/lang.javascript.framework.next-api.md) | javascript | http_framework | Next.js API routes |
| [lang.javascript.framework.nuxt](detail/lang.javascript.framework.nuxt.md) | javascript | http_framework | Nuxt |
| [lang.javascript.framework.remix](detail/lang.javascript.framework.remix.md) | javascript | http_framework | Remix |
| [lang.javascript.framework.sveltekit](detail/lang.javascript.framework.sveltekit.md) | javascript | http_framework | Svelte / SvelteKit |
| [lang.javascript.framework.trpc](detail/lang.javascript.framework.trpc.md) | javascript | http_framework | tRPC |
| [lang.javascript.orm.drizzle](detail/lang.javascript.orm.drizzle.md) | javascript | orm | Drizzle ORM |
| [lang.javascript.orm.mongoose](detail/lang.javascript.orm.mongoose.md) | javascript | orm | Mongoose |
| [lang.javascript.orm.prisma](detail/lang.javascript.orm.prisma.md) | javascript | orm | Prisma |
| [lang.javascript.orm.sequelize](detail/lang.javascript.orm.sequelize.md) | javascript | orm | Sequelize |
| [lang.javascript.orm.typeorm](detail/lang.javascript.orm.typeorm.md) | javascript | orm | TypeORM |
| [lang.kotlin](detail/lang.kotlin.md) | kotlin | language | Kotlin |
| [lang.kotlin.framework.http4k](detail/lang.kotlin.framework.http4k.md) | kotlin | http_framework | http4k |
| [lang.kotlin.framework.ktor](detail/lang.kotlin.framework.ktor.md) | kotlin | http_framework | Ktor |
| [lang.kotlin.framework.spring-boot](detail/lang.kotlin.framework.spring-boot.md) | kotlin | http_framework | Spring Boot (Kotlin) |
| [lang.lua.framework.lapis](detail/lang.lua.framework.lapis.md) | lua | http_framework | Lapis |
| [lang.lua.framework.openresty](detail/lang.lua.framework.openresty.md) | lua | http_framework | OpenResty |
| [lang.nim](detail/lang.nim.md) | nim | language | Nim |
| [lang.ocaml](detail/lang.ocaml.md) | ocaml | language | OCaml |
| [lang.php](detail/lang.php.md) | php | language | PHP |
| [lang.php.framework.laravel](detail/lang.php.framework.laravel.md) | php | http_framework | Laravel |
| [lang.php.framework.slim](detail/lang.php.framework.slim.md) | php | http_framework | Slim |
| [lang.php.framework.symfony](detail/lang.php.framework.symfony.md) | php | http_framework | Symfony |
| [lang.php.orm.doctrine](detail/lang.php.orm.doctrine.md) | php | orm | Doctrine |
| [lang.php.orm.eloquent](detail/lang.php.orm.eloquent.md) | php | orm | Eloquent (Laravel ActiveRecord) |
| [lang.python](detail/lang.python.md) | python | language | Python |
| [lang.python.framework.aiohttp](detail/lang.python.framework.aiohttp.md) | python | http_framework | aiohttp |
| [lang.python.framework.bottle](detail/lang.python.framework.bottle.md) | python | http_framework | Bottle |
| [lang.python.framework.django](detail/lang.python.framework.django.md) | python | http_framework | Django (URLconf) |
| [lang.python.framework.django-drf](detail/lang.python.framework.django-drf.md) | python | http_framework | Django REST Framework |
| [lang.python.framework.fastapi](detail/lang.python.framework.fastapi.md) | python | http_framework | FastAPI |
| [lang.python.framework.flask](detail/lang.python.framework.flask.md) | python | http_framework | Flask |
| [lang.python.framework.litestar](detail/lang.python.framework.litestar.md) | python | http_framework | Litestar |
| [lang.python.framework.pyramid](detail/lang.python.framework.pyramid.md) | python | http_framework | Pyramid |
| [lang.python.framework.robyn](detail/lang.python.framework.robyn.md) | python | http_framework | Robyn |
| [lang.python.framework.sanic](detail/lang.python.framework.sanic.md) | python | http_framework | Sanic |
| [lang.python.framework.starlette](detail/lang.python.framework.starlette.md) | python | http_framework | Starlette |
| [lang.python.framework.tornado](detail/lang.python.framework.tornado.md) | python | http_framework | Tornado |
| [lang.python.orm.django](detail/lang.python.orm.django.md) | python | orm | Django ORM |
| [lang.python.orm.mongoengine](detail/lang.python.orm.mongoengine.md) | python | orm | MongoEngine |
| [lang.python.orm.peewee](detail/lang.python.orm.peewee.md) | python | orm | Peewee |
| [lang.python.orm.sqlalchemy](detail/lang.python.orm.sqlalchemy.md) | python | orm | SQLAlchemy |
| [lang.python.orm.sqlmodel](detail/lang.python.orm.sqlmodel.md) | python | orm | SQLModel |
| [lang.python.orm.tortoise](detail/lang.python.orm.tortoise.md) | python | orm | Tortoise ORM |
| [lang.ruby](detail/lang.ruby.md) | ruby | language | Ruby |
| [lang.ruby.framework.grape](detail/lang.ruby.framework.grape.md) | ruby | http_framework | Grape |
| [lang.ruby.framework.hanami](detail/lang.ruby.framework.hanami.md) | ruby | http_framework | Hanami |
| [lang.ruby.framework.rails](detail/lang.ruby.framework.rails.md) | ruby | http_framework | Ruby on Rails |
| [lang.ruby.framework.sinatra](detail/lang.ruby.framework.sinatra.md) | ruby | http_framework | Sinatra |
| [lang.ruby.orm.activerecord](detail/lang.ruby.orm.activerecord.md) | ruby | orm | ActiveRecord |
| [lang.rust](detail/lang.rust.md) | rust | language | Rust |
| [lang.rust.framework.actix](detail/lang.rust.framework.actix.md) | rust | http_framework | Actix |
| [lang.rust.framework.axum](detail/lang.rust.framework.axum.md) | rust | http_framework | Axum |
| [lang.rust.framework.hyper](detail/lang.rust.framework.hyper.md) | rust | http_framework | Hyper |
| [lang.rust.framework.rocket](detail/lang.rust.framework.rocket.md) | rust | http_framework | Rocket |
| [lang.rust.framework.warp](detail/lang.rust.framework.warp.md) | rust | http_framework | Warp |
| [lang.scala](detail/lang.scala.md) | scala | language | Scala |
| [lang.solidity](detail/lang.solidity.md) | solidity | language | Solidity |
| [lang.swift](detail/lang.swift.md) | swift | language | Swift |
| [lang.swift.framework.vapor](detail/lang.swift.framework.vapor.md) | swift | http_framework | Vapor |
| [lang.typescript](detail/lang.typescript.md) | typescript | language | TypeScript (shares the JavaScript extractor) |
| [lang.zig](detail/lang.zig.md) | zig | language | Zig |
| [msg.broker.cloudevents](detail/msg.broker.cloudevents.md) | multi | message_broker | CloudEvents |
| [msg.broker.debezium](detail/msg.broker.debezium.md) | multi | message_broker | Debezium / Kafka Connect CDC |
| [msg.broker.eventbridge](detail/msg.broker.eventbridge.md) | multi | message_broker | AWS EventBridge |
| [msg.broker.eventgrid](detail/msg.broker.eventgrid.md) | multi | message_broker | Azure Event Grid |
| [msg.broker.gcp-pubsub](detail/msg.broker.gcp-pubsub.md) | multi | message_broker | GCP Pub/Sub |
| [msg.broker.kafka](detail/msg.broker.kafka.md) | multi | message_broker | Apache Kafka |
| [msg.broker.mqtt](detail/msg.broker.mqtt.md) | multi | message_broker | MQTT |
| [msg.broker.nats](detail/msg.broker.nats.md) | multi | message_broker | NATS |
| [msg.broker.pulsar](detail/msg.broker.pulsar.md) | multi | message_broker | Apache Pulsar |
| [msg.broker.rabbitmq](detail/msg.broker.rabbitmq.md) | multi | message_broker | RabbitMQ |
| [msg.broker.redis](detail/msg.broker.redis.md) | multi | message_broker | Redis pub/sub + Streams |
| [msg.broker.sns](detail/msg.broker.sns.md) | multi | message_broker | AWS SNS (IaC-declared) |
| [msg.broker.sqs](detail/msg.broker.sqs.md) | multi | message_broker | AWS SQS |
| [msg.bullmq](detail/msg.bullmq.md) | javascript | message_broker | BullMQ / bull (Node task queue) |
| [msg.celery](detail/msg.celery.md) | python | message_broker | Celery (Python task queue) |
| [msg.django-signals](detail/msg.django-signals.md) | python | message_broker | Django signals (intra-repo pub/sub) |
| [msg.dramatiq](detail/msg.dramatiq.md) | python | message_broker | Dramatiq (Python task queue) |
| [msg.graphql-subscriptions](detail/msg.graphql-subscriptions.md) | multi | message_broker | GraphQL subscriptions |
| [msg.kafka-streams](detail/msg.kafka-streams.md) | multi | message_broker | Kafka Streams / Faust |
| [msg.sidekiq](detail/msg.sidekiq.md) | ruby | message_broker | Sidekiq (Ruby task queue) |
| [msg.sse](detail/msg.sse.md) | multi | message_broker | Server-Sent Events |
| [msg.webhook](detail/msg.webhook.md) | multi | message_broker | Webhook inbound (Stripe, GitHub, Twilio, Slack, SendGrid, Mailgun, ...) |
| [msg.websocket](detail/msg.websocket.md) | multi | message_broker | WebSocket channels |
| [pkg.cargo](detail/pkg.cargo.md) | rust | package_manager | Cargo.toml |
| [pkg.composer](detail/pkg.composer.md) | php | package_manager | composer.json |
| [pkg.csproj](detail/pkg.csproj.md) | csharp | package_manager | .csproj / packages.config |
| [pkg.gemfile](detail/pkg.gemfile.md) | ruby | package_manager | Gemfile |
| [pkg.go-mod](detail/pkg.go-mod.md) | go | package_manager | go.mod |
| [pkg.gradle](detail/pkg.gradle.md) | java | package_manager | build.gradle / build.gradle.kts |
| [pkg.mix](detail/pkg.mix.md) | elixir | package_manager | mix.exs |
| [pkg.npm](detail/pkg.npm.md) | javascript | package_manager | package.json (npm/yarn/pnpm) |
| [pkg.pipfile](detail/pkg.pipfile.md) | python | package_manager | Pipfile / Pipfile.lock |
| [pkg.pom](detail/pkg.pom.md) | java | package_manager | pom.xml |
| [pkg.pubspec](detail/pkg.pubspec.md) | dart | package_manager | pubspec.yaml |
| [pkg.pyproject](detail/pkg.pyproject.md) | python | package_manager | pyproject.toml |
| [pkg.requirements](detail/pkg.requirements.md) | python | package_manager | requirements.txt |
| [pkg.sbt](detail/pkg.sbt.md) | scala | package_manager | build.sbt |
| [pkg.swift-package](detail/pkg.swift-package.md) | swift | package_manager | Package.swift / Podfile |
| [protocol.graphql](detail/protocol.graphql.md) | multi | protocol | GraphQL SDL (Query/Mutation/Subscription) |
| [protocol.grpc](detail/protocol.grpc.md) | multi | protocol | gRPC services |
| [protocol.openapi](detail/protocol.openapi.md) | multi | protocol | OpenAPI / Swagger spec |
| [protocol.protobuf](detail/protocol.protobuf.md) | multi | protocol | Protocol Buffers (.proto) |
| [security.auth-java](detail/security.auth-java.md) | java | security | Auth policy resolver (Java/Kotlin — Phase 1 of #1942) |
| [security.auth-other](detail/security.auth-other.md) | multi | security | Auth policy resolver (Python / NestJS / Go / Ruby / ASP.NET — Phases 2-4 of #1942) |
| [security.csrf](detail/security.csrf.md) | multi | security | CSRF heuristic detector |
| [security.secrets](detail/security.secrets.md) | multi | security | Secret material extraction (Phase 1 security audit) |
| [security.sql-injection](detail/security.sql-injection.md) | multi | security | SQL injection heuristic (f-string / .format() / % interpolation into SQL) |
