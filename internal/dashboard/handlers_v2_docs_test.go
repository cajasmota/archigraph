package dashboard

import (
	"archive/zip"
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/cajasmota/grafel/internal/daemon"
	"github.com/cajasmota/grafel/internal/registry"
)

// daemonBusinessDocsDir returns (and creates) the post-#1624 group-level
// business docs directory for tests.
func daemonBusinessDocsDir(t *testing.T, group string) string {
	t.Helper()
	dir := daemon.BusinessDocsDir(group)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	return dir
}

func contains(s, substr string) bool   { return strings.Contains(s, substr) }
func startsWith(s, prefix string) bool { return strings.HasPrefix(s, prefix) }
func bytesReaderAt(b []byte) *bytes.Reader {
	return bytes.NewReader(b)
}

// buildV2DocsTestServer creates a Server with one group "testgrp" backed by a
// registry config + on-disk generated markdown docs under <repo>/docs/.
// Returns the server and the repo path so callers can inspect the layout.
func buildV2DocsTestServer(t *testing.T) *Server {
	t.Helper()

	home := t.TempDir()
	t.Setenv("GRAFEL_HOME", home)

	repoPath := t.TempDir()
	docsDir := filepath.Join(repoPath, "docs")

	// Lay out a representative generated-docs tree.
	mustWrite(t, filepath.Join(docsDir, "overview.md"), "# Repo One\n\nTop-level overview.\n")
	mustWrite(t, filepath.Join(docsDir, "modules", "order-service", "README.md"), "# Order Service\n\nModule deep-dive.\n")
	mustWrite(t, filepath.Join(docsDir, "reference", "api.md"), "# API Reference\n\nEndpoints.\n")
	mustWrite(t, filepath.Join(docsDir, "patterns", "structural", "repository.md"), "# Repository Pattern\n")
	// Enrichment frontmatter files must be excluded from the portal tree.
	mustWrite(t, filepath.Join(docsDir, "enrichments", "http_endpoint", "ep1.md"), "---\nsummary: x\n---\n")

	// Register the group config pointing at the repo.
	cfgPath := filepath.Join(home, "testgrp.json")
	cfg := &registry.GroupConfig{
		Name:  "testgrp",
		Repos: []registry.Repo{{Slug: "repo1", Path: repoPath}},
	}
	if err := registry.SaveGroupConfig(cfgPath, cfg); err != nil {
		t.Fatalf("SaveGroupConfig: %v", err)
	}
	if err := registry.AddGroup("testgrp", cfgPath); err != nil {
		t.Fatalf("AddGroup: %v", err)
	}

	srv, err := NewServer(DefaultConfig(), newFakeStore())
	if err != nil {
		t.Fatalf("NewServer: %v", err)
	}
	// Group must exist in the in-memory cache for the handlers' guard check.
	srv.graphs.mu.Lock()
	srv.graphs.entries["testgrp"] = &cacheEntry{
		group:    &DashGroup{Name: "testgrp", Repos: map[string]*DashRepo{"repo1": {Slug: "repo1", Path: repoPath}}},
		loadedAt: time.Now().Add(60 * time.Second),
	}
	srv.graphs.mu.Unlock()

	return srv
}

func mustWrite(t *testing.T, path, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("write %s: %v", path, err)
	}
}

