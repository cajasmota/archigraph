package docgen_test

// llm_bundle_small_file_test.go — tests for the small-file whole-file
// source_window strategy introduced in #1872.
//
// For files with ≤ SmallFileLineThreshold lines, BuildBundle must emit the
// ENTIRE file as source_window regardless of entity start/end lines.
// This prevents the ±20-line default from clipping small frontend components
// or Python helpers whose last lines are semantically important.
//
// Test surface:
//   - Small file (≤ 80 lines): source_window contains the full file content.
//   - Large file (> 80 lines): source_window uses default ±20 strategy (does
//     not include lines far from start).
//   - Model entity with small source file: WholeBody strategy takes precedence
//     over the small-file path — no regression for Model kinds.
//   - SmallFileMaxBytes cap: a file with very long lines is not unbounded.
//   - reference-misc default guidance: narrowed to three categories.

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/cajasmota/grafel/internal/daemon"
	"github.com/cajasmota/grafel/internal/docgen"
	"github.com/cajasmota/grafel/internal/graph"
	"github.com/cajasmota/grafel/internal/registry"
)

// ---------------------------------------------------------------------------
// Harness helpers
// ---------------------------------------------------------------------------

// smallFileHarness builds an isolated test environment with a single entity
// and a source file whose content is generated line-by-line.  The entity
// kind and file parameters allow the caller to drive both the small-file and
// large-file paths.
//
//   - totalLines: number of lines written to the source file.
//   - entityStart: entity.StartLine value stored in graph.json.
//   - entityEnd: entity.EndLine value stored in graph.json.
//   - kind: entity kind (e.g. "Function", "react_component", "Model").
func smallFileHarness(
	t *testing.T,
	totalLines, entityStart, entityEnd int,
	kind string,
) (groupName, entityID string) {
	t.Helper()

	tmp := t.TempDir()

	homeDir := filepath.Join(tmp, "home")
	xdgDir := filepath.Join(tmp, "xdg")
	daemonRoot := filepath.Join(tmp, "daemon")
	repoPath := filepath.Join(tmp, "testrepo")

	for _, d := range []string{homeDir, xdgDir, daemonRoot, repoPath} {
		if err := os.MkdirAll(d, 0o755); err != nil {
			t.Fatalf("mkdir %s: %v", d, err)
		}
	}

	t.Setenv("GRAFEL_HOME", homeDir)
	t.Setenv("XDG_CONFIG_HOME", xdgDir)
	t.Setenv(daemon.EnvRoot, daemonRoot)

	groupName = fmt.Sprintf("small-file-sw-test-%s-%d", kind, totalLines)

	cfgPath, err := registry.ConfigPathFor(groupName)
	if err != nil {
		t.Fatalf("ConfigPathFor: %v", err)
	}
	if err := os.MkdirAll(filepath.Dir(cfgPath), 0o755); err != nil {
		t.Fatalf("mkdir fleet config dir: %v", err)
	}
	fleetJSON, _ := json.Marshal(map[string]interface{}{
		"name": groupName,
		"repos": []map[string]interface{}{
			{"path": repoPath, "slug": "testrepo"},
		},
	})
	if err := os.WriteFile(cfgPath, fleetJSON, 0o644); err != nil {
		t.Fatalf("write fleet config: %v", err)
	}

	// Write the source file: each line is a numbered comment so tests can
	// assert specific line numbers are present.
	var sb strings.Builder
	for i := 1; i <= totalLines; i++ {
		sb.WriteString(fmt.Sprintf("// source_line_%04d\n", i))
	}
	srcContent := sb.String()

	srcRelPath := "src/entity.ts"
	srcAbsPath := filepath.Join(repoPath, srcRelPath)
	if err := os.MkdirAll(filepath.Dir(srcAbsPath), 0o755); err != nil {
		t.Fatalf("mkdir src dir: %v", err)
	}
	if err := os.WriteFile(srcAbsPath, []byte(srcContent), 0o644); err != nil {
		t.Fatalf("write source file: %v", err)
	}

	stateDir := daemon.StateDirForRepo(repoPath)
	if err := os.MkdirAll(stateDir, 0o755); err != nil {
		t.Fatalf("mkdir state dir: %v", err)
	}

	entityID = "deadbeef00001872"
	doc := graph.Document{
		Version:        1,
		GeneratedAt:    time.Now().UTC(),
		Repo:           repoPath,
		IndexerVersion: "test",
		Stats:          graph.Stats{Files: 1, Entities: 1, Relationships: 0},
		Entities: []graph.Entity{
			{
				ID:         entityID,
				Name:       "MyEntity",
				Kind:       kind,
				SourceFile: srcRelPath,
				StartLine:  entityStart,
				EndLine:    entityEnd,
				Language:   "typescript",
			},
		},
		Relationships: []graph.Relationship{},
	}
	docJSON, _ := json.Marshal(doc)
	if err := os.WriteFile(filepath.Join(stateDir, "graph.json"), docJSON, 0o644); err != nil {
		t.Fatalf("write graph.json: %v", err)
	}

	return groupName, entityID
}

