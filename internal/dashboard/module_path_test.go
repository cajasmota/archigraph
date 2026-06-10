package dashboard

import "testing"

// Mirrors the #4698 acceptance fixture: a monorepo group config with module
// roots (packages/api, packages/worker) attributes records to their longest
// matching module root, and files outside any root get an empty module_path.
func TestModulePathFor(t *testing.T) {
	mono := map[string][]string{
		"mono": {"packages/api", "packages/worker", "packages"},
	}

	cases := []struct {
		name       string
		repo       string
		sourceFile string
		roots      map[string][]string
		want       string
	}{
		{
			name:       "file under module root → module sub-path",
			repo:       "mono",
			sourceFile: "packages/api/src/x.ts",
			roots:      mono,
			want:       "packages/api",
		},
		{
			name:       "file at the module root itself",
			repo:       "mono",
			sourceFile: "packages/worker",
			roots:      mono,
			want:       "packages/worker",
		},
		{
			name:       "longest-prefix wins over a broader root",
			repo:       "mono",
			sourceFile: "packages/api/handler.ts",
			roots:      mono,
			want:       "packages/api", // not "packages"
		},
		{
			name:       "broader root still matches when no deeper root does",
			repo:       "mono",
			sourceFile: "packages/shared/util.ts",
			roots:      mono,
			want:       "packages",
		},
		{
			name:       "file outside any module root → empty",
			repo:       "mono",
			sourceFile: "tools/build.ts",
			roots:      map[string][]string{"mono": {"packages/api", "packages/worker"}},
			want:       "",
		},
		{
			name:       "sibling-prefix is NOT a match (apiv2 vs api)",
			repo:       "mono",
			sourceFile: "packages/apiv2/src/x.ts",
			roots:      map[string][]string{"mono": {"packages/api"}},
			want:       "",
		},
		{
			name:       "leading ./ and interior // are tolerated",
			repo:       "mono",
			sourceFile: "./packages/api//src/x.ts",
			roots:      map[string][]string{"mono": {"packages/api"}},
			want:       "packages/api",
		},
		{
			name:       "backslash paths are normalised",
			repo:       "mono",
			sourceFile: `packages\worker\job.ts`,
			roots:      map[string][]string{"mono": {"packages/worker"}},
			want:       "packages/worker",
		},
		{
			name:       "single-repo / non-monorepo group → empty (no roots)",
			repo:       "solo",
			sourceFile: "src/x.ts",
			roots:      nil,
			want:       "",
		},
		{
			name:       "repo not in the monorepo map → empty",
			repo:       "other",
			sourceFile: "packages/api/src/x.ts",
			roots:      mono,
			want:       "",
		},
		{
			name:       "empty source file → empty",
			repo:       "mono",
			sourceFile: "",
			roots:      mono,
			want:       "",
		},
		{
			name:       "path escaping the repo root is rejected",
			repo:       "mono",
			sourceFile: "../outside/x.ts",
			roots:      map[string][]string{"mono": {"packages/api"}},
			want:       "",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := modulePathFor(tc.repo, tc.sourceFile, tc.roots)
			if got != tc.want {
				t.Fatalf("modulePathFor(%q, %q) = %q; want %q", tc.repo, tc.sourceFile, got, tc.want)
			}
		})
	}
}

func TestModuleRootsByRepo(t *testing.T) {
	repos := []repoRef{
		{Slug: "api", Path: "/p/api", Modules: nil},                              // single-repo: omitted
		{Slug: "mono", Path: "/p/mono", Modules: []string{"packages/api", "packages/worker"}},
	}
	got := moduleRootsByRepo(repos)
	if _, ok := got["api"]; ok {
		t.Fatalf("repo with no modules should be omitted; got %v", got)
	}
	if want := []string{"packages/api", "packages/worker"}; len(got["mono"]) != len(want) {
		t.Fatalf("mono roots = %v; want %v", got["mono"], want)
	}

	// All-single-repo group → nil map → modulePathFor short-circuits to "".
	if m := moduleRootsByRepo([]repoRef{{Slug: "solo", Modules: nil}}); m != nil {
		t.Fatalf("expected nil map for non-monorepo group; got %v", m)
	}
}
