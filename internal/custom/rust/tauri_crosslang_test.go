package rust_test

// tauri_crosslang_test.go — end-to-end proof of the Tauri JS↔Rust IPC link
// (#5023 / #5105). The Rust extractor emits the `#[tauri::command]` handler as
// an entity Named "tauri:command:<cmd>"; the JS/TS extractor emits a frontend
// `invoke("<cmd>")` CALLS edge whose ToID is the SAME token. We run BOTH
// extractors, merge the records, build the resolver's by-name index, and assert
// the cross-language CALLS edge binds to the real Rust command entity — the
// actual frontend→backend link, with no bespoke cross-language linker.

import (
	"context"
	"testing"

	extreg "github.com/cajasmota/grafel/internal/extractor"
	"github.com/cajasmota/grafel/internal/resolve"
	"github.com/cajasmota/grafel/internal/types"

	// Register BOTH the Rust and the JS/TS Tauri extractors.
	_ "github.com/cajasmota/grafel/internal/custom/javascript"
	_ "github.com/cajasmota/grafel/internal/custom/rust"
)

func runExtractor(t *testing.T, name, path, lang, src string) []types.EntityRecord {
	t.Helper()
	e, ok := extreg.Get(name)
	if !ok {
		t.Fatalf("extractor %q not registered", name)
	}
	recs, err := e.Extract(context.Background(), extreg.FileInput{
		Path: path, Language: lang, Content: []byte(src),
	})
	if err != nil {
		t.Fatalf("%s extract error: %v", name, err)
	}
	return recs
}

func TestTauri_CrossLang_InvokeBindsToCommand(t *testing.T) {
	rustSrc := `
use tauri::Manager;

#[tauri::command]
async fn greet(name: String) -> String { format!("Hi {}", name) }

fn main() {
    tauri::Builder::default()
        .invoke_handler(tauri::generate_handler![greet])
        .run(tauri::generate_context!())
        .unwrap();
}
`
	tsSrc := `
import { invoke } from "@tauri-apps/api/core";
async function run() { await invoke("greet", { name: "x" }); }
`
	rustRecs := runExtractor(t, "custom_rust_tauri", "src-tauri/main.rs", "rust", rustSrc)
	tsRecs := runExtractor(t, "custom_js_tauri", "src/api.ts", "typescript", tsSrc)

	all := append(append([]types.EntityRecord{}, rustRecs...), tsRecs...)
	// Assign IDs the way the indexer does, then build the by-name index.
	for i := range all {
		if all[i].ID == "" {
			all[i].ID = all[i].ComputeID()
		}
	}
	idx := resolve.BuildIndex(all)

	// The Rust command entity must resolve under its stable token.
	cmdID, ok := idx.Lookup("tauri:command:greet")
	if !ok {
		t.Fatal("tauri:command:greet (Rust #[tauri::command]) did not resolve in by-name index")
	}

	// Collect the frontend invoke CALLS edge and resolve it.
	var invokeRels []types.RelationshipRecord
	for _, e := range tsRecs {
		for _, r := range e.Relationships {
			if r.Kind == string(types.RelationshipKindCalls) && r.ToID == "tauri:command:greet" {
				invokeRels = append(invokeRels, r)
			}
		}
	}
	if len(invokeRels) == 0 {
		t.Fatal("frontend invoke(\"greet\") produced no CALLS edge to tauri:command:greet")
	}
	resolve.References(invokeRels, idx)
	if invokeRels[0].ToID != cmdID {
		t.Errorf("cross-lang invoke->command edge resolved to %q, want Rust command %q",
			invokeRels[0].ToID, cmdID)
	}
}
