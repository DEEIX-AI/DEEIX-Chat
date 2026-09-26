// OAuth loopback redirect receiver (RFC 8252 §7.3).
//
// The server's provider-auth bridge only accepts `http://127.0.0.1:<port>/oauth/callback`
// as the redirect for the desktop client id. We bind an ephemeral port on the
// loopback interface for the duration of one sign-in, hand the full callback
// URL to the webview, and shut the listener down. Nothing is parsed here: the
// web app owns state verification and the grant exchange.

use std::io::{Read, Write};
use std::net::{Ipv4Addr, TcpListener, TcpStream};
use std::sync::Mutex;
use std::time::Duration;

use serde::Serialize;
use tauri::{AppHandle, Emitter, Manager, Runtime};

/// Event emitted to the webview with the full callback URL.
pub const CALLBACK_EVENT: &str = "oauth-loopback-callback";

const CALLBACK_PATH: &str = "/oauth/callback";
const ACCEPT_TIMEOUT: Duration = Duration::from_secs(600);
const READ_TIMEOUT: Duration = Duration::from_secs(5);
const MAX_REQUEST_BYTES: usize = 16 * 1024;

#[derive(Default)]
pub struct LoopbackState {
    active: Mutex<Option<u16>>,
}

#[derive(Debug, Serialize)]
pub struct LoopbackError {
    message: String,
}

impl<E: std::fmt::Display> From<E> for LoopbackError {
    fn from(error: E) -> Self {
        Self {
            message: error.to_string(),
        }
    }
}

/// Start listening and return the redirect URI the web app must register with
/// the server. Only one listener runs at a time; starting a new one while a
/// previous sign-in is pending replaces it.
#[tauri::command]
pub fn start_oauth_loopback<R: Runtime>(
    app: AppHandle<R>,
    state: tauri::State<'_, LoopbackState>,
) -> Result<String, LoopbackError> {
    let listener = TcpListener::bind((Ipv4Addr::LOCALHOST, 0))?;
    let port = listener.local_addr()?.port();
    listener.set_nonblocking(true)?;

    *state.active.lock().map_err(|e| e.to_string())? = Some(port);

    let handle = app.clone();
    std::thread::spawn(move || serve_once(handle, listener, port));

    Ok(format!("http://127.0.0.1:{port}{CALLBACK_PATH}"))
}

fn serve_once<R: Runtime>(app: AppHandle<R>, listener: TcpListener, port: u16) {
    let deadline = std::time::Instant::now() + ACCEPT_TIMEOUT;
    let stream = loop {
        if !is_current(&app, port) || std::time::Instant::now() > deadline {
            return;
        }
        match listener.accept() {
            Ok((stream, _)) => break stream,
            Err(e) if e.kind() == std::io::ErrorKind::WouldBlock => {
                std::thread::sleep(Duration::from_millis(50));
            }
            Err(_) => return,
        }
    };

    let target = match read_request_target(&stream) {
        Some(t) => t,
        None => {
            let _ = respond(stream, 400, "Bad request");
            return;
        }
    };

    if !target.starts_with(CALLBACK_PATH) {
        let _ = respond(stream, 404, "Not found");
        return;
    }

    let url = format!("http://127.0.0.1:{port}{target}");
    let _ = respond(
        stream,
        200,
        "Sign-in complete. You can return to DEEIX Chat.",
    );
    clear_if_current(&app, port);

    if let Some(window) = app.get_webview_window("main") {
        let _ = window.set_focus();
    }
    let _ = app.emit(CALLBACK_EVENT, url);
}

fn read_request_target(stream: &TcpStream) -> Option<String> {
    let mut stream = stream.try_clone().ok()?;
    stream.set_read_timeout(Some(READ_TIMEOUT)).ok()?;
    let mut buf = Vec::with_capacity(1024);
    let mut chunk = [0u8; 512];
    loop {
        let n = stream.read(&mut chunk).ok()?;
        if n == 0 {
            break;
        }
        buf.extend_from_slice(&chunk[..n]);
        if buf.len() > MAX_REQUEST_BYTES {
            return None;
        }
        if buf.windows(4).any(|w| w == b"\r\n\r\n") {
            break;
        }
    }
    let line = buf.split(|&b| b == b'\n').next()?;
    let line = std::str::from_utf8(line).ok()?.trim_end_matches('\r');
    let mut parts = line.split(' ');
    if parts.next()? != "GET" {
        return None;
    }
    let target = parts.next()?;
    if !target.starts_with('/') || target.contains("..") {
        return None;
    }
    Some(target.to_string())
}

