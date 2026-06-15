package javascript_test

// tauri_test.go — tests for the custom_js_tauri frontend IPC extractor.
//
// Proves the cross-language caller half of the Tauri IPC contract (#5023 /
// #5105): frontend invoke("cmd") → CALLS tauri:command:<cmd> (the token the
// Rust #[tauri::command] entity carries as its Name), and emit/listen → a
// shared tauri:event:<evt> channel node also keyed identically to the Rust
// side, so the by-name resolver joins JS↔Rust producer/consumer.

import (
	"context"
	"testing"

	extreg "github.com/cajasmota/grafel/internal/extractor"
	"github.com/cajasmota/grafel/internal/types"
)

// extractTauriRecords runs the named extractor and returns the raw records
// (with Relationships), unlike extract() which flattens to entitySummary.
func extractTauriRecords(t *testing.T, name string, file extreg.FileInput) []types.EntityRecord {
	t.Helper()
	e, ok := extreg.Get(name)
	if !ok {
		t.Fatalf("extractor %q not registered", name)
	}
	recs, err := e.Extract(context.Background(), file)
	if err != nil {
		t.Fatalf("extract error: %v", err)
	}
	return recs
}

// findTauriRel reports whether any record carries a relationship of the given
// kind from→to.
func findTauriRel(recs []types.EntityRecord, kind, fromID, toID string) bool {
	for _, e := range recs {
		for _, r := range e.Relationships {
			if r.Kind == kind && r.FromID == fromID && r.ToID == toID {
				return true
			}
		}
	}
	return false
}

func hasTauriEntity(recs []types.EntityRecord, kind, name string) bool {
	for _, e := range recs {
		if e.Kind == kind && e.Name == name {
			return true
		}
	}
	return false
}

// Happy path: invoke("cmd") emits a cross-language CALLS edge to the Rust
// command token, in all common call shapes.
func TestTauriFrontend_InvokeCallsCommand(t *testing.T) {
	src := `
import { invoke } from "@tauri-apps/api/core";
import { core } from "@tauri-apps/api";

async function run() {
  const a = await invoke("greet", { name: "x" });
  const b = await core.invoke("read_file", { path: "/tmp" });
  const c = await window.__TAURI__.invoke("get_version");
}
`
	recs := extractTauriRecords(t, "custom_js_tauri", fi("app.ts", "typescript", src))
	for _, cmd := range []string{"greet", "read_file", "get_version"} {
		if !hasTauriEntity(recs, "SCOPE.Operation", "tauri:invoke:"+cmd) {
			t.Errorf("expected ipc_invoke caller tauri:invoke:%s", cmd)
		}
		if !findTauriRel(recs, string(types.RelationshipKindCalls),
			"tauri:invoke:"+cmd, "tauri:command:"+cmd) {
			t.Errorf("expected CALLS tauri:invoke:%s -> tauri:command:%s (cross-lang)", cmd, cmd)
		}
	}
}

// emit/listen frontend sites share ONE channel node per event name, keyed
// identically to the Rust side so producer↔consumer join across languages.
func TestTauriFrontend_EmitListenChannels(t *testing.T) {
	src := `
import { emit, emitTo, listen, once } from "@tauri-apps/api/event";
import { getCurrentWindow } from "@tauri-apps/api/window";

async function wire() {
  await emit("frontend-ready", { ok: true });
  await emitTo("main", "targeted-evt", 1);
  await getCurrentWindow().emit("window-evt", 2);
  await listen("backend-ready", (e) => {});
  await once("one-shot", (e) => {});
}
`
	recs := extractTauriRecords(t, "custom_js_tauri", fi("events.ts", "typescript", src))

	for _, evt := range []string{"frontend-ready", "targeted-evt", "window-evt"} {
		if !hasTauriEntity(recs, "SCOPE.Datastore", "tauri:event:"+evt) {
			t.Errorf("expected channel node tauri:event:%s", evt)
		}
		if !findTauriRel(recs, string(types.RelationshipKindPublishesTo),
			"tauri:fe_publish:"+evt, "tauri:event:"+evt) {
			t.Errorf("expected PUBLISHES_TO tauri:event:%s", evt)
		}
	}
	for _, evt := range []string{"backend-ready", "one-shot"} {
		if !findTauriRel(recs, string(types.RelationshipKindSubscribesTo),
			"tauri:fe_subscribe:"+evt, "tauri:event:"+evt) {
			t.Errorf("expected SUBSCRIBES_TO tauri:event:%s", evt)
		}
	}
}

// A frontend emit and listen on the SAME event name must share ONE channel
// node, so producer↔consumer join through it.
func TestTauriFrontend_EmitListenSameChannelJoin(t *testing.T) {
	src := `
import { emit, listen } from "@tauri-apps/api/event";
async function p() { await emit("sync", 1); }
async function c() { await listen("sync", (e) => {}); }
`
	recs := extractTauriRecords(t, "custom_js_tauri", fi("both.ts", "typescript", src))
	count := 0
	for _, e := range recs {
		if e.Name == "tauri:event:sync" {
			count++
		}
	}
	if count != 1 {
		t.Errorf("expected exactly 1 shared tauri:event:sync channel, got %d", count)
	}
}

// Wrong-language no-op: a .go file is never a Tauri frontend.
func TestTauriFrontend_WrongLanguageNoOp(t *testing.T) {
	src := `
import { invoke } from "@tauri-apps/api/core";
await invoke("greet");
`
	recs := extractTauriRecords(t, "custom_js_tauri", fi("app.go", "go", src))
	if len(recs) != 0 {
		t.Errorf("expected no entities for non-JS/TS language, got %d", len(recs))
	}
}

// No-match no-op: a bare invoke()/emit() with NO Tauri import signal must not
// be misattributed to Tauri.
func TestTauriFrontend_NoTauriSignalNoOp(t *testing.T) {
	src := `
import { invoke } from "some-other-rpc-lib";
async function run() {
  await invoke("doThing");
  await emit("evt");
  await listen("evt", () => {});
}
`
	recs := extractTauriRecords(t, "custom_js_tauri", fi("app.ts", "typescript", src))
	if len(recs) != 0 {
		t.Errorf("expected no entities without a Tauri import signal, got %d", len(recs))
	}
}

// Cross-language join proof at the token level (#5105): the frontend invoke
// edge ToID is byte-identical to the Name the Rust #[tauri::command] entity
// carries, which is the contract the by-name resolver binds on.
func TestTauriFrontend_CrossLangTokenParity(t *testing.T) {
	src := `
import { invoke } from "@tauri-apps/api/core";
await invoke("greet");
`
	recs := extractTauriRecords(t, "custom_js_tauri", fi("app.ts", "typescript", src))
	var toID string
	for _, e := range recs {
		for _, r := range e.Relationships {
			if r.Kind == string(types.RelationshipKindCalls) {
				toID = r.ToID
			}
		}
	}
	// This MUST equal the Rust-side entity Name (internal/custom/rust/tauri.go
	// emits makeEntity("tauri:command:"+cmdName, ...)).
	if toID != "tauri:command:greet" {
		t.Errorf("cross-lang token mismatch: got %q, want tauri:command:greet", toID)
	}
}
