package php

import (
	"context"
	"testing"

	"github.com/cajasmota/archigraph/internal/extractor"
	"github.com/cajasmota/archigraph/internal/types"
)

func phpDIEdges(t *testing.T, path, src string) []types.RelationshipRecord {
	t.Helper()
	ext := &phpDIExtractor{}
	ents, err := ext.Extract(context.Background(), extractor.FileInput{
		Path: path, Content: []byte(src), Language: "php",
	})
	if err != nil {
		t.Fatalf("extract: %v", err)
	}
	var rels []types.RelationshipRecord
	for _, e := range ents {
		rels = append(rels, e.Relationships...)
	}
	return rels
}

func phpHasEdge(rels []types.RelationshipRecord, from, to, kind string) bool {
	for _, r := range rels {
		if r.FromID == from && r.ToID == to && r.Kind == kind {
			return true
		}
	}
	return false
}

// Laravel: app->bind(PaymentInterface::class, StripePayment::class) →
// BINDS(PaymentInterface → StripePayment).
func TestPhpDI_LaravelBind(t *testing.T) {
	src := `<?php
class AppServiceProvider {
    public function register() {
        $this->app->bind(PaymentInterface::class, StripePayment::class);
        $this->app->singleton(\App\Cache\CacheInterface::class, \App\Cache\RedisCache::class);
    }
}`
	rels := phpDIEdges(t, "Provider.php", src)
	if !phpHasEdge(rels, "PaymentInterface", "StripePayment", string(types.RelationshipKindBinds)) {
		t.Fatalf("expected BINDS(PaymentInterface -> StripePayment); got %+v", rels)
	}
	if !phpHasEdge(rels, "CacheInterface", "RedisCache", string(types.RelationshipKindBinds)) {
		t.Fatalf("expected BINDS(CacheInterface -> RedisCache); got %+v", rels)
	}
}

// Laravel/Symfony: constructor type-hint → INJECTED_INTO(type → class).
func TestPhpDI_ConstructorInjection(t *testing.T) {
	src := `<?php
class OrderController {
    public function __construct(private PaymentInterface $payment, LoggerInterface $log) {}
}`
	rels := phpDIEdges(t, "OrderController.php", src)
	if !phpHasEdge(rels, "PaymentInterface", "OrderController", string(types.RelationshipKindInjectedInto)) {
		t.Fatalf("expected INJECTED_INTO(PaymentInterface -> OrderController); got %+v", rels)
	}
	if !phpHasEdge(rels, "LoggerInterface", "OrderController", string(types.RelationshipKindInjectedInto)) {
		t.Fatalf("expected INJECTED_INTO(LoggerInterface -> OrderController); got %+v", rels)
	}
}

// Negative: scalar type-hints produce no edge.
func TestPhpDI_ScalarParamNoEdge(t *testing.T) {
	src := `<?php
class Config {
    public function __construct(string $name, int $count, array $opts) {}
}`
	rels := phpDIEdges(t, "Config.php", src)
	for _, r := range rels {
		if r.Kind == string(types.RelationshipKindInjectedInto) {
			t.Fatalf("expected no INJECTED_INTO for scalar params; got %+v", r)
		}
	}
}

// Negative: closure-form Laravel binding has no resolvable impl → no BINDS.
func TestPhpDI_LaravelClosureBindNoEdge(t *testing.T) {
	src := `<?php
$this->app->bind(PaymentInterface::class, function ($app) { return new StripePayment(); });`
	rels := phpDIEdges(t, "Provider.php", src)
	for _, r := range rels {
		if r.Kind == string(types.RelationshipKindBinds) {
			t.Fatalf("expected no BINDS for closure binding; got %+v", r)
		}
	}
}

// Symfony services.yaml: alias `Foo: '@Bar'` → BINDS(Foo → Bar).
func TestPhpDI_SymfonyServicesYAML(t *testing.T) {
	src := `services:
  App\Service\TransportInterface: '@App\Service\SmtpTransport'
  App\Repository\UserRepository:
    autowire: true
`
	rels := phpDIEdges(t, "config/services.yaml", src)
	if !phpHasEdge(rels, "TransportInterface", "SmtpTransport", string(types.RelationshipKindBinds)) {
		t.Fatalf("expected BINDS(TransportInterface -> SmtpTransport); got %+v", rels)
	}
}
