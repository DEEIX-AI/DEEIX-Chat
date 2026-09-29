// The one place that decides where the app keeps its files. Installed builds
// use Tauri's per-user directories (unchanged from earlier releases, so no
// migration). A portable build keeps everything in `<exe dir>/data/` and never
// falls back to %APPDATA%: if that folder is not writable the app refuses to
// start (see `prepare_portable`).
//
// Portable layout:
//   DEEIX Chat.exe, deeix-chat-server.exe, portable (marker), README.txt
//   webview2/            optional Fixed Version WebView2 runtime
//   data/instance-id     identity of this copy (keychain + single-instance)
//   data/tabs.json       open tabs
//   data/local/          bundled server (SQLite, uploads)
//   data/tmp/            sidecar temp files, cleared on start
//   data/webview/        WebView2 user data (cookies, cache, storage)

use std::fs;
use std::io::{ErrorKind, Write};
use std::path::{Path, PathBuf};

use tauri::{AppHandle, Manager, Runtime};

/// File next to the executable that switches the app into portable mode.
pub const PORTABLE_MARKER: &str = "portable";
/// Folder next to the executable holding a WebView2 Fixed Version runtime.
pub const BUNDLED_WEBVIEW2_DIR: &str = "webview2";
const PORTABLE_DATA_DIR: &str = "data";
const INSTANCE_ID_FILE: &str = "instance-id";
const WEBVIEW_DIR: &str = "webview";
const TMP_DIR: &str = "tmp";
const INSTANCE_ID_LEN: usize = 16;

/// A portable copy that passed the startup checks.
#[derive(Debug, Clone)]
pub struct PortableLayout {
    pub exe_dir: PathBuf,
    pub data_dir: PathBuf,
    /// Random id persisted in `data/instance-id`; see `load_or_create_instance_id`.
    pub instance_id: String,
}

impl PortableLayout {
    /// `<exe dir>/webview2` when present. It is used even if incomplete, so a
    /// broken offline package fails loudly instead of silently using (or
    /// missing) the system runtime.
    pub fn bundled_webview2(&self) -> Option<PathBuf> {
        let dir = self.exe_dir.join(BUNDLED_WEBVIEW2_DIR);
        dir.is_dir().then_some(dir)
    }
}

/// Directory of the running executable, without a Windows verbatim prefix.
pub fn exe_dir() -> std::io::Result<PathBuf> {
    // Tauri's variant canonicalises and rejects symlinked binaries on macOS.
    let exe = tauri::utils::platform::current_exe()?;
    let dir = exe
        .parent()
        .ok_or_else(|| std::io::Error::new(ErrorKind::NotFound, "executable has no parent"))?;
    Ok(dunce::simplified(dir).to_path_buf())
}

/// Portable mode is a Windows distribution; the marker is ignored elsewhere
/// because macOS/Linux webviews cannot be pointed at a custom profile folder.
pub fn portable_marker_present(exe_dir: &Path) -> bool {
    cfg!(windows) && exe_dir.join(PORTABLE_MARKER).is_file()
}

/// Create `<exe dir>/data`, prove it is writable and load this copy's id.
/// The error is a user-facing sentence; the caller shows it and exits.
pub fn prepare_portable(exe_dir: &Path) -> Result<PortableLayout, String> {
    let data_dir = exe_dir.join(PORTABLE_DATA_DIR);
    let fail = |e: std::io::Error| format!("{}: {e}", data_dir.display());
    fs::create_dir_all(&data_dir).map_err(fail)?;
    probe_writable(&data_dir).map_err(fail)?;
    let instance_id = load_or_create_instance_id(&data_dir).map_err(fail)?;
    Ok(PortableLayout {
        exe_dir: exe_dir.to_path_buf(),
        data_dir,
        instance_id,
    })
}

/// Some locations accept `create_dir` but not file writes (e.g. a read-only
/// share or a folder with a deny ACE for files), so write a real file.
fn probe_writable(dir: &Path) -> std::io::Result<()> {
    let probe = dir.join(format!(".write-test-{}", std::process::id()));
    let result = fs::OpenOptions::new()
        .write(true)
        .create(true)
        .truncate(true)
        .open(&probe)
        .and_then(|mut file| file.write_all(b"ok").and_then(|_| file.sync_all()));
    let _ = fs::remove_file(&probe);
    result
}

