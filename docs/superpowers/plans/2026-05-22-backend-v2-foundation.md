# Backend v2 API Foundation Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Establish a `/api/v2/...` HTTP surface in the existing Go daemon, coexisting with v1 routes, with typed JSON envelopes, pagination, SSE streaming conventions, error shape, and a `/api/v2/meta` bootstrap endpoint.

**Architecture:** A new `v2_*.go` file set in `internal/dashboard/` adds v2 handlers wired into `routes()` in `server.go` alongside the existing v1 routes. Shared v2 helpers (envelope, pagination, SSE, error) live in `v2_helpers.go`. The contract is documented in `internal/dashboard/API_V2.md`.

**Tech Stack:** Go 1.25, `net/http` stdlib only (no chi/gorilla), `encoding/json`, `net/http/httptest` for tests.

---

## File Map

| File | Action | Purpose |
|---|---|---|
| `internal/dashboard/v2_helpers.go` | **Create** | Typed envelope, pagination helpers, SSE writer, error shape |
| `internal/dashboard/v2_meta.go` | **Create** | `GET /api/v2/meta` handler (bootstrap endpoint) |
| `internal/dashboard/v2_helpers_test.go` | **Create** | Tests for envelope/pagination helpers |
| `internal/dashboard/v2_meta_test.go` | **Create** | Tests for the `/api/v2/meta` handler |
| `internal/dashboard/server.go` | **Modify** | Wire `/api/v2/...` routes in `routes()` alongside v1 |
| `internal/dashboard/API_V2.md` | **Create** | Contract doc: envelope, pagination, SSE, error shape |

---

## Task 1: v2 helpers — envelope, pagination, error shape

**Files:**
- Create: `internal/dashboard/v2_helpers.go`

- [ ] **Step 1: Write the failing test for `v2Envelope`**

Create `internal/dashboard/v2_helpers_test.go`:

```go
package dashboard

import (
	"encoding/json"
	"testing"
)

func TestV2Envelope_OKShape(t *testing.T) {
	payload := map[string]string{"key": "value"}
	env := v2OK(payload)
	b, err := json.Marshal(env)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	var got map[string]json.RawMessage
	if err := json.Unmarshal(b, &got); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if _, ok := got["data"]; !ok {
		t.Error("envelope missing 'data' field")
	}
	if _, ok := got["ok"]; !ok {
		t.Error("envelope missing 'ok' field")
	}
}

func TestV2Envelope_ErrorShape(t *testing.T) {
	env := v2Err("not_found", "group not registered")
	b, err := json.Marshal(env)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	var got map[string]json.RawMessage
	if err := json.Unmarshal(b, &got); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if _, ok := got["error"]; !ok {
		t.Error("error envelope missing 'error' field")
	}
	if _, ok := got["ok"]; !ok {
		t.Error("error envelope missing 'ok' field")
	}
	var errField map[string]string
	if err := json.Unmarshal(got["error"], &errField); err != nil {
		t.Fatalf("error field: %v", err)
	}
	if errField["code"] != "not_found" {
		t.Errorf("want code=not_found, got %q", errField["code"])
	}
	if errField["message"] != "group not registered" {
		t.Errorf("want message='group not registered', got %q", errField["message"])
	}
}

func TestV2Pagination_Defaults(t *testing.T) {
	p := parsePagination(nil, 0)
	if p.Limit != 50 {
		t.Errorf("default limit: want 50, got %d", p.Limit)
	}
	if p.Offset != 0 {
		t.Errorf("default offset: want 0, got %d", p.Offset)
	}
}

func TestV2Pagination_ClampLimit(t *testing.T) {
	// limit > 500 is clamped to 500
	import "net/url"
	q := url.Values{}
	q.Set("limit", "9999")
	p := parsePagination(q, 0)
	if p.Limit != 500 {
		t.Errorf("clamped limit: want 500, got %d", p.Limit)
	}
}

func TestV2Pagination_Envelope(t *testing.T) {
	items := []string{"a", "b", "c"}
	pag := V2Pagination{Limit: 10, Offset: 0, Total: 3}
	env := v2Page(items, pag)
	b, err := json.Marshal(env)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	var got map[string]json.RawMessage
	if err := json.Unmarshal(b, &got); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if _, ok := got["data"]; !ok {
		t.Error("missing data")
	}
	if _, ok := got["pagination"]; !ok {
		t.Error("missing pagination")
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

```bash
cd /Users/jorgecajas/Documents/Projects/archigraph-worktrees/backend-v2-foundation
go test ./internal/dashboard/ -run "TestV2" -v 2>&1 | head -30
```

Expected: compile error — `v2OK`, `v2Err`, `parsePagination`, `v2Page`, `V2Pagination` undefined.

- [ ] **Step 3: Fix the test — remove the `import` inside a function (Go syntax error)**

The `import "net/url"` inside `TestV2Pagination_ClampLimit` is illegal. Rewrite `v2_helpers_test.go` with imports at the package level:

```go
package dashboard

