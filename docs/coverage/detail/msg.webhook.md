<!-- DO NOT EDIT — generated from docs/coverage/registry.json by 'go run ./tools/coverage gen' -->
# `msg.webhook` — Webhooks

Auto-generated. Back to [summary](../summary.md).

- **Language:** [multi](../by-language/multi.md)
- **Category:** [message_broker](../by-category/message_broker.md)
- **Capability cells:** 4

## Capabilities

| Capability | Status | Verified at | Issue | Cites | Notes |
|------------|--------|-------------|-------|-------|-------|
| Consumer extraction | ✅ `full` | `2026-05-28` | — | `internal/engine/webhooks_edges.go` | — |
| Producer extraction | ✅ `full` | `2026-05-28` | — | `internal/engine/webhooks_edges.go` | — |
| Signature verification | ✅ `full` | — | 3628 | `internal/engine/webhooks_edges.go`<br>`internal/engine/webhooks_edges_test.go` | [security] webhook-receiver signature-verification posture (#3628 child): every detected webhook endpoint carries webhook=true + webhook_provider + webhook_signature_verified=true|false so the graph answers 'which endpoints are webhook receivers, and do they verify signatures?'. webhookSignatureVerified asserts true ONLY on a concrete signal — a provider-SDK verify call (Stripe construct_event/constructEvent/ConstructEvent/Event.constructFrom, Twilio RequestValidator/validateRequest, Slack SignatureVerifier/verifyRequestSignature, GitHub github.ValidatePayload, Svix wh.verify) OR a *-Signature/*-Hmac request-header read paired with a generic HMAC compute/compare (hmac.new/compare_digest, crypto.createHmac+timingSafeEqual, Mac.getInstance/HmacUtils, hmac.New/hmac.Equal, OpenSSL::HMAC/secure_compare, verify_signature(...)). Providers: stripe/github/twilio/slack/shopify(new: X-Shopify-Hmac-Sha256)/mailgun/svix/generic. Honest-partial: an unverified-but-webhook route (path contains /webhook, no verification observed) is stamped webhook_signature_verified=false — the security-relevant finding (forgeable receiver). Value-asserting tests pin verified=true for Stripe/GitHub/Shopify/Twilio and verified=false for a bare /webhook body-parse handler; negatives: /users and an unrelated 'signature' mention emit no webhook. Langs: python/jsts/java+kotlin/go/ruby. DEPLOY-DEFERRED. |
| Topic attribution | 🟢 `partial` | `2026-05-28` | — | `internal/engine/webhooks_edges.go` | — |

## Provenance

This record is sourced from `docs/coverage/registry.json`. To update it, edit the JSON
(or use `go run ./tools/coverage update msg.webhook ...`) then regenerate:

```
go run ./tools/coverage validate
go run ./tools/coverage gen
```
