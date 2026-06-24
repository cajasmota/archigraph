package manifest

import "testing"

// ---------------------------------------------------------------------------
// corral.json (Pony / corral) — happy path
// ---------------------------------------------------------------------------

func TestCorralJSON_Deps(t *testing.T) {
	src := `{
  "deps": [
    { "locator": "github.com/ponylang/http_server.git", "version": "0.2.1" },
    { "locator": "github.com/ponylang/net_ssl.git", "version": "1.3.2" },
    { "locator": "git@github.com:ponylang/json.git" }
  ]
}`
	got := depNamesSet(t, "/proj/corral.json", src)
	for _, want := range []string{"http_server", "net_ssl", "json"} {
		if got[want] != "corral" {
			t.Errorf("expected dep %q with pm=corral, got pm=%q (deps: %v)", want, got[want], got)
		}
	}
	if len(got) != 3 {
		t.Errorf("expected exactly 3 deps, got %d: %v", len(got), got)
	}
}

func TestCorralJSON_VersionCarried(t *testing.T) {
	src := `{"deps":[{"locator":"github.com/ponylang/http_server.git","version":"0.2.1"}]}`
	records := runExtract(t, "/proj/corral.json", src)
	found := false
	for _, r := range records {
		if r.Subtype == "external_dependency" && r.Name == "http_server" {
			found = true
			if r.Properties["version"] != "0.2.1" {
				t.Errorf("http_server version = %q, want 0.2.1", r.Properties["version"])
			}
		}
	}
	if !found {
		t.Fatal("http_server dependency not found")
	}
}

// Legacy bundle.json with a corral-shaped deps array IS recognised, including
// the older {type,repo} entry form (with `tag` as version).
func TestBundleJSON_LegacyDeps(t *testing.T) {
	src := `{
  "type": "program",
  "deps": [
    { "type": "github", "repo": "ponylang/regex", "tag": "0.4.0" }
  ]
}`
	got := depNamesSet(t, "/proj/bundle.json", src)
	if got["regex"] != "corral" {
		t.Errorf("expected legacy bundle.json dep regex with pm=corral, got %v", got)
	}
	records := runExtract(t, "/proj/bundle.json", src)
	for _, r := range records {
		if r.Subtype == "external_dependency" && r.Name == "regex" && r.Properties["version"] != "0.4.0" {
			t.Errorf("regex version = %q, want 0.4.0 (from tag)", r.Properties["version"])
		}
	}
}

// ---------------------------------------------------------------------------
// Negative: a generic / non-corral bundle.json is a complete no-op (no deps,
// no project anchor). bundle.json is an ambiguous basename, so the corral
// signal (a parseable deps[] array) is required.
// ---------------------------------------------------------------------------

func TestBundleJSON_GenericNoOp(t *testing.T) {
	// A typical JS/webpack-style bundle.json with no corral deps array.
	src := `{"version":3,"sources":["a.js","b.js"],"mappings":"AAAA"}`
	records := runExtract(t, "/proj/bundle.json", src)
	if len(records) != 0 {
		t.Errorf("generic bundle.json must be a no-op (no anchor, no deps), got %d records", len(records))
	}
}

func TestCorralJSON_NoDepsNoOp(t *testing.T) {
	// corral.json is an unambiguous name: it still anchors, but yields no deps.
	src := `{"info":{"name":"myproj"}}`
	got := depNamesSet(t, "/proj/corral.json", src)
	if len(got) != 0 {
		t.Errorf("corral.json with no deps should yield no dependency records, got %v", got)
	}
}

func TestCorral_IsManifest(t *testing.T) {
	if !IsManifest("/proj/corral.json") {
		t.Error("corral.json must be treated as a manifest")
	}
	if detectPackageManager("/proj/corral.json") != "corral" {
		t.Errorf("corral.json package manager = %q, want corral", detectPackageManager("/proj/corral.json"))
	}
}