fn respond(mut stream: TcpStream, status: u16, body: &str) -> std::io::Result<()> {
    let reason = match status {
        200 => "OK",
        400 => "Bad Request",
        _ => "Not Found",
    };
    let html = format!(
        "<!doctype html><meta charset=\"utf-8\"><title>DEEIX Chat</title>\
         <body style=\"font-family:system-ui;padding:2rem\">{body}</body>"
    );
    write!(
        stream,
        "HTTP/1.1 {status} {reason}\r\nContent-Type: text/html; charset=utf-8\r\n\
         Content-Length: {}\r\nConnection: close\r\nCache-Control: no-store\r\n\r\n{html}",
        html.len()
    )?;
    stream.flush()
}

fn is_current<R: Runtime>(app: &AppHandle<R>, port: u16) -> bool {
    app.state::<LoopbackState>()
        .active
        .lock()
        .map(|guard| *guard == Some(port))
        .unwrap_or(false)
}

fn clear_if_current<R: Runtime>(app: &AppHandle<R>, port: u16) {
    if let Ok(mut guard) = app.state::<LoopbackState>().active.lock() {
        if *guard == Some(port) {
            *guard = None;
        }
    }
}

/// Cancel a pending listener (user closed the sign-in dialog).
#[tauri::command]
pub fn stop_oauth_loopback(state: tauri::State<'_, LoopbackState>) -> Result<(), LoopbackError> {
    *state.active.lock().map_err(|e| e.to_string())? = None;
    Ok(())
}

#[cfg(test)]
mod tests {
    use super::*;

    fn request(raw: &[u8]) -> Option<String> {
        let raw = raw.to_vec();
        let listener = TcpListener::bind((Ipv4Addr::LOCALHOST, 0)).unwrap();
        let addr = listener.local_addr().unwrap();
        let client = std::thread::spawn(move || {
            let mut s = TcpStream::connect(addr).unwrap();
            s.write_all(&raw).unwrap();
            s.flush().unwrap();
            s
        });
        let (stream, _) = listener.accept().unwrap();
        let target = read_request_target(&stream);
        drop(client.join().unwrap());
        target
    }

    #[test]
    fn extracts_target_from_get() {
        let t =
            request(b"GET /oauth/callback?grant=abc&state=xyz HTTP/1.1\r\nHost: 127.0.0.1\r\n\r\n");
        assert_eq!(t.as_deref(), Some("/oauth/callback?grant=abc&state=xyz"));
    }

    #[test]
    fn rejects_non_get() {
        assert!(request(b"POST /oauth/callback HTTP/1.1\r\n\r\n").is_none());
    }

    #[test]
    fn rejects_traversal_and_relative_targets() {
        assert!(request(b"GET /oauth/../callback HTTP/1.1\r\n\r\n").is_none());
        assert!(request(b"GET oauth/callback HTTP/1.1\r\n\r\n").is_none());
    }

    #[test]
    fn rejects_oversized_request() {
        let mut raw = b"GET /oauth/callback?x=".to_vec();
        raw.extend(std::iter::repeat(b'a').take(MAX_REQUEST_BYTES + 1));
        raw.extend_from_slice(b" HTTP/1.1\r\n\r\n");
        assert!(request(&raw).is_none());
    }

    #[test]
    fn response_is_well_formed() {
        let listener = TcpListener::bind((Ipv4Addr::LOCALHOST, 0)).unwrap();
        let addr = listener.local_addr().unwrap();
        let client = std::thread::spawn(move || {
            let mut s = TcpStream::connect(addr).unwrap();
            let mut out = String::new();
            s.read_to_string(&mut out).unwrap();
            out
        });
        let (stream, _) = listener.accept().unwrap();
        respond(stream, 200, "done").unwrap();
        let out = client.join().unwrap();
        assert!(out.starts_with("HTTP/1.1 200 OK\r\n"));
        assert!(out.contains("Cache-Control: no-store"));
        assert!(out.ends_with("done</body>"));
    }
}
