package mcp

import (
	"os"
	"path/filepath"
	"testing"
)

// edge_confidence_test.go asserts the MCP read-side surface for the #3628
// extraction-confidence honesty marker. The neighbors handler emits
// l.EdgeConfidence() under the "confidence" key for every cross-repo overlay
// edge; these tests pin the value-mapping and the absence⇒resolved contract
// that the handler relies on.

func TestCrossRepoLink_EdgeConfidence_StampedValues(t *testing.T) {
	cases := []struct {
		name string
		prop string
		want string
	}{
		{"resolved", "resolved", "resolved"},
		{"heuristic", "heuristic", "heuristic"},
		{"inferred", "inferred", "inferred"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			l := CrossRepoLink{
				Source:     "frontend::fn1",
				Target:     "backend::h1",
				Kind:       "calls",
				Properties: map[string]string{edgeConfidenceProp: tc.prop},
			}
			if got := l.EdgeConfidence(); got != tc.want {
				t.Errorf("EdgeConfidence() = %q, want %q", got, tc.want)
			}
		})
	}
}

// TestCrossRepoLink_EdgeConfidence_AbsentDefaultsResolved pins the honesty
// contract: an AST-grounded structural edge (import_pass) carries no marker,
// and a consumer MUST read that absence as "resolved".
func TestCrossRepoLink_EdgeConfidence_AbsentDefaultsResolved(t *testing.T) {
	// No Properties at all.
	if got := (CrossRepoLink{Kind: "import"}).EdgeConfidence(); got != "resolved" {
		t.Errorf("absent marker: want resolved, got %q", got)
	}
	// Properties present but no confidence key.
	l := CrossRepoLink{Properties: map[string]string{"resolve_strategy": "exact"}}
	if got := l.EdgeConfidence(); got != "resolved" {
		t.Errorf("no-confidence-key: want resolved, got %q", got)
	}
}

// TestCrossRepoLink_Properties_RoundTrip verifies the on-disk links-pass
// "properties" object (where the marker is stamped) deserialises into the
// MCP-side CrossRepoLink struct so the neighbors handler can read it.
func TestCrossRepoLink_Properties_RoundTrip(t *testing.T) {
	const data = `[{"source":"frontend::fn1","target":"backend::h1","relation":"calls",
		"method":"http","properties":{"confidence":"heuristic","resolve_strategy":"exact"}}]`
	path := filepath.Join(t.TempDir(), "g-links.json")
	if err := os.WriteFile(path, []byte(data), 0o644); err != nil {
		t.Fatal(err)
	}
	links, err := readLinks(path)
	if err != nil {
		t.Fatal(err)
	}
	if len(links) != 1 {
		t.Fatalf("want 1 link, got %d", len(links))
	}
	if got := links[0].EdgeConfidence(); got != "heuristic" {
		t.Errorf("round-tripped confidence: want heuristic, got %q", got)
	}
}
