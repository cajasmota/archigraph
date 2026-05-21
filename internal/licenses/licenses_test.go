package licenses

import (
	"os"
	"path/filepath"
	"testing"
)

// ---------------------------------------------------------------------------
// SPDX normalizer
// ---------------------------------------------------------------------------

func TestNormalizeSPDX(t *testing.T) {
	cases := []struct {
		in, want string
	}{
		{"mit", "MIT"},
		{"MIT", "MIT"},
		{"MIT License", "MIT"},
		{"Apache 2.0", "Apache-2.0"},
		{"Apache-2.0", "Apache-2.0"},
		{"GPL-3.0", "GPL-3.0-only"},
		{"GPL-2.0", "GPL-2.0-only"},
		{"AGPL-3.0", "AGPL-3.0-only"},
		{"LGPL-2.1", "LGPL-2.1-only"},
		{"ISC", "ISC"},
		{"BSD 3-Clause", "BSD-3-Clause"},
		{"MPL-2.0", "MPL-2.0"},
		{"Unlicense", "Unlicense"},
		{"proprietary", "Proprietary"},
		{"SomeFancyLicense-9.9", "SomeFancyLicense-9.9"}, // pass-through
	}
	for _, c := range cases {
		got := normalizeSPDX(c.in)
		if got != c.want {
			t.Errorf("normalizeSPDX(%q) = %q; want %q", c.in, got, c.want)
		}
	}
}

// ---------------------------------------------------------------------------
// Compatibility matrix
// ---------------------------------------------------------------------------

func TestCheckCompatibility(t *testing.T) {
	cases := []struct {
		proj, dep string
		want      CompatibilityLevel
	}{
		// MIT project, GPL dep → error
		{"MIT", "GPL-3.0-only", CompatError},
		{"MIT", "AGPL-3.0-only", CompatError},
		{"MIT", "GPL-2.0-only", CompatError},
		// MIT project, MIT dep → ok
		{"MIT", "MIT", CompatOK},
		{"MIT", "Apache-2.0", CompatOK},
		{"MIT", "BSD-3-Clause", CompatOK},
		{"MIT", "ISC", CompatOK},
		// MIT project, LGPL dep → warn
		{"MIT", "LGPL-2.1-only", CompatWarn},
		{"MIT", "LGPL-3.0-only", CompatWarn},
		{"MIT", "MPL-2.0", CompatWarn},
		// GPL project, GPL dep → ok
		{"GPL-3.0-only", "GPL-3.0-only", CompatOK},
		{"GPL-3.0-only", "GPL-2.0-only", CompatOK},
		// AGPL project, GPL dep → ok
		{"AGPL-3.0-only", "GPL-3.0-only", CompatOK},
		// Apache project, AGPL dep → error
		{"Apache-2.0", "AGPL-3.0-only", CompatError},
		// Unknown dep → unknown
		{"MIT", "", CompatUnknown},
		{"MIT", "NOASSERTION", CompatUnknown},
	}
	for _, c := range cases {
		got := CheckCompatibility(c.proj, c.dep)
		if got != c.want {
			t.Errorf("CheckCompatibility(%q, %q) = %q; want %q", c.proj, c.dep, got, c.want)
		}
	}
}

// ---------------------------------------------------------------------------
// inferFromLicenseText
// ---------------------------------------------------------------------------

func TestInferFromLicenseText(t *testing.T) {
	cases := []struct {
		text, want string
	}{
		{"MIT License\nPermission is hereby granted", "MIT"},
		{"GNU General Public License version 3", "GPL-3.0-only"},
		{"GNU General Public License version 2", "GPL-2.0-only"},
		{"GNU Affero General Public License", "AGPL-3.0-only"},
		{"GNU Lesser General Public License version 3", "LGPL-3.0-only"},
		{"Apache License", "Apache-2.0"},
		{"BSD 3-Clause License\nRedistributions of source code", "BSD-3-Clause"},
		{"ISC License", "ISC"},
		{"Mozilla Public License", "MPL-2.0"},
		{"This is UNLICENSE — public domain", "Unlicense"},
		{"some random text with no license info", ""},
	}
	for _, c := range cases {
		got := inferFromLicenseText(c.text)
		if got != c.want {
			t.Errorf("inferFromLicenseText(%q) = %q; want %q", c.text, got, c.want)
		}
	}
}

