package ingest

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/cajasmota/grafel/internal/graph"
	"github.com/cajasmota/grafel/internal/types"
)

// --- #4308 emit -------------------------------------------------------------

func TestEmitSemanticBundles_PerDocumentWellFormed(t *testing.T) {
	repoRoot, err := filepath.Abs("testdata/repo")
	if err != nil {
		t.Fatal(err)
	}
	repoTag := "repo"
	code := codeEntitiesFixture(repoTag)

	bundles := EmitSemanticBundles(repoRoot, repoTag, []string{"docs/orders.md"}, code)
	if len(bundles) != 1 {
		t.Fatalf("bundles = %d, want 1 (one per document)", len(bundles))
	}
	b := bundles[0]

	if b.Version != SemanticBundleVersion {
		t.Errorf("version = %q, want %q", b.Version, SemanticBundleVersion)
	}
	if b.RepoTag != repoTag {
		t.Errorf("repo_tag = %q, want %q", b.RepoTag, repoTag)
	}
	wantDocID := graph.EntityID(repoTag, string(types.EntityKindMarkdownDocument), "docs/orders.md", "docs/orders.md")
	if b.DocumentID != wantDocID {
		t.Errorf("document_id = %q, want %q", b.DocumentID, wantDocID)
	}
	if b.PromptHash == "" {
		t.Error("bundle prompt_hash is empty")
	}

	// N sections → N prompts; the markdown has 5 headings (# Orders, ## Placing,
	// ### Validation, ## Refunds), i.e. 4 sections.
	if len(b.Sections) != 4 {
		t.Fatalf("sections = %d, want 4", len(b.Sections))
	}

	// Schema is self-describing: closed-set classes + RATIONALE_FOR edge kind.
	if b.Schema.EdgeKind != string(types.RelationshipKindRationaleFor) {
		t.Errorf("schema edge_kind = %q, want %q", b.Schema.EdgeKind, types.RelationshipKindRationaleFor)
	}
	if len(b.Schema.Classes) == 0 || b.Schema.Guidance == "" {
		t.Error("schema must carry classes + guidance")
	}

	// Each section prompt is well-formed and carries section text + a section ID.
	for _, sp := range b.Sections {
		if sp.SectionID == "" {
			t.Errorf("section %q has empty section_id", sp.Heading)
		}
		if sp.PromptHash == "" {
			t.Errorf("section %q has empty prompt_hash", sp.Heading)
		}
	}

	// Grounding: the "Placing an order" section body mentions placeOrder +
	// validateOrder, so its prompt must carry those as mention targets.
	var placing *SemanticSectionPrompt
	for i := range b.Sections {
		if b.Sections[i].Heading == "Placing an order" {
			placing = &b.Sections[i]
		}
	}
	if placing == nil {
		t.Fatal("missing 'Placing an order' section")
	}
	gotTargets := map[string]bool{}
	for _, tgt := range placing.MentionTargets {
		gotTargets[tgt.Name] = true
	}
	if !gotTargets["placeOrder"] || !gotTargets["validateOrder"] {
		t.Errorf("placing.mention_targets = %+v, want placeOrder + validateOrder", placing.MentionTargets)
	}
}

func TestEmitSemanticBundles_Deterministic(t *testing.T) {
	repoRoot, _ := filepath.Abs("testdata/repo")
	code := codeEntitiesFixture("repo")
	a := EmitSemanticBundles(repoRoot, "repo", []string{"docs/orders.md"}, code)
	b := EmitSemanticBundles(repoRoot, "repo", []string{"docs/orders.md"}, code)
	if len(a) != 1 || len(b) != 1 {
		t.Fatalf("want 1 bundle each, got %d/%d", len(a), len(b))
	}
	if a[0].PromptHash != b[0].PromptHash {
		t.Errorf("non-deterministic prompt_hash: %q vs %q", a[0].PromptHash, b[0].PromptHash)
	}
}

func TestWriteReadBundle_Roundtrip(t *testing.T) {
	repoRoot, _ := filepath.Abs("testdata/repo")
	code := codeEntitiesFixture("repo")
	bundles := EmitSemanticBundles(repoRoot, "repo", []string{"docs/orders.md"}, code)
	if len(bundles) != 1 {
		t.Fatalf("want 1 bundle, got %d", len(bundles))
	}
	dir := t.TempDir()
	path, err := WriteBundle(dir, bundles[0])
	if err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(path); err != nil {
		t.Fatalf("bundle not written: %v", err)
	}
	got, err := ReadBundle(path)
	if err != nil {
		t.Fatal(err)
	}
	if got.PromptHash != bundles[0].PromptHash {
		t.Errorf("roundtrip prompt_hash mismatch: %q vs %q", got.PromptHash, bundles[0].PromptHash)
	}
}
