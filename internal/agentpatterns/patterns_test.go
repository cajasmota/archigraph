package agentpatterns_test

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/cajasmota/grafel/internal/agentpatterns"
)

// ---------------------------------------------------------------------------
// PatternID determinism
// ---------------------------------------------------------------------------

func TestPatternID_Deterministic(t *testing.T) {
	group := "my-group"
	nl := "Always register a health-check endpoint at /healthz"

	id1 := agentpatterns.PatternID(group, nl)
	id2 := agentpatterns.PatternID(group, nl)
	if id1 != id2 {
		t.Fatalf("PatternID not deterministic: %q vs %q", id1, id2)
	}
	if len(id1) != 16 {
		t.Fatalf("PatternID length = %d, want 16", len(id1))
	}
}

func TestPatternID_DifferentGroupsDifferentIDs(t *testing.T) {
	nl := "Always use atomic file writes"
	id1 := agentpatterns.PatternID("group-a", nl)
	id2 := agentpatterns.PatternID("group-b", nl)
	if id1 == id2 {
		t.Fatalf("different groups should produce different IDs, both got %q", id1)
	}
}

func TestPatternID_DifferentTriggersDifferentIDs(t *testing.T) {
	group := "my-group"
	id1 := agentpatterns.PatternID(group, "use chi router")
	id2 := agentpatterns.PatternID(group, "use gorilla/mux router")
	if id1 == id2 {
		t.Fatalf("different triggers should produce different IDs, both got %q", id1)
	}
}

// ---------------------------------------------------------------------------
// Round-trip serialize / deserialize (all fields populated)
// ---------------------------------------------------------------------------

func fullPattern(group string) agentpatterns.Pattern {
	nl := "Add a new HTTP endpoint following the chi handler pattern"
	return agentpatterns.Pattern{
		ID:   agentpatterns.PatternID(group, nl),
		Kind: "AgentPattern",
		Trigger: agentpatterns.Trigger{
			NaturalLanguage:   nl,
			Keywords:          []string{"endpoint", "chi", "handler", "http"},
			TargetEntityKinds: []string{"SCOPE.Endpoint", "SCOPE.Function"},
		},
		Steps: []string{
			"Create handler function in internal/handlers/",
			"Register route in cmd/server/routes.go",
			"Add OpenAPI operation annotation",
			"Write integration test in internal/handlers/*_test.go",
		},
		AntiPatterns: []agentpatterns.AntiPattern{
			{DoNot: "Inline business logic in the handler", Reason: "Violates separation of concerns", Private: false},
			{DoNot: "Skip the integration test", Reason: "CI will catch a missing test", Private: true},
		},
		Scope: agentpatterns.Scope{
			Repos:       []string{"my-service"},
			ModulePaths: []string{"internal/handlers"},
			Languages:   []string{"go"},
			Stacks:      []string{"go/chi"},
			EntityKinds: []string{"SCOPE.Endpoint"},
		},
		Category:          agentpatterns.CategoryCode,
		Confidence:        0.75,
		Observations:      5,
		LastValidated:     1716100000,
		LastApplied:       1716200000,
		IsCandidate:       false,
		ConvergenceCount:  3,
		ProposerSubagents: []string{"subagent-a", "subagent-b", "subagent-c"},
		DocumentationURL:  "docs/patterns/code/chi-handler.md",
	}
}

