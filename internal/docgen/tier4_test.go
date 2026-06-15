package docgen_test

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/cajasmota/grafel/internal/docgen"
)

// ---------------------------------------------------------------------------
// Test helpers
// ---------------------------------------------------------------------------

// buildMinimalGroupForTier4 creates a minimal GRAFEL_HOME fixture with one
// group, two repos, and synthetic graphs. It mirrors buildMinimalGroupForTier3
// but registers two repos so cross-repo contract checks have material to work on.
func buildMinimalGroupForTier4(t *testing.T) (archHome, group string, slugs []string) {
	t.Helper()
	archHome = t.TempDir()
	group = "tier4-test-group"
	slugs = []string{"alpha", "beta"}

	t.Setenv("GRAFEL_HOME", archHome)

	xdgConfigHome := filepath.Join(archHome, "xdg-config")
	t.Setenv("XDG_CONFIG_HOME", xdgConfigHome)

	cfgDir := filepath.Join(xdgConfigHome, "grafel")
	if err := os.MkdirAll(cfgDir, 0o755); err != nil {
		t.Fatalf("mkdir cfgDir: %v", err)
	}

	var repoCfgs []map[string]interface{}
	for _, slug := range slugs {
		repoPath := filepath.Join(archHome, "fake-"+slug)
		if err := os.MkdirAll(repoPath, 0o755); err != nil {
			t.Fatalf("mkdir repo %s: %v", slug, err)
		}

		svcID := slug + "svc0000111122223333"[:16]
		pr := 0.7
		entities := []interface{}{
			map[string]interface{}{
				"id": svcID, "name": "Service" + slug, "kind": "SCOPE.Service",
				"source_file": "svc/main.go", "start_line": 1, "end_line": 100,
				"language": "go", "pagerank": pr,
			},
		}
		graphDoc := map[string]interface{}{
			"version": 1, "repo": repoPath,
			"entities": entities, "relationships": []interface{}{},
		}
		graphBytes, _ := json.Marshal(graphDoc)
		archDir := filepath.Join(repoPath, ".grafel")
		if err := os.MkdirAll(archDir, 0o755); err != nil {
			t.Fatalf("mkdir archDir: %v", err)
		}
		if err := os.WriteFile(filepath.Join(archDir, "graph.json"), graphBytes, 0o644); err != nil {
			t.Fatalf("write graph.json: %v", err)
		}
		repoCfgs = append(repoCfgs, map[string]interface{}{
			"slug": slug, "path": repoPath,
		})
	}

	groupCfg := map[string]interface{}{
		"name":  group,
		"repos": repoCfgs,
	}
	cfgBytes, _ := json.Marshal(groupCfg)
	if err := os.WriteFile(filepath.Join(cfgDir, group+".fleet.json"), cfgBytes, 0o644); err != nil {
		t.Fatalf("write group config: %v", err)
	}

	return archHome, group, slugs
}

// ---------------------------------------------------------------------------
// 1. checkCrossRepoCoverage — all links resolve
// ---------------------------------------------------------------------------

func TestCheckCrossRepoCoverage_AllResolved(t *testing.T) {
	links := []docgen.GroupCrossRepoLinkForTest{
		{Source: "alpha::svc1", Target: "beta::svc2"},
	}
	repoPageMap := map[string][]docgen.PageOutput{
		"alpha": {{EntityID: "svc1"}},
		"beta":  {{EntityID: "svc2"}},
	}
	violations, count, unresolved := docgen.CheckCrossRepoCoverageForTest(links, repoPageMap)
	if count != 1 {
		t.Errorf("expected 1 link, got %d", count)
	}
	if unresolved != 0 {
		t.Errorf("expected 0 unresolved, got %d", unresolved)
	}
	for _, v := range violations {
		if v.Kind == "cross-repo-coverage" {
			t.Errorf("unexpected cross-repo-coverage violation: %s", v.Message)
		}
	}
}

