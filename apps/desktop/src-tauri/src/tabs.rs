// Browser-style tabs: one window, a "chrome" webview for the tab strip and one
// content webview per tab. A tab is bound to at most one server and a server
// is shown by at most one tab, so each tab is an isolated instance of the web
// app. The strip (/desktop/tabs) drives this module through the `tabs_*`
// commands and the `tabs:changed` event.

use std::fs;
use std::path::PathBuf;
use std::sync::Mutex;
use std::time::{Duration, Instant};

use serde::{Deserialize, Serialize};
use tauri::{
    AppHandle, Emitter, EventTarget, LogicalPosition, LogicalSize, Manager, Runtime,
    WebviewBuilder, WebviewUrl, Window, WindowEvent,
};
use tauri_plugin_opener::OpenerExt;

use crate::distribution::Distribution;
use crate::paths;
use crate::session::{self, ServerMode};
use crate::sidecar;

pub const WINDOW_LABEL: &str = "main";
pub const CHROME_LABEL: &str = "chrome";
const CHROME_URL: &str = "desktop/tabs";
/// Not `/`: the static export renders it as a redirect payload only.
const CONTENT_URL: &str = "/chat";
const TABS_FILE: &str = "tabs.json";
const CHANGED_EVENT: &str = "tabs:changed";
/// Must match the strip's CSS; on macOS also the compact title bar height.
const STRIP_HEIGHT: f64 = 38.0;
/// A tab hidden this long has its webview discarded; it reloads on activation.
const DISCARD_AFTER: Duration = Duration::from_secs(30 * 60);
const DISCARD_SWEEP: Duration = Duration::from_secs(60);

#[derive(Debug, Clone, PartialEq, Eq, Serialize, Deserialize)]
pub struct Server {
    pub mode: ServerMode,
    #[serde(default)]
    pub origin: String,
}

impl Server {
    pub fn local() -> Self {
        Self {
            mode: ServerMode::Local,
            origin: String::new(),
        }
    }
    pub fn remote(origin: String) -> Self {
        Self {
            mode: ServerMode::Remote,
            origin,
        }
    }
    /// Keychain account. Local is a constant because the sidecar port changes per launch.
    pub fn keychain_key(&self) -> String {
        match self.mode {
            ServerMode::Local => "local".to_string(),
            ServerMode::Remote => self.origin.clone(),
        }
    }
}

#[derive(Debug, Clone, Serialize)]
#[serde(rename_all = "camelCase")]
pub struct Tab {
    /// Webview label; stable for the tab's lifetime.
    pub id: String,
    /// None until the user picks a server on the setup screen.
    pub server: Option<Server>,
    /// Document title reported by the page; the strip falls back to the server.
    #[serde(default)]
    pub title: String,
    /// When the tab was last hidden; None while active. Drives discarding.
    #[serde(skip)]
    hidden_since: Option<Instant>,
}

#[derive(Debug, Clone, Serialize)]
#[serde(rename_all = "camelCase")]
pub struct TabsSnapshot {
    pub tabs: Vec<Tab>,
    pub active: Option<String>,
    /// "macos" | "windows" | "linux" — the strip lays out around native chrome.
    pub platform: &'static str,
}

/// On-disk form: only bound tabs survive a restart.
#[derive(Serialize, Deserialize, Default)]
struct TabsFile {
    servers: Vec<Server>,
    #[serde(default)]
    active: usize,
}

#[derive(Default)]
struct Inner {
    tabs: Vec<Tab>,
    active: Option<String>,
    next_id: u64,
}

// Pure bookkeeping, kept free of Tauri handles so it can be unit-tested.
impl Inner {
    fn allocate_id(&mut self) -> String {
        self.next_id += 1;
        format!("tab-{}", self.next_id)
    }

    fn find_by_server(&self, server: &Server) -> Option<&Tab> {
        self.tabs.iter().find(|t| t.server.as_ref() == Some(server))
    }

    /// Bind `server` to tab `id`. A server may only be shown by one tab.
    fn bind(&mut self, id: &str, server: Server) -> Result<()> {
        if let Some(other) = self.find_by_server(&server) {
            if other.id != id {
                return Err(TabsError(
                    "that server is already open in another tab".into(),
                ));
            }
        }
        let tab = self
            .tabs
            .iter_mut()
            .find(|t| t.id == id)
            .ok_or_else(|| TabsError(format!("unknown tab {id}")))?;
        if tab.server.is_some() && tab.server.as_ref() != Some(&server) {
            return Err(TabsError("tab is already bound to a server".into()));
        }
        tab.server = Some(server);
        Ok(())
    }

