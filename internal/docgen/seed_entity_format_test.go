// Package docgen_test — integration tests for #1826: --seed-entity accepts
// both raw hex and prefixed (group::<hex>) forms.
package docgen_test

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/cajasmota/grafel/internal/daemon"
	"github.com/cajasmota/grafel/internal/docgen"
)

// ---------------------------------------------------------------------------
// Fixture helpers
// ---------------------------------------------------------------------------

// buildSeedEntityFixture creates a minimal GRAFEL_HOME + GRAFEL_DAEMON_ROOT
// with a single group containing one repo with one entity whose ID is rawHex.
// It returns the group name. The graph.json is placed at the path that
// daemon.StateDirForRepo resolves to under GRAFEL_DAEMON_ROOT.
func buildSeedEntityFixture(t *testing.T, rawHex string) (group string) {
	t.Helper()
	archHome := t.TempDir()
	daemonRoot := t.TempDir()
	group = "grafel"

	t.Setenv("GRAFEL_HOME", archHome)
	t.Setenv(daemon.EnvRoot, daemonRoot)

	xdgConfigHome := filepath.Join(archHome, "xdg-config")
	t.Setenv("XDG_CONFIG_HOME", xdgConfigHome)

	cfgDir := filepath.Join(xdgConfigHome, "grafel")
	if err := os.MkdirAll(cfgDir, 0o755); err != nil {
		t.Fatalf("mkdir cfgDir: %v", err)
	}

	// Use a simple unique repo path inside archHome so the hash is stable.
	repoPath := filepath.Join(archHome, "fake-repo")
	if err := os.MkdirAll(repoPath, 0o755); err != nil {
		t.Fatalf("mkdir repoPath: %v", err)
	}

	// Write a minimal graph.json at the daemon.StateDirForRepo location.
	stateDir := daemon.StateDirForRepo(repoPath)
	if err := os.MkdirAll(stateDir, 0o755); err != nil {
		t.Fatalf("mkdir stateDir %s: %v", stateDir, err)
	}

	entity := map[string]interface{}{
		"id":          rawHex,
		"name":        "SeedFunc",
		"kind":        "function",
		"source_file": "pkg/seed.go",
		"start_line":  1,
		"end_line":    42,
		"language":    "go",
	}
	graphDoc := map[string]interface{}{
		"version":       1,
		"repo":          repoPath,
		"entities":      []interface{}{entity},
		"relationships": []interface{}{},
	}
	graphBytes, err := json.Marshal(graphDoc)
	if err != nil {
		t.Fatalf("marshal graph: %v", err)
	}
	if err := os.WriteFile(filepath.Join(stateDir, "graph.json"), graphBytes, 0o644); err != nil {
		t.Fatalf("write graph.json: %v", err)
	}

	// Write the fleet config pointing at the repo.
	groupCfg := map[string]interface{}{
		"name": group,
		"repos": []map[string]interface{}{
			{"slug": "core", "path": repoPath},
		},
	}
	cfgBytes, cfgErr := json.Marshal(groupCfg)
	if cfgErr != nil {
		t.Fatalf("marshal group config: %v", cfgErr)
	}
	if err := os.WriteFile(filepath.Join(cfgDir, group+".fleet.json"), cfgBytes, 0o644); err != nil {
		t.Fatalf("write group config: %v", err)
	}

	return group
}

// ---------------------------------------------------------------------------
// Integration tests — full Run() path (#1826)
// ---------------------------------------------------------------------------

// TestSeedEntity_RawHex checks that a raw hex ID resolves correctly (regression escape).
func TestSeedEntity_RawHex(t *testing.T) {
	const rawHex = "7a349f6cd77984c9"
	group := buildSeedEntityFixture(t, rawHex)

	_, _, score, err := docgen.Run(docgen.RunOpts{
		Group:        group,
		SeedEntityID: rawHex, // raw hex — the form that always worked
		Section:      "overview",
		OutputDir:    t.TempDir(),
	})
	if err != nil {
		t.Fatalf("Run with raw hex: %v", err)
	}
	if !score.SeedEntityFound {
		t.Errorf("seed_entity_found: want true (raw hex), got false")
	}
}

// TestSeedEntity_GrafelPrefixed checks that "grafel::<hex>" resolves —
// this was the broken form before #1826.
func TestSeedEntity_GrafelPrefixed(t *testing.T) {
	const rawHex = "7a349f6cd77984c9"
	group := buildSeedEntityFixture(t, rawHex)

	_, _, score, err := docgen.Run(docgen.RunOpts{
		Group:        group,
		SeedEntityID: "grafel::" + rawHex, // prefixed form from grafel_find
		Section:      "overview",
		OutputDir:    t.TempDir(),
	})
	if err != nil {
		t.Fatalf("Run with prefixed grafel:: form: %v", err)
	}
	if !score.SeedEntityFound {
		t.Errorf("seed_entity_found: want true (grafel:: prefix), got false — #1826 regression")
	}
}

// TestSeedEntity_ArbitraryGroupPrefixed checks that "<group>::<hex>" resolves
// for a repo-specific group name (e.g. acme-core).
func TestSeedEntity_ArbitraryGroupPrefixed(t *testing.T) {
	const rawHex = "7a349f6cd77984c9"
	group := buildSeedEntityFixture(t, rawHex)

	_, _, score, err := docgen.Run(docgen.RunOpts{
		Group:        group,
		SeedEntityID: "acme-core::" + rawHex, // any group:: prefix must work
		Section:      "overview",
		OutputDir:    t.TempDir(),
	})
	if err != nil {
		t.Fatalf("Run with prefixed acme-core:: form: %v", err)
	}
	if !score.SeedEntityFound {
		t.Errorf("seed_entity_found: want true (acme-core:: prefix), got false")
	}
}

// TestSeedEntity_InvalidPrefixedForm checks that "grafel::" (empty RHS) returns
// a clear error rather than silently producing found:false.
func TestSeedEntity_InvalidPrefixedForm(t *testing.T) {
	const rawHex = "7a349f6cd77984c9"
	group := buildSeedEntityFixture(t, rawHex)

	_, _, _, err := docgen.Run(docgen.RunOpts{
		Group:        group,
		SeedEntityID: "grafel::", // empty RHS
		Section:      "overview",
		OutputDir:    t.TempDir(),
	})
	if err == nil {
		t.Fatal("expected error for 'grafel::', got nil")
	}
}
