package resolve

import "testing"

// TestDynamicPatterns_Razor covers the razorDynamicPatterns catalog.
//
// Audit baseline (issue #44, razor slice):
//
//	The razor extractor emits CALLS edges from @code block event-handler
//	bodies. Three categories of stubs appear as unresolved before this fix:
//
//	  1. PascalCase*Async service calls (e.g. GetForecastAsync, FetchItemsAsync)
//	     → were BugExtractor; after fix → Dynamic. These are interface-dispatch
//	     calls on @inject'd services — not statically bindable.
//
//	  2. HttpClient/IHttpClientFactory extension methods (GetFromJsonAsync,
//	     PostAsJsonAsync, etc.) → were BugExtractor; after fix → Dynamic.
//
//	  3. EventCallback bare names (OnChanged, OnSubmit, etc.) → were
//	     BugExtractor; after fix → Dynamic.
//
//	SQL slice: the sql extractor emits only READS_FROM / WRITES_TO edges.
//	Cross-file table references that lack a matching entity route to
//	DispositionExternalSQL (issue #507). No dynamic-pattern slice needed;
//	documented in the PR body.
func TestDynamicPatterns_Razor(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name string
		lang string
		stub string
		want bool
	}{
		// ---- Injected-service PascalCase*Async calls (new in razor slice) ----

		// Typical Blazor service method: await WeatherSvc.GetForecastAsync()
		// extractor strips receiver → bare "GetForecastAsync".
		{"service_get_forecast_async", "razor", "GetForecastAsync", true},
		{"service_fetch_items_async", "razor", "FetchItemsAsync", true},
		{"service_load_data_async", "razor", "LoadDataAsync", true},
		{"service_save_changes_async", "razor", "SaveChangesAsync", true},
		{"service_add_item_async", "razor", "AddItemAsync", true},
		{"service_delete_async_short", "razor", "DeleteUserAsync", true},
		// Short forms still match (≥ 6 char suffix, uppercase first letter).
		{"service_do_async", "razor", "DoAsync", true},

		// ---- Inherited C# patterns must still fire for "razor" ----

		// PascalCase namespace import (from csharpDynamicPatterns).
		{"csharp_namespace_import_razor", "razor", "MyApp.Services.Weather", true},
		// Generic static-factory calls (from csharpDynamicPatterns).
		{"quartz_jobbuilder_razor", "razor", "JobBuilder.Create<ReportJob>", true},
		// Fluent builder bare-name (from csharpDynamicPatterns).
		{"quartz_withidentity_razor", "razor", "WithIdentity", true},

		// ---- HttpClient extension methods ----

		{"httpclient_get_json_async", "razor", "GetFromJsonAsync", true},
		{"httpclient_post_json_async", "razor", "PostAsJsonAsync", true},
		{"httpclient_put_json_async", "razor", "PutAsJsonAsync", true},
		{"httpclient_delete_async", "razor", "DeleteAsync", true},
		{"httpclient_send_async", "razor", "SendAsync", true},
		{"httpclient_read_json_async", "razor", "ReadFromJsonAsync", true},

		// ---- EventCallback bare names ----

		{"event_callback_on_changed", "razor", "OnChanged", true},
		{"event_callback_on_clicked", "razor", "OnClicked", true},
		{"event_callback_on_submit", "razor", "OnSubmit", true},
		{"event_callback_on_valid_submit", "razor", "OnValidSubmit", true},
		{"event_callback_on_invalid_submit", "razor", "OnInvalidSubmit", true},

		// ---- JSInterop generic variants ----

		{"jsinterop_invoke_async_generic", "razor", "InvokeAsync<string>", true},
		{"jsinterop_invoke_void_async_generic", "razor", "InvokeVoidAsync<bool>", true},

		// ---- Patterns must NOT fire for other languages (language gate) ----

		// Go has no *Async convention — must not misfire.
		{"async_go_neg", "go", "GetForecastAsync", false},
		{"async_python_neg", "python", "FetchItemsAsync", false},
		{"async_java_neg", "java", "LoadDataAsync", false},
		{"async_typescript_neg", "typescript", "GetForecastAsync", false},
		{"async_ruby_neg", "ruby", "SaveChangesAsync", false},

		// HttpClient names must not fire outside razor.
		{"get_json_go_neg", "go", "GetFromJsonAsync", false},
		{"post_json_python_neg", "python", "PostAsJsonAsync", false},

		// EventCallback names must not fire outside razor.
		{"on_changed_go_neg", "go", "OnChanged", false},
		{"on_submit_python_neg", "python", "OnSubmit", false},

		// ---- Legitimate user-defined methods that SHOULD resolve (must not match) ----

		// Short bare names without Async suffix — private methods in the same
		// component that the resolver CAN bind from the entity index.
		// These must remain resolvable, not Dynamic.
		{"user_method_load_initial_data", "razor", "LoadInitialData", false},
		{"user_method_fetch_data", "razor", "FetchData", false},
		{"user_method_build_view_model", "razor", "BuildViewModel", false},
		// Lowercase names are always non-Dynamic (not PascalCase).
		{"lowercase_method_neg", "razor", "fetchData", false},
		{"lowercase_async_neg", "razor", "getAsync", false},
		// Single-letter type-param stub (e.g. a stray generic token) — must not match.
		{"single_char_t_neg", "razor", "T", false},
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

// TestDynamicPatterns_Razor_CatalogSize guards that the razor catalog is
// strictly larger than the csharp catalog (razor extends, not replaces).
func TestDynamicPatterns_Razor_CatalogSize(t *testing.T) {
	t.Parallel()
	razorLen := len(razorDynamicPatterns)
	csharpLen := len(csharpDynamicPatterns)
	if razorLen <= csharpLen {
		t.Fatalf("razorDynamicPatterns len=%d must be > csharpDynamicPatterns len=%d (razor extends csharp)", razorLen, csharpLen)
	}
}

// TestDynamicPatterns_Razor_RazorKeyRegistered verifies the "razor" language
// key in dynamicPatternsByLang resolves to the extended catalog.
func TestDynamicPatterns_Razor_RazorKeyRegistered(t *testing.T) {
	t.Parallel()
	patterns, ok := dynamicPatternsByLang["razor"]
	if !ok {
		t.Fatal("dynamicPatternsByLang[\"razor\"] not registered")
	}
	if len(patterns) != len(razorDynamicPatterns) {
		t.Fatalf("registered razor catalog len=%d, want %d (razorDynamicPatterns)", len(patterns), len(razorDynamicPatterns))
	}
}

// TestDynamicPatterns_SQL_NoSlice documents the audit finding: the sql
// extractor emits no CALLS edges (only READS_FROM / WRITES_TO to table names).
// Cross-file table stubs that lack a matching entity are routed to
// DispositionExternalSQL (issue #507). No dynamic-pattern entry is needed.
// This test serves as a regression guard for the "no-op" decision.
func TestDynamicPatterns_SQL_NoSlice(t *testing.T) {
	t.Parallel()
	// Confirm "sql" has no per-language dynamic pattern catalog.
	_, hasSQLCatalog := dynamicPatternsByLang["sql"]
	if hasSQLCatalog {
		t.Error("dynamicPatternsByLang[\"sql\"] should not exist: sql extractor emits no CALLS stubs (only READS_FROM/WRITES_TO routed to ExternalSQL)")
	}
	// Spot-check: a SQL stored-proc name must not accidentally hit Dynamic
	// via the cross-language catalog.
	if isDynamicPatternLang("usp_GetUserById", "sql") {
		t.Error("usp_GetUserById should NOT be Dynamic for sql (no sql catalog; cross-lang catalog must not fire)")
	}
	if isDynamicPatternLang("sp_GetOrders", "sql") {
		t.Error("sp_GetOrders should NOT be Dynamic for sql")
	}
}
