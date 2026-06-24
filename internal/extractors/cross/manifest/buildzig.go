// buildzig.go — Zig build system + package manifest parsers (#5377, epic #5360).
//
// Zig has no separate package-manager binary: `zig build` is both the build
// orchestrator and the dependency fetcher. Two files describe a Zig package:
//
//	build.zig      — the build script (Zig source). `pub fn build(b: *std.Build)`
//	                 declares build TARGETS (b.addExecutable / addStaticLibrary /
//	                 addSharedLibrary / addObject / addModule / addTest) and pulls
//	                 in declared dependencies via b.dependency("name", …). It is
//	                 the Zig analog of CMakeLists.txt / a Makefile (modelled here
//	                 as a build_system, like cmake/make/xmake).
//	build.zig.zon  — the package MANIFEST (Zig Object Notation, since Zig 0.11).
//	                 The `.dependencies = .{ .name = .{ .url=…, .hash=… } }` map
//	                 declares external dependencies; `.name`/`.version` describe
//	                 the package itself. Dependencies are content-addressed by a
//	                 SHA-256 `.hash`, so build.zig.zon IS the lockfile — there is
//	                 no separate lockfile format (lockfile_parsing is N/A).
//
// Both feed the shared manifest entity/edge builder (DEPENDS_ON +
// DEPENDS_ON_PACKAGE + SBOM package nodes) under package_manager "zig". The
// build.zig TARGET names are surfaced as a comma-joined "build_targets" property
// on the project anchor so target_extraction is queryable without a new entity
// kind, mirroring the cmake target treatment.
package manifest

import (
	"regexp"
	"strings"
)

// ---------------------------------------------------------------------------
// Parser: build.zig.zon  (Zig package manifest — manifest_parsing)
// ---------------------------------------------------------------------------

// zonDependenciesBlockRE captures the body of the top-level `.dependencies =
// .{ … }` anonymous-struct literal. Group 1 is the inner content, which
// zonDepEntryRE then mines for individual `.name = .{ … }` entries. The match is
// brace-balanced via a hand walk (regex cannot balance nested `.{ }`), so this
// RE only locates the `.dependencies = .{` head; the caller slices to the
// matching close brace.
var zonDependenciesHeadRE = regexp.MustCompile(`(?s)\.dependencies\s*=\s*\.\{`)

// zonDepEntryRE matches one dependency entry header inside the dependencies
// block: `.foo = .{`. Group 1 is the dependency name (the field identifier).
// Zig field names are `.ident` or `.@"quoted ident"`; both shapes are captured.
var zonDepEntryRE = regexp.MustCompile(`\.(?:@"([^"\n]+)"|([A-Za-z_][\w]*))\s*=\s*\.\{`)

// zonUrlRE / zonHashRE pull the `.url = "…"` and `.hash = "…"` fields out of a
// single dependency entry body so the resolved version (the content hash, or the
// archive URL tail) can be recorded as the dep version.
var zonUrlRE = regexp.MustCompile(`\.url\s*=\s*"([^"\n]+)"`)
var zonHashRE = regexp.MustCompile(`\.hash\s*=\s*"([^"\n]+)"`)

// matchingBrace returns the index just past the `}` that closes the `.{` whose
// opening brace is at openIdx (openIdx points at the `{`). Returns -1 when no
// balanced close is found. Quote-aware so a `{`/`}` inside a string literal is
// ignored.
func matchingBrace(s string, openIdx int) int {
	depth := 0
	inStr := false
	for i := openIdx; i < len(s); i++ {
		c := s[i]
		if inStr {
			if c == '\\' {
				i++
				continue
			}
			if c == '"' {
				inStr = false
			}
			continue
		}
		switch c {
		case '"':
			inStr = true
		case '{':
			depth++
		case '}':
			depth--
			if depth == 0 {
				return i + 1
			}
		}
	}
	return -1
}

// parseBuildZigZon parses a build.zig.zon manifest and returns its declared
// external dependencies. Each `.name = .{ .url=…, .hash=… }` entry under the
// top-level `.dependencies` block is one runtime dependency; the version is the
// content hash (preferred — it is the exact pin) falling back to the archive URL.
func parseBuildZigZon(source string) []dep {
	head := zonDependenciesHeadRE.FindStringIndex(source)
	if head == nil {
		return nil
	}
	// head[1]-1 is the `{` of the `.{` that opens the dependencies block.
	open := head[1] - 1
	end := matchingBrace(source, open)
	if end < 0 {
		end = len(source)
	}
	block := source[open+1 : end-1]

	var out []dep
	seen := map[string]bool{}
	entries := zonDepEntryRE.FindAllStringSubmatchIndex(block, -1)
	for i, m := range entries {
		// Group 1 (quoted) or group 2 (bare) is the dependency name.
		name := ""
		if m[2] >= 0 {
			name = block[m[2]:m[3]]
		} else if m[4] >= 0 {
			name = block[m[4]:m[5]]
		}
		if name == "" || seen[name] {
			continue
		}
		seen[name] = true
		// The entry body runs from this entry's `.{` to the next entry header
		// (or the end of the block) — a cheap slice that need not be balanced
		// since we only mine .url/.hash scalar fields from it.
		bodyStart := m[1]
		bodyEnd := len(block)
		if i+1 < len(entries) {
			bodyEnd = entries[i+1][0]
		}
		body := block[bodyStart:bodyEnd]
		version := ""
		if hm := zonHashRE.FindStringSubmatch(body); hm != nil {
			version = hm[1]
		} else if um := zonUrlRE.FindStringSubmatch(body); um != nil {
			version = um[1]
		}
		out = append(out, dep{name: name, version: version, kind: "runtime"})
	}
	return out
}

