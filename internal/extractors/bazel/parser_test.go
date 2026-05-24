package bazel

import (
	"testing"
)

// TestParseBUILD_Basic verifies parsing of a simple multi-rule BUILD file.
func TestParseBUILD_Basic(t *testing.T) {
	src := []byte(`
py_library(
    name = "mylib",
    srcs = ["lib.py"],
    deps = [
        "//other:dep",
        ":sibling",
    ],
)

py_binary(
    name = "mybin",
    srcs = ["main.py"],
    deps = [":mylib"],
)
`)
	rules, err := ParseBUILD(src, "services/foo", "services/foo/BUILD")
	if err != nil {
		t.Fatalf("ParseBUILD: %v", err)
	}
	if len(rules) != 2 {
		t.Fatalf("expected 2 rules, got %d", len(rules))
	}

	lib := rules[0]
	if lib.Kind != "py_library" {
		t.Errorf("rules[0].Kind = %q, want py_library", lib.Kind)
	}
	if lib.Name != "mylib" {
		t.Errorf("rules[0].Name = %q, want mylib", lib.Name)
	}
	if lib.Package != "services/foo" {
		t.Errorf("rules[0].Package = %q", lib.Package)
	}
	if lib.Label() != "//services/foo:mylib" {
		t.Errorf("rules[0].Label() = %q", lib.Label())
	}
	// ":sibling" should be resolved to "//services/foo:sibling".
	wantDeps := []string{"//other:dep", "//services/foo:sibling"}
	if !equalDeps(lib.Deps, wantDeps) {
		t.Errorf("rules[0].Deps = %v, want %v", lib.Deps, wantDeps)
	}

	bin := rules[1]
	if bin.Kind != "py_binary" {
		t.Errorf("rules[1].Kind = %q", bin.Kind)
	}
	if bin.Name != "mybin" {
		t.Errorf("rules[1].Name = %q", bin.Name)
	}
	// Single-line deps = [":mylib"] → resolved to //services/foo:mylib.
	if len(bin.Deps) != 1 || bin.Deps[0] != "//services/foo:mylib" {
		t.Errorf("rules[1].Deps = %v", bin.Deps)
	}
}

// TestParseBUILD_GoLibrary verifies parsing of rules_go-style go_library.
func TestParseBUILD_GoLibrary(t *testing.T) {
	src := []byte(`
go_library(
    name = "utils",
    importpath = "github.com/example/app/utils",
    srcs = ["utils.go"],
    deps = [],
    visibility = ["//visibility:public"],
)
`)
	rules, err := ParseBUILD(src, "common/utils", "common/utils/BUILD")
	if err != nil {
		t.Fatalf("ParseBUILD: %v", err)
	}
	if len(rules) != 1 {
		t.Fatalf("expected 1 rule, got %d", len(rules))
	}
	r := rules[0]
	if r.Kind != "go_library" {
		t.Errorf("Kind = %q", r.Kind)
	}
	if r.Name != "utils" {
		t.Errorf("Name = %q", r.Name)
	}
	if len(r.Deps) != 0 {
		t.Errorf("Deps should be empty, got %v", r.Deps)
	}
}

// TestParseBUILD_JavaBinary verifies Java rule parsing.
func TestParseBUILD_JavaBinary(t *testing.T) {
	src := []byte(`
java_binary(
    name = "server",
    main_class = "com.example.Main",
    deps = [
        ":server_lib",
        "@maven//:com_google_guava_guava",
    ],
)
`)
	rules, err := ParseBUILD(src, "cmd/server", "cmd/server/BUILD")
	if err != nil {
		t.Fatalf("ParseBUILD: %v", err)
	}
	if len(rules) != 1 {
		t.Fatalf("expected 1 rule, got %d: %+v", len(rules), rules)
	}
	r := rules[0]
	if r.Kind != "java_binary" {
		t.Errorf("Kind = %q", r.Kind)
	}
	if r.Name != "server" {
		t.Errorf("Name = %q", r.Name)
	}
	wantDeps := []string{"//cmd/server:server_lib", "@maven//:com_google_guava_guava"}
	if !equalDeps(r.Deps, wantDeps) {
		t.Errorf("Deps = %v, want %v", r.Deps, wantDeps)
	}
}

