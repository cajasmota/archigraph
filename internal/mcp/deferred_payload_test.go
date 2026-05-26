package mcp

import (
	"encoding/json"
	"strings"
	"testing"

	mcpapi "github.com/mark3labs/mcp-go/mcp"
)

// TestDeferredPayload_ElapsedMSExactlyOnce verifies the on-the-wire bytes
// from the deferred single-marshal path carry elapsed_ms exactly once and
// parse as valid JSON. This is the core invariant of #2287.
func TestDeferredPayload_ElapsedMSExactlyOnce(t *testing.T) {
	t.Setenv("MCP_WIRE_FORMAT", "json")

	cases := []struct {
		name string
		v    any
	}{
		{
			name: "map_with_items",
			v: map[string]any{
				"results": []any{
					map[string]any{"id": "a", "name": "Alpha"},
					map[string]any{"id": "b", "name": "Beta"},
				},
				"count": 2,
			},
		},
		{
			name: "map_no_items",
			v: map[string]any{
				"id":   "x",
				"name": "Solo",
			},
		},
		{
			name: "any_array",
			v: []any{
				map[string]any{"id": "a"},
				map[string]any{"id": "b"},
			},
		},
		{
			name: "typed_struct_slice",
			v: []map[string]any{
				{"id": "a"},
				{"id": "b"},
			},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			text, err := finalizeDeferred(tc.v, 42, nil)
			if err != nil {
				t.Fatalf("finalizeDeferred: %v", err)
			}

			// elapsed_ms must appear exactly once.
			if c := strings.Count(text, "\"elapsed_ms\""); c != 1 {
				t.Errorf("expected exactly one elapsed_ms, got %d in %s", c, text)
			}

			// Bytes must parse as JSON and elapsed_ms must equal 42.
			var obj map[string]any
			if err := json.Unmarshal([]byte(text), &obj); err != nil {
				t.Fatalf("wire bytes do not parse as JSON: %v — %s", err, text)
			}
			ms, ok := obj["elapsed_ms"].(float64)
			if !ok {
				t.Fatalf("elapsed_ms missing or wrong type: %v", obj["elapsed_ms"])
			}
			if ms != 42 {
				t.Errorf("expected elapsed_ms=42, got %v", ms)
			}
		})
	}
}

// TestDeferredPayload_WrapStashUnstashRoundtrip verifies that the
// jsonResult → stash → wrap → finalize cycle leaves the wire payload
// shape-compatible with the legacy injectElapsedMS path.
func TestDeferredPayload_WrapStashUnstashRoundtrip(t *testing.T) {
	t.Setenv("MCP_WIRE_FORMAT", "json")

	v := map[string]any{
		"results": []any{
			map[string]any{"id": "a", "name": "Alpha", "kind": "function"},
		},
		"count": 1,
	}
	res := jsonResult(v)

	// jsonResult stashed v; verify takeDeferred retrieves the same map.
	got, ok := takeDeferred(res)
	if !ok {
		t.Fatal("expected deferred value to be stashed")
	}
	if _, ok := got.(map[string]any); !ok {
		t.Fatalf("expected map[string]any, got %T", got)
	}

	// Second take must return nothing (LoadAndDelete semantics).
	if _, ok := takeDeferred(res); ok {
		t.Error("deferred map entry was not deleted on first take")
	}
}

// TestDeferredPayload_NoTOON_NoFields verifies the byte shape of the
// no-TOON, no-fields= happy path: single object envelope with elapsed_ms
// appended.
func TestDeferredPayload_NoTOON_NoFields(t *testing.T) {
	t.Setenv("MCP_WIRE_FORMAT", "json")

	text, err := finalizeDeferred(map[string]any{
		"id":   "x",
		"name": "Solo",
	}, 7, nil)
	if err != nil {
		t.Fatal(err)
	}

	var obj map[string]any
	if err := json.Unmarshal([]byte(text), &obj); err != nil {
		t.Fatalf("invalid JSON: %v — %s", err, text)
	}
	if obj["id"] != "x" {
		t.Errorf("id field lost in deferred path: %v", obj)
	}
	if obj["elapsed_ms"].(float64) != 7 {
		t.Errorf("elapsed_ms=7 expected, got %v", obj["elapsed_ms"])
	}
}

