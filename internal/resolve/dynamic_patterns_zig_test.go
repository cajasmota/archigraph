package resolve

import "testing"

// TestDynamicPatterns_Zig covers the zigDynamicPatterns catalog.
func TestDynamicPatterns_Zig(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name string
		lang string
		stub string
		want bool
	}{
		// Zig dynamic patterns (issue #44 — Zig resolver slice).
		// All Zig std lib namespaces (std.mem, std.fmt, std.ascii, std.json,
		// std.meta, std.ArrayList) have names unavailable as in-tree entities.
		//
		// Tier 1: Zig-unique std.mem split/tokenize family.
		{"zig_splitScalar", "zig", `splitScalar`, true},
		{"zig_splitSequence", "zig", `splitSequence`, true},
		{"zig_splitBackwardsScalar", "zig", `splitBackwardsScalar`, true},
		{"zig_tokenizeScalar", "zig", `tokenizeScalar`, true},
		{"zig_tokenizeSequence", "zig", `tokenizeSequence`, true},
		{"zig_tokenizeAny", "zig", `tokenizeAny`, true},
		// Tier 1: Zig-unique std.mem search functions.
		{"zig_indexOfScalar", "zig", `indexOfScalar`, true},
		{"zig_lastIndexOfScalar", "zig", `lastIndexOfScalar`, true},
		{"zig_indexOfPosLinear", "zig", `indexOfPosLinear`, true},
		{"zig_containsAtLeast", "zig", `containsAtLeast`, true},
		// Tier 1: Zig-unique std.ascii functions.
		{"zig_eqlIgnoreCase", "zig", `eqlIgnoreCase`, true},
		{"zig_isPrint", "zig", `isPrint`, true},
		{"zig_isAlpha", "zig", `isAlpha`, true},
		{"zig_isDigit", "zig", `isDigit`, true},
		{"zig_toUpper", "zig", `toUpper`, true},
		{"zig_toLower", "zig", `toLower`, true},
		{"zig_lowerString", "zig", `lowerString`, true},
		// Tier 1: Zig-unique std.meta / std.fmt / std.json.
		{"zig_stringToEnum", "zig", `stringToEnum`, true},
		{"zig_toOwnedSlice", "zig", `toOwnedSlice`, true},
		{"zig_parseFromSlice", "zig", `parseFromSlice`, true},
		{"zig_allocPrint", "zig", `allocPrint`, true},
		{"zig_bufPrint", "zig", `bufPrint`, true},
		{"zig_parseInt", "zig", `parseInt`, true},
		// Tier 2: Zig std lib leaf names gated to lang=="zig".
		{"zig_stringify", "zig", `stringify`, true},
		{"zig_writeAll", "zig", `writeAll`, true},
		{"zig_startsWith", "zig", `startsWith`, true},
		{"zig_endsWith", "zig", `endsWith`, true},
		{"zig_trim", "zig", `trim`, true},
		{"zig_trimLeft", "zig", `trimLeft`, true},
		{"zig_trimRight", "zig", `trimRight`, true},
		{"zig_eql", "zig", `eql`, true},
		{"zig_writeVecAll", "zig", `writeVecAll`, true},
		{"zig_print", "zig", `print`, true},
		{"zig_assert", "zig", `assert`, true},
		{"zig_clamp", "zig", `clamp`, true},
		{"zig_parseInt_zig_only", "zig", `parseFloat`, true},
		// Cross-language gate: Zig patterns MUST NOT fire for other languages.
		// splitScalar/tokenizeScalar are entirely Zig-specific names.
		{"zig_splitScalar_go_neg", "go", `splitScalar`, false},
		{"zig_splitScalar_python_neg", "python", `splitScalar`, false},
		{"zig_splitSequence_js_neg", "javascript", `splitSequence`, false},
		{"zig_tokenizeScalar_ruby_neg", "ruby", `tokenizeScalar`, false},
		{"zig_indexOfScalar_go_neg", "go", `indexOfScalar`, false},
		{"zig_indexOfScalar_python_neg", "python", `indexOfScalar`, false},
		{"zig_eqlIgnoreCase_go_neg", "go", `eqlIgnoreCase`, false},
		{"zig_eqlIgnoreCase_java_neg", "java", `eqlIgnoreCase`, false},
		{"zig_stringToEnum_go_neg", "go", `stringToEnum`, false},
		{"zig_stringToEnum_ruby_neg", "ruby", `stringToEnum`, false},
		{"zig_toOwnedSlice_go_neg", "go", `toOwnedSlice`, false},
		{"zig_toOwnedSlice_python_neg", "python", `toOwnedSlice`, false},
		{"zig_parseFromSlice_go_neg", "go", `parseFromSlice`, false},
		{"zig_writeAll_go_neg", "go", `writeAll`, false},
		{"zig_writeAll_java_neg", "java", `writeAll`, false},
		{"zig_trim_go_neg", "go", `trim`, false},
		{"zig_trim_python_neg", "python", `trim`, false},
		{"zig_startsWith_go_neg", "go", `startsWith`, false},
		{"zig_startsWith_js_neg", "javascript", `startsWith`, false},
		{"zig_stringify_go_neg", "go", `stringify`, false},
		{"zig_stringify_js_neg", "javascript", `stringify`, false},
		{"zig_print_go_neg", "go", `print`, false},
		{"zig_print_python_neg", "python", `print`, false},
		{"zig_assert_go_neg", "go", `assert`, false},
		{"zig_assert_python_neg", "python", `assert`, false},
		{"zig_clamp_go_neg", "go", `clamp`, false},
		{"zig_clamp_python_neg", "python", `clamp`, false},
		{"zig_parseInt_go_neg", "go", `parseInt`, false},
		{"zig_eql_go_neg", "go", `eql`, false},
		{"zig_eql_python_neg", "python", `eql`, false},
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
