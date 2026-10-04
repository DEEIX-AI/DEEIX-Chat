// OAuth loopback redirect receiver (RFC 8252 §7.3): an ephemeral loopback port
// for one sign-in, whose callback URL is handed to the webview unparsed.

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

/// Start listening and return the redirect URI; a new listener replaces a pending one.
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
            let _ = respond(stream, 400);
            return;
        }
    };

    if !target.starts_with(CALLBACK_PATH) {
        let _ = respond(stream, 404);
        return;
    }

    let url = format!("http://127.0.0.1:{port}{target}");
    let _ = respond(stream, 200);
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

/// The web app's logos, so this page carries the same mark as the setup and login screens.
const LOGO_LIGHT: &str = include_str!("../../../web/public/logo.svg");
const LOGO_DARK: &str = include_str!("../../../web/public/logo-white.svg");

/// Styles and script for the loopback page. The page is served by a throwaway
/// listener with no assets and no build step, so everything is inlined and
/// nothing is fetched from the network. Colours, type and controls mirror the
/// web app's tokens (`apps/web/app/globals.css`) and the desktop setup screen.
const PAGE_STYLE: &str = r#"
:root {
  color-scheme: light dark;
  --background: oklch(97.86% 0.0027 106.45);
  --foreground: oklch(0.3438 0.0269 95.7226);
  --muted-foreground: oklch(0.6059 0.0075 97.4233);
}
@media (prefers-color-scheme: dark) {
  :root {
    --background: oklch(23.89% 0.0019 106.54);
    --foreground: oklch(0.8074 0.0142 93.0137);
    --muted-foreground: oklch(0.7713 0.0169 99.0657);
  }
}
*, *::before, *::after { box-sizing: border-box; }
html, body { height: 100%; }
body {
  margin: 0;
  display: flex;
  align-items: center;
  justify-content: center;
  padding: 32px 16px;
  font-family: ui-sans-serif, system-ui, -apple-system, BlinkMacSystemFont, "Segoe UI", Roboto, "Helvetica Neue", Arial, "Noto Sans", "PingFang SC", "Microsoft YaHei", sans-serif;
  color: var(--foreground);
  background: var(--background);
  -webkit-font-smoothing: antialiased;
}
main {
  width: 100%;
  max-width: 360px;
  text-align: center;
  animation: enter 0.3s ease-out both;
}
@keyframes enter {
  from { opacity: 0; transform: translateY(4px); }
  to { opacity: 1; transform: none; }
}
.logo { display: block; height: 24px; width: auto; margin: 0 auto; }
.logo--dark { display: none; }
@media (prefers-color-scheme: dark) {
  .logo--light { display: none; }
  .logo--dark { display: block; }
}
.status {
  margin-top: 20px;
  padding: 0 8px;
}
h1 {
  margin: 0;
  font-size: 16px;
  line-height: 24px;
  font-weight: 600;
}
.lead {
  margin: 6px 0 0;
  font-size: 14px;
  line-height: 20px;
  color: var(--muted-foreground);
}
@media (prefers-reduced-motion: reduce) {
  main { animation: none; }
}
"#;

const PAGE_SCRIPT: &str = r#"(function () {
  // The one-time grant is already in the app; keep it out of the address bar and history.
  try { window.history.replaceState(null, "", window.location.pathname); } catch (e) {}
  var chinese = /^zh\b/i.test(navigator.language || "");
  if (chinese) document.documentElement.lang = "zh-CN";
  var nodes = document.querySelectorAll("[data-en]");
  for (var i = 0; i < nodes.length; i += 1) {
    var node = nodes[i];
    node.textContent = chinese ? node.getAttribute("data-zh") : node.getAttribute("data-en");
  }
})();"#;

/// Percent-encode an SVG into a `data:` URI. Inlining the SVG markup instead would
/// clash, because both logo files reuse the same gradient ids.
fn svg_data_uri(svg: &str) -> String {
    let mut out = String::with_capacity(svg.len() * 2 + 24);
    out.push_str("data:image/svg+xml,");
    for byte in svg.trim().bytes() {
        if byte.is_ascii_alphanumeric() || b"-_.~".contains(&byte) {
            out.push(byte as char);
        } else {
            out.push_str(&format!("%{byte:02X}"));
        }
    }
    out
}

/// Copy for one loopback outcome, in the two languages the page can render.
struct PageCopy {
    status: u16,
    reason: &'static str,
    title_en: &'static str,
    title_zh: &'static str,
    lead_en: &'static str,
    lead_zh: &'static str,
}

impl PageCopy {
    fn render(&self) -> String {
        let logo_light = svg_data_uri(LOGO_LIGHT);
        let logo_dark = svg_data_uri(LOGO_DARK);
        let mut html =
            String::with_capacity(PAGE_STYLE.len() + logo_light.len() + logo_dark.len() + 2048);
        html.push_str("<!doctype html>\n<html lang=\"en\">\n<head>\n");
        html.push_str("<meta charset=\"utf-8\">\n");
        html.push_str("<meta name=\"viewport\" content=\"width=device-width, initial-scale=1\">\n");
        html.push_str("<meta name=\"color-scheme\" content=\"light dark\">\n");
        html.push_str("<meta name=\"robots\" content=\"noindex\">\n");
        html.push_str("<title>DEEIX Chat</title>\n");
        html.push_str("<style>");
        html.push_str(PAGE_STYLE);
        html.push_str("</style>\n</head>\n<body>\n<main>\n");
        html.push_str(&format!(
            "<img class=\"logo logo--light\" src=\"{logo_light}\" alt=\"DEEIX Chat\">\n\
             <img class=\"logo logo--dark\" src=\"{logo_dark}\" alt=\"DEEIX Chat\">\n"
        ));
        html.push_str("<div class=\"status\">\n");
        html.push_str(&format!(
            "<h1 data-en=\"{}\" data-zh=\"{}\">{}</h1>\n",
            self.title_en, self.title_zh, self.title_en
        ));
        html.push_str(&format!(
            "<p class=\"lead\" data-en=\"{}\" data-zh=\"{}\">{}</p>\n",
            self.lead_en, self.lead_zh, self.lead_en
        ));
        html.push_str("</div>\n</main>\n<script>");
        html.push_str(PAGE_SCRIPT);
        html.push_str("</script>\n</body>\n</html>");
        html
    }
}

