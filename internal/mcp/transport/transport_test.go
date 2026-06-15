package transport

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	mcpapi "github.com/mark3labs/mcp-go/mcp"
	mcpsrv "github.com/mark3labs/mcp-go/server"
)

// newTestMCP builds a minimal mcp-go server with one trivial tool so the
// Streamable-HTTP handler has something to dispatch to.
func newTestMCP(t *testing.T) *mcpsrv.MCPServer {
	t.Helper()
	srv := mcpsrv.NewMCPServer("grafel-test", "0.0.0", mcpsrv.WithToolCapabilities(true))
	srv.AddTool(
		mcpapi.NewTool("ping", mcpapi.WithDescription("returns pong")),
		func(ctx context.Context, _ mcpapi.CallToolRequest) (*mcpapi.CallToolResult, error) {
			return mcpapi.NewToolResultText("pong"), nil
		},
	)
	return srv
}

// Transport interface satisfaction — both impls must compile against it.
func TestTransportInterfaceSatisfied(t *testing.T) {
	var _ Transport = NewStdioTransport(newTestMCP(t))
	ht, err := NewHTTPTransport(newTestMCP(t), HTTPConfig{
		Addr: "127.0.0.1:0",
		Auth: StaticTokenAuthenticator{Token: "x"},
	})
	if err != nil {
		t.Fatalf("NewHTTPTransport: %v", err)
	}
	var _ Transport = ht
	if ht.Name() != "http" {
		t.Fatalf("Name = %q, want http", ht.Name())
	}
	if NewStdioTransport(newTestMCP(t)).Name() != "stdio" {
		t.Fatalf("stdio Name mismatch")
	}
}

// Feature flag is OFF by default and only specific truthy values enable it.
func TestHTTPEnabledDefaultOff(t *testing.T) {
	t.Setenv(EnvHTTPMCP, "")
	if HTTPEnabled() {
		t.Fatal("HTTPEnabled() must be false when unset")
	}
	for _, off := range []string{"", "0", "false", "off", "no", "garbage"} {
		t.Setenv(EnvHTTPMCP, off)
		if HTTPEnabled() {
			t.Fatalf("HTTPEnabled() must be false for %q", off)
		}
	}
	for _, on := range []string{"1", "true", "TRUE", "on", "Yes"} {
		t.Setenv(EnvHTTPMCP, on)
		if !HTTPEnabled() {
			t.Fatalf("HTTPEnabled() must be true for %q", on)
		}
	}
}

// Construction fails closed: no addr, no auth.
func TestNewHTTPTransportFailsClosed(t *testing.T) {
	if _, err := NewHTTPTransport(newTestMCP(t), HTTPConfig{Auth: StaticTokenAuthenticator{Token: "x"}}); err == nil {
		t.Fatal("expected error for missing bind address")
	}
	if _, err := NewHTTPTransport(newTestMCP(t), HTTPConfig{Addr: "127.0.0.1:0"}); err == nil {
		t.Fatal("expected error for nil Authenticator")
	}
	if _, err := NewHTTPTransport(nil, HTTPConfig{Addr: "127.0.0.1:0", Auth: StaticTokenAuthenticator{Token: "x"}}); err == nil {
		t.Fatal("expected error for nil MCPServer")
	}
}

// Static token: valid bearer authenticates, missing/invalid does not.
func TestStaticTokenAuthenticator(t *testing.T) {
	a := StaticTokenAuthenticator{Token: "secret-token", Subject: "alice"}

	req := httptest.NewRequest(http.MethodPost, "/mcp", nil)
	req.Header.Set("Authorization", "Bearer secret-token")
	id, err := a.Authenticate(req)
	if err != nil {
		t.Fatalf("valid token rejected: %v", err)
	}
	if id.Subject != "alice" {
		t.Fatalf("Subject = %q, want alice", id.Subject)
	}

	for _, bad := range []string{"", "Bearer wrong", "secret-token", "Basic secret-token"} {
		req := httptest.NewRequest(http.MethodPost, "/mcp", nil)
		if bad != "" {
			req.Header.Set("Authorization", bad)
		}
		if _, err := a.Authenticate(req); err == nil {
			t.Fatalf("expected rejection for header %q", bad)
		}
	}

	// Empty configured token must reject everything (fail-closed).
	empty := StaticTokenAuthenticator{Token: ""}
	req2 := httptest.NewRequest(http.MethodPost, "/mcp", nil)
	req2.Header.Set("Authorization", "Bearer anything")
	if _, err := empty.Authenticate(req2); err == nil {
		t.Fatal("empty-token authenticator must reject all requests")
	}
}

// End-to-end through the auth middleware + mcp-go handler via httptest:
//   - valid token  → request handled (not 401),
//   - missing/invalid token → 401.
func TestHTTPTransportAuthMiddleware(t *testing.T) {
	ht, err := NewHTTPTransport(newTestMCP(t), HTTPConfig{
		Addr:      "127.0.0.1:0",
		Auth:      StaticTokenAuthenticator{Token: "good", Subject: "svc"},
		Stateless: true,
	})
	if err != nil {
		t.Fatalf("NewHTTPTransport: %v", err)
	}
	ts := httptest.NewServer(ht.Handler())
	defer ts.Close()

	// MCP initialize request body (JSON-RPC 2.0).
	body := `{"jsonrpc":"2.0","id":1,"method":"initialize","params":{"protocolVersion":"2024-11-05","capabilities":{},"clientInfo":{"name":"t","version":"0"}}}`

	// Missing token → 401.
	resp, err := http.Post(ts.URL, "application/json", strings.NewReader(body))
	if err != nil {
		t.Fatalf("post (no token): %v", err)
	}
	resp.Body.Close()
	if resp.StatusCode != http.StatusUnauthorized {
		t.Fatalf("no-token status = %d, want 401", resp.StatusCode)
	}

	// Invalid token → 401.
	req, _ := http.NewRequest(http.MethodPost, ts.URL, strings.NewReader(body))
	req.Header.Set("Authorization", "Bearer nope")
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json, text/event-stream")
	resp, err = http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("post (bad token): %v", err)
	}
	resp.Body.Close()
	if resp.StatusCode != http.StatusUnauthorized {
		t.Fatalf("bad-token status = %d, want 401", resp.StatusCode)
	}

	// Valid token → NOT 401 (request reaches MCP dispatch).
	req, _ = http.NewRequest(http.MethodPost, ts.URL, strings.NewReader(body))
	req.Header.Set("Authorization", "Bearer good")
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json, text/event-stream")
	resp, err = http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("post (good token): %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode == http.StatusUnauthorized {
		t.Fatalf("good-token request was rejected with 401")
	}
}

// IdentityFromContext round-trips the authenticated identity (the seam tools
// would use for authz/audit).
func TestIdentityContextRoundTrip(t *testing.T) {
	ctx := withIdentity(context.Background(), Identity{Subject: "bob"})
	id, ok := IdentityFromContext(ctx)
	if !ok || id.Subject != "bob" {
		t.Fatalf("IdentityFromContext = (%+v, %v), want bob/true", id, ok)
	}
	if _, ok := IdentityFromContext(context.Background()); ok {
		t.Fatal("empty context should have no identity")
	}
}