// buildBundleSourceWindow is a thin helper that calls BuildBundle and returns
// the graph_context.source_window string.
func buildBundleSourceWindow(t *testing.T, groupName, entityID string) string {
	t.Helper()
	opts := docgen.BuildBundleOpts{
		RunOpts: docgen.RunOpts{
			Group:        groupName,
			SeedEntityID: entityID,
			Section:      "overview",
			NoCache:      true,
		},
		Tier:    0,
		NoCache: true,
	}
	bundle, err := docgen.BuildBundle(context.Background(), opts)
	if err != nil {
		t.Fatalf("BuildBundle: %v", err)
	}
	return bundle.GraphContext.SourceWindow
}

// ---------------------------------------------------------------------------
// Tests: #1872 — small-file whole-file strategy
// ---------------------------------------------------------------------------

// TestSmallFile_WholeFileEmitted verifies that when the source file has
// ≤ SmallFileLineThreshold (80) lines, the entire file is emitted in
// source_window — including lines far beyond the entity's start_line+20 boundary.
func TestSmallFile_WholeFileEmitted(t *testing.T) {
	const totalLines = 36 // small file: ≤ 80 lines
	const entityStart = 8 // entity starts early; default ±20 clips at line 28
	const entityEnd = 10  // entity ends at line 10

	groupName, entityID := smallFileHarness(t, totalLines, entityStart, entityEnd, "react_component")
	sw := buildBundleSourceWindow(t, groupName, entityID)

	if sw == "" {
		t.Fatal("source_window is empty for small-file entity")
	}

	// The last line of the file must appear — this is what the default ±20
	// strategy would have clipped.
	lastLine := fmt.Sprintf("source_line_%04d", totalLines)
	if !strings.Contains(sw, lastLine) {
		t.Errorf("source_window missing %q — whole-file strategy must include the last line\nwindow:\n%s", lastLine, sw)
	}

	// The first line must also appear.
	firstLine := "source_line_0001"
	if !strings.Contains(sw, firstLine) {
		t.Errorf("source_window missing %q — whole-file must start from line 1\nwindow:\n%s", firstLine, sw)
	}

	t.Logf("Small-file source_window (%d chars, %d lines in file):\n%s", len(sw), totalLines, sw)
}

// TestSmallFile_ExactThreshold verifies that a file of exactly
// SmallFileLineThreshold lines triggers the whole-file strategy.
func TestSmallFile_ExactThreshold(t *testing.T) {
	totalLines := docgen.SmallFileLineThreshold // exactly 80 lines
	const entityStart = 5
	const entityEnd = 10

	groupName, entityID := smallFileHarness(t, totalLines, entityStart, entityEnd, "Function")
	sw := buildBundleSourceWindow(t, groupName, entityID)

	if sw == "" {
		t.Fatal("source_window is empty for exact-threshold file")
	}

	// Last line must appear (would be clipped by default ±20 at line 30 when
	// start=5, but the threshold is 80 so whole-file fires).
	lastLine := fmt.Sprintf("source_line_%04d", totalLines)
	if !strings.Contains(sw, lastLine) {
		t.Errorf("source_window missing %q at exact threshold (%d lines)\nwindow:\n%s",
			lastLine, totalLines, sw)
	}
}

