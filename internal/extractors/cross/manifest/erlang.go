// Erlang build-tooling manifest parsers (#4930 / #4987).
//
// Three manifest shapes are recognised:
//
//   - rebar.config   — rebar3 build config. Dependencies live in the
//     {deps, [...]} term as `dep` atoms or {dep, "1.2.3"} / {dep, {pkg, ...}}
//     / {dep, {git, "url", ...}} tuples. Plugins live in {plugins, [...]}.
//     Resolved from hex.pm (package_manager = rebar3).
//   - rebar.lock     — rebar3 lockfile. Each locked dep is a
//     {<<"name">>, {pkg,<<"name">>,<<"1.2.3">>}, N} tuple → kind=locked.
//   - *.app.src      — Erlang/OTP application resource. Runtime application
//     dependencies live in the {applications, [...]} list.
//   - erlang.mk / Makefile (with erlang.mk include) — the erlang.mk build
//     system. Dependencies are declared `DEPS = a b c` (+ test/build/local
//     variants) with `dep_a = git url ref` provenance lines.
//
// These mirror the existing cross-manifest parsers (Cargo/npm/...) and emit
// the same SCOPE.Component(external_dependency) + DEPENDS_ON + SCOPE.Package
// records via buildEntitiesAndRels.
package manifest

import (
	"regexp"
	"strings"
)

// ---------------------------------------------------------------------------
// rebar.config (rebar3)
// ---------------------------------------------------------------------------

// rebarDepsBlockRE captures the content of the {deps, [ ... ]} term.
var rebarDepsBlockRE = regexp.MustCompile(`(?s)\{\s*deps\s*,\s*\[(.*?)\]\s*\}`)

// rebarPluginsBlockRE captures the content of the {plugins, [ ... ]} term.
var rebarPluginsBlockRE = regexp.MustCompile(`(?s)\{\s*plugins\s*,\s*\[(.*?)\]\s*\}`)

// rebarTupleDepRE matches a {name, "version"} or {name, {pkg, ...}} /
// {name, {git, ...}} dependency tuple — group 1 is the dep atom, group 2 the
// optional quoted version when present in the simple {name, "x.y.z"} form.
var rebarTupleDepRE = regexp.MustCompile(`\{\s*([a-z][a-zA-Z0-9_]*)\s*,\s*(?:"([^"]*)")?`)

// rebarAtomDepRE matches a bare-atom dependency entry (latest hex version):
// {deps, [cowboy, jsx]}.
var rebarAtomDepRE = regexp.MustCompile(`(?m)(?:^|[,\[])\s*([a-z][a-zA-Z0-9_]*)\s*(?:,|\]|$)`)

// parseRebarConfig parses rebar.config deps + plugins.
func parseRebarConfig(source string) []dep {
	scrubbed := stripErlPercentComments(source)
	var out []dep
	seen := map[string]bool{}

	add := func(name, version, kind string) {
		if name == "" || seen[name] {
			return
		}
		seen[name] = true
		out = append(out, dep{name: name, version: version, kind: kind})
	}

	parseList := func(block, kind string) {
		// Tuple deps first (carry a version / source spec).
		tupleNames := map[string]bool{}
		for _, m := range rebarTupleDepRE.FindAllStringSubmatch(block, -1) {
			name := m[1]
			// Skip inner-tuple keywords (pkg/git/branch/tag/ref) which appear as
			// the head atom of a nested source tuple, not a dependency name.
			if rebarSourceKeyword(name) {
				continue
			}
			tupleNames[name] = true
			add(name, m[2], kind)
		}
		// Bare-atom deps — only those not already captured as a tuple head.
		for _, m := range rebarAtomDepRE.FindAllStringSubmatch(block, -1) {
			name := m[1]
			if tupleNames[name] || rebarSourceKeyword(name) {
				continue
			}
			add(name, "", kind)
		}
	}

	if m := rebarDepsBlockRE.FindStringSubmatch(scrubbed); m != nil {
		parseList(m[1], "runtime")
	}
	if m := rebarPluginsBlockRE.FindStringSubmatch(scrubbed); m != nil {
		parseList(m[1], "dev")
	}
	return out
}

// rebarSourceKeyword reports whether an atom is a rebar dep source-spec keyword
// (the head of a nested {pkg,...}/{git,...} tuple) rather than a dependency name.
func rebarSourceKeyword(atom string) bool {
	switch atom {
	case "pkg", "git", "git_subdir", "hg", "branch", "tag", "ref":
		return true
	}
	return false
}

// ---------------------------------------------------------------------------
// rebar.lock (rebar3 lockfile)
// ---------------------------------------------------------------------------

// rebarLockEntryRE matches a locked package entry:
//
//	{<<"cowboy">>,{pkg,<<"cowboy">>,<<"2.9.0">>},0},
//
// Group 1: dep name; Group 2: locked version (from the {pkg,...} tuple).
var rebarLockEntryRE = regexp.MustCompile(
	`\{<<"([a-z][a-zA-Z0-9_]*)">>\s*,\s*\{pkg\s*,\s*<<"[^"]*">>\s*,\s*<<"([^"]*)">>`)

