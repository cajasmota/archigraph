<!-- DO NOT EDIT — generated from docs/coverage.json by 'go run ./tools/coverage gen' -->
# Coverage — language: `multi`

Auto-generated. Back to [summary](../summary.md).

- Records: **50**
- Full: **76** · Partial: **7** · Missing: **17** · N/A: **0**

## Records

| ID | Category | Label | Capabilities |
|----|----------|-------|--------------|
| [build.bazel](../detail/build.bazel.md) | [build_system](../by-category/build_system.md) | Bazel / BUCK / WORKSPACE | dependency_graph=full, target_extraction=full |
| [build.dockerfile](../detail/build.dockerfile.md) | [build_system](../by-category/build_system.md) | Dockerfile | dependency_graph=full, target_extraction=full |
| [build.justfile](../detail/build.justfile.md) | [build_system](../by-category/build_system.md) | Justfile | dependency_graph=full, target_extraction=full |
| [build.makefile](../detail/build.makefile.md) | [build_system](../by-category/build_system.md) | Makefile | dependency_graph=missing, target_extraction=partial |
| [config.docker-compose](../detail/config.docker-compose.md) | [configuration](../by-category/configuration.md) | docker-compose.yml | file_parsing=full |
| [config.dotenv](../detail/config.dotenv.md) | [configuration](../by-category/configuration.md) | .env (names-only — values stripped at extraction boundary) | env_resolution=full, file_parsing=full |
| [config.github-actions](../detail/config.github-actions.md) | [configuration](../by-category/configuration.md) | GitHub Actions workflows | file_parsing=full |
| [config.gitlab-ci](../detail/config.gitlab-ci.md) | [configuration](../by-category/configuration.md) | GitLab CI | file_parsing=partial |
| [config.ini](../detail/config.ini.md) | [configuration](../by-category/configuration.md) | .ini / setup.cfg / flake8 / mypy / pytest.ini | file_parsing=partial |
| [config.jenkins](../detail/config.jenkins.md) | [configuration](../by-category/configuration.md) | Jenkinsfile / Jenkins Pipeline DSL | file_parsing=missing |
| [config.toml](../detail/config.toml.md) | [configuration](../by-category/configuration.md) | .toml | file_parsing=full |
| [config.yaml](../detail/config.yaml.md) | [configuration](../by-category/configuration.md) | .yaml / .yml | file_parsing=full |
| [infra.observability.datadog](../detail/infra.observability.datadog.md) | [observability](../by-category/observability.md) | Datadog APM / StatsD | metric_extraction=missing, trace_extraction=missing |
| [infra.observability.logging-config](../detail/infra.observability.logging-config.md) | [observability](../by-category/observability.md) | Generic logging-config extractor (Python logging, Go slog, Node winston/pino, .NET NLog/Serilog, log4j/logback) | log_extraction=full |
| [infra.observability.newrelic](../detail/infra.observability.newrelic.md) | [observability](../by-category/observability.md) | New Relic | metric_extraction=missing, trace_extraction=missing |
| [infra.observability.opentelemetry](../detail/infra.observability.opentelemetry.md) | [observability](../by-category/observability.md) | OpenTelemetry instrumentation | log_extraction=missing, metric_extraction=missing, trace_extraction=missing |
| [infra.observability.prometheus](../detail/infra.observability.prometheus.md) | [observability](../by-category/observability.md) | Prometheus client libraries | metric_extraction=missing |
| [infra.observability.sentry](../detail/infra.observability.sentry.md) | [observability](../by-category/observability.md) | Sentry SDK | trace_extraction=missing |
| [infra.resource.aws-cdk](../detail/infra.resource.aws-cdk.md) | [infrastructure](../by-category/infrastructure.md) | AWS CDK | resource_extraction=full |
| [infra.resource.cloudformation](../detail/infra.resource.cloudformation.md) | [infrastructure](../by-category/infrastructure.md) | AWS CloudFormation | resource_extraction=partial |
| [infra.resource.helm](../detail/infra.resource.helm.md) | [infrastructure](../by-category/infrastructure.md) | Helm charts | resource_extraction=missing |
| [infra.resource.kubernetes](../detail/infra.resource.kubernetes.md) | [infrastructure](../by-category/infrastructure.md) | Kubernetes manifests | resource_extraction=full |
| [infra.resource.pulumi](../detail/infra.resource.pulumi.md) | [infrastructure](../by-category/infrastructure.md) | Pulumi | resource_extraction=full |
| [infra.resource.terraform](../detail/infra.resource.terraform.md) | [infrastructure](../by-category/infrastructure.md) | Terraform / OpenTofu / Vault / Nomad / Packer / Waypoint | dependency_attribution=full, resource_extraction=full |
| [msg.broker.cloudevents](../detail/msg.broker.cloudevents.md) | [message_broker](../by-category/message_broker.md) | CloudEvents | consumer_extraction=full, producer_extraction=full, topic_attribution=full |
| [msg.broker.debezium](../detail/msg.broker.debezium.md) | [message_broker](../by-category/message_broker.md) | Debezium / Kafka Connect CDC | consumer_extraction=full, producer_extraction=full, topic_attribution=partial |
| [msg.broker.eventbridge](../detail/msg.broker.eventbridge.md) | [message_broker](../by-category/message_broker.md) | AWS EventBridge | consumer_extraction=full, producer_extraction=full, topic_attribution=full |
| [msg.broker.eventgrid](../detail/msg.broker.eventgrid.md) | [message_broker](../by-category/message_broker.md) | Azure Event Grid | consumer_extraction=full, producer_extraction=full, topic_attribution=full |
| [msg.broker.gcp-pubsub](../detail/msg.broker.gcp-pubsub.md) | [message_broker](../by-category/message_broker.md) | GCP Pub/Sub | consumer_extraction=full, producer_extraction=full, topic_attribution=full |
| [msg.broker.kafka](../detail/msg.broker.kafka.md) | [message_broker](../by-category/message_broker.md) | Apache Kafka | consumer_extraction=full, producer_extraction=full, topic_attribution=full |
| [msg.broker.mqtt](../detail/msg.broker.mqtt.md) | [message_broker](../by-category/message_broker.md) | MQTT | consumer_extraction=missing, producer_extraction=missing |
| [msg.broker.nats](../detail/msg.broker.nats.md) | [message_broker](../by-category/message_broker.md) | NATS | consumer_extraction=full, producer_extraction=full, topic_attribution=full |
| [msg.broker.pulsar](../detail/msg.broker.pulsar.md) | [message_broker](../by-category/message_broker.md) | Apache Pulsar | consumer_extraction=full, producer_extraction=full, topic_attribution=full |
| [msg.broker.rabbitmq](../detail/msg.broker.rabbitmq.md) | [message_broker](../by-category/message_broker.md) | RabbitMQ | consumer_extraction=full, producer_extraction=full, topic_attribution=full |
| [msg.broker.redis](../detail/msg.broker.redis.md) | [message_broker](../by-category/message_broker.md) | Redis pub/sub + Streams | consumer_extraction=full, producer_extraction=full, topic_attribution=full |
| [msg.broker.sns](../detail/msg.broker.sns.md) | [message_broker](../by-category/message_broker.md) | AWS SNS (IaC-declared) | consumer_extraction=full, producer_extraction=full, topic_attribution=full |
| [msg.broker.sqs](../detail/msg.broker.sqs.md) | [message_broker](../by-category/message_broker.md) | AWS SQS | consumer_extraction=full, producer_extraction=full, topic_attribution=full |
| [msg.graphql-subscriptions](../detail/msg.graphql-subscriptions.md) | [message_broker](../by-category/message_broker.md) | GraphQL subscriptions | consumer_extraction=full, producer_extraction=full, topic_attribution=full |
| [msg.kafka-streams](../detail/msg.kafka-streams.md) | [message_broker](../by-category/message_broker.md) | Kafka Streams / Faust | consumer_extraction=missing, producer_extraction=missing |
| [msg.sse](../detail/msg.sse.md) | [message_broker](../by-category/message_broker.md) | Server-Sent Events | consumer_extraction=full, producer_extraction=full |
| [msg.webhook](../detail/msg.webhook.md) | [message_broker](../by-category/message_broker.md) | Webhook inbound (Stripe, GitHub, Twilio, Slack, SendGrid, Mailgun, ...) | consumer_extraction=full, producer_extraction=full |
| [msg.websocket](../detail/msg.websocket.md) | [message_broker](../by-category/message_broker.md) | WebSocket channels | consumer_extraction=full, producer_extraction=full, topic_attribution=partial |
| [protocol.graphql](../detail/protocol.graphql.md) | [protocol](../by-category/protocol.md) | GraphQL SDL (Query/Mutation/Subscription) | cross_repo_linkage=partial, method_attribution=full, service_extraction=full |
| [protocol.grpc](../detail/protocol.grpc.md) | [protocol](../by-category/protocol.md) | gRPC services | cross_repo_linkage=full, method_attribution=full, service_extraction=full |
| [protocol.openapi](../detail/protocol.openapi.md) | [protocol](../by-category/protocol.md) | OpenAPI / Swagger spec | cross_repo_linkage=full, method_attribution=full, service_extraction=full |
| [protocol.protobuf](../detail/protocol.protobuf.md) | [protocol](../by-category/protocol.md) | Protocol Buffers (.proto) | cross_repo_linkage=full, method_attribution=full, service_extraction=full |
| [security.auth-other](../detail/security.auth-other.md) | [security](../by-category/security.md) | Auth policy resolver (Python / NestJS / Go / Ruby / ASP.NET — Phases 2-4 of #1942) | auth_policy=missing |
| [security.csrf](../detail/security.csrf.md) | [security](../by-category/security.md) | CSRF heuristic detector | auth_policy=full |
| [security.secrets](../detail/security.secrets.md) | [security](../by-category/security.md) | Secret material extraction (Phase 1 security audit) | secret_detection=full |
| [security.sql-injection](../detail/security.sql-injection.md) | [security](../by-category/security.md) | SQL injection heuristic (f-string / .format() / % interpolation into SQL) | sql_injection=full |
