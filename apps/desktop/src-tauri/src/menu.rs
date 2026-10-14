// Application menu (macOS). Tauri's default menu binds ⌘W to Close Window, which
// hides this tab container, and muda gives that predefined item a fixed
// accelerator. This menu mirrors the default without it and adds browser-style
// File ▸ Close Tab (⌘W) / Close Window (⇧⌘W).

use tauri::{
    menu::{
        AboutMetadata, Menu, MenuItem, PredefinedMenuItem, Submenu, HELP_SUBMENU_ID,
        WINDOW_SUBMENU_ID,
    },
    AppHandle, Manager, Runtime,
};

/// The ⌘W item. Reads "Close Tab", or "Close Window" while a single tab is open.
const CLOSE_TAB: &str = "close-tab";
/// The ⇧⌘W item: the window, with tabs still open. Absent with a single tab.
const CLOSE_WINDOW: &str = "close-window";
/// The submenu `sync` finds both items in.
const FILE_MENU: &str = "file";

const CLOSE_TAB_TEXT: &str = "Close Tab";
const CLOSE_WINDOW_TEXT: &str = "Close Window";

/// Build the app menu: `Menu::default` without Close Window, plus File. The
/// Window submenu keeps [`WINDOW_SUBMENU_ID`] so macOS still appends the window
/// list and tiling entries to it.
pub fn build<R: Runtime>(app: &AppHandle<R>) -> tauri::Result<Menu<R>> {
    let package = app.package_info();
    let config = app.config();
    let about = AboutMetadata {
        name: Some(package.name.clone()),
        version: Some(package.version.to_string()),
        copyright: config.bundle.copyright.clone(),
        authors: config.bundle.publisher.clone().map(|p| vec![p]),
        ..Default::default()
    };

    let app_menu = Submenu::with_items(
        app,
        package.name.clone(),
        true,
        &[
            &PredefinedMenuItem::about(app, None, Some(about))?,
            &PredefinedMenuItem::separator(app)?,
            &PredefinedMenuItem::services(app, None)?,
            &PredefinedMenuItem::separator(app)?,
            &PredefinedMenuItem::hide(app, None)?,
            &PredefinedMenuItem::hide_others(app, None)?,
            &PredefinedMenuItem::separator(app)?,
            &PredefinedMenuItem::quit(app, None)?,
        ],
    )?;

    let file_menu =
        Submenu::with_id_and_items(app, FILE_MENU, "File", true, &[&close_tab_item(app)?])?;

    let edit_menu = Submenu::with_items(
        app,
        "Edit",
        true,
        &[
            &PredefinedMenuItem::undo(app, None)?,
            &PredefinedMenuItem::redo(app, None)?,
            &PredefinedMenuItem::separator(app)?,
            &PredefinedMenuItem::cut(app, None)?,
            &PredefinedMenuItem::copy(app, None)?,
            &PredefinedMenuItem::paste(app, None)?,
            &PredefinedMenuItem::select_all(app, None)?,
        ],
    )?;

    let view_menu = Submenu::with_items(
        app,
        "View",
        true,
        &[&PredefinedMenuItem::fullscreen(app, None)?],
    )?;

    let window_menu = Submenu::with_id_and_items(
        app,
        WINDOW_SUBMENU_ID,
        "Window",
        true,
        &[
            &PredefinedMenuItem::minimize(app, None)?,
            &PredefinedMenuItem::maximize(app, None)?,
        ],
    )?;

    let help_menu = Submenu::with_id_and_items(app, HELP_SUBMENU_ID, "Help", true, &[])?;

    Menu::with_items(
        app,
        &[
            &app_menu,
            &file_menu,
            &edit_menu,
            &view_menu,
            &window_menu,
            &help_menu,
        ],
    )
}

/// ⌘W, File's first item: built with the menu, its text follows the tab count.
fn close_tab_item<R: Runtime>(app: &AppHandle<R>) -> tauri::Result<MenuItem<R>> {
    MenuItem::with_id(app, CLOSE_TAB, CLOSE_TAB_TEXT, true, Some("CmdOrCtrl+W"))
}

/// ⇧⌘W: added by `sync` while more than one tab is open. muda rebuilds the
/// native item whenever it is re-added, so the AppKit fix runs after each append.
fn close_window_item<R: Runtime>(app: &AppHandle<R>) -> tauri::Result<MenuItem<R>> {
    MenuItem::with_id(
        app,
        CLOSE_WINDOW,
        CLOSE_WINDOW_TEXT,
        true,
        Some("CmdOrCtrl+Shift+W"),
    )
}