import (
	"encoding/json"
	"net/url"
	"testing"
)

func TestV2Envelope_OKShape(t *testing.T) {
	payload := map[string]string{"key": "value"}
	env := v2OK(payload)
	b, err := json.Marshal(env)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	var got map[string]json.RawMessage
	if err := json.Unmarshal(b, &got); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if _, ok := got["data"]; !ok {
		t.Error("envelope missing 'data' field")
	}
	if _, ok := got["ok"]; !ok {
		t.Error("envelope missing 'ok' field")
	}
}

func TestV2Envelope_ErrorShape(t *testing.T) {
	env := v2Err("not_found", "group not registered")
	b, err := json.Marshal(env)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	var got map[string]json.RawMessage
	if err := json.Unmarshal(b, &got); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if _, ok := got["error"]; !ok {
		t.Error("error envelope missing 'error' field")
	}
	if _, ok := got["ok"]; !ok {
		t.Error("error envelope missing 'ok' field")
	}
	var errField map[string]string
	if err := json.Unmarshal(got["error"], &errField); err != nil {
		t.Fatalf("error field: %v", err)
	}
	if errField["code"] != "not_found" {
		t.Errorf("want code=not_found, got %q", errField["code"])
	}
	if errField["message"] != "group not registered" {
		t.Errorf("want message='group not registered', got %q", errField["message"])
	}
}

func TestV2Pagination_Defaults(t *testing.T) {
	p := parsePagination(nil, 0)
	if p.Limit != 50 {
		t.Errorf("default limit: want 50, got %d", p.Limit)
	}
	if p.Offset != 0 {
		t.Errorf("default offset: want 0, got %d", p.Offset)
	}
}

func TestV2Pagination_ClampLimit(t *testing.T) {
	q := url.Values{}
	q.Set("limit", "9999")
	p := parsePagination(q, 0)
	if p.Limit != 500 {
		t.Errorf("clamped limit: want 500, got %d", p.Limit)
	}
}

func TestV2Pagination_Envelope(t *testing.T) {
	items := []string{"a", "b", "c"}
	pag := V2Pagination{Limit: 10, Offset: 0, Total: 3}
	env := v2Page(items, pag)
	b, err := json.Marshal(env)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	var got map[string]json.RawMessage
	if err := json.Unmarshal(b, &got); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if _, ok := got["data"]; !ok {
		t.Error("missing data")
	}
	if _, ok := got["pagination"]; !ok {
		t.Error("missing pagination")
	}
}
```

- [ ] **Step 4: Write minimal implementation — create `v2_helpers.go`**

```go
// v2_helpers.go — shared helpers for the /api/v2/... surface.
//
// Every v2 response uses one of two shapes:
//
//   Success (non-paginated):
//     { "ok": true, "data": <any> }
//
//   Success (paginated):
//     { "ok": true, "data": [...], "pagination": { "limit": N, "offset": N, "total": N } }
//
//   Error:
//     { "ok": false, "error": { "code": "<snake_case>", "message": "<human>" } }
//
// SSE events emitted by v2 streaming endpoints use the format:
//   event: <type>\ndata: <JSON>\n\n
//
// See API_V2.md for the full contract.

