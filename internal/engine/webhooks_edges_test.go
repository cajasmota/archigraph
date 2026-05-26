// Tests for the webhook endpoint detection pass (#728).
//
// Each test verifies that applyWebhookEdges correctly identifies webhook
// endpoints and emits the appropriate entities + SUBSCRIBES_TO edges.
// Tests call applyWebhookEdges directly for speed, matching the pattern
// established by kafka_edges_test.go.
package engine

import (
	"testing"

	"github.com/cajasmota/archigraph/internal/types"
)

// runWebhookDetect is a lightweight in-process driver.
func runWebhookDetect(t *testing.T, lang, path, src string) ([]types.EntityRecord, []types.RelationshipRecord) {
	t.Helper()
	res := applyWebhookEdges(DetectorPassArgs{Lang: lang, Path: path, Content: []byte(src)})
	return res.Entities, res.Relationships
}

func webhookEntities(ents []types.EntityRecord) []types.EntityRecord {
	var out []types.EntityRecord
	for _, e := range ents {
		if e.Properties["is_webhook"] == "true" {
			out = append(out, e)
		}
	}
	return out
}

func externalEntities(ents []types.EntityRecord) []types.EntityRecord {
	var out []types.EntityRecord
	for _, e := range ents {
		if e.Kind == webhookExternalKind {
			out = append(out, e)
		}
	}
	return out
}

func subscribesToEdges(rels []types.RelationshipRecord) []types.RelationshipRecord {
	var out []types.RelationshipRecord
	for _, r := range rels {
		if r.Kind == subscribesToEdgeKind {
			out = append(out, r)
		}
	}
	return out
}

// ---------------------------------------------------------------------------
// Python — Stripe webhook
// ---------------------------------------------------------------------------

func TestWebhook_PyStripe_ConstructEvent(t *testing.T) {
	src := `import stripe
from flask import Flask, request

app = Flask(__name__)

@app.post('/stripe/webhook')
def stripe_webhook():
    payload = request.data
    sig_header = request.headers.get('Stripe-Signature')
    event = stripe.Webhook.construct_event(payload, sig_header, 'whsec_xxx')
    return '', 200
`
	ents, rels := runWebhookDetect(t, "python", "webhooks.py", src)
	hooks := webhookEntities(ents)
	if len(hooks) == 0 {
		t.Fatalf("expected at least 1 webhook entity, got 0 (entities=%v)", ents)
	}
	if hooks[0].Properties["webhook_provider"] != "stripe" {
		t.Errorf("webhook_provider = %q, want stripe", hooks[0].Properties["webhook_provider"])
	}
	if hooks[0].Properties["confidence"] != "high" {
		t.Errorf("confidence = %q, want high (route + sig + import)", hooks[0].Properties["confidence"])
	}
	subs := subscribesToEdges(rels)
	if len(subs) == 0 {
		t.Fatalf("expected SUBSCRIBES_TO edge, got none")
	}
}

// ---------------------------------------------------------------------------
// Python — GitHub webhook with HMAC check
// ---------------------------------------------------------------------------

func TestWebhook_PyGitHub_HMACSignature(t *testing.T) {
	src := `import hmac, hashlib
from flask import request, abort

@app.post('/github/events')
def github_webhook():
    sig = request.headers.get('X-Hub-Signature-256')
    if not verify_signature(request.data, sig):
        abort(403)
    return 'ok'
`
	ents, rels := runWebhookDetect(t, "python", "github_hook.py", src)
	hooks := webhookEntities(ents)
	if len(hooks) == 0 {
		t.Fatalf("expected at least 1 webhook entity for GitHub, got 0")
	}
	if hooks[0].Properties["webhook_provider"] != "github" {
		t.Errorf("webhook_provider = %q, want github", hooks[0].Properties["webhook_provider"])
	}
	_ = rels
}

// ---------------------------------------------------------------------------
// Node — Stripe webhook
// ---------------------------------------------------------------------------

func TestWebhook_NodeStripe_ConstructEvent(t *testing.T) {
	src := `const stripe = require('stripe')('sk_test_...');
const express = require('express');

app.post('/stripe/webhook', express.raw({type: 'application/json'}), (req, res) => {
  const sig = req.headers['stripe-signature'];
  const event = stripe.webhooks.constructEvent(req.body, sig, process.env.STRIPE_WEBHOOK_SECRET);
  res.json({ received: true });
});
`
	ents, rels := runWebhookDetect(t, "javascript", "routes/stripe.js", src)
	hooks := webhookEntities(ents)
	if len(hooks) == 0 {
		t.Fatalf("expected at least 1 Node/Stripe webhook entity, got 0")
	}
	if hooks[0].Properties["webhook_provider"] != "stripe" {
		t.Errorf("webhook_provider = %q, want stripe", hooks[0].Properties["webhook_provider"])
	}
	if hooks[0].Properties["confidence"] == "low" {
		t.Errorf("expected medium or high confidence for Stripe with raw body check, got low")
	}
	subs := subscribesToEdges(rels)
	if len(subs) == 0 {
		t.Fatalf("expected SUBSCRIBES_TO edge for Node Stripe webhook, got none")
	}
}

// ---------------------------------------------------------------------------
// Node — Slack webhook (Bolt SDK)
// ---------------------------------------------------------------------------