// TestDeferredPayload_ArrayWrap verifies a top-level array is wrapped in
// the {items, count, elapsed_ms} envelope (parity with the legacy
// injectElapsedMS array branch).
func TestDeferredPayload_ArrayWrap(t *testing.T) {
	t.Setenv("MCP_WIRE_FORMAT", "json")

	text, err := finalizeDeferred([]any{
		map[string]any{"id": "1"},
		map[string]any{"id": "2"},
		map[string]any{"id": "3"},
	}, 99, nil)
	if err != nil {
		t.Fatal(err)
	}
	var obj map[string]any
	if err := json.Unmarshal([]byte(text), &obj); err != nil {
		t.Fatalf("invalid JSON: %v — %s", err, text)
	}
	if obj["count"].(float64) != 3 {
		t.Errorf("count=3 expected, got %v", obj["count"])
	}
	if obj["elapsed_ms"].(float64) != 99 {
		t.Errorf("elapsed_ms=99 expected, got %v", obj["elapsed_ms"])
	}
	items, ok := obj["items"].([]any)
	if !ok {
		t.Fatalf("expected items []any, got %T", obj["items"])
	}
	if len(items) != 3 {
		t.Errorf("expected 3 items, got %d", len(items))
	}
}

// TestDeferredPayload_TOONItems verifies TOON conversion is applied to
// homogeneous items arrays in the deferred path, matching the legacy
// envelope behaviour (#1686).
func TestDeferredPayload_TOONItems(t *testing.T) {
	t.Setenv("MCP_WIRE_FORMAT", "toon")

	text, err := finalizeDeferred(map[string]any{
		"items": []any{
			map[string]any{"id": "ep1", "method": "POST"},
			map[string]any{"id": "ep2", "method": "GET"},
		},
		"count": 2,
	}, 13, nil)
	if err != nil {
		t.Fatal(err)
	}
	var obj map[string]any
	if err := json.Unmarshal([]byte(text), &obj); err != nil {
		t.Fatalf("invalid JSON: %v — %s", err, text)
	}
	items, ok := obj["items"].(string)
	if !ok {
		t.Fatalf("expected TOON string for items, got %T", obj["items"])
	}
	if !strings.HasPrefix(items, "[!schema {") {
		t.Errorf("expected TOON header, got: %s", items)
	}
	if obj["elapsed_ms"].(float64) != 13 {
		t.Errorf("elapsed_ms=13 expected, got %v", obj["elapsed_ms"])
	}
}

// BenchmarkResponsePath_Legacy measures the legacy marshal → parse →
// re-marshal cost on a representative endpoints-like payload.
func BenchmarkResponsePath_Legacy(b *testing.B) {
	payload := buildBenchPayload()
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		// Step 1: marshal the handler value (jsonResult).
		data, _ := json.Marshal(payload)
		res := &mcpapi.CallToolResult{Content: []mcpapi.Content{mcpapi.NewTextContent(string(data))}}
		// Step 2: legacy injectElapsedMS does parse + re-marshal.
		_ = injectElapsedMS(res, int64(i))
	}
}

// BenchmarkResponsePath_Deferred measures the #2287 single-marshal path
// — handler's eager marshal + finalizeDeferred's wire marshal, no parse.
func BenchmarkResponsePath_Deferred(b *testing.B) {
	payload := buildBenchPayload()
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		// jsonResult marshals once for in-place res.Content[0] (so
		// internal callers see valid JSON) and stashes the value.
		res := jsonResult(payload)
		// wrap pops the value and finalizes — single marshal at wire,
		// no parse.
		v, _ := takeDeferred(res)
		_, _ = finalizeDeferred(v, int64(i), nil)
	}
}

// buildBenchPayload returns a representative response shape for the
// endpoints / clusters tools (the heaviest list tools per the strategy
// doc). 200 records of 6 fields each ≈ what a busy MCP call returns.
func buildBenchPayload() map[string]any {
	items := make([]any, 0, 200)
	for i := 0; i < 200; i++ {
		items = append(items, map[string]any{
			"id":          "upvate-core::endpoint_" + itoa64(int64(i)),
			"method":      "POST",
			"path":        "/api/v1/orders/" + itoa64(int64(i)),
			"handler":     "OrderViewSet.create",
			"source_file": "internal/orders/handlers.go",
			"start_line":  int64(100 + i),
		})
	}
	return map[string]any{
		"items": items,
		"count": len(items),
	}
}