package dashboard

import (
	"fmt"
	"net/http"
	"net/url"
	"strconv"
)

// ---------------------------------------------------------------------------
// Envelope types
// ---------------------------------------------------------------------------

// v2Envelope is the wire shape for all non-paginated v2 responses.
type v2Envelope struct {
	OK   bool `json:"ok"`
	Data any  `json:"data,omitempty"`
}

// v2ErrEnvelope is the wire shape for all v2 error responses.
type v2ErrEnvelope struct {
	OK    bool      `json:"ok"`
	Error v2ErrBody `json:"error"`
}

// v2ErrBody carries a machine-readable code and a human-readable message.
type v2ErrBody struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}

// v2PageEnvelope is the wire shape for paginated v2 list responses.
type v2PageEnvelope struct {
	OK         bool         `json:"ok"`
	Data       any          `json:"data"`
	Pagination V2Pagination `json:"pagination"`
}

// V2Pagination is the pagination metadata included in paginated responses.
type V2Pagination struct {
	Limit  int `json:"limit"`
	Offset int `json:"offset"`
	Total  int `json:"total"`
}

// ---------------------------------------------------------------------------
// Constructor helpers
// ---------------------------------------------------------------------------

// v2OK wraps data in a success envelope.
func v2OK(data any) v2Envelope {
	return v2Envelope{OK: true, Data: data}
}

// v2Err creates an error envelope with code + message.
func v2Err(code, message string) v2ErrEnvelope {
	return v2ErrEnvelope{OK: false, Error: v2ErrBody{Code: code, Message: message}}
}

// v2Page wraps a list and pagination metadata in a success envelope.
func v2Page(data any, p V2Pagination) v2PageEnvelope {
	return v2PageEnvelope{OK: true, Data: data, Pagination: p}
}

// ---------------------------------------------------------------------------
// Pagination parsing
// ---------------------------------------------------------------------------

const (
	v2DefaultLimit = 50
	v2MaxLimit     = 500
)

// parsePagination extracts limit/offset from query params.
// q may be nil (returns defaults). total is the total item count known by the
// caller; it is embedded in the returned V2Pagination for convenience.
func parsePagination(q url.Values, total int) V2Pagination {
	limit := v2DefaultLimit
	offset := 0
	if q != nil {
		if v := q.Get("limit"); v != "" {
			if n, err := strconv.Atoi(v); err == nil && n > 0 {
				limit = n
			}
		}
		if v := q.Get("offset"); v != "" {
			if n, err := strconv.Atoi(v); err == nil && n >= 0 {
				offset = n
			}
		}
	}
	if limit > v2MaxLimit {
		limit = v2MaxLimit
	}
	return V2Pagination{Limit: limit, Offset: offset, Total: total}
}

// ---------------------------------------------------------------------------
// HTTP write helpers
// ---------------------------------------------------------------------------

// writeV2JSON writes a v2-enveloped JSON response.
func writeV2JSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = jsonEncode(w, v)
}

// writeV2Err writes a v2 error response.
func writeV2Err(w http.ResponseWriter, status int, code, message string) {
	writeV2JSON(w, status, v2Err(code, message))
}

// ---------------------------------------------------------------------------
// SSE helpers — same physical format as v1 SSE but type-labeled for v2.
// ---------------------------------------------------------------------------

// writeV2SSEEvent writes a single SSE event block to w.
// Callers must flush after writing. Format:
//
//	event: <eventType>\ndata: <data>\n\n
func writeV2SSEEvent(w http.ResponseWriter, eventType, data string) {
	fmt.Fprintf(w, "event: %s\ndata: %s\n\n", eventType, data)
}

