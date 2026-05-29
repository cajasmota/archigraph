package manifest

// Tests for Pipfile / Pipfile.lock (pkg.pipfile) and Python lockfile parsers
// (uv.lock / pdm.lock / poetry.lock) for pkg.pyproject.
//
// Issue: #3075 — lockfile parsing for Pipfile + pyproject (uv/pdm)

import (
	"testing"
)

// ---------------------------------------------------------------------------
// Helper
// ---------------------------------------------------------------------------

func findDep(deps []dep, name string) *dep {
	for i := range deps {
		if deps[i].name == name {
			return &deps[i]
		}
	}
	return nil
}

// ---------------------------------------------------------------------------
// Pipfile (manifest)
// ---------------------------------------------------------------------------

func TestParsePipfile_BasicDeps(t *testing.T) {
	src := `
[[source]]
url = "https://pypi.org/simple"

[packages]
requests = "*"
Django = ">=3.2,<4"
flask = {version = "^2.0", extras = ["async"]}

[dev-packages]
pytest = ">=7"
black = "*"

[requires]
python_version = "3.11"
`
	deps := parsePipfile(src)
	if len(deps) != 5 {
		t.Fatalf("expected 5 deps, got %d: %+v", len(deps), deps)
	}

	requests := findDep(deps, "requests")
	if requests == nil {
		t.Fatal("expected requests dep")
	}
	if requests.isDev {
		t.Error("requests should not be dev")
	}
	if requests.kind != "runtime" {
		t.Errorf("requests kind=%q want runtime", requests.kind)
	}

	django := findDep(deps, "Django")
	if django == nil {
		t.Fatal("expected Django dep")
	}
	if django.version != ">=3.2,<4" {
		t.Errorf("Django version=%q want >=3.2,<4", django.version)
	}

	flask := findDep(deps, "flask")
	if flask == nil {
		t.Fatal("expected flask dep")
	}
	if flask.version != "^2.0" {
		t.Errorf("flask version=%q want ^2.0", flask.version)
	}

	pytest := findDep(deps, "pytest")
	if pytest == nil {
		t.Fatal("expected pytest dep")
	}
	if !pytest.isDev {
		t.Error("pytest should be isDev=true")
	}
	if pytest.kind != "dev" {
		t.Errorf("pytest kind=%q want dev", pytest.kind)
	}

	black := findDep(deps, "black")
	if black == nil {
		t.Fatal("expected black dep")
	}
	if !black.isDev {
		t.Error("black should be isDev=true")
	}
	// "*" version should be normalised to ""
	if black.version != "" {
		t.Errorf("black version=%q want empty string (wildcard normalised)", black.version)
	}
}

func TestParsePipfile_EmptyFile(t *testing.T) {
	deps := parsePipfile("")
	if len(deps) != 0 {
		t.Errorf("expected 0 deps for empty Pipfile, got %d", len(deps))
	}
}

func TestParsePipfile_SkipsPythonVersion(t *testing.T) {
	// python_version pseudo-key should not be emitted as a dependency.
	src := `
[packages]
requests = "*"

[requires]
python_version = "3.11"
python_full_version = "3.11.2"
`
	deps := parsePipfile(src)
	for _, d := range deps {
		if d.name == "python_version" || d.name == "python_full_version" {
			t.Errorf("should not emit %q as a dependency", d.name)
		}
	}
}

func TestParsePipfile_ViaExtractor(t *testing.T) {
	// End-to-end: extractor recognises "Pipfile" as a manifest.
	src := `
[packages]
httpx = ">=0.23"

[dev-packages]
mypy = "*"
`
	records := runExtract(t, "project/Pipfile", src)
	deps := depEntities(records)
	if len(deps) != 2 {
		t.Fatalf("expected 2 dep entities, got %d", len(deps))
	}
	for _, d := range deps {
		if d.Properties["package_manager"] != "pipenv" {
			t.Errorf("package_manager=%q want pipenv", d.Properties["package_manager"])
		}
	}
}

// ---------------------------------------------------------------------------
// Pipfile.lock
// ---------------------------------------------------------------------------