// ---------------------------------------------------------------------------
// Parser: build.zig  (Zig build system — dependency_graph + target_extraction)
// ---------------------------------------------------------------------------

// buildZigDependencyRE matches a `b.dependency("name", …)` call (the build-script
// side of a build.zig.zon dependency: it materialises a declared dependency for
// use in the build graph). Group 1 is the dependency name. The receiver is any
// identifier (`b`, `builder`, …) so non-canonical builder names still match.
var buildZigDependencyRE = regexp.MustCompile(`\b\w+\.dependency\(\s*"([^"\n]+)"`)

// buildZigTargetRE matches the build-target constructors. Zig 0.12+ takes an
// options struct whose `.name = "…"` field is the target name:
//
//	const exe = b.addExecutable(.{ .name = "myapp", … });
//	const lib = b.addStaticLibrary(.{ .name = "mylib", … });
//	const mod = b.addModule("mymod", .{ … });            // addModule: positional
//
// Group 1 is the constructor (addExecutable/…); group 2 is the positional name
// string when present (addModule form); the struct-field `.name` form is mined
// separately from the call's argument slice.
var buildZigTargetRE = regexp.MustCompile(
	`\b\w+\.(addExecutable|addStaticLibrary|addSharedLibrary|addObject|addModule|addTest|addLibrary)\s*\(`)

// buildZigNameFieldRE pulls a `.name = "…"` field out of a target constructor's
// argument slice.
var buildZigNameFieldRE = regexp.MustCompile(`\.name\s*=\s*"([^"\n]+)"`)

// buildZigPositionalNameRE pulls the leading positional `"name"` argument of an
// addModule("name", .{…}) call.
var buildZigPositionalNameRE = regexp.MustCompile(`^\s*"([^"\n]+)"`)

// extractBuildZigTargets returns the names of the build targets declared in a
// build.zig (addExecutable/addStaticLibrary/addSharedLibrary/addObject/addModule/
// addTest/addLibrary). Names come from the `.name = "…"` options field or the
// leading positional string (addModule). Deduplicated, declaration order kept.
func extractBuildZigTargets(source string) []string {
	var out []string
	seen := map[string]bool{}
	for _, m := range buildZigTargetRE.FindAllStringSubmatchIndex(source, -1) {
		// The call's argument slice runs from the `(` (m[1]-1) to its matching `)`.
		argStart := m[1] - 1
		argEnd := matchingParen(source, argStart)
		if argEnd < 0 {
			argEnd = len(source)
		}
		args := source[argStart+1 : argEnd-1]
		name := ""
		if pm := buildZigPositionalNameRE.FindStringSubmatch(args); pm != nil {
			name = pm[1]
		} else if nm := buildZigNameFieldRE.FindStringSubmatch(args); nm != nil {
			name = nm[1]
		}
		if name == "" || seen[name] {
			continue
		}
		seen[name] = true
		out = append(out, name)
	}
	return out
}

// matchingParen returns the index just past the `)` that closes the `(` at
// openIdx. Quote-aware. Returns -1 on imbalance.
func matchingParen(s string, openIdx int) int {
	depth := 0
	inStr := false
	for i := openIdx; i < len(s); i++ {
		c := s[i]
		if inStr {
			if c == '\\' {
				i++
				continue
			}
			if c == '"' {
				inStr = false
			}
			continue
		}
		switch c {
		case '"':
			inStr = true
		case '(':
			depth++
		case ')':
			depth--
			if depth == 0 {
				return i + 1
			}
		}
	}
	return -1
}

// parseBuildZig parses a build.zig build script and returns the dependencies it
// materialises via b.dependency("…", …). The declared build TARGET names are NOT
// dependencies; they are surfaced separately by extractBuildZigTargets and
// attached to the project anchor as a property (see buildZigTargetsProperty).
func parseBuildZig(source string) []dep {
	var out []dep
	seen := map[string]bool{}
	for _, m := range buildZigDependencyRE.FindAllStringSubmatch(source, -1) {
		name := m[1]
		if name == "" || seen[name] {
			continue
		}
		seen[name] = true
		out = append(out, dep{name: name, kind: "runtime"})
	}
	return out
}

// buildZigTargetsProperty returns the comma-joined build-target names for a
// build.zig source, or "" when none are found / the file is not a build.zig.
// Surfaced on the manifest project anchor as the "build_targets" property so
// target_extraction is queryable without introducing a new entity kind.
func buildZigTargetsProperty(filePath, source string) string {
	if !strings.HasSuffix(filePath, "build.zig") {
		return ""
	}
	targets := extractBuildZigTargets(source)
	if len(targets) == 0 {
		return ""
	}
	return strings.Join(targets, ",")
}
