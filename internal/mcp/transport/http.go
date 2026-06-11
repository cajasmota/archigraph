package transport

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"os"
	"strings"
	"time"

	mcpsrv "github.com/mark3labs/mcp-go/server"
)

// EnvHTTPMCP is the feature-flag environment variable that gates the shared
// HTTP MCP transport. It is OFF by default (ADR-0022): an empty/unset value
// disables the transport entirely. Any value the daemon recognises as
// "enabled" (see HTTPEnabled) plus an explicit bind address is required before
// the server will start.
//
// This transport's SECURITY MODEL IS UNRATIFIED. Enabling it in production
// requires maintainer/security sign-off (ADR-0022 "Decisions for the
// maintainer"). The shipped authenticator is a stub.
const EnvHTTPMCP = "ARCHIGRAPH_HTTP_MCP"

// HTTPEnabled reports whether the HTTP MCP transport feature flag is switched
// on. Default OFF: only "1", "true", "on", "yes" (case-insensitive) enable it.
// Anything else — including unset/empty — leaves it disabled.
func HTTPEnabled() bool {
	switch strings.ToLower(strings.TrimSpace(os.Getenv(EnvHTTPMCP))) {
	case "1", "true", "on", "yes":
		return true
	default:
		return false
	}
}

// HTTPConfig configures the HTTP transport skeleton.
//
// PROTOTYPE scope: enough to stand the server up behind a reverse proxy for
// evaluation. Production concerns (TLS-in-process specifics, mTLS, OAuth, rate
// limiting, per-user authz, audit) are deliberately left as TODOs / future
// options — see ADR-0022.
type HTTPConfig struct {
	// Addr is the bind address (e.g. "127.0.0.1:8765"). REQUIRED — the
	// transport refuses to construct without one and never implicitly binds a
	// wildcard/public interface. Operators must choose exposure explicitly;
	// the recommendation (ADR-0022) is loopback/private + reverse-proxy TLS.
	Addr string

	// Auth is the authentication middleware. REQUIRED and must be non-nil:
	// an HTTP transport with no authenticator would expose the entire graph
	// unauthenticated, which is never acceptable. Use StaticTokenAuthenticator
	// only for evaluation.
	Auth Authenticator

	// Stateless selects mcp-go's stateless Streamable-HTTP mode (recommended
	// default for shared/CI deployments, ADR-0022): no server-side session
	// table, each request self-contained.
	Stateless bool

	// ReadHeaderTimeout bounds slow-header attacks. Defaults to 10s when zero.
	ReadHeaderTimeout time.Duration
}

// HTTPTransport is a feature-flagged, OFF-BY-DEFAULT Streamable-HTTP MCP
// transport (ADR-0022, #4296). It wraps mcp-go's StreamableHTTPServer (an
// http.Handler) behind an authentication middleware, on archigraph's own
// *http.Server so the auth choke point is in front of every MCP request.
//
// It is NOT wired into the default daemon path. Construction is gated by the
// caller checking HTTPEnabled(); New* still validates its inputs so a
// misconfiguration fails closed rather than exposing an open port.
type HTTPTransport struct {
	cfg     HTTPConfig
	httpSrv *http.Server
	mcpHTTP *mcpsrv.StreamableHTTPServer
}

// NewHTTPTransport builds the HTTP transport skeleton around an mcp-go
// *MCPServer. It validates configuration and fails closed:
//   - a missing bind address is an error (no implicit/wildcard bind),
//   - a nil Authenticator is an error (no unauthenticated exposure).
//
// It does NOT consult the feature flag — callers must gate construction on
// HTTPEnabled() so the default daemon path never instantiates it. It also does
// not bind a socket; that happens in Serve.
func NewHTTPTransport(srv *mcpsrv.MCPServer, cfg HTTPConfig) (*HTTPTransport, error) {
	if srv == nil {
		return nil, errors.New("transport: nil MCPServer")
	}
	if strings.TrimSpace(cfg.Addr) == "" {
		return nil, errors.New("transport: HTTP bind address is required (refusing to bind implicitly)")
	}
	if cfg.Auth == nil {
		return nil, errors.New("transport: Authenticator is required (refusing unauthenticated HTTP exposure)")
	}

	opts := []mcpsrv.StreamableHTTPOption{}
	if cfg.Stateless {
		opts = append(opts, mcpsrv.WithStateLess(true))
	}
	// WithHTTPContextFunc is the per-request seam where the authenticated
	// Identity (set by authMiddleware on the request context) is forwarded into
	// the MCP server's call context, so tool handlers can read it via
	// IdentityFromContext for authz/audit.
	opts = append(opts, mcpsrv.WithHTTPContextFunc(func(ctx context.Context, r *http.Request) context.Context {
		if id, ok := IdentityFromContext(r.Context()); ok {
			return withIdentity(ctx, id)
		}
		return ctx
	}))

	mcpHTTP := mcpsrv.NewStreamableHTTPServer(srv, opts...)

	readHdr := cfg.ReadHeaderTimeout
	if readHdr <= 0 {
		readHdr = 10 * time.Second
	}

	// Auth middleware wraps the mcp-go handler: every request authenticates
	// before reaching MCP dispatch.
	handler := authMiddleware(cfg.Auth, mcpHTTP)

	t := &HTTPTransport{
		cfg:     cfg,
		mcpHTTP: mcpHTTP,
		httpSrv: &http.Server{
			Addr:              cfg.Addr,
			Handler:           handler,
			ReadHeaderTimeout: readHdr,
			// TODO(ADR-0022, maintainer): TLS. Recommendation is reverse-proxy
			// termination (plain HTTP on a private bind). In-process TLS
			// (TLSConfig / ListenAndServeTLS) and mTLS (ClientCAs +
			// RequireAndVerifyClientCert) are future options gated on the
			// maintainer's network decision.
		},
	}
	return t, nil
}

// Name returns "http".
func (t *HTTPTransport) Name() string { return "http" }

// Serve binds the listener and serves until Shutdown or a fatal error. It
// blocks. A clean shutdown returns nil (http.ErrServerClosed is normalised).
func (t *HTTPTransport) Serve(ctx context.Context) error {
	// Honour ctx cancellation by triggering shutdown.
	go func() {
		<-ctx.Done()
		shutCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		_ = t.Shutdown(shutCtx)
	}()

	err := t.httpSrv.ListenAndServe()
	if errors.Is(err, http.ErrServerClosed) {
		return nil
	}
	if err != nil {
		return fmt.Errorf("transport http serve: %w", err)
	}
	return nil
}

// Shutdown gracefully stops the HTTP server. Safe to call concurrently and
// idempotent (http.Server.Shutdown is a no-op once closed).
func (t *HTTPTransport) Shutdown(ctx context.Context) error {
	if t.mcpHTTP != nil {
		// Drain mcp-go session state first (no-op in stateless mode).
		_ = t.mcpHTTP.Shutdown(ctx)
	}
	return t.httpSrv.Shutdown(ctx)
}

// Handler exposes the fully-wrapped HTTP handler (auth middleware + mcp-go
// StreamableHTTP). Primarily for tests (httptest) and for embedding behind an
// external mux without binding a socket.
func (t *HTTPTransport) Handler() http.Handler { return t.httpSrv.Handler }

var _ Transport = (*HTTPTransport)(nil)
