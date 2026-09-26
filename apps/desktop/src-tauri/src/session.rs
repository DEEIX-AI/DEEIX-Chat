// Session persistence and refresh.
//
// The refresh token lives in the OS keychain and is only ever sent to the
// server it was issued for, by this module. The webview hands it over once
// after login and can afterwards only request an access token or sign out —
// the HttpOnly-cookie property of the browser build.
//
// Commands resolve their server from the calling webview's tab (tabs.rs); the
// keychain is keyed per server. Local mode uses the constant key "local" and
// resolves the sidecar's origin at call time, since its port changes per launch.

use std::time::Duration;

use keyring::Entry;
use serde::{Deserialize, Serialize};
use tauri::{AppHandle, Manager, Runtime, Webview};

use crate::sidecar;
use crate::tabs::{self, Server};

const CLIENT_PLATFORM_HEADER: &str = "X-Client-Platform";
const REFRESH_PATH: &str = "/api/v1/auth/refresh";
const LOCAL_EXCHANGE_PATH: &str = "/api/v1/auth/local/exchange";
const HTTP_TIMEOUT: Duration = Duration::from_secs(15);
const MAX_BODY_BYTES: u64 = 256 * 1024;

/// Serialises `local_sign_in`; the local server is shared by all local tabs.
static LOCAL_SIGN_IN: tokio::sync::Mutex<()> = tokio::sync::Mutex::const_new(());

#[derive(Debug, Serialize)]
#[serde(rename_all = "camelCase")]
pub struct SessionError {
    /// "network" | "http" | "storage" | "no_session" | "invalid_origin" | "sidecar" | "no_server" | "tabs"
    pub kind: &'static str,
    pub message: String,
    #[serde(skip_serializing_if = "Option::is_none")]
    pub status: Option<u16>,
    #[serde(skip_serializing_if = "Option::is_none")]
    pub error_code: Option<String>,
}

impl SessionError {
    fn new(kind: &'static str, message: impl std::fmt::Display) -> Self {
        Self {
            kind,
            message: message.to_string(),
            status: None,
            error_code: None,
        }
    }
    fn storage(e: impl std::fmt::Display) -> Self {
        Self::new("storage", e)
    }
    fn network(e: impl std::fmt::Display) -> Self {
        Self::new("network", e)
    }
    fn no_session() -> Self {
        Self::new("no_session", "no stored session")
    }
    fn no_server() -> Self {
        Self::new("no_server", "no server configured")
    }
}

impl From<sidecar::SidecarError> for SessionError {
    fn from(e: sidecar::SidecarError) -> Self {
        Self::new("sidecar", e.0)
    }
}

impl From<tabs::TabsError> for SessionError {
    fn from(e: tabs::TabsError) -> Self {
        Self::new("tabs", e.0)
    }
}

#[derive(Debug, Serialize, Deserialize)]
#[serde(rename_all = "camelCase")]
pub struct SessionCredentials {
    pub access_token: String,
    #[serde(rename = "sessionID")]
    pub session_id: String,
}

#[derive(Deserialize)]
#[serde(rename_all = "camelCase")]
struct Envelope {
    #[serde(default)]
    error_msg: String,
    #[serde(default)]
    error_code: Option<String>,
    #[serde(default)]
    data: Option<RefreshData>,
}

#[derive(Deserialize)]
#[serde(rename_all = "camelCase")]
struct RefreshData {
    #[serde(default)]
    access_token: String,
    #[serde(rename = "sessionID", default)]
    session_id: String,
    #[serde(default)]
    refresh_token: Option<String>,
}

/// reqwest is built with `rustls-no-provider`, so a process-wide crypto provider
/// must exist before the first client is constructed. Idempotent.
pub(crate) fn ensure_tls_provider() {
    if rustls::crypto::CryptoProvider::get_default().is_none() {
        let _ = rustls::crypto::ring::default_provider().install_default();
    }
}

// ---------- server selection ----------

#[derive(Debug, Clone, Copy, PartialEq, Eq, Serialize, Deserialize)]
#[serde(rename_all = "lowercase")]
pub enum ServerMode {
    Local,
    Remote,
}

/// What the webview needs to know about its server.
#[derive(Debug, Serialize)]
#[serde(rename_all = "camelCase")]
pub struct ServerInfo {
    pub mode: ServerMode,
    pub origin: String,
}

/// Server bound to the calling webview's tab.
fn server_for<R: Runtime>(webview: &Webview<R>) -> Option<Server> {
    tabs::server_of(webview.app_handle(), webview.label())
}

