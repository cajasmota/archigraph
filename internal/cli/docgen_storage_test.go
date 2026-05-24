package cli

// Tests for the storage-discipline helpers introduced by #2190:
//   - `archigraph docgen migrate-in-repo`
//   - `archigraph docgen audit`
//
// Fixtures use synthetic ("client-fixture-X") directory names — no real
// client or product names appear in any test path.

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/cajasmota/archigraph/internal/registry"
)

// makeFixtureGroup creates a synthetic group config under tmpDir that
// registers two repos: client-fixture-a and client-fixture-b.
// Each repo gets a .git directory so isDocgenOutput won't confuse them
// with store directories.
func makeFixtureGroup(t *testing.T, tmpDir string) (cfgPath string, repoA, repoB string) {
	t.Helper()

	repoA = filepath.Join(tmpDir, "client-fixture-a")
	repoB = filepath.Join(tmpDir, "client-fixture-b")
	for _, r := range []string{repoA, repoB} {
		if err := os.MkdirAll(filepath.Join(r, ".git"), 0o755); err != nil {
			t.Fatalf("create repo dir: %v", err)
		}
	}

	cfg := registry.GroupConfig{
		Name: "fixture-group",
		Repos: []registry.Repo{
			{Slug: "client-fixture-a", Path: repoA},
			{Slug: "client-fixture-b", Path: repoB},
		},
	}
	cfgPath = filepath.Join(tmpDir, "fixture-group.fleet.json")
	data, _ := json.Marshal(cfg)
	if err := os.WriteFile(cfgPath, data, 0o644); err != nil {
		t.Fatalf("write group config: %v", err)
	}
	return cfgPath, repoA, repoB
}

// plantDocgenMarker writes one of the heuristic marker files into dir/docs/
// so that isDocgenOutput returns true for that directory.
func plantDocgenMarker(t *testing.T, repoDir, markerFile string) string {
	t.Helper()
	docsDir := filepath.Join(repoDir, "docs")
	if err := os.MkdirAll(docsDir, 0o755); err != nil {
		t.Fatalf("create docs dir: %v", err)
	}
	markerPath := filepath.Join(docsDir, markerFile)
	if err := os.WriteFile(markerPath, []byte("{}"), 0o644); err != nil {
		t.Fatalf("write marker: %v", err)
	}
	return docsDir
}

// ---------------------------------------------------------------------------
// isDocgenOutput unit tests
// ---------------------------------------------------------------------------

func TestIsDocgenOutput_NoMarkers(t *testing.T) {
	dir := t.TempDir()
	if isDocgenOutput(dir) {
		t.Error("empty dir should not be detected as docgen output")
	}
}

func TestIsDocgenOutput_PlanMd(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, ".plan.md"), []byte("# plan"), 0o644); err != nil {
		t.Fatal(err)
	}
	if !isDocgenOutput(dir) {
		t.Error("dir with .plan.md should be detected as docgen output")
	}
}

func TestIsDocgenOutput_InventoryJSON(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, ".inventory.json"), []byte("{}"), 0o644); err != nil {
		t.Fatal(err)
	}
	if !isDocgenOutput(dir) {
		t.Error("dir with .inventory.json should be detected as docgen output")
	}
}

func TestIsDocgenOutput_MetadataJSON(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, ".metadata.json"), []byte("{}"), 0o644); err != nil {
		t.Fatal(err)
	}
	if !isDocgenOutput(dir) {
		t.Error("dir with .metadata.json should be detected as docgen output")
	}
}

func TestIsDocgenOutput_UnrelatedDocsDir(t *testing.T) {
	// A docs/ with only README.md should NOT be flagged.
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "README.md"), []byte("# docs"), 0o644); err != nil {
		t.Fatal(err)
	}
	if isDocgenOutput(dir) {
		t.Error("docs/ with only README.md should not be flagged as docgen output")
	}
}

// ---------------------------------------------------------------------------
// findInRepoDocgenDirs unit tests
// ---------------------------------------------------------------------------

func TestFindInRepoDocgenDirs_NonePresent(t *testing.T) {
	tmpDir := t.TempDir()
	_, repoA, repoB := makeFixtureGroup(t, tmpDir)
	// repos exist but have no docs/ at all
	_ = repoA
	_ = repoB

	cfgPath := filepath.Join(tmpDir, "fixture-group.fleet.json")
	cfg, err := registry.LoadGroupConfig(cfgPath)
	if err != nil {
		t.Fatalf("load config: %v", err)
	}
	dirs := findInRepoDocgenDirs(cfg)
	if len(dirs) != 0 {
		t.Errorf("expected 0 dirs, got %d: %v", len(dirs), dirs)
	}
}

