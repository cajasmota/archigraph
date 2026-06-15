package mcp

// effects_branches_4666_brace_test.go — method-boundary scoping regression for
// the BRACE-language branch analyzers (#4666, epic #4419).
//
// #4533 fixed the over-capture for Python only (bodyEndPython). But every
// brace-language analyzer (jsts/java/go/php/csharp/kotlin/scala/rust) walks its
// WHOLE input window — so when the effects tool pads StartLine+N because the
// entity's EndLine is missing/zero (#4488), a TS/NestJS controller method's
// branch list bled into the sibling @Put method that follows it, exactly as the
// Python case did. ClampToFunctionBody now clamps the padded window to the
// target method's own `{`…`}` body for all brace languages.
//
// This test pins that fix on a verbatim two-method NestJS-style controller
// (createContact immediately followed by updateContact): with createContact
// given NO EndLine, effects(createContact, include=branches) must return ONLY
// createContact's own branches (400/409/500 + its 404) and NONE of
// updateContact's distinct branches (its own 404 'contact does not exist', the
// 422 email conflict, the 503).

import (
	"path/filepath"
	"testing"

	"github.com/cajasmota/grafel/internal/graph"
)

func braceBranchScopeTestServer(t *testing.T) *Server {
	t.Helper()
	doc := &graph.Document{
		Repo: "upvate-core",
		Entities: []graph.Entity{
			{
				ID: "op_create_contact_ts", Name: "ContractsController.createContact",
				Kind:          "SCOPE.Operation",
				QualifiedName: "contracts.controller.ContractsController.createContact",
				// StartLine is the @Post decorator (fixture line 1). EndLine omitted
				// (0) → degenerate span → padded window → would overshoot into
				// updateContact (fixture line 23+) pre-fix.
				SourceFile: "contracts_controller_two_methods.ts", StartLine: 1, EndLine: 0,
			},
		},
	}
	srv := newTestServer(t, doc)
	abs, err := filepath.Abs(filepath.Join("testdata", "branches_4666_brace"))
	if err != nil {
		t.Fatalf("abs testdata: %v", err)
	}
	srv.State.mu.Lock()
	srv.State.groups["test"].Repos["upvate-core"].Path = abs
	srv.State.mu.Unlock()
	return srv
}

func TestEffectsBranches_BraceMethodScope(t *testing.T) {
	srv := braceBranchScopeTestServer(t)
	out := callEffects(t, srv, "ContractsController.createContact", "branches")

	if sup, _ := out["branches_supported"].(bool); !sup {
		t.Fatalf("branches_supported should be true for jsts; got %v", out["branches_supported"])
	}
	branches := branchList(t, out)
	if len(branches) == 0 {
		t.Fatalf("expected createContact's own branches, got none")
	}

	// --- createContact's OWN branches are present ---
	if b := findBranch(branches, "contract.client === null"); b == nil {
		t.Fatalf("missing createContact's 400 guard: %v", branches)
	} else {
		assertStatus(t, b, "400")
	}
	if b := findBranch(branches, "existing !== null"); b == nil {
		t.Fatalf("missing createContact's 409 guard: %v", branches)
	} else {
		assertStatus(t, b, "409")
	}

	// --- updateContact's DISTINCT branches must NOT appear (the over-capture) ---
	for _, b := range branches {
		cond, _ := b["condition"].(string)
		if cond == "if (conflict !== null && conflict.id !== contactId)" {
			t.Errorf("over-capture: createContact branch list bled into updateContact's 422 conflict guard (%q)", cond)
		}
		if st := branchStatus(b); st == "422" || st == "503" {
			t.Errorf("over-capture: createContact branch carries updateContact-only status %s (cond=%v)", st, cond)
		}
	}

	// No branch may be reported at/after updateContact's region (fixture line 23).
	for _, b := range branches {
		if line := toInt(b["line"]); line >= 23 {
			t.Errorf("over-capture: branch at line %d is inside/after the sibling updateContact method (cond=%v)", line, b["condition"])
		}
	}
}

// branchStatus pulls the returns.status off a branch facet, or "".
func branchStatus(b map[string]any) string {
	r, ok := b["returns"].(map[string]any)
	if !ok {
		return ""
	}
	s, _ := r["status"].(string)
	return s
}