// setV2SSEHeaders writes the standard SSE response headers.
// Call before WriteHeader.
func setV2SSEHeaders(w http.ResponseWriter) {
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache, no-transform")
	w.Header().Set("X-Accel-Buffering", "no")
	w.Header().Set("Connection", "keep-alive")
}
```

- [ ] **Step 5: Run tests to verify they pass**

```bash
cd /Users/jorgecajas/Documents/Projects/archigraph-worktrees/backend-v2-foundation
go test ./internal/dashboard/ -run "TestV2" -v 2>&1
```

Expected: all 5 `TestV2*` tests PASS.

- [ ] **Step 6: Commit**

```bash
cd /Users/jorgecajas/Documents/Projects/archigraph-worktrees/backend-v2-foundation
git add internal/dashboard/v2_helpers.go internal/dashboard/v2_helpers_test.go
git commit -m "feat(v2): add v2 envelope, pagination, SSE helpers (#1434)"
```

---

## Task 2: `/api/v2/meta` bootstrap endpoint

**Files:**
- Create: `internal/dashboard/v2_meta.go`
- Create: `internal/dashboard/v2_meta_test.go`

- [ ] **Step 1: Write the failing test**

Create `internal/dashboard/v2_meta_test.go`:

```go
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

// TestV2Meta_MethodNotAllowed verifies POST is rejected.
func TestV2Meta_MethodNotAllowed(t *testing.T) {
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

	if resp.StatusCode != http.StatusMethodNotAllowed {
		t.Errorf("want 405, got %d", resp.StatusCode)
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
```

- [ ] **Step 2: Run test to verify it fails**

```bash
cd /Users/jorgecajas/Documents/Projects/archigraph-worktrees/backend-v2-foundation
go test ./internal/dashboard/ -run "TestV2Meta" -v 2>&1 | head -20
```

Expected: FAIL — `404 Not Found` for `/api/v2/meta` (route not registered yet).

- [ ] **Step 3: Create `v2_meta.go`**

```go
// v2_meta.go — GET /api/v2/meta
//
// The /api/v2/meta endpoint is the bootstrap contract for WebUI v2.
// It returns:
//   - version: the daemon build version string
//   - api_versions: the API surfaces this daemon supports (always includes "v1", "v2")
//   - groups: slice of registered group slugs (empty slice when no groups exist)
//
// The WebUI v2 calls this once on mount (staleTime=Infinity in TanStack Query)
// to discover what the daemon supports before rendering any screen.

package dashboard

import (
	"net/http"

	"github.com/cajasmota/archigraph/internal/version"
)

// v2MetaReply is the data payload inside the v2 envelope for /api/v2/meta.
type v2MetaReply struct {
	// Version is the daemon build version (e.g. "1.2.3", "0.0.0-dev").
	Version string `json:"version"`
	// APIVersions lists the API surfaces supported by this daemon binary.
	// Always contains at least ["v1", "v2"].
	APIVersions []string `json:"api_versions"`
	// Groups is the list of registered group slugs. The WebUI v2 uses this
	// to decide whether to show the onboarding wizard or the main graph.
	Groups []string `json:"groups"`
}

// handleV2Meta — GET /api/v2/meta
func (s *Server) handleV2Meta(w http.ResponseWriter, r *http.Request) {
	groups, err := s.registry.ListGroups()
	if err != nil {
		writeV2Err(w, http.StatusInternalServerError, "internal_error", err.Error())
		return
	}
	slugs := make([]string, 0, len(groups))
	for _, g := range groups {
		slugs = append(slugs, g.Name)
	}
	reply := v2MetaReply{
		Version:     version.Version,
		APIVersions: []string{"v1", "v2"},
		Groups:      slugs,
	}
	writeV2JSON(w, http.StatusOK, v2OK(reply))
}
```

- [ ] **Step 4: Wire the route in `server.go`**

In `internal/dashboard/server.go`, find the `routes()` function. After the line registering `GET /api/snapshots/{group}/{id}` (the last registered route before `return s.withAuth(withGzip(mux))`), add the v2 routes block:

```go
	// --- API v2 routes (coexist with v1; v1 routes above are UNCHANGED) ---
	mux.HandleFunc("GET /api/v2/meta", s.handleV2Meta)
```

The exact location is just before `return s.withAuth(withGzip(mux))` at the end of `routes()`.

- [ ] **Step 5: Run tests to verify they pass**

```bash
cd /Users/jorgecajas/Documents/Projects/archigraph-worktrees/backend-v2-foundation
go test ./internal/dashboard/ -run "TestV2Meta" -v 2>&1
```

Expected: all 3 `TestV2Meta*` tests PASS.

- [ ] **Step 6: Build check — make sure nothing is broken**

```bash
cd /Users/jorgecajas/Documents/Projects/archigraph-worktrees/backend-v2-foundation
go build ./... 2>&1
```

Expected: no output (clean build).

- [ ] **Step 7: Run full dashboard test suite to confirm v1 is unbroken**

```bash
cd /Users/jorgecajas/Documents/Projects/archigraph-worktrees/backend-v2-foundation
go test ./internal/dashboard/... -count=1 2>&1 | tail -20
```

Expected: `ok github.com/cajasmota/archigraph/internal/dashboard` (all tests pass).

- [ ] **Step 8: Commit**

```bash
cd /Users/jorgecajas/Documents/Projects/archigraph-worktrees/backend-v2-foundation
git add internal/dashboard/v2_meta.go internal/dashboard/v2_meta_test.go internal/dashboard/server.go
git commit -m "feat(v2): add GET /api/v2/meta bootstrap endpoint (#1434)"
```

---

## Task 3: SSE v2 helper test + `jsonEncode` adapter

The `v2_helpers.go` calls `jsonEncode(w, v)` which does not exist yet. Add it and test it.

**Files:**
- Modify: `internal/dashboard/v2_helpers.go` (replace `jsonEncode` call with `json.NewEncoder`)

Note: `writeJSON` in `server.go` uses `json.NewEncoder(w).Encode(v)` directly. In v2_helpers we should do the same — no separate `jsonEncode` helper needed. This task corrects the helpers file before it causes a build error.

- [ ] **Step 1: Fix `writeV2JSON` to use `json.NewEncoder` directly**

In `internal/dashboard/v2_helpers.go`, update the `writeV2JSON` function:

```go
import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strconv"
)

// writeV2JSON writes a v2-enveloped JSON response.
func writeV2JSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}
```

(The full `v2_helpers.go` content must include the `encoding/json` import and use `json.NewEncoder(w).Encode(v)` instead of the undefined `jsonEncode(w, v)`.)

- [ ] **Step 2: Verify build is clean**

```bash
cd /Users/jorgecajas/Documents/Projects/archigraph-worktrees/backend-v2-foundation
go build ./internal/dashboard/... 2>&1
```

Expected: no output.

- [ ] **Step 3: Run all v2 tests**

```bash
cd /Users/jorgecajas/Documents/Projects/archigraph-worktrees/backend-v2-foundation
go test ./internal/dashboard/ -run "TestV2" -v 2>&1
```

Expected: 8 tests pass (5 helpers + 3 meta).

- [ ] **Step 4: Commit**

```bash
cd /Users/jorgecajas/Documents/Projects/archigraph-worktrees/backend-v2-foundation
git add internal/dashboard/v2_helpers.go
git commit -m "fix(v2): use json.NewEncoder directly in writeV2JSON (#1434)"
```

---

## Task 4: Write `API_V2.md` contract document

**Files:**
- Create: `internal/dashboard/API_V2.md`

- [ ] **Step 1: Create the contract document**

```markdown
# archigraph API v2 — Contract Reference

