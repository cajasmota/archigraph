package resolve

import "testing"

// TestDynamicPatterns_Go covers the goDynamicPatterns catalog.
func TestDynamicPatterns_Go(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name string
		lang string
		stub string
		want bool
	}{
		{"go_reflect_call", "go", `reflect.Value.Call`, true},
		{"go_reflect_valueof", "go", `reflect.ValueOf(x)`, true},
		{"go_method_by_name", "go", `v.MethodByName("Foo").Call(args)`, true},
		{"go_field_by_name", "go", `v.FieldByName("X")`, true},
		{"go_plugin_open", "go", `plugin.Open("./mod.so")`, true},
		{"go_plugin_lookup", "go", `plugin.Lookup("Sym")`, true},
		// Negative: `repo.Lookup(id)` in Go MUST NOT match `plugin.Lookup`.
		{"neg_go_repo_lookup", "go", `repo.Lookup(id)`, false},
		// Extra: ensure receiver-anchored Lookup is required for Go.
		{"neg_unknown_lang_repo_lookup", "", `repo.Lookup(id)`, false},
		// Spring builder names MUST NOT fire for Go.
		{"spring_go_body_neg", "go", `body`, false},
		// Rust channel must not fire for Go.
		{"rust_channel_go_neg", "go", `channel`, false},
		// JVM bare newInstance must not fire for Go.
		{"jvm_bare_newInstance_go_negative", "go", `newInstance`, false},
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