func TestRoundTrip_AllFields(t *testing.T) {
	dir := t.TempDir()
	group := "test-group"

	original := fullPattern(group)
	if err := agentpatterns.Save(dir, []agentpatterns.Pattern{original}); err != nil {
		t.Fatalf("Save: %v", err)
	}

	loaded, err := agentpatterns.Load(dir)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if len(loaded) != 1 {
		t.Fatalf("got %d patterns, want 1", len(loaded))
	}

	got := loaded[0]
	if got.ID != original.ID {
		t.Errorf("ID: got %q want %q", got.ID, original.ID)
	}
	if got.Kind != "AgentPattern" {
		t.Errorf("Kind: got %q want AgentPattern", got.Kind)
	}
	if got.Trigger.NaturalLanguage != original.Trigger.NaturalLanguage {
		t.Errorf("Trigger.NaturalLanguage mismatch")
	}
	if len(got.Trigger.Keywords) != len(original.Trigger.Keywords) {
		t.Errorf("Keywords len: got %d want %d", len(got.Trigger.Keywords), len(original.Trigger.Keywords))
	}
	if len(got.Trigger.TargetEntityKinds) != len(original.Trigger.TargetEntityKinds) {
		t.Errorf("TargetEntityKinds len mismatch")
	}
	if len(got.Steps) != len(original.Steps) {
		t.Errorf("Steps len: got %d want %d", len(got.Steps), len(original.Steps))
	}
	if len(got.AntiPatterns) != len(original.AntiPatterns) {
		t.Fatalf("AntiPatterns len: got %d want %d", len(got.AntiPatterns), len(original.AntiPatterns))
	}
	if got.Scope.Languages[0] != "go" {
		t.Errorf("Scope.Languages[0]: got %q want go", got.Scope.Languages[0])
	}
	if got.Category != agentpatterns.CategoryCode {
		t.Errorf("Category: got %q want code", got.Category)
	}
	if got.Confidence != original.Confidence {
		t.Errorf("Confidence: got %v want %v", got.Confidence, original.Confidence)
	}
	if got.Observations != original.Observations {
		t.Errorf("Observations: got %d want %d", got.Observations, original.Observations)
	}
	if got.LastValidated != original.LastValidated {
		t.Errorf("LastValidated mismatch")
	}
	if got.LastApplied != original.LastApplied {
		t.Errorf("LastApplied mismatch")
	}
	if got.IsCandidate != original.IsCandidate {
		t.Errorf("IsCandidate: got %v want %v", got.IsCandidate, original.IsCandidate)
	}
	if got.ConvergenceCount != original.ConvergenceCount {
		t.Errorf("ConvergenceCount: got %d want %d", got.ConvergenceCount, original.ConvergenceCount)
	}
	if len(got.ProposerSubagents) != len(original.ProposerSubagents) {
		t.Errorf("ProposerSubagents len mismatch")
	}
	if got.DocumentationURL != original.DocumentationURL {
		t.Errorf("DocumentationURL: got %q want %q", got.DocumentationURL, original.DocumentationURL)
	}
}

// ---------------------------------------------------------------------------
// Anti-pattern with private=true serializes correctly
// ---------------------------------------------------------------------------

func TestAntiPattern_PrivateFlag(t *testing.T) {
	dir := t.TempDir()
	p := agentpatterns.Pattern{
		ID:   agentpatterns.PatternID("g", "test trigger"),
		Kind: "AgentPattern",
		Trigger: agentpatterns.Trigger{
			NaturalLanguage: "test trigger",
		},
		AntiPatterns: []agentpatterns.AntiPattern{
			{DoNot: "skip linting", Reason: "CI requires it", Private: false},
			{DoNot: "hardcode secrets", Reason: "security risk", Private: true},
		},
		Category:   agentpatterns.CategoryTooling,
		Confidence: agentpatterns.InitialConfidence,
	}

	if err := agentpatterns.Save(dir, []agentpatterns.Pattern{p}); err != nil {
		t.Fatalf("Save: %v", err)
	}

	// Read raw bytes to confirm the private field is present in JSON.
	raw, _ := os.ReadFile(filepath.Join(dir, "patterns.json"))
	var envelope struct {
		Patterns []struct {
			AntiPatterns []struct {
				DoNot   string `json:"do_not"`
				Private bool   `json:"private"`
			} `json:"anti_patterns"`
		} `json:"patterns"`
	}
	if err := json.Unmarshal(raw, &envelope); err != nil {
		t.Fatalf("unmarshal raw: %v", err)
	}
	if len(envelope.Patterns) != 1 {
		t.Fatalf("expected 1 pattern in raw JSON")
	}
	aps := envelope.Patterns[0].AntiPatterns
	if len(aps) != 2 {
		t.Fatalf("expected 2 anti-patterns in raw JSON, got %d", len(aps))
	}
	// The private one should have private: true in the output.
	if !aps[1].Private {
		t.Errorf("second anti-pattern private flag not preserved: got false, want true")
	}
	// The public one should have private: false (or omitted and thus false).
	if aps[0].Private {
		t.Errorf("first anti-pattern should not be private")
	}
}