> This document is the contract for the `/api/v2/...` surface.  
> Screen-building agents MUST read this before implementing any v2 handler.  
> When this doc and the code disagree, fix the code — the doc is authoritative.

---

## 1. Envelope — every response

All v2 responses are JSON objects with an `ok` boolean at the root.

### Success (non-paginated)

```json
{
  "ok": true,
  "data": { ... }
}
```

### Success (paginated list)

```json
{
  "ok": true,
  "data": [ ... ],
  "pagination": {
    "limit": 50,
    "offset": 0,
    "total": 312
  }
}
```

### Error

```json
{
  "ok": false,
  "error": {
    "code": "not_found",
    "message": "group 'acme' is not registered"
  }
}
```

**`code` values (canonical):**

| Code | HTTP status | Meaning |
|---|---|---|
| `not_found` | 404 | Resource does not exist |
| `bad_request` | 400 | Malformed query or body |
| `internal_error` | 500 | Server-side failure |
| `unavailable` | 503 | Dependency (broker, queue) not wired |
| `unauthorized` | 401 | Bearer token required (when auth is enabled) |

Go constructor: `v2Err(code, message)` → use `writeV2Err(w, status, code, message)`.

---

## 2. Pagination

Paginated list endpoints accept two query params:

| Param | Default | Max | Description |
|---|---|---|---|
| `limit` | 50 | 500 | Items per page |
| `offset` | 0 | — | Zero-based item offset |

