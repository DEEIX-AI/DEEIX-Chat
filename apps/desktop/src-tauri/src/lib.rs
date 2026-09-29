// DEEIX Chat desktop shell: window/tray lifecycle, OAuth loopback, updater and
// session storage. No business logic — everything else is the apps/web build.

mod dialog;
mod distribution;
mod oauth_loopback;
mod paths;
mod policy;
mod session;
mod sidecar;
mod tabs;
mod tray;

use tauri::Manager;

#[cfg_attr(mobile, tauri::mobile_entry_point)]
pub fn run() {
    // One TLS provider for both the updater and session refresh.
    session::ensure_tls_provider();

    // Before the builder: portable setup, WebView2 check, policy, and the
    // identifier a portable copy runs under (single-instance key + keychain).
    let mut context = tauri::generate_context!();
    let distribution = distribution::bootstrap(&mut context);
    let portable = distribution.portable.clone();

    tauri::Builder::default()
        .plugin(tauri_plugin_single_instance::init(|app, _args, _cwd| {
            tray::show_main_window(app);
        }))
        // Registered in every mode: `check_update_notice` uses it from Rust.
        // The webview only gets its permissions in `auto` mode (grant_updater).
        .plugin(tauri_plugin_updater::Builder::new().build())
        .plugin(tauri_plugin_process::init())
        .plugin(tauri_plugin_opener::init())
        // Rust-side only: no `shell:*` permission is granted to the webview.
        .plugin(tauri_plugin_shell::init())
        .manage(distribution)
        .manage(oauth_loopback::LoopbackState::default())
        .manage(sidecar::SidecarState::default())
        .manage(tabs::TabsState::default())
        .invoke_handler(tauri::generate_handler![
            distribution::get_distribution,
            distribution::check_update_notice,
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
        .setup(move |app| {
            // Plugins are set up first, so a second instance has already
            // handed over and exited by now.
            let paths = paths::AppPaths::resolve(app.handle(), portable.as_ref())?;
            paths.reset_tmp();
            app.manage(paths);
            distribution::grant_updater(app.handle())?;
            tabs::init(app.handle()).map_err(|e| e.0)?;
            #[cfg(desktop)]
            tray::create_tray(app.handle())?;
            Ok(())
        })
        .build(context)
        .expect("error while building DEEIX Chat desktop")
        .run(|app, event| match event {
            tauri::RunEvent::Exit => sidecar::stop_blocking(app),
            #[cfg(target_os = "macos")]
            tauri::RunEvent::Reopen { .. } => tray::show_main_window(app),
            _ => {}
        });
}
