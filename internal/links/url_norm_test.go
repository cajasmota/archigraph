package links

// url_norm_test.go tests normalizeURLPattern and the end-to-end HTTP pass
// confidence-boost logic for cross-repo candidates that differ only in
// path-parameter syntax (issue #2588).

import (
	"path/filepath"
	"testing"
)

// TestURLNorm_NormalizeURLPattern covers the pure normalizeURLPattern helper.
func TestURLNorm_NormalizeURLPattern(t *testing.T) {
	cases := []struct {
		name string
		in   string
		want string
	}{
		// Curly-brace params (OpenAPI / DRF router style).
		{"brace_id", "/users/{id}", "/users/<PARAM>"},
		{"brace_pk", "/users/{pk}", "/users/<PARAM>"},
		{"brace_multi", "/users/{id}/posts/{postId}", "/users/<PARAM>/posts/<PARAM>"},
		// Angle-bracket params (Django URL conf style).
		{"angle_plain", "/users/<id>", "/users/<PARAM>"},
		{"angle_typed", "/users/<pk:int>", "/users/<PARAM>"},
		{"angle_slug", "/items/<slug:name>", "/items/<PARAM>"},
		// Colon-prefix params (Express / Rails style).
		{"colon_id", "/users/:id", "/users/<PARAM>"},
		{"colon_pk", "/users/:pk", "/users/<PARAM>"},
		// Trailing slash stripped (unless root).
		{"trailing_slash", "/users/", "/users"},
		{"root_preserved", "/", "/"},
		// Query-string stripped.
		{"query_stripped", "/users?foo=bar", "/users"},
		{"query_with_param", "/users/{id}?include=profile", "/users/<PARAM>"},
		// Lowercased.
		{"uppercase_path", "/Users/Profile", "/users/profile"},
		// Mixed styles normalise identically.
		{"brace_vs_angle_eq", "/inspections/{id}", "/inspections/<PARAM>"},
		{"angle_typed_eq", "/inspections/<pk:int>", "/inspections/<PARAM>"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := normalizeURLPattern(tc.in); got != tc.want {
				t.Errorf("normalizeURLPattern(%q) = %q, want %q", tc.in, got, tc.want)
			}
		})
	}
}

// TestURLNorm_BraceVsAngleParam_Matches verifies that a client emitting
// /users/{id} and a server emitting /users/<id> are resolved as an HTTP link.
// normalizeURLPattern unifies both to /users/<PARAM>; the byPath index also
// handles this via pathParamRe, so the link is always emitted at high
// confidence regardless of which resolution path fires first.
func TestURLNorm_BraceVsAngleParam_Matches(t *testing.T) {
	root := fixtureRoot(t)
	writeFixture(t, root, fixtureGraph{
		Repo: "backend",
		Entities: []map[string]any{
			{
				"id": "h1", "name": "UserView", "kind": "Controller",
				"source_file": "app/views.py",
			},
			{
				// Server uses Django angle-bracket param syntax.
				"id": "ep1", "name": "http:GET:/users/<id>", "kind": "http_endpoint",
				"source_file": "app/views.py",
				"properties": map[string]any{
					"verb":         "GET",
					"path":         "/users/<id>",
					"framework":    "django",
					"pattern_type": "http_endpoint_synthesis",
				},
			},
		},
		Edges: []map[string]string{
			{"from_id": "h1", "to_id": "ep1", "kind": "IMPLEMENTS"},
		},
	})
	writeFixture(t, root, fixtureGraph{
		Repo: "frontend",
		Entities: []map[string]any{
			{
				"id": "fn1", "name": "loadUser", "kind": "Function",
				"source_file": "src/api.ts",
			},
			{
				// Client uses OpenAPI curly-brace param syntax.
				"id": "ep2", "name": "http:GET:/users/{id}", "kind": "http_endpoint",
				"source_file": "src/api.ts",
				"properties": map[string]any{
					"verb":          "GET",
					"path":          "/users/{id}",
					"framework":     "fetch",
					"pattern_type":  "http_endpoint_client_synthesis",
					"source_caller": "Function:loadUser",
				},
			},
		},
		Edges: []map[string]string{},
	})
	home := filepath.Join(root, "ag-home")
	if _, err := RunAllPasses("gnorm1", root, home); err != nil {
		t.Fatal(err)
	}
	doc, err := readDoc(filepath.Join(home, "groups", "gnorm1-links.json"))
	if err != nil {
		t.Fatal(err)
	}
	var hit *Link
	for i, l := range doc.Links {
		if l.Method == MethodHTTP {
			hit = &doc.Links[i]
			break
		}
	}
	if hit == nil {
		t.Fatalf("expected HTTP link for brace-vs-angle param match (/users/{id} vs /users/<id>), got %+v", doc.Links)
	}
	// Link must be emitted at high confidence (byPath index resolves these).
	if hit.Confidence < 0.9 {
		t.Errorf("expected confidence ≥ 0.9, got %v", hit.Confidence)
	}
}

