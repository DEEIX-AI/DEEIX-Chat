// How this copy was distributed, and what that means for updates:
//
//   kind       detected from                                update mode
//   portable   `portable` file next to the exe (Windows)    notify (link to the download page)
//   dev        debug build, or no bundle type               disabled
//   msi        Tauri bundle type MSI                        disabled (IT manages versions)
//   installer  any other bundle (NSIS, .app/.dmg, deb, …)   auto
//
// Policy DisableAutoUpdate turns every mode into `disabled`. Detection and the
// startup checks that must happen before Tauri starts (portable data folder,
// bundled WebView2, instance identity) live in `bootstrap`.

use serde::Serialize;
use tauri::utils::config::BundleType;
use tauri::{AppHandle, Manager, Runtime};
use tauri_plugin_updater::UpdaterExt;

use crate::dialog;
use crate::paths::{self, PortableLayout};
use crate::policy::{self, Policy};

/// Download page when the updater endpoint is not a GitHub release URL.
const DEFAULT_REPOSITORY_URL: &str = "https://github.com/DEEIX-AI/DEEIX-Chat";
#[cfg_attr(not(windows), allow(dead_code))]
pub const WEBVIEW2_DOWNLOAD_URL: &str = "https://developer.microsoft.com/microsoft-edge/webview2/";
const UPDATE_CHECK_TIMEOUT: std::time::Duration = std::time::Duration::from_secs(20);
/// Runtime capability that exposes the JS updater (download + install).
const UPDATER_CAPABILITY: &str = "updater-capability";

#[derive(Debug, Clone, Copy, PartialEq, Eq, Serialize)]
#[serde(rename_all = "lowercase")]
pub enum Kind {
    Installer,
    Msi,
    Portable,
    Dev,
}

#[derive(Debug, Clone, Copy, PartialEq, Eq, Serialize)]
#[serde(rename_all = "lowercase")]
pub enum UpdateMode {
    Auto,
    Notify,
    Disabled,
}

/// Managed state; fixed for the lifetime of the process.
#[derive(Debug, Clone)]
pub struct Distribution {
    pub kind: Kind,
    pub update_mode: UpdateMode,
    pub releases_url: String,
    pub policy: Policy,
    pub portable: Option<PortableLayout>,
}

/// `get_distribution` payload.
#[derive(Debug, Serialize)]
#[serde(rename_all = "camelCase")]
pub struct DistributionInfo {
    pub kind: Kind,
    pub update_mode: UpdateMode,
    pub releases_url: String,
    pub policy: Policy,
}

/// `check_update_notice` payload.
#[derive(Debug, Serialize)]
#[serde(rename_all = "camelCase")]
pub struct UpdateNotice {
    pub version: String,
    pub current_version: String,
}

// ---------- pure rules ----------

/// Precedence: portable marker, then debug build, then the bundle type the
/// bundler patched into the binary; anything unrecognised is `dev`.
///
/// The bundler patches the type into the copy it packs into each installer and
/// restores the unpatched binary afterwards, so `target/release/*.exe` (what
/// the portable zip ships) always reports `None` — portable must not rely on it.
pub fn detect_kind(portable_marker: bool, debug_build: bool, bundle: Option<BundleType>) -> Kind {
    if portable_marker {
        return Kind::Portable;
    }
    if debug_build {
        return Kind::Dev;
    }
    match bundle {
        Some(BundleType::Msi) => Kind::Msi,
        Some(
            BundleType::Nsis
            | BundleType::App
            | BundleType::Dmg
            | BundleType::AppImage
            | BundleType::Deb
            | BundleType::Rpm,
        ) => Kind::Installer,
        None => Kind::Dev,
    }
}

pub fn update_mode(kind: Kind, disabled_by_policy: bool) -> UpdateMode {
    if disabled_by_policy {
        return UpdateMode::Disabled;
    }
    match kind {
        Kind::Installer => UpdateMode::Auto,
        Kind::Portable => UpdateMode::Notify,
        Kind::Msi | Kind::Dev => UpdateMode::Disabled,
    }
}

/// Download page for manual updates, derived from the updater endpoint so a
/// fork's build points at the fork: stable → `/releases/latest`, beta →
/// `/releases` (GitHub's "latest" never shows a prerelease).
pub fn releases_url(updater_endpoint: Option<&str>, prerelease: bool) -> String {
    let repository = updater_endpoint
        .and_then(github_repository)
        .unwrap_or_else(|| DEFAULT_REPOSITORY_URL.to_string());
    if prerelease {
        format!("{repository}/releases")
    } else {
        format!("{repository}/releases/latest")
    }
}