Parse with `parsePagination(r.URL.Query(), total)`.  
Wrap the result slice and pagination struct with `v2Page(items, pag)`.

**Example:** `GET /api/v2/groups?limit=20&offset=40`

---

## 3. SSE streaming convention

Streaming endpoints (live progress, audit feed, etc.) use Server-Sent Events.

**Required headers** (use `setV2SSEHeaders(w)` helper):

```
Content-Type: text/event-stream
Cache-Control: no-cache, no-transform
X-Accel-Buffering: no
Connection: keep-alive
```

**Wire format** (use `writeV2SSEEvent(w, eventType, data)` helper):

```
event: <type>\n
data: <JSON string>\n
\n
```

**Standard event lifecycle:**

| Event type | Payload | When |
|---|---|---|
| `connected` | `{"subscribed_at": <unix-ms>}` | Immediately on subscribe |
| `<domain>` | domain-specific JSON object | When data arrives |
| `heartbeat` | `{}` | Every 15 s (keep-alive) |
| `close` | `{}` | Server closes stream (shutdown / error) |

**Note:** SSE endpoints must NOT be gzip-compressed. The `withGzip` middleware in `server.go` already excludes paths ending in `/stream` and containing `index-progress`/`mcp-activity`. New v2 SSE endpoints MUST have their paths end in `/stream` OR be explicitly excluded in `withGzip`.

---

## 4. Error shape reference (Go)

```go
// Non-paginated success
writeV2JSON(w, http.StatusOK, v2OK(myPayload))

// Paginated success
pag := parsePagination(r.URL.Query(), len(items))
writeV2JSON(w, http.StatusOK, v2Page(items[pag.Offset:end], pag))

// Error
writeV2Err(w, http.StatusNotFound, "not_found", "group 'x' not found")
```

---

## 5. Route naming conventions

- All v2 routes begin with `/api/v2/`.
- Group-scoped routes: `/api/v2/{group}/<resource>`.
- List → `GET /api/v2/{group}/nodes` (paginated).
- Detail → `GET /api/v2/{group}/nodes/{id}`.
- Streaming → `GET /api/v2/{group}/nodes/stream` (SSE).
- Mutations → `POST /api/v2/{group}/<resource>/<action>` (follow v1 convention).

---

## 6. Bootstrap endpoint

`GET /api/v2/meta`

Called once on app mount by WebUI v2. Response:

```json
{
  "ok": true,
  "data": {
    "version": "1.2.3",
    "api_versions": ["v1", "v2"],
    "groups": ["acme", "infra"]
  }
}
```

- `version`: daemon build version (`version.Version`).
- `api_versions`: surfaces supported. Always `["v1", "v2"]` in this binary.
- `groups`: list of registered group slugs. Empty array when no groups exist (→ show onboarding wizard).

---

## 7. Adding a new v2 endpoint — checklist