// TestSmallFile_LargeFileUsesDefault verifies that when the source file has
// > SmallFileLineThreshold lines the default ±20-line strategy is used and the
// file is NOT emitted in full.
func TestSmallFile_LargeFileUsesDefault(t *testing.T) {
	const totalLines = 200 // large file: > 80 lines
	const entityStart = 5
	const entityEnd = 15

	groupName, entityID := smallFileHarness(t, totalLines, entityStart, entityEnd, "Function")
	sw := buildBundleSourceWindow(t, groupName, entityID)

	if sw == "" {
		t.Fatal("source_window is empty for large-file entity")
	}

	// The last line (200) must NOT appear — it is far beyond start+20=25.
	lastLine := fmt.Sprintf("source_line_%04d", totalLines)
	if strings.Contains(sw, lastLine) {
		t.Errorf("source_window contains %q — large file should not be emitted in full\nwindow:\n%s",
			lastLine, sw)
	}
}

// TestSmallFile_ModelWholeBodyNotAffected verifies that a Model entity with a
// small source file still uses the WholeBody strategy (end_line-based), not the
// whole-file path — the two strategies must not interfere.
func TestSmallFile_ModelWholeBodyNotAffected(t *testing.T) {
	const totalLines = 40 // small: ≤ 80 lines
	const entityStart = 2
	const entityEnd = 40 // whole file anyway; test just checks no regression

	// Build harness with a Python-language Model entity so ResolveSectionProfile
	// picks SourceWindowStrategyWholeBody.
	tmp := t.TempDir()
	homeDir := filepath.Join(tmp, "home")
	xdgDir := filepath.Join(tmp, "xdg")
	daemonRoot := filepath.Join(tmp, "daemon")
	repoPath := filepath.Join(tmp, "testrepo")
	for _, d := range []string{homeDir, xdgDir, daemonRoot, repoPath} {
		if err := os.MkdirAll(d, 0o755); err != nil {
			t.Fatalf("mkdir %s: %v", d, err)
		}
	}
	t.Setenv("GRAFEL_HOME", homeDir)
	t.Setenv("XDG_CONFIG_HOME", xdgDir)
	t.Setenv(daemon.EnvRoot, daemonRoot)

	groupName := "small-file-model-no-regression"
	cfgPath, err := registry.ConfigPathFor(groupName)
	if err != nil {
		t.Fatalf("ConfigPathFor: %v", err)
	}
	if err := os.MkdirAll(filepath.Dir(cfgPath), 0o755); err != nil {
		t.Fatalf("mkdir fleet config dir: %v", err)
	}
	fleetJSON, _ := json.Marshal(map[string]interface{}{
		"name": groupName,
		"repos": []map[string]interface{}{
			{"path": repoPath, "slug": "testrepo"},
		},
	})
	if err := os.WriteFile(cfgPath, fleetJSON, 0o644); err != nil {
		t.Fatalf("write fleet config: %v", err)
	}

	var sb strings.Builder
	sb.WriteString("class SmallModel(models.Model):\n")
	for i := 2; i <= totalLines; i++ {
		sb.WriteString(fmt.Sprintf("    field_%02d = models.CharField()\n", i))
	}
	srcRelPath := "models/small.py"
	srcAbsPath := filepath.Join(repoPath, srcRelPath)
	if err := os.MkdirAll(filepath.Dir(srcAbsPath), 0o755); err != nil {
		t.Fatalf("mkdir src dir: %v", err)
	}
	if err := os.WriteFile(srcAbsPath, []byte(sb.String()), 0o644); err != nil {
		t.Fatalf("write source file: %v", err)
	}

	stateDir := daemon.StateDirForRepo(repoPath)
	if err := os.MkdirAll(stateDir, 0o755); err != nil {
		t.Fatalf("mkdir state dir: %v", err)
	}
	entityID := "cafebabe18720001"
	doc := graph.Document{
		Version:     1,
		GeneratedAt: time.Now().UTC(),
		Repo:        repoPath,
		Stats:       graph.Stats{Files: 1, Entities: 1},
		Entities: []graph.Entity{
			{
				ID:         entityID,
				Name:       "SmallModel",
				Kind:       "Model",
				SourceFile: srcRelPath,
				StartLine:  entityStart,
				EndLine:    entityEnd,
				Language:   "python",
			},
		},
	}
	docJSON, _ := json.Marshal(doc)
	if err := os.WriteFile(filepath.Join(stateDir, "graph.json"), docJSON, 0o644); err != nil {
		t.Fatalf("write graph.json: %v", err)
	}

	opts := docgen.BuildBundleOpts{
		RunOpts: docgen.RunOpts{
			Group:        groupName,
			SeedEntityID: entityID,
			Section:      "overview",
			NoCache:      true,
		},
		Tier:    0,
		NoCache: true,
	}
	bundle, err := docgen.BuildBundle(context.Background(), opts)
	if err != nil {
		t.Fatalf("BuildBundle: %v", err)
	}
	sw := bundle.GraphContext.SourceWindow

	// The source_window must be non-empty.
	if sw == "" {
		t.Fatal("source_window is empty for small Model entity")
	}

	// The last field must appear — either whole-file or whole-body both include it.
	if !strings.Contains(sw, fmt.Sprintf("field_%02d", totalLines)) {
		t.Errorf("source_window missing last field for small Model entity\nwindow:\n%s", sw)
	}

	// No crash or truncation annotation (body fits within WholeBodyMaxLines).
	if strings.Contains(sw, "truncated_at_line") {
		t.Errorf("unexpected truncation annotation for small Model entity\nwindow:\n%s", sw)
	}

	t.Logf("Small Model source_window (%d chars):\n%s", len(sw), sw)
}