fn github_repository(endpoint: &str) -> Option<String> {
    let url = reqwest::Url::parse(endpoint).ok()?;
    if url.scheme() != "https" || url.host_str() != Some("github.com") {
        return None;
    }
    let mut segments = url.path_segments()?;
    let owner = segments.next().filter(|s| !s.is_empty())?;
    let repo = segments.next().filter(|s| !s.is_empty())?;
    (segments.next() == Some("releases")).then(|| format!("https://github.com/{owner}/{repo}"))
}

/// Keychain service and single-instance key of a portable copy. Installed
/// builds keep the bundle identifier unchanged.
pub fn portable_identifier(base: &str, instance_id: &str) -> String {
    format!("{base}.portable-{instance_id}")
}

// ---------- startup ----------

/// Runs before the Tauri builder: detects the distribution, prepares a portable
/// copy (or exits with a native error), points WebView2 at a bundled runtime,
/// verifies WebView2 is usable and reads policy. Portable copies get their own
/// identifier, which isolates the keychain entries and the single-instance
/// lock from the installed app and from other portable copies.
pub fn bootstrap<R: Runtime>(context: &mut tauri::Context<R>) -> Distribution {
    let exe_dir = match paths::exe_dir() {
        Ok(dir) => Some(dir),
        Err(e) => {
            eprintln!("[distribution] cannot locate the executable: {e}");
            None
        }
    };
    let marker = exe_dir
        .as_deref()
        .is_some_and(paths::portable_marker_present);
    let kind = detect_kind(
        marker,
        cfg!(debug_assertions),
        tauri::utils::platform::bundle_type(),
    );

    let portable = match (kind, exe_dir) {
        (Kind::Portable, Some(exe_dir)) => Some(
            paths::prepare_portable(&exe_dir)
                .unwrap_or_else(|detail| dialog::fatal(&data_dir_message(&detail), None)),
        ),
        _ => None,
    };

    let bundled_webview2 = portable.as_ref().and_then(PortableLayout::bundled_webview2);
    #[cfg(windows)]
    if let Some(dir) = &bundled_webview2 {
        // Read by the WebView2 loader for every environment wry creates (wry
        // passes a null browserExecutableFolder, so the variable decides) and
        // by the version probe below. Set before any thread exists.
        std::env::set_var("WEBVIEW2_BROWSER_EXECUTABLE_FOLDER", dir);
        grant_app_container_access(dir);
    }
    ensure_webview_runtime(bundled_webview2.as_deref());

    if let Some(layout) = &portable {
        let identifier = portable_identifier(&context.config().identifier, &layout.instance_id);
        eprintln!(
            "[distribution] portable: data={} identifier={identifier}",
            layout.data_dir.display()
        );
        context.config_mut().identifier = identifier;
    }

    let policy = policy::load();
    let endpoint = context
        .config()
        .plugins
        .0
        .get("updater")
        .and_then(|updater| updater.get("endpoints"))
        .and_then(|endpoints| endpoints.get(0))
        .and_then(|endpoint| endpoint.as_str())
        .map(str::to_string);
    let prerelease = !context.package_info().version.pre.is_empty();
    let distribution = Distribution {
        kind,
        update_mode: update_mode(kind, policy.auto_update_disabled_by_policy),
        releases_url: releases_url(endpoint.as_deref(), prerelease),
        policy,
        portable,
    };
    eprintln!(
        "[distribution] kind={:?} update={:?}",
        distribution.kind, distribution.update_mode
    );
    distribution
}

/// Grant the JS updater (check + download + install) only where installing is
/// allowed. Everywhere else the webview cannot reach the updater plugin at all,
/// whatever the web code does; `check_update_notice` covers `notify`.
pub fn grant_updater<R: Runtime>(app: &AppHandle<R>) -> tauri::Result<()> {
    if app.state::<Distribution>().update_mode != UpdateMode::Auto {
        return Ok(());
    }
    app.add_capability(
        tauri::ipc::CapabilityBuilder::new(UPDATER_CAPABILITY)
            .webview("tab-*")
            .permission("updater:default"),
    )
}

fn data_dir_message(detail: &str) -> String {
    format!(
        "DEEIX Chat (portable) cannot write to its data folder:\n{detail}\n\n\
         Move the whole \"DEEIX Chat\" folder to a location where you can create files \
         (for example Documents, the desktop or a USB drive) and start it again. \
         The portable version never stores its data anywhere else.\n\n\
         便携版 DEEIX Chat 无法写入数据文件夹：\n{detail}\n\n\
         请将整个“DEEIX Chat”文件夹移动到可以写入文件的位置（例如“文档”、桌面或 U 盘）后重新启动。\
         便携版不会把数据存放到其他位置。"
    )
}

