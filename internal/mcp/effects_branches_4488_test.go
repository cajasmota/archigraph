package mcp

// effects_branches_4488_test.go — method-boundary scoping regression test for
// the #4423 `branches` facet (#4488, epic #4419).
//
// Wave-2 downstream feedback (mcp-wave2-test-feedback §3) reported that
// effects(create_contact, include=branches) OVER-CAPTURED: the branch list bled
// into the adjacent `@action` methods (update_contact and beyond), because the
// entity's EndLine was missing/zero and branchSourceSpan padded a generous tail
// window that overshot into sibling defs — and the analyzer walked the whole
// padded window rather than the single method's own body.
//
// This test pins the fix: it uses a byte-copy of the REAL ContractViewSet
// region (create_contact immediately followed by update_contact, copied
// verbatim into testdata/branches_4488/) and asserts that
// effects(create_contact, include=branches) returns ONLY create_contact's own
// branches (400/409/200/201/500) and NONE of update_contact's distinct branches
// (the 404 `User.DoesNotExist` handler, update_contact's own 409 conflict on
// `conflict_user is not None`). The entity is given StartLine with NO EndLine to
// reproduce the exact degenerate-span condition that triggered the bug.

import (
	"path/filepath"
	"testing"

	"github.com/cajasmota/grafel/internal/graph"
)

// branchScopeTestServer builds a server whose repo Path points at the
// branches_4488 testdata dir. The create_contact entity deliberately has NO
// EndLine (EndLine: 0) so the source window is padded — exactly the condition
// that produced the over-capture in the field.
func branchScopeTestServer(t *testing.T) *Server {
	t.Helper()
	doc := &graph.Document{
		Repo: "acme-core",
		Entities: []graph.Entity{
			{
				ID: "op_create_contact", Name: "ContractViewSet.create_contact",
				Kind:          "SCOPE.Operation",
				QualifiedName: "core.views.contract_viewset.ContractViewSet.create_contact",
				// StartLine is the @action decorator line (fixture line 1); the
				// `def` is the next line. EndLine omitted (0) → degenerate span →
				// padded window → would overshoot into update_contact pre-fix.
				SourceFile: "contract_viewset_two_methods.py", StartLine: 1, EndLine: 0,
			},
		},
	}
	srv := newTestServer(t, doc)
	abs, err := filepath.Abs(filepath.Join("testdata", "branches_4488"))
	if err != nil {
		t.Fatalf("abs testdata: %v", err)
	}
	srv.State.mu.Lock()
	srv.State.groups["test"].Repos["acme-core"].Path = abs
	srv.State.mu.Unlock()
	return srv
}

// TestEffectsBranches_MethodScope is the #4488 regression: create_contact's
// branch list must contain ONLY its own branches and must NOT bleed into the
// adjacent update_contact method.
func TestEffectsBranches_MethodScope(t *testing.T) {
	srv := branchScopeTestServer(t)
	out := callEffects(t, srv, "ContractViewSet.create_contact", "branches")

	if sup, _ := out["branches_supported"].(bool); !sup {
		t.Fatalf("branches_supported should be true for python; got %v", out["branches_supported"])
	}
	branches := branchList(t, out)

	// --- create_contact's OWN branches are present ---
	if b := findBranch(branches, "client is None"); b == nil {
		t.Fatalf("missing create_contact's 400 `client is None` guard: %v", branches)
	} else {
		assertStatus(t, b, "400")
	}
	if b := findBranch(branches, "available"); b == nil {
		t.Fatalf("missing create_contact's 409 availability guard: %v", branches)
	} else {
		assertStatus(t, b, "409")
	}

	// --- update_contact's DISTINCT branches must NOT appear (the over-capture) ---
	// The 404 handler `except User.DoesNotExist` belongs solely to
	// update_contact. Its presence is the canonical over-capture signal.
	for _, b := range branches {
		cond, _ := b["condition"].(string)
		if cond == "except User.DoesNotExist" {
			t.Errorf("over-capture: create_contact branch list bled into update_contact's 404 handler (%q)", cond)
		}
		if cond == "if conflict_user is not None" {
			t.Errorf("over-capture: create_contact branch list bled into update_contact's conflict guard (%q)", cond)
		}
	}

	// No branch may be reported below update_contact's `def` (fixture line 73).
	// The padded window covers all 145 lines; a correct scope stops before line
	// 72 (the @action decorator preceding update_contact).
	for _, b := range branches {
		if line := toInt(b["line"]); line >= 72 {
			t.Errorf("over-capture: branch at line %d is inside/after the sibling update_contact method (cond=%v)", line, b["condition"])
		}
	}

	// create_contact has exactly its 400, 409, 200, 500 control-altering
	// branches (the 201 is a fall-through bare return — not a branch). All
	// reported lines must be within the method body (< 72).
	if len(branches) == 0 {
		t.Fatalf("expected create_contact's own branches, got none")
	}
}