    /// Remove tab `id`; returns it and the next active tab (left neighbour).
    fn remove(&mut self, id: &str) -> Result<(Tab, Option<String>)> {
        let index = self
            .tabs
            .iter()
            .position(|t| t.id == id)
            .ok_or_else(|| TabsError(format!("unknown tab {id}")))?;
        let removed = self.tabs.remove(index);
        let next_active = if self.active.as_deref() == Some(id) {
            self.tabs
                .get(index.saturating_sub(1))
                .or(self.tabs.first())
                .map(|t| t.id.clone())
        } else {
            self.active.clone()
        };
        Ok((removed, next_active))
    }

    /// Move tab `id` to position `index` (clamped). Order is what the strip shows and what is persisted.
    fn move_to(&mut self, id: &str, index: usize) -> Result<()> {
        let from = self
            .tabs
            .iter()
            .position(|t| t.id == id)
            .ok_or_else(|| TabsError(format!("unknown tab {id}")))?;
        let tab = self.tabs.remove(from);
        let to = index.min(self.tabs.len());
        self.tabs.insert(to, tab);
        Ok(())
    }

    /// On-disk form: bound tabs in order plus which of them is active.
    fn to_file(&self) -> TabsFile {
        let bound: Vec<&Tab> = self.tabs.iter().filter(|t| t.server.is_some()).collect();
        let active = bound
            .iter()
            .position(|t| Some(&t.id) == self.active.as_ref())
            .unwrap_or(0);
        TabsFile {
            servers: bound.iter().filter_map(|t| t.server.clone()).collect(),
            active,
        }
    }
}

#[derive(Default)]
pub struct TabsState {
    inner: Mutex<Inner>,
    /// Serializes webview create/show/hide. Never lock from the main thread.
    webviews: Mutex<()>,
}

#[derive(Debug)]
pub struct TabsError(pub String);

impl<E: std::fmt::Display> From<E> for TabsError {
    fn from(e: E) -> Self {
        Self(e.to_string())
    }
}

type Result<T> = std::result::Result<T, TabsError>;

// ---------- window + layout ----------

/// Create the main window with the tab strip and restore the saved tabs.
pub fn init<R: Runtime>(app: &AppHandle<R>) -> Result<()> {
    let config = tauri::utils::config::WindowConfig {
        label: WINDOW_LABEL.into(),
        title: "DEEIX Chat".into(),
        width: 1280.0,
        height: 840.0,
        min_width: Some(960.0),
        min_height: Some(640.0),
        center: true,
        // Windows draws its own caption buttons in the strip (README "Tabs").
        decorations: !cfg!(target_os = "windows"),
        title_bar_style: if cfg!(target_os = "macos") {
            tauri::TitleBarStyle::Overlay
        } else {
            tauri::TitleBarStyle::Visible
        },
        hidden_title: cfg!(target_os = "macos"),
        ..Default::default()
    };
    let window = tauri::window::WindowBuilder::from_config(app, &config)?.build()?;
    #[cfg(target_os = "macos")]
    {
        disable_titlebar_drag(&window)?;
        lower_traffic_lights(&window)?;
    }

    let (width, _) = logical_size(&window)?;
    let _strip = window.add_child(
        content_webview(
            app,
            WebviewBuilder::new(CHROME_LABEL, WebviewUrl::App(CHROME_URL.into())),
        ),
        LogicalPosition::new(0.0, 0.0),
        LogicalSize::new(width, STRIP_HEIGHT),
    )?;
    #[cfg(target_os = "linux")]
    pin_strip_height(&_strip)?;

    let event_window = window.clone();
    window.on_window_event(move |event| match event {
        WindowEvent::Resized(_) => {
            let _ = relayout(&event_window);
        }
        // Close hides; the tray quits. Not on Linux, where a tray is not guaranteed.
        #[cfg(not(target_os = "linux"))]
        WindowEvent::CloseRequested { api, .. } => {
            api.prevent_close();
            let _ = event_window.hide();
        }
        _ => {}
    });

    let saved = read_file(app)?;
    if saved.servers.is_empty() {
        open(app, None)?;
    } else {
        // Only the active tab gets a webview now; the rest load on first click.
        let active_index = saved.active.min(saved.servers.len() - 1);
        let ids: Vec<String> = saved
            .servers
            .into_iter()
            .map(|server| insert(app, Some(server)))
            .collect();
        activate(app, &ids[active_index], true)?;
    }

    let sweep_app = app.clone();
    tauri::async_runtime::spawn(async move {
        loop {
            tokio::time::sleep(DISCARD_SWEEP).await;
            discard_stale(&sweep_app);
        }
    });
    Ok(())
}

