package rust_test

// tauri_test.go — tests for custom_rust_tauri extractor.
// Proves ipc_extraction, main_renderer_split, and native_module_imports.

import (
	"testing"

	"github.com/cajasmota/grafel/internal/types"
)

func TestTauri_IPCCommand(t *testing.T) {
	src := `
use tauri::Manager;

#[tauri::command]
async fn greet(name: String) -> String {
    format!("Hello, {}!", name)
}

#[tauri::command]
fn get_version() -> &'static str {
    "1.0.0"
}
`
	ents := extract(t, "custom_rust_tauri", fi("commands.rs", "rust", src))
	if !containsEntity(ents, "SCOPE.Operation", "tauri:command:greet") {
		t.Error("expected tauri:command:greet ipc_command")
	}
	if !containsEntity(ents, "SCOPE.Operation", "tauri:command:get_version") {
		t.Error("expected tauri:command:get_version ipc_command")
	}
}

func TestTauri_GenerateHandler(t *testing.T) {
	src := `
use tauri::Manager;

fn main() {
    tauri::Builder::default()
        .invoke_handler(tauri::generate_handler![greet, get_version, read_file])
        .run(tauri::generate_context!())
        .expect("error while running tauri application");
}
`
	ents := extract(t, "custom_rust_tauri", fi("main.rs", "rust", src))
	if !containsEntity(ents, "SCOPE.Component", "tauri:generate_handler") {
		t.Error("expected tauri:generate_handler ipc_handler_registration")
	}
}

func TestTauri_BuilderMainProcess(t *testing.T) {
	src := `
use tauri::Manager;

fn main() {
    tauri::Builder::default()
        .run(tauri::generate_context!())
        .unwrap();
}
`
	ents := extract(t, "custom_rust_tauri", fi("main.rs", "rust", src))
	if !containsEntity(ents, "SCOPE.Component", "tauri:Builder::default") {
		t.Error("expected tauri:Builder::default main_process component")
	}
	if !containsEntitySubtype(ents, "SCOPE.Function", "main_entry_point") {
		t.Error("expected main_entry_point function")
	}
}

func TestTauri_NativeModuleImports(t *testing.T) {
	src := `
use tauri::Manager;

fn setup(app: &mut tauri::App) -> Result<(), Box<dyn std::error::Error>> {
    let path = tauri::api::path::app_data_dir(app.config()).unwrap();
    let dir = tauri::api::dir::read_dir(&path, true).unwrap();
    Ok(())
}
`
	ents := extract(t, "custom_rust_tauri", fi("setup.rs", "rust", src))
	if !containsEntity(ents, "SCOPE.Pattern", "tauri:api:path") {
		t.Error("expected tauri:api:path native_module")
	}
	if !containsEntity(ents, "SCOPE.Pattern", "tauri:api:dir") {
		t.Error("expected tauri:api:dir native_module")
	}
}

func TestTauri_NativePlugin(t *testing.T) {
	src := `
use tauri::Manager;
use tauri_plugin_sql::{Migration, MigrationKind};
use tauri_plugin_fs::FsExt;

fn main() {
    tauri::Builder::default()
        .plugin(tauri_plugin_sql::Builder::new().build())
        .run(tauri::generate_context!())
        .unwrap();
}
`
	ents := extract(t, "custom_rust_tauri", fi("main.rs", "rust", src))
	if !containsEntity(ents, "SCOPE.Pattern", "tauri:plugin:sql") {
		t.Error("expected tauri:plugin:sql native_plugin")
	}
}

func TestTauri_WindowBuilder(t *testing.T) {
	src := `
use tauri::Manager;

fn create_window(app: &tauri::AppHandle) {
    tauri::WindowBuilder::new(app, "main", tauri::WindowUrl::App("index.html".into()))
        .title("My App")
        .build()
        .unwrap();
}
`
	ents := extract(t, "custom_rust_tauri", fi("window.rs", "rust", src))
	if !containsEntity(ents, "SCOPE.Component", "tauri:WindowBuilder") {
		t.Error("expected tauri:WindowBuilder renderer_window component")
	}
}

func TestTauri_NoMatch(t *testing.T) {
	src := `
use std::collections::HashMap;

fn main() {
    let mut map = HashMap::new();
    map.insert("key", "value");
}
`
	ents := extract(t, "custom_rust_tauri", fi("main.rs", "rust", src))
	if len(ents) != 0 {
		t.Errorf("expected no entities for non-tauri file, got %d", len(ents))
	}
}