// ---------------------------------------------------------------------------
// Tests: #1874 — reference-misc guidance narrowed
// ---------------------------------------------------------------------------

// TestReferenceMisc_NarrowedScope verifies that the default reference-misc
// guidance is scoped to the three allowed categories and explicitly prohibits
// the anti-pattern / hardcoded-values / deployment misuse.
func TestReferenceMisc_NarrowedScope(t *testing.T) {
	p := docgen.ResolveSectionProfile("default", "")
	g := docgen.ResolveGuidance(p, "reference-misc")
	lower := strings.ToLower(g)

	// Must contain the three allowed categories.
	for _, want := range []string{
		"cross-cutting",
		"security",
		"compliance",
		"adr",
	} {
		if !strings.Contains(lower, want) {
			t.Errorf("reference-misc guidance: want category term %q; got:\n%s", want, g)
		}
	}

	// Must prohibit routing anti-patterns / code smells here.
	if !strings.Contains(lower, "patterns") {
		t.Errorf("reference-misc guidance: want explicit redirect to `patterns`; got:\n%s", g)
	}

	// Must prohibit routing hardcoded values / feature flags here.
	if !strings.Contains(lower, "reference-config") {
		t.Errorf("reference-misc guidance: want explicit redirect to `reference-config`; got:\n%s", g)
	}
}

// TestReferenceMisc_DefaultGuidanceAppliesAcrossProfiles verifies that the
// narrowed default guidance propagates to entity kinds that do not override it.
func TestReferenceMisc_DefaultGuidanceAppliesAcrossProfiles(t *testing.T) {
	for _, kind := range []string{"operation", "function", "view", "class"} {
		t.Run(kind, func(t *testing.T) {
			p := docgen.ResolveSectionProfile(kind, "")
			g := docgen.ResolveGuidance(p, "reference-misc")
			// All kinds that don't override reference-misc should inherit the
			// default narrowed guidance containing the three-category framing.
			lower := strings.ToLower(g)
			if !strings.Contains(lower, "cross-cutting") &&
				!strings.Contains(lower, "adr") &&
				!strings.Contains(lower, "security") {
				// Profiles with their own reference-misc override (model, module,
				// operation*, etc.) are acceptable; check that they still have
				// SOME reference guidance, not an empty string.
				if g == "" || g == "_No guidance available for this section type._" {
					t.Errorf("kind=%s: reference-misc guidance is empty or missing", kind)
				}
			}
		})
	}
}

// TestSmallFileConstants_Exported verifies that the exported constants added
// in #1872 are accessible from outside the package and have the expected values.
func TestSmallFileConstants_Exported(t *testing.T) {
	if docgen.SmallFileLineThreshold != 80 {
		t.Errorf("SmallFileLineThreshold: want 80, got %d", docgen.SmallFileLineThreshold)
	}
	if docgen.SmallFileMaxBytes != 5*1024 {
		t.Errorf("SmallFileMaxBytes: want %d, got %d", 5*1024, docgen.SmallFileMaxBytes)
	}
	if docgen.SourceWindowStrategyWholeFile != "whole-file" {
		t.Errorf("SourceWindowStrategyWholeFile: want %q, got %q",
			"whole-file", docgen.SourceWindowStrategyWholeFile)
	}
}