func TestHandleV2DocsTree(t *testing.T) {
	srv := buildV2DocsTestServer(t)
	r := httptest.NewRequest("GET", "/api/v2/groups/testgrp/docs/tree", nil)
	w := httptest.NewRecorder()
	srv.routes().ServeHTTP(w, r)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}
	var env v2Envelope
	if err := json.NewDecoder(w.Body).Decode(&env); err != nil {
		t.Fatalf("decode envelope: %v", err)
	}
	if !env.OK {
		t.Fatal("envelope.ok is false")
	}
	// The reply is the v2DocsTreeReply object: {skillGenerated, nodes, businessNodes}.
	reply, ok := env.Data.(map[string]interface{})
	if !ok {
		t.Fatalf("expected reply object, got %T %v", env.Data, env.Data)
	}
	if reply["skillGenerated"] != true {
		t.Errorf("expected skillGenerated=true, got %v", reply["skillGenerated"])
	}
	data, ok := reply["nodes"].([]interface{})
	if !ok || len(data) == 0 {
		t.Fatalf("expected non-empty tree, got %T %v", reply["nodes"], reply["nodes"])
	}
	// No business/ dir in this fixture → businessNodes is empty.
	if biz, _ := reply["businessNodes"].([]interface{}); len(biz) != 0 {
		t.Errorf("expected empty businessNodes, got %v", biz)
	}
	// Root is the repo folder; it should contain category folders.
	repo, _ := data[0].(map[string]interface{})
	if repo["type"] != "folder" || repo["name"] != "repo1" {
		t.Fatalf("expected repo folder, got %v", repo)
	}
	cats, _ := repo["children"].([]interface{})
	if len(cats) != 4 {
		t.Fatalf("expected 4 category folders (overview/modules/reference/patterns), got %d: %v", len(cats), cats)
	}
	// First category should be Overview (deterministic order).
	first, _ := cats[0].(map[string]interface{})
	if first["name"] != "Overview" {
		t.Errorf("expected first category=Overview, got %v", first["name"])
	}
	// Enrichments must not appear as a category.
	for _, c := range cats {
		cm, _ := c.(map[string]interface{})
		if cm["category"] == "guide" {
			// enrichments would land in "Guides" — assert it isn't there
			t.Errorf("enrichments leaked into doc tree: %v", cm)
		}
	}
}

// TestHandleV2DocsTreeBusiness verifies that a `business/` doc set is split out
// into businessNodes (the separate, non-per-repo Business view) and is NOT
// duplicated in the technical per-repo tree. See #1622/#1623.
func TestHandleV2DocsTreeBusiness(t *testing.T) {
	home := t.TempDir()
	t.Setenv("GRAFEL_HOME", home)

	repoPath := t.TempDir()
	docsDir := filepath.Join(repoPath, "docs")
	mustWrite(t, filepath.Join(docsDir, "overview.md"), "# Repo\n\nOverview.\n")
	mustWrite(t, filepath.Join(docsDir, "business", "capabilities.md"), "# Capabilities\n")
	mustWrite(t, filepath.Join(docsDir, "business", "glossary.md"), "# Glossary\n")

	cfgPath := filepath.Join(home, "bizgrp.json")
	cfg := &registry.GroupConfig{Name: "bizgrp", Repos: []registry.Repo{{Slug: "repo1", Path: repoPath}}}
	if err := registry.SaveGroupConfig(cfgPath, cfg); err != nil {
		t.Fatalf("SaveGroupConfig: %v", err)
	}
	if err := registry.AddGroup("bizgrp", cfgPath); err != nil {
		t.Fatalf("AddGroup: %v", err)
	}

	srv, err := NewServer(DefaultConfig(), newFakeStore())
	if err != nil {
		t.Fatalf("NewServer: %v", err)
	}
	srv.graphs.mu.Lock()
	srv.graphs.entries["bizgrp"] = &cacheEntry{
		group:    &DashGroup{Name: "bizgrp", Repos: map[string]*DashRepo{"repo1": {Slug: "repo1", Path: repoPath}}},
		loadedAt: time.Now().Add(60 * time.Second),
	}
	srv.graphs.mu.Unlock()

	r := httptest.NewRequest("GET", "/api/v2/groups/bizgrp/docs/tree", nil)
	w := httptest.NewRecorder()
	srv.routes().ServeHTTP(w, r)
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}

	var env v2Envelope
	if err := json.NewDecoder(w.Body).Decode(&env); err != nil {
		t.Fatalf("decode: %v", err)
	}
	reply, _ := env.Data.(map[string]interface{})

	// businessNodes must contain the business/ docs.
	biz, _ := reply["businessNodes"].([]interface{})
	if len(biz) != 1 {
		t.Fatalf("expected 1 business repo node, got %d: %v", len(biz), biz)
	}
	bizRepo, _ := biz[0].(map[string]interface{})
	bizDocs, _ := bizRepo["children"].([]interface{})
	if len(bizDocs) != 2 {
		t.Fatalf("expected 2 business docs, got %d: %v", len(bizDocs), bizDocs)
	}
	// Business doc keys must keep the business/ prefix so the page endpoint resolves.
	d0, _ := bizDocs[0].(map[string]interface{})
	if got := d0["path"]; got != "repo1/business/capabilities.md" {
		t.Errorf("expected business doc key repo1/business/capabilities.md, got %v", got)
	}

	// The technical tree must NOT contain the business/ docs.
	nodes, _ := reply["nodes"].([]interface{})
	for _, n := range nodes {
		walkAssertNoBusiness(t, n)
	}
}

