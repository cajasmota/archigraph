package dashboard

// handlers_webhooks_test.go — Tests for webhook CRUD and test-ping endpoints.

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/cajasmota/grafel/internal/notifications"
)

// ─────────────────────────────────────────────────────────────────────────────
// Helpers
// ─────────────────────────────────────────────────────────────────────────────

func testWebhookServer(t *testing.T) (*Server, func()) {
	t.Helper()
	tmp := t.TempDir()
	t.Setenv("GRAFEL_HOME", tmp)
	srv, err := NewServer(DefaultConfig(), newFakeStore())
	if err != nil {
		t.Fatalf("NewServer: %v", err)
	}
	return srv, func() {}
}

func postJSON(t *testing.T, srv *Server, path string, body any) *httptest.ResponseRecorder {
	t.Helper()
	b, err := json.Marshal(body)
	if err != nil {
		t.Fatal(err)
	}
	req := httptest.NewRequest(http.MethodPost, path, bytes.NewReader(b))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	srv.routes().ServeHTTP(rec, req)
	return rec
}

func getPath(t *testing.T, srv *Server, path string) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequest(http.MethodGet, path, nil)
	rec := httptest.NewRecorder()
	srv.routes().ServeHTTP(rec, req)
	return rec
}

func deletePath(t *testing.T, srv *Server, path string) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequest(http.MethodDelete, path, nil)
	rec := httptest.NewRecorder()
	srv.routes().ServeHTTP(rec, req)
	return rec
}

func putJSON(t *testing.T, srv *Server, path string, body any) *httptest.ResponseRecorder {
	t.Helper()
	b, _ := json.Marshal(body)
	req := httptest.NewRequest(http.MethodPut, path, bytes.NewReader(b))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	srv.routes().ServeHTTP(rec, req)
	return rec
}

// ─────────────────────────────────────────────────────────────────────────────
// GET /api/webhooks — list
// ─────────────────────────────────────────────────────────────────────────────

func TestHandleListWebhooks_Empty(t *testing.T) {
	srv, cleanup := testWebhookServer(t)
	defer cleanup()

	rec := getPath(t, srv, "/api/webhooks")
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}
	var reply map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &reply); err != nil {
		t.Fatal(err)
	}
	hooks := reply["webhooks"].([]any)
	if len(hooks) != 0 {
		t.Errorf("expected empty webhooks, got %v", hooks)
	}
}

// ─────────────────────────────────────────────────────────────────────────────
// POST /api/webhooks — create
// ─────────────────────────────────────────────────────────────────────────────

func TestHandleCreateWebhook_Valid(t *testing.T) {
	srv, cleanup := testWebhookServer(t)
	defer cleanup()

	cfg := notifications.WebhookConfig{
		ID:      "slack-platform",
		URL:     "https://hooks.slack.com/T123/B456/abc",
		Flavor:  notifications.FlavorSlack,
		Enabled: true,
		Events:  []notifications.EventType{notifications.EventQualityRegressed},
	}

	rec := postJSON(t, srv, "/api/webhooks", cfg)
	if rec.Code != http.StatusCreated {
		t.Fatalf("expected 201, got %d: %s", rec.Code, rec.Body.String())
	}

	// Verify it shows up in list.
	list := getPath(t, srv, "/api/webhooks")
	var reply map[string]any
	_ = json.Unmarshal(list.Body.Bytes(), &reply)
	hooks := reply["webhooks"].([]any)
	if len(hooks) != 1 {
		t.Errorf("expected 1 webhook, got %d", len(hooks))
	}
}

func TestHandleCreateWebhook_MissingURL(t *testing.T) {
	srv, cleanup := testWebhookServer(t)
	defer cleanup()

	cfg := notifications.WebhookConfig{ID: "bad", Enabled: true}
	rec := postJSON(t, srv, "/api/webhooks", cfg)
	if rec.Code != http.StatusUnprocessableEntity {
		t.Errorf("expected 422, got %d", rec.Code)
	}
}

func TestHandleCreateWebhook_DuplicateID(t *testing.T) {
	srv, cleanup := testWebhookServer(t)
	defer cleanup()

	cfg := notifications.WebhookConfig{ID: "dup", URL: "https://example.com", Enabled: true}
	postJSON(t, srv, "/api/webhooks", cfg)
	rec := postJSON(t, srv, "/api/webhooks", cfg)
	if rec.Code != http.StatusConflict {
		t.Errorf("expected 409, got %d: %s", rec.Code, rec.Body.String())
	}
}

// ─────────────────────────────────────────────────────────────────────────────
// PUT /api/webhooks/{id} — update
// ─────────────────────────────────────────────────────────────────────────────

func TestHandleUpdateWebhook(t *testing.T) {
	srv, cleanup := testWebhookServer(t)
	defer cleanup()

	cfg := notifications.WebhookConfig{ID: "myhook", URL: "https://old.example.com", Enabled: true}
	postJSON(t, srv, "/api/webhooks", cfg)

	cfg.URL = "https://new.example.com"
	rec := putJSON(t, srv, "/api/webhooks/myhook", cfg)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}

	var reply map[string]any
	_ = json.Unmarshal(rec.Body.Bytes(), &reply)
	hook := reply["webhook"].(map[string]any)
	if hook["url"] != "https://new.example.com" {
		t.Errorf("expected updated url, got %v", hook["url"])
	}
}

