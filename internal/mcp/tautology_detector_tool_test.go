package mcp

// tautology_detector_tool_test.go — end-to-end test for the #4893
// archigraph_contract_test_effectiveness tool. Runs the REAL handler against a
// small Jest/NestJS-shaped spec fixture in testdata/tautology_4893/ that holds
// one tautological `it(...)` (self-compare / constant-true / same-literal) and
// one real `it(...)`. The tautological spec must be flagged ineffective; the
// real one must NOT be.

import (
	"path/filepath"
	"testing"

	"github.com/cajasmota/archigraph/internal/graph"
)

func tautologyTestServer(t *testing.T) *Server {
	t.Helper()
	doc := &graph.Document{
		Repo: "orders-v3",
		Entities: []graph.Entity{
			{
				ID: "spec_bad", Name: "tautological",
				Kind:          "SCOPE.Operation",
				QualifiedName: "OrdersController.tautological",
				Language:      "typescript",
				SourceFile:    "orders.spec.ts", StartLine: 2, EndLine: 11,
			},
			{
				ID: "spec_good", Name: "real_assertion",
				Kind:          "SCOPE.Operation",
				QualifiedName: "OrdersController.real_assertion",
				Language:      "typescript",
				SourceFile:    "orders.spec.ts", StartLine: 13, EndLine: 17,
			},
		},
	}
	srv := newTestServer(t, doc)
	abs, err := filepath.Abs(filepath.Join("testdata", "tautology_4893"))
	if err != nil {
		t.Fatalf("abs testdata: %v", err)
	}
	srv.State.mu.Lock()
	srv.State.groups["test"].Repos["orders-v3"].Path = abs
	srv.State.mu.Unlock()
	return srv
}

func specByID(specs []any, id string) map[string]any {
	for _, s := range specs {
		m, ok := s.(map[string]any)
		if !ok {
			continue
		}
		if m["entity_id"] == id {
			return m
		}
	}
	return nil
}

func reasonSet(spec map[string]any) map[string]bool {
	out := map[string]bool{}
	fs, _ := spec["findings"].([]any)
	for _, f := range fs {
		m, _ := f.(map[string]any)
		if r, ok := m["reason"].(string); ok {
			out[r] = true
		}
	}
	return out
}

func TestContractTestEffectiveness_FlagsTautological(t *testing.T) {
	srv := tautologyTestServer(t)
	// only_ineffective default (true): only the tautological spec should appear.
	out := callToolArgs(t, srv.handleContractTestEffectiveness, map[string]any{
		"group": "test",
	})

	specs, ok := out["specs"].([]any)
	if !ok {
		t.Fatalf("no specs array: %v", out)
	}
	bad := specByID(specs, "orders-v3::spec_bad")
	if bad == nil {
		t.Fatalf("tautological spec not flagged; specs=%v", specs)
	}
	if bad["verdict"] != "ineffective" {
		t.Errorf("tautological verdict = %v, want ineffective", bad["verdict"])
	}
	rs := reasonSet(bad)
	for _, want := range []string{"self_compare", "constant_true", "same_literal"} {
		if !rs[want] {
			t.Errorf("missing reason %q; got %v", want, rs)
		}
	}

	// The real spec must NOT appear when only_ineffective is on.
	if specByID(specs, "orders-v3::spec_good") != nil {
		t.Errorf("real spec wrongly flagged ineffective")
	}

	if ic, _ := out["ineffective_specs"].(float64); ic != 1 {
		t.Errorf("ineffective_specs = %v, want 1", out["ineffective_specs"])
	}
	if an, _ := out["analysed_specs"].(float64); an != 2 {
		t.Errorf("analysed_specs = %v, want 2", out["analysed_specs"])
	}
}

func TestContractTestEffectiveness_RealSpecNotFlagged(t *testing.T) {
	srv := tautologyTestServer(t)
	// only_ineffective=false: both specs returned; real one must be effective.
	out := callToolArgs(t, srv.handleContractTestEffectiveness, map[string]any{
		"group":            "test",
		"only_ineffective": "false",
	})
	specs, _ := out["specs"].([]any)
	good := specByID(specs, "orders-v3::spec_good")
	if good == nil {
		t.Fatalf("real spec missing when only_ineffective=false; specs=%v", specs)
	}
	if good["verdict"] != "effective" {
		t.Errorf("real spec verdict = %v, want effective", good["verdict"])
	}
	if fs, _ := good["findings"].([]any); len(fs) != 0 {
		t.Errorf("real spec has findings: %v", fs)
	}
}
