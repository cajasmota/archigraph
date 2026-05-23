package daemon

import (
	"encoding/json"
	"fmt"
	"testing"
)

// ── helpers ───────────────────────────────────────────────────────────────────

func testService(listTools MCPListToolsFunc, callTool MCPCallToolFunc) *Service {
	return &Service{
		mcpListTools: listTools,
		mcpCallTool:  callTool,
		progress:     make(map[string]*rebuildSession),
	}
}

// stubSchema is a minimal valid JSONSchema for test tools.
var stubSchema = json.RawMessage(`{"type":"object","properties":{}}`)

// ── MCPToolList tests ─────────────────────────────────────────────────────────

func TestMCPToolList_NilFunc_ReturnsEmpty(t *testing.T) {
	svc := testService(nil, nil)
	var reply MCPToolListReply
	if err := svc.MCPToolList(&MCPToolListArgs{}, &reply); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(reply.Tools) != 0 {
		t.Fatalf("expected empty tools when mcpListTools is nil, got %d", len(reply.Tools))
	}
}

func TestMCPToolList_ReturnsCatalog(t *testing.T) {
	wantTools := []MCPToolEntry{
		{Name: "archigraph_find", Description: "BM25 search", InputSchema: stubSchema},
		{Name: "archigraph_stats", Description: "Corpus metrics", InputSchema: stubSchema},
	}
	svc := testService(func(_ string) ([]MCPToolEntry, error) {
		return wantTools, nil
	}, nil)

	var reply MCPToolListReply
	if err := svc.MCPToolList(&MCPToolListArgs{}, &reply); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(reply.Tools) != 2 {
		t.Fatalf("expected 2 tools, got %d", len(reply.Tools))
	}
	if reply.Tools[0].Name != "archigraph_find" {
		t.Errorf("first tool name: %q", reply.Tools[0].Name)
	}
	if reply.Tools[1].Name != "archigraph_stats" {
		t.Errorf("second tool name: %q", reply.Tools[1].Name)
	}
}

func TestMCPToolList_PropagatesError(t *testing.T) {
	svc := testService(func(_ string) ([]MCPToolEntry, error) {
		return nil, fmt.Errorf("registry read failed")
	}, nil)

	var reply MCPToolListReply
	err := svc.MCPToolList(&MCPToolListArgs{}, &reply)
	if err == nil {
		t.Fatal("expected error when listTools returns error")
	}
}

func TestMCPToolList_InputSchemaIncluded(t *testing.T) {
	schema := json.RawMessage(`{"type":"object","properties":{"question":{"type":"string"}}}`)
	svc := testService(func(_ string) ([]MCPToolEntry, error) {
		return []MCPToolEntry{
			{Name: "archigraph_find", Description: "BM25 search", InputSchema: schema},
		}, nil
	}, nil)

	var reply MCPToolListReply
	if err := svc.MCPToolList(&MCPToolListArgs{}, &reply); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(reply.Tools) != 1 {
		t.Fatalf("expected 1 tool, got %d", len(reply.Tools))
	}
	var parsedSchema map[string]any
	if err := json.Unmarshal(reply.Tools[0].InputSchema, &parsedSchema); err != nil {
		t.Fatalf("inputSchema is not valid JSON: %v", err)
	}
	if parsedSchema["type"] != "object" {
		t.Errorf("inputSchema type: %v", parsedSchema["type"])
	}
}

// ── MCPToolCall tests ─────────────────────────────────────────────────────────

func TestMCPToolCall_NilFunc_ReturnsErrorBlock(t *testing.T) {
	svc := testService(nil, nil)
	var reply MCPToolCallReply
	if err := svc.MCPToolCall(&MCPToolCallArgs{Name: "archigraph_stats"}, &reply); err != nil {
		t.Fatalf("unexpected protocol error: %v", err)
	}
	if !reply.IsError {
		t.Fatal("expected IsError=true when mcpCallTool is nil")
	}
	if len(reply.Content) == 0 {
		t.Fatal("expected content block when mcpCallTool is nil")
	}
}

func TestMCPToolCall_NilArgs_ReturnsError(t *testing.T) {
	svc := testService(func(_ string) ([]MCPToolEntry, error) { return nil, nil }, nil)
	var reply MCPToolCallReply
	err := svc.MCPToolCall(nil, &reply)
	if err == nil {
		t.Fatal("expected error for nil args")
	}
}

func TestMCPToolCall_EmptyName_ReturnsError(t *testing.T) {
	svc := testService(nil, nil)
	var reply MCPToolCallReply
	err := svc.MCPToolCall(&MCPToolCallArgs{Name: ""}, &reply)
	if err == nil {
		t.Fatal("expected error for empty tool name")
	}
}

