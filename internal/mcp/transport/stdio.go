package transport

import (
	"context"

	mcpsrv "github.com/mark3labs/mcp-go/server"
)

// StdioTransport is the default, local-only MCP transport (ADR-0004). It drives
// the wrapped *MCPServer over the process's stdin/stdout via mcp-go's
// ServeStdio. This is the transport behind the per-machine model: there is no
// authentication because the trust boundary is the OS — the caller is always
// the same local user that owns the process.
//
// StdioTransport exists primarily to demonstrate that the pre-existing stdio
// path satisfies the Transport interface; internal/mcp.Server.ServeStdio
// remains the production entrypoint and is unchanged by this seam.
type StdioTransport struct {
	srv *mcpsrv.MCPServer
}

// NewStdioTransport wraps an mcp-go *MCPServer as a stdio Transport.
func NewStdioTransport(srv *mcpsrv.MCPServer) *StdioTransport {
	return &StdioTransport{srv: srv}
}

// Name returns "stdio".
func (t *StdioTransport) Name() string { return "stdio" }

// Serve runs ServeStdio until stdin closes. mcp-go's ServeStdio does not take a
// context; ctx cancellation is honoured cooperatively by ServeStdio's own
// signal handling, so we simply block on it here.
func (t *StdioTransport) Serve(ctx context.Context) error {
	return mcpsrv.ServeStdio(t.srv)
}

// Shutdown is a no-op for stdio: the transport ends when stdin closes. It is
// provided to satisfy the Transport interface.
func (t *StdioTransport) Shutdown(ctx context.Context) error { return nil }

// compile-time assertion.
var _ Transport = (*StdioTransport)(nil)