/// Live origin for a server (starts the sidecar in local mode).
async fn resolve_origin<R: Runtime>(
    app: &AppHandle<R>,
    server: &Server,
) -> Result<String, SessionError> {
    match server.mode {
        ServerMode::Local => Ok(sidecar::ensure_running(app).await?),
        ServerMode::Remote => Ok(server.origin.clone()),
    }
}

/// Drop everything stored for a server (tab closed, server no longer open).
pub fn forget<R: Runtime>(app: &AppHandle<R>, server: &Server) -> Result<(), SessionError> {
    delete_token(app, &server.keychain_key())
}

/// Accept only an absolute http(s) origin with no path, query, fragment or userinfo.
pub(crate) fn normalize_origin(raw: &str) -> Option<String> {
    let url = reqwest::Url::parse(raw.trim()).ok()?;
    if !matches!(url.scheme(), "http" | "https") {
        return None;
    }
    if !url.username().is_empty()
        || url.password().is_some()
        || url.query().is_some()
        || url.fragment().is_some()
    {
        return None;
    }
    if !matches!(url.path(), "" | "/") {
        return None;
    }
    url.host_str()?;
    Some(url.origin().ascii_serialization())
}

// ---------- keychain ----------

/// Keychain service = bundle identifier, so dev and installed builds
/// (different identifiers) never share credentials.
fn entry<R: Runtime>(app: &AppHandle<R>, key: &str) -> Result<Entry, SessionError> {
    Entry::new(&app.config().identifier, &format!("refresh-token:{key}"))
        .map_err(SessionError::storage)
}

fn read_token<R: Runtime>(app: &AppHandle<R>, key: &str) -> Result<Option<String>, SessionError> {
    match entry(app, key)?.get_password() {
        Ok(token) => Ok(Some(token)),
        Err(keyring::Error::NoEntry) => Ok(None),
        Err(e) => Err(SessionError::storage(e)),
    }
}

fn write_token<R: Runtime>(app: &AppHandle<R>, key: &str, token: &str) -> Result<(), SessionError> {
    entry(app, key)?
        .set_password(token)
        .map_err(SessionError::storage)
}

fn delete_token<R: Runtime>(app: &AppHandle<R>, key: &str) -> Result<(), SessionError> {
    match entry(app, key)?.delete_credential() {
        Ok(()) | Err(keyring::Error::NoEntry) => Ok(()),
        Err(e) => Err(SessionError::storage(e)),
    }
}

// ---------- commands ----------

/// The calling tab's server, or null when the tab has not chosen one yet. In
/// local mode the origin is the live sidecar address (started if necessary).
#[tauri::command]
pub async fn get_server<R: Runtime>(
    app: AppHandle<R>,
    webview: Webview<R>,
) -> Result<Option<ServerInfo>, SessionError> {
    let Some(server) = server_for(&webview) else {
        return Ok(None);
    };
    let origin = resolve_origin(&app, &server).await?;
    Ok(Some(ServerInfo {
        mode: server.mode,
        origin,
    }))
}

/// Bind this tab to a remote server.
#[tauri::command]
pub async fn set_remote_server<R: Runtime>(
    app: AppHandle<R>,
    webview: Webview<R>,
    origin: String,
) -> Result<ServerInfo, SessionError> {
    let normalized = normalize_origin(&origin).ok_or_else(|| {
        SessionError::new(
            "invalid_origin",
            "origin must be an absolute http(s) URL without a path",
        )
    })?;
    tabs::bind(&app, webview.label(), Server::remote(normalized.clone()))?;
    Ok(ServerInfo {
        mode: ServerMode::Remote,
        origin: normalized,
    })
}

/// Bind this tab to the bundled local server. Starts the sidecar and returns its origin.
#[tauri::command]
pub async fn set_local_server<R: Runtime>(
    app: AppHandle<R>,
    webview: Webview<R>,
) -> Result<ServerInfo, SessionError> {
    tabs::bind(&app, webview.label(), Server::local())?;
    let origin = sidecar::ensure_running(&app).await?;
    Ok(ServerInfo {
        mode: ServerMode::Local,
        origin,
    })
}

/// Drop this tab's credential and return it to the setup screen. Sign-out in
/// local mode maps to this: the local owner has no login form.
#[tauri::command]
pub async fn leave_server<R: Runtime>(
    app: AppHandle<R>,
    webview: Webview<R>,
) -> Result<(), SessionError> {
    tabs::unbind(&app, webview.label()).await?;
    Ok(())
}