/// Register a tab in state without creating its webview.
fn insert<R: Runtime>(app: &AppHandle<R>, server: Option<Server>) -> String {
    let state = app.state::<TabsState>();
    let mut inner = state.inner.lock().unwrap_or_else(|e| e.into_inner());
    let id = inner.allocate_id();
    eprintln!(
        "[tabs] open {id} server={:?}",
        server.as_ref().map(|s| s.mode)
    );
    inner.tabs.push(Tab {
        id: id.clone(),
        server,
        title: String::new(),
        hidden_since: Some(Instant::now()),
    });
    id
}

/// Create the webview for a tab that does not have one yet.
fn materialize<R: Runtime>(app: &AppHandle<R>, window: &Window<R>, id: &str) -> Result<()> {
    if window.webviews().iter().any(|w| w.label() == id) {
        return Ok(());
    }
    let (width, height) = logical_size(window)?;
    let title_app = app.clone();
    let title_id = id.to_string();
    let webview = window.add_child(
        content_webview(
            app,
            WebviewBuilder::new(id, WebviewUrl::App(CONTENT_URL.into())),
        )
        .on_document_title_changed(move |_, title| set_title(&title_app, &title_id, title)),
        LogicalPosition::new(0.0, STRIP_HEIGHT),
        LogicalSize::new(width, (height - STRIP_HEIGHT).max(0.0)),
    )?;
    webview.hide()?;
    Ok(())
}

/// Drop webviews of tabs that have been hidden for longer than DISCARD_AFTER.
fn discard_stale<R: Runtime>(app: &AppHandle<R>) {
    let Some(window) = app.get_window(WINDOW_LABEL) else {
        return;
    };
    let stale: Vec<String> = {
        let state = app.state::<TabsState>();
        let inner = state.inner.lock().unwrap_or_else(|e| e.into_inner());
        inner
            .tabs
            .iter()
            .filter(|t| Some(&t.id) != inner.active.as_ref())
            .filter(|t| {
                t.hidden_since
                    .is_some_and(|since| since.elapsed() >= DISCARD_AFTER)
            })
            .map(|t| t.id.clone())
            .collect()
    };
    for webview in window.webviews() {
        if stale.iter().any(|id| id == webview.label()) {
            eprintln!(
                "[tabs] discard {} (hidden for {:?})",
                webview.label(),
                DISCARD_AFTER
            );
            let _ = webview.close();
        }
    }
}

/// Webviews may only navigate within the app; anything else opens in the browser.
/// Every webview (strip and tabs) shares one profile: Tauri's default when
/// installed, `data/webview` when portable. WebView2 requires webviews in one
/// process that share a user data folder to share environment options, which
/// they do since all are created here.
fn content_webview<R: Runtime>(
    app: &AppHandle<R>,
    builder: WebviewBuilder<R>,
) -> WebviewBuilder<R> {
    let open_app = app.clone();
    let builder = match paths::webview_dir(app) {
        Some(dir) => builder.data_directory(dir),
        None => builder,
    };
    builder
        .on_navigation(is_app_url)
        .on_new_window(move |url, _| {
            if matches!(url.scheme(), "http" | "https") {
                let _ = open_app.opener().open_url(url.as_str(), None::<&str>);
            }
            tauri::webview::NewWindowResponse::Deny
        })
}

fn is_app_url(url: &tauri::Url) -> bool {
    match url.scheme() {
        "tauri" => true,
        "http" | "https" => {
            let host = url.host_str().unwrap_or_default();
            // Windows/Android serve the app from http://tauri.localhost; dev from the Next server.
            host == "tauri.localhost"
                || (cfg!(debug_assertions) && (host == "localhost" || host == "127.0.0.1"))
        }
        "about" | "blob" => true,
        _ => false,
    }
}

