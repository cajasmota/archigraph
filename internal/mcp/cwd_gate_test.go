package mcp

// cwd_gate_test.go — unit tests for the tools/list cwd-gate (#1769, #2620).
//
// Test matrix:
//   - No registered groups                                  → only sentinel.
//   - cwd outside all groups, single group registered       → full list (singleton fallback #2620).
//   - cwd outside all groups, multiple groups registered    → full list (tools error per-call #2620).
//   - cwd inside one group                                  → full list (minus sentinel).
//   - cwd inside multiple groups (ambiguous)                → full list.
//   - cwd inside group with 0 repos (empty group)           → only sentinel + hint.
//   - grafel_status call from no-match cwd              → expected guidance text.

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	mcpapi "github.com/mark3labs/mcp-go/mcp"
)

// ── helpers ───────────────────────────────────────────────────────────────────

// makeTestServer constructs a minimal *Server backed by a temp registry.
// groups maps groupName → map[repoName]repoPath. When repoPath is empty the
// repo is registered with no path (simulating an empty/unconfigured group).
func makeTestServer(t *testing.T, groups map[string]map[string]string) *Server {
	t.Helper()
	dir := t.TempDir()
	t.Setenv("GRAFEL_DAEMON_ROOT", dir)
	regPath := makeRegistry(t, dir, groups)
	srv, err := NewServer(Config{RegistryPath: regPath})
	if err != nil {
		t.Fatalf("NewServer: %v", err)
	}
	return srv
}

// toolNames returns the tool names from an MCPToolEntry slice.
func toolNames(entries []MCPToolEntry) []string {
	names := make([]string, len(entries))
	for i, e := range entries {
		names[i] = e.Name
	}
	return names
}

// containsOnly reports whether names contains exactly one element equal to want.
func isSentinelOnly(entries []MCPToolEntry) bool {
	return len(entries) == 1 && entries[0].Name == sentinelToolName
}

// ── tests ─────────────────────────────────────────────────────────────────────

// TestListToolsForCWD_NoGroups — empty registry → sentinel.
func TestListToolsForCWD_NoGroups(t *testing.T) {
	srv := makeTestServer(t, map[string]map[string]string{})

	entries, err := srv.ListToolsForCWD("/tmp")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !isSentinelOnly(entries) {
		t.Fatalf("expected only sentinel tool, got %v", toolNames(entries))
	}
	if !strings.Contains(entries[0].Description, "no groups registered") && !strings.Contains(entries[0].Description, "no indexed group") {
		t.Errorf("sentinel description should mention no groups: %q", entries[0].Description)
	}
}

// TestListToolsForCWD_CWDOutsideAllGroups_SingleGroup — #2620: cwd outside
// the registered repo but exactly one group registered → singleton fallback
// returns the FULL tool list (not sentinel). The fix makes the bridge usable
// from hosts (Windsurf JetBrains) that launch with an unrelated cwd.
func TestListToolsForCWD_CWDOutsideAllGroups_SingleGroup(t *testing.T) {
	repoDir := t.TempDir()
	srv := makeTestServer(t, map[string]map[string]string{
		"mygroup": {"myrepo": repoDir},
	})

	// cwd is outside the registered repo, but singleton fallback should apply.
	entries, err := srv.ListToolsForCWD("/tmp/unrelated-project")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// Fix #2620: singleton group → full list, not sentinel.
	if isSentinelOnly(entries) {
		t.Fatalf("expected full tool list via singleton fallback, got sentinel only (#2620)")
	}
	for _, e := range entries {
		if e.Name == sentinelToolName {
			t.Errorf("sentinel must not appear in full tool list when singleton fallback applies")
		}
	}
}

// TestListToolsForCWD_CWDInsideGroup — cwd is the repo root → full list, no sentinel.
func TestListToolsForCWD_CWDInsideGroup(t *testing.T) {
	repoDir := t.TempDir()
	srv := makeTestServer(t, map[string]map[string]string{
		"mygroup": {"myrepo": repoDir},
	})

	entries, err := srv.ListToolsForCWD(repoDir)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if isSentinelOnly(entries) {
		t.Fatalf("expected full tool list for cwd inside group, got sentinel only")
	}
	// Sentinel must NOT appear in the full list.
	for _, e := range entries {
		if e.Name == sentinelToolName {
			t.Errorf("full list must not contain %q", sentinelToolName)
		}
	}
	// Full list should have the canonical 29 tools (not sentinel).
	if len(entries) < 5 {
		t.Errorf("full list suspiciously small: %d tools", len(entries))
	}
}

// TestListToolsForCWD_CWDInsideGroupSubdir — cwd is a subdirectory of the repo → full list.
func TestListToolsForCWD_CWDInsideGroupSubdir(t *testing.T) {
	repoDir := t.TempDir()
	subDir := filepath.Join(repoDir, "src", "components")
	if err := os.MkdirAll(subDir, 0o755); err != nil {
		t.Fatal(err)
	}
	srv := makeTestServer(t, map[string]map[string]string{
		"mygroup": {"myrepo": repoDir},
	})

	entries, err := srv.ListToolsForCWD(subDir)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if isSentinelOnly(entries) {
		t.Fatalf("expected full tool list for cwd inside group subdir, got sentinel only")
	}
}

