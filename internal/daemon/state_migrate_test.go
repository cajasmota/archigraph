package daemon

import (
	"os"
	"path/filepath"
	"testing"
)

// TestMigrateInRepoState_MovesArtifactsAndCleansRepo verifies the #1626
// migration: a pre-existing in-repo `.archigraph/` with a graph is moved
// into the external store, and the repo working tree is left clean.
func TestMigrateInRepoState_MovesArtifactsAndCleansRepo(t *testing.T) {
	t.Setenv(EnvRoot, "")
	home := t.TempDir()
	t.Setenv("ARCHIGRAPH_HOME", home)

	repo := t.TempDir()
	legacy := filepath.Join(repo, ".archigraph")
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
	t.Setenv("ARCHIGRAPH_HOME", home)

	repo := t.TempDir()
	legacy := filepath.Join(repo, ".archigraph")
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
// no-op under ARCHIGRAPH_DAEMON_ROOT (isolation mode never used in-repo).
func TestMigrateInRepoState_NoopWhenIsolatedRoot(t *testing.T) {
	t.Setenv(EnvRoot, t.TempDir())
	repo := t.TempDir()
	legacy := filepath.Join(repo, ".archigraph")
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
	t.Setenv("ARCHIGRAPH_HOME", home)

	repo := t.TempDir()
	store := StateDirForRepo(repo)
	if err := os.MkdirAll(store, 0o755); err != nil {
		t.Fatal(err)
	}
	mustWrite(t, filepath.Join(store, "graph.fb"), "fresh")

	legacy := filepath.Join(repo, ".archigraph")
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
