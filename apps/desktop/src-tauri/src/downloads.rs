// Downloads started by the web app (`<a download>`: images, exports, files).
//
// Without a download handler wry cancels them on macOS (WKWebView asks the
// navigation delegate and wry answers Cancel), so the button does nothing.
// With one, every platform saves to the user's Downloads folder under a free
// name (wry picks it, `name (1).png` on a clash), and the webview that started
// it gets `downloads:finished`: a webview shows no download UI of its own.

use std::collections::HashMap;
use std::path::Path;
use std::sync::Mutex;

use serde::Serialize;
use tauri::webview::DownloadEvent;
use tauri::{Emitter, EventTarget, Manager, Runtime, Webview, WebviewBuilder};

const FINISHED_EVENT: &str = "downloads:finished";

/// File names of downloads in flight, by URL: macOS reports no path when a
/// download finishes, so the name is taken when it is requested.
#[derive(Default)]
pub struct DownloadsState {
    pending: Mutex<HashMap<String, String>>,
}

#[derive(Clone, Serialize)]
#[serde(rename_all = "camelCase")]
struct FinishedPayload {
    file_name: String,
    success: bool,
}

pub fn configure<R: Runtime>(builder: WebviewBuilder<R>) -> WebviewBuilder<R> {
    builder.on_download(|webview, event| match event {
        DownloadEvent::Requested { url, destination } => {
            // wry falls back to the working directory (`/` for a bundled app)
            // when the OS reports no Downloads folder; refuse rather than fail there.
            if !destination.is_absolute() || !destination.parent().is_some_and(Path::is_dir) {
                notify(&webview, file_name(destination), false);
                return false;
            }
            let state = webview.state::<DownloadsState>();
            let mut pending = state.pending.lock().unwrap_or_else(|e| e.into_inner());
            pending.insert(url.to_string(), file_name(destination));
            true
        }
        DownloadEvent::Finished { url, path, success } => {
            let state = webview.state::<DownloadsState>();
            let requested = state
                .pending
                .lock()
                .unwrap_or_else(|e| e.into_inner())
                .remove(url.as_str());
            let Some(file_name) = path.as_deref().map(file_name).or(requested) else {
                return true;
            };
            notify(&webview, file_name, success);
            true
        }
        _ => true,
    })
}

fn notify<R: Runtime>(webview: &Webview<R>, file_name: String, success: bool) {
    let _ = webview.emit_to(
        EventTarget::webview(webview.label()),
        FINISHED_EVENT,
        FinishedPayload { file_name, success },
    );
}

fn file_name(path: &Path) -> String {
    path.file_name()
        .map(|name| name.to_string_lossy().into_owned())
        .unwrap_or_default()
}

#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    fn file_name_keeps_the_deduplicated_name() {
        assert_eq!(
            file_name(Path::new("/Users/a/Downloads/chart (1).png")),
            "chart (1).png"
        );
        assert_eq!(file_name(Path::new("/")), "");
    }
}