// TestURLNorm_DjangoTypedParam_Matches verifies that a client emitting
// /inspections/{id} and a server emitting /inspections/<pk:int> (Django typed
// param) are resolved as an HTTP link. normalizeURLPattern unifies both to
// /inspections/<PARAM>; pathParamRe in the byPath index also collapses both to
// {*}, so the link is emitted at high confidence.
func TestURLNorm_DjangoTypedParam_Matches(t *testing.T) {
	root := fixtureRoot(t)
	writeFixture(t, root, fixtureGraph{
		Repo: "backend",
		Entities: []map[string]any{
			{
				"id": "h1", "name": "InspectionView", "kind": "Controller",
				"source_file": "app/views.py",
			},
			{
				// Server uses Django typed-param syntax.
				"id": "ep1", "name": "http:GET:/inspections/<pk:int>", "kind": "http_endpoint",
				"source_file": "app/views.py",
				"properties": map[string]any{
					"verb":         "GET",
					"path":         "/inspections/<pk:int>",
					"framework":    "django",
					"pattern_type": "http_endpoint_synthesis",
				},
			},
		},
		Edges: []map[string]string{
			{"from_id": "h1", "to_id": "ep1", "kind": "IMPLEMENTS"},
		},
	})
	writeFixture(t, root, fixtureGraph{
		Repo: "frontend",
		Entities: []map[string]any{
			{
				"id": "fn1", "name": "fetchInspection", "kind": "Function",
				"source_file": "src/api.ts",
			},
			{
				// Client uses curly-brace syntax (mirrors acme #2588 case).
				"id": "ep2", "name": "http:GET:/inspections/{id}", "kind": "http_endpoint",
				"source_file": "src/api.ts",
				"properties": map[string]any{
					"verb":          "GET",
					"path":          "/inspections/{id}",
					"framework":     "fetch",
					"pattern_type":  "http_endpoint_client_synthesis",
					"source_caller": "Function:fetchInspection",
				},
			},
		},
		Edges: []map[string]string{},
	})
	home := filepath.Join(root, "ag-home")
	if _, err := RunAllPasses("gnorm2", root, home); err != nil {
		t.Fatal(err)
	}
	doc, err := readDoc(filepath.Join(home, "groups", "gnorm2-links.json"))
	if err != nil {
		t.Fatal(err)
	}
	var hit *Link
	for i, l := range doc.Links {
		if l.Method == MethodHTTP {
			hit = &doc.Links[i]
			break
		}
	}
	if hit == nil {
		t.Fatalf("expected HTTP link for Django typed-param match (/inspections/{id} vs /inspections/<pk:int>), got %+v", doc.Links)
	}
	if hit.Confidence < 0.9 {
		t.Errorf("expected confidence ≥ 0.9, got %v", hit.Confidence)
	}
}

