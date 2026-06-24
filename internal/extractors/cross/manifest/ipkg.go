// ipkg.go — Idris (Idris2) *.ipkg package manifest parser (#5382, epic #5360).
//
// An Idris package is described by a `*.ipkg` manifest. It is a small, line- and
// field-oriented format (one `key = value` per logical field, values may span
// lines via a trailing operator/comma):
//
//	package myproject
//
//	authors = "Jane Doe"
//	version = 0.1.0
//	sourcedir = "src"
//
//	depends = base
//	        , contrib
//	        , network
//
//	modules = Data.Foo
//	        , Data.Bar
//	        , Main
//
//	main = Main
//	executable = myproject
//
// Dependency surface — `depends`:
//
//	The `depends` field is a comma-separated list of bare package names (Idris's
//	ipkg `depends` has NO version-constraint syntax — a dep is just a package
//	name resolved by the package collection / `pack`). Each name becomes a
//	DEPENDS_ON / DEPENDS_ON_PACKAGE edge with an EMPTY version (honest: there is
//	no version literal to record). The Idris stdlib floor packages (`base`,
//	`prelude`, `contrib`, `network`, `linear`, …) are KEPT as real edges,
//	mirroring the nimble `nim` / luarocks `lua` / cabal `base` interpreter-floor
//	treatment (#5365/#5367/#5373). Some build setups also use a `pkgs` field as a
//	synonym for the dependency list; it is parsed identically.
//
// Manifest metadata — surfaced on the project anchor (no new entity kind, the
// same model as the Zig `build_targets` / ReScript `rescript_config` props):
//
//	package    — the declared package name
//	modules    — the comma-separated module list (Data.Foo, Data.Bar, Main)
//	main       — the program entry module
//	executable — the produced executable name
//	sourcedir  — the source root directory
//
// These are joined into a compact, deterministic `ipkg_config` property so the
// package/modules/executable/sourcedir configuration is queryable without
// introducing a new entity kind.
//
// Honest scope: only the declared manifest surface is recovered. `depends`
// versions are always empty (ipkg has no constraint syntax); there is no ipkg
// lockfile format (the package collection / `pack.toml` pins the resolved set,
// which is not a per-package manifest), so lockfile_parsing is not_applicable.
package manifest

import (
	"regexp"
	"strings"
)

// ipkgFieldRE matches an `ipkg` field key at the start of a logical line, e.g.
// `depends`, `modules`, `package`, `executable`. Group 1 is the field key. The
// `package`/`module` declaration headers (`package foo`, no `=`) are handled
// separately because they have no `=` separator.
var ipkgFieldRE = regexp.MustCompile(`(?mi)^[ \t]*([A-Za-z_][A-Za-z0-9_-]*)[ \t]*=`)

// ipkgPackageHeaderRE matches the `package <name>` declaration header (the one
// field written without an `=`). Group 1 is the package name.
var ipkgPackageHeaderRE = regexp.MustCompile(`(?mi)^[ \t]*package[ \t]+([A-Za-z0-9_.-]+)[ \t]*$`)

// ipkgNameTokenRE validates/extracts a single dependency or module token: an
// Idris identifier path (letters, digits, `_`, `-` and `.` for module names).
var ipkgNameTokenRE = regexp.MustCompile(`^[A-Za-z0-9_][A-Za-z0-9_.-]*$`)

// parseIpkg parses an Idris `*.ipkg` manifest and returns its declared
// dependencies (the `depends`/`pkgs` field). Dependency versions are always
// empty (ipkg has no version-constraint syntax). First declaration of a name
// wins on duplicates.
func parseIpkg(source string) []dep {
	var out []dep
	seen := map[string]bool{}

	for _, field := range []string{"depends", "pkgs"} {
		body := ipkgFieldValue(source, field)
		if body == "" {
			continue
		}
		for _, name := range splitIpkgList(body) {
			if name == "" || seen[name] || !ipkgNameTokenRE.MatchString(name) {
				continue
			}
			seen[name] = true
			out = append(out, dep{name: name, version: "", kind: "runtime"})
		}
	}
	return out
}

