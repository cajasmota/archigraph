package hooks

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func gitRepo(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	if err := os.MkdirAll(filepath.Join(dir, ".git", "hooks"), 0o755); err != nil {
		t.Fatal(err)
	}
	return dir
}

func TestInstallIdempotent(t *testing.T) {
	repo := gitRepo(t)
	if err := Install(repo, "/usr/local/bin/grafel"); err != nil {
		t.Fatal(err)
	}
	first, err := os.ReadFile(filepath.Join(repo, ".git/hooks/post-commit"))
	if err != nil {
		t.Fatal(err)
	}
	if err := Install(repo, "/usr/local/bin/grafel"); err != nil {
		t.Fatal(err)
	}
	second, _ := os.ReadFile(filepath.Join(repo, ".git/hooks/post-commit"))
	if string(first) != string(second) {
		t.Fatalf("non-idempotent install:\n%s\n---\n%s", first, second)
	}
	// Every hook installed.
	for _, name := range HookNames {
		b, err := os.ReadFile(filepath.Join(repo, ".git/hooks", name))
		if err != nil {
			t.Fatal(err)
		}
		if !strings.Contains(string(b), MarkerBegin) {
			t.Fatalf("%s missing marker: %s", name, b)
		}
	}
}

// TestBlockForWorktreeResolution asserts that every generated hook script
// uses `git rev-parse --show-toplevel` at runtime to discover the repo/worktree
// path, and does NOT hardcode the registered repo path. This ensures that when
// the hook fires inside a worktree it indexes that worktree's branch, not main.
func TestBlockForWorktreeResolution(t *testing.T) {
	const binPath = "/usr/local/bin/grafel"
	const registeredRepo = "/home/user/myrepo"
	const group = "mygroup"

	for _, hookName := range HookNames {
		t.Run(hookName+"/no-group", func(t *testing.T) {
			block := BlockFor(hookName, binPath, registeredRepo)
			if !strings.Contains(block, `git rev-parse --show-toplevel`) {
				t.Errorf("hook %s: missing git rev-parse --show-toplevel:\n%s", hookName, block)
			}
			if !strings.Contains(block, `index --async "$_ag_repo"`) {
				t.Errorf("hook %s: missing `index --async \"$_ag_repo\"`:\n%s", hookName, block)
			}
			// Must NOT hardcode the registered repo path as an index argument.
			if strings.Contains(block, `index --async "`+registeredRepo+`"`) {
				t.Errorf("hook %s: hardcoded repo path found — worktrees would be mis-indexed:\n%s", hookName, block)
			}
			assertRebaseGuard(t, hookName, block)
		})

		t.Run(hookName+"/with-group", func(t *testing.T) {
			block := BlockFor(hookName, binPath, registeredRepo, group)
			if !strings.Contains(block, `git rev-parse --show-toplevel`) {
				t.Errorf("hook %s+group: missing git rev-parse --show-toplevel:\n%s", hookName, block)
			}
			if !strings.Contains(block, `index --async "$_ag_repo"`) {
				t.Errorf("hook %s+group: missing `index --async \"$_ag_repo\"`:\n%s", hookName, block)
			}
			if strings.Contains(block, `index --async "`+registeredRepo+`"`) {
				t.Errorf("hook %s+group: hardcoded repo path found — worktrees would be mis-indexed:\n%s", hookName, block)
			}
			assertRebaseGuard(t, hookName, block)
			// Links pass must be present ONLY for post-merge (#3366): a merge
			// is the point cross-repo wiring can change; ordinary commits and
			// checkouts skip it to stay cheap.
			hasLinks := strings.Contains(block, `links pass "`+group+`"`)
			if hookName == "post-merge" && !hasLinks {
				t.Errorf("hook %s+group: links pass line missing:\n%s", hookName, block)
			}
			if hookName != "post-merge" && hasLinks {
				t.Errorf("hook %s+group: links pass must only appear in post-merge:\n%s", hookName, block)
			}
		})
	}
}

// assertRebaseGuard verifies the managed block refuses to fire mid-rebase so
// that replaying N commits during a rebase does not trigger N reindexes (#3366).
func assertRebaseGuard(t *testing.T, hookName, block string) {
	t.Helper()
	for _, want := range []string{
		`_ag_rbm="$(git rev-parse --git-path rebase-merge 2>/dev/null)"`,
		`_ag_rba="$(git rev-parse --git-path rebase-apply 2>/dev/null)"`,
		`[ ! -d "$_ag_rbm" ] && [ ! -d "$_ag_rba" ]`,
	} {
		if !strings.Contains(block, want) {
			t.Errorf("hook %s: missing rebase-guard fragment %q:\n%s", hookName, want, block)
		}
	}
}

// TestInstallWorktreeResolution verifies the generated hook file (after a real
// Install call) uses runtime worktree resolution and preserves the managed markers.
func TestInstallWorktreeResolution(t *testing.T) {
	repo := gitRepo(t)
	const binPath = "/usr/local/bin/grafel"
	if err := Install(repo, binPath, "mygroup"); err != nil {
		t.Fatal(err)
	}
	for _, name := range HookNames {
		b, err := os.ReadFile(filepath.Join(repo, ".git/hooks", name))
		if err != nil {
			t.Fatalf("%s: %v", name, err)
		}
		script := string(b)
		if !strings.Contains(script, `git rev-parse --show-toplevel`) {
			t.Errorf("%s: runtime worktree resolution missing:\n%s", name, script)
		}
		if !strings.Contains(script, `index --async "$_ag_repo"`) {
			t.Errorf("%s: async index with runtime var missing:\n%s", name, script)
		}
		if !strings.Contains(script, MarkerBegin) || !strings.Contains(script, MarkerEnd) {
			t.Errorf("%s: managed markers missing:\n%s", name, script)
		}
	}
}

func TestInstallPreservesUserContent(t *testing.T) {
	repo := gitRepo(t)
	user := "#!/bin/sh\necho user-script\n"
	hookPath := filepath.Join(repo, ".git/hooks/post-commit")
	if err := os.WriteFile(hookPath, []byte(user), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := Install(repo, "/bin/grafel"); err != nil {
		t.Fatal(err)
	}
	got, _ := os.ReadFile(hookPath)
	if !strings.Contains(string(got), "echo user-script") {
		t.Fatalf("user content lost: %s", got)
	}
	if err := Uninstall(repo); err != nil {
		t.Fatal(err)
	}
	got, _ = os.ReadFile(hookPath)
	if !strings.Contains(string(got), "echo user-script") {
		t.Fatalf("user content lost on uninstall: %s", got)
	}
	if strings.Contains(string(got), MarkerBegin) {
		t.Fatalf("marker still present after uninstall: %s", got)
	}
}

func TestStripBlockUnterminated(t *testing.T) {
	got := stripBlock("a\n" + MarkerBegin + "\noops")
	if strings.Contains(got, MarkerBegin) {
		t.Fatalf("unterminated block not stripped: %q", got)
	}
}