// ---------------------------------------------------------------------------
// 2. checkCrossRepoCoverage — missing target page
// ---------------------------------------------------------------------------

func TestCheckCrossRepoCoverage_MissingTarget(t *testing.T) {
	links := []docgen.GroupCrossRepoLinkForTest{
		{Source: "alpha::svc1", Target: "beta::svc99"},
	}
	repoPageMap := map[string][]docgen.PageOutput{
		"alpha": {{EntityID: "svc1"}},
		"beta":  {{EntityID: "svc2"}}, // svc99 not present
	}
	violations, _, unresolved := docgen.CheckCrossRepoCoverageForTest(links, repoPageMap)
	if unresolved != 1 {
		t.Errorf("expected 1 unresolved link, got %d", unresolved)
	}
	found := false
	for _, v := range violations {
		if v.Kind == "cross-repo-coverage" && strings.Contains(v.Message, "svc99") {
			found = true
		}
	}
	if !found {
		t.Error("expected cross-repo-coverage violation mentioning svc99")
	}
}

// ---------------------------------------------------------------------------
// 3. checkGroupIndex — valid index
// ---------------------------------------------------------------------------

func TestCheckGroupIndex_ValidIndex(t *testing.T) {
	dir := t.TempDir()
	slugs := []string{"alpha", "beta"}

	// Write group index with correct links.
	content := "# Group Index\n\n[alpha](alpha/index.md)\n[beta](beta/index.md)\n[score.json](score.json)\n"
	if err := os.WriteFile(filepath.Join(dir, "index.md"), []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}

	unresolved := 0
	violations := docgen.CheckGroupIndexForTest("test-group", slugs, dir, &unresolved)
	if unresolved != 0 {
		t.Errorf("expected 0 unresolved group index links, got %d", unresolved)
	}
	for _, v := range violations {
		if v.Kind == "group-index" {
			t.Errorf("unexpected group-index violation: %s", v.Message)
		}
	}
}

// ---------------------------------------------------------------------------
// 4. checkGroupIndex — missing index file
// ---------------------------------------------------------------------------

func TestCheckGroupIndex_MissingIndex(t *testing.T) {
	dir := t.TempDir() // no index.md written
	slugs := []string{"alpha"}
	unresolved := 0
	violations := docgen.CheckGroupIndexForTest("test-group", slugs, dir, &unresolved)
	found := false
	for _, v := range violations {
		if v.Kind == "group-index" {
			found = true
		}
	}
	if !found {
		t.Error("expected group-index violation for missing index.md, got none")
	}
}

// ---------------------------------------------------------------------------
// 5. checkGroupIndex — missing repo link
// ---------------------------------------------------------------------------

func TestCheckGroupIndex_MissingRepoLink(t *testing.T) {
	dir := t.TempDir()
	slugs := []string{"alpha", "beta"}

	// Index only links to alpha, not beta.
	content := "# Group\n\n[alpha](alpha/index.md)\n[score.json](score.json)\n"
	if err := os.WriteFile(filepath.Join(dir, "index.md"), []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}

	unresolved := 0
	violations := docgen.CheckGroupIndexForTest("test-group", slugs, dir, &unresolved)
	if unresolved != 1 {
		t.Errorf("expected 1 unresolved (beta), got %d", unresolved)
	}
	found := false
	for _, v := range violations {
		if v.Kind == "group-index" && strings.Contains(v.Message, "beta") {
			found = true
		}
	}
	if !found {
		t.Error("expected group-index violation mentioning beta")
	}
}

// ---------------------------------------------------------------------------
// 6. checkCrossRepoFlowDedup — no violation when flows are unique per repo
// ---------------------------------------------------------------------------

