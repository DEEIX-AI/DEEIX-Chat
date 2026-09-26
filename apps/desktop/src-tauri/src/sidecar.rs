// Local-mode sidecar: the Go server as a child process on the loopback
// interface, with SQLite and local storage under the app data directory.
// Handshake: one JSON line on stdout with the origin and a one-time login
// grant (see backend/internal/cli); the grant is redeemed by
// `session::local_sign_in` and never reaches the webview.

use std::path::PathBuf;
use std::time::Duration;

use serde::Deserialize;
use tauri::async_runtime::Mutex;
use tauri::{AppHandle, Manager, Runtime};
use tauri_plugin_shell::process::{CommandChild, CommandEvent};
use tauri_plugin_shell::ShellExt;

/// File name of the bundled Go binary, resolved next to the app executable.
/// Tauri strips the `binaries/` directory of `bundle.externalBin` when copying.
const SIDECAR_PROGRAM: &str = "deeix-chat-server";
const READY_TIMEOUT: Duration = Duration::from_secs(30);
const LOCAL_DATA_DIR: &str = "local";

#[derive(Debug, Clone, Deserialize)]
pub struct Handoff {
    #[serde(rename = "type")]
    pub kind: String,
    pub version: String,
    pub origin: String,
    pub grant: String,
    pub pid: u32,
}

#[derive(Default)]
pub struct SidecarState {
    inner: Mutex<Option<Running>>,
}

struct Running {
    child: CommandChild,
    origin: String,
    /// Consumed on first use; `None` afterwards.
    grant: Option<String>,
}

#[derive(Debug)]
pub struct SidecarError(pub String);

impl<E: std::fmt::Display> From<E> for SidecarError {
    fn from(e: E) -> Self {
        Self(e.to_string())
    }
}

pub fn data_dir<R: Runtime>(app: &AppHandle<R>) -> Result<PathBuf, SidecarError> {
    let dir = app.path().app_data_dir()?.join(LOCAL_DATA_DIR);
    std::fs::create_dir_all(&dir)?;
    Ok(dir)
}

/// Origin of the running sidecar, starting it if needed.
pub async fn ensure_running<R: Runtime>(app: &AppHandle<R>) -> Result<String, SidecarError> {
    let state = app.state::<SidecarState>();
    let mut guard = state.inner.lock().await;
    if let Some(running) = guard.as_ref() {
        return Ok(running.origin.clone());
    }
    let running = spawn(app).await?;
    let origin = running.origin.clone();
    *guard = Some(running);
    Ok(origin)
}

/// Take the one-time login grant. Restarts the sidecar if the current grant
/// was already consumed: a fresh process is the only way to get a fresh grant,
/// and that is deliberate — grants must not be mintable over the network.
pub async fn take_grant<R: Runtime>(app: &AppHandle<R>) -> Result<(String, String), SidecarError> {
    let state = app.state::<SidecarState>();
    let mut guard = state.inner.lock().await;

    if let Some(running) = guard.as_mut() {
        if let Some(grant) = running.grant.take() {
            return Ok((running.origin.clone(), grant));
        }
    }
    if let Some(previous) = guard.take() {
        let _ = previous.child.kill();
    }
    let mut running = spawn(app).await?;
    let grant = running
        .grant
        .take()
        .ok_or_else(|| SidecarError("sidecar started without a grant".into()))?;
    let origin = running.origin.clone();
    *guard = Some(running);
    Ok((origin, grant))
}

/// Stop the sidecar (app exit, or the user switching to a remote server).
pub async fn stop<R: Runtime>(app: &AppHandle<R>) {
    let state = app.state::<SidecarState>();
    let running = state.inner.lock().await.take();
    if let Some(running) = running {
        let _ = running.child.kill();
    }
}

/// Synchronous variant for the exit hook, where no async runtime is available.
pub fn stop_blocking<R: Runtime>(app: &AppHandle<R>) {
    let state = app.state::<SidecarState>();
    let running = match state.inner.try_lock() {
        Ok(mut guard) => guard.take(),
        Err(_) => None,
    };
    if let Some(running) = running {
        let _ = running.child.kill();
    }
}

async fn spawn<R: Runtime>(app: &AppHandle<R>) -> Result<Running, SidecarError> {
    let dir = data_dir(app)?;
    let (mut events, child) = app
        .shell()
        .sidecar(SIDECAR_PROGRAM)?
        .args(["--local", "--data-dir", &dir.to_string_lossy()])
        .spawn()?;

    let handoff = match wait_ready(&mut events).await {
        Ok(handoff) => handoff,
        Err(err) => {
            let _ = child.kill();
            return Err(err);
        }
    };

    // Keep draining stderr so the child never blocks on a full pipe, and drop
    // our record of it when it exits so the next call restarts it. Match on
    // pid: a replaced process exiting must not erase the record of its successor.
    let app_handle = app.clone();
    let pid = child.pid();
    tauri::async_runtime::spawn(async move {
        while let Some(event) = events.recv().await {
            match event {
                CommandEvent::Stderr(line) => {
                    eprintln!("[sidecar] {}", String::from_utf8_lossy(&line).trim_end())
                }
                CommandEvent::Terminated(payload) => {
                    eprintln!("[sidecar] pid={pid} terminated: {:?}", payload.code);
                    let state = app_handle.state::<SidecarState>();
                    let mut guard = state.inner.lock().await;
                    if guard.as_ref().is_some_and(|r| r.child.pid() == pid) {
                        *guard = None;
                    }
                    break;
                }
                _ => {}
            }
        }
    });

    eprintln!(
        "[sidecar] ready pid={} origin={} version={}",
        handoff.pid, handoff.origin, handoff.version
    );
    Ok(Running {
        child,
        origin: handoff.origin,
        grant: Some(handoff.grant),
    })
}

async fn wait_ready(
    events: &mut tauri::async_runtime::Receiver<CommandEvent>,
) -> Result<Handoff, SidecarError> {
    tokio::time::timeout(READY_TIMEOUT, async {
        while let Some(event) = events.recv().await {
            match event {
                CommandEvent::Stdout(line) => {
                    if let Ok(handoff) = serde_json::from_slice::<Handoff>(&line) {
                        if handoff.kind == "ready" {
                            return Ok(handoff);
                        }
                    }
                }
                CommandEvent::Stderr(line) => {
                    eprintln!("[sidecar] {}", String::from_utf8_lossy(&line).trim_end());
                }
                CommandEvent::Error(message) => return Err(SidecarError(message)),
                CommandEvent::Terminated(payload) => {
                    return Err(SidecarError(format!(
                        "sidecar exited before ready (code {:?})",
                        payload.code
                    )));
                }
                _ => {}
            }
        }
        Err(SidecarError(
            "sidecar closed its output before ready".into(),
        ))
    })
    .await
    .map_err(|_| SidecarError("sidecar did not become ready in time".into()))?
}