/// This copy's identity: a random id stored in the data folder the first time
/// it runs. It is part of the keychain service and single-instance names, so
/// two portable copies (or a portable copy and an installed app) never share
/// credentials or block each other. Because it lives *in* the data folder it
/// survives moving the folder, a changed USB drive letter and an update that
/// keeps `data/`, where a hash of the path would not.
pub fn load_or_create_instance_id(data_dir: &Path) -> std::io::Result<String> {
    let path = data_dir.join(INSTANCE_ID_FILE);
    if let Some(id) = read_instance_id(&path)? {
        return Ok(id);
    }
    let id = new_instance_id(data_dir);
    // create_new: when two first launches race, the loser adopts the winner's id.
    match fs::OpenOptions::new()
        .write(true)
        .create_new(true)
        .open(&path)
    {
        Ok(mut file) => {
            file.write_all(id.as_bytes())?;
            file.sync_all()?;
            Ok(id)
        }
        Err(e) if e.kind() == ErrorKind::AlreadyExists => match read_instance_id(&path)? {
            Some(existing) => Ok(existing),
            None => {
                // Present but damaged: replace it atomically.
                let tmp = path.with_extension("tmp");
                fs::write(&tmp, id.as_bytes())?;
                fs::rename(&tmp, &path)?;
                Ok(id)
            }
        },
        Err(e) => Err(e),
    }
}

fn read_instance_id(path: &Path) -> std::io::Result<Option<String>> {
    match fs::read_to_string(path) {
        Ok(text) => {
            let id = text.trim().to_ascii_lowercase();
            Ok(is_valid_instance_id(&id).then_some(id))
        }
        Err(e) if e.kind() == ErrorKind::NotFound => Ok(None),
        Err(e) => Err(e),
    }
}

pub fn is_valid_instance_id(id: &str) -> bool {
    id.len() == INSTANCE_ID_LEN
        && id
            .bytes()
            .all(|b| b.is_ascii_hexdigit() && !b.is_ascii_uppercase())
}

/// 64 bits from the process' randomly keyed SipHash (std's `RandomState`),
/// mixed with time and location. Uniqueness, not secrecy, is what matters.
fn new_instance_id(data_dir: &Path) -> String {
    use std::hash::{BuildHasher, Hasher};
    let mut hasher = std::collections::hash_map::RandomState::new().build_hasher();
    hasher.write(data_dir.as_os_str().as_encoded_bytes());
    hasher.write_u32(std::process::id());
    if let Ok(elapsed) = std::time::SystemTime::now().duration_since(std::time::UNIX_EPOCH) {
        hasher.write_u128(elapsed.as_nanos());
    }
    format!("{:016x}", hasher.finish())
}

/// Resolved directories, managed as app state.
#[derive(Debug, Clone)]
pub struct AppPaths {
    config_dir: PathBuf,
    data_dir: PathBuf,
    /// `None` keeps Tauri's default WebView2 profile (installed builds).
    webview_dir: Option<PathBuf>,
    /// Temp folder for child processes; `None` inherits the system one.
    tmp_dir: Option<PathBuf>,
}

impl AppPaths {
    pub fn resolve<R: Runtime>(
        app: &AppHandle<R>,
        portable: Option<&PortableLayout>,
    ) -> tauri::Result<Self> {
        Ok(match portable {
            Some(layout) => Self::portable(&layout.data_dir),
            None => Self {
                config_dir: app.path().app_config_dir()?,
                data_dir: app.path().app_data_dir()?,
                webview_dir: None,
                tmp_dir: None,
            },
        })
    }

    fn portable(data_dir: &Path) -> Self {
        Self {
            config_dir: data_dir.to_path_buf(),
            data_dir: data_dir.to_path_buf(),
            webview_dir: Some(data_dir.join(WEBVIEW_DIR)),
            tmp_dir: Some(data_dir.join(TMP_DIR)),
        }
    }

    /// Start every run with an empty temp folder. Only called by the primary
    /// instance (after the single-instance check), before the sidecar starts.
    pub fn reset_tmp(&self) {
        if let Some(tmp) = &self.tmp_dir {
            let _ = fs::remove_dir_all(tmp);
            if let Err(e) = fs::create_dir_all(tmp) {
                eprintln!("[paths] cannot create {}: {e}", tmp.display());
            }
        }
    }
}

fn state<R: Runtime>(app: &AppHandle<R>) -> tauri::State<'_, AppPaths> {
    app.state::<AppPaths>()
}

/// Settings such as tabs.json.
pub fn config_dir<R: Runtime>(app: &AppHandle<R>) -> PathBuf {
    state(app).config_dir.clone()
}

/// Application data such as the bundled server's database.
pub fn data_dir<R: Runtime>(app: &AppHandle<R>) -> PathBuf {
    state(app).data_dir.clone()
}

/// WebView2 user data folder for every webview, when not Tauri's default.
pub fn webview_dir<R: Runtime>(app: &AppHandle<R>) -> Option<PathBuf> {
    state(app).webview_dir.clone()
}

/// TMP/TEMP for child processes, when not the system default.
pub fn tmp_dir<R: Runtime>(app: &AppHandle<R>) -> Option<PathBuf> {
    state(app).tmp_dir.clone()
}

#[cfg(test)]
mod tests {
    use super::*;

