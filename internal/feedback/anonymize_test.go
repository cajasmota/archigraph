package feedback

import (
	"strings"
	"testing"
)

func TestNameHash_BasicFormat(t *testing.T) {
	salt := []byte("testsalt123")
	h := NameHash("UserController", "class", salt)
	if !strings.HasPrefix(h, "ent_") {
		t.Errorf("expected ent_ prefix for class kind, got %q", h)
	}
	if len(h) != 8 { // "ent_" (4) + 4 hex chars (4)
		t.Errorf("expected length 8, got %d: %q", len(h), h)
	}
}

func TestNameHash_KindPrefixes(t *testing.T) {
	salt := []byte("salt")
	cases := []struct {
		kind   string
		prefix string
	}{
		{"function", "op_"},
		{"method", "op_"},
		{"class", "ent_"},
		{"struct", "ent_"},
		{"interface", "ent_"},
		{"module", "mod_"},
		{"http_endpoint", "ep_"},
		{"variable", "var_"},
		{"unknown_kind", "ent_"}, // fallback
	}
	for _, tc := range cases {
		h := NameHash("SomeName", tc.kind, salt)
		if !strings.HasPrefix(h, tc.prefix) {
			t.Errorf("kind=%q: expected prefix %q, got %q", tc.kind, tc.prefix, h)
		}
	}
}

func TestNameHash_SaltDifferentiates(t *testing.T) {
	name := "UserService"
	kind := "class"
	h1 := NameHash(name, kind, []byte("salt-a"))
	h2 := NameHash(name, kind, []byte("salt-b"))
	if h1 == h2 {
		t.Errorf("different salts should produce different hashes, both got %q", h1)
	}
}

func TestNameHash_SaltMakesStable(t *testing.T) {
	salt := []byte("stable-salt")
	h1 := NameHash("getUsers", "function", salt)
	h2 := NameHash("getUsers", "function", salt)
	if h1 != h2 {
		t.Errorf("same salt should produce same hash: %q vs %q", h1, h2)
	}
}

func TestNameHash_OnlyHexInSuffix(t *testing.T) {
	salt := []byte("hexcheck")
	h := NameHash("AnyName", "class", salt)
	suffix := strings.TrimPrefix(h, "ent_")
	for _, ch := range suffix {
		if !strings.ContainsRune("0123456789abcdef", ch) {
			t.Errorf("non-hex char %q in hash suffix %q", ch, suffix)
		}
	}
}

func TestPathScrub_BasicGo(t *testing.T) {
	got := PathScrub("internal/graph/load.go")
	if !strings.HasPrefix(got, "<go>/") {
		t.Errorf("expected <go>/ prefix, got %q", got)
	}
	if !strings.HasSuffix(got, ".go") {
		t.Errorf("expected .go suffix, got %q", got)
	}
}

func TestPathScrub_DeepPath(t *testing.T) {
	// depth 6 dirs + filename: should cap at 4 dir segments + <...>
	got := PathScrub("a/b/c/d/e/f/file.ts")
	if !strings.Contains(got, "<...>") {
		t.Errorf("expected <...> for depth > 5, got %q", got)
	}
}

func TestPathScrub_ShallowPath(t *testing.T) {
	got := PathScrub("src/main.py")
	if !strings.HasPrefix(got, "<py>/") {
		t.Errorf("expected <py>/ prefix, got %q", got)
	}
	if strings.Contains(got, "<...>") {
		t.Errorf("shallow path should not have <...>, got %q", got)
	}
}

func TestPathScrub_UnusualExt_Clojure(t *testing.T) {
	got := PathScrub("src/core/main.clj")
	if !strings.Contains(got, "<jvm-lang>") {
		t.Errorf("expected <jvm-lang> bucket for .clj, got %q", got)
	}
}

func TestPathScrub_UnusualExt_Elixir(t *testing.T) {
	got := PathScrub("lib/app/router.ex")
	if !strings.Contains(got, "<beam-lang>") {
		t.Errorf("expected <beam-lang> bucket for .ex, got %q", got)
	}
}

func TestPathScrub_NoRealSegments(t *testing.T) {
	got := PathScrub("a/b/c/MyController.java")
	// Should not contain "a", "b", "c", or "MyController"
	for _, bad := range []string{"a/", "/b/", "/c/", "MyController"} {
		if strings.Contains(got, bad) {
			t.Errorf("scrubbed path %q still contains raw segment %q", got, bad)
		}
	}
}

func TestPathScrub_PreservesSegCount(t *testing.T) {
	// 2 dirs + filename = 3 parts total
	got := PathScrub("src/utils/helper.go")
	// Expected: <go>/<seg-1>/<seg-2>.go — 3 slash-separated parts including the last
	parts := strings.Split(got, "/")
	// <go> + <seg-1> + <seg-2>.go = 3
	if len(parts) != 3 {
		t.Errorf("expected 3 parts, got %d: %q", len(parts), got)
	}
}
