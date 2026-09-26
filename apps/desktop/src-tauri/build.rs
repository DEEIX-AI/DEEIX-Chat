// Declaring the app manifest makes Tauri enforce the ACL on our own commands
// (without it, every webview may call every app command). Each command must
// then be granted explicitly in capabilities/*.json.
fn main() {
    tauri_build::try_build(tauri_build::Attributes::new().app_manifest(
        tauri_build::AppManifest::new().commands(&[
            // session (content tabs)
            "get_server",
            "set_remote_server",
            "set_local_server",
            "leave_server",
            "local_sign_in",
            "store_session",
            "clear_session",
            "refresh_session",
            "start_oauth_loopback",
            "stop_oauth_loopback",
            // tab strip only
            "tabs_list",
            "tabs_open",
            "tabs_activate",
            "tabs_close",
            "tabs_move",
        ]),
    ))
    .expect("failed to run tauri-build");
}
