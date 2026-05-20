package resolve

import "testing"

// TestDynamicPatterns_Rust covers the rustDynamicPatterns catalog.
func TestDynamicPatterns_Rust(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name string
		lang string
		stub string
		want bool
	}{
		// Bare channel-constructor names.
		{"rust_channel_bare", "rust", `channel`, true},
		// Generic-receiver method stubs.
		{"rust_receiver_string_recv", "rust", `Receiver<String>.recv`, true},
		{"rust_sender_string_send", "rust", `Sender<String>.send`, true},
		{"rust_receiver_u64_next", "rust", `Receiver<u64>.next`, true},
		{"rust_vec_u8_push", "rust", `Vec<u8>.push`, true},
		{"rust_option_string_unwrap", "rust", `Option<String>.unwrap`, true},
		{"rust_arc_mutex_lock", "rust", `Arc<Mutex<State>>.lock`, true},
		// Cross-language gate: Rust stubs MUST NOT fire for other languages.
		{"rust_channel_go_neg", "go", `channel`, false},
		{"rust_channel_python_neg", "python", `channel`, false},
		{"rust_channel_java_neg", "java", `channel`, false},
		{"rust_channel_kotlin_neg", "kotlin", `channel`, false},
		{"rust_receiver_recv_go_neg", "go", `Receiver<String>.recv`, false},
		{"rust_receiver_recv_python_neg", "python", `Receiver<String>.recv`, false},
		{"rust_receiver_recv_java_neg", "java", `Receiver<String>.recv`, false},
		// Patterns that look similar but should NOT be dynamic in Rust itself.
		{"rust_store_get_no_generic_neg", "rust", `Store.get`, false},
		{"rust_lowercase_recv_neg", "rust", `foo.recv`, false},
		// Rails set_callback must not fire for rust.
		{"rb_rails_set_callback_rust_negative", "rust", `set_callback`, false},
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