// walkAssertNoBusiness fails the test if any doc leaf path contains "/business/".
func walkAssertNoBusiness(t *testing.T, node interface{}) {
	t.Helper()
	m, _ := node.(map[string]interface{})
	if p, _ := m["path"].(string); p != "" {
		if filepath.Base(filepath.Dir(p)) == "business" {
			t.Errorf("business doc leaked into technical tree: %v", p)
		}
	}
	for _, c := range func() []interface{} { cs, _ := m["children"].([]interface{}); return cs }() {
		walkAssertNoBusiness(t, c)
	}
}

func TestHandleV2DocsPage(t *testing.T) {
	srv := buildV2DocsTestServer(t)
	r := httptest.NewRequest("GET", "/api/v2/groups/testgrp/docs/page?path=repo1/overview.md", nil)
	w := httptest.NewRecorder()
	srv.routes().ServeHTTP(w, r)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}
	var env v2Envelope
	if err := json.NewDecoder(w.Body).Decode(&env); err != nil {
		t.Fatalf("decode envelope: %v", err)
	}
	obj, ok := env.Data.(map[string]interface{})
	if !ok {
		t.Fatalf("expected object data, got %T", env.Data)
	}
	if obj["title"] != "Repo One" {
		t.Errorf("expected title from H1 'Repo One', got %v", obj["title"])
	}
	md, _ := obj["markdown"].(string)
	if md == "" {
		t.Error("expected non-empty markdown")
	}
}

func TestHandleV2DocsPageTraversal(t *testing.T) {
	srv := buildV2DocsTestServer(t)
	r := httptest.NewRequest("GET", "/api/v2/groups/testgrp/docs/page?path=repo1/../../../etc/passwd", nil)
	w := httptest.NewRecorder()
	srv.routes().ServeHTTP(w, r)
	// filepath.Clean collapses the traversal back inside docRoot → 404 (file absent).
	if w.Code == http.StatusOK {
		t.Fatalf("path traversal returned 200: %s", w.Body.String())
	}
}

func TestHandleV2DocsPageMissingParam(t *testing.T) {
	srv := buildV2DocsTestServer(t)
	r := httptest.NewRequest("GET", "/api/v2/groups/testgrp/docs/page", nil)
	w := httptest.NewRecorder()
	srv.routes().ServeHTTP(w, r)
	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", w.Code)
	}
}

func TestHandleV2DocsTreeGroupNotFound(t *testing.T) {
	srv := buildV2DocsTestServer(t)
	r := httptest.NewRequest("GET", "/api/v2/groups/ghost/docs/tree", nil)
	w := httptest.NewRecorder()
	srv.routes().ServeHTTP(w, r)

	if w.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d", w.Code)
	}
}

// ── #1624: docs export ─────────────────────────────────────────────────────