fn set_title<R: Runtime>(app: &AppHandle<R>, id: &str, title: String) {
    {
        let state = app.state::<TabsState>();
        let mut inner = state.inner.lock().unwrap_or_else(|e| e.into_inner());
        match inner.tabs.iter_mut().find(|t| t.id == id) {
            Some(tab) if tab.title != title => tab.title = title,
            _ => return,
        }
    }
    let _ = app.emit_to(
        EventTarget::webview(CHROME_LABEL),
        CHANGED_EVENT,
        snapshot(app),
    );
}

/// Stops AppKit's title-band drag from stealing pointer events from the strip.
#[cfg(target_os = "macos")]
fn disable_titlebar_drag<R: Runtime>(window: &Window<R>) -> Result<()> {
    use objc2::msg_send;
    use objc2::runtime::AnyObject;
    let ns_window = window.ns_window()? as *mut AnyObject;
    unsafe {
        let _: () = msg_send![ns_window, setMovable: false];
    }
    Ok(())
}

/// An empty compact toolbar makes AppKit centre the traffic lights on the strip.
#[cfg(target_os = "macos")]
fn lower_traffic_lights<R: Runtime>(window: &Window<R>) -> Result<()> {
    use objc2::msg_send;
    use objc2::runtime::AnyObject;
    use objc2_foundation::NSString;

    let ns_window = window.ns_window()? as *mut AnyObject;
    unsafe {
        let identifier = NSString::from_str("com.deeix.chat.desktop.tabstrip");
        let toolbar: *mut AnyObject = msg_send![objc2::class!(NSToolbar), alloc];
        let toolbar: *mut AnyObject = msg_send![toolbar, initWithIdentifier: &*identifier];
        let _: () = msg_send![toolbar, setShowsBaselineSeparator: false];
        // NSWindowToolbarStyleUnifiedCompact = 4
        let _: () = msg_send![ns_window, setToolbarStyle: 4isize];
        let _: () = msg_send![ns_window, setToolbar: toolbar];
    }
    Ok(())
}

fn logical_size<R: Runtime>(window: &Window<R>) -> Result<(f64, f64)> {
    let size = window
        .inner_size()?
        .to_logical::<f64>(window.scale_factor()?);
    Ok((size.width, size.height))
}

/// Linux packs child webviews into the window's GtkBox instead of placing them:
/// position and size requests are ignored and every visible webview gets an
/// equal share of the height, so the strip would take half the window. Pin it
/// to its height and let the active tab (packed after it) take the rest.
#[cfg(target_os = "linux")]
fn pin_strip_height<R: Runtime>(strip: &tauri::Webview<R>) -> Result<()> {
    use gtk::prelude::*;
    strip.with_webview(|platform| {
        let webview = platform.inner();
        webview.set_size_request(-1, STRIP_HEIGHT as i32);
        if let Some(parent) = webview.parent().and_then(|p| p.downcast::<gtk::Box>().ok()) {
            parent.set_child_packing(&webview, false, false, 0, gtk::PackType::Start);
        }
    })?;
    Ok(())
}

fn relayout<R: Runtime>(window: &Window<R>) -> Result<()> {
    let (width, height) = logical_size(window)?;
    for webview in window.webviews() {
        if webview.label() == CHROME_LABEL {
            webview.set_position(LogicalPosition::new(0.0, 0.0))?;
            webview.set_size(LogicalSize::new(width, STRIP_HEIGHT))?;
        } else {
            webview.set_position(LogicalPosition::new(0.0, STRIP_HEIGHT))?;
            webview.set_size(LogicalSize::new(width, (height - STRIP_HEIGHT).max(0.0)))?;
        }
    }
    Ok(())
}

// ---------- persistence ----------

fn file_path<R: Runtime>(app: &AppHandle<R>) -> Result<PathBuf> {
    let dir = paths::config_dir(app);
    fs::create_dir_all(&dir)?;
    Ok(dir.join(TABS_FILE))
}

