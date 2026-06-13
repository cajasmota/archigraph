package manifest

import (
	"testing"
)

// depNames returns the sorted-insensitive set of dependency entity names.
func depNamesSet(t *testing.T, filePath, source string) map[string]string {
	t.Helper()
	records := runExtract(t, filePath, source)
	out := map[string]string{}
	for _, r := range records {
		if r.Kind == "SCOPE.Component" && r.Subtype == "external_dependency" {
			out[r.Name] = r.Properties["package_manager"]
		}
	}
	return out
}

// ---------------------------------------------------------------------------
// rebar.config (rebar3 / hex) — happy path
// ---------------------------------------------------------------------------

func TestRebarConfig_Deps(t *testing.T) {
	src := `
{erl_opts, [debug_info]}.
{deps, [
    cowboy,
    {jsx, "3.1.0"},
    {ranch, {pkg, <<"ranch">>, <<"2.1.0">>}},
    {mylib, {git, "https://github.com/me/mylib.git", {branch, "main"}}}
]}.
{plugins, [rebar3_hex]}.
`
	got := depNamesSet(t, "/proj/rebar.config", src)
	for _, want := range []string{"cowboy", "jsx", "ranch", "mylib", "rebar3_hex"} {
		if got[want] != "rebar3" {
			t.Errorf("expected dep %q with pm=rebar3, got pm=%q (deps: %v)", want, got[want], got)
		}
	}
	// Source-spec keywords must NOT leak in as deps.
	for _, bad := range []string{"pkg", "git", "branch", "ranch_2"} {
		if _, ok := got[bad]; ok && bad != "ranch" {
			if bad == "pkg" || bad == "git" || bad == "branch" {
				t.Errorf("source-keyword %q leaked as a dependency", bad)
			}
		}
	}
}

func TestRebarConfig_VersionCarried(t *testing.T) {
	src := `{deps, [{jsx, "3.1.0"}]}.`
	records := runExtract(t, "/proj/rebar.config", src)
	found := false
	for _, r := range records {
		if r.Subtype == "external_dependency" && r.Name == "jsx" {
			found = true
			if r.Properties["version"] != "3.1.0" {
				t.Errorf("jsx version = %q, want 3.1.0", r.Properties["version"])
			}
		}
	}
	if !found {
		t.Fatal("jsx dependency not emitted")
	}
}

// ---------------------------------------------------------------------------
// rebar.lock — locked versions
// ---------------------------------------------------------------------------

func TestRebarLock_Locked(t *testing.T) {
	src := `{"1.2.0",
[{<<"cowboy">>,{pkg,<<"cowboy">>,<<"2.9.0">>},0},
 {<<"ranch">>,{pkg,<<"ranch">>,<<"1.8.0">>},1}]}.
`
	records := runExtract(t, "/proj/rebar.lock", src)
	want := map[string]string{"cowboy": "2.9.0", "ranch": "1.8.0"}
	seen := map[string]bool{}
	for _, r := range records {
		if r.Subtype != "external_dependency" {
			continue
		}
		seen[r.Name] = true
		if r.Properties["version"] != want[r.Name] {
			t.Errorf("%s version=%q want %q", r.Name, r.Properties["version"], want[r.Name])
		}
		if r.Properties["dependency_kind"] != "locked" {
			t.Errorf("%s dependency_kind=%q want locked", r.Name, r.Properties["dependency_kind"])
		}
	}
	for k := range want {
		if !seen[k] {
			t.Errorf("locked dep %q not emitted", k)
		}
	}
}

// ---------------------------------------------------------------------------
// *.app.src — application runtime deps (OTP stdlib filtered)
// ---------------------------------------------------------------------------

func TestAppSrc_Applications(t *testing.T) {
	src := `{application, myapp,
 [{description, "My app"},
  {vsn, "0.1.0"},
  {applications, [kernel, stdlib, cowboy, jsx]},
  {mod, {myapp_app, []}}
 ]}.
`
	got := depNamesSet(t, "/proj/src/myapp.app.src", src)
	if got["cowboy"] != "rebar3" || got["jsx"] != "rebar3" {
		t.Errorf("expected cowboy+jsx as rebar3 deps, got %v", got)
	}
	if _, ok := got["kernel"]; ok {
		t.Error("OTP stdlib app kernel must be filtered out")
	}
	if _, ok := got["stdlib"]; ok {
		t.Error("OTP stdlib app stdlib must be filtered out")
	}
}

// ---------------------------------------------------------------------------
// erlang.mk / Makefile — DEPS lines
// ---------------------------------------------------------------------------

func TestErlangMk_Deps(t *testing.T) {
	src := `PROJECT = myapp
DEPS = cowboy jsx
TEST_DEPS = meck
dep_cowboy = git https://github.com/ninenines/cowboy 2.9.0

include erlang.mk
`
	got := depNamesSet(t, "/proj/Makefile", src)
	if got["cowboy"] != "erlang_mk" || got["jsx"] != "erlang_mk" {
		t.Errorf("expected cowboy+jsx as erlang_mk deps, got %v", got)
	}
	// TEST_DEPS → dev.
	records := runExtract(t, "/proj/Makefile", src)
	for _, r := range records {
		if r.Name == "meck" && r.Subtype == "external_dependency" {
			if r.Properties["is_dev"] != "true" {
				t.Errorf("meck (TEST_DEPS) is_dev=%q want true", r.Properties["is_dev"])
			}
		}
	}
}

// ---------------------------------------------------------------------------
// Negative: plain (non-erlang.mk) Makefile is a no-op
// ---------------------------------------------------------------------------

func TestErlangMk_PlainMakefileNoOp(t *testing.T) {
	src := `CC = gcc
DEPS = libfoo libbar
build:
	$(CC) -o app main.c
`
	got := depNamesSet(t, "/proj/Makefile", src)
	if len(got) != 0 {
		t.Errorf("plain Makefile (no erlang.mk/PROJECT signal) should yield no deps, got %v", got)
	}
}

// ---------------------------------------------------------------------------
// Negative: wrong-language file / no-match → no-op
// ---------------------------------------------------------------------------

func TestRebarConfig_NoDepsNoOp(t *testing.T) {
	src := `{erl_opts, [debug_info]}.` // no {deps, ...}
	got := depNamesSet(t, "/proj/rebar.config", src)
	if len(got) != 0 {
		t.Errorf("rebar.config with no deps should yield no deps, got %v", got)
	}
}

func TestErlang_NotAManifest(t *testing.T) {
	// A random .erl source is not a manifest at all → IsManifest false.
	if IsManifest("/proj/src/foo.erl") {
		t.Error("foo.erl must not be treated as a manifest")
	}
	if !IsManifest("/proj/rebar.config") {
		t.Error("rebar.config must be a manifest")
	}
	if !IsManifest("/proj/src/myapp.app.src") {
		t.Error("*.app.src must be a manifest")
	}
}