func TestParsePipfileLock_Basic(t *testing.T) {
	src := `{
  "_meta": {
    "hash": {"sha256": "abc"},
    "pipfile-spec": 6,
    "requires": {"python_version": "3.11"},
    "sources": [{"name": "pypi", "url": "https://pypi.org/simple", "verify_ssl": true}]
  },
  "default": {
    "certifi": {"hashes": ["sha256:abc"], "version": "==2023.7.22"},
    "requests": {"hashes": ["sha256:def"], "version": "==2.31.0"},
    "urllib3": {"hashes": ["sha256:ghi"], "version": "==2.0.4"}
  },
  "develop": {
    "pytest": {"hashes": ["sha256:jkl"], "version": "==7.4.0"},
    "black": {"hashes": ["sha256:mno"], "version": "==23.7.0"}
  }
}`
	deps := parsePipfileLock(src)
	if len(deps) != 5 {
		t.Fatalf("expected 5 locked deps, got %d: %+v", len(deps), deps)
	}

	requests := findDep(deps, "requests")
	if requests == nil {
		t.Fatal("expected requests dep")
	}
	if requests.version != "2.31.0" {
		t.Errorf("requests version=%q want 2.31.0 (== prefix stripped)", requests.version)
	}
	if requests.isDev {
		t.Error("requests should not be isDev")
	}
	if requests.kind != "locked" {
		t.Errorf("requests kind=%q want locked", requests.kind)
	}

	pytest := findDep(deps, "pytest")
	if pytest == nil {
		t.Fatal("expected pytest dep")
	}
	if !pytest.isDev {
		t.Error("pytest should be isDev=true")
	}
	if pytest.version != "7.4.0" {
		t.Errorf("pytest version=%q want 7.4.0", pytest.version)
	}
}

func TestParsePipfileLock_InvalidJSON(t *testing.T) {
	deps := parsePipfileLock("{invalid}")
	if deps != nil {
		t.Errorf("expected nil for invalid JSON, got %v", deps)
	}
}

func TestParsePipfileLock_ViaExtractor(t *testing.T) {
	src := `{
  "default": {
    "httpx": {"version": "==0.24.1"}
  },
  "develop": {
    "pytest": {"version": "==7.4.0"}
  }
}`
	records := runExtract(t, "project/Pipfile.lock", src)
	deps := depEntities(records)
	if len(deps) != 2 {
		t.Fatalf("expected 2 dep entities, got %d", len(deps))
	}
	for _, d := range deps {
		if d.Properties["package_manager"] != "pipenv" {
			t.Errorf("package_manager=%q want pipenv", d.Properties["package_manager"])
		}
		if d.Properties["dependency_kind"] != "locked" {
			t.Errorf("dependency_kind=%q want locked", d.Properties["dependency_kind"])
		}
	}
}

// ---------------------------------------------------------------------------
// uv.lock
// ---------------------------------------------------------------------------

func TestParseUvLock_Basic(t *testing.T) {
	src := `version = 1
requires-python = ">=3.11"

[[package]]
name = "certifi"
version = "2023.7.22"
source = { registry = "https://pypi.org/simple" }

[[package]]
name = "httpx"
version = "0.24.1"
source = { registry = "https://pypi.org/simple" }
dependencies = [
  { name = "certifi" },
]

[[package]]
name = "pytest"
version = "7.4.0"
source = { registry = "https://pypi.org/simple" }
`
	deps := parseUvLock(src)
	if len(deps) != 3 {
		t.Fatalf("expected 3 locked deps, got %d: %+v", len(deps), deps)
	}

	httpx := findDep(deps, "httpx")
	if httpx == nil {
		t.Fatal("expected httpx dep")
	}
	if httpx.version != "0.24.1" {
		t.Errorf("httpx version=%q want 0.24.1", httpx.version)
	}
	if httpx.kind != "locked" {
		t.Errorf("httpx kind=%q want locked", httpx.kind)
	}
}

func TestParseUvLock_EmptyFile(t *testing.T) {
	deps := parseUvLock("")
	if len(deps) != 0 {
		t.Errorf("expected 0 deps, got %d", len(deps))
	}
}

func TestParseUvLock_ViaExtractor(t *testing.T) {
	src := `version = 1

[[package]]
name = "requests"
version = "2.31.0"
`
	records := runExtract(t, "project/uv.lock", src)
	deps := depEntities(records)
	if len(deps) != 1 {
		t.Fatalf("expected 1 dep entity, got %d", len(deps))
	}
	if deps[0].Properties["package_manager"] != "uv" {
		t.Errorf("package_manager=%q want uv", deps[0].Properties["package_manager"])
	}
}

// ---------------------------------------------------------------------------
// pdm.lock
// ---------------------------------------------------------------------------