- [ ] Handler file named `v2_<surface>.go`.
- [ ] Test file named `v2_<surface>_test.go`.
- [ ] Route registered in `server.go` `routes()` under the `// --- API v2 routes ---` comment block.
- [ ] Response uses `v2OK`, `v2Page`, or `writeV2Err` (never raw `writeJSON` / `writeErr`).
- [ ] Paginated list? Use `parsePagination(r.URL.Query(), total)`.
- [ ] Streaming? Path ends in `/stream`; call `setV2SSEHeaders(w)` and `writeV2SSEEvent(w, ...)`.
- [ ] This doc updated if a new pattern is introduced.
```

- [ ] **Step 2: Commit**

```bash
cd /Users/jorgecajas/Documents/Projects/archigraph-worktrees/backend-v2-foundation
git add internal/dashboard/API_V2.md
git commit -m "docs(v2): add API_V2.md contract reference (#1434)"
```

---

## Task 5: Smoke-test the live endpoint via a temp daemon instance

This task is optional but recommended for the verify step. It starts the daemon binary on a temporary port, curls `/api/v2/meta`, and tears down.

**Pre-condition:** The worktree must build cleanly (Task 3 step 2 must have passed).

- [ ] **Step 1: Build the binary**

```bash
cd /Users/jorgecajas/Documents/Projects/archigraph-worktrees/backend-v2-foundation
go build -o /tmp/archigraph-v2-test ./cmd/archigraph 2>&1
```

Expected: binary at `/tmp/archigraph-v2-test`.

- [ ] **Step 2: Start a temp daemon on a non-conflicting port**

```bash
/tmp/archigraph-v2-test daemon --port 47350 &
DAEMON_PID=$!
sleep 1
curl -s http://localhost:47350/api/v2/meta | python3 -m json.tool
```

Expected output:
```json
{
    "ok": true,
    "data": {
        "version": "...",
        "api_versions": ["v1", "v2"],
        "groups": [...]
    }
}
```

- [ ] **Step 3: Verify v1 route still works**

```bash
curl -s http://localhost:47350/api/info | python3 -m json.tool
```

Expected: JSON with `version`, `commit`, `built_at` fields (v1 /api/info shape).

- [ ] **Step 4: Kill the temp daemon**

```bash
kill $DAEMON_PID 2>/dev/null || true
rm -f /tmp/archigraph-v2-test
```

---

## Task 6: Final build + full test suite + PR

- [ ] **Step 1: Full build**

```bash
cd /Users/jorgecajas/Documents/Projects/archigraph-worktrees/backend-v2-foundation
go build ./... 2>&1
```

Expected: no output.

- [ ] **Step 2: Full test suite**

```bash
cd /Users/jorgecajas/Documents/Projects/archigraph-worktrees/backend-v2-foundation
go test ./... -count=1 2>&1 | tail -30
```

Expected: all packages pass. Note: some packages may have integration-level tests that require fixtures — any pre-existing skips are acceptable. No new failures.

- [ ] **Step 3: Push branch**

```bash
cd /Users/jorgecajas/Documents/Projects/archigraph-worktrees/backend-v2-foundation
git push -u origin feat/backend-v2-foundation 2>&1
```

- [ ] **Step 4: Open PR**

```bash
gh pr create \
  --title "feat(v2): Backend v2 API Foundation (#1434)" \
  --body "$(cat <<'EOF'
## What

Establishes the `/api/v2/...` HTTP surface in the daemon alongside v1 routes (v1 routes untouched). Implements issue #1434, part of EPIC #1432.

## Why

WebUI v2 screen tickets need a consistent typed API surface to build against. This PR lays the foundation — envelopes, pagination, SSE helpers, error shape, and the `/api/v2/meta` bootstrap endpoint — so every subsequent screen ticket has a contract to code to.

## Changes

