// Tauri application backend fixture

use tauri::{Builder, Manager, AppHandle};
use tauri::api::path;

/// IPC command: greet user
#[tauri::command]
async fn greet(name: String) -> String {
    format!("Hello, {}!", name)
}

/// IPC command: read a file
#[tauri::command]
fn read_file(path: String, app: AppHandle) -> Result<String, String> {
    let base = tauri::api::path::app_data_dir(&app.config())
        .ok_or("no app data dir")?;
    // Publish a progress event back to the renderer.
    app.emit_all("file-read-progress", 50).unwrap();
    Ok(format!("{:?}", base.join(path)))
}

/// Background worker that emits and listens to IPC events.
fn wire_events(app: &AppHandle) {
    // Event-publish site.
    app.emit("backend-ready", ()).unwrap();
    // Event-subscribe site (renderer → backend).
    app.listen("frontend-command", move |event| {
        println!("got event: {:?}", event.payload());
    });
}

/// Main entry point — Rust backend (main process)
fn main() {
    tauri::Builder::default()
        .invoke_handler(tauri::generate_handler![greet, read_file])
        .run(tauri::generate_context!())
        .expect("error while running tauri application");
}