/// Persist the refresh token issued at login. This is the only moment the
/// webview holds the token; there is no read command.
#[tauri::command]
pub fn store_session<R: Runtime>(
    app: AppHandle<R>,
    webview: Webview<R>,
    refresh_token: String,
) -> Result<(), SessionError> {
    let server = server_for(&webview).ok_or_else(SessionError::no_server)?;
    let key = server.keychain_key();
    if refresh_token.trim().is_empty() {
        return delete_token(&app, &key);
    }
    write_token(&app, &key, refresh_token.trim())
}

/// Drop the stored refresh token (sign-out).
#[tauri::command]
pub fn clear_session<R: Runtime>(
    app: AppHandle<R>,
    webview: Webview<R>,
) -> Result<(), SessionError> {
    match server_for(&webview) {
        Some(server) => delete_token(&app, &server.keychain_key()),
        None => Ok(()),
    }
}

/// Exchange the stored refresh token for a new access token. Rotates the
/// stored token on success; clears it when the server says the session is gone.
#[tauri::command]
pub async fn refresh_session<R: Runtime>(
    app: AppHandle<R>,
    webview: Webview<R>,
) -> Result<SessionCredentials, SessionError> {
    let server = server_for(&webview).ok_or_else(SessionError::no_server)?;
    let key = server.keychain_key();
    let token = read_token(&app, &key)?.ok_or_else(SessionError::no_session)?;
    let origin = resolve_origin(&app, &server).await?;

    match perform_refresh(&origin, &token).await {
        Ok(Refreshed {
            credentials,
            rotated_token,
        }) => {
            if let Some(rotated) = rotated_token {
                write_token(&app, &key, &rotated)?;
            }
            Ok(credentials)
        }
        Err(e) => {
            // 401 for any reason means the server will not honour this token again.
            if e.status == Some(401) || e.kind == "no_session" {
                delete_token(&app, &key)?;
            }
            Err(e)
        }
    }
}

/// Local mode sign-in: refresh the stored token, or redeem the sidecar's
/// one-time grant. The grant never reaches the webview.
#[tauri::command]
pub async fn local_sign_in<R: Runtime>(
    app: AppHandle<R>,
    webview: Webview<R>,
) -> Result<SessionCredentials, SessionError> {
    let server = server_for(&webview).ok_or_else(SessionError::no_server)?;
    if server.mode != ServerMode::Local {
        return Err(SessionError::new(
            "invalid_origin",
            "local sign-in requires local mode",
        ));
    }
    let key = server.keychain_key();
    // Serialise sign-ins: a concurrent caller would otherwise find the grant
    // consumed and restart the sidecar underneath the exchange in flight.
    let _guard = LOCAL_SIGN_IN.lock().await;
    if let Some(token) = read_token(&app, &key)? {
        match perform_refresh(&sidecar::ensure_running(&app).await?, &token).await {
            Ok(Refreshed {
                credentials,
                rotated_token,
            }) => {
                if let Some(rotated) = rotated_token {
                    write_token(&app, &key, &rotated)?;
                }
                return Ok(credentials);
            }
            // A rejected token is dropped and replaced through a fresh grant.
            Err(e) if e.status == Some(401) => delete_token(&app, &key)?,
            Err(e) => return Err(e),
        }
    }
    let (origin, grant) = sidecar::take_grant(&app).await?;
    let Refreshed {
        credentials,
        rotated_token,
    } = post_session(
        &origin,
        LOCAL_EXCHANGE_PATH,
        &serde_json::json!({ "grant": grant }),
    )
    .await?;
    let token = rotated_token
        .ok_or_else(|| SessionError::new("http", "local exchange returned no refresh token"))?;
    write_token(&app, &key, &token)?;
    Ok(credentials)
}

#[derive(Debug)]
pub(crate) struct Refreshed {
    pub credentials: SessionCredentials,
    pub rotated_token: Option<String>,
}

/// The HTTP half of a refresh, independent of any storage.
pub(crate) async fn perform_refresh(origin: &str, token: &str) -> Result<Refreshed, SessionError> {
    post_session(
        origin,
        REFRESH_PATH,
        &serde_json::json!({ "refreshToken": token }),
    )
    .await
}