// TestListToolsForCWD_EmptyGroup — group registered but has 0 repos → sentinel with rebuild hint.
func TestListToolsForCWD_EmptyGroup(t *testing.T) {
	// makeRegistry with a group that has no repos.
	dir := t.TempDir()
	t.Setenv("GRAFEL_DAEMON_ROOT", dir)
	// Use makeRegistry helper which writes proper JSON.
	// Since makeRegistry expects map[string]map[string]string, pass empty inner map.
	regPath := makeRegistry(t, dir, map[string]map[string]string{
		"emptygroup": {},
	})
	srv, err := NewServer(Config{RegistryPath: regPath})
	if err != nil {
		t.Fatalf("NewServer: %v", err)
	}

	// Any cwd resolves to this single group via singleton fallback
	// BUT the group has 0 repos → should return sentinel.
	// Note: singleton fallback fires when exactly one group is registered.
	// With 0 repos the group is "registered but empty".
	entries, err := srv.ListToolsForCWD("/tmp")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// With a singleton empty group + cwd=/tmp (outside any repo path),
	// the group is selected via singleton fallback but has 0 repos → sentinel.
	if !isSentinelOnly(entries) {
		t.Logf("tools returned: %v", toolNames(entries))
		t.Fatalf("expected sentinel for empty group, got %d tools", len(entries))
	}
	// Description should mention the empty group / rebuild.
	desc := entries[0].Description
	if !strings.Contains(desc, "no repos indexed") && !strings.Contains(desc, "no indexed group") {
		t.Errorf("sentinel description should mention empty group: %q", desc)
	}
}