// ipkgFieldValue returns the raw value body of the named `key = value` field,
// gathering field continuations. An ipkg field value continues onto subsequent
// lines while those lines are indented MORE than the field key (the standard
// ipkg layout for multi-line `depends`/`modules` lists, whose continuation lines
// start with a leading `,`). Returns "" when the field is absent.
func ipkgFieldValue(source, key string) string {
	lines := strings.Split(source, "\n")
	keyRE := regexp.MustCompile(`(?i)^[ \t]*` + regexp.QuoteMeta(key) + `[ \t]*=`)
	for i := 0; i < len(lines); i++ {
		if !keyRE.MatchString(lines[i]) {
			continue
		}
		fieldIndent := indentWidth(lines[i])
		body := lines[i][strings.IndexByte(lines[i], '=')+1:]
		for j := i + 1; j < len(lines); j++ {
			cont := lines[j]
			if strings.TrimSpace(cont) == "" {
				break
			}
			// A continuation line is more-indented than the field key, OR begins
			// with a leading comma (the canonical multi-line-list layout).
			if indentWidth(cont) > fieldIndent || strings.HasPrefix(strings.TrimSpace(cont), ",") {
				body += "\n" + cont
				continue
			}
			break
		}
		return body
	}
	return ""
}

// splitIpkgList splits a comma/newline-separated ipkg field body into trimmed,
// unquoted tokens, dropping `--`/`---` line comments.
func splitIpkgList(body string) []string {
	// Strip `-- ...` line comments from each line first.
	var cleaned strings.Builder
	for _, line := range strings.Split(body, "\n") {
		if idx := strings.Index(line, "--"); idx >= 0 {
			line = line[:idx]
		}
		cleaned.WriteString(line)
		cleaned.WriteByte('\n')
	}
	fields := strings.FieldsFunc(cleaned.String(), func(r rune) bool {
		return r == ',' || r == '\n' || r == '\r'
	})
	out := make([]string, 0, len(fields))
	for _, f := range fields {
		f = strings.TrimSpace(f)
		f = strings.Trim(f, `"'`)
		f = strings.TrimSpace(f)
		if f != "" {
			out = append(out, f)
		}
	}
	return out
}

// ipkgConfigProperty returns a compact, deterministic summary of an `*.ipkg`
// manifest's metadata fields (package/modules/main/executable/sourcedir),
// surfaced on the project anchor as the "ipkg_config" property, or "" when the
// source declares none of them. Mirrors the Zig build_targets / ReScript
// rescript_config anchor-property model — no new entity kind.
func ipkgConfigProperty(source string) string {
	var parts []string

	if m := ipkgPackageHeaderRE.FindStringSubmatch(source); m != nil {
		parts = append(parts, "package="+strings.TrimSpace(m[1]))
	}
	// scalar single-value fields
	for _, key := range []string{"main", "executable", "sourcedir", "version"} {
		if v := ipkgScalarField(source, key); v != "" {
			parts = append(parts, key+"="+v)
		}
	}
	// module list (comma-joined)
	if body := ipkgFieldValue(source, "modules"); body != "" {
		var mods []string
		for _, mod := range splitIpkgList(body) {
			if ipkgNameTokenRE.MatchString(mod) {
				mods = append(mods, mod)
			}
		}
		if len(mods) > 0 {
			parts = append(parts, "modules="+strings.Join(mods, " "))
		}
	}
	return strings.Join(parts, "; ")
}

// ipkgScalarField returns the trimmed, unquoted single-line value of a scalar
// `key = value` field (main/executable/sourcedir/version), or "".
func ipkgScalarField(source, key string) string {
	body := ipkgFieldValue(source, key)
	if body == "" {
		return ""
	}
	// Scalar value: first non-empty token only (strip a trailing comment).
	if idx := strings.Index(body, "--"); idx >= 0 {
		body = body[:idx]
	}
	body = strings.TrimSpace(body)
	body = strings.Trim(body, `"'`)
	return strings.TrimSpace(body)
}
