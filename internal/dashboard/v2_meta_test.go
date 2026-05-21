package dashboard

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

// TestV2Meta_Shape verifies the /api/v2/meta response shape.
func TestV2Meta_Shape(t *testing.T) {
	srv, err := NewServer(DefaultConfig(), newFakeStore())
	if err != nil {
		t.Fatalf("NewServer: %v", err)
	}
	ts := httptest.NewServer(srv.routes())
	defer ts.Close()

	resp, err := http.Get(ts.URL + "/api/v2/meta")
	if err != nil {
		t.Fatalf("GET /api/v2/meta: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("want 200, got %d", resp.StatusCode)
	}
	if ct := resp.Header.Get("Content-Type"); ct != "application/json" {
		t.Errorf("Content-Type: want application/json, got %q", ct)
	}

	var body struct {
		OK   bool `json:"ok"`
		Data struct {
			Version string   `json:"version"`
			APIVers []string `json:"api_versions"`
			Groups  []string `json:"groups"`
		} `json:"data"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if !body.OK {
		t.Error("ok field: want true")
	}
	if body.Data.Version == "" {
		t.Error("version field is empty")
	}
	if len(body.Data.APIVers) == 0 {
		t.Error("api_versions field is empty")
	}
	found := false
	for _, v := range body.Data.APIVers {
		if v == "v2" {
			found = true
		}
	}
	if !found {
		t.Errorf("api_versions does not include 'v2': %v", body.Data.APIVers)
	}
}

// TestV2Meta_NonGETReturnsNonJSON verifies POST to /api/v2/meta does not
// return a JSON API response (the SPA handler absorbs unmatched methods,
// returning HTML; this confirms the API route is GET-only and the catch-all
// SPA does not expose an accidental JSON endpoint on POST).
func TestV2Meta_NonGETReturnsNonJSON(t *testing.T) {
	srv, err := NewServer(DefaultConfig(), newFakeStore())
	if err != nil {
		t.Fatalf("NewServer: %v", err)
	}
	ts := httptest.NewServer(srv.routes())
	defer ts.Close()

	resp, err := http.Post(ts.URL+"/api/v2/meta", "application/json", nil)
	if err != nil {
		t.Fatalf("POST /api/v2/meta: %v", err)
	}
	defer resp.Body.Close()

	// The SPA catch-all absorbs POSTs to unregistered-method API paths.
	// What matters is that we do NOT get the JSON API response (Content-Type
	// application/json) — confirming the v2 handler is GET-only.
	ct := resp.Header.Get("Content-Type")
	if ct == "application/json" {
		t.Errorf("POST /api/v2/meta should not return a JSON API response; got Content-Type: %q", ct)
	}
}

// TestV2Meta_V1RegistryStillWorks verifies the v1 /api/registry route is
// unaffected by the addition of v2 routes.
func TestV2Meta_V1RegistryStillWorks(t *testing.T) {
	st := newFakeStore()
	st.groups["legacy"] = GroupSummary{Name: "legacy", ConfigPath: "/x.json", Repos: []string{}}
	srv, err := NewServer(DefaultConfig(), st)
	if err != nil {
		t.Fatalf("NewServer: %v", err)
	}
	ts := httptest.NewServer(srv.routes())
	defer ts.Close()

	resp, err := http.Get(ts.URL + "/api/registry")
	if err != nil {
		t.Fatalf("GET /api/registry: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("v1 registry: want 200, got %d", resp.StatusCode)
	}
	var body struct {
		Groups []GroupSummary `json:"groups"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if len(body.Groups) != 1 || body.Groups[0].Name != "legacy" {
		t.Errorf("v1 registry returned unexpected groups: %+v", body.Groups)
	}
}