func parseRebarLock(source string) []dep {
	scrubbed := stripErlPercentComments(source)
	var out []dep
	seen := map[string]bool{}
	for _, m := range rebarLockEntryRE.FindAllStringSubmatch(scrubbed, -1) {
		name := m[1]
		if seen[name] {
			continue
		}
		seen[name] = true
		out = append(out, dep{name: name, version: m[2], kind: "locked"})
	}
	return out
}

// ---------------------------------------------------------------------------
// *.app.src (Erlang/OTP application resource)
// ---------------------------------------------------------------------------

// appSrcApplicationsRE captures the {applications, [ ... ]} runtime-dep list.
var appSrcApplicationsRE = regexp.MustCompile(`(?s)\{\s*applications\s*,\s*\[(.*?)\]\s*\}`)

// appSrcAtomRE matches an application atom in the applications list.
var appSrcAtomRE = regexp.MustCompile(`[a-z][a-zA-Z0-9_]*`)

// otpStdlibApps are the OTP/stdlib applications every release ships with; they
// are not third-party dependencies and are excluded from the dep set.
var otpStdlibApps = map[string]bool{
	"kernel": true, "stdlib": true, "sasl": true, "crypto": true,
	"ssl": true, "public_key": true, "asn1": true, "inets": true,
	"runtime_tools": true, "tools": true, "compiler": true, "syntax_tools": true,
	"mnesia": true, "os_mon": true, "xmerl": true, "eunit": true,
}

func parseAppSrc(source string) []dep {
	scrubbed := stripErlPercentComments(source)
	var out []dep
	seen := map[string]bool{}
	m := appSrcApplicationsRE.FindStringSubmatch(scrubbed)
	if m == nil {
		return nil
	}
	for _, am := range appSrcAtomRE.FindAllString(m[1], -1) {
		if otpStdlibApps[am] || seen[am] {
			continue
		}
		seen[am] = true
		out = append(out, dep{name: am, kind: "runtime"})
	}
	return out
}

// ---------------------------------------------------------------------------
// erlang.mk / Makefile (erlang.mk build system)
// ---------------------------------------------------------------------------

// erlangMkDepsLineRE matches a `DEPS = a b c` (or TEST_DEPS / BUILD_DEPS /
// LOCAL_DEPS / DOC_DEPS / SHELL_DEPS) assignment line. Group 1 is the variant
// prefix (empty for plain DEPS), group 2 the space-separated dep list.
var erlangMkDepsLineRE = regexp.MustCompile(
	`(?m)^(TEST_|BUILD_|LOCAL_|DOC_|SHELL_)?DEPS\s*[:+]?=\s*(.*)$`)

// isErlangMk reports whether a Makefile is an erlang.mk-driven build (it
// includes erlang.mk or declares a PROJECT + DEPS in the erlang.mk style). Used
// so a plain (non-Erlang) Makefile is a no-op rather than a false build record.
func isErlangMk(source string) bool {
	if strings.Contains(source, "erlang.mk") {
		return true
	}
	// A Makefile that declares PROJECT and uses DEPS the erlang.mk way.
	return strings.Contains(source, "PROJECT =") || strings.Contains(source, "PROJECT=")
}

// parseErlangMk parses erlang.mk DEPS lines. For a Makefile, dependencies are
// only mined when the file is an erlang.mk build (isErlangMk) — a plain Makefile
// yields no deps (no-op).
func parseErlangMk(source string, requireSignal bool) []dep {
	if requireSignal && !isErlangMk(source) {
		return nil
	}
	var out []dep
	seen := map[string]bool{}
	for _, m := range erlangMkDepsLineRE.FindAllStringSubmatch(source, -1) {
		variant := m[1] // "TEST_", "BUILD_", ...
		isDev := variant == "TEST_" || variant == "DOC_" || variant == "SHELL_"
		kind := "runtime"
		if isDev {
			kind = "dev"
		}
		for _, name := range strings.Fields(m[2]) {
			// Skip make-variable references ($(...)) and obvious non-atoms.
			if name == "" || strings.HasPrefix(name, "$") {
				continue
			}
			if !erlAtomRE.MatchString(name) || seen[name] {
				continue
			}
			seen[name] = true
			out = append(out, dep{name: name, isDev: isDev, kind: kind})
		}
	}
	return out
}

// erlAtomRE matches a bare lowercase Erlang/erlang.mk dependency atom.
var erlAtomRE = regexp.MustCompile(`^[a-z][a-zA-Z0-9_]*$`)

// ---------------------------------------------------------------------------
// Helpers
// ---------------------------------------------------------------------------

// stripErlPercentComments blanks out Erlang `%`-to-end-of-line comments so a
// commented-out dep tuple is not parsed.
func stripErlPercentComments(src string) string {
	var b strings.Builder
	b.Grow(len(src))
	inComment := false
	for i := 0; i < len(src); i++ {
		c := src[i]
		switch {
		case c == '\n':
			inComment = false
			b.WriteByte(c)
		case c == '%':
			inComment = true
			b.WriteByte(' ')
		case inComment:
			b.WriteByte(' ')
		default:
			b.WriteByte(c)
		}
	}
	return b.String()
}
