package feedback

import (
	"context"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/cajasmota/grafel/internal/graph"
)

// makeEntity is a test helper that builds a minimal graph.Entity.
func makeEntity(id, name, kind, lang, srcFile string, startLine, endLine int) graph.Entity {
	return graph.Entity{
		ID:         id,
		Name:       name,
		Kind:       kind,
		Language:   lang,
		SourceFile: srcFile,
		StartLine:  startLine,
		EndLine:    endLine,
		Properties: map[string]string{},
	}
}

// makeDoc builds a minimal graph.Document from a slice of entities.
func makeDoc(entities []graph.Entity, rels []graph.Relationship) *graph.Document {
	return &graph.Document{
		Entities:      entities,
		Relationships: rels,
		Stats: graph.Stats{
			Entities:      len(entities),
			Relationships: len(rels),
		},
	}
}

// repeat produces n copies of e with unique IDs.
func repeat(e graph.Entity, n int) []graph.Entity {
	out := make([]graph.Entity, n)
	for i := range out {
		out[i] = e
		out[i].ID = e.ID + string(rune('a'+i%26)) + strings.Repeat("x", i/26)
	}
	return out
}

func TestGenerate_SuppressedWhenTooFewEntities(t *testing.T) {
	// 10 entities — below the 50 minimum.
	entities := repeat(makeEntity("e1", "MyFunc", "function", "go", "main.go", 1, 10), 10)
	doc := makeDoc(entities, nil)

	r, err := Generate(context.Background(), []*graph.Document{doc}, Opts{GroupName: "test-group"})
	if err != nil {
		t.Fatalf("Generate: %v", err)
	}
	if !r.IsSuppressed() {
		t.Error("expected report to be suppressed for < 50 entities")
	}
}

func TestGenerate_NotSuppressedAtThreshold(t *testing.T) {
	entities := repeat(makeEntity("e1", "MyFunc", "function", "go", "main.go", 1, 10), 50)
	doc := makeDoc(entities, nil)

	r, err := Generate(context.Background(), []*graph.Document{doc}, Opts{GroupName: "test-group"})
	if err != nil {
		t.Fatalf("Generate: %v", err)
	}
	if r.IsSuppressed() {
		t.Error("expected report to NOT be suppressed at exactly 50 entities")
	}
}

func TestGenerate_LanguageCountsSuppressed(t *testing.T) {
	// 5 Go entities + 50 Python entities. Go should be suppressed (< 10).
	var entities []graph.Entity
	entities = append(entities, repeat(makeEntity("g1", "GoFunc", "function", "go", "main.go", 1, 5), 5)...)
	entities = append(entities, repeat(makeEntity("p1", "PyFunc", "function", "python", "main.py", 1, 5), 50)...)
	doc := makeDoc(entities, nil)

	r, err := Generate(context.Background(), []*graph.Document{doc}, Opts{})
	if err != nil {
		t.Fatalf("Generate: %v", err)
	}
	if _, ok := r.EntitiesByLanguage["go"]; ok {
		t.Error("go with only 5 entities should be suppressed (< 10)")
	}
	if _, ok := r.EntitiesByLanguage["python"]; !ok {
		t.Error("python with 50 entities should be present")
	}
}

func TestGenerate_OrphanRateComputed(t *testing.T) {
	// 20 function entities, no outgoing semantic edges → all orphan.
	entities := repeat(makeEntity("f1", "DoWork", "function", "go", "a.go", 1, 10), 20)
	doc := makeDoc(entities, nil)

	r, err := Generate(context.Background(), []*graph.Document{doc}, Opts{})
	if err != nil {
		t.Fatalf("Generate: %v", err)
	}
	ks, ok := r.OrphanByKind["function"]
	if !ok {
		t.Fatal("expected OrphanByKind[function]")
	}
	if ks.OrphanCount != 20 {
		t.Errorf("expected 20 orphans, got %d", ks.OrphanCount)
	}
	if ks.OrphanPct != 100.0 {
		t.Errorf("expected 100%% orphan rate, got %.1f%%", ks.OrphanPct)
	}
}