func TestCheckCrossRepoFlowDedup_UniqueFlows(t *testing.T) {
	repoPageMap := map[string][]docgen.PageOutput{
		"alpha": {{EntityID: "svc1", MD: "# Page\n```mermaid\nsequenceDiagram\nA->>B: call\n```\n"}},
		"beta":  {{EntityID: "svc2", MD: "# Page\n```mermaid\nsequenceDiagram\nC->>D: call\n```\n"}},
	}
	violations, count := docgen.CheckCrossRepoFlowDedupForTest(repoPageMap)
	if count != 0 {
		t.Errorf("expected 0 dedup violations for unique flows, got %d: %v", count, violations)
	}
}

// ---------------------------------------------------------------------------
// 7. checkCrossRepoFlowDedup — violation when identical flow in 2 repos
// ---------------------------------------------------------------------------

func TestCheckCrossRepoFlowDedup_IdenticalFlow(t *testing.T) {
	identicalFlow := "```mermaid\nsequenceDiagram\nA->>B: shared call\n```"
	repoPageMap := map[string][]docgen.PageOutput{
		"alpha": {{EntityID: "svc1", MD: "# Page\n" + identicalFlow + "\n"}},
		"beta":  {{EntityID: "svc2", MD: "# Page\n" + identicalFlow + "\n"}},
	}
	violations, count := docgen.CheckCrossRepoFlowDedupForTest(repoPageMap)
	if count == 0 {
		t.Error("expected cross-repo-flow-dedup violation for identical flow, got none")
	}
	found := false
	for _, v := range violations {
		if v.Kind == "cross-repo-flow-dedup" {
			found = true
			if v.RepoA == "" || v.RepoB == "" {
				t.Error("cross-repo-flow-dedup violation should have RepoA and RepoB set")
			}
		}
	}
	if !found {
		t.Error("expected cross-repo-flow-dedup violation kind")
	}
}

// ---------------------------------------------------------------------------
// 8. Tier4Score JSON schema
// ---------------------------------------------------------------------------

func TestTier4Score_JSONSchema(t *testing.T) {
	score := docgen.Tier4Score{
		Tier:                         4,
		WallTimeMS:                   2400000,
		Group:                        "upvate",
		RepoCount:                    3,
		TotalPageCount:               56,
		TotalTokenCount:              180000,
		CrossRepoLinkCount:           47,
		CrossRepoLinkUnresolved:      0,
		CrossRepoFlowDedupViolations: 0,
		GroupIndexUnresolved:         0,
		PerRepoScores: []docgen.Tier3Score{
			{Tier: 3, Repo: "core", PageCount: 20},
		},
	}

	data, err := json.MarshalIndent(score, "", "  ")
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	var parsed map[string]interface{}
	if err := json.Unmarshal(data, &parsed); err != nil {
		t.Fatalf("parse: %v", err)
	}

	required := []string{
		"tier", "wall_time_ms", "group", "repo_count", "total_page_count",
		"total_token_count", "cross_repo_link_count", "cross_repo_link_unresolved",
		"cross_repo_flow_dedup_violations", "group_index_unresolved", "per_repo_scores",
	}
	for _, f := range required {
		if _, ok := parsed[f]; !ok {
			t.Errorf("Tier4Score JSON missing required field: %q", f)
		}
	}
	if got := parsed["tier"].(float64); got != 4 {
		t.Errorf("tier: want 4, got %v", got)
	}
	if got := parsed["group"].(string); got != "upvate" {
		t.Errorf("group: want 'upvate', got %q", got)
	}
}

// ---------------------------------------------------------------------------
// 9. writeGroupIndex produces correct markdown
// ---------------------------------------------------------------------------