func TestParsePdmLock_Basic(t *testing.T) {
	src := `[metadata]
groups = ["default", "dev"]
strategy = ["cross_platform", "static_urls"]
lock_version = "4.5"
content_hash = "sha256:abc123"

[[package]]
name = "requests"
version = "2.31.0"

[[package]]
name = "pytest"
version = "7.4.0"
`
	deps := parsePdmLock(src)
	if len(deps) != 2 {
		t.Fatalf("expected 2 locked deps, got %d: %+v", len(deps), deps)
	}

	requests := findDep(deps, "requests")
	if requests == nil {
		t.Fatal("expected requests dep")
	}
	if requests.version != "2.31.0" {
		t.Errorf("requests version=%q want 2.31.0", requests.version)
	}
	if requests.kind != "locked" {
		t.Errorf("requests kind=%q want locked", requests.kind)
	}
}

func TestParsePdmLock_ViaExtractor(t *testing.T) {
	src := `[[package]]
name = "django"
version = "4.2.0"
`
	records := runExtract(t, "project/pdm.lock", src)
	deps := depEntities(records)
	if len(deps) != 1 {
		t.Fatalf("expected 1 dep entity, got %d", len(deps))
	}
	if deps[0].Properties["package_manager"] != "pdm" {
		t.Errorf("package_manager=%q want pdm", deps[0].Properties["package_manager"])
	}
}

// ---------------------------------------------------------------------------
// poetry.lock
// ---------------------------------------------------------------------------

func TestParsePoetryLock_Basic(t *testing.T) {
	src := `# This file is automatically @generated by Poetry 1.6.0 and should not be changed by hand.

[[package]]
name = "certifi"
version = "2023.7.22"
description = "Python package for providing Mozilla's CA Bundle."
category = "main"
optional = false
python-versions = ">=3.6"

[[package]]
name = "requests"
version = "2.31.0"
description = "Python HTTP for Humans."
category = "main"
optional = false
python-versions = ">=3.7"

[[package]]
name = "pytest"
version = "7.4.0"
description = "pytest: simple powerful testing with Python"
category = "dev"
optional = false
python-versions = ">=3.7"

[metadata]
lock-version = "1.1"
python-versions = "^3.11"
content-hash = "abc123"
`
	deps := parsePoetryLock(src)
	if len(deps) != 3 {
		t.Fatalf("expected 3 locked deps, got %d: %+v", len(deps), deps)
	}

	requests := findDep(deps, "requests")
	if requests == nil {
		t.Fatal("expected requests dep")
	}
	if requests.version != "2.31.0" {
		t.Errorf("requests version=%q want 2.31.0", requests.version)
	}
	if requests.isDev {
		t.Error("requests (main) should not be isDev")
	}
	if requests.kind != "locked" {
		t.Errorf("requests kind=%q want locked", requests.kind)
	}

	pytest := findDep(deps, "pytest")
	if pytest == nil {
		t.Fatal("expected pytest dep")
	}
	if !pytest.isDev {
		t.Error("pytest (dev) should be isDev=true")
	}
}

func TestParsePoetryLock_ViaExtractor(t *testing.T) {
	src := `[[package]]
name = "flask"
version = "2.3.0"
category = "main"

[[package]]
name = "black"
version = "23.7.0"
category = "dev"
`
	records := runExtract(t, "project/poetry.lock", src)
	deps := depEntities(records)
	if len(deps) != 2 {
		t.Fatalf("expected 2 dep entities, got %d", len(deps))
	}
	for _, d := range deps {
		if d.Properties["package_manager"] != "poetry" {
			t.Errorf("package_manager=%q want poetry", d.Properties["package_manager"])
		}
	}
}

// ---------------------------------------------------------------------------
// IsManifest / detectPackageManager integration
// ---------------------------------------------------------------------------

func TestIsManifest_PythonLockfiles(t *testing.T) {
	cases := []struct {
		path string
		want bool
	}{
		{"project/Pipfile", true},
		{"project/Pipfile.lock", true},
		{"project/uv.lock", true},
		{"project/pdm.lock", true},
		{"project/poetry.lock", true},
		{"project/unknown.lock", false},
	}
	for _, tc := range cases {
		got := IsManifest(tc.path)
		if got != tc.want {
			t.Errorf("IsManifest(%q)=%v want %v", tc.path, got, tc.want)
		}
	}
}

func TestDetectPackageManager_PythonLockfiles(t *testing.T) {
	cases := []struct {
		path string
		want string
	}{
		{"Pipfile", "pipenv"},
		{"Pipfile.lock", "pipenv"},
		{"uv.lock", "uv"},
		{"pdm.lock", "pdm"},
		{"poetry.lock", "poetry"},
	}
	for _, tc := range cases {
		got := detectPackageManager(tc.path)
		if got != tc.want {
			t.Errorf("detectPackageManager(%q)=%q want %q", tc.path, got, tc.want)
		}
	}
}
