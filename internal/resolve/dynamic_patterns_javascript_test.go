package resolve

import "testing"

// TestDynamicPatterns_JavaScript covers the jsDynamicPatterns catalog
// for both javascript and typescript language tags.
func TestDynamicPatterns_JavaScript(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name string
		lang string
		stub string
		want bool
	}{
		{"js_reflect_apply", "javascript", `Reflect.apply(fn, this, args)`, true},
		{"js_reflect_construct", "javascript", `Reflect.construct(C, args)`, true},
		{"js_function_ctor", "javascript", `Function("return 1")`, true},
		{"js_new_function", "javascript", `new Function("return 1")`, true},
		{"js_dynamic_import_var", "javascript", `import(modName)`, true},
		{"js_require_dynamic_var", "javascript", `require(modName)`, true},
		{"js_process_env", "javascript", `process.env.NODE_ENV`, true},
		{"ts_reflect_apply", "typescript", `Reflect.apply(fn, this, args)`, true},
		// Negative: literal-string require/import must NOT be dynamic.
		{"neg_require_literal_dquote", "javascript", `require("fs")`, false},
		{"neg_require_literal_squote", "javascript", `require('fs')`, false},
		{"neg_import_literal", "javascript", `import("./literal-mod")`, false},
		// `res.send("hello")` in Node must NOT match Ruby `.send`.
		{"neg_node_res_send", "javascript", `res.send("hello")`, false},
		// `discount.apply(order)` — domain method, not Function.prototype.apply.
		{"neg_discount_apply", "javascript", `discount.apply(order)`, false},
		// `controller.call(...)` — domain method.
		{"neg_controller_call", "javascript", `controller.call(req, res)`, false},
		// `db.bind(":id", 1)` — DB driver bind.
		{"neg_db_bind", "javascript", `db.bind(":id", 1)`, false},
		// Spring builder names MUST NOT fire for JS.
		{"spring_js_ok_neg", "javascript", `ok`, false},
		{"spring_ts_noContent_neg", "typescript", `noContent`, false},
		// JVM bare invoke must not fire for JS.
		{"jvm_bare_invoke_js_negative", "javascript", `invoke`, false},
		// Rails names must not fire for JS.
		{"rb_rails_get_js_negative", "javascript", `resources`, false},
		// Elixir names must not fire for JS.
		{"elixir_render_ts_neg", "typescript", `render`, false},
		{"elixir_where_ts_neg", "typescript", `where`, false},
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