// ---------------------------------------------------------------------------
// extractTOMLField
// ---------------------------------------------------------------------------

func TestExtractTOMLField(t *testing.T) {
	cases := []struct {
		src, key, want string
	}{
		{"[package]\nlicense = \"MIT\"", "license", "MIT"},
		{"[package]\nlicense = {text = \"Apache-2.0\"}", "license", "Apache-2.0"},
		{"[package]\nversion = \"1.0.0\"", "license", ""},
		{"license = \"GPL-3.0-only\"", "license", "GPL-3.0-only"},
	}
	for _, c := range cases {
		got := extractTOMLField(c.src, c.key)
		if got != c.want {
			t.Errorf("extractTOMLField(%q, %q) = %q; want %q", c.src, c.key, got, c.want)
		}
	}
}

// ---------------------------------------------------------------------------
// DetectProjectLicense — package.json path
// ---------------------------------------------------------------------------

func TestDetectProjectLicenseFromPackageJSON(t *testing.T) {
	tmp := t.TempDir()
	if err := os.WriteFile(filepath.Join(tmp, "package.json"),
		[]byte(`{"name":"test","license":"MIT"}`), 0o644); err != nil {
		t.Fatal(err)
	}
	lic, src := DetectProjectLicense(tmp)
	if lic != "MIT" {
		t.Errorf("license = %q; want MIT", lic)
	}
	if src != "package.json" {
		t.Errorf("source = %q; want package.json", src)
	}
}

func TestDetectProjectLicenseFromFile(t *testing.T) {
	tmp := t.TempDir()
	if err := os.WriteFile(filepath.Join(tmp, "LICENSE"),
		[]byte("MIT License\nPermission is hereby granted"), 0o644); err != nil {
		t.Fatal(err)
	}
	lic, _ := DetectProjectLicense(tmp)
	if lic != "MIT" {
		t.Errorf("license = %q; want MIT", lic)
	}
}

func TestDetectProjectLicenseGPL(t *testing.T) {
	tmp := t.TempDir()
	if err := os.WriteFile(filepath.Join(tmp, "LICENSE"),
		[]byte("GNU General Public License version 3"), 0o644); err != nil {
		t.Fatal(err)
	}
	lic, _ := DetectProjectLicense(tmp)
	if lic != "GPL-3.0-only" {
		t.Errorf("license = %q; want GPL-3.0-only", lic)
	}
}

// ---------------------------------------------------------------------------
// DetectNPMLicenses — local node_modules
// ---------------------------------------------------------------------------

func TestDetectNPMLicenses(t *testing.T) {
	tmp := t.TempDir()
	pkgDir := filepath.Join(tmp, "node_modules", "express")
	if err := os.MkdirAll(pkgDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(pkgDir, "package.json"),
		[]byte(`{"name":"express","license":"MIT"}`), 0o644); err != nil {
		t.Fatal(err)
	}
	m := DetectNPMLicenses(tmp, []string{"express", "missing-pkg"})
	if m["express"] != "MIT" {
		t.Errorf("express license = %q; want MIT", m["express"])
	}
	if m["missing-pkg"] != "Unknown" {
		t.Errorf("missing-pkg license = %q; want Unknown", m["missing-pkg"])
	}
}

func TestDetectNPMLicensesLegacyFormat(t *testing.T) {
	tmp := t.TempDir()
	pkgDir := filepath.Join(tmp, "node_modules", "oldpkg")
	if err := os.MkdirAll(pkgDir, 0o755); err != nil {
		t.Fatal(err)
	}
	// Old-format with licenses array.
	if err := os.WriteFile(filepath.Join(pkgDir, "package.json"),
		[]byte(`{"name":"oldpkg","licenses":[{"type":"BSD-3-Clause"}]}`), 0o644); err != nil {
		t.Fatal(err)
	}
	m := DetectNPMLicenses(tmp, []string{"oldpkg"})
	if m["oldpkg"] != "BSD-3-Clause" {
		t.Errorf("oldpkg license = %q; want BSD-3-Clause", m["oldpkg"])
	}
}

// ---------------------------------------------------------------------------
// SuggestAlternatives
// ---------------------------------------------------------------------------