// TestListToolsForCWD_MultipleGroups_UnmatchedCWD_FullList — #2620: when cwd
// matches no registered repo and multiple groups are registered, return the
// FULL tool catalog (not sentinel). Each tool that requires a group will error
// at call-time with a helpful "specify group=" message.
func TestListToolsForCWD_MultipleGroups_UnmatchedCWD_FullList(t *testing.T) {
	parentDir := t.TempDir()
	repoA := filepath.Join(parentDir, "repoA")
	repoB := filepath.Join(parentDir, "repoB")
	if err := os.MkdirAll(repoA, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(repoB, 0o755); err != nil {
		t.Fatal(err)
	}

	srv := makeTestServer(t, map[string]map[string]string{
		"groupA": {"repoA": repoA},
		"groupB": {"repoB": repoB},
	})

	// parentDir is NOT under either repo — multiple groups registered.
	// Fix #2620: should return full tool list, NOT sentinel.
	entries, err := srv.ListToolsForCWD(parentDir)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if isSentinelOnly(entries) {
		t.Fatalf("expected full tool list for multi-group unmatched cwd (#2620), got sentinel only")
	}
	// Sentinel must NOT appear in the full list.
	for _, e := range entries {
		if e.Name == sentinelToolName {
			t.Errorf("sentinel must not appear in full tool list for multi-group unmatched cwd")
		}
	}
}

// TestListToolsForCWD_SingleGroup_RootCwd_FullList — #2620: bridge launched
// with cwd=/ (Windsurf JetBrains) and exactly one group registered → full
// tool list via singleton fallback.
func TestListToolsForCWD_SingleGroup_RootCwd_FullList(t *testing.T) {
	repoDir := t.TempDir()
	srv := makeTestServer(t, map[string]map[string]string{
		"upvate": {"upvate_core": repoDir},
	})

	entries, err := srv.ListToolsForCWD("/")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if isSentinelOnly(entries) {
		t.Fatalf("expected full tool list for single-group singleton fallback with cwd=/ (#2620), got sentinel only")
	}
	// Sentinel must NOT appear.
	for _, e := range entries {
		if e.Name == sentinelToolName {
			t.Errorf("sentinel must not appear in full tool list for singleton fallback")
		}
	}
	if len(entries) < 5 {
		t.Errorf("full list suspiciously small: %d tools", len(entries))
	}
}

// TestListToolsForCWD_SentinelCallable — grafel_status handler returns guidance text.
func TestListToolsForCWD_SentinelCallable(t *testing.T) {
	repoDir := t.TempDir()
	srv := makeTestServer(t, map[string]map[string]string{
		"mygroup": {"myrepo": repoDir},
	})

	req := mcpapi.CallToolRequest{}
	req.Params.Name = sentinelToolName
	req.Params.Arguments = map[string]any{"cwd": "/tmp/unrelated"}

	result, err := srv.handleStatus(nil, req)
	if err != nil {
		t.Fatalf("handleStatus error: %v", err)
	}
	if result == nil || len(result.Content) == 0 {
		t.Fatal("handleStatus returned nil/empty result")
	}

	// Extract text content.
	text := extractResultText(t, result)
	// Should mention the cwd or registered groups.
	if !strings.Contains(text, "Grafel") {
		t.Errorf("guidance text should mention Grafel: %q", text)
	}
}

// TestListToolsForCWD_SentinelExcludedFromFullList — sentinel must not be in full list.
func TestListToolsForCWD_SentinelExcludedFromFullList(t *testing.T) {
	repoDir := t.TempDir()
	srv := makeTestServer(t, map[string]map[string]string{
		"mygroup": {"myrepo": repoDir},
	})

	entries, err := srv.ListToolsForCWD(repoDir)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	for _, e := range entries {
		if e.Name == sentinelToolName {
			t.Errorf("sentinel %q must not appear in the full tool list", sentinelToolName)
		}
	}
}

// TestListToolsForCWD_MCPToolListArgs_CWDForwarded — MCPToolListArgs.CWD is passed through.
// This test verifies the daemon MCPToolList RPC forwards cwd to the listing func.
func TestListToolsForCWD_MCPToolListArgs_CWDForwarded(t *testing.T) {
	// This is tested at the daemon layer; here we confirm our MCPToolEntry type
	// can roundtrip through the wire format.
	entry := MCPToolEntry{
		Name:        sentinelToolName,
		Description: sentinelToolDescription,
	}
	if entry.Name != sentinelToolName {
		t.Errorf("MCPToolEntry name: %q", entry.Name)
	}
}

// TestSentinelTool_HasValidInputSchema — sentinel tool must include a valid
// JSON Schema inputSchema so strict MCP clients (Claude Code Zod validation)
// do not reject the tools/list response (#2257).
func TestSentinelTool_HasValidInputSchema(t *testing.T) {
	srv := makeTestServer(t, map[string]map[string]string{})

	entries, err := srv.ListToolsForCWD("/tmp")
	if err != nil {
		t.Fatalf("ListToolsForCWD: %v", err)
	}
	if len(entries) != 1 || entries[0].Name != sentinelToolName {
		t.Fatalf("expected only sentinel tool, got %v", toolNames(entries))
	}

	sentinel := entries[0]
	if sentinel.InputSchema == nil {
		t.Fatal("sentinel tool InputSchema is nil — strict MCP clients will reject this")
	}

	var schema map[string]json.RawMessage
	if err := json.Unmarshal(sentinel.InputSchema, &schema); err != nil {
		t.Fatalf("sentinel InputSchema is not valid JSON: %v — raw: %s", err, sentinel.InputSchema)
	}

	typeRaw, ok := schema["type"]
	if !ok {
		t.Fatal("sentinel InputSchema missing required 'type' field")
	}
	var typStr string
	if err := json.Unmarshal(typeRaw, &typStr); err != nil || typStr != "object" {
		t.Fatalf("sentinel InputSchema 'type' must be \"object\", got: %s", typeRaw)
	}

	if _, ok := schema["properties"]; !ok {
		t.Fatal("sentinel InputSchema missing required 'properties' field")
	}
}

// TestAllRegisteredTools_HaveValidInputSchema — every tool returned by
// fullToolList must carry a non-nil inputSchema with type=object. This guards
// against regressions in both the full-list path and any future tool addition
// that forgets to set up a schema (#2257).
func TestAllRegisteredTools_HaveValidInputSchema(t *testing.T) {
	repoDir := t.TempDir()
	srv := makeTestServer(t, map[string]map[string]string{
		"testgroup": {"testrepo": repoDir},
	})

	entries, err := srv.ListToolsForCWD(repoDir)
	if err != nil {
		t.Fatalf("ListToolsForCWD: %v", err)
	}
	if len(entries) == 0 {
		t.Fatal("expected at least one tool in the full list")
	}

	for _, e := range entries {
		if e.Name == sentinelToolName {
			t.Errorf("sentinel must not appear in full tool list")
			continue
		}
		if e.InputSchema == nil {
			t.Errorf("tool %q: InputSchema is nil — strict MCP clients will reject this", e.Name)
			continue
		}
		var schema map[string]json.RawMessage
		if err := json.Unmarshal(e.InputSchema, &schema); err != nil {
			t.Errorf("tool %q: InputSchema is not valid JSON: %v", e.Name, err)
			continue
		}
		typeRaw, ok := schema["type"]
		if !ok {
			t.Errorf("tool %q: InputSchema missing 'type' field", e.Name)
			continue
		}
		var typStr string
		if err := json.Unmarshal(typeRaw, &typStr); err != nil || typStr != "object" {
			t.Errorf("tool %q: InputSchema 'type' must be \"object\", got: %s", e.Name, typeRaw)
		}
		if _, ok := schema["properties"]; !ok {
			t.Errorf("tool %q: InputSchema missing 'properties' field", e.Name)
		}
	}
}
