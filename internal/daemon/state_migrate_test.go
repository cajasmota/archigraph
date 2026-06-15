package daemon

import (
	"os"
	"path/filepath"
	"testing"
)

// TestMigrateInRepoState_MovesArtifactsAndCleansRepo verifies the #1626
// migration: a pre-existing in-repo `.grafel/` with a graph is moved
// into the external store, and the repo working tree is left clean.
func TestMigrateInRepoState_MovesArtifactsAndCleansRepo(t *testing.T) {
	t.Setenv(EnvRoot, "")
	home := t.TempDir()
	t.Setenv("GRAFEL_HOME", home)

	repo := t.TempDir()
	legacy := filepath.Join(repo, ".grafel")
	if err := os.MkdirAll(filepath.Join(legacy, "enrichments"), 0o755); err != nil {
		t.Fatal(err)
	}
	mustWrite(t, filepath.Join(legacy, "graph.fb"), "fbdata")
	mustWrite(t, filepath.Join(legacy, "graph.json"), "{}")
	mustWrite(t, filepath.Join(legacy, "enrichments", "e.json"), "{}")
	// Committed manifest must be PRESERVED.
	mustWrite(t, filepath.Join(legacy, "group.json"), `{"group":"g"}`)

	migrated, err := MigrateInRepoState(repo)
	if err != nil {
		t.Fatalf("migrate: %v", err)
	}
	if !migrated {
		t.Fatal("expected migrated=true")
	}

	store := StateDirForRepo(repo)
	for _, n := range []string{"graph.fb", "graph.json", filepath.Join("enrichments", "e.json")} {
		if _, e := os.Stat(filepath.Join(store, n)); e != nil {
			t.Fatalf("expected %s in store: %v", n, e)
		}
	}

	// Repo: generated artifacts gone; group.json kept.
	if _, e := os.Stat(filepath.Join(legacy, "graph.fb")); !os.IsNotExist(e) {
		t.Fatalf("graph.fb still in repo: err=%v", e)
	}
	if _, e := os.Stat(filepath.Join(legacy, "group.json")); e != nil {
		t.Fatalf("group.json should be preserved in repo: %v", e)
	}
}

// TestMigrateInRepoState_CleansEmptyLegacyDir verifies the legacy dir is
// removed entirely when no group.json remains.
func TestMigrateInRepoState_CleansEmptyLegacyDir(t *testing.T) {
	t.Setenv(EnvRoot, "")
	home := t.TempDir()
	t.Setenv("GRAFEL_HOME", home)

	repo := t.TempDir()
	legacy := filepath.Join(repo, ".grafel")
	if err := os.MkdirAll(legacy, 0o755); err != nil {
		t.Fatal(err)
	}
	mustWrite(t, filepath.Join(legacy, "graph.fb"), "fbdata")

	if _, err := MigrateInRepoState(repo); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	if _, e := os.Stat(legacy); !os.IsNotExist(e) {
		t.Fatalf("expected legacy dir removed, err=%v", e)
	}
}

// TestMigrateInRepoState_NoopWhenIsolatedRoot verifies migration is a
// no-op under GRAFEL_DAEMON_ROOT (isolation mode never used in-repo).
func TestMigrateInRepoState_NoopWhenIsolatedRoot(t *testing.T) {
	t.Setenv(EnvRoot, t.TempDir())
	repo := t.TempDir()
	legacy := filepath.Join(repo, ".grafel")
	if err := os.MkdirAll(legacy, 0o755); err != nil {
		t.Fatal(err)
	}
	mustWrite(t, filepath.Join(legacy, "graph.fb"), "fbdata")

	migrated, err := MigrateInRepoState(repo)
	if err != nil {
		t.Fatalf("migrate: %v", err)
	}
	if migrated {
		t.Fatal("expected no migration under isolated root")
	}
	if _, e := os.Stat(filepath.Join(legacy, "graph.fb")); e != nil {
		t.Fatalf("legacy graph.fb should be untouched: %v", e)
	}
}

// TestMigrateInRepoState_NoopWhenStoreAlreadyHasGraph ensures we don't
// clobber a freshly-indexed store with stale in-repo artifacts.
func TestMigrateInRepoState_NoopWhenStoreAlreadyHasGraph(t *testing.T) {
	t.Setenv(EnvRoot, "")
	home := t.TempDir()
	t.Setenv("GRAFEL_HOME", home)

	repo := t.TempDir()
	store := StateDirForRepo(repo)
	if err := os.MkdirAll(store, 0o755); err != nil {
		t.Fatal(err)
	}
	mustWrite(t, filepath.Join(store, "graph.fb"), "fresh")

	legacy := filepath.Join(repo, ".grafel")
	if err := os.MkdirAll(legacy, 0o755); err != nil {
		t.Fatal(err)
	}
	mustWrite(t, filepath.Join(legacy, "graph.fb"), "stale")

	migrated, err := MigrateInRepoState(repo)
	if err != nil {
		t.Fatalf("migrate: %v", err)
	}
	if migrated {
		t.Fatal("expected no migration when store already has a graph")
	}
	b, _ := os.ReadFile(filepath.Join(store, "graph.fb"))
	if string(b) != "fresh" {
		t.Fatalf("store graph.fb was clobbered: %q", string(b))
	}
}

func mustWrite(t *testing.T, path, content string) {
	t.Helper()
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("write %s: %v", path, err)
	}
}

// ---------------------------------------------------------------------------
// #2130 PH1a heal-pass tests
// ---------------------------------------------------------------------------