// TestURLNorm_QueryStringStripped verifies that a client emitting
// /users?foo=bar and a server emitting /users are matched after query-string
// stripping.
func TestURLNorm_QueryStringStripped(t *testing.T) {
	root := fixtureRoot(t)
	writeFixture(t, root, fixtureGraph{
		Repo: "backend",
		Entities: []map[string]any{
			{
				"id": "h1", "name": "UserListView", "kind": "Controller",
				"source_file": "app/views.py",
			},
			{
				"id": "ep1", "name": "http:GET:/users", "kind": "http_endpoint",
				"source_file": "app/views.py",
				"properties": map[string]any{
					"verb":         "GET",
					"path":         "/users",
					"framework":    "django",
					"pattern_type": "http_endpoint_synthesis",
				},
			},
		},
		Edges: []map[string]string{
			{"from_id": "h1", "to_id": "ep1", "kind": "IMPLEMENTS"},
		},
	})
	writeFixture(t, root, fixtureGraph{
		Repo: "frontend",
		Entities: []map[string]any{
			{
				"id": "fn1", "name": "listUsers", "kind": "Function",
				"source_file": "src/api.ts",
			},
			{
				// Client emits path with a query string.
				"id": "ep2", "name": "http:GET:/users?foo=bar", "kind": "http_endpoint",
				"source_file": "src/api.ts",
				"properties": map[string]any{
					"verb":          "GET",
					"path":          "/users?foo=bar",
					"framework":     "fetch",
					"pattern_type":  "http_endpoint_client_synthesis",
					"source_caller": "Function:listUsers",
				},
			},
		},
		Edges: []map[string]string{},
	})
	home := filepath.Join(root, "ag-home")
	if _, err := RunAllPasses("gnorm3", root, home); err != nil {
		t.Fatal(err)
	}
	doc, err := readDoc(filepath.Join(home, "groups", "gnorm3-links.json"))
	if err != nil {
		t.Fatal(err)
	}
	var hit *Link
	for i, l := range doc.Links {
		if l.Method == MethodHTTP {
			hit = &doc.Links[i]
			break
		}
	}
	if hit == nil {
		t.Fatalf("expected HTTP link for query-string stripped match (/users?foo=bar vs /users), got %+v", doc.Links)
	}
	if hit.Confidence < 0.9 {
		t.Errorf("expected boosted confidence ≥ 0.9, got %v", hit.Confidence)
	}
	if hit.Properties == nil || hit.Properties["normalization"] != "url_pattern" {
		t.Errorf("expected Properties[\"normalization\"]=\"url_pattern\", got %v", hit.Properties)
	}
}

// TestURLNorm_DifferentPaths_NoMatch verifies that /users and /products do
// NOT produce a false-positive URL-pattern normalized link.
func TestURLNorm_DifferentPaths_NoMatch(t *testing.T) {
	root := fixtureRoot(t)
	writeFixture(t, root, fixtureGraph{
		Repo: "backend",
		Entities: []map[string]any{
			{
				"id": "h1", "name": "ProductView", "kind": "Controller",
				"source_file": "app/views.py",
			},
			{
				"id": "ep1", "name": "http:GET:/products", "kind": "http_endpoint",
				"source_file": "app/views.py",
				"properties": map[string]any{
					"verb":         "GET",
					"path":         "/products",
					"framework":    "django",
					"pattern_type": "http_endpoint_synthesis",
				},
			},
		},
		Edges: []map[string]string{
			{"from_id": "h1", "to_id": "ep1", "kind": "IMPLEMENTS"},
		},
	})
	writeFixture(t, root, fixtureGraph{
		Repo: "frontend",
		Entities: []map[string]any{
			{
				"id": "fn1", "name": "listUsers", "kind": "Function",
				"source_file": "src/api.ts",
			},
			{
				// Client calls /users — completely different path from /products.
				"id": "ep2", "name": "http:GET:/users", "kind": "http_endpoint",
				"source_file": "src/api.ts",
				"properties": map[string]any{
					"verb":          "GET",
					"path":          "/users",
					"framework":     "fetch",
					"pattern_type":  "http_endpoint_client_synthesis",
					"source_caller": "Function:listUsers",
				},
			},
		},
		Edges: []map[string]string{},
	})
	home := filepath.Join(root, "ag-home")
	if _, err := RunAllPasses("gnorm4", root, home); err != nil {
		t.Fatal(err)
	}
	doc, err := readDoc(filepath.Join(home, "groups", "gnorm4-links.json"))
	if err != nil {
		t.Fatal(err)
	}
	for _, l := range doc.Links {
		if l.Method == MethodHTTP && l.Properties != nil && l.Properties["normalization"] == "url_pattern" {
			t.Errorf("unexpected url_pattern-normalised HTTP link for /users vs /products: %+v", l)
		}
	}
}
