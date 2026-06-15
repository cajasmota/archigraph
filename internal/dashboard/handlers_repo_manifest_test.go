package dashboard

// handlers_repo_manifest_test.go — unit tests for the Repo Manifest viewer (#1351).
//
// Tests cover:
//   - scanAgentsMD: detection, preview lines, marker injection flag, editor URI
//   - detectLanguages: multi-stack detection
//   - scanDependencyManifests: known file enumeration
//   - scanGrafelState: absent directory returns empty slice
//   - computeQuality: score and signals
//   - buildManifest: smoke test on a temp directory
//   - hasLockFile: identifies known lock files

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// ─────────────────────────────────────────────────────────────────────────────
// scanAgentsMD
// ─────────────────────────────────────────────────────────────────────────────

func TestScanAgentsMD_NoFile(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	info := scanAgentsMD(dir)
	if info.Exists {
		t.Errorf("want Exists=false, got true")
	}
	if info.Filename != "" {
		t.Errorf("want empty Filename, got %q", info.Filename)
	}
}

func TestScanAgentsMD_AgentsMD(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	content := "# Agents\n\nThis repo is indexed by grafel.\n\nMore text here.\n"
	if err := os.WriteFile(filepath.Join(dir, "AGENTS.md"), []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
	info := scanAgentsMD(dir)
	if !info.Exists {
		t.Errorf("want Exists=true")
	}
	if info.Filename != "AGENTS.md" {
		t.Errorf("want Filename=AGENTS.md, got %q", info.Filename)
	}
	if !info.InjectedByGrafel {
		t.Errorf("want InjectedByGrafel=true (marker present in content)")
	}
	if len(info.PreviewLines) == 0 {
		t.Errorf("want non-empty PreviewLines")
	}
	if !strings.HasPrefix(info.EditorURI, "file://") {
		t.Errorf("want EditorURI to start with file://, got %q", info.EditorURI)
	}
}

func TestScanAgentsMD_ClaudeMD_NoMarker(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	content := strings.Repeat("# CLAUDE config\nNo marker here.\n", 5)
	if err := os.WriteFile(filepath.Join(dir, "CLAUDE.md"), []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
	info := scanAgentsMD(dir)
	if !info.Exists {
		t.Errorf("want Exists=true for CLAUDE.md")
	}
	if info.Filename != "CLAUDE.md" {
		t.Errorf("want Filename=CLAUDE.md, got %q", info.Filename)
	}
	if info.InjectedByGrafel {
		t.Errorf("want InjectedByGrafel=false (no marker)")
	}
}

func TestScanAgentsMD_PreviewCappedAt50Lines(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	var sb strings.Builder
	for i := 0; i < 100; i++ {
		sb.WriteString("line\n")
	}
	if err := os.WriteFile(filepath.Join(dir, "AGENTS.md"), []byte(sb.String()), 0o644); err != nil {
		t.Fatal(err)
	}
	info := scanAgentsMD(dir)
	if len(info.PreviewLines) > 50 {
		t.Errorf("want at most 50 preview lines, got %d", len(info.PreviewLines))
	}
}

// ─────────────────────────────────────────────────────────────────────────────
// detectLanguages
// ─────────────────────────────────────────────────────────────────────────────

func TestDetectLanguages_GoOnly(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "go.mod"), []byte("module example.com/m\ngo 1.21\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	langs := detectLanguages(dir)
	if !sliceContains(langs, "go") {
		t.Errorf("want 'go' in languages, got %v", langs)
	}
}

func TestDetectLanguages_Polyglot(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	touch := func(name string) {
		t.Helper()
		if err := os.WriteFile(filepath.Join(dir, name), []byte("{}"), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	touch("go.mod")
	touch("package.json")
	touch("requirements.txt")
	langs := detectLanguages(dir)
	for _, want := range []string{"go", "node", "python"} {
		if !sliceContains(langs, want) {
			t.Errorf("want %q in languages, got %v", want, langs)
		}
	}
}

func TestDetectLanguages_NoDuplicates(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	// Both pyproject.toml and requirements.txt → still only one "python" entry.
	touch := func(name string) {
		t.Helper()
		if err := os.WriteFile(filepath.Join(dir, name), []byte(""), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	touch("pyproject.toml")
	touch("requirements.txt")
	langs := detectLanguages(dir)
	count := 0
	for _, l := range langs {
		if l == "python" {
			count++
		}
	}
	if count != 1 {
		t.Errorf("want exactly 1 'python' entry, got %d in %v", count, langs)
	}
}

// ─────────────────────────────────────────────────────────────────────────────
// scanDependencyManifests
// ─────────────────────────────────────────────────────────────────────────────

func TestScanDependencyManifests_Empty(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	got := scanDependencyManifests(dir)
	if len(got) != 0 {
		t.Errorf("want no manifests, got %v", got)
	}
}

func TestScanDependencyManifests_KnownFiles(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	for _, name := range []string{"go.mod", "go.sum", "package.json"} {
		if err := os.WriteFile(filepath.Join(dir, name), []byte(""), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	got := scanDependencyManifests(dir)
	for _, want := range []string{"go.mod", "go.sum", "package.json"} {
		if !sliceContains(got, want) {
			t.Errorf("want %q in manifests, got %v", want, got)
		}
	}
}

// ─────────────────────────────────────────────────────────────────────────────
// scanGrafelState
// ─────────────────────────────────────────────────────────────────────────────

func TestScanGrafelState_AbsentDir(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	// Use a non-existent path so StateDirForRepo won't find anything.
	got := scanGrafelState(dir)
	// Must return a non-nil slice (possibly empty).
	if got == nil {
		t.Errorf("want non-nil slice, got nil")
	}
}

func TestScanGrafelState_InRepoDir(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	archDir := filepath.Join(dir, ".grafel")
	if err := os.MkdirAll(archDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(archDir, "group.json"), []byte(`{"group":"test"}`), 0o644); err != nil {
		t.Fatal(err)
	}
	got := scanGrafelState(dir)
	if !sliceContains(got, "group.json") && !containsStrPrefix(got, "group.json") {
		t.Errorf("want group.json in state, got %v", got)
	}
}

// ─────────────────────────────────────────────────────────────────────────────
// computeQuality
// ─────────────────────────────────────────────────────────────────────────────

func TestComputeQuality_FullRepo(t *testing.T) {
	t.Parallel()
	r := RepoManifestReply{
		Stack:     "go",
		Languages: []string{"go"},
		AgentsMD: AgentsMDInfo{
			Exists:           true,
			InjectedByGrafel: true,
		},
		GrafelState:         []string{"graph.json"},
		DependencyManifests: []string{"go.mod", "go.sum"},
	}
	signals, score := computeQuality(r)
	if score <= 0 {
		t.Errorf("want positive score, got %d", score)
	}
	if score > 100 {
		t.Errorf("score must be ≤100, got %d", score)
	}
	if len(signals) == 0 {
		t.Errorf("want at least one signal")
	}
	// Check that each signal has a name and points.
	for _, s := range signals {
		if s.Name == "" {
			t.Errorf("signal has empty name")
		}
		if s.Points <= 0 {
			t.Errorf("signal %q has non-positive points: %d", s.Name, s.Points)
		}
	}
}

func TestComputeQuality_EmptyRepo(t *testing.T) {
	t.Parallel()
	r := RepoManifestReply{}
	_, score := computeQuality(r)
	if score != 0 {
		t.Errorf("want 0 for empty manifest, got %d", score)
	}
}

// ─────────────────────────────────────────────────────────────────────────────
// hasLockFile
// ─────────────────────────────────────────────────────────────────────────────

func TestHasLockFile(t *testing.T) {
	t.Parallel()
	cases := []struct {
		manifests []string
		want      bool
	}{
		{[]string{"go.mod", "go.sum"}, true},
		{[]string{"package.json", "yarn.lock"}, true},
		{[]string{"go.mod", "package.json"}, false},
		{[]string{}, false},
	}
	for _, tc := range cases {
		got := hasLockFile(tc.manifests)
		if got != tc.want {
			t.Errorf("hasLockFile(%v) = %v, want %v", tc.manifests, got, tc.want)
		}
	}
}

// ─────────────────────────────────────────────────────────────────────────────
// buildManifest smoke test
// ─────────────────────────────────────────────────────────────────────────────

func TestBuildManifest_Smoke(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	// Set up a minimal Go repo.
	if err := os.WriteFile(filepath.Join(dir, "go.mod"), []byte("module example.com/m\ngo 1.21\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "go.sum"), []byte(""), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "AGENTS.md"), []byte("# Agents\nindexed by grafel\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	m := buildManifest("mygroup", "myrepo", dir)
	if m.Group != "mygroup" {
		t.Errorf("want Group=mygroup, got %q", m.Group)
	}
	if m.Repo != "myrepo" {
		t.Errorf("want Repo=myrepo, got %q", m.Repo)
	}
	if m.AbsPath != dir {
		t.Errorf("want AbsPath=%q, got %q", dir, m.AbsPath)
	}
	if m.Stack != "go" {
		t.Errorf("want Stack=go, got %q", m.Stack)
	}
	if !m.AgentsMD.Exists {
		t.Errorf("want AgentsMD.Exists=true")
	}
	if m.QualityScore <= 0 {
		t.Errorf("want positive quality score, got %d", m.QualityScore)
	}
}

// ─────────────────────────────────────────────────────────────────────────────
// Helpers
// ─────────────────────────────────────────────────────────────────────────────

func sliceContains(slice []string, s string) bool {
	for _, v := range slice {
		if v == s {
			return true
		}
	}
	return false
}

func containsStrPrefix(slice []string, prefix string) bool {
	for _, v := range slice {
		if strings.HasPrefix(v, prefix) {
			return true
		}
	}
	return false
}