// TestMigrateToRefStore_HealsUnknownRef_MovesToKnownRef verifies the heal
// path: refs/_unknown/graph.fb is moved to refs/<X>/ when exactly one
// non-_unknown ref dir exists and it is missing a graph file.
func TestMigrateToRefStore_HealsUnknownRef_MovesToKnownRef(t *testing.T) {
	store := t.TempDir()
	slotDir := filepath.Join(store, "myrepo-abc123")
	unknownDir := filepath.Join(slotDir, "refs", "_unknown")
	mainDir := filepath.Join(slotDir, "refs", "main")

	// Partial-migration state: _unknown has the graph; main has only sidecars.
	if err := os.MkdirAll(unknownDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(mainDir, 0o755); err != nil {
		t.Fatal(err)
	}
	mustWrite(t, filepath.Join(unknownDir, "graph.fb"), "fbdata")
	mustWrite(t, filepath.Join(unknownDir, "enrichment-candidates.json"), "[]")
	mustWrite(t, filepath.Join(mainDir, "enrichment-candidates.json"), "[]") // sidecar present, no graph

	if err := MigrateToRefStore(store); err != nil {
		t.Fatalf("MigrateToRefStore: %v", err)
	}

	// graph.fb must now live in main/.
	if _, err := os.Stat(filepath.Join(mainDir, "graph.fb")); err != nil {
		t.Fatalf("graph.fb not in refs/main: %v", err)
	}
	// enrichment-candidates.json from _unknown must have moved too.
	if _, err := os.Stat(filepath.Join(mainDir, "enrichment-candidates.json")); err != nil {
		t.Fatalf("enrichment-candidates.json not in refs/main: %v", err)
	}
	// _unknown dir should be gone.
	if _, err := os.Stat(unknownDir); !os.IsNotExist(err) {
		t.Fatalf("_unknown dir still present after heal: err=%v", err)
	}
}

// TestMigrateToRefStore_HealsUnknownRef_NoopWhenAlreadyMigrated ensures
// the heal pass is a no-op when the target ref already has its own graph.
func TestMigrateToRefStore_HealsUnknownRef_NoopWhenAlreadyMigrated(t *testing.T) {
	store := t.TempDir()
	slotDir := filepath.Join(store, "myrepo-abc123")
	unknownDir := filepath.Join(slotDir, "refs", "_unknown")
	mainDir := filepath.Join(slotDir, "refs", "main")

	if err := os.MkdirAll(unknownDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(mainDir, 0o755); err != nil {
		t.Fatal(err)
	}
	mustWrite(t, filepath.Join(unknownDir, "graph.fb"), "stale-unknown")
	mustWrite(t, filepath.Join(mainDir, "graph.fb"), "fresh-main") // already has graph

	if err := MigrateToRefStore(store); err != nil {
		t.Fatalf("MigrateToRefStore: %v", err)
	}

	// main/graph.fb must remain unchanged.
	b, err := os.ReadFile(filepath.Join(mainDir, "graph.fb"))
	if err != nil {
		t.Fatalf("read main/graph.fb: %v", err)
	}
	if string(b) != "fresh-main" {
		t.Fatalf("main/graph.fb was clobbered: %q", string(b))
	}
	// _unknown/graph.fb must still be present (not moved or removed).
	if _, err := os.Stat(filepath.Join(unknownDir, "graph.fb")); err != nil {
		t.Fatalf("_unknown/graph.fb missing (should be preserved): %v", err)
	}
}

// TestMigrateToRefStore_HealsUnknownRef_NoopWhenMultipleNonUnknownRefs
// verifies the ambiguity guard: when two or more non-_unknown refs exist,
// the heal pass leaves all directories untouched.
func TestMigrateToRefStore_HealsUnknownRef_NoopWhenMultipleNonUnknownRefs(t *testing.T) {
	store := t.TempDir()
	slotDir := filepath.Join(store, "myrepo-abc123")
	unknownDir := filepath.Join(slotDir, "refs", "_unknown")
	mainDir := filepath.Join(slotDir, "refs", "main")
	featDir := filepath.Join(slotDir, "refs", "feat%2Ffoo")

	for _, d := range []string{unknownDir, mainDir, featDir} {
		if err := os.MkdirAll(d, 0o755); err != nil {
			t.Fatal(err)
		}
	}
	mustWrite(t, filepath.Join(unknownDir, "graph.fb"), "unknown-graph")
	mustWrite(t, filepath.Join(mainDir, "enrichment-candidates.json"), "[]")
	mustWrite(t, filepath.Join(featDir, "enrichment-candidates.json"), "[]")

	if err := MigrateToRefStore(store); err != nil {
		t.Fatalf("MigrateToRefStore: %v", err)
	}

	// _unknown/graph.fb must remain — ambiguous, leave alone.
	if _, err := os.Stat(filepath.Join(unknownDir, "graph.fb")); err != nil {
		t.Fatalf("_unknown/graph.fb missing (should be preserved in ambiguous case): %v", err)
	}
	// Neither main nor feat should have received graph.fb.
	if _, err := os.Stat(filepath.Join(mainDir, "graph.fb")); !os.IsNotExist(err) {
		t.Fatalf("main/graph.fb should not exist in ambiguous case: err=%v", err)
	}
	if _, err := os.Stat(filepath.Join(featDir, "graph.fb")); !os.IsNotExist(err) {
		t.Fatalf("feat/graph.fb should not exist in ambiguous case: err=%v", err)
	}
}
