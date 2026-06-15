package mcp

// effect_contexts_4821_test.go — end-to-end test for the #4821 `effect_contexts`
// facet (control-flow epic #4820 part a): conditional/loop effect attribution +
// per-function cyclomatic complexity, surfaced via grafel_effects
// include="effect_contexts". Runs the REAL handler against small Python
// (Django-shaped) and TS (NestJS-shaped) fixtures in testdata/effect_contexts_4821/.

import (
	"path/filepath"
	"testing"

	"github.com/cajasmota/grafel/internal/graph"
)

func effectContextsTestServer(t *testing.T, sourceFile string, start, end int) *Server {
	t.Helper()
	doc := &graph.Document{
		Repo: "upvate-core",
		Entities: []graph.Entity{
			{
				ID: "op_sync", Name: "sync",
				Kind:          "SCOPE.Operation",
				QualifiedName: "svc.sync",
				SourceFile:    sourceFile, StartLine: start, EndLine: end,
			},
		},
	}
	srv := newTestServer(t, doc)
	abs, err := filepath.Abs(filepath.Join("testdata", "effect_contexts_4821"))
	if err != nil {
		t.Fatalf("abs testdata: %v", err)
	}
	srv.State.mu.Lock()
	srv.State.groups["test"].Repos["upvate-core"].Path = abs
	srv.State.mu.Unlock()
	return srv
}

func effectContextList(t *testing.T, out map[string]any) []map[string]any {
	t.Helper()
	raw, ok := out["effect_contexts"]
	if !ok {
		t.Fatalf("no effect_contexts in result: %v", out)
	}
	arr, ok := raw.([]any)
	if !ok {
		t.Fatalf("effect_contexts not an array: %T", raw)
	}
	out2 := make([]map[string]any, 0, len(arr))
	for _, e := range arr {
		if m, ok := e.(map[string]any); ok {
			out2 = append(out2, m)
		}
	}
	return out2
}

func findEffectCtx(ctxs []map[string]any, effect string) map[string]any {
	for _, c := range ctxs {
		if c["effect"] == effect {
			return c
		}
	}
	return nil
}

// TestEffectContexts_Python: db_read top-level (unconditional), db_write under
// an `if` (conditional + condition), http_out inside a `for` (in_loop), plus a
// cyclomatic_complexity number.
func TestEffectContexts_Python(t *testing.T) {
	srv := effectContextsTestServer(t, "sync_service.py", 1, 7)
	out := callEffects(t, srv, "sync", "effect_contexts")

	if sup, _ := out["effect_contexts_supported"].(bool); !sup {
		t.Fatalf("effect_contexts_supported should be true for python; got %v", out["effect_contexts_supported"])
	}
	if cx, _ := out["cyclomatic_complexity"].(float64); cx < 3 {
		t.Errorf("cyclomatic_complexity = %v; want >= 3", out["cyclomatic_complexity"])
	}
	if _, ok := out["branch_count"]; !ok {
		t.Errorf("branch_count missing")
	}

	ctxs := effectContextList(t, out)

	if read := findEffectCtx(ctxs, "db_read"); read == nil {
		t.Fatalf("no db_read context; got %v", ctxs)
	} else if cond, _ := read["conditional"].(bool); cond {
		t.Errorf("db_read should be unconditional, got %v", read)
	}

	if write := findEffectCtx(ctxs, "db_write"); write == nil {
		t.Fatalf("no db_write context; got %v", ctxs)
	} else {
		if cond, _ := write["conditional"].(bool); !cond {
			t.Errorf("db_write should be conditional, got %v", write)
		}
		if write["condition"] == nil || write["condition"] == "" {
			t.Errorf("db_write should carry condition, got %v", write)
		}
	}

	if http := findEffectCtx(ctxs, "http_out"); http == nil {
		t.Fatalf("no http_out context; got %v", ctxs)
	} else if loop, _ := http["in_loop"].(bool); !loop {
		t.Errorf("http_out should be in_loop, got %v", http)
	}
}

// TestEffectContexts_TS: same shape on a NestJS-flavoured TS service.
func TestEffectContexts_TS(t *testing.T) {
	srv := effectContextsTestServer(t, "sync.service.ts", 2, 11)
	out := callEffects(t, srv, "sync", "effect_contexts")

	if sup, _ := out["effect_contexts_supported"].(bool); !sup {
		t.Fatalf("effect_contexts_supported should be true for jsts; got %v", out["effect_contexts_supported"])
	}
	if cx, _ := out["cyclomatic_complexity"].(float64); cx < 3 {
		t.Errorf("cyclomatic_complexity = %v; want >= 3", out["cyclomatic_complexity"])
	}

	ctxs := effectContextList(t, out)

	var sawCondWrite, sawLoopHTTP bool
	for _, c := range ctxs {
		switch c["effect"] {
		case "db_write":
			cond, _ := c["conditional"].(bool)
			if cond && c["condition"] != nil && c["condition"] != "" {
				sawCondWrite = true
			}
		case "http_out":
			if loop, _ := c["in_loop"].(bool); loop {
				sawLoopHTTP = true
			}
		}
	}
	if !sawCondWrite {
		t.Errorf("expected conditional db_write with condition; got %v", ctxs)
	}
	if !sawLoopHTTP {
		t.Errorf("expected http_out in_loop; got %v", ctxs)
	}
}

// TestEffectContexts_DefaultUnchanged: without include=effect_contexts the
// payload carries none of the new keys (#2828 opt-in contract).
func TestEffectContexts_DefaultUnchanged(t *testing.T) {
	srv := effectContextsTestServer(t, "sync_service.py", 1, 7)
	out := callEffects(t, srv, "sync", "")
	for _, k := range []string{"effect_contexts", "effect_contexts_supported", "cyclomatic_complexity", "branch_count"} {
		if _, ok := out[k]; ok {
			t.Errorf("key %q must be absent when facet not requested", k)
		}
	}
}
