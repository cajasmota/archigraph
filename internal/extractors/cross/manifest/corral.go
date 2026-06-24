// corral.go — Pony `corral.json` / `bundle.json` package-manifest parser
// (#5384, epic #5360).
//
// corral (https://github.com/ponylang/corral) is the dependency manager for the
// Pony language. A project's dependencies live in a `corral.json` manifest as a
// `deps` array; each entry is an object with a `locator` (a VCS path such as
// `github.com/ponylang/http_server.git`) plus an optional `version` (a git tag
// or commit). corral also records transitive deps it has resolved into the same
// `deps` array, so the manifest doubles as the resolved set — there is no
// separate lockfile format.
//
//	{
//	  "deps": [
//	    { "locator": "github.com/ponylang/http_server.git", "version": "0.2.1" },
//	    { "locator": "github.com/ponylang/net_ssl.git",     "version": "1.3.2" }
//	  ]
//	}
//
// `bundle.json` is the legacy manifest name corral grew out of (the older
// `pony-stable` tool used it); it carries the same `deps` array shape — entries
// may use `locator` or the older `{ "type": "github", "repo": "owner/name" }`
// form. Both are recognised here, package_manager=`corral`.
//
// The dependency NAME is the locator's trailing path segment with any `.git`
// suffix stripped (e.g. `github.com/ponylang/http_server.git` -> `http_server`),
// which is the package's Pony `use` name. The full locator is preserved in the
// `version` provenance only when no explicit version is given is NOT done — the
// version field carries the declared git tag/commit when present.
package manifest

import (
	"encoding/json"
	"strings"
)

// corralManifest is the decoded shape of a corral.json / bundle.json file. Only
// the dependency-bearing fields are modelled; unknown keys (e.g. `info`,
// `packages`) are ignored by encoding/json.
type corralManifest struct {
	Deps []corralDep `json:"deps"`
}

// corralDep is one entry of the `deps` array. Both the modern `locator` form and
// the legacy `{type,repo}` form are accepted; `version` and the legacy `tag` are
// both treated as the declared version.
type corralDep struct {
	Locator string `json:"locator"`
	Version string `json:"version"`
	Tag     string `json:"tag"`
	Repo    string `json:"repo"`
}

// parseCorralJSON parses a corral.json / bundle.json manifest and returns its
// declared dependencies (package_manager=corral). The dep name is the locator's
// final path segment with a trailing `.git` removed; the declared version is the
// `version` field (or the legacy `tag`). First declaration of a name wins on
// duplicates.
func parseCorralJSON(source string) []dep {
	var data corralManifest
	if err := json.Unmarshal([]byte(source), &data); err != nil {
		return nil
	}
	var out []dep
	seen := map[string]bool{}
	for _, d := range data.Deps {
		locator := d.Locator
		if locator == "" {
			locator = d.Repo // legacy {type,repo} form
		}
		name := corralDepName(locator)
		if name == "" || seen[name] {
			continue
		}
		version := d.Version
		if version == "" {
			version = d.Tag
		}
		seen[name] = true
		out = append(out, dep{name: name, version: version, kind: "runtime"})
	}
	return out
}

// corralDepName derives the Pony package name from a corral locator. The locator
// is a VCS path (e.g. `github.com/ponylang/http_server.git`,
// `git@github.com:ponylang/net_ssl.git`, or an `owner/name` slug); the package
// name is its final path segment with a `.git` suffix and any trailing slash
// removed. Returns "" when nothing usable can be isolated.
func corralDepName(locator string) string {
	s := strings.TrimSpace(locator)
	if s == "" {
		return ""
	}
	s = strings.TrimSuffix(s, "/")
	// Split on both '/' and ':' so scp-style git@host:owner/name locators reduce
	// to their final segment too.
	if i := strings.LastIndexAny(s, "/:"); i >= 0 {
		s = s[i+1:]
	}
	s = strings.TrimSuffix(s, ".git")
	return strings.TrimSpace(s)
}