func TestGenerate_SemanticEdgeReducesOrphanRate(t *testing.T) {
	entities := repeat(makeEntity("f1", "Caller", "function", "go", "a.go", 1, 10), 20)
	// Give the first entity a semantic CALLS edge.
	rels := []graph.Relationship{
		{ID: "r1", FromID: "f1a", ToID: "f1b", Kind: "CALLS"},
	}
	doc := makeDoc(entities, rels)

	r, err := Generate(context.Background(), []*graph.Document{doc}, Opts{})
	if err != nil {
		t.Fatalf("Generate: %v", err)
	}
	ks, ok := r.OrphanByKind["function"]
	if !ok {
		t.Fatal("expected OrphanByKind[function]")
	}
	// f1a has a CALLS edge so it is not an orphan; 19 are orphans.
	if ks.OrphanCount != 19 {
		t.Errorf("expected 19 orphans (1 has CALLS edge), got %d", ks.OrphanCount)
	}
}

func TestGenerate_ContainsDeclaresDontReduceOrphan(t *testing.T) {
	entities := repeat(makeEntity("e1", "Thing", "class", "java", "A.java", 1, 20), 15)
	// CONTAINS and DECLARES edges should NOT count as semantic.
	rels := []graph.Relationship{
		{ID: "r1", FromID: "e1a", ToID: "e1b", Kind: "CONTAINS"},
		{ID: "r2", FromID: "e1c", ToID: "e1d", Kind: "DECLARES"},
	}
	doc := makeDoc(entities, rels)

	r, err := Generate(context.Background(), []*graph.Document{doc}, Opts{})
	if err != nil {
		t.Fatalf("Generate: %v", err)
	}
	ks := r.OrphanByKind["class"]
	// All 15 are still orphans (CONTAINS/DECLARES don't reduce orphan count).
	if ks.OrphanCount != 15 {
		t.Errorf("expected 15 orphans (CONTAINS/DECLARES excluded), got %d", ks.OrphanCount)
	}
}

func TestGenerate_ResolutionDisposition(t *testing.T) {
	entities := repeat(makeEntity("e1", "X", "function", "go", "x.go", 1, 5), 50)
	rels := []graph.Relationship{
		{ID: "r1", FromID: "e1a", ToID: "e1b", Kind: "CALLS", Properties: map[string]string{"resolution": "resolved"}},
		{ID: "r2", FromID: "e1c", ToID: "e1d", Kind: "CALLS", Properties: map[string]string{"resolution": "bug_extractor"}},
		{ID: "r3", FromID: "e1e", ToID: "e1f", Kind: "CALLS", Properties: map[string]string{"resolution": "external_known"}},
		{ID: "r4", FromID: "e1g", ToID: "e1h", Kind: "CALLS", Properties: map[string]string{"resolution": "dynamic"}},
	}
	doc := makeDoc(entities, rels)

	r, err := Generate(context.Background(), []*graph.Document{doc}, Opts{})
	if err != nil {
		t.Fatalf("Generate: %v", err)
	}
	if r.ResolutionTotal != 4 {
		t.Errorf("expected ResolutionTotal=4, got %d", r.ResolutionTotal)
	}
	if r.Resolution.ResolvedPct != 25.0 {
		t.Errorf("expected resolved 25%%, got %.1f%%", r.Resolution.ResolvedPct)
	}
}

func TestGenerate_SourceWindowCompleteness(t *testing.T) {
	entities := []graph.Entity{
		makeEntity("e1", "Good", "function", "go", "a.go", 5, 15), // valid window
		makeEntity("e2", "Bad", "function", "go", "a.go", 0, 0),   // no window
		makeEntity("e3", "Bad2", "function", "go", "a.go", 5, 5),  // equal lines
	}
	// Need at least 50 total — pad with valid-window entities.
	for i := 3; i < 50; i++ {
		e := makeEntity("pad", "Fn", "function", "go", "a.go", 1, 10)
		e.ID = fmt.Sprintf("pad%d", i)
		entities = append(entities, e)
	}
	doc := makeDoc(entities, nil)

	r, err := Generate(context.Background(), []*graph.Document{doc}, Opts{})
	if err != nil {
		t.Fatalf("Generate: %v", err)
	}
	if r.SourceWindow.TotalEntities != 50 {
		t.Errorf("expected 50 total entities, got %d", r.SourceWindow.TotalEntities)
	}
	// 1 good from the first three + 47 good from padding = 48 with valid windows.
	if r.SourceWindow.TotalWithWindow != 48 {
		t.Errorf("expected 48 entities with valid window, got %d", r.SourceWindow.TotalWithWindow)
	}
}