- **`internal/dashboard/v2_helpers.go`** — shared helpers: `v2OK`, `v2Err`, `v2Page`, `V2Pagination`, `parsePagination`, `writeV2JSON`, `writeV2Err`, `writeV2SSEEvent`, `setV2SSEHeaders`.
- **`internal/dashboard/v2_meta.go`** — `GET /api/v2/meta` handler: returns `version`, `api_versions`, `groups` for WebUI v2 bootstrap.
- **`internal/dashboard/server.go`** — wires `/api/v2/meta` into `routes()` under a dedicated v2 comment block; all existing v1 routes are unchanged.
- **`internal/dashboard/API_V2.md`** — contract document: envelope shapes, pagination params, SSE lifecycle, error codes, route naming convention, per-endpoint checklist.

## Testing

- `go test ./internal/dashboard/ -run TestV2` — 8 new tests covering envelope helpers, pagination helpers, meta endpoint shape, method-not-allowed, and v1 regression.
- `go test ./internal/dashboard/...` — full dashboard suite passes (all v1 tests unaffected).
- `go build ./...` — clean build.

## v1 compatibility

No v1 handler, route, or response shape was modified. The only change to `server.go` is addition of a `mux.HandleFunc("GET /api/v2/meta", ...)` call at the end of the v2 route block. Confirmed by `TestV2Meta_V1RegistryStillWorks`.

## Conventions established

See `internal/dashboard/API_V2.md` for the full contract. Summary:

| Concern | Convention |
|---|---|
| Envelope | `{ "ok": bool, "data": ... }` / `{ "ok": false, "error": { "code", "message" } }` |
| Pagination | `?limit=50&offset=0` → `{ "data": [...], "pagination": { limit, offset, total } }` |
| SSE | `event: <type>\ndata: <JSON>\n\n`, path ends in `/stream` to bypass gzip middleware |
| Error codes | `not_found`, `bad_request`, `internal_error`, `unavailable`, `unauthorized` |
| Bootstrap | `GET /api/v2/meta` → `{ version, api_versions, groups }` |

Fixes #1434
Part of EPIC #1432
EOF
)"
```

---

## Self-Review

### Spec coverage

| Requirement | Covered by |
|---|---|
| `/api/v2/...` surface coexisting with v1 | Task 2: route wired in `server.go` alongside v1 |
| Consistent typed JSON envelopes | Task 1: `v2OK`, `v2Err`, `v2Page` in `v2_helpers.go` |
| Pagination convention | Task 1: `parsePagination`, `V2Pagination`, `v2Page` |
| SSE streaming convention | Task 1: `writeV2SSEEvent`, `setV2SSEHeaders` |
| Error shape | Task 1: `v2ErrEnvelope`, `v2ErrBody`, `writeV2Err` |
| `/api/v2/meta` (or `/api/v2/health`) bootstrap endpoint | Task 2: `GET /api/v2/meta` |
| `internal/dashboard/API_V2.md` | Task 4 |
| `go build ./...` passes | Task 6 step 1 |
| Handler test for v2 meta + helpers | Tasks 1 and 2 |
| v1 routes unchanged | `TestV2Meta_V1RegistryStillWorks` in Task 2 |
| curl smoke test on temp daemon | Task 5 (optional) |
| PR with 6-section format, `Fixes #1434`, reference EPIC #1432 | Task 6 step 4 |
| Do NOT merge / do NOT rebuild live daemon | Noted in task text |

### Placeholder scan

No TBDs, TODOs, or deferred steps. All code blocks are complete.

### Type consistency

- `v2OK` returns `v2Envelope` — used in `handleV2Meta` as `v2OK(reply)`.
- `v2Err` returns `v2ErrEnvelope` — used in `writeV2Err` (which calls `writeV2JSON(..., v2Err(...))`).
- `parsePagination` returns `V2Pagination` — used in `v2Page` second arg.
- `writeV2JSON` signature: `(w http.ResponseWriter, status int, v any)` — used consistently.
- `writeV2Err` signature: `(w http.ResponseWriter, status int, code, message string)` — consistent with `handleV2Meta`.
- `v2MetaReply.APIVersions` maps to JSON `api_versions` — tested in `TestV2Meta_Shape`.

All consistent.