func TestSuggestAlternatives(t *testing.T) {
	alts := SuggestAlternatives("node-forge")
	if len(alts) == 0 {
		t.Error("expected at least one alternative for node-forge")
	}
	alts2 := SuggestAlternatives("some-unknown-pkg")
	if len(alts2) != 0 {
		t.Errorf("expected no alternatives for unknown pkg, got %v", alts2)
	}
}

// ---------------------------------------------------------------------------
// ResolveNPMTransitiveDeps
// ---------------------------------------------------------------------------

func TestResolveNPMTransitiveDeps(t *testing.T) {
	tmp := t.TempDir()
	lock := `{"lockfileVersion":2,"packages":{"":{},"node_modules/lodash":{"version":"4.17.21"}}}`
	if err := os.WriteFile(filepath.Join(tmp, "package-lock.json"),
		[]byte(lock), 0o644); err != nil {
		t.Fatal(err)
	}
	m := ResolveNPMTransitiveDeps(tmp)
	if m["lodash"] != "4.17.21" {
		t.Errorf("lodash version = %q; want 4.17.21", m["lodash"])
	}
}

func TestResolveNPMTransitiveDepsV1(t *testing.T) {
	tmp := t.TempDir()
	lock := `{"lockfileVersion":1,"dependencies":{"chalk":{"version":"4.1.2"}}}`
	if err := os.WriteFile(filepath.Join(tmp, "package-lock.json"),
		[]byte(lock), 0o644); err != nil {
		t.Fatal(err)
	}
	m := ResolveNPMTransitiveDeps(tmp)
	if m["chalk"] != "4.1.2" {
		t.Errorf("chalk version = %q; want 4.1.2", m["chalk"])
	}
}

// ---------------------------------------------------------------------------
// ScanRepoLicenses — end-to-end with local fixtures
// ---------------------------------------------------------------------------

func TestScanRepoLicensesCompatError(t *testing.T) {
	tmp := t.TempDir()
	// Project is MIT.
	if err := os.WriteFile(filepath.Join(tmp, "package.json"),
		[]byte(`{"license":"MIT"}`), 0o644); err != nil {
		t.Fatal(err)
	}
	// Create a fake GPL node_modules dep.
	pkgDir := filepath.Join(tmp, "node_modules", "gpl-lib")
	if err := os.MkdirAll(pkgDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(pkgDir, "package.json"),
		[]byte(`{"name":"gpl-lib","license":"GPL-3.0"}`), 0o644); err != nil {
		t.Fatal(err)
	}

	pkgList := []map[string]string{
		{"name": "gpl-lib", "package_manager": "npm", "version": "1.0.0"},
	}
	res, err := ScanRepoLicenses(tmp, pkgList, 100)
	if err != nil {
		t.Fatal(err)
	}
	if res.ProjectLicense != "MIT" {
		t.Errorf("project license = %q; want MIT", res.ProjectLicense)
	}
	if len(res.Dependencies) == 0 {
		t.Fatal("expected at least one dependency")
	}
	dep := res.Dependencies[0]
	if dep.License != "GPL-3.0-only" {
		t.Errorf("dep license = %q; want GPL-3.0-only", dep.License)
	}
	if dep.Compatibility != CompatError {
		t.Errorf("dep compat = %q; want error", dep.Compatibility)
	}
	if len(res.Incompatible) == 0 {
		t.Error("expected incompatible list to be non-empty")
	}
}

func TestScanRepoLicensesAllOK(t *testing.T) {
	tmp := t.TempDir()
	if err := os.WriteFile(filepath.Join(tmp, "package.json"),
		[]byte(`{"license":"MIT"}`), 0o644); err != nil {
		t.Fatal(err)
	}
	pkgDir := filepath.Join(tmp, "node_modules", "lodash")
	if err := os.MkdirAll(pkgDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(pkgDir, "package.json"),
		[]byte(`{"name":"lodash","license":"MIT"}`), 0o644); err != nil {
		t.Fatal(err)
	}

	pkgList := []map[string]string{
		{"name": "lodash", "package_manager": "npm", "version": "4.17.21"},
	}
	res, err := ScanRepoLicenses(tmp, pkgList, 200)
	if err != nil {
		t.Fatal(err)
	}
	if len(res.Incompatible) != 0 {
		t.Errorf("expected no incompatible deps; got %d", len(res.Incompatible))
	}
	if res.LicenseDensity["MIT"] != 1.0 {
		t.Errorf("license density MIT = %v; want 1.0", res.LicenseDensity["MIT"])
	}
}