/// Exit with an explanation instead of crashing when no WebView2 runtime can
/// be loaded. Only Windows has an optional system webview.
#[cfg_attr(not(windows), allow(unused_variables))]
fn ensure_webview_runtime(bundled: Option<&std::path::Path>) {
    #[cfg(windows)]
    {
        let error = match tauri::webview_version() {
            Ok(version) if !version.trim().is_empty() => {
                eprintln!("[distribution] WebView2 {version}");
                return;
            }
            Ok(_) => "no version reported".to_string(),
            Err(e) => e.to_string(),
        };
        let message = match bundled {
            Some(dir) => format!(
                "The WebView2 runtime included with this portable copy could not be loaded:\n{}\n({error})\n\n\
                 Extract the offline package again, or delete the \"webview2\" folder to use the \
                 WebView2 Runtime installed on this computer.\n\n\
                 无法加载此便携版自带的 WebView2 运行时：\n{}\n\n\
                 请重新解压离线包，或删除“webview2”文件夹以改用本机已安装的 WebView2 运行时。",
                dir.display(),
                dir.display(),
            ),
            None => format!(
                "DEEIX Chat needs the Microsoft Edge WebView2 Runtime, which was not found on this \
                 computer ({error}).\n\n\
                 Install the Evergreen WebView2 Runtime from Microsoft (administrators can deploy it \
                 for all users), or use the portable \"-offline\" package, which includes it.\n\n\
                 DEEIX Chat 需要 Microsoft Edge WebView2 运行时，但此电脑上未找到。\n\
                 请从 Microsoft 安装 Evergreen WebView2 运行时（管理员可为所有用户统一部署），\
                 或使用自带运行时的便携版“-offline”包。"
            ),
        };
        dialog::fatal(&message, Some(WEBVIEW2_DOWNLOAD_URL));
    }
}

/// WebView2 runs its renderer and GPU processes in an AppContainer sandbox,
/// which can only load a Fixed Version runtime that ALL APPLICATION PACKAGES
/// (S-1-15-2-1) and ALL RESTRICTED APPLICATION PACKAGES (S-1-15-2-2) may read.
/// Program Files grants that; a folder extracted from a zip into a user
/// profile does not, and zip files cannot carry ACLs. Grant it once per
/// extracted folder (the stamp lives inside it, so a re-extracted folder is
/// processed again). FAT/exFAT drives have no ACLs; icacls fails harmlessly.
#[cfg(windows)]
fn grant_app_container_access(dir: &std::path::Path) {
    use std::os::windows::process::CommandExt;
    const CREATE_NO_WINDOW: u32 = 0x0800_0000;
    const STAMP: &str = ".appcontainer-acl";

    let stamp = dir.join(STAMP);
    if stamp.is_file() {
        return;
    }
    let system_root = std::env::var_os("SystemRoot").unwrap_or_else(|| "C:\\Windows".into());
    let icacls = std::path::Path::new(&system_root)
        .join("System32")
        .join("icacls.exe");
    let status = std::process::Command::new(icacls)
        .arg(dir)
        .args([
            "/grant",
            "*S-1-15-2-1:(OI)(CI)(RX)",
            "*S-1-15-2-2:(OI)(CI)(RX)",
            "/Q",
        ])
        .creation_flags(CREATE_NO_WINDOW)
        .status();
    match status {
        Ok(status) if status.success() => {
            let _ = std::fs::write(&stamp, b"granted\n");
        }
        Ok(status) => eprintln!("[distribution] icacls on bundled WebView2 exited with {status}"),
        Err(e) => eprintln!("[distribution] cannot run icacls: {e}"),
    }
}

// ---------- commands (content tabs) ----------

#[tauri::command]
pub fn get_distribution<R: Runtime>(app: AppHandle<R>) -> DistributionInfo {
    let distribution = app.state::<Distribution>();
    DistributionInfo {
        kind: distribution.kind,
        update_mode: distribution.update_mode,
        releases_url: distribution.releases_url.clone(),
        policy: distribution.policy.clone(),
    }
}

/// Whether the updater endpoint announces a newer version. Checks only: the
/// payload is neither downloaded nor verified here. Uses the updater plugin so
/// the endpoint (including a beta build's override), proxy and version rules
/// are exactly the ones `auto` mode uses. Works without a bundle type: the
/// plugin then looks up the plain `windows-x86_64` entry of latest.json.
#[tauri::command]
pub async fn check_update_notice<R: Runtime>(
    app: AppHandle<R>,
) -> Result<Option<UpdateNotice>, String> {
    let (mode, by_policy) = {
        let distribution = app.state::<Distribution>();
        (
            distribution.update_mode,
            distribution.policy.auto_update_disabled_by_policy,
        )
    };
    if mode == UpdateMode::Disabled {
        return Err(if by_policy {
            policy::UPDATES_DISABLED.to_string()
        } else {
            "update checks are disabled for this distribution".to_string()
        });
    }
    let update = app
        .updater_builder()
        .timeout(UPDATE_CHECK_TIMEOUT)
        .build()
        .map_err(|e| e.to_string())?
        .check()
        .await
        .map_err(|e| e.to_string())?;
    Ok(update.map(|update| UpdateNotice {
        version: update.version,
        current_version: update.current_version,
    }))
}

