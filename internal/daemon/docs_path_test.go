package daemon

import (
	"os"
	"path/filepath"
	"testing"
)

func TestDocsDir_HonoursGrafelHome(t *testing.T) {
	home := t.TempDir()
	t.Setenv("GRAFEL_HOME", home)

	want := filepath.Join(home, "docs")
	if got := DocsDir(); got != want {
		t.Fatalf("DocsDir() = %q, want %q", got, want)
	}
	if got, want := RepoDocsDir("demo", "api"), filepath.Join(home, "docs", "demo", "api"); got != want {
		t.Fatalf("RepoDocsDir = %q, want %q", got, want)
	}
	if got, want := BusinessDocsDir("demo"), filepath.Join(home, "docs", "demo", "business"); got != want {
		t.Fatalf("BusinessDocsDir = %q, want %q", got, want)
	}
}

func TestMigrateInRepoDocs_MovesSkillOutputToStore(t *testing.T) {
	home := t.TempDir()
	t.Setenv("GRAFEL_HOME", home)

	repo := t.TempDir()
	docs := filepath.Join(repo, "docs")
	if err := os.MkdirAll(filepath.Join(docs, "modules", "core"), 0o755); err != nil {
		t.Fatal(err)
	}
	mustWriteDoc(t, filepath.Join(docs, "overview.md"), "# overview\n")
	mustWriteDoc(t, filepath.Join(docs, "modules", "core", "README.md"), "# core\n")

	migrated, err := MigrateInRepoDocs("demo", "api", repo)
	if err != nil {
		t.Fatalf("migrate err: %v", err)
	}
	if !migrated {
		t.Fatal("expected migrated=true")
	}

	store := RepoDocsDir("demo", "api")
	if _, err := os.Stat(filepath.Join(store, "overview.md")); err != nil {
		t.Fatalf("overview.md not in store: %v", err)
	}
	if _, err := os.Stat(filepath.Join(store, "modules", "core", "README.md")); err != nil {
		t.Fatalf("module readme not in store: %v", err)
	}
	if _, err := os.Stat(docs); err == nil {
		// The legacy dir should be moved entirely (rename) — leftover is OK as
		// long as it's empty, but on a single-fs tempdir rename removes it.
		t.Logf("legacy docs/ still exists (acceptable if empty)")
	}
}

func TestMigrateInRepoDocs_SkipsHandAuthoredDocs(t *testing.T) {
	home := t.TempDir()
	t.Setenv("GRAFEL_HOME", home)

	repo := t.TempDir()
	docs := filepath.Join(repo, "docs")
	if err := os.MkdirAll(docs, 0o755); err != nil {
		t.Fatal(err)
	}
	// No overview.md, no modules/ — looks like hand-authored docs.
	mustWriteDoc(t, filepath.Join(docs, "README.md"), "# hand-authored\n")

	migrated, err := MigrateInRepoDocs("demo", "api", repo)
	if err != nil {
		t.Fatalf("migrate err: %v", err)
	}
	if migrated {
		t.Fatal("expected migrated=false for hand-authored docs")
	}
	if _, err := os.Stat(filepath.Join(docs, "README.md")); err != nil {
		t.Fatalf("hand-authored README was disturbed: %v", err)
	}
}

func TestMigrateInRepoDocs_SkipsWhenStoreHasContent(t *testing.T) {
	home := t.TempDir()
	t.Setenv("GRAFEL_HOME", home)

	repo := t.TempDir()
	docs := filepath.Join(repo, "docs")
	if err := os.MkdirAll(docs, 0o755); err != nil {
		t.Fatal(err)
	}
	mustWriteDoc(t, filepath.Join(docs, "overview.md"), "# legacy\n")

	store := RepoDocsDir("demo", "api")
	if err := os.MkdirAll(store, 0o755); err != nil {
		t.Fatal(err)
	}
	mustWriteDoc(t, filepath.Join(store, "overview.md"), "# store-wins\n")

	migrated, err := MigrateInRepoDocs("demo", "api", repo)
	if err != nil {
		t.Fatalf("migrate err: %v", err)
	}
	if migrated {
		t.Fatal("expected migrated=false when store already has docs")
	}
	data, _ := os.ReadFile(filepath.Join(store, "overview.md"))
	if string(data) != "# store-wins\n" {
		t.Fatalf("store overview was overwritten: %q", data)
	}
}

func mustWriteDoc(t *testing.T, path, body string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
}
