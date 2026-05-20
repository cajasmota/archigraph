package main

// daemon_mcp_rpc.go wires the internal/mcp tool catalog and dispatcher
// into the daemon.Config as MCPListTools / MCPCallTool function values.
//
// This file lives in cmd/archigraph (not internal/daemon) to break the
// import cycle: internal/mcp imports internal/daemon for layout paths.
// The wiring layer sits above both packages, so it can import freely.
//
// The *mcp.Server is initialised lazily on first call (via sync.Once)
// using the default registry path (~/.archigraph/registry.json). This
// mirrors the standalone `archigraph mcp serve` startup without
// blocking the daemon's socket listener.

import (
	"context"
	"encoding/json"
	"fmt"
	"sort"
	"sync"

	mcpapi "github.com/mark3labs/mcp-go/mcp"

	"github.com/cajasmota/archigraph/internal/daemon"
	"github.com/cajasmota/archigraph/internal/mcp"
)

// mcpServerOnce guards single initialisation of the global MCP server used
// by the daemon's MCPToolList / MCPToolCall RPC methods.
var (
	mcpServerOnce    sync.Once
	mcpServerShared  *mcp.Server
	mcpServerInitErr error
)

// mcpServerInstance returns the lazily-initialised *mcp.Server. The first
// call constructs it from the default registry; subsequent calls return the
// cached instance. Returns an error string if construction failed.
func mcpServerInstance() (*mcp.Server, error) {
	mcpServerOnce.Do(func() {
		srv, err := mcp.NewServer(mcp.Config{})
		if err != nil {
			mcpServerInitErr = fmt.Errorf("mcp server init: %w", err)
			return
		}
		mcpServerShared = srv
	})
	return mcpServerShared, mcpServerInitErr
}

// daemonMCPListTools is the MCPListToolsFunc injected into daemon.Config.
// It returns the 14-tool catalog derived from the *mcp.Server.
func daemonMCPListTools() ([]daemon.MCPToolEntry, error) {
	srv, err := mcpServerInstance()
	if err != nil {
		return nil, err
	}

	toolMap := srv.MCP.ListTools()

	// Sort for deterministic output.
	names := make([]string, 0, len(toolMap))
	for n := range toolMap {
		names = append(names, n)
	}
	sort.Strings(names)

	out := make([]daemon.MCPToolEntry, 0, len(names))
	for _, name := range names {
		st := toolMap[name]
		// Marshal the Tool to get the canonical inputSchema JSON,
		// which honours both InputSchema and RawInputSchema.
		raw, err := json.Marshal(st.Tool)
		if err != nil {
			continue
		}
		var m map[string]json.RawMessage
		if err := json.Unmarshal(raw, &m); err != nil {
			continue
		}
		entry := daemon.MCPToolEntry{Name: name}
		if v, ok := m["description"]; ok {
			_ = json.Unmarshal(v, &entry.Description)
		}
		if v, ok := m["inputSchema"]; ok {
			entry.InputSchema = v
		}
		out = append(out, entry)
	}
	return out, nil
}

// daemonMCPCallTool is the MCPCallToolFunc injected into daemon.Config.
// It dispatches a single tool call via the *mcp.Server's registered handler,
// forwarding CWD for ADR-0008 routing.
func daemonMCPCallTool(name string, args map[string]any, cwd string) (daemon.MCPCallResult, error) {
	srv, err := mcpServerInstance()
	if err != nil {
		return daemon.MCPCallResult{
			IsError: true,
			Content: []map[string]any{
				{"type": "text", "text": fmt.Sprintf("mcp server unavailable: %v", err)},
			},
		}, nil
	}

	// Look up the handler.
	st := srv.MCP.GetTool(name)
	if st == nil {
		return daemon.MCPCallResult{
			IsError: true,
			Content: []map[string]any{
				{"type": "text", "text": fmt.Sprintf("tool not found: %s", name)},
			},
		}, nil
	}

	// Build the CallToolRequest. Inject CWD into arguments so
	// ADR-0008 CWD-aware routing works identically to the stdio path.
	callArgs := make(map[string]any, len(args)+1)
	for k, v := range args {
		callArgs[k] = v
	}
	if cwd != "" {
		if _, exists := callArgs["cwd"]; !exists {
			callArgs["cwd"] = cwd
		}
	}

	req := mcpapi.CallToolRequest{}
	req.Params.Name = name
	req.Params.Arguments = callArgs

	result, err := st.Handler(context.Background(), req)
	if err != nil {
		return daemon.MCPCallResult{
			IsError: true,
			Content: []map[string]any{
				{"type": "text", "text": fmt.Sprintf("tool error: %v", err)},
			},
		}, nil
	}
	if result == nil {
		return daemon.MCPCallResult{Content: []map[string]any{}}, nil
	}

	return daemon.MCPCallResult{
		IsError: result.IsError,
		Content: mcpContentToMaps(result.Content),
	}, nil
}

// mcpContentToMaps converts the mcp-go Content slice to the
// []map[string]any wire shape expected by the bridge.
func mcpContentToMaps(content []mcpapi.Content) []map[string]any {
	if len(content) == 0 {
		return []map[string]any{}
	}
	out := make([]map[string]any, 0, len(content))
	for _, c := range content {
		raw, err := json.Marshal(c)
		if err != nil {
			continue
		}
		var m map[string]any
		if err := json.Unmarshal(raw, &m); err != nil {
			continue
		}
		out = append(out, m)
	}
	return out
}
