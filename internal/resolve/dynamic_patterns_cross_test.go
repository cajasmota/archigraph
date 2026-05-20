package resolve

import "testing"

// TestDynamicPatterns_Cross covers cross-language patterns and negative cases
// that guard against false positives across all language gates.
func TestDynamicPatterns_Cross(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name string
		lang string
		stub string
		want bool
	}{
		// Cross-language: template-built strings with ${x} interpolation.
		{"interpolated_template_js", "javascript", "`prefix-${name}-suffix`", true},
		{"interpolated_template_unknown", "", "`prefix-${name}-suffix`", true},

		// Negative cases (must NOT be dynamic for any language).
		{"plain_kindname", "", `Function:Hello`, false},
		{"plain_bare_name", "", `Foo`, false},
		{"empty", "", ``, false},
		{"plain_call", "", `MyService.save()`, false},
		{"plain_attribute", "", `obj.attribute`, false},
		{"normal_function_call", "", `helper(x, y)`, false},
		{"structural_ref", "", `scope:operation:method:python:app/views.py:UserView#save`, false},
		{"ext_pkg", "", `ext:django`, false},

		// Cross-language collision guard: ensure unknown-language stubs don't match language-gated patterns.
		{"neg_unknown_lang_res_send", "", `res.send("hello")`, false},
		{"neg_unknown_lang_repo_lookup", "", `repo.Lookup(id)`, false},
	}

	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			got := isDynamicPatternLang(tc.stub, tc.lang)
			if got != tc.want {
				t.Fatalf("isDynamicPatternLang(%q, lang=%q) = %v, want %v", tc.stub, tc.lang, got, tc.want)
			}
		})
	}
}

// TestInferLangFromStub_StructuralRef confirms that structural-ref stubs
// carry their language in segment 3 of `scope:<kind>:<subtype>:<lang>:...`,
// so isDynamicPattern (the no-lang wrapper) routes them to the right
// per-language catalog without the caller having to thread language down.
func TestInferLangFromStub_StructuralRef(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name string
		stub string
		want string
	}{
		{"py_struct_ref", `scope:operation:method:python:app/views.py:UserView#save`, "python"},
		{"go_struct_ref", `scope:operation:method:go:internal/svc/handler.go:Handle`, "go"},
		{"js_struct_ref", `scope:operation:method:javascript:src/api.ts:request`, "javascript"},
		{"jvm_struct_ref", `scope:operation:method:java:src/Foo.java:bar`, "java"},
		{"non_struct", `Function:Hello`, ""},
		{"empty", ``, ""},
	}
	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			if got := inferLangFromStub(tc.stub); got != tc.want {
				t.Fatalf("inferLangFromStub(%q) = %q, want %q", tc.stub, got, tc.want)
			}
		})
	}
}
