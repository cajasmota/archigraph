// Issue #2691 integration guard: every Rails / Sinatra http_endpoint_definition
// must point at the handler's source file + line.
//
//   - Rails: route declared in config/routes.rb, handler lives in
//     app/controllers/<name>_controller.rb at the `def <action>` line.
//     The audit fixture uses both the explicit verb form
//     (`get '/things/search', to: 'things#search'`), the `resources :things`
//     macro (7 CRUD endpoints), and a `namespace :api do; resources :widgets end`
//     block (api/widgets_controller.rb).
//
//   - Sinatra: each `get '/x' do … end` block attributes to its own line in
//     the registration file (same-file by construction).
//
// Mirrors the DRF (#2677), Laravel (#2680), Gin/Echo (#2679), JS/TS (#2687)
// acceptance shape: source_file ends in the handler file, source_line matches
// the def line.
package main

import (
	"strings"
	"testing"

	"github.com/cajasmota/grafel/internal/graph"
)

func TestIssue2691_Rails_EndpointAttribution(t *testing.T) {
	doc := runIndexerOn(t, "testdata/audit2678_ruby/rails", "issue2691_rails", nil)

	type ep struct {
		Name string
		File string
		Line int
	}
	got := map[string]ep{}
	for _, e := range doc.Entities {
		if e.Kind != "http_endpoint_definition" && e.Kind != "http_endpoint" {
			continue
		}
		got[e.Name] = ep{Name: e.Name, File: e.SourceFile, Line: e.StartLine}
	}
	if len(got) < 8 {
		// Sanity floor: 1 explicit verb + 7 resources + 7 namespaced resources = 15.
		// Allow some slack but catch a regression to "nothing was synthesised".
		t.Fatalf("Rails fixture: expected >= 8 endpoint definitions, got %d (entities=%v)",
			len(got), rubyEndpointNames(doc.Entities))
	}

	// Acceptance per ticket — `GET /things` attributes to ThingsController#index,
	// `POST /things` to #create. Lines are 1-based against the fixture file.
	//
	// things_controller.rb (1-based):
	//   1: class ThingsController < ApplicationController
	//   2:   def index
	//   ...
	//   10:  def create
	//   ...
	cases := []struct {
		endpoint   string
		wantFile   string
		wantLine   int
		wantAction string
	}{
		{"http:GET:/things", "app/controllers/things_controller.rb", 2, "index"},
		{"http:POST:/things", "app/controllers/things_controller.rb", 10, "create"},
		{"http:GET:/things/{id}", "app/controllers/things_controller.rb", 6, "show"},
		{"http:GET:/things/search", "app/controllers/things_controller.rb", 30, "search"},
		// Namespaced resource: namespace :api do; resources :widgets end →
		// api/widgets_controller.rb. The Api:: module-prefix maps to a
		// lowercase subdirectory under app/controllers/ per Rails convention.
		{"http:GET:/api/widgets", "app/controllers/api/widgets_controller.rb", 3, "index"},
		{"http:POST:/api/widgets", "app/controllers/api/widgets_controller.rb", 11, "create"},
	}
	for _, tc := range cases {
		g, ok := got[tc.endpoint]
		if !ok {
			t.Errorf("endpoint %s missing — synthesized set: %v", tc.endpoint, rubyMapKeys(got))
			continue
		}
		if !strings.HasSuffix(strings.ReplaceAll(g.File, "\\", "/"), tc.wantFile) {
			t.Errorf("endpoint %s: source_file=%s, want suffix %s (registration site, not handler — #2691 rebind regressed)",
				tc.endpoint, g.File, tc.wantFile)
		}
		if g.Line != tc.wantLine {
			t.Errorf("endpoint %s: start_line=%d, want %d (def %s line)",
				tc.endpoint, g.Line, tc.wantLine, tc.wantAction)
		}
	}
}

func TestIssue2691_Sinatra_EndpointAttribution(t *testing.T) {
	doc := runIndexerOn(t, "testdata/audit2678_ruby/sinatra", "issue2691_sinatra", nil)

	type ep struct {
		Name string
		File string
		Line int
	}
	got := map[string]ep{}
	for _, e := range doc.Entities {
		if e.Kind != "http_endpoint_definition" && e.Kind != "http_endpoint" {
			continue
		}
		got[e.Name] = ep{Name: e.Name, File: e.SourceFile, Line: e.StartLine}
	}
	if len(got) < 4 {
		t.Fatalf("Sinatra fixture: expected >= 4 endpoint definitions, got %d (entities=%v)",
			len(got), rubyEndpointNames(doc.Entities))
	}

	// Sinatra fixture app.rb (1-based):
	//   1: require 'sinatra'
	//   2:
	//   3: get '/things' do
	//   ...
	//   8: post '/things' do
	//   ...
	//   13: get '/things/:id' do
	//   ...
	//   18: delete '/things/:id' do
	cases := []struct {
		endpoint string
		wantLine int
	}{
		{"http:GET:/things", 3},
		{"http:POST:/things", 8},
		{"http:GET:/things/{id}", 13},
		{"http:DELETE:/things/{id}", 18},
	}
	for _, tc := range cases {
		g, ok := got[tc.endpoint]
		if !ok {
			t.Errorf("endpoint %s missing — synthesized set: %v", tc.endpoint, rubyMapKeys(got))
			continue
		}
		if !strings.HasSuffix(strings.ReplaceAll(g.File, "\\", "/"), "app.rb") {
			t.Errorf("endpoint %s: source_file=%s, want suffix app.rb",
				tc.endpoint, g.File)
		}
		if g.Line != tc.wantLine {
			t.Errorf("endpoint %s: start_line=%d, want %d (verb block line)",
				tc.endpoint, g.Line, tc.wantLine)
		}
	}
}

// rubyEndpointNames returns just the Names of every http endpoint in a doc
// (debug helper for the #2691 acceptance tests).
func rubyEndpointNames(es []graph.Entity) []string {
	out := make([]string, 0, len(es))
	for _, e := range es {
		if e.Kind == "http_endpoint_definition" || e.Kind == "http_endpoint" {
			out = append(out, e.Name)
		}
	}
	return out
}

func rubyMapKeys[V any](m map[string]V) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	return out
}