fn read_file<R: Runtime>(app: &AppHandle<R>) -> Result<TabsFile> {
    match fs::read(file_path(app)?) {
        Ok(bytes) => {
            let mut file: TabsFile = match serde_json::from_slice(&bytes) {
                Ok(file) => file,
                Err(e) => {
                    eprintln!("[tabs] ignoring unreadable {TABS_FILE}: {e}");
                    return Ok(TabsFile::default());
                }
            };
            // Drop malformed remote entries rather than failing the whole launch.
            file.servers.retain(|s| match s.mode {
                ServerMode::Local => true,
                ServerMode::Remote => {
                    session::normalize_origin(&s.origin).as_deref() == Some(s.origin.as_str())
                }
            });
            file.servers.dedup();
            // A policy set since the last run (address locked, local mode
            // disabled) closes the tabs it no longer allows, credentials included.
            let policy = app.state::<Distribution>().policy.clone();
            file.servers.retain(|s| {
                if policy.allows(s) {
                    return true;
                }
                eprintln!(
                    "[tabs] not restoring {:?} {}: blocked by policy",
                    s.mode, s.origin
                );
                let _ = session::forget(app, s);
                false
            });
            Ok(file)
        }
        Err(e) if e.kind() == std::io::ErrorKind::NotFound => Ok(TabsFile::default()),
        Err(e) => Err(e.into()),
    }
}

fn persist<R: Runtime>(app: &AppHandle<R>) -> Result<()> {
    let file = {
        let state = app.state::<TabsState>();
        let inner = state.inner.lock().unwrap_or_else(|e| e.into_inner());
        inner.to_file()
    };
    let path = file_path(app)?;
    let tmp = path.with_extension("json.tmp");
    fs::write(&tmp, serde_json::to_vec(&file)?)?;
    fs::rename(tmp, path)?;
    Ok(())
}

fn changed<R: Runtime>(app: &AppHandle<R>) -> Result<()> {
    persist(app)?;
    app.emit_to(
        EventTarget::webview(CHROME_LABEL),
        CHANGED_EVENT,
        snapshot(app),
    )?;
    Ok(())
}

// ---------- state ----------

pub fn snapshot<R: Runtime>(app: &AppHandle<R>) -> TabsSnapshot {
    let state = app.state::<TabsState>();
    let inner = state.inner.lock().unwrap_or_else(|e| e.into_inner());
    TabsSnapshot {
        tabs: inner.tabs.clone(),
        active: inner.active.clone(),
        platform: std::env::consts::OS,
    }
}

/// Server bound to the tab that owns `label`, if any.
pub fn server_of<R: Runtime>(app: &AppHandle<R>, label: &str) -> Option<Server> {
    let state = app.state::<TabsState>();
    let inner = state.inner.lock().unwrap_or_else(|e| e.into_inner());
    inner
        .tabs
        .iter()
        .find(|t| t.id == label)
        .and_then(|t| t.server.clone())
}

fn find_by_server<R: Runtime>(app: &AppHandle<R>, server: &Server) -> Option<String> {
    let state = app.state::<TabsState>();
    let inner = state.inner.lock().unwrap_or_else(|e| e.into_inner());
    inner.find_by_server(server).map(|t| t.id.clone())
}

fn references<R: Runtime>(app: &AppHandle<R>, server: &Server) -> bool {
    find_by_server(app, server).is_some()
}

/// Open a tab. With a server that is already open, activates that tab instead.
pub fn open<R: Runtime>(app: &AppHandle<R>, server: Option<Server>) -> Result<Tab> {
    if let Some(server) = server.as_ref() {
        if let Some(id) = find_by_server(app, server) {
            activate(app, &id, true)?;
            let state = app.state::<TabsState>();
            let inner = state.inner.lock().unwrap_or_else(|e| e.into_inner());
            return Ok(inner
                .tabs
                .iter()
                .find(|t| t.id == id)
                .cloned()
                .expect("tab exists"));
        }
    }

    let id = insert(app, server);
    activate(app, &id, true)?;
    let state = app.state::<TabsState>();
    let inner = state.inner.lock().unwrap_or_else(|e| e.into_inner());
    Ok(inner
        .tabs
        .iter()
        .find(|t| t.id == id)
        .cloned()
        .expect("tab exists"))
}

