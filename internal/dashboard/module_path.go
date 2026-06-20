package dashboard

// module_path.go — monorepo module-path attribution for dashboard payloads (#4698).
//
// The WebUI v2 scope selector (#4637) derives module options from the group's
// `monorepos` config (parent-repo slug → list of module sub-paths) and filters
// records with `matchesScope(repo, modulePath?)`. For module-precision filtering
// to work, each dashboard record must carry the module sub-path it belongs to —
// not just its repo slug. Without it the selector degrades to repo-level for any
// module scope (the TODO in the client `matchesScope`).
//
// `modulePathFor` derives that sub-path from a record's repo-relative source
// file and the repo's configured module roots, using a longest-prefix match:
//
//	roots = ["packages/api", "packages/worker"]
//	"packages/api/src/x.ts" → "packages/api"   (longest matching root)
//	"tools/build.ts"        → ""                (under no root)
//
// The derivation is layout-agnostic (Nx / Turbo / Gradle multi-module / Go
// workspaces / Python monorepo): it relies solely on the configured roots, never
// on framework heuristics. Single-repo / non-monorepo groups configure no module
// roots, so every record yields "" and behaviour is unchanged.

import (
	"path"
	"strings"
)

// moduleRootsByRepo builds the parent-repo-slug → module-roots map (the same
// shape as Group.monorepos) from the resolved repoRefs for a group. Repos with
// no configured modules are omitted, so the result is empty for single-repo /
// non-monorepo groups and modulePathFor short-circuits to "".
func moduleRootsByRepo(repos []repoRef) map[string][]string {
	var m map[string][]string
	for _, r := range repos {
		if len(r.Modules) == 0 {
			continue
		}
		if m == nil {
			m = make(map[string][]string, len(repos))
		}
		m[r.Slug] = r.Modules
	}
	return m
}

// modulePathFor returns the configured module sub-path that owns sourceFile
// within repo, or "" when the file is under no configured module root.
//
//   - repo:           the record's repo slug.
//   - sourceFile:     the record's repo-relative source path (forward slashes;
//     a leading "./" or "/" is tolerated).
//   - modulesByRepo:  parent-repo slug → configured module roots (the same map
//     that populates Group.monorepos). nil / missing repo ⇒ "".
//
// Matching rules:
//   - Roots are normalised (slashes, trailing "/" stripped) before comparison.
//   - A file matches a root when it equals the root or sits beneath it
//     ("packages/api" matches "packages/api" and "packages/api/src/x.ts" but NOT
//     "packages/apiv2/x.ts").
//   - On overlapping roots the LONGEST matching root wins ("packages/api" over
//     "packages"), so nested modules attribute to their most-specific module.
func modulePathFor(repo, sourceFile string, modulesByRepo map[string][]string) string {
	if repo == "" || sourceFile == "" || len(modulesByRepo) == 0 {
		return ""
	}
	roots := modulesByRepo[repo]
	if len(roots) == 0 {
		return ""
	}
	file := normalizeModuleRel(sourceFile)
	if file == "" {
		return ""
	}
	best := ""
	for _, r := range roots {
		root := normalizeModuleRel(r)
		if root == "" {
			continue
		}
		if file == root || strings.HasPrefix(file, root+"/") {
			if len(root) > len(best) {
				best = root
			}
		}
	}
	return best
}

// normalizeModuleRel cleans a repo-relative path for prefix matching: forward
// slashes, no leading "./" or "/", no trailing "/". Returns "" for paths that
// escape the repo root ("..") or are empty after cleaning.
func normalizeModuleRel(p string) string {
	p = strings.ReplaceAll(p, "\\", "/")
	p = strings.TrimPrefix(p, "./")
	p = strings.TrimPrefix(p, "/")
	if p == "" {
		return ""
	}
	// path.Clean collapses "a//b", "a/./b", and resolves interior "..".
	cleaned := path.Clean(p)
	if cleaned == "." || cleaned == ".." || strings.HasPrefix(cleaned, "../") {
		return ""
	}
	return strings.TrimSuffix(cleaned, "/")
}
