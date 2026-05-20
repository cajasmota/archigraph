package resolve

import "testing"

// TestDynamicPatterns_Elixir covers the elixirDynamicPatterns catalog.
func TestDynamicPatterns_Elixir(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name string
		lang string
		stub string
		want bool
	}{
		// Ecto.Repo query + mutation methods.
		{"elixir_ecto_all", "elixir", `all`, true},
		{"elixir_ecto_one", "elixir", `one`, true},
		{"elixir_ecto_get", "elixir", `get`, true},
		{"elixir_ecto_get_bang", "elixir", `get!`, true},
		{"elixir_ecto_get_by", "elixir", `get_by`, true},
		{"elixir_ecto_get_by_bang", "elixir", `get_by!`, true},
		{"elixir_ecto_preload", "elixir", `preload`, true},
		{"elixir_ecto_insert", "elixir", `insert`, true},
		{"elixir_ecto_insert_bang", "elixir", `insert!`, true},
		{"elixir_ecto_update", "elixir", `update`, true},
		{"elixir_ecto_update_bang", "elixir", `update!`, true},
		{"elixir_ecto_delete", "elixir", `delete`, true},
		{"elixir_ecto_delete_bang", "elixir", `delete!`, true},
		{"elixir_ecto_transaction", "elixir", `transaction`, true},
		{"elixir_ecto_insert_all", "elixir", `insert_all`, true},
		{"elixir_ecto_update_all", "elixir", `update_all`, true},
		{"elixir_ecto_delete_all", "elixir", `delete_all`, true},
		// Phoenix.Conn pipeline helpers.
		{"elixir_phoenix_render", "elixir", `render`, true},
		{"elixir_phoenix_json", "elixir", `json`, true},
		{"elixir_phoenix_text", "elixir", `text`, true},
		{"elixir_phoenix_html", "elixir", `html`, true},
		{"elixir_phoenix_send_resp", "elixir", `send_resp`, true},
		{"elixir_phoenix_put_flash", "elixir", `put_flash`, true},
		{"elixir_phoenix_redirect", "elixir", `redirect`, true},
		{"elixir_phoenix_halt", "elixir", `halt`, true},
		{"elixir_phoenix_assign", "elixir", `assign`, true},
		{"elixir_phoenix_put_session", "elixir", `put_session`, true},
		{"elixir_phoenix_get_session", "elixir", `get_session`, true},
		{"elixir_phoenix_put_resp_content_type", "elixir", `put_resp_content_type`, true},
		{"elixir_phoenix_put_resp_header", "elixir", `put_resp_header`, true},
		{"elixir_phoenix_fetch_session", "elixir", `fetch_session`, true},
		// GenServer / OTP behaviour callbacks.
		{"elixir_otp_init", "elixir", `init`, true},
		{"elixir_otp_handle_call", "elixir", `handle_call`, true},
		{"elixir_otp_handle_cast", "elixir", `handle_cast`, true},
		{"elixir_otp_handle_info", "elixir", `handle_info`, true},
		{"elixir_otp_handle_continue", "elixir", `handle_continue`, true},
		{"elixir_otp_terminate", "elixir", `terminate`, true},
		{"elixir_otp_code_change", "elixir", `code_change`, true},
		{"elixir_otp_start_link", "elixir", `start_link`, true},
		{"elixir_otp_child_spec", "elixir", `child_spec`, true},
		// Ecto.Changeset builder methods.
		{"elixir_cs_cast", "elixir", `cast`, true},
		{"elixir_cs_cast_assoc", "elixir", `cast_assoc`, true},
		{"elixir_cs_validate_required", "elixir", `validate_required`, true},
		{"elixir_cs_validate_length", "elixir", `validate_length`, true},
		{"elixir_cs_validate_format", "elixir", `validate_format`, true},
		{"elixir_cs_validate_inclusion", "elixir", `validate_inclusion`, true},
		{"elixir_cs_put_assoc", "elixir", `put_assoc`, true},
		{"elixir_cs_put_change", "elixir", `put_change`, true},
		{"elixir_cs_change", "elixir", `change`, true},
		{"elixir_cs_changeset", "elixir", `changeset`, true},
		{"elixir_cs_add_error", "elixir", `add_error`, true},
		{"elixir_cs_apply_action", "elixir", `apply_action`, true},
		{"elixir_cs_get_field", "elixir", `get_field`, true},
		{"elixir_cs_get_change", "elixir", `get_change`, true},
		// Ecto.Query DSL.
		{"elixir_query_from", "elixir", `from`, true},
		{"elixir_query_where", "elixir", `where`, true},
		{"elixir_query_select", "elixir", `select`, true},
		{"elixir_query_order_by", "elixir", `order_by`, true},
		{"elixir_query_group_by", "elixir", `group_by`, true},
		{"elixir_query_join", "elixir", `join`, true},
		{"elixir_query_limit", "elixir", `limit`, true},
		{"elixir_query_offset", "elixir", `offset`, true},
		{"elixir_query_distinct", "elixir", `distinct`, true},
		{"elixir_query_fragment", "elixir", `fragment`, true},
		// Logger module calls.
		{"elixir_logger_debug", "elixir", `debug`, true},
		{"elixir_logger_info", "elixir", `info`, true},
		{"elixir_logger_warning", "elixir", `warning`, true},
		{"elixir_logger_error", "elixir", `error`, true},
		// Cross-language gate: Elixir patterns MUST NOT fire for other langs.
		{"elixir_all_go_neg", "go", `all`, false},
		{"elixir_all_python_neg", "python", `all`, false},
		{"elixir_all_ruby_neg", "ruby", `all`, false},
		{"elixir_all_js_neg", "javascript", `all`, false},
		{"elixir_get_go_neg", "go", `get`, false},
		{"elixir_get_python_neg", "python", `get`, false},
		{"elixir_get_java_neg", "java", `get`, false},
		{"elixir_insert_go_neg", "go", `insert`, false},
		{"elixir_insert_python_neg", "python", `insert`, false},
		{"elixir_insert_ruby_neg", "ruby", `insert`, false},
		{"elixir_render_go_neg", "go", `render`, false},
		{"elixir_render_python_neg", "python", `render`, false},
		{"elixir_cast_go_neg", "go", `cast`, false},
		{"elixir_cast_python_neg", "python", `cast`, false},
		{"elixir_cast_java_neg", "java", `cast`, false},
		{"elixir_init_go_neg", "go", `init`, false},
		{"elixir_init_python_neg", "python", `init`, false},
		{"elixir_init_java_neg", "java", `init`, false},
		{"elixir_change_go_neg", "go", `change`, false},
		{"elixir_change_python_neg", "python", `change`, false},
		{"elixir_from_go_neg", "go", `from`, false},
		{"elixir_from_python_neg", "python", `from`, false},
		{"elixir_where_go_neg", "go", `where`, false},
		{"elixir_join_go_neg", "go", `join`, false},
		{"elixir_join_python_neg", "python", `join`, false},
		{"elixir_limit_go_neg", "go", `limit`, false},
		{"elixir_limit_python_neg", "python", `limit`, false},
		{"elixir_json_go_neg", "go", `json`, false},
		{"elixir_json_python_neg", "python", `json`, false},
		{"elixir_debug_go_neg", "go", `debug`, false},
		{"elixir_info_go_neg", "go", `info`, false},
		{"elixir_error_go_neg", "go", `error`, false},
		{"elixir_update_go_neg", "go", `update`, false},
		{"elixir_update_python_neg", "python", `update`, false},
		{"elixir_delete_go_neg", "go", `delete`, false},
		// NOTE: `delete` is also in Python/SQLAlchemy patterns, so we skip the python gate test here.
		{"elixir_halt_go_neg", "go", `halt`, false},
		{"elixir_redirect_go_neg", "go", `redirect`, false},
		{"elixir_transaction_go_neg", "go", `transaction`, false},
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