func TestTauri_FixtureFile(t *testing.T) {
	src := readFixture(t, "testdata/tauri_app.rs")
	ents := extract(t, "custom_rust_tauri", fi("tauri_app.rs", "rust", src))
	if !containsEntity(ents, "SCOPE.Operation", "tauri:command:greet") {
		t.Error("expected tauri:command:greet from fixture")
	}
	if !containsEntity(ents, "SCOPE.Operation", "tauri:command:read_file") {
		t.Error("expected tauri:command:read_file from fixture")
	}
	if !containsEntity(ents, "SCOPE.Component", "tauri:generate_handler") {
		t.Error("expected tauri:generate_handler from fixture")
	}
	if !containsEntity(ents, "SCOPE.Component", "tauri:Builder::default") {
		t.Error("expected tauri:Builder::default from fixture")
	}
	if !containsEntity(ents, "SCOPE.Pattern", "tauri:api:path") {
		t.Error("expected tauri:api:path from fixture")
	}

	// IPC topology edges (#5023): registration → command CALLS + event pub/sub.
	recs := extractRecords(t, "custom_rust_tauri", fi("tauri_app.rs", "rust", src))
	if _, ok := findRel(recs, string(types.RelationshipKindCalls),
		"tauri:generate_handler", "tauri:command:greet"); !ok {
		t.Error("expected CALLS tauri:generate_handler -> tauri:command:greet from fixture")
	}
	if _, ok := findRel(recs, string(types.RelationshipKindCalls),
		"tauri:generate_handler", "tauri:command:read_file"); !ok {
		t.Error("expected CALLS tauri:generate_handler -> tauri:command:read_file from fixture")
	}
	if _, ok := findRel(recs, string(types.RelationshipKindPublishesTo),
		"tauri:publish:backend-ready", "tauri:event:backend-ready"); !ok {
		t.Error("expected PUBLISHES_TO backend-ready event from fixture")
	}
	if _, ok := findRel(recs, string(types.RelationshipKindPublishesTo),
		"tauri:publish:file-read-progress", "tauri:event:file-read-progress"); !ok {
		t.Error("expected PUBLISHES_TO file-read-progress (emit_all) from fixture")
	}
	if _, ok := findRel(recs, string(types.RelationshipKindSubscribesTo),
		"tauri:subscribe:frontend-command", "tauri:event:frontend-command"); !ok {
		t.Error("expected SUBSCRIBES_TO frontend-command event from fixture")
	}
}

// -----------------------------------------------------------------------------
// #5023 — IPC topology edges: invoke()->command CALLS + emit/listen pub/sub.
// -----------------------------------------------------------------------------

func TestTauri_GenerateHandlerCallsCommands(t *testing.T) {
	src := `
use tauri::Manager;

#[tauri::command]
fn greet() {}

#[tauri::command]
fn read_file() {}

fn main() {
    tauri::Builder::default()
        .invoke_handler(tauri::generate_handler![greet, commands::read_file])
        .run(tauri::generate_context!())
        .unwrap();
}
`
	recs := extractRecords(t, "custom_rust_tauri", fi("main.rs", "rust", src))
	if _, ok := findRel(recs, string(types.RelationshipKindCalls),
		"tauri:generate_handler", "tauri:command:greet"); !ok {
		t.Error("expected CALLS registration -> tauri:command:greet")
	}
	// Path-qualified entry commands::read_file resolves to the final ident.
	if _, ok := findRel(recs, string(types.RelationshipKindCalls),
		"tauri:generate_handler", "tauri:command:read_file"); !ok {
		t.Error("expected CALLS registration -> tauri:command:read_file (path-qualified)")
	}
}

func TestTauri_EmitPublishes(t *testing.T) {
	src := `
use tauri::Manager;

fn notify(app: &tauri::AppHandle) {
    app.emit("status-changed", "ok").unwrap();
    app.emit_all("broadcast-evt", 1).unwrap();
    app.emit_to("main-window", "targeted-evt", 2).unwrap();
}
`
	recs := extractRecords(t, "custom_rust_tauri", fi("evt.rs", "rust", src))
	for _, evt := range []string{"status-changed", "broadcast-evt", "targeted-evt"} {
		if _, ok := findRel(recs, string(types.RelationshipKindPublishesTo),
			"tauri:publish:"+evt, "tauri:event:"+evt); !ok {
			t.Errorf("expected PUBLISHES_TO tauri:event:%s", evt)
		}
		if !containsEntity(toSummaries(recs), "SCOPE.Datastore", "tauri:event:"+evt) {
			t.Errorf("expected synthetic ipc_event channel tauri:event:%s", evt)
		}
	}
}

func TestTauri_ListenSubscribes(t *testing.T) {
	src := `
use tauri::Manager;

fn wire(app: &tauri::AppHandle) {
    app.listen("renderer-event", |_e| {});
    app.listen_global("global-event", |_e| {});
    app.once("one-shot", |_e| {});
}
`
	recs := extractRecords(t, "custom_rust_tauri", fi("listen.rs", "rust", src))
	for _, evt := range []string{"renderer-event", "global-event", "one-shot"} {
		if _, ok := findRel(recs, string(types.RelationshipKindSubscribesTo),
			"tauri:subscribe:"+evt, "tauri:event:"+evt); !ok {
			t.Errorf("expected SUBSCRIBES_TO tauri:event:%s", evt)
		}
	}
}

func TestTauri_EmitListenSameChannelJoin(t *testing.T) {
	// An emit and a listen on the SAME event name must share ONE channel node,
	// so producer ↔ consumer join through it.
	src := `
use tauri::Manager;

fn p(app: &tauri::AppHandle) { app.emit("sync", 1).unwrap(); }
fn c(app: &tauri::AppHandle) { app.listen("sync", |_e| {}); }
`
	recs := extractRecords(t, "custom_rust_tauri", fi("both.rs", "rust", src))
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

// toSummaries adapts raw EntityRecords to the entitySummary shape used by
// containsEntity (kind+name only).
func toSummaries(recs []types.EntityRecord) []entitySummary {
	out := make([]entitySummary, 0, len(recs))
	for _, e := range recs {
		out = append(out, entitySummary{Kind: e.Kind, Name: e.Name, Subtype: e.Subtype})
	}
	return out
}