/// POST to a session-issuing endpoint as a native client and parse the envelope.
async fn post_session(
    origin: &str,
    path: &str,
    body: &serde_json::Value,
) -> Result<Refreshed, SessionError> {
    ensure_tls_provider();
    let client = reqwest::Client::builder()
        .timeout(HTTP_TIMEOUT)
        .redirect(reqwest::redirect::Policy::none())
        .build()
        .map_err(SessionError::network)?;

    let response = client
        .post(format!("{origin}{path}"))
        .header(CLIENT_PLATFORM_HEADER, "desktop")
        .json(body)
        .send()
        .await
        .map_err(SessionError::network)?;

    let status = response.status().as_u16();
    let bytes = response.bytes().await.map_err(SessionError::network)?;
    if bytes.len() as u64 > MAX_BODY_BYTES {
        return Err(SessionError::network("response too large"));
    }
    let envelope: Envelope = serde_json::from_slice(&bytes).unwrap_or(Envelope {
        error_msg: String::from_utf8_lossy(&bytes).into_owned(),
        error_code: None,
        data: None,
    });

    let Some(data) = envelope.data.filter(|_| (200..300).contains(&status)) else {
        return Err(SessionError {
            kind: "http",
            message: if envelope.error_msg.is_empty() {
                format!("request failed: {status}")
            } else {
                envelope.error_msg
            },
            status: Some(status),
            error_code: envelope.error_code,
        });
    };
    if data.access_token.is_empty() {
        return Err(SessionError::no_session());
    }
    Ok(Refreshed {
        credentials: SessionCredentials {
            access_token: data.access_token,
            session_id: data.session_id,
        },
        rotated_token: data.refresh_token.filter(|t| !t.is_empty()),
    })
}

#[cfg(test)]
mod tests {
    use super::normalize_origin;

    #[test]
    fn accepts_plain_origins() {
        assert_eq!(
            normalize_origin("https://chat.example.com").as_deref(),
            Some("https://chat.example.com")
        );
        assert_eq!(
            normalize_origin(" http://127.0.0.1:8080/ ").as_deref(),
            Some("http://127.0.0.1:8080")
        );
        assert_eq!(
            normalize_origin("HTTPS://Chat.Example.com").as_deref(),
            Some("https://chat.example.com")
        );
    }

    #[test]
    fn rejects_anything_that_is_not_an_origin() {
        for bad in [
            "chat.example.com",
            "ftp://chat.example.com",
            "https://user:pw@chat.example.com",
            "https://chat.example.com/api",
            "https://chat.example.com/?x=1",
            "https://chat.example.com/#f",
            "javascript:alert(1)",
            "",
        ] {
            assert!(normalize_origin(bad).is_none(), "{bad} should be rejected");
        }
    }
}

// Live integration test: needs a running server and a desktop-issued refresh token.
//   DEEIX_TEST_ORIGIN=http://127.0.0.1:8080 DEEIX_TEST_REFRESH_TOKEN=... cargo test --lib -- --ignored
#[cfg(test)]
mod live {
    use super::perform_refresh;

    #[tokio::test]
    #[ignore = "requires DEEIX_TEST_ORIGIN and DEEIX_TEST_REFRESH_TOKEN"]
    async fn refresh_against_live_server_rotates_token() {
        let origin = std::env::var("DEEIX_TEST_ORIGIN").expect("DEEIX_TEST_ORIGIN");
        let token = std::env::var("DEEIX_TEST_REFRESH_TOKEN").expect("DEEIX_TEST_REFRESH_TOKEN");

        let first = perform_refresh(&origin, &token)
            .await
            .expect("first refresh succeeds");
        assert!(!first.credentials.access_token.is_empty());
        assert!(!first.credentials.session_id.is_empty());
        let rotated = first
            .rotated_token
            .expect("native refresh returns a rotated token in the body");
        assert_ne!(rotated, token, "server must rotate the refresh token");

        // The server keeps the previous token valid for a short grace window
        // (refreshTokenPreviousHashGrace, 15s) so a lost rotation response does
        // not strand the client. Reuse *outside* the window revokes the session;
        // that path is covered by the backend's own tests.
        let replay = perform_refresh(&origin, &token)
            .await
            .expect("replay inside the grace window is tolerated");
        assert!(replay.rotated_token.is_some());

        // The rotated token is the live one.
        let second = perform_refresh(&origin, &rotated)
            .await
            .expect("rotated token is valid");
        assert_ne!(second.rotated_token.as_deref(), Some(rotated.as_str()));

        // A token that was never issued is rejected with the terminating code.
        let bogus = perform_refresh(&origin, "not-a-token")
            .await
            .expect_err("bogus token must fail");
        assert_eq!(bogus.status, Some(401));
        assert_eq!(
            bogus.error_code.as_deref(),
            Some("auth.invalid_refresh_token")
        );
    }
}