func TestHandleV2DocsExport_ZipAll(t *testing.T) {
	srv := buildV2DocsTestServer(t)

	// Seed business-tier docs in the post-#1624 group-level store location.
	bizDir := daemonBusinessDocsDir(t, "testgrp")
	mustWrite(t, filepath.Join(bizDir, "overview.md"), "# Business\n")
	mustWrite(t, filepath.Join(bizDir, "capabilities", "billing.md"), "# Billing\n")

	r := httptest.NewRequest("GET", "/api/v2/groups/testgrp/docs/export?format=zip&kind=all", nil)
	w := httptest.NewRecorder()
	srv.routes().ServeHTTP(w, r)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}
	if got := w.Header().Get("Content-Type"); got != "application/zip" {
		t.Fatalf("Content-Type = %q, want application/zip", got)
	}
	if cd := w.Header().Get("Content-Disposition"); cd == "" || !contains(cd, "testgrp-docs-") {
		t.Fatalf("Content-Disposition = %q, expected to contain 'testgrp-docs-'", cd)
	}

	body := w.Body.Bytes()
	zr, err := zip.NewReader(bytesReaderAt(body), int64(len(body)))
	if err != nil {
		t.Fatalf("zip parse: %v", err)
	}
	names := map[string]bool{}
	for _, f := range zr.File {
		names[f.Name] = true
	}
	// Technical tier: per-repo prefix.
	if !names["testgrp/repo1/overview.md"] {
		t.Errorf("missing testgrp/repo1/overview.md; got: %v", names)
	}
	if !names["testgrp/repo1/modules/order-service/README.md"] {
		t.Errorf("missing module readme in zip")
	}
	// Business tier: group-level prefix.
	if !names["testgrp/business/overview.md"] {
		t.Errorf("missing testgrp/business/overview.md; got: %v", names)
	}
	if !names["testgrp/business/capabilities/billing.md"] {
		t.Errorf("missing capability doc")
	}
}

func TestHandleV2DocsExport_UnsupportedFormat(t *testing.T) {
	srv := buildV2DocsTestServer(t)
	r := httptest.NewRequest("GET", "/api/v2/groups/testgrp/docs/export?format=tar", nil)
	w := httptest.NewRecorder()
	srv.routes().ServeHTTP(w, r)
	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", w.Code)
	}
}

func TestHandleV2DocsExport_BusinessKindOnly(t *testing.T) {
	srv := buildV2DocsTestServer(t)
	bizDir := daemonBusinessDocsDir(t, "testgrp")
	mustWrite(t, filepath.Join(bizDir, "overview.md"), "# Business\n")

	r := httptest.NewRequest("GET", "/api/v2/groups/testgrp/docs/export?kind=business", nil)
	w := httptest.NewRecorder()
	srv.routes().ServeHTTP(w, r)
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}
	body := w.Body.Bytes()
	zr, err := zip.NewReader(bytesReaderAt(body), int64(len(body)))
	if err != nil {
		t.Fatalf("zip parse: %v", err)
	}
	for _, f := range zr.File {
		if !startsWith(f.Name, "testgrp/business/") {
			t.Errorf("business-only export contained non-business entry: %s", f.Name)
		}
	}
}

// TestHandleV2DocsTree_MigratesLegacyInRepoDocs verifies the post-#1624
// migration: a pre-existing `<repo>/docs/` set produced by the skill is moved
// into ~/.grafel/docs/<group>/<repoSlug>/ on first read.
func TestHandleV2DocsTree_MigratesLegacyInRepoDocs(t *testing.T) {
	srv := buildV2DocsTestServer(t)
	// buildV2DocsTestServer seeds <repo>/docs/ — invoking the tree handler
	// should migrate it transparently.
	r := httptest.NewRequest("GET", "/api/v2/groups/testgrp/docs/tree", nil)
	w := httptest.NewRecorder()
	srv.routes().ServeHTTP(w, r)
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}
	store := daemon.RepoDocsDir("testgrp", "repo1")
	if _, err := os.Stat(filepath.Join(store, "overview.md")); err != nil {
		t.Fatalf("overview.md not migrated to store: %v", err)
	}
	if _, err := os.Stat(filepath.Join(store, "modules", "order-service", "README.md")); err != nil {
		t.Fatalf("module readme not migrated: %v", err)
	}
}