    fn scratch(name: &str) -> PathBuf {
        let dir = std::env::temp_dir().join(format!(
            "deeix-paths-{name}-{}-{}",
            std::process::id(),
            new_instance_id(Path::new(name))
        ));
        let _ = fs::remove_dir_all(&dir);
        fs::create_dir_all(&dir).unwrap();
        dir
    }

    #[test]
    fn instance_id_is_created_once_and_then_stable() {
        let dir = scratch("stable");
        let first = load_or_create_instance_id(&dir).unwrap();
        assert!(is_valid_instance_id(&first), "{first}");
        assert_eq!(load_or_create_instance_id(&dir).unwrap(), first);
        assert_eq!(
            fs::read_to_string(dir.join(INSTANCE_ID_FILE)).unwrap(),
            first
        );
        fs::remove_dir_all(dir).unwrap();
    }

    #[test]
    fn instance_id_travels_with_the_data_folder() {
        let from = scratch("move-from");
        let id = load_or_create_instance_id(&from).unwrap();
        let to = scratch("move-to");
        fs::copy(from.join(INSTANCE_ID_FILE), to.join(INSTANCE_ID_FILE)).unwrap();
        assert_eq!(load_or_create_instance_id(&to).unwrap(), id);
        fs::remove_dir_all(from).unwrap();
        fs::remove_dir_all(to).unwrap();
    }

    #[test]
    fn separate_copies_get_different_ids() {
        let a = scratch("copy-a");
        let b = scratch("copy-b");
        assert_ne!(
            load_or_create_instance_id(&a).unwrap(),
            load_or_create_instance_id(&b).unwrap()
        );
        fs::remove_dir_all(a).unwrap();
        fs::remove_dir_all(b).unwrap();
    }

    #[test]
    fn damaged_instance_id_is_replaced() {
        let dir = scratch("damaged");
        fs::write(dir.join(INSTANCE_ID_FILE), "not an id").unwrap();
        let id = load_or_create_instance_id(&dir).unwrap();
        assert!(is_valid_instance_id(&id));
        assert_eq!(load_or_create_instance_id(&dir).unwrap(), id);
        // Hand-edited ids are accepted case-insensitively and with whitespace.
        fs::write(dir.join(INSTANCE_ID_FILE), " 0123456789ABCDEF\r\n").unwrap();
        assert_eq!(
            load_or_create_instance_id(&dir).unwrap(),
            "0123456789abcdef"
        );
        fs::remove_dir_all(dir).unwrap();
    }

    #[test]
    fn validates_instance_id_shape() {
        assert!(is_valid_instance_id("0123456789abcdef"));
        assert!(!is_valid_instance_id("0123456789ABCDEF"));
        assert!(!is_valid_instance_id("0123456789abcde"));
        assert!(!is_valid_instance_id("0123456789abcdeg"));
        assert!(!is_valid_instance_id("../../etc/passwd"));
    }

    #[test]
    fn prepare_portable_creates_the_data_folder() {
        let exe_dir = scratch("prepare");
        let layout = prepare_portable(&exe_dir).unwrap();
        assert_eq!(layout.data_dir, exe_dir.join("data"));
        assert!(layout.data_dir.is_dir());
        assert!(layout.bundled_webview2().is_none());
        // No probe file is left behind.
        let leftovers: Vec<_> = fs::read_dir(&layout.data_dir)
            .unwrap()
            .filter_map(|e| e.ok())
            .map(|e| e.file_name().to_string_lossy().into_owned())
            .filter(|name| name.starts_with(".write-test"))
            .collect();
        assert!(leftovers.is_empty(), "{leftovers:?}");
        fs::remove_dir_all(exe_dir).unwrap();
    }

    #[cfg(unix)]
    #[test]
    fn prepare_portable_fails_on_a_read_only_folder() {
        use std::os::unix::fs::PermissionsExt;
        let exe_dir = scratch("readonly");
        fs::set_permissions(&exe_dir, fs::Permissions::from_mode(0o555)).unwrap();
        let result = prepare_portable(&exe_dir);
        fs::set_permissions(&exe_dir, fs::Permissions::from_mode(0o755)).unwrap();
        // Root ignores permission bits; only assert when they are enforced.
        if !running_as_root() {
            let message = result.expect_err("a read-only folder must be refused");
            assert!(message.contains("data"), "{message}");
        }
        fs::remove_dir_all(exe_dir).unwrap();
    }

    #[cfg(unix)]
    fn running_as_root() -> bool {
        std::env::var("USER").map(|u| u == "root").unwrap_or(false)
    }

    #[test]
    fn portable_paths_stay_inside_the_data_folder() {
        let data = Path::new("/portable/DEEIX Chat/data");
        let paths = AppPaths::portable(data);
        assert_eq!(paths.config_dir, data);
        assert_eq!(paths.data_dir, data);
        assert_eq!(
            paths.webview_dir.as_deref(),
            Some(data.join("webview").as_path())
        );
        assert_eq!(paths.tmp_dir.as_deref(), Some(data.join("tmp").as_path()));
    }
}
