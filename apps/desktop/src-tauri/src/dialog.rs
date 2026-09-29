// Native error dialogs for failures before any window exists (portable data
// folder not writable, WebView2 missing). The process exits right after, so
// these must not depend on Tauri or the webview.

/// Show `message` and exit with status 1. With `link`, the dialog asks whether
/// to open it in the default browser first.
pub fn fatal(message: &str, link: Option<&str>) -> ! {
    eprintln!("[fatal] {message}");
    show(message, link);
    std::process::exit(1)
}

#[cfg(windows)]
fn show(message: &str, link: Option<&str>) {
    use windows_sys::Win32::UI::Shell::ShellExecuteW;
    use windows_sys::Win32::UI::WindowsAndMessaging::{
        MessageBoxW, IDYES, MB_ICONERROR, MB_OK, MB_SETFOREGROUND, MB_TOPMOST, MB_YESNO,
        SW_SHOWNORMAL,
    };

    const TITLE: &str = "DEEIX Chat";

    fn wide(s: &str) -> Vec<u16> {
        s.encode_utf16().chain(std::iter::once(0)).collect()
    }

    let text = match link {
        Some(url) => format!("{message}\n\n{url}\n\nOpen this page now?\n现在打开此页面吗？"),
        None => message.to_string(),
    };
    let buttons = if link.is_some() { MB_YESNO } else { MB_OK };
    let text = wide(&text);
    let title = wide(TITLE);
    // SAFETY: both strings are NUL-terminated UTF-16 buffers that outlive the call.
    let answer = unsafe {
        MessageBoxW(
            std::ptr::null_mut(),
            text.as_ptr(),
            title.as_ptr(),
            buttons | MB_ICONERROR | MB_SETFOREGROUND | MB_TOPMOST,
        )
    };
    if let (Some(url), IDYES) = (link, answer) {
        // Only ever called with our own https:// constants.
        let verb = wide("open");
        let url = wide(url);
        // SAFETY: NUL-terminated buffers; null hwnd/params/dir are allowed.
        unsafe {
            ShellExecuteW(
                std::ptr::null_mut(),
                verb.as_ptr(),
                url.as_ptr(),
                std::ptr::null(),
                std::ptr::null(),
                SW_SHOWNORMAL,
            );
        }
    }
}

#[cfg(not(windows))]
fn show(_message: &str, link: Option<&str>) {
    if let Some(url) = link {
        eprintln!("[fatal] see {url}");
    }
}