// ---------------------------------------------------------------------------
// Scope validation: empty constraint = wildcard
// ---------------------------------------------------------------------------

func TestScope_EmptyIsWildcard(t *testing.T) {
	empty := agentpatterns.Scope{}
	if len(empty.Repos) != 0 {
		t.Error("empty scope repos should be zero-length")
	}
	if len(empty.Languages) != 0 {
		t.Error("empty scope languages should be zero-length")
	}
	if len(empty.Stacks) != 0 {
		t.Error("empty scope stacks should be zero-length")
	}
	if len(empty.EntityKinds) != 0 {
		t.Error("empty scope entity_kinds should be zero-length")
	}
}

func TestScope_PartialConstraint(t *testing.T) {
	dir := t.TempDir()
	// A pattern scoped only by language (all repos, paths, stacks).
	p := agentpatterns.Pattern{
		ID:   agentpatterns.PatternID("g", "python pattern"),
		Kind: "AgentPattern",
		Trigger: agentpatterns.Trigger{
			NaturalLanguage: "python pattern",
		},
		Scope: agentpatterns.Scope{
			Languages: []string{"python"},
			// Repos, ModulePaths, Stacks, EntityKinds are all empty → wildcard
		},
		Category:   agentpatterns.CategoryCode,
		Confidence: agentpatterns.InitialConfidence,
	}
	if err := agentpatterns.Save(dir, []agentpatterns.Pattern{p}); err != nil {
		t.Fatalf("Save: %v", err)
	}
	loaded, err := agentpatterns.Load(dir)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if len(loaded) != 1 {
		t.Fatalf("got %d patterns, want 1", len(loaded))
	}
	if len(loaded[0].Scope.Languages) != 1 || loaded[0].Scope.Languages[0] != "python" {
		t.Errorf("scope.languages not preserved")
	}
	if len(loaded[0].Scope.Repos) != 0 {
		t.Errorf("scope.repos should remain empty (wildcard), got %v", loaded[0].Scope.Repos)
	}
}

// ---------------------------------------------------------------------------
// Load returns empty slice for missing file (not an error)
// ---------------------------------------------------------------------------

func TestLoad_MissingFile_ReturnsEmpty(t *testing.T) {
	dir := t.TempDir()
	patterns, err := agentpatterns.Load(dir)
	if err != nil {
		t.Fatalf("expected no error for missing file, got %v", err)
	}
	if patterns == nil {
		t.Error("Load should return non-nil slice for missing file")
	}
	if len(patterns) != 0 {
		t.Errorf("expected 0 patterns, got %d", len(patterns))
	}
}

// ---------------------------------------------------------------------------
// Save atomicity: tmp+rename approach
// ---------------------------------------------------------------------------

func TestSave_AtomicWrite(t *testing.T) {
	dir := t.TempDir()
	p := agentpatterns.Pattern{
		ID:   agentpatterns.PatternID("g", "atomic test"),
		Kind: "AgentPattern",
		Trigger: agentpatterns.Trigger{
			NaturalLanguage: "atomic test",
		},
		Category:   agentpatterns.CategoryProcess,
		Confidence: agentpatterns.InitialConfidence,
	}
	if err := agentpatterns.Save(dir, []agentpatterns.Pattern{p}); err != nil {
		t.Fatalf("Save: %v", err)
	}
	// Confirm no stray .tmp file remains.
	tmpPath := filepath.Join(dir, "patterns.json.tmp")
	if _, err := os.Stat(tmpPath); !os.IsNotExist(err) {
		t.Error("stray .tmp file found after Save; atomic rename failed")
	}
}

// ---------------------------------------------------------------------------
// Upsert and ByID helpers
// ---------------------------------------------------------------------------

