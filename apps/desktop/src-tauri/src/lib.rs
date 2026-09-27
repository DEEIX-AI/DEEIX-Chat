// DEEIX Chat desktop shell: window/tray lifecycle, OAuth loopback, updater and
// session storage. No business logic — everything else is the apps/web build.

mod oauth_loopback;
mod session;
mod sidecar;
mod tabs;
mod tray;

#[cfg_attr(mobile, tauri::mobile_entry_point)]
pub fn run() {
    // One TLS provider for both the updater and session refresh.
    session::ensure_tls_provider();

    tauri::Builder::default()
        .plugin(tauri_plugin_single_instance::init(|app, _args, _cwd| {
            tray::show_main_window(app);
        }))
        .plugin(tauri_plugin_updater::Builder::new().build())
        .plugin(tauri_plugin_process::init())
        .plugin(tauri_plugin_opener::init())
        // Rust-side only: no `shell:*` permission is granted to the webview.
        .plugin(tauri_plugin_shell::init())
        .manage(oauth_loopback::LoopbackState::default())
        .manage(sidecar::SidecarState::default())
        .manage(tabs::TabsState::default())
        .invoke_handler(tauri::generate_handler![
            session::get_server,
            session::set_remote_server,
            session::set_local_server,
            session::local_sign_in,
            session::leave_server,
            session::store_session,
            session::clear_session,
            session::refresh_session,
            oauth_loopback::start_oauth_loopback,
            oauth_loopback::stop_oauth_loopback,
            tabs::tabs_list,
            tabs::tabs_open,
            tabs::tabs_activate,
            tabs::tabs_close,
            tabs::tabs_move,
        ])
        .setup(|app| {
            tabs::init(app.handle()).map_err(|e| e.0)?;
            #[cfg(desktop)]
            tray::create_tray(app.handle())?;
            Ok(())
        })
        .build(tauri::generate_context!())
        .expect("error while building DEEIX Chat desktop")
        .run(|app, event| match event {
            tauri::RunEvent::Exit => sidecar::stop_blocking(app),
            #[cfg(target_os = "macos")]
            tauri::RunEvent::Reopen { .. } => tray::show_main_window(app),
            _ => {}
        });
}
