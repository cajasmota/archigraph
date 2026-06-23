package sql

import (
	"testing"

	"github.com/cajasmota/grafel/internal/treesitter/ts/official"
)

// TestSQLSmokeParse is the ABI guard for the vendored (regenerated) sql grammar.
// A grammar whose LANGUAGE_VERSION outruns the runtime compiles but SIGSEGVs at
// RootNode (ADR 0023 §6); the regenerated parser.c is ABI 14, inside the v0.24.0
// window. This parses trivial SQL (exercising the external scanner) through the
// official adapter and asserts a sane, non-error root.
func TestSQLSmokeParse(t *testing.T) {
	adapter := official.New()
	parser, err := adapter.NewParser(Language())
	if err != nil {
		t.Fatalf("NewParser failed (ABI mismatch?): %v", err)
	}
	defer parser.Close()

	src := []byte("SELECT id FROM t WHERE id = 1;\n")
	tree, err := parser.Parse(src)
	if err != nil {
		t.Fatalf("Parse failed: %v", err)
	}
	if tree == nil {
		t.Fatal("Parse returned nil tree")
	}
	defer tree.Close()

	root := tree.RootNode()
	if root == nil {
		t.Fatal("RootNode is nil (ABI mismatch crashes here in the bad pairing)")
	}
	if got := root.Type(); got != "program" {
		t.Fatalf("root kind = %q, want program", got)
	}
	if root.IsError() {
		t.Fatal("root is an ERROR node")
	}
}