/// Show tab `id`. `focus` is false mid-press: moving focus then kills the drag.
pub fn activate<R: Runtime>(app: &AppHandle<R>, id: &str, focus: bool) -> Result<()> {
    let window = app
        .get_window(WINDOW_LABEL)
        .ok_or_else(|| TabsError("main window missing".into()))?;
    {
        let state = app.state::<TabsState>();
        let mut inner = state.inner.lock().unwrap_or_else(|e| e.into_inner());
        if !inner.tabs.iter().any(|t| t.id == id) {
            return Err(TabsError(format!("unknown tab {id}")));
        }
        inner.active = Some(id.to_string());
        let now = Instant::now();
        for tab in &mut inner.tabs {
            tab.hidden_since = if tab.id == id {
                None
            } else {
                tab.hidden_since.or(Some(now))
            };
        }
    }
    {
        let state = app.state::<TabsState>();
        let _guard = state.webviews.lock().unwrap_or_else(|e| e.into_inner());
        materialize(app, &window, id)?;
        for webview in window.webviews() {
            let label = webview.label();
            if label == CHROME_LABEL {
                continue;
            }
            if label == id {
                webview.show()?;
                if focus {
                    let _ = webview.set_focus();
                }
            } else {
                webview.hide()?;
            }
        }
    }
    changed(app)
}

/// Bind a server to a not-yet-bound tab (the setup screen's choice).
pub fn bind<R: Runtime>(app: &AppHandle<R>, id: &str, server: Server) -> Result<()> {
    {
        let state = app.state::<TabsState>();
        let mut inner = state.inner.lock().unwrap_or_else(|e| e.into_inner());
        inner.bind(id, server)?;
    }
    changed(app)
}

/// Detach a tab from its server; it returns to the setup screen.
pub async fn unbind<R: Runtime>(app: &AppHandle<R>, id: &str) -> Result<()> {
    let removed = {
        let state = app.state::<TabsState>();
        let mut inner = state.inner.lock().unwrap_or_else(|e| e.into_inner());
        let tab = inner
            .tabs
            .iter_mut()
            .find(|t| t.id == id)
            .ok_or_else(|| TabsError(format!("unknown tab {id}")))?;
        tab.server.take()
    };
    if let Some(server) = removed {
        forget_server(app, &server).await?;
    }
    changed(app)
}

/// Delete a server's credential and stop the sidecar if nothing else uses it.
async fn forget_server<R: Runtime>(app: &AppHandle<R>, server: &Server) -> Result<()> {
    if references(app, server) {
        return Ok(());
    }
    session::forget(app, server).map_err(|e| TabsError(e.message))?;
    if server.mode == ServerMode::Local {
        sidecar::stop(app).await;
    }
    Ok(())
}

/// Close a tab and forget its server; the last tab is replaced by an empty one.
pub async fn close<R: Runtime>(app: &AppHandle<R>, id: &str) -> Result<()> {
    let window = app
        .get_window(WINDOW_LABEL)
        .ok_or_else(|| TabsError("main window missing".into()))?;
    let (removed, next_active) = {
        let state = app.state::<TabsState>();
        let mut inner = state.inner.lock().unwrap_or_else(|e| e.into_inner());
        inner.remove(id)?
    };

    eprintln!("[tabs] close {id}");
    if let Some(webview) = window.webviews().into_iter().find(|w| w.label() == id) {
        webview.close()?;
    }
    if let Some(server) = removed.server {
        forget_server(app, &server).await?;
    }

    match next_active {
        Some(next) => activate(app, &next, true),
        None => open(app, None).map(|_| ()),
    }
}

pub fn move_to<R: Runtime>(app: &AppHandle<R>, id: &str, index: usize) -> Result<()> {
    {
        let state = app.state::<TabsState>();
        let mut inner = state.inner.lock().unwrap_or_else(|e| e.into_inner());
        inner.move_to(id, index)?;
    }
    changed(app)
}

// ---------- commands (tab strip) ----------

#[tauri::command]
pub fn tabs_list<R: Runtime>(app: AppHandle<R>) -> TabsSnapshot {
    snapshot(&app)
}

// Async: `add_child` from a sync command re-enters WebView2 and freezes.
#[tauri::command]
pub async fn tabs_open<R: Runtime>(app: AppHandle<R>) -> std::result::Result<Tab, String> {
    open(&app, None).map_err(|e| e.0)
}