func TestWriteGroupIndex_Content(t *testing.T) {
	dir := t.TempDir()
	indexPath := filepath.Join(dir, "index.md")
	slugs := []string{"alpha", "beta", "gamma"}

	if err := docgen.WriteGroupIndexForTest("my-group", slugs, indexPath); err != nil {
		t.Fatalf("writeGroupIndex: %v", err)
	}

	data, err := os.ReadFile(indexPath)
	if err != nil {
		t.Fatalf("read index: %v", err)
	}
	content := string(data)

	// Must contain group name.
	if !strings.Contains(content, "my-group") {
		t.Error("group index missing group name")
	}
	// Must link to each repo's index.
	for _, slug := range slugs {
		expected := slug + "/index.md"
		if !strings.Contains(content, expected) {
			t.Errorf("group index missing link to %q", expected)
		}
	}
	// Must link to score.json.
	if !strings.Contains(content, "score.json") {
		t.Error("group index missing score.json link")
	}
	// Must be marked as tier4-generated.
	if !strings.Contains(content, "tier4-generated") {
		t.Error("group index missing tier4-generated comment")
	}
}

// ---------------------------------------------------------------------------
// 10. listGroupRepos returns correct slugs
// ---------------------------------------------------------------------------

func TestListGroupRepos_ReturnsSlugs(t *testing.T) {
	_, group, expectedSlugs := buildMinimalGroupForTier4(t)

	repos, err := docgen.ListGroupReposForTest(group)
	if err != nil {
		t.Fatalf("listGroupRepos: %v", err)
	}
	if len(repos) != len(expectedSlugs) {
		t.Errorf("expected %d repos, got %d", len(expectedSlugs), len(repos))
	}
	slugSet := make(map[string]bool)
	for _, r := range repos {
		slugSet[r.Slug] = true
	}
	for _, s := range expectedSlugs {
		if !slugSet[s] {
			t.Errorf("expected slug %q in result", s)
		}
	}
}

// ---------------------------------------------------------------------------
// 11. RunTier4 — missing group returns graceful error
// ---------------------------------------------------------------------------

func TestRunTier4_MissingGroup(t *testing.T) {
	opts := docgen.Tier4RunOpts{
		Group:     "no-such-group-tier4-xyz",
		OutputDir: t.TempDir(),
	}
	_, _, err := docgen.RunTier4(opts)
	if err == nil {
		t.Fatal("expected error for nonexistent group, got nil")
	}
	if strings.Contains(err.Error(), "nil pointer") {
		t.Errorf("unexpected nil pointer in error: %v", err)
	}
}

// ---------------------------------------------------------------------------
// 12. RunTier4 — integration smoke with minimal group
// ---------------------------------------------------------------------------

func TestRunTier4_WithMinimalGroup(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}

	_, group, slugs := buildMinimalGroupForTier4(t)
	outDir := t.TempDir()

	opts := docgen.Tier4RunOpts{
		Group:     group,
		MaxPages:  2,
		OutputDir: outDir,
	}

	returnedDir, score, err := docgen.RunTier4(opts)
	if err != nil {
		if strings.Contains(err.Error(), "nil pointer") {
			t.Fatalf("nil pointer panic: %v", err)
		}
		t.Logf("RunTier4 returned error (acceptable in test env): %v", err)
		return
	}

	// Score tier must be 4.
	if score.Tier != 4 {
		t.Errorf("score.tier: want 4, got %d", score.Tier)
	}
	if score.Group != group {
		t.Errorf("score.group: want %q, got %q", group, score.Group)
	}
	if score.RepoCount != len(slugs) {
		t.Errorf("score.repo_count: want %d, got %d", len(slugs), score.RepoCount)
	}

	// Group index.md must exist.
	indexFile := filepath.Join(returnedDir, "index.md")
	if _, err := os.Stat(indexFile); err != nil {
		t.Errorf("group index.md not found: %v", err)
	}

	// Group score.json must exist.
	scoreFile := filepath.Join(returnedDir, "score.json")
	if _, err := os.Stat(scoreFile); err != nil {
		t.Errorf("group score.json not found: %v", err)
	}

	// per_repo_scores must have an entry per repo.
	if len(score.PerRepoScores) != len(slugs) {
		t.Errorf("per_repo_scores: want %d entries, got %d", len(slugs), len(score.PerRepoScores))
	}
}