#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    fn portable_marker_wins_over_everything() {
        for bundle in [None, Some(BundleType::Msi), Some(BundleType::Nsis)] {
            assert_eq!(detect_kind(true, false, bundle.clone()), Kind::Portable);
            assert_eq!(detect_kind(true, true, bundle), Kind::Portable);
        }
    }

    #[test]
    fn debug_builds_are_dev() {
        assert_eq!(detect_kind(false, true, Some(BundleType::Nsis)), Kind::Dev);
        assert_eq!(detect_kind(false, true, Some(BundleType::Msi)), Kind::Dev);
        assert_eq!(detect_kind(false, true, None), Kind::Dev);
    }

    #[test]
    fn release_builds_follow_the_bundle_type() {
        assert_eq!(detect_kind(false, false, Some(BundleType::Msi)), Kind::Msi);
        for bundle in [
            BundleType::Nsis,
            BundleType::App,
            BundleType::Dmg,
            BundleType::AppImage,
            BundleType::Deb,
            BundleType::Rpm,
        ] {
            assert_eq!(detect_kind(false, false, Some(bundle)), Kind::Installer);
        }
        // An unpatched binary run straight from target/release.
        assert_eq!(detect_kind(false, false, None), Kind::Dev);
    }

    #[test]
    fn update_mode_table() {
        let table = [
            (Kind::Installer, false, UpdateMode::Auto),
            (Kind::Portable, false, UpdateMode::Notify),
            (Kind::Msi, false, UpdateMode::Disabled),
            (Kind::Dev, false, UpdateMode::Disabled),
            (Kind::Installer, true, UpdateMode::Disabled),
            (Kind::Portable, true, UpdateMode::Disabled),
            (Kind::Msi, true, UpdateMode::Disabled),
            (Kind::Dev, true, UpdateMode::Disabled),
        ];
        for (kind, by_policy, expected) in table {
            assert_eq!(
                update_mode(kind, by_policy),
                expected,
                "{kind:?} policy={by_policy}"
            );
        }
    }

    #[test]
    fn releases_url_follows_endpoint_and_channel() {
        let stable = "https://github.com/DEEIX-AI/DEEIX-Chat/releases/latest/download/latest.json";
        let beta = "https://github.com/fork/Chat/releases/download/desktop-beta/latest.json";
        assert_eq!(
            releases_url(Some(stable), false),
            "https://github.com/DEEIX-AI/DEEIX-Chat/releases/latest"
        );
        assert_eq!(
            releases_url(Some(beta), true),
            "https://github.com/fork/Chat/releases"
        );
        for other in [
            None,
            Some("https://updates.example.com/latest.json"),
            Some("not a url"),
        ] {
            assert_eq!(
                releases_url(other, false),
                "https://github.com/DEEIX-AI/DEEIX-Chat/releases/latest"
            );
        }
    }

    #[test]
    fn portable_identifier_keeps_the_base_and_adds_the_instance() {
        assert_eq!(
            portable_identifier("com.deeix.chat.desktop", "0123456789abcdef"),
            "com.deeix.chat.desktop.portable-0123456789abcdef"
        );
    }

    #[test]
    fn serialises_as_the_contract_says() {
        let info = DistributionInfo {
            kind: Kind::Portable,
            update_mode: UpdateMode::Notify,
            releases_url: "https://x/releases/latest".into(),
            policy: Policy::default(),
        };
        let json = serde_json::to_value(&info).unwrap();
        assert_eq!(
            json,
            serde_json::json!({
                "kind": "portable",
                "updateMode": "notify",
                "releasesUrl": "https://x/releases/latest",
                "policy": {
                    "defaultServerUrl": null,
                    "serverUrlLocked": false,
                    "localModeAllowed": true,
                    "autoUpdateDisabledByPolicy": false
                }
            })
        );
        let notice = serde_json::to_value(UpdateNotice {
            version: "1.2.3".into(),
            current_version: "1.2.2".into(),
        })
        .unwrap();
        assert_eq!(
            notice,
            serde_json::json!({"version": "1.2.3", "currentVersion": "1.2.2"})
        );
    }
}