func TestUpsert_Insert(t *testing.T) {
	var patterns []agentpatterns.Pattern
	p := agentpatterns.Pattern{ID: "aabbccddeeff0011", Kind: "AgentPattern"}
	patterns = agentpatterns.Upsert(patterns, p)
	if len(patterns) != 1 {
		t.Fatalf("expected 1, got %d", len(patterns))
	}
}

func TestUpsert_Replace(t *testing.T) {
	p1 := agentpatterns.Pattern{ID: "aabbccddeeff0011", Kind: "AgentPattern", Confidence: 0.4}
	p2 := agentpatterns.Pattern{ID: "aabbccddeeff0011", Kind: "AgentPattern", Confidence: 0.7}
	patterns := []agentpatterns.Pattern{p1}
	patterns = agentpatterns.Upsert(patterns, p2)
	if len(patterns) != 1 {
		t.Fatalf("expected 1 after replace, got %d", len(patterns))
	}
	if patterns[0].Confidence != 0.7 {
		t.Errorf("confidence not updated: got %v want 0.7", patterns[0].Confidence)
	}
}

func TestByID_Found(t *testing.T) {
	p := agentpatterns.Pattern{ID: "deadbeef00001234", Kind: "AgentPattern"}
	patterns := []agentpatterns.Pattern{p}
	found := agentpatterns.ByID(patterns, "deadbeef00001234")
	if found == nil {
		t.Error("ByID returned nil for existing pattern")
	}
}

func TestByID_NotFound(t *testing.T) {
	p := agentpatterns.Pattern{ID: "deadbeef00001234", Kind: "AgentPattern"}
	patterns := []agentpatterns.Pattern{p}
	found := agentpatterns.ByID(patterns, "0000000000000000")
	if found != nil {
		t.Error("ByID should return nil for unknown ID")
	}
}

// ---------------------------------------------------------------------------
// Sort stability: Save produces stable output across multiple calls
// ---------------------------------------------------------------------------

func TestSave_StableSort(t *testing.T) {
	dir := t.TempDir()
	patterns := []agentpatterns.Pattern{
		{ID: "zzzzzzzzzzzzzzzz", Kind: "AgentPattern", Trigger: agentpatterns.Trigger{NaturalLanguage: "z"}},
		{ID: "aaaaaaaaaaaaaaaa", Kind: "AgentPattern", Trigger: agentpatterns.Trigger{NaturalLanguage: "a"}},
		{ID: "mmmmmmmmmmmmmmmm", Kind: "AgentPattern", Trigger: agentpatterns.Trigger{NaturalLanguage: "m"}},
	}

	if err := agentpatterns.Save(dir, patterns); err != nil {
		t.Fatalf("Save: %v", err)
	}
	raw1, _ := os.ReadFile(filepath.Join(dir, "patterns.json"))

	// Save again in different order — output should be identical.
	patterns2 := []agentpatterns.Pattern{patterns[2], patterns[0], patterns[1]}
	if err := agentpatterns.Save(dir, patterns2); err != nil {
		t.Fatalf("Save2: %v", err)
	}
	raw2, _ := os.ReadFile(filepath.Join(dir, "patterns.json"))

	if string(raw1) != string(raw2) {
		t.Error("Save produced different output for the same patterns in different insertion order (sort not stable)")
	}
}

// ---------------------------------------------------------------------------
// New() helper
// ---------------------------------------------------------------------------

func TestNew_DefaultValues(t *testing.T) {
	trigger := agentpatterns.Trigger{
		NaturalLanguage: "register chi middleware",
		Keywords:        []string{"chi", "middleware"},
	}
	p := agentpatterns.New("group-x", trigger, agentpatterns.CategoryCode)
	if p.Confidence != agentpatterns.InitialConfidence {
		t.Errorf("confidence = %v, want %v", p.Confidence, agentpatterns.InitialConfidence)
	}
	if !p.IsCandidate {
		t.Error("new pattern should be a candidate by default")
	}
	if p.Kind != "AgentPattern" {
		t.Errorf("kind = %q, want AgentPattern", p.Kind)
	}
	expectedID := agentpatterns.PatternID("group-x", "register chi middleware")
	if p.ID != expectedID {
		t.Errorf("id = %q, want %q", p.ID, expectedID)
	}
}