func TestWebhook_NodeSlack_BoltVerifier(t *testing.T) {
	src := `const { App } = require('@slack/bolt');

const slackApp = new App({
  signingSecret: process.env.SLACK_SIGNING_SECRET,
  token: process.env.SLACK_BOT_TOKEN,
});

app.post('/slack/events', (req, res) => {
  slackApp.receiver.verifyRequestSignature(req);
  res.send('OK');
});
`
	ents, rels := runWebhookDetect(t, "javascript", "slack_handler.js", src)
	hooks := webhookEntities(ents)
	if len(hooks) == 0 {
		t.Fatalf("expected at least 1 Slack webhook entity, got 0")
	}
	if hooks[0].Properties["webhook_provider"] != "slack" {
		t.Errorf("webhook_provider = %q, want slack", hooks[0].Properties["webhook_provider"])
	}
	_ = rels
}

// ---------------------------------------------------------------------------
// Java — Stripe webhook via Spring @PostMapping
// ---------------------------------------------------------------------------

func TestWebhook_JavaStripe_SpringRoute(t *testing.T) {
	src := `package com.example.webhooks;

import com.stripe.net.Webhook;
import org.springframework.web.bind.annotation.*;

@RestController
public class StripeWebhookController {

    @PostMapping("/stripe/webhook")
    public ResponseEntity<String> handleWebhook(
            @RequestBody String payload,
            @RequestHeader("Stripe-Signature") String sigHeader) {
        Event event = Event.constructFrom(payload, sigHeader);
        return ResponseEntity.ok("received");
    }
}
`
	ents, rels := runWebhookDetect(t, "java", "StripeWebhookController.java", src)
	hooks := webhookEntities(ents)
	if len(hooks) == 0 {
		t.Fatalf("expected at least 1 Java Stripe webhook entity, got 0")
	}
	if hooks[0].Properties["webhook_provider"] != "stripe" {
		t.Errorf("webhook_provider = %q, want stripe", hooks[0].Properties["webhook_provider"])
	}
	_ = rels
}

// ---------------------------------------------------------------------------
// Go — GitHub webhook with HMAC check
// ---------------------------------------------------------------------------

func TestWebhook_GoGitHub_WebhookHandler(t *testing.T) {
	src := `package main

import (
    "github.com/google/go-github/github"
    "net/http"
)

func main() {
    http.HandleFunc("/github/webhook", handleGitHub)
}

func handleGitHub(w http.ResponseWriter, r *http.Request) {
    payload, _ := github.ValidatePayload(r, []byte(secret))
    event, _ := github.ParseWebHook(github.WebHookType(r), payload)
    w.WriteHeader(200)
}
`
	ents, rels := runWebhookDetect(t, "go", "main.go", src)
	hooks := webhookEntities(ents)
	if len(hooks) == 0 {
		t.Fatalf("expected at least 1 Go GitHub webhook entity, got 0")
	}
	subs := subscribesToEdges(rels)
	if len(subs) == 0 {
		t.Fatalf("expected SUBSCRIBES_TO edge for Go GitHub webhook, got none")
	}
}

// ---------------------------------------------------------------------------
// Confidence: route name only → low
// ---------------------------------------------------------------------------

func TestWebhook_Confidence_LowForNameOnly(t *testing.T) {
	src := `from flask import Flask, request

app = Flask(__name__)

@app.post('/webhook')
def generic_webhook():
    data = request.json
    return 'ok'
`
	ents, _ := runWebhookDetect(t, "python", "handler.py", src)
	hooks := webhookEntities(ents)
	if len(hooks) == 0 {
		t.Fatalf("expected at least 1 generic webhook entity, got 0")
	}
	if hooks[0].Properties["confidence"] != "low" {
		t.Errorf("confidence = %q, want low (only route name matches)", hooks[0].Properties["confidence"])
	}
}

// ---------------------------------------------------------------------------
// External entity dedup: same provider called twice → one SCOPE.External
// ---------------------------------------------------------------------------

func TestWebhook_ExternalEntityDedup(t *testing.T) {
	src := `import stripe

@app.post('/stripe/webhook')
def stripe_wh():
    stripe.Webhook.construct_event(request.data, sig, secret)

@app.post('/stripe/webhook2')
def stripe_wh2():
    stripe.Webhook.construct_event(request.data, sig, secret)
`
	ents, _ := runWebhookDetect(t, "python", "multi.py", src)
	ext := externalEntities(ents)
	stripeExt := 0
	for _, e := range ext {
		if e.Properties["provider"] == "stripe" {
			stripeExt++
		}
	}
	if stripeExt != 1 {
		t.Errorf("expected exactly 1 SCOPE.External stripe entity, got %d", stripeExt)
	}
}

// ---------------------------------------------------------------------------
// Non-match: file with no webhook content emits nothing
// ---------------------------------------------------------------------------

func TestWebhook_NoMatch(t *testing.T) {
	src := `package main

import "fmt"

func main() {
    fmt.Println("hello")
}
`
	ents, rels := runWebhookDetect(t, "go", "main.go", src)
	if len(ents) != 0 || len(rels) != 0 {
		t.Errorf("expected no entities/rels for unrelated file, got %d/%d", len(ents), len(rels))
	}
}
