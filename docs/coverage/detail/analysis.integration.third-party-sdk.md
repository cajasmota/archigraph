<!-- DO NOT EDIT — generated from docs/coverage/registry.json by 'go run ./tools/coverage gen' -->
# `analysis.integration.third-party-sdk` — Third-party SDK service dependencies (DEPENDS_ON_SERVICE)

Auto-generated. Back to [summary](../summary.md).

- **Language:** [multi](../by-language/multi.md)
- **Category:** [platform](../by-category/platform.md)
- **Subcategory:** App Topology & Integration
- **Capability cells:** 1

## Capabilities

| Capability | Status | Verified at | Issue | Cites | Notes |
|------------|--------|-------------|-------|-------|-------|
| External service dependency | 🟢 `partial` | `2026-06-02` | 3628 | `internal/extractor/external_service.go`<br>`internal/extractor/external_service_test.go`<br>`internal/extractors/javascript/external_service.go`<br>`internal/extractors/javascript/external_service_saas_5502_test.go`<br>`internal/extractors/javascript/external_service_test.go`<br>`internal/extractors/python/external_service.go`<br>`internal/extractors/python/external_service_test.go`<br>`internal/types/kinds.go` | #3628 area: SDK-LEVEL named third-party integration detection, distinct from raw HTTP-client CONSUMES_API (path-level). Each language extractor emits a DEPENDS_ON_SERVICE edge from the calling function/method to a synthetic SCOPE.ExternalService node (Name "service:<name>") so every call site of a service converges on ONE node and the graph answers "what third-party services does this codebase integrate with, and where?". The shared dictionary + node/edge builders live in internal/extractor/external_service.go (ServiceForImportSource, AWSServiceFromArg, ExternalServiceEntity, ExternalServiceTargetID, EmitServiceDependencyEdges). Service dictionary covers stripe, twilio, sendgrid, openai, slack, sentry, firebase, algolia, and the AWS family (aws-s3/ses/sns/sqs/dynamodb/lambda/cognito/... via boto3 client/resource literal arg or @aws-sdk v3 client class name; cognito folds boto3 cognito-idp/cognito-identity and the aws-sdk v3 CognitoIdentityProvider/CognitoIdentity client classes into one aws-cognito node). Python pass (internal/extractors/python/external_service.go): import-gated attribute-chain + from-import constructor detection; AWS service resolved from the boto3.client/resource string literal. JS/TS pass (internal/extractors/javascript/external_service.go): import-gated `new Stripe()` + local-var-back-reference (stripe.charges.create), default-import receiver (sgMail.send), namespace init (Sentry.init), @aws-sdk client-class -> aws-<svc>. Optional `operation` edge property records the SDK call. PRECISION-FIRST / honest-partial: a dynamic boto3 service string -> aws-generic; an unrecognised SDK or a bare .create()/.send() on a non-imported object emits NO edge. PARTIAL because only python + javascript/typescript lanes are implemented (java/go/ruby/php and the long tail of SDKs are future lanes), and recall within a lane is deliberately bounded to import-rooted shapes. DEPLOY-DEFERRED: extractor + kinds land here; live-daemon reindex is a separate coordinated step. #5502 SaaS-SDK allow-list uplift: ServiceForImportSource now resolves the common TS SaaS SDKs through ONE shared table (no per-SDK detector sprawl) — Stripe, Plaid (plaid), Knock (@knocklabs/node), Postmark, Slack (@slack/web-api), PostHog (posthog-node), Linear (@linear/sdk), HubSpot (@hubspot/api-client), Intercom, AWS (@aws-sdk/*), UploadThing, Contentful, Mapbox (@mapbox/mapbox-sdk), Cal.com (@calcom/*), Vercel AI SDK (ai, @ai-sdk/*) and OpenFeature (@openfeature/*). The existing import-gated call-shape pass (new Ctor() + receiver, default-import receiver call) and the converged SCOPE.ExternalService node are unchanged; the recorded operation now prefers the real SDK call (stripe.charges.create -> charges.create) over the new <Ctor> construction placeholder (EmitServiceDependencyEdges/isConstructorOp/findServiceRel). Import gate preserved: a non-imported stripe-named local emits no edge. |

## Provenance

This record is sourced from `docs/coverage/registry.json`. To update it, edit the JSON
(or use `go run ./tools/coverage update analysis.integration.third-party-sdk ...`) then regenerate:

```
go run ./tools/coverage validate
go run ./tools/coverage gen
```