/// Keep File in step with the tab count, as a browser does: with several tabs ⌘W
/// closes the active one and ⇧⌘W the window; with one, ⌘W is Close Window and
/// ⇧⌘W is removed. Called from `tabs::changed`.
pub fn sync<R: Runtime>(app: &AppHandle<R>) {
    let several = crate::tabs::snapshot(app).tabs.len() > 1;
    let Some(file) = file_menu(app) else {
        return;
    };

    let close_tab = file
        .get(CLOSE_TAB)
        .and_then(|item| item.as_menuitem().cloned());
    let close_window = file
        .get(CLOSE_WINDOW)
        .and_then(|item| item.as_menuitem().cloned());

    if let Some(item) = &close_tab {
        let text = if several {
            CLOSE_TAB_TEXT
        } else {
            CLOSE_WINDOW_TEXT
        };
        if item.text().unwrap_or_default() != text {
            let _ = item.set_text(text);
        }
    }
    match (several, close_window) {
        (true, None) => {
            if let Ok(item) = close_window_item(app) {
                let _ = file.append(&item);
                promote_shifted_shortcut();
            }
        }
        (false, Some(item)) => {
            let _ = file.remove(&item);
        }
        _ => {}
    }
}

/// The File submenu of the installed menu.
fn file_menu<R: Runtime>(app: &AppHandle<R>) -> Option<Submenu<R>> {
    app.menu()?
        .get(FILE_MENU)
        .and_then(|item| item.as_submenu().cloned())
}

/// Act on the items this module created. Other menus (the tray's) keep their
/// own handlers; their ids are ignored here.
pub fn on_event<R: Runtime>(app: &AppHandle<R>, id: &str) {
    match id {
        CLOSE_TAB => close_tab(app),
        CLOSE_WINDOW => close_window(app),
        _ => {}
    }
}

/// ⌘W: close the active tab. The last tab is kept — closing it would forget its
/// server (token deleted, sidecar stopped) — and the window closes instead, as in
/// a browser.
fn close_tab<R: Runtime>(app: &AppHandle<R>) {
    let snapshot = crate::tabs::snapshot(app);
    let Some(id) = snapshot.active.filter(|_| snapshot.tabs.len() > 1) else {
        return close_window(app);
    };
    // `tabs::close` drives webviews and the sidecar: off the main thread, as the
    // `tabs_close` command does.
    let app = app.clone();
    tauri::async_runtime::spawn(async move {
        if let Err(error) = crate::tabs::close(&app, &id).await {
            eprintln!("[menu] close tab {id}: {}", error.0);
        }
    });
}

/// Close the window: the request the red traffic light sends. `tabs::init`
/// hides instead of destroying it, and the tray brings it back.
fn close_window<R: Runtime>(app: &AppHandle<R>) {
    if let Some(window) = app.get_window(crate::tabs::WINDOW_LABEL) {
        let _ = window.close();
    }
}

/// AppKit matches ⇧⌘W against the shifted key equivalent "W", but muda turns a
/// `Cmd+Shift+W` accelerator into "w", so the item would show its shortcut and
/// never fire. Patch the installed item through AppKit.
fn promote_shifted_shortcut() {
    use objc2::runtime::AnyObject;
    use objc2::{class, msg_send};
    use objc2_foundation::NSString;

    /// NSEventModifierFlagShift.
    const SHIFT: usize = 1 << 17;

    unsafe {
        let application: *mut AnyObject = msg_send![class!(NSApplication), sharedApplication];
        let menu: *mut AnyObject = msg_send![application, mainMenu];
        if menu.is_null() {
            return;
        }
        let submenu_count: isize = msg_send![menu, numberOfItems];
        for submenu_index in 0..submenu_count {
            let submenu_item: *mut AnyObject = msg_send![menu, itemAtIndex: submenu_index];
            let submenu: *mut AnyObject = msg_send![submenu_item, submenu];
            if submenu.is_null() {
                continue;
            }
            let item_count: isize = msg_send![submenu, numberOfItems];
            for item_index in 0..item_count {
                let item: *mut AnyObject = msg_send![submenu, itemAtIndex: item_index];
                let modifiers: usize = msg_send![item, keyEquivalentModifierMask];
                if modifiers & SHIFT == 0 {
                    continue;
                }
                let key_equivalent: *mut AnyObject = msg_send![item, keyEquivalent];
                if key_equivalent.is_null() {
                    continue;
                }
                let key_equivalent = &*(key_equivalent as *const NSString);
                if key_equivalent.to_string() != "w" {
                    continue;
                }
                let shifted = NSString::from_str("W");
                let _: () = msg_send![item, setKeyEquivalent: &*shifted];
            }
        }
    }
}