#[tauri::command]
pub async fn tabs_activate<R: Runtime>(
    app: AppHandle<R>,
    id: String,
    focus: bool,
) -> std::result::Result<(), String> {
    activate(&app, &id, focus).map_err(|e| e.0)
}

#[tauri::command]
pub fn tabs_move<R: Runtime>(
    app: AppHandle<R>,
    id: String,
    index: usize,
) -> std::result::Result<(), String> {
    move_to(&app, &id, index).map_err(|e| e.0)
}

#[tauri::command]
pub async fn tabs_close<R: Runtime>(
    app: AppHandle<R>,
    id: String,
) -> std::result::Result<(), String> {
    close(&app, &id).await.map_err(|e| e.0)
}

#[cfg(test)]
mod tests {
    use super::*;

    fn inner_with(servers: &[Option<Server>]) -> Inner {
        let mut inner = Inner::default();
        for server in servers {
            let id = inner.allocate_id();
            inner.tabs.push(Tab {
                id,
                server: server.clone(),
                title: String::new(),
                hidden_since: None,
            });
        }
        inner.active = inner.tabs.last().map(|t| t.id.clone());
        inner
    }

    #[test]
    fn a_server_is_shown_by_at_most_one_tab() {
        let mut inner = inner_with(&[Some(Server::local()), None]);
        assert!(inner.bind("tab-2", Server::local()).is_err());
        assert!(inner
            .bind("tab-2", Server::remote("https://a.example".into()))
            .is_ok());
        // Rebinding to the same server is idempotent; to a different one is refused.
        assert!(inner
            .bind("tab-2", Server::remote("https://a.example".into()))
            .is_ok());
        assert!(inner
            .bind("tab-2", Server::remote("https://b.example".into()))
            .is_err());
        assert!(inner.bind("tab-9", Server::local()).is_err());
    }

    #[test]
    fn closing_the_active_tab_activates_its_left_neighbour() {
        let mut inner = inner_with(&[None, None, None]);
        inner.active = Some("tab-2".into());
        let (removed, next) = inner.remove("tab-2").unwrap();
        assert_eq!(removed.id, "tab-2");
        assert_eq!(next.as_deref(), Some("tab-1"));

        // Closing the first tab falls forward to the new first tab.
        inner.active = Some("tab-1".into());
        let (_, next) = inner.remove("tab-1").unwrap();
        assert_eq!(next.as_deref(), Some("tab-3"));

        // Closing the last remaining tab leaves nothing to activate.
        inner.active = Some("tab-3".into());
        let (_, next) = inner.remove("tab-3").unwrap();
        assert_eq!(next, None);
    }

    #[test]
    fn closing_an_inactive_tab_keeps_the_active_one() {
        let mut inner = inner_with(&[None, None]);
        inner.active = Some("tab-2".into());
        let (_, next) = inner.remove("tab-1").unwrap();
        assert_eq!(next.as_deref(), Some("tab-2"));
        assert!(inner.remove("tab-1").is_err());
    }

    #[test]
    fn moving_reorders_and_clamps() {
        let mut inner = inner_with(&[None, None, None]);
        inner.move_to("tab-1", 2).unwrap();
        assert_eq!(
            inner.tabs.iter().map(|t| t.id.as_str()).collect::<Vec<_>>(),
            ["tab-2", "tab-3", "tab-1"]
        );
        inner.move_to("tab-1", 0).unwrap();
        assert_eq!(
            inner.tabs.iter().map(|t| t.id.as_str()).collect::<Vec<_>>(),
            ["tab-1", "tab-2", "tab-3"]
        );
        inner.move_to("tab-2", 99).unwrap();
        assert_eq!(inner.tabs.last().unwrap().id, "tab-2");
        assert!(inner.move_to("tab-9", 0).is_err());
    }

    #[test]
    fn only_bound_tabs_are_persisted_and_active_index_follows_them() {
        let mut inner = inner_with(&[
            None,
            Some(Server::local()),
            None,
            Some(Server::remote("https://a.example".into())),
        ]);
        inner.active = Some("tab-4".into());
        let file = inner.to_file();
        assert_eq!(file.servers.len(), 2);
        assert_eq!(file.active, 1);

        // An unbound active tab is not persisted; the first bound tab is restored active.
        inner.active = Some("tab-3".into());
        assert_eq!(inner.to_file().active, 0);
    }
}