fn page_copy(status: u16) -> PageCopy {
    match status {
        200 => PageCopy {
            status: 200,
            reason: "OK",
            title_en: "Sign-in complete",
            title_zh: "登录完成",
            lead_en: "You can close this tab and return to DEEIX Chat.",
            lead_zh: "你可以关闭此标签页并返回 DEEIX Chat。",
        },
        400 => PageCopy {
            status: 400,
            reason: "Bad Request",
            title_en: "Sign-in could not be completed",
            title_zh: "无法完成登录",
            lead_en:
                "The response from the provider was not valid. Return to DEEIX Chat and try again.",
            lead_zh: "第三方返回的响应无效，请返回 DEEIX Chat 重试。",
        },
        _ => PageCopy {
            status: 404,
            reason: "Not Found",
            title_en: "Address not found",
            title_zh: "地址不存在",
            lead_en: "This address is not part of the sign-in flow. Return to DEEIX Chat.",
            lead_zh: "该地址不属于登录流程，请返回 DEEIX Chat。",
        },
    }
}

fn respond(mut stream: TcpStream, status: u16) -> std::io::Result<()> {
    let page = page_copy(status);
    let html = page.render();
    write!(
        stream,
        "HTTP/1.1 {} {}\r\nContent-Type: text/html; charset=utf-8\r\n\
         Content-Length: {}\r\nConnection: close\r\nCache-Control: no-store\r\n\
         Content-Security-Policy: default-src 'none'; img-src data:; \
         style-src 'unsafe-inline'; script-src 'unsafe-inline'\r\nX-Content-Type-Options: nosniff\r\n\
         Referrer-Policy: no-referrer\r\n\r\n{}",
        page.status,
        page.reason,
        html.len(),
        html
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

    /// Send one response through `respond` and return the raw HTTP payload.
    fn exchange(status: u16) -> String {
        let listener = TcpListener::bind((Ipv4Addr::LOCALHOST, 0)).unwrap();
        let addr = listener.local_addr().unwrap();
        let client = std::thread::spawn(move || {
            let mut s = TcpStream::connect(addr).unwrap();
            let mut out = String::new();
            s.read_to_string(&mut out).unwrap();
            out
        });
        let (stream, _) = listener.accept().unwrap();
        respond(stream, status).unwrap();
        client.join().unwrap()
    }

    #[test]
    fn response_is_well_formed() {
        let out = exchange(200);
        assert!(out.starts_with("HTTP/1.1 200 OK\r\n"));
        assert!(out.contains("Content-Type: text/html; charset=utf-8"));
        assert!(out.contains("Cache-Control: no-store"));
        assert!(out.contains("X-Content-Type-Options: nosniff"));
        assert!(
            out.contains("img-src data:;"),
            "the embedded logos need data: images"
        );
        assert!(out.ends_with("</html>"));
    }

    #[test]
    fn content_length_counts_body_bytes() {
        let out = exchange(200);
        let (head, body) = out.split_once("\r\n\r\n").unwrap();
        let declared: usize = head
            .lines()
            .find_map(|line| line.strip_prefix("Content-Length: "))
            .expect("a Content-Length header")
            .parse()
            .expect("a numeric Content-Length");
        assert_eq!(declared, body.len(), "the header must count UTF-8 bytes");
    }

    #[test]
    fn page_is_bilingual_and_self_contained() {
        let html = page_copy(200).render();
        assert!(html.contains("<title>DEEIX Chat</title>"));
        // No close button: browsers refuse window.close() on a tab the script did not open.
        assert!(!html.contains("<button"));
        assert!(html.contains("data-en=\"Sign-in complete\""));
        assert!(html.contains("data-zh=\"登录完成\""));
        // The listener serves no assets, so the page must not reference any.
        assert!(!html.contains("http://"));
        assert!(!html.contains("https://"));
        // The grant must not linger in the address bar or browser history.
        assert!(html.contains("history.replaceState"));
        // Same mark as the app, embedded so the page stays self-contained.
        assert!(html.contains("class=\"logo logo--light\" src=\"data:image/svg+xml,"));
        assert!(html.contains("class=\"logo logo--dark\" src=\"data:image/svg+xml,"));
    }

    #[test]
    fn unknown_status_falls_back_to_not_found() {
        let copy = page_copy(500);
        assert_eq!(copy.status, 404);
        assert_eq!(copy.reason, "Not Found");
        assert!(exchange(404).starts_with("HTTP/1.1 404 Not Found\r\n"));
        assert!(exchange(400).starts_with("HTTP/1.1 400 Bad Request\r\n"));
    }
}
