package cli

import (
	"path/filepath"
	"testing"

	"github.com/cajasmota/grafel/internal/registry"
)

// TestDefaultGroupName_ContainerFolderForGroup verifies the #5338 fix: a group
// of sibling repos under a container folder (ivivo/backend, ivivo/frontend)
// defaults the group name to the CONTAINER folder ("ivivo"), not a child repo.
func TestDefaultGroupName_ContainerFolderForGroup(t *testing.T) {
	root := filepath.Join(string(filepath.Separator), "home", "me", "ivivo")
	repos := []registry.Repo{
		{Path: filepath.Join(root, "backend")},
		{Path: filepath.Join(root, "frontend")},
	}
	if got := defaultGroupName(repos); got != "ivivo" {
		t.Fatalf("want container folder %q, got %q", "ivivo", got)
	}
}

// TestDefaultGroupName_SingleRepoUsesRepoName verifies a single selected repo
// still defaults to the repo's own basename.
func TestDefaultGroupName_SingleRepoUsesRepoName(t *testing.T) {
	repos := []registry.Repo{
		{Path: filepath.Join(string(filepath.Separator), "home", "me", "ivivo", "backend")},
	}
	if got := defaultGroupName(repos); got != "backend" {
		t.Fatalf("want repo basename %q, got %q", "backend", got)
	}
}

// TestDefaultGroupName_MonorepoPackagesUseRoot verifies monorepo package paths
// (all under one root) default to the monorepo root's basename.
func TestDefaultGroupName_MonorepoPackagesUseRoot(t *testing.T) {
	root := filepath.Join(string(filepath.Separator), "src", "myapp")
	repos := []registry.Repo{
		{Path: filepath.Join(root, "api")},
		{Path: filepath.Join(root, "web")},
		{Path: filepath.Join(root, "shared")},
	}
	if got := defaultGroupName(repos); got != "myapp" {
		t.Fatalf("want monorepo root %q, got %q", "myapp", got)
	}
}

// TestDefaultGroupName_DisjointParentsFallBack verifies that repos without a
// single shared parent fall back to the first repo's container folder rather
// than picking an unrelated ancestor.
func TestDefaultGroupName_DisjointParentsFallBack(t *testing.T) {
	repos := []registry.Repo{
		{Path: filepath.Join(string(filepath.Separator), "a", "alpha", "svc1")},
		{Path: filepath.Join(string(filepath.Separator), "b", "beta", "svc2")},
	}
	if got := defaultGroupName(repos); got != "alpha" {
		t.Fatalf("want first repo's container %q, got %q", "alpha", got)
	}
}