func TestFindInRepoDocgenDirs_OnePresent(t *testing.T) {
	tmpDir := t.TempDir()
	cfgPath, repoA, _ := makeFixtureGroup(t, tmpDir)
	plantDocgenMarker(t, repoA, ".plan.md")

	cfg, err := registry.LoadGroupConfig(cfgPath)
	if err != nil {
		t.Fatalf("load config: %v", err)
	}
	dirs := findInRepoDocgenDirs(cfg)
	if len(dirs) != 1 {
		t.Fatalf("expected 1 dir, got %d: %v", len(dirs), dirs)
	}
	if !strings.HasSuffix(dirs[0], filepath.Join("client-fixture-a", "docs")) {
		t.Errorf("unexpected dir: %s", dirs[0])
	}
}

func TestFindInRepoDocgenDirs_BothPresent(t *testing.T) {
	tmpDir := t.TempDir()
	cfgPath, repoA, repoB := makeFixtureGroup(t, tmpDir)
	plantDocgenMarker(t, repoA, ".inventory.json")
	plantDocgenMarker(t, repoB, ".metadata.json")

	cfg, err := registry.LoadGroupConfig(cfgPath)
	if err != nil {
		t.Fatalf("load config: %v", err)
	}
	dirs := findInRepoDocgenDirs(cfg)
	if len(dirs) != 2 {
		t.Fatalf("expected 2 dirs, got %d: %v", len(dirs), dirs)
	}
}

// ---------------------------------------------------------------------------
// migrate-in-repo: move happens when confirmed
// ---------------------------------------------------------------------------

func TestMigrateInRepo_MovesDir(t *testing.T) {
	tmpDir := t.TempDir()
	cfgPath, repoA, _ := makeFixtureGroup(t, tmpDir)

	// Plant docgen output inside repoA.
	srcDocs := plantDocgenMarker(t, repoA, ".plan.md")
	// Also write a real doc file so we can verify content moved.
	if err := os.WriteFile(filepath.Join(srcDocs, "overview.md"), []byte("# overview"), 0o644); err != nil {
		t.Fatal(err)
	}

	// Override ARCHIGRAPH_HOME to a temp dir so we don't touch real user state.
	storeRoot := filepath.Join(tmpDir, "store")
	t.Setenv("ARCHIGRAPH_HOME", storeRoot)

	// Build the cobra command tree.
	root := newRoot()
	_ = cfgPath

	// We need to invoke migrate-in-repo --group fixture-group --yes
	// But the group config is at a non-standard path. Seed the registry.
	seedRegistry(t, tmpDir, cfgPath)

	root.SetArgs([]string{"docgen", "migrate-in-repo", "--group", "fixture-group", "--yes"})
	if err := root.Execute(); err != nil {
		t.Fatalf("migrate-in-repo failed: %v", err)
	}

	// Source should be gone.
	if _, err := os.Stat(srcDocs); !os.IsNotExist(err) {
		t.Errorf("source docs dir should have been removed, stat err: %v", err)
	}

	// Target should exist with the overview file.
	targetDocs := filepath.Join(storeRoot, "docs", "fixture-group", "client-fixture-a")
	if _, err := os.Stat(filepath.Join(targetDocs, "overview.md")); err != nil {
		t.Errorf("overview.md should exist in target: %v", err)
	}
}

// ---------------------------------------------------------------------------
// migrate-in-repo: idempotent (target already exists → skip)
// ---------------------------------------------------------------------------

func TestMigrateInRepo_IdempotentSkipsExisting(t *testing.T) {
	tmpDir := t.TempDir()
	cfgPath, repoA, _ := makeFixtureGroup(t, tmpDir)
	srcDocs := plantDocgenMarker(t, repoA, ".plan.md")

	storeRoot := filepath.Join(tmpDir, "store")
	t.Setenv("ARCHIGRAPH_HOME", storeRoot)

	// Pre-create the target so the idempotency guard triggers.
	targetDocs := filepath.Join(storeRoot, "docs", "fixture-group", "client-fixture-a")
	if err := os.MkdirAll(targetDocs, 0o755); err != nil {
		t.Fatal(err)
	}

	seedRegistry(t, tmpDir, cfgPath)

	root := newRoot()
	root.SetArgs([]string{"docgen", "migrate-in-repo", "--group", "fixture-group", "--yes"})
	if err := root.Execute(); err != nil {
		t.Fatalf("migrate-in-repo failed: %v", err)
	}

	// Source should STILL exist because the target was already there.
	if _, err := os.Stat(srcDocs); err != nil {
		t.Errorf("source docs dir should NOT have been removed (target existed): %v", err)
	}
}

