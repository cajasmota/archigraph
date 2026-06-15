// Package transport defines the MCP server transport seam for grafel.
//
// grafel's MCP server (internal/mcp.Server) wraps a transport-agnostic
// mark3labs/mcp-go *MCPServer. Historically there has been exactly one way to
// drive it for agents — stdio (see ADR-0004): the per-machine daemon is
// reached through the `grafel mcp-bridge` stdio process, which proxies to
// the daemon's Unix-socket / named-pipe RPC surface. The OS permissions on
// that socket are the entire trust boundary; there is no authentication,
// because every caller is the same local user.
//
// This package extracts the implicit "transport" concept into an explicit
// Transport interface so that additional transports can slot in beside stdio
// without disturbing the local-only default path. The first additional
// transport under evaluation is an authenticated, shared Streamable-HTTP
// server for team deployments (ADR-0022, issue #4296).
//
// IMPORTANT — this is an EVALUATION seam (ADR-0022). The HTTP transport here is
// a feature-flagged, OFF-BY-DEFAULT skeleton with a STUB authenticator. It is
// not wired into the production daemon and its security model is an explicit
// maintainer decision. Do not enable it in production without that sign-off.
package transport

import "context"

// Transport drives an MCP server until it is told to stop. Implementations own
// their wire (stdio pipes, an HTTP listener, …) and block in Serve until the
// connection closes or Shutdown is called.
//
// The interface is deliberately small: grafel's mcp.Server already holds
// the *MCPServer and all tool registration. A Transport only decides how bytes
// reach it.
type Transport interface {
	// Name is a short, stable identifier for logs/metrics ("stdio", "http").
	Name() string

	// Serve runs the transport, blocking until the underlying connection
	// closes or Shutdown is invoked. It returns the terminal error, if any.
	// Serve is expected to honour ctx cancellation as a shutdown signal.
	Serve(ctx context.Context) error

	// Shutdown asks the transport to stop accepting work and unblock Serve.
	// It must be safe to call from another goroutine and idempotent.
	Shutdown(ctx context.Context) error
}
