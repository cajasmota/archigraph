package dashboard

import (
	"strings"
	"testing"

	"github.com/cajasmota/grafel/internal/links"
)

// TestDataflowConfidenceFloorMatchesProducer asserts the dashboard's surfaced
// confidence floor stays in lock-step with the taint pass that produces the
// sidecar. The dashboard keeps a local const (to avoid importing the producer's
// enum surface in the request path) — this test is the coupling guard.
func TestDataflowConfidenceFloorMatchesProducer(t *testing.T) {
	if dataflowConfidenceFloor != links.TaintFindingFloor() {
		t.Fatalf("dataflowConfidenceFloor = %v, want links.TaintFindingFloor() = %v",
			dataflowConfidenceFloor, links.TaintFindingFloor())
	}
}

func TestDfEndpointTail(t *testing.T) {
	cases := []struct {
		in, want string
	}{
		{"upvate-core::SCOPE.Function:handleLogin", "handleLogin"},
		{"repo::Kind:Name", "Name"},
		{"sink:raw_sql_exec", "raw_sql_exec"},
		{"bare", "bare"},
		{"a/b/c", "c"},
		{"", ""},
	}
	for _, c := range cases {
		if got := dfEndpointTail(c.in); got != c.want {
			t.Errorf("dfEndpointTail(%q) = %q, want %q", c.in, got, c.want)
		}
	}
}

func TestDfEntityIndexResolve(t *testing.T) {
	ix := &dfEntityIndex{
		bySuffix:  map[string]dfResolvedEntity{},
		ambiguous: map[string]bool{},
	}
	ix.add("api", dfResolvedEntity{id: "Func:login", name: "login", kind: "SCOPE.Function", sourceFile: "auth.go", line: 12})
	// Duplicate suffix across repos ⇒ ambiguous, never resolved.
	ix.add("api", dfResolvedEntity{id: "Func:dup", name: "a"})
	ix.add("web", dfResolvedEntity{id: "Func:dup", name: "b"})

	// Resolved endpoint: name/file/line come from the index; primitive passed through.
	got := ix.resolve("api::Func:login", "req.body", 0)
	if got.Name != "login" || got.Repo != "api" || got.SourceFile != "auth.go" || got.Line != 12 {
		t.Fatalf("resolve(known) = %+v", got)
	}
	if got.EntityID != "api/Func:login" {
		t.Fatalf("resolve(known).EntityID = %q, want api/Func:login", got.EntityID)
	}
	if got.Primitive != "req.body" {
		t.Fatalf("resolve(known).Primitive = %q, want req.body", got.Primitive)
	}

	// fallbackLine used when the index has no line.
	ix.add("api", dfResolvedEntity{id: "Func:noline", name: "noline"})
	if got := ix.resolve("api::Func:noline", "", 99); got.Line != 99 {
		t.Fatalf("resolve fallbackLine = %d, want 99", got.Line)
	}

	// Ambiguous suffix ⇒ falls back to raw tail label, raw key as entity_id.
	amb := ix.resolve("api::Func:dup", "", 0)
	if amb.Name != "dup" || amb.EntityID != "api::Func:dup" {
		t.Fatalf("resolve(ambiguous) = %+v, want raw fallback", amb)
	}

	// Unknown suffix ⇒ raw fallback.
	unk := ix.resolve("api::Func:missing", "", 0)
	if unk.Name != "missing" || unk.EntityID != "api::Func:missing" {
		t.Fatalf("resolve(unknown) = %+v, want raw fallback", unk)
	}
}

func TestDfFindingExplanation(t *testing.T) {
	src := DataflowEndpoint{Name: "handleSearch"}
	sink := DataflowEndpoint{Name: "rawQuery"}

	intra := dfFindingExplanation(taintFindingSidecar{
		Category: "sql_injection", SourcePrimitive: "req.query", SourceLine: 4,
		SinkPrimitive: "db.exec", SinkLine: 9, Confidence: 0.91,
	}, src, sink, 0)
	if intra == "" || !strings.Contains(intra, "SQL injection") || !strings.Contains(intra, "same function") {
		t.Fatalf("intra explanation unexpected: %q", intra)
	}

	inter := dfFindingExplanation(taintFindingSidecar{
		Category: "command_injection", SourcePrimitive: "req.body", SourceLine: 4,
		SinkPrimitive: "exec", SinkLine: 40, Confidence: 0.72,
	}, src, sink, 2)
	if !strings.Contains(inter, "2 call hop(s)") {
		t.Fatalf("inter explanation should mention hops: %q", inter)
	}
}