// ---------------------------------------------------------------------------
// audit: detects without moving
// ---------------------------------------------------------------------------

func TestDocgenAudit_ReportsWithoutMoving(t *testing.T) {
	tmpDir := t.TempDir()
	cfgPath, repoA, _ := makeFixtureGroup(t, tmpDir)
	srcDocs := plantDocgenMarker(t, repoA, ".inventory.json")

	storeRoot := filepath.Join(tmpDir, "store")
	t.Setenv("ARCHIGRAPH_HOME", storeRoot)
	seedRegistry(t, tmpDir, cfgPath)

	root := newRoot()
	var out strings.Builder
	root.SetOut(&out)
	root.SetArgs([]string{"docgen", "audit", "--group", "fixture-group"})
	err := root.Execute()

	// audit returns a non-nil error (exit code 1) when offenders found.
	if err == nil {
		t.Error("audit should return non-nil error when offenders found")
	}

	// Source should still exist (audit never moves).
	if _, statErr := os.Stat(srcDocs); statErr != nil {
		t.Errorf("audit should not have moved source: %v", statErr)
	}

	output := out.String()
	if !strings.Contains(output, "client-fixture-a") {
		t.Errorf("audit output should mention the offending repo; got: %s", output)
	}
}

func TestDocgenAudit_CleanGroupReturnsNil(t *testing.T) {
	tmpDir := t.TempDir()
	cfgPath, _, _ := makeFixtureGroup(t, tmpDir)
	// No docgen markers planted.

	storeRoot := filepath.Join(tmpDir, "store")
	t.Setenv("ARCHIGRAPH_HOME", storeRoot)
	seedRegistry(t, tmpDir, cfgPath)

	root := newRoot()
	root.SetArgs([]string{"docgen", "audit", "--group", "fixture-group"})
	if err := root.Execute(); err != nil {
		t.Errorf("audit on clean group should return nil, got: %v", err)
	}
}

// ---------------------------------------------------------------------------
// doctor --audit-docs integration
// ---------------------------------------------------------------------------

func TestDoctorAuditDocs_ReportsOffenders(t *testing.T) {
	tmpDir := t.TempDir()
	cfgPath, repoA, _ := makeFixtureGroup(t, tmpDir)
	plantDocgenMarker(t, repoA, ".plan.md")

	storeRoot := filepath.Join(tmpDir, "store")
	t.Setenv("ARCHIGRAPH_HOME", storeRoot)
	seedRegistry(t, tmpDir, cfgPath)

	root := newRoot()
	var out strings.Builder
	root.SetOut(&out)
	root.SetArgs([]string{"doctor", "--audit-docs"})
	// doctor returns nil even with offenders (it's a report command).
	if err := root.Execute(); err != nil {
		t.Logf("doctor --audit-docs returned error (may be expected): %v", err)
	}

	output := out.String()
	if !strings.Contains(output, "Storage Discipline Audit") {
		t.Errorf("expected audit header in output; got: %s", output)
	}
}

// ---------------------------------------------------------------------------
// DocsDirFor helper
// ---------------------------------------------------------------------------

func TestDocsDirFor(t *testing.T) {
	storeRoot := filepath.Join(t.TempDir(), "store")
	t.Setenv("ARCHIGRAPH_HOME", storeRoot)

	got, err := DocsDirFor("my-group")
	if err != nil {
		t.Fatalf("DocsDirFor: %v", err)
	}
	want := filepath.Join(storeRoot, "docs", "my-group")
	if got != want {
		t.Errorf("DocsDirFor = %q, want %q", got, want)
	}
}

// ---------------------------------------------------------------------------
// Test helpers
// ---------------------------------------------------------------------------

// seedRegistry writes a minimal registry.json pointing to cfgPath so that
// resolveGroup("fixture-group") works in tests.
func seedRegistry(t *testing.T, tmpDir, cfgPath string) {
	t.Helper()
	storeRoot := os.Getenv("ARCHIGRAPH_HOME")
	if storeRoot == "" {
		storeRoot = filepath.Join(tmpDir, "store")
	}
	if err := os.MkdirAll(storeRoot, 0o755); err != nil {
		t.Fatalf("create store root: %v", err)
	}
	regPath := filepath.Join(storeRoot, "registry.json")
	reg := map[string]interface{}{
		"version": 1,
		"groups": []map[string]interface{}{
			{"name": "fixture-group", "config_path": cfgPath},
		},
	}
	data, _ := json.Marshal(reg)
	if err := os.WriteFile(regPath, data, 0o644); err != nil {
		t.Fatalf("write registry: %v", err)
	}
}