func TestMCPToolCall_DispatchesToHandler(t *testing.T) {
	called := false
	svc := testService(nil, func(name string, args map[string]any, cwd string) (MCPCallResult, error) {
		called = true
		if name != "archigraph_stats" {
			t.Errorf("unexpected tool name: %q", name)
		}
		return MCPCallResult{
			Content: []map[string]any{
				{"type": "text", "text": `{"node_count":42}`},
			},
		}, nil
	})

	var reply MCPToolCallReply
	if err := svc.MCPToolCall(&MCPToolCallArgs{Name: "archigraph_stats"}, &reply); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !called {
		t.Fatal("handler was not called")
	}
	if reply.IsError {
		t.Fatal("expected IsError=false on success")
	}
	if len(reply.Content) == 0 {
		t.Fatal("expected content block in reply")
	}
	text, _ := reply.Content[0]["text"].(string)
	if text != `{"node_count":42}` {
		t.Errorf("unexpected content: %q", text)
	}
}

func TestMCPToolCall_ForwardsCWD(t *testing.T) {
	var gotCWD string
	svc := testService(nil, func(_ string, args map[string]any, cwd string) (MCPCallResult, error) {
		gotCWD = cwd
		return MCPCallResult{Content: []map[string]any{{"type": "text", "text": "ok"}}}, nil
	})

	const wantCWD = "/home/user/myproject"
	var reply MCPToolCallReply
	if err := svc.MCPToolCall(&MCPToolCallArgs{
		Name: "archigraph_find",
		CWD:  wantCWD,
	}, &reply); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if gotCWD != wantCWD {
		t.Errorf("CWD not forwarded: got %q, want %q", gotCWD, wantCWD)
	}
}

func TestMCPToolCall_HandlerError_ReturnsErrorBlock(t *testing.T) {
	svc := testService(nil, func(_ string, _ map[string]any, _ string) (MCPCallResult, error) {
		return MCPCallResult{}, fmt.Errorf("internal tool failure")
	})

	var reply MCPToolCallReply
	if err := svc.MCPToolCall(&MCPToolCallArgs{Name: "archigraph_find"}, &reply); err != nil {
		t.Fatalf("unexpected protocol error: %v", err)
	}
	if !reply.IsError {
		t.Fatal("expected IsError=true on handler failure")
	}
	if len(reply.Content) == 0 {
		t.Fatal("expected error content block")
	}
}

func TestMCPToolCall_EmptyContent_NormalisedToEmptySlice(t *testing.T) {
	svc := testService(nil, func(_ string, _ map[string]any, _ string) (MCPCallResult, error) {
		return MCPCallResult{Content: nil}, nil
	})

	var reply MCPToolCallReply
	if err := svc.MCPToolCall(&MCPToolCallArgs{Name: "archigraph_stats"}, &reply); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if reply.Content == nil {
		t.Fatal("Content should be normalised to empty slice, not nil")
	}
}

// ── cwd-gate (#1769) tests ────────────────────────────────────────────────────

// TestMCPToolList_ForwardsCWD_ToListFunc verifies that the CWD from
// MCPToolListArgs is forwarded to the injected MCPListToolsFunc (#1769).
func TestMCPToolList_ForwardsCWD_ToListFunc(t *testing.T) {
	var receivedCWD string
	svc := testService(func(cwd string) ([]MCPToolEntry, error) {
		receivedCWD = cwd
		return []MCPToolEntry{{Name: "archigraph_find"}}, nil
	}, nil)

	const wantCWD = "/home/user/myproject"
	var reply MCPToolListReply
	if err := svc.MCPToolList(&MCPToolListArgs{CWD: wantCWD}, &reply); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if receivedCWD != wantCWD {
		t.Errorf("CWD not forwarded to list func: got %q, want %q", receivedCWD, wantCWD)
	}
}

// TestMCPToolList_NilArgs_EmptyCWD verifies that nil MCPToolListArgs is
// handled gracefully (cwd treated as "").
func TestMCPToolList_NilArgs_EmptyCWD(t *testing.T) {
	var receivedCWD string
	svc := testService(func(cwd string) ([]MCPToolEntry, error) {
		receivedCWD = cwd
		return []MCPToolEntry{{Name: "archigraph_find"}}, nil
	}, nil)

	var reply MCPToolListReply
	if err := svc.MCPToolList(nil, &reply); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if receivedCWD != "" {
		t.Errorf("expected empty cwd for nil args, got %q", receivedCWD)
	}
}

// TestMCPToolList_SentinelReturned verifies that when the listing func returns
// only the sentinel, the reply contains exactly one tool.
func TestMCPToolList_SentinelReturned(t *testing.T) {
	sentinel := MCPToolEntry{
		Name:        "archigraph_status",
		Description: "Archigraph: no indexed group covers this directory.",
	}
	svc := testService(func(_ string) ([]MCPToolEntry, error) {
		return []MCPToolEntry{sentinel}, nil
	}, nil)

	var reply MCPToolListReply
	if err := svc.MCPToolList(&MCPToolListArgs{CWD: "/tmp"}, &reply); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(reply.Tools) != 1 {
		t.Fatalf("expected 1 sentinel tool, got %d", len(reply.Tools))
	}
	if reply.Tools[0].Name != "archigraph_status" {
		t.Errorf("unexpected sentinel name: %q", reply.Tools[0].Name)
	}
}