func TestHandleUpdateWebhook_NotFound(t *testing.T) {
	srv, cleanup := testWebhookServer(t)
	defer cleanup()

	cfg := notifications.WebhookConfig{ID: "ghost", URL: "https://example.com", Enabled: true}
	rec := putJSON(t, srv, "/api/webhooks/ghost", cfg)
	if rec.Code != http.StatusNotFound {
		t.Errorf("expected 404, got %d", rec.Code)
	}
}

// ─────────────────────────────────────────────────────────────────────────────
// DELETE /api/webhooks/{id}
// ─────────────────────────────────────────────────────────────────────────────

func TestHandleDeleteWebhook(t *testing.T) {
	srv, cleanup := testWebhookServer(t)
	defer cleanup()

	cfg := notifications.WebhookConfig{ID: "deleteme", URL: "https://example.com", Enabled: true}
	postJSON(t, srv, "/api/webhooks", cfg)

	rec := deletePath(t, srv, "/api/webhooks/deleteme")
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}

	// Verify gone.
	list := getPath(t, srv, "/api/webhooks")
	var reply map[string]any
	_ = json.Unmarshal(list.Body.Bytes(), &reply)
	hooks := reply["webhooks"].([]any)
	if len(hooks) != 0 {
		t.Errorf("expected 0 webhooks after delete, got %d", len(hooks))
	}
}

func TestHandleDeleteWebhook_NotFound(t *testing.T) {
	srv, cleanup := testWebhookServer(t)
	defer cleanup()

	rec := deletePath(t, srv, "/api/webhooks/ghost")
	if rec.Code != http.StatusNotFound {
		t.Errorf("expected 404, got %d", rec.Code)
	}
}

// ─────────────────────────────────────────────────────────────────────────────
// POST /api/webhooks/test — ad-hoc test ping
// ─────────────────────────────────────────────────────────────────────────────

func TestHandleTestWebhookAdhoc_NoDispatcher(t *testing.T) {
	srv, cleanup := testWebhookServer(t)
	defer cleanup()
	// No dispatcher wired — should 503.
	rec := postJSON(t, srv, "/api/webhooks/test", map[string]any{"url": "https://example.com", "enabled": true})
	if rec.Code != http.StatusServiceUnavailable {
		t.Errorf("expected 503, got %d", rec.Code)
	}
}

func TestHandleTestWebhookAdhoc_Success(t *testing.T) {
	srv, cleanup := testWebhookServer(t)
	defer cleanup()

	// Stand up a test HTTP server to receive the ping.
	pinged := false
	target := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		pinged = true
		w.WriteHeader(http.StatusOK)
	}))
	defer target.Close()

	srv.SetWebhookDispatcher(notifications.NewDispatcher())

	body := map[string]any{
		"id":      "adhoc",
		"url":     target.URL,
		"enabled": true,
		"flavor":  "generic",
	}
	rec := postJSON(t, srv, "/api/webhooks/test", body)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}

	var reply map[string]any
	_ = json.Unmarshal(rec.Body.Bytes(), &reply)
	if reply["success"] != true {
		t.Errorf("expected success=true, got %v", reply)
	}
	if !pinged {
		t.Error("expected test server to be pinged")
	}
}

func TestHandleTestWebhookAdhoc_MissingURL(t *testing.T) {
	srv, cleanup := testWebhookServer(t)
	defer cleanup()
	srv.SetWebhookDispatcher(notifications.NewDispatcher())

	rec := postJSON(t, srv, "/api/webhooks/test", map[string]any{"enabled": true})
	if rec.Code != http.StatusUnprocessableEntity {
		t.Errorf("expected 422, got %d", rec.Code)
	}
}

// ─────────────────────────────────────────────────────────────────────────────
// GET /api/webhooks/failures
// ─────────────────────────────────────────────────────────────────────────────

func TestHandleWebhookFailures_Empty(t *testing.T) {
	srv, cleanup := testWebhookServer(t)
	defer cleanup()
	srv.SetWebhookDispatcher(notifications.NewDispatcher())

	rec := getPath(t, srv, "/api/webhooks/failures")
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}
	var reply map[string]any
	_ = json.Unmarshal(rec.Body.Bytes(), &reply)
	failures := reply["failures"].([]any)
	if len(failures) != 0 {
		t.Errorf("expected 0 failures, got %d", len(failures))
	}
}

func TestHandleWebhookFailures_NoDispatcher(t *testing.T) {
	srv, cleanup := testWebhookServer(t)
	defer cleanup()
	// nil dispatcher still returns empty list (not 503)
	rec := getPath(t, srv, "/api/webhooks/failures")
	if rec.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", rec.Code)
	}
}
