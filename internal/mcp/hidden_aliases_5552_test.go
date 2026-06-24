package mcp

// hidden_aliases_5552_test.go — guards for the #5546/#5552 consolidation:
// the handshake advertises exactly the 22 canonical tools, while the ~56
// absorbed legacy names stay REGISTERED and dispatchable for one release
// (code-only back-compat aliases hidden from tools/list).

import (
	"context"
	"testing"

	mcpapi "github.com/mark3labs/mcp-go/mcp"
)

// TestAdvertisedHandshakeIs22 — fullToolList() (the per-connect tools/list
// payload) advertises exactly the 22 canonical tools and zero hidden aliases.
func TestAdvertisedHandshakeIs22(t *testing.T) {
	repoDir := t.TempDir()
	srv := makeTestServer(t, map[string]map[string]string{
		"mygroup": {"myrepo": repoDir},
	})

	entries, err := srv.ListToolsForCWD(repoDir)
	if err != nil {
		t.Fatalf("ListToolsForCWD: %v", err)
	}

	got := make(map[string]bool, len(entries))
	for _, e := range entries {
		got[e.Name] = true
	}

	if len(entries) != len(canonicalToolNames) {
		t.Errorf("advertised %d tools, want %d (the canonical set): got %v",
			len(entries), len(canonicalToolNames), toolNames(entries))
	}

	// Every advertised name must be canonical; no alias may leak.
	for _, e := range entries {
		if !canonicalToolNames[e.Name] {
			t.Errorf("advertised non-canonical tool %q", e.Name)
		}
		if aliasToolNames[e.Name] {
			t.Errorf("hidden alias %q leaked into the handshake", e.Name)
		}
	}
	// Every canonical name must be advertised.
	for name := range canonicalToolNames {
		if !got[name] {
			t.Errorf("canonical tool %q missing from the advertised handshake", name)
		}
	}
}

// TestAdvertisedCount22 — explicit count guard (22 advertised).
func TestAdvertisedCount22(t *testing.T) {
	repoDir := t.TempDir()
	srv := makeTestServer(t, map[string]map[string]string{
		"mygroup": {"myrepo": repoDir},
	})
	entries, err := srv.ListToolsForCWD(repoDir)
	if err != nil {
		t.Fatalf("ListToolsForCWD: %v", err)
	}
	if len(entries) != 22 {
		t.Errorf("expected 22 advertised tools, got %d — update this guard only if the canonical set changes (#5546)", len(entries))
	}
}

// TestHiddenAliasStillDispatchable — a hidden alias (grafel_find_callers) is
// excluded from the handshake yet stays REGISTERED and callable directly, so
// un-redeployed consumers keep working for one release (back-compat).
func TestHiddenAliasStillDispatchable(t *testing.T) {
	repoDir := t.TempDir()
	srv := makeTestServer(t, map[string]map[string]string{
		"mygroup": {"myrepo": repoDir},
	})

	const alias = "grafel_find_callers"
	if !aliasToolNames[alias] {
		t.Fatalf("%q expected to be a hidden alias", alias)
	}

	// Hidden from the advertised list.
	entries, err := srv.ListToolsForCWD(repoDir)
	if err != nil {
		t.Fatalf("ListToolsForCWD: %v", err)
	}
	for _, e := range entries {
		if e.Name == alias {
			t.Fatalf("hidden alias %q must not be advertised", alias)
		}
	}

	// Still registered → still dispatchable directly.
	st, ok := srv.MCP.ListTools()[alias]
	if !ok {
		t.Fatalf("hidden alias %q must remain REGISTERED for back-compat", alias)
	}
	if st.Handler == nil {
		t.Fatalf("hidden alias %q has no handler", alias)
	}
	req := mcpapi.CallToolRequest{}
	req.Params.Name = alias
	req.Params.Arguments = map[string]any{"entity_id": "nonexistent", "group": "mygroup"}
	// The handler must resolve and return (an error result is fine — we only
	// assert the alias is wired to a live handler, not that the entity exists).
	if _, err := st.Handler(context.Background(), req); err != nil {
		// A Go-level error is acceptable here (entity-not-found may surface as
		// a CallToolResult or an error); the point is the handler ran.
		t.Logf("alias %q handler returned err (acceptable): %v", alias, err)
	}
}

// TestHiddenAliasPartition — invariant: the alias set is exactly
// registered − canonical − sentinel. Guards against a new tool being added
// without classifying it (advertised vs hidden), or an alias being misspelled.
func TestHiddenAliasPartition(t *testing.T) {
	repoDir := t.TempDir()
	srv := makeTestServer(t, map[string]map[string]string{
		"mygroup": {"myrepo": repoDir},
	})

	registered := srv.MCP.ListTools()

	// Every registered tool is exactly one of: sentinel, canonical, or alias.
	for name := range registered {
		if name == sentinelToolName {
			continue
		}
		canon := canonicalToolNames[name]
		alias := aliasToolNames[name]
		if canon && alias {
			t.Errorf("tool %q is in BOTH canonical and alias sets", name)
		}
		if !canon && !alias {
			t.Errorf("registered tool %q is neither canonical nor a hidden alias — classify it (#5546)", name)
		}
	}

	// Every canonical name must actually be registered.
	for name := range canonicalToolNames {
		if _, ok := registered[name]; !ok {
			t.Errorf("canonical tool %q is not registered", name)
		}
	}
	// Every alias name must actually be registered (else it can't be callable).
	for name := range aliasToolNames {
		if _, ok := registered[name]; !ok {
			t.Errorf("alias tool %q is not registered — it cannot be callable", name)
		}
	}

	// Count check: registered == sentinel + canonical + alias.
	want := 1 + len(canonicalToolNames) + len(aliasToolNames)
	if len(registered) != want {
		t.Errorf("registered=%d, want sentinel(1)+canonical(%d)+alias(%d)=%d",
			len(registered), len(canonicalToolNames), len(aliasToolNames), want)
	}
}