func TestGenerate_MultipleDocsAggregated(t *testing.T) {
	entities1 := repeat(makeEntity("e1", "GoFunc", "function", "go", "a.go", 1, 5), 30)
	entities2 := repeat(makeEntity("e2", "PyFunc", "function", "python", "b.py", 1, 5), 30)
	doc1 := makeDoc(entities1, nil)
	doc2 := makeDoc(entities2, nil)

	r, err := Generate(context.Background(), []*graph.Document{doc1, doc2}, Opts{})
	if err != nil {
		t.Fatalf("Generate: %v", err)
	}
	if r.TotalEntities != 60 {
		t.Errorf("expected 60 total entities, got %d", r.TotalEntities)
	}
	if r.IsSuppressed() {
		t.Error("expected report to NOT be suppressed with 60 entities")
	}
}

func TestRender_SuppressedReport(t *testing.T) {
	r := &Report{
		TotalEntities: 10,
		GeneratedAt:   mustParseTime("2026-05-27T00:00:00Z"),
		GroupName:     "tiny-group",
		suppressed:    true,
	}
	var sb strings.Builder
	if err := Render(&sb, r); err != nil {
		t.Fatalf("Render: %v", err)
	}
	out := sb.String()
	if !strings.Contains(out, "suppressed") {
		t.Error("suppressed report should contain the word 'suppressed'")
	}
	if strings.Contains(out, "## 1.") {
		t.Error("suppressed report should not contain section headers")
	}
}

func TestRender_FullReport(t *testing.T) {
	r := &Report{
		TotalEntities:      100,
		TotalRelationships: 200,
		GeneratedAt:        mustParseTime("2026-05-27T00:00:00Z"),
		GroupName:          "test-group",
		Version:            "v1.0.0",
		Languages:          []string{"go", "typescript"},
		EntitiesByLanguage: map[string]int{"go": 80, "typescript": 20},
		EntityKindDist:     []EntityKindLang{{Kind: "function", Language: "go", Count: 50}},
		SourceWindow:       SourceWindowStats{TotalWithWindow: 90, TotalEntities: 100, PctComplete: 90.0},
		OrphanByKind:       map[string]KindStats{"function": {Total: 80, OrphanCount: 16, OrphanPct: 20.0}},
		Resolution: ResolutionVector{
			ResolvedPct:        70.0,
			ExternalKnownPct:   10.0,
			ExternalUnknownPct: 10.0,
			BugExtractorPct:    5.0,
			BugResolverPct:     4.0,
			DynamicPct:         1.0,
		},
		ResolutionTotal: 200,
		FrameworkHits:   map[string]int{"gin": 15},
		SanityResults:   []SanityResult{{Name: "minimum-entity-count", Passed: true}},
		Confidence:      100,
	}
	r.AnnotationCoverage.Total = 100
	r.AnnotationCoverage.TotalAnnotated = 15
	r.AnnotationCoverage.PctAnnotated = 15.0

	var sb strings.Builder
	if err := Render(&sb, r); err != nil {
		t.Fatalf("Render: %v", err)
	}
	out := sb.String()

	// Check all sections present.
	for _, section := range []string{
		"## 1. Extractor Coverage",
		"## 2. Orphan Rate",
		"## 3. Resolution Disposition",
		"## 4. Framework Recognition",
		"## 5. Cross-Stack Flows",
		"## 6. Docgen Quality",
		"## 7. Sanity Check Details",
	} {
		if !strings.Contains(out, section) {
			t.Errorf("expected section %q in output", section)
		}
	}

	// Phase 1 placeholders.
	if !strings.Contains(out, "(not in Phase 1)") {
		t.Error("expected Phase 1 placeholder text for cross-stack and docgen sections")
	}

	// Footer privacy note.
	if !strings.Contains(out, "ephemeral") {
		t.Error("expected privacy footer mentioning ephemeral salt")
	}

	// Framework hits.
	if !strings.Contains(out, "gin") {
		t.Error("expected framework 'gin' in output")
	}
}

// mustParseTime parses an RFC3339 time for use in tests.
func mustParseTime(s string) time.Time {
	t, err := time.Parse(time.RFC3339, s)
	if err != nil {
		panic(err)
	}
	return t
}