// TestParseBUILD_RootPackage verifies that rules in the root BUILD file (pkg="")
// have the correct label format "//:name".
func TestParseBUILD_RootPackage(t *testing.T) {
	src := []byte(`
py_library(
    name = "root_lib",
    srcs = ["lib.py"],
    deps = [],
)
`)
	rules, err := ParseBUILD(src, "", "BUILD")
	if err != nil {
		t.Fatalf("ParseBUILD: %v", err)
	}
	if len(rules) != 1 {
		t.Fatalf("expected 1 rule, got %d", len(rules))
	}
	if rules[0].Label() != "//:root_lib" {
		t.Errorf("Label() = %q, want //:root_lib", rules[0].Label())
	}
}

// TestParseBUILD_Comments verifies that comment-only lines and inline comments
// are handled without emitting spurious rules.
func TestParseBUILD_Comments(t *testing.T) {
	src := []byte(`
# Top-level comment
py_library(
    name = "lib",  # inline comment
    srcs = ["lib.py"],
    deps = [
        # commented-out dep
        "//other:dep",  # another inline comment
    ],
)
`)
	rules, err := ParseBUILD(src, "pkg", "pkg/BUILD")
	if err != nil {
		t.Fatalf("ParseBUILD: %v", err)
	}
	if len(rules) != 1 {
		t.Fatalf("expected 1 rule, got %d", len(rules))
	}
	r := rules[0]
	if r.Name != "lib" {
		t.Errorf("Name = %q", r.Name)
	}
	if len(r.Deps) != 1 || r.Deps[0] != "//other:dep" {
		t.Errorf("Deps = %v, want [//other:dep]", r.Deps)
	}
}

// TestParseBUILD_MultipleRuleKinds verifies that all rule suffix variants
// (library, binary, test) are accepted.
func TestParseBUILD_MultipleRuleKinds(t *testing.T) {
	src := []byte(`
cc_library(name = "cc_lib", srcs = [], deps = [])
cc_binary(name = "cc_bin", srcs = [], deps = [":cc_lib"])
cc_test(name = "cc_tst", srcs = [], deps = [":cc_lib"])
proto_library(name = "my_proto", srcs = ["api.proto"])
`)
	rules, err := ParseBUILD(src, "protos", "protos/BUILD")
	if err != nil {
		t.Fatalf("ParseBUILD: %v", err)
	}
	if len(rules) != 4 {
		t.Fatalf("expected 4 rules, got %d: %+v", len(rules), rules)
	}
}

// TestParseBUILD_StartLine verifies that StartLine is set correctly.
func TestParseBUILD_StartLine(t *testing.T) {
	src := []byte(`# line 1 comment
# line 2 comment

py_library(
    name = "lib",
    deps = [],
)
`)
	rules, err := ParseBUILD(src, "pkg", "pkg/BUILD")
	if err != nil {
		t.Fatalf("ParseBUILD: %v", err)
	}
	if len(rules) != 1 {
		t.Fatalf("expected 1 rule")
	}
	// py_library( is on line 4.
	if rules[0].StartLine != 4 {
		t.Errorf("StartLine = %d, want 4", rules[0].StartLine)
	}
}

// TestDetectRuleKind checks the internal detectRuleKind function.
func TestDetectRuleKind(t *testing.T) {
	cases := []struct {
		line string
		want string
		ok   bool
	}{
		{"py_library(", "py_library", true},
		{"java_binary(", "java_binary", true},
		{"go_test(", "go_test", true},
		{"custom_framework_library(", "custom_framework_library", true},
		{"proto_library(", "proto_library", true},
		{"alias(", "alias", true},
		{"filegroup(", "filegroup", true},
		{"name = \"foo\"", "", false},
		{"# py_library(", "", false},
		{"genrule(", "", false}, // not a recognised kind
	}
	for _, tc := range cases {
		got, ok := detectRuleKind(tc.line)
		if ok != tc.ok || (ok && got != tc.want) {
			t.Errorf("detectRuleKind(%q) = (%q, %v), want (%q, %v)",
				tc.line, got, ok, tc.want, tc.ok)
		}
	}
}

// TestParseDepsBuffer tests the internal dep list parser.
func TestParseDepsBuffer(t *testing.T) {
	buf := `
        "//a:b",
        ":c",  # inline comment
        "@ext//:dep",
`
	deps := parseDepsBuffer(buf, "pkg/sub")
	want := []string{"//a:b", "//pkg/sub:c", "@ext//:dep"}
	if !equalDeps(deps, want) {
		t.Errorf("parseDepsBuffer = %v, want %v", deps, want)
	}
}

// equalDeps compares two dep slices for equality (order-sensitive).
func equalDeps(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}
