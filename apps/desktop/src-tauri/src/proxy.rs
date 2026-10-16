// Network proxy for everything that leaves the machine, resolved once per
// launch and handed to each consumer in the form it understands:
//
//   bundled server   HTTP_PROXY / HTTPS_PROXY / NO_PROXY on the child process
//   shell requests   reqwest clients: session refresh, update checks (`apply`)
//   webviews         WebView2 arguments (Windows); elsewhere they follow the OS
//
// Precedence: enterprise policy, then the app setting, then the process
// environment, then the OS. The bundled server keeps its own contract — proxy
// settings reach it only through the environment — and in local mode this
// shell is the one deploying it.
//
// Not applied, and reported instead: PAC scripts and WPAD, which no proxy
// address can express. A configured proxy that is unreachable is not bypassed:
// requests fail, as they do in the system browser, rather than leaving through
// a route the user or the administrator did not choose.

use std::fs;
use std::path::{Path, PathBuf};
use std::sync::{Mutex, OnceLock};

use keyring::Entry;
use serde::{Deserialize, Serialize};
use tauri::{AppHandle, Manager, Runtime};

use crate::distribution::Distribution;

/// Next to `tabs.json`.
const SETTINGS_FILE: &str = "network.json";
/// Keychain account for the manual proxy password; the service is the bundle
/// identifier, as for refresh tokens.
const KEYCHAIN_ACCOUNT: &str = "proxy-password";
/// Added to every bypass list handed out. Go, reqwest and the webviews never
/// proxy loopback anyway; listing it keeps the rule visible in logs.
const LOOPBACK_BYPASS: [&str; 3] = ["localhost", "127.0.0.0/8", "::1"];
/// wry's WebView2 defaults, which `additional_browser_args` replaces.
#[cfg_attr(not(windows), allow(dead_code))]
const WEBVIEW2_DISABLED_FEATURES: &str = "msWebOOUI,msPdfOOUI,msSmartScreenProtection";

/// Same prefix as the other `policy::*` texts, which the web app reports as
/// "managed by your organization".
pub const PROXY_MANAGED: &str = "policy: the network proxy is managed by your administrator";

// ---------- settings ----------

#[derive(Debug, Clone, Copy, PartialEq, Eq, Serialize, Deserialize, Default)]
#[serde(rename_all = "lowercase")]
pub enum Mode {
    /// The OS settings (the environment on Linux).
    #[default]
    System,
    /// The address configured in the app.
    Manual,
    /// Never use a proxy.
    Direct,
}

/// `https://` proxies are not offered: every client here dials the proxy
/// itself in clear text.
#[derive(Debug, Clone, Copy, PartialEq, Eq, Serialize, Deserialize, Default)]
#[serde(rename_all = "lowercase")]
pub enum Scheme {
    #[default]
    Http,
    Socks5,
}

impl Scheme {
    fn as_str(self) -> &'static str {
        match self {
            Scheme::Http => "http",
            Scheme::Socks5 => "socks5",
        }
    }
}

/// The app setting, as stored in `network.json`. The password is in the keychain.
#[derive(Debug, Clone, Default, PartialEq, Eq, Serialize, Deserialize)]
#[serde(rename_all = "camelCase", default)]
pub struct Settings {
    pub mode: Mode,
    pub scheme: Scheme,
    pub host: String,
    pub port: Option<u16>,
    pub username: String,
    /// Hosts, addresses and ranges that skip the proxy, separated by commas,
    /// semicolons or spaces.
    pub bypass: String,
}

impl Settings {
    /// `scheme://host:port`, or `None` while the address is incomplete or not
    /// a bare host.
    fn endpoint(&self) -> Option<String> {
        let host = self.host.trim();
        if host.is_empty() || host.contains(['/', '@', '?', '#', ' ']) {
            return None;
        }
        let host = match host.parse::<std::net::Ipv6Addr>() {
            Ok(ip) => format!("[{ip}]"),
            Err(_) => host.to_string(),
        };
        let port = self.port.filter(|port| *port != 0)?;
        parse_address(&format!("{}://{host}:{port}", self.scheme.as_str()))
    }
}

/// Why settings from the app were refused. `field` lets the page point at it.
#[derive(Debug, Serialize, PartialEq, Eq)]
#[serde(rename_all = "camelCase")]
pub struct ProxyError {
    /// "policy" | "invalid" | "storage"
    pub kind: &'static str,
    #[serde(skip_serializing_if = "Option::is_none")]
    pub field: Option<&'static str>,
    /// The offending input, so the page can name it in its own language.
    #[serde(skip_serializing_if = "Option::is_none")]
    pub value: Option<String>,
    pub message: String,
}

impl ProxyError {
    fn invalid(field: &'static str, message: impl Into<String>) -> Self {
        Self {
            kind: "invalid",
            field: Some(field),
            value: None,
            message: message.into(),
        }
    }

    fn storage(message: impl std::fmt::Display) -> Self {
        Self {
            kind: "storage",
            field: None,
            value: None,
            message: message.to_string(),
        }
    }
}

/// Trim, and refuse what would not work. Fields of a mode that is not selected
/// are kept as typed, so switching back restores them.
pub fn validate(settings: &Settings) -> Result<Settings, ProxyError> {
    let mut valid = settings.clone();
    valid.host = valid.host.trim().to_string();
    valid.username = valid.username.trim().to_string();
    valid.bypass = valid.bypass.trim().to_string();
    if valid.mode != Mode::Manual {
        return Ok(valid);
    }
    if !matches!(valid.port, Some(port) if port != 0) {
        return Err(ProxyError::invalid("port", "enter the proxy port"));
    }
    if valid.endpoint().is_none() {
        return Err(ProxyError::invalid(
            "host",
            "enter a host name or IP address without a scheme or path",
        ));
    }
    if let Some(entry) = split_list(&valid.bypass)
        .find(|entry| !is_special_bypass(entry) && normalize_bypass_entry(entry).is_none())
    {
        return Err(ProxyError {
            value: Some(entry.to_string()),
            ..ProxyError::invalid("bypass", format!("{entry} is not a host, address or range"))
        });
    }
    Ok(valid)
}

// ---------- resolution ----------

/// Which source decided the route. Reported to the settings page.
#[derive(Debug, Clone, Copy, PartialEq, Eq, Serialize)]
#[serde(rename_all = "lowercase")]
pub enum Source {
    Policy,
    Manual,
    Direct,
    Environment,
    System,
    /// Following the system, which has no proxy configured.
    None,
}

/// Proxy credentials. Kept out of `Route` so addresses can be logged and shown.
#[derive(Clone, PartialEq, Eq)]
pub struct Credentials {
    username: String,
    password: String,
}

impl std::fmt::Debug for Credentials {
    fn fmt(&self, f: &mut std::fmt::Formatter<'_>) -> std::fmt::Result {
        f.debug_struct("Credentials")
            .field("username", &self.username)
            .finish_non_exhaustive()
    }
}

/// A proxy address per destination scheme, and the hosts that skip it.
#[derive(Debug, Clone, Default, PartialEq, Eq)]
pub struct Route {
    pub http: Option<String>,
    pub https: Option<String>,
    /// NO_PROXY syntax, already normalised and including loopback.
    pub bypass: String,
}

impl Route {
    fn has_proxy(&self) -> bool {
        self.http.is_some() || self.https.is_some()
    }

    fn both(address: String, bypass: &str) -> Self {
        Self {
            http: Some(address.clone()),
            https: Some(address),
            bypass: normalized_bypass(bypass),
        }
    }
}

#[derive(Debug, Clone, PartialEq, Eq)]
pub struct Effective {
    pub source: Source,
    pub route: Route,
    credentials: Option<Credentials>,
    /// The OS points at a PAC script, which is not applied, and nothing else.
    pub pac_ignored: bool,
}

impl Effective {
    /// Connect directly.
    pub fn direct() -> Self {
        Self::new(Source::Direct, Route::default())
    }

    fn new(source: Source, route: Route) -> Self {
        Self {
            source,
            route,
            credentials: None,
            pac_ignored: false,
        }
    }

    /// The address with the credentials, for a client that needs them inline.
    fn address_with_credentials(&self, address: &str) -> String {
        let Some(credentials) = &self.credentials else {
            return address.to_string();
        };
        let Ok(mut url) = reqwest::Url::parse(address) else {
            return address.to_string();
        };
        // Percent-encodes, so any character is allowed in either field.
        let _ = url.set_username(&credentials.username);
        if !credentials.password.is_empty() {
            let _ = url.set_password(Some(&credentials.password));
        }
        url.to_string()
    }
}

/// Enterprise proxy policy; registry values in `policy.rs`.
#[derive(Debug, Clone, PartialEq, Eq)]
pub enum PolicyProxy {
    /// Follow the OS, whatever the app setting says.
    System,
    Direct,
    Fixed {
        address: String,
        bypass: String,
    },
}

/// Chrome's `ProxyMode` vocabulary, so existing administrator knowledge
/// carries over. An unusable combination is logged and treated as unset: a
/// typo must not take the network away.
pub fn policy_from_raw(mode: &str, server: &str, bypass: &str) -> Option<PolicyProxy> {
    match mode.trim().to_ascii_lowercase().as_str() {
        "" => None,
        "system" => Some(PolicyProxy::System),
        "direct" => Some(PolicyProxy::Direct),
        "fixed_servers" => match parse_address(server) {
            Some(address) => Some(PolicyProxy::Fixed {
                address,
                bypass: bypass.trim().to_string(),
            }),
            None => {
                eprintln!(
                    "[policy] ignoring ProxyMode fixed_servers: ProxyServer {server:?} is not \
                     host:port, http://host:port or socks5://host:port"
                );
                None
            }
        },
        other => {
            eprintln!("[policy] ignoring unknown ProxyMode {other:?}");
            None
        }
    }
}

/// Resolve the route for a launch. `system` is only consulted when the
/// policy or the setting defer to it.
fn resolve(
    policy: Option<&PolicyProxy>,
    settings: &Settings,
    password: Option<&str>,
    system: impl FnOnce() -> Effective,
) -> Effective {
    match policy {
        Some(PolicyProxy::Direct) => return Effective::new(Source::Policy, Route::default()),
        Some(PolicyProxy::Fixed { address, bypass }) => {
            return Effective::new(Source::Policy, Route::both(address.clone(), bypass));
        }
        Some(PolicyProxy::System) => return system(),
        None => {}
    }
    match settings.mode {
        Mode::Direct => Effective::direct(),
        Mode::System => system(),
        // Saved settings were validated; a hand-edited file that no longer
        // parses falls back to the OS rather than to an unintended direct route.
        Mode::Manual => match settings.endpoint() {
            Some(address) => Effective {
                credentials: (!settings.username.is_empty()).then(|| Credentials {
                    username: settings.username.clone(),
                    password: password.unwrap_or_default().to_string(),
                }),
                ..Effective::new(Source::Manual, Route::both(address, &settings.bypass))
            },
            None => {
                eprintln!("[proxy] manual proxy address is incomplete; following the system");
                system()
            }
        },
    }
}

/// Environment first (the curl and Go convention), then the OS.
fn system_proxy() -> Effective {
    if let Some(route) = environment_route(|name| std::env::var(name).ok()) {
        return Effective::new(Source::Environment, route);
    }
    match read_os_proxy() {
        Some(os) => os.effective(),
        None => Effective::new(Source::None, Route::default()),
    }
}

/// `HTTP_PROXY` / `HTTPS_PROXY`, with `ALL_PROXY` covering a scheme that has
/// none (curl's rule; Go itself ignores `ALL_PROXY`).
fn environment_route(var: impl Fn(&str) -> Option<String>) -> Option<Route> {
    let first = |names: &[&str]| {
        names
            .iter()
            .filter_map(|name| var(name))
            .map(|value| value.trim().to_string())
            .find(|value| !value.is_empty())
    };
    let all = first(&["ALL_PROXY", "all_proxy"]);
    let address = |names: &[&str]| {
        let raw = first(names).or_else(|| all.clone())?;
        let parsed = parse_address(&raw);
        if parsed.is_none() {
            eprintln!("[proxy] ignoring {}: not a proxy address", names[0]);
        }
        parsed
    };
    let http = address(&["HTTP_PROXY", "http_proxy"]);
    let https = address(&["HTTPS_PROXY", "https_proxy"]);
    if http.is_none() && https.is_none() {
        return None;
    }
    Some(Route {
        http,
        https,
        bypass: normalized_bypass(&first(&["NO_PROXY", "no_proxy"]).unwrap_or_default()),
    })
}

// ---------- the OS ----------

/// The OS proxy settings, before translation.
#[derive(Debug, Clone, Default, PartialEq, Eq)]
struct OsProxy {
    http: Option<String>,
    https: Option<String>,
    socks: Option<String>,
    bypass: String,
    /// A PAC script URL is set.
    pac: bool,
}

impl OsProxy {
    /// A scheme without its own proxy goes through the SOCKS one, as in WinINet
    /// and CFNetwork.
    fn effective(self) -> Effective {
        let http = self.http.or_else(|| self.socks.clone());
        let https = self.https.or(self.socks);
        let route = Route {
            http: http.as_deref().and_then(parse_address),
            https: https.as_deref().and_then(parse_address),
            bypass: normalized_bypass(&self.bypass),
        };
        let source = match route.has_proxy() {
            true => Source::System,
            false => Source::None,
        };
        Effective {
            pac_ignored: self.pac && !route.has_proxy(),
            ..Effective::new(source, route)
        }
    }
}

/// WinHTTP's proxy string: one `host:port` for every scheme, or
/// `http=host:port;https=host:port;socks=host:port` in any order or subset.
#[cfg_attr(not(windows), allow(dead_code))]
fn parse_windows_proxy(value: &str, bypass: &str, pac: bool) -> OsProxy {
    let mut os = OsProxy {
        bypass: bypass.to_string(),
        pac,
        ..OsProxy::default()
    };
    for entry in value
        .split([';', ' ', '\t'])
        .map(str::trim)
        .filter(|entry| !entry.is_empty())
    {
        match entry.split_once('=') {
            Some((scheme, address)) => {
                let address = address.trim();
                if address.is_empty() {
                    continue;
                }
                match scheme.trim().to_ascii_lowercase().as_str() {
                    "http" => os.http = Some(with_scheme(address, "http")),
                    "https" => os.https = Some(with_scheme(address, "http")),
                    // WinINet means SOCKS4 here, which no client of ours
                    // speaks; the proxies people run accept SOCKS5 on the same
                    // port.
                    "socks" => os.socks = Some(with_scheme(address, "socks5")),
                    _ => {}
                }
            }
            None => {
                os.http = Some(with_scheme(entry, "http"));
                os.https = Some(with_scheme(entry, "http"));
            }
        }
    }
    os
}

fn with_scheme(address: &str, scheme: &str) -> String {
    match address.contains("://") {
        true => address.to_string(),
        false => format!("{scheme}://{address}"),
    }
}

/// Internet Options, through the documented WinHTTP call rather than the
/// registry layout behind it.
#[cfg(windows)]
fn read_os_proxy() -> Option<OsProxy> {
    use windows_sys::Win32::Foundation::GlobalFree;
    use windows_sys::Win32::Networking::WinHttp::{
        WinHttpGetIEProxyConfigForCurrentUser, WINHTTP_CURRENT_USER_IE_PROXY_CONFIG,
    };

    let mut config = WINHTTP_CURRENT_USER_IE_PROXY_CONFIG::default();
    // SAFETY: `config` is a valid out-parameter; on success every non-null
    // string is a NUL-terminated GlobalAlloc buffer that we free below.
    let ok = unsafe { WinHttpGetIEProxyConfigForCurrentUser(&mut config) } != 0;
    let os = ok.then(|| {
        parse_windows_proxy(
            &wide_to_string(config.lpszProxy),
            &wide_to_string(config.lpszProxyBypass),
            !config.lpszAutoConfigUrl.is_null(),
        )
    });
    for pointer in [
        config.lpszAutoConfigUrl,
        config.lpszProxy,
        config.lpszProxyBypass,
    ] {
        if !pointer.is_null() {
            // SAFETY: allocated by WinHTTP with GlobalAlloc, freed exactly once.
            unsafe { GlobalFree(pointer.cast()) };
        }
    }
    os
}

#[cfg(windows)]
fn wide_to_string(pointer: *const u16) -> String {
    if pointer.is_null() {
        return String::new();
    }
    // SAFETY: WinHTTP returns NUL-terminated UTF-16 strings.
    unsafe {
        let mut len = 0;
        while *pointer.add(len) != 0 {
            len += 1;
        }
        String::from_utf16_lossy(std::slice::from_raw_parts(pointer, len))
    }
}

/// System Settings › Network › Proxies, through the dynamic store.
#[cfg(target_os = "macos")]
fn read_os_proxy() -> Option<OsProxy> {
    use system_configuration::core_foundation::array::CFArray;
    use system_configuration::core_foundation::base::{CFType, TCFType};
    use system_configuration::core_foundation::number::CFNumber;
    use system_configuration::core_foundation::string::{CFString, CFStringRef};
    use system_configuration::dynamic_store::SCDynamicStoreBuilder;
    use system_configuration::sys::schema_definitions::{
        kSCPropNetProxiesExceptionsList, kSCPropNetProxiesExcludeSimpleHostnames,
        kSCPropNetProxiesHTTPEnable, kSCPropNetProxiesHTTPPort, kSCPropNetProxiesHTTPProxy,
        kSCPropNetProxiesHTTPSEnable, kSCPropNetProxiesHTTPSPort, kSCPropNetProxiesHTTPSProxy,
        kSCPropNetProxiesProxyAutoConfigEnable, kSCPropNetProxiesProxyAutoConfigURLString,
        kSCPropNetProxiesSOCKSEnable, kSCPropNetProxiesSOCKSPort, kSCPropNetProxiesSOCKSProxy,
    };

    let store = SCDynamicStoreBuilder::new("deeix-chat-proxy").build()?;
    let map = store.get_proxies()?;
    let text = |key: CFStringRef| {
        map.find(key)
            .and_then(|value| value.downcast::<CFString>())
            .map(|value| value.to_string())
    };
    let number = |key: CFStringRef| {
        map.find(key)
            .and_then(|value| value.downcast::<CFNumber>())
            .and_then(|value| value.to_i32())
    };
    let endpoint = |enable: CFStringRef, host: CFStringRef, port: CFStringRef, scheme: &str| {
        if number(enable) != Some(1) {
            return None;
        }
        let host = text(host)?;
        Some(match number(port) {
            Some(port) => format!("{scheme}://{host}:{port}"),
            None => format!("{scheme}://{host}"),
        })
    };

    // SAFETY: the schema keys are immutable CFStrings exported by the framework.
    unsafe {
        let mut bypass: Vec<String> = Vec::new();
        if let Some(entries) = map
            .find(kSCPropNetProxiesExceptionsList)
            .and_then(|value| value.downcast::<CFArray>())
        {
            for entry in entries.iter() {
                let entry = CFType::wrap_under_get_rule(*entry as _);
                if let Some(entry) = entry.downcast::<CFString>() {
                    bypass.push(entry.to_string());
                }
            }
        }
        if number(kSCPropNetProxiesExcludeSimpleHostnames) == Some(1) {
            // Dotless names such as "nas", which NO_PROXY cannot express; kept
            // so the drop is visible in one place (`normalize_bypass_entry`).
            bypass.push("<local>".to_string());
        }
        Some(OsProxy {
            http: endpoint(
                kSCPropNetProxiesHTTPEnable,
                kSCPropNetProxiesHTTPProxy,
                kSCPropNetProxiesHTTPPort,
                "http",
            ),
            https: endpoint(
                kSCPropNetProxiesHTTPSEnable,
                kSCPropNetProxiesHTTPSProxy,
                kSCPropNetProxiesHTTPSPort,
                "http",
            ),
            socks: endpoint(
                kSCPropNetProxiesSOCKSEnable,
                kSCPropNetProxiesSOCKSProxy,
                kSCPropNetProxiesSOCKSPort,
                "socks5",
            ),
            bypass: bypass.join(","),
            pac: number(kSCPropNetProxiesProxyAutoConfigEnable) == Some(1)
                && text(kSCPropNetProxiesProxyAutoConfigURLString).is_some(),
        })
    }
}

/// Linux has no proxy service: the environment is the convention there, and
/// WebKitGTK follows the desktop's own settings.
#[cfg(not(any(windows, target_os = "macos")))]
fn read_os_proxy() -> Option<OsProxy> {
    None
}

// ---------- addresses and bypass lists ----------

/// `host:port`, `http://host:port` or `socks5://host:port`, canonicalised.
/// `None` for anything else, including addresses with credentials or a path.
fn parse_address(raw: &str) -> Option<String> {
    let url = reqwest::Url::parse(&with_scheme(raw.trim(), "http")).ok()?;
    let scheme = url.scheme();
    if !matches!(scheme, "http" | "socks5")
        || !url.username().is_empty()
        || url.password().is_some()
    {
        return None;
    }
    if !matches!(url.path(), "" | "/") || url.query().is_some() || url.fragment().is_some() {
        return None;
    }
    let host = url.host_str().filter(|host| !host.is_empty())?;
    // SOCKS has no default port.
    let port = url.port_or_known_default()?;
    Some(format!("{scheme}://{host}:{port}"))
}

fn split_list(raw: &str) -> impl Iterator<Item = &str> {
    raw.split([',', ';', ' ', '\t', '\n', '\r'])
        .map(str::trim)
        .filter(|entry| !entry.is_empty())
}

/// `<local>` (dotless hosts) and `<-loopback>` belong to WinINet and Chromium
/// and have no NO_PROXY form. Dropped, not rejected: loopback is never proxied
/// here, and the OS settings are allowed to contain them.
fn is_special_bypass(entry: &str) -> bool {
    entry.starts_with('<') && entry.ends_with('>')
}

/// Rewrite a list from Windows (`;`), macOS (`,`) or NO_PROXY into the form Go,
/// reqwest and Chromium all read, with loopback added.
fn normalized_bypass(raw: &str) -> String {
    let mut entries: Vec<String> = Vec::new();
    for entry in split_list(raw).filter_map(normalize_bypass_entry) {
        if !entries.contains(&entry) {
            entries.push(entry);
        }
    }
    for loopback in LOOPBACK_BYPASS {
        if !entries.iter().any(|entry| entry == loopback) {
            entries.push(loopback.to_string());
        }
    }
    entries.join(",")
}

fn normalize_bypass_entry(entry: &str) -> Option<String> {
    if is_special_bypass(entry) {
        return None;
    }
    let entry = entry
        .strip_prefix("http://")
        .or_else(|| entry.strip_prefix("https://"))
        .unwrap_or(entry)
        .trim_end_matches('/')
        .to_ascii_lowercase();
    if entry.is_empty() || entry.contains('@') {
        return None;
    }
    if entry == "*" {
        return Some(entry);
    }
    if let Some((base, prefix)) = entry.split_once('/') {
        if let (Ok(ip), Ok(prefix)) = (base.parse::<std::net::IpAddr>(), prefix.parse::<u8>()) {
            let bits = if ip.is_ipv4() { 32 } else { 128 };
            return (prefix <= bits).then(|| format!("{ip}/{prefix}"));
        }
        return short_cidr(&entry);
    }
    if let Some(cidr) = wildcard_cidr(&entry) {
        return Some(cidr);
    }
    if entry.parse::<std::net::IpAddr>().is_ok() {
        return Some(entry);
    }
    // A host name, optionally with a port. "*.corp.example" and ".corp.example"
    // both mean "every subdomain"; a bare "*corp.example" is read the same way.
    let (host, port) = match entry.rsplit_once(':') {
        Some((host, port)) if !port.is_empty() && port.chars().all(|c| c.is_ascii_digit()) => {
            (host, Some(port))
        }
        Some(_) => return None,
        None => (entry.as_str(), None),
    };
    let domain = match host.strip_prefix("*.") {
        Some(rest) => format!(".{rest}"),
        None => host.trim_start_matches('*').to_string(),
    };
    let name = domain.trim_start_matches('.');
    let valid = !name.is_empty()
        && name
            .chars()
            .all(|c| c.is_ascii_alphanumeric() || c == '-' || c == '.');
    valid.then(|| match port {
        Some(port) => format!("{domain}:{port}"),
        None => domain,
    })
}

/// `10.*`, `192.168.*`, `172.16.*.*`: Windows writes ranges as trailing
/// wildcards, which Go's `httpproxy` and hyper-util read as domain names.
fn wildcard_cidr(entry: &str) -> Option<String> {
    let parts: Vec<&str> = entry.split('.').collect();
    let fixed: Vec<u8> = parts
        .iter()
        .take_while(|part| **part != "*")
        .map(|part| part.parse().ok())
        .collect::<Option<_>>()?;
    let wildcards = &parts[fixed.len()..];
    if fixed.is_empty() || fixed.len() > 3 || wildcards.is_empty() || parts.len() > 4 {
        return None;
    }
    if wildcards.iter().any(|part| *part != "*") {
        return None;
    }
    let mut octets = [0u8; 4];
    octets[..fixed.len()].copy_from_slice(&fixed);
    Some(format!(
        "{}/{}",
        std::net::Ipv4Addr::from(octets),
        fixed.len() * 8
    ))
}

/// `169.254/16`: macOS writes CIDR blocks without their trailing zero octets.
fn short_cidr(entry: &str) -> Option<String> {
    let (base, prefix) = entry.split_once('/')?;
    let prefix: u8 = prefix.parse().ok().filter(|prefix| *prefix <= 32)?;
    let parts: Vec<u8> = base
        .split('.')
        .map(|part| part.parse().ok())
        .collect::<Option<_>>()?;
    if parts.is_empty() || parts.len() > 4 {
        return None;
    }
    let mut octets = [0u8; 4];
    octets[..parts.len()].copy_from_slice(&parts);
    Some(format!("{}/{prefix}", std::net::Ipv4Addr::from(octets)))
}

// ---------- consumers ----------

/// Whether a destination is this machine, which is never proxied.
pub fn is_loopback(url: &str) -> bool {
    let Ok(url) = reqwest::Url::parse(url) else {
        return false;
    };
    match url.host_str() {
        Some("localhost") => true,
        Some(host) => host
            .trim_start_matches('[')
            .trim_end_matches(']')
            .parse::<std::net::IpAddr>()
            .is_ok_and(|ip| ip.is_loopback()),
        None => false,
    }
}

/// Give a reqwest client exactly this route. reqwest would otherwise read the
/// OS settings itself (the updater plugin enables that feature), with its own
/// bypass rules.
pub fn apply(builder: reqwest::ClientBuilder, effective: &Effective) -> reqwest::ClientBuilder {
    let mut builder = builder.no_proxy();
    let no_proxy = reqwest::NoProxy::from_string(&effective.route.bypass);
    let proxies = [
        (&effective.route.http, Destination::Http),
        (&effective.route.https, Destination::Https),
    ];
    for (address, destination) in proxies {
        let Some(address) = address else {
            continue;
        };
        let url = effective.address_with_credentials(address);
        let proxy = match destination {
            Destination::Http => reqwest::Proxy::http(url),
            Destination::Https => reqwest::Proxy::https(url),
        };
        match proxy {
            Ok(proxy) => builder = builder.proxy(proxy.no_proxy(no_proxy.clone())),
            // Addresses are canonicalised on the way in; this is a bug, and
            // connecting directly is the only thing left to do.
            Err(e) => eprintln!("[proxy] cannot use {address}: {e}"),
        }
    }
    builder
}

/// Which destinations a proxy entry is for: Windows can name a different proxy
/// for each.
#[derive(Clone, Copy)]
enum Destination {
    Http,
    Https,
}

/// Variables for a child process. All spellings are set, empty ones included,
/// so nothing inherited from this process survives: Go reads the upper-case
/// name and falls back to the lower-case one, and on Windows both are one
/// variable.
pub fn child_env(effective: &Effective) -> Vec<(&'static str, String)> {
    let address = |address: &Option<String>| {
        address
            .as_deref()
            .map(|address| effective.address_with_credentials(address))
            .unwrap_or_default()
    };
    let http = address(&effective.route.http);
    let https = address(&effective.route.https);
    let no_proxy = match effective.route.has_proxy() {
        true => effective.route.bypass.clone(),
        false => String::new(),
    };
    vec![
        ("HTTP_PROXY", http.clone()),
        ("http_proxy", http),
        ("HTTPS_PROXY", https.clone()),
        ("https_proxy", https),
        ("NO_PROXY", no_proxy.clone()),
        ("no_proxy", no_proxy),
        // Go ignores it, but the backend's startup log reports it.
        ("ALL_PROXY", String::new()),
        ("all_proxy", String::new()),
    ]
}

/// How the webviews are created. Fixed for the launch: WebView2 requires every
/// webview sharing a profile to share its environment options, and none of the
/// engines change proxy after creation.
#[derive(Debug, Clone, PartialEq, Eq)]
pub enum WebviewProxy {
    /// The engine's own handling of the OS settings, PAC included.
    System,
    Fixed {
        address: String,
        bypass: String,
    },
    Direct,
}

impl WebviewProxy {
    /// The webview counterpart of `resolve`. The environment does not count:
    /// WebView2 and WebKit read the OS settings, not variables.
    fn resolve(policy: Option<&PolicyProxy>, settings: &Settings) -> Self {
        let fixed = |address: String, bypass: &str| WebviewProxy::Fixed {
            address,
            // NO_PROXY form; each consumer converts it (see `bypass_list`).
            bypass: normalized_bypass(bypass),
        };
        match policy {
            Some(PolicyProxy::Direct) => WebviewProxy::Direct,
            Some(PolicyProxy::Fixed { address, bypass }) => fixed(address.clone(), bypass),
            Some(PolicyProxy::System) => WebviewProxy::System,
            None => match settings.mode {
                Mode::System => WebviewProxy::System,
                Mode::Direct => WebviewProxy::Direct,
                Mode::Manual => match settings.endpoint() {
                    Some(address) => fixed(address, &settings.bypass),
                    None => WebviewProxy::System,
                },
            },
        }
    }

    /// Bypass list for the webview: Windows and Chromium write `;`, and the
    /// loopback entries Chromium already handles are left out.
    #[cfg_attr(not(windows), allow(dead_code))]
    fn bypass_list(&self) -> String {
        match self {
            WebviewProxy::Fixed { bypass, .. } => split_list(bypass)
                .filter(|entry| !LOOPBACK_BYPASS.contains(entry))
                .collect::<Vec<_>>()
                .join(";"),
            _ => String::new(),
        }
    }

    /// Command-line arguments for a Chromium-based webview (Windows). Empty
    /// means "leave the engine alone".
    #[cfg_attr(not(windows), allow(dead_code))]
    fn chromium_args(&self) -> String {
        match self {
            WebviewProxy::System => String::new(),
            WebviewProxy::Direct => {
                format!("--disable-features={WEBVIEW2_DISABLED_FEATURES} --no-proxy-server")
            }
            WebviewProxy::Fixed { address, .. } => {
                let mut args = format!(
                    "--disable-features={WEBVIEW2_DISABLED_FEATURES} --proxy-server={address}"
                );
                let bypass = self.bypass_list();
                if !bypass.is_empty() {
                    args.push_str(&format!(" --proxy-bypass-list={bypass}"));
                }
                args
            }
        }
    }

    /// Apply to a webview builder. Every webview of the app goes through here.
    pub fn configure<R: Runtime>(
        &self,
        builder: tauri::webview::WebviewBuilder<R>,
    ) -> tauri::webview::WebviewBuilder<R> {
        self.platform_configure(builder)
    }

    /// Windows: WebView2 takes Chromium arguments, and `wry` then applies none
    /// of its own defaults, so `chromium_args` repeats them.
    #[cfg(windows)]
    fn platform_configure<R: Runtime>(
        &self,
        builder: tauri::webview::WebviewBuilder<R>,
    ) -> tauri::webview::WebviewBuilder<R> {
        match self {
            WebviewProxy::System => builder,
            _ => builder.additional_browser_args(&self.chromium_args()),
        }
    }

    /// macOS and Linux: the interface follows the OS. A per-webview proxy needs
    /// macOS 14 (this app supports 10.15), and Tauri hands WebKitGTK a proxy
    /// with no bypass list at all, which would send the local server's own
    /// traffic to the proxy. The settings page reports it.
    #[cfg(not(windows))]
    fn platform_configure<R: Runtime>(
        &self,
        builder: tauri::webview::WebviewBuilder<R>,
    ) -> tauri::webview::WebviewBuilder<R> {
        builder
    }

    fn as_str(&self) -> &'static str {
        match self {
            WebviewProxy::System => "system",
            WebviewProxy::Fixed { .. } => "fixed",
            WebviewProxy::Direct => "direct",
        }
    }

    /// What the webviews of this platform can actually take. Neither engine
    /// accepts proxy credentials up front, and Tauri forwards no sign-in
    /// prompt, so an authenticating proxy would break every page load; those
    /// and the settings a platform cannot express leave the webviews on the OS
    /// settings, and the settings page says so.
    fn for_platform(self, needs_credentials: bool) -> Self {
        let supported = match &self {
            WebviewProxy::System => true,
            WebviewProxy::Fixed { .. } => cfg!(windows) && !needs_credentials,
            WebviewProxy::Direct => cfg!(windows),
        };
        match supported {
            true => self,
            false => WebviewProxy::System,
        }
    }
}

// ---------- state ----------

/// Read once at startup, like enterprise policy: the sidecar and the webviews
/// take their proxy when they are created, so a saved change applies to the
/// next launch, and the settings page says so.
pub struct ProxyState {
    file: PathBuf,
    identifier: String,
    active: Settings,
    password: Option<String>,
    webview: WebviewProxy,
    /// The webviews could not take what the policy or the setting asks for.
    webview_follows_system: bool,
    saved: Mutex<Saved>,
    system: OnceLock<Effective>,
}

struct Saved {
    settings: Settings,
    password_changed: bool,
}

impl ProxyState {
    pub fn load(config_dir: &Path, identifier: &str, policy: Option<&PolicyProxy>) -> Self {
        let file = config_dir.join(SETTINGS_FILE);
        let settings = read_settings(&file);
        // Only touch the keychain when the password would be used.
        let password = (settings.mode == Mode::Manual && !settings.username.is_empty())
            .then(|| stored_password(identifier))
            .flatten();
        let wanted = WebviewProxy::resolve(policy, &settings);
        let needs_credentials =
            policy.is_none() && settings.mode == Mode::Manual && !settings.username.is_empty();
        let webview = wanted.clone().for_platform(needs_credentials);
        eprintln!(
            "[proxy] setting={:?} webview={}",
            settings.mode,
            webview.as_str()
        );
        Self {
            file,
            identifier: identifier.to_string(),
            active: settings.clone(),
            password,
            webview_follows_system: webview != wanted,
            webview,
            saved: Mutex::new(Saved {
                settings,
                password_changed: false,
            }),
            system: OnceLock::new(),
        }
    }

    /// The route for this launch.
    pub fn effective(&self, policy: Option<&PolicyProxy>) -> Effective {
        resolve(policy, &self.active, self.password.as_deref(), || {
            self.system.get_or_init(system_proxy).clone()
        })
    }

    pub fn webview(&self) -> &WebviewProxy {
        &self.webview
    }

    fn status(&self, policy: Option<&PolicyProxy>) -> ProxyStatus {
        let saved = self.saved.lock().unwrap_or_else(|e| e.into_inner());
        let effective = self.effective(policy);
        let uses_system = matches!(
            effective.source,
            Source::System | Source::None | Source::Environment
        );
        ProxyStatus {
            settings: saved.settings.clone(),
            has_password: !saved.settings.username.is_empty()
                && stored_password(&self.identifier).is_some(),
            managed_by_policy: policy.is_some(),
            restart_required: saved.password_changed || saved.settings != self.active,
            // The OS settings were read at startup; tell the user when they
            // have moved since.
            system_changed: uses_system
                && system_proxy() != effective_without_credentials(&effective),
            effective: EffectiveView {
                source: effective.source,
                http: effective.route.http.clone(),
                https: effective.route.https.clone(),
                bypass: effective.route.bypass.clone(),
                pac_ignored: effective.pac_ignored,
                webview: self.webview.as_str(),
                webview_follows_system: self.webview_follows_system,
            },
        }
    }
}

fn effective_without_credentials(effective: &Effective) -> Effective {
    Effective {
        credentials: None,
        ..effective.clone()
    }
}

fn read_settings(file: &Path) -> Settings {
    let Ok(bytes) = fs::read(file) else {
        return Settings::default();
    };
    serde_json::from_slice(&bytes).unwrap_or_else(|e| {
        eprintln!("[proxy] ignoring unreadable {}: {e}", file.display());
        Settings::default()
    })
}

/// Atomic, like `tabs.json`.
fn write_settings(file: &Path, settings: &Settings) -> std::io::Result<()> {
    if let Some(dir) = file.parent() {
        fs::create_dir_all(dir)?;
    }
    let tmp = file.with_extension("json.tmp");
    fs::write(&tmp, serde_json::to_vec_pretty(settings)?)?;
    fs::rename(tmp, file)
}

fn entry(identifier: &str) -> keyring::Result<Entry> {
    Entry::new(identifier, KEYCHAIN_ACCOUNT)
}

fn stored_password(identifier: &str) -> Option<String> {
    match entry(identifier).and_then(|entry| entry.get_password()) {
        Ok(password) => Some(password),
        Err(keyring::Error::NoEntry) => None,
        Err(e) => {
            eprintln!("[proxy] cannot read the proxy password: {e}");
            None
        }
    }
}

// ---------- commands ----------

/// `get_network_proxy` payload.
#[derive(Debug, Serialize)]
#[serde(rename_all = "camelCase")]
pub struct ProxyStatus {
    /// As saved; may differ from what this launch uses (`restart_required`).
    pub settings: Settings,
    pub has_password: bool,
    pub managed_by_policy: bool,
    pub restart_required: bool,
    pub system_changed: bool,
    pub effective: EffectiveView,
}

/// The route in force, without credentials.
#[derive(Debug, Serialize)]
#[serde(rename_all = "camelCase")]
pub struct EffectiveView {
    pub source: Source,
    pub http: Option<String>,
    pub https: Option<String>,
    pub bypass: String,
    pub pac_ignored: bool,
    /// "system" | "fixed" | "direct": what the webviews were created with.
    pub webview: &'static str,
    /// The webviews could not take the policy or the setting (platform limits,
    /// or a proxy that needs a password), so the window follows the OS.
    pub webview_follows_system: bool,
}

/// What the settings page sends.
#[derive(Debug, Deserialize)]
#[serde(rename_all = "camelCase")]
pub struct ProxyUpdate {
    pub settings: Settings,
    /// Absent keeps the stored password; empty removes it.
    #[serde(default)]
    pub password: Option<String>,
}

/// The route in force, for anything that needs it outside a command.
pub fn effective<R: Runtime>(app: &AppHandle<R>) -> Effective {
    let policy = app.state::<Distribution>().policy.proxy.clone();
    app.state::<ProxyState>().effective(policy.as_ref())
}

#[tauri::command]
pub fn get_network_proxy<R: Runtime>(app: AppHandle<R>) -> ProxyStatus {
    let policy = app.state::<Distribution>().policy.proxy.clone();
    app.state::<ProxyState>().status(policy.as_ref())
}

/// Save the setting; it applies from the next launch.
#[tauri::command]
pub fn set_network_proxy<R: Runtime>(
    app: AppHandle<R>,
    update: ProxyUpdate,
) -> Result<ProxyStatus, ProxyError> {
    let policy = app.state::<Distribution>().policy.proxy.clone();
    if policy.is_some() {
        return Err(ProxyError {
            kind: "policy",
            field: None,
            value: None,
            message: PROXY_MANAGED.to_string(),
        });
    }
    let state = app.state::<ProxyState>();
    let settings = validate(&update.settings)?;
    // Without a username there is nothing for a password to authenticate;
    // drop it rather than keep a dormant credential, as refresh tokens are.
    let password = match settings.username.is_empty() {
        true => Some(String::new()),
        false => update.password,
    };
    let mut saved = state.saved.lock().unwrap_or_else(|e| e.into_inner());
    match password.as_deref() {
        None => {}
        Some("") => match entry(&state.identifier).and_then(|entry| entry.delete_credential()) {
            Ok(()) => saved.password_changed |= state.password.is_some(),
            Err(keyring::Error::NoEntry) => {}
            Err(e) => return Err(ProxyError::storage(e)),
        },
        Some(password) => {
            entry(&state.identifier)
                .and_then(|entry| entry.set_password(password))
                .map_err(ProxyError::storage)?;
            saved.password_changed |= state.password.as_deref() != Some(password);
        }
    }
    write_settings(&state.file, &settings).map_err(ProxyError::storage)?;
    saved.settings = settings;
    drop(saved);
    Ok(state.status(policy.as_ref()))
}

#[cfg(test)]
mod tests {
    use super::*;

    fn manual(host: &str, port: Option<u16>) -> Settings {
        Settings {
            mode: Mode::Manual,
            host: host.into(),
            port,
            ..Settings::default()
        }
    }

    fn system(http: &str) -> Effective {
        Effective::new(
            Source::System,
            Route {
                http: Some(http.into()),
                https: Some(http.into()),
                bypass: normalized_bypass(""),
            },
        )
    }

    #[test]
    fn windows_proxy_strings() {
        let single = parse_windows_proxy("127.0.0.1:7890", "", false).effective();
        assert_eq!(single.route.http.as_deref(), Some("http://127.0.0.1:7890"));
        assert_eq!(single.route.https.as_deref(), Some("http://127.0.0.1:7890"));
        assert_eq!(single.source, Source::System);

        let split = parse_windows_proxy(
            "http=proxy.corp:8080;https=secure.corp:8443;ftp=x:21",
            "",
            false,
        )
        .effective();
        assert_eq!(split.route.http.as_deref(), Some("http://proxy.corp:8080"));
        assert_eq!(
            split.route.https.as_deref(),
            Some("http://secure.corp:8443")
        );

        // A scheme without its own entry falls back to SOCKS.
        let socks =
            parse_windows_proxy("socks=127.0.0.1:1080;http=127.0.0.1:8080", "", false).effective();
        assert_eq!(socks.route.http.as_deref(), Some("http://127.0.0.1:8080"));
        assert_eq!(
            socks.route.https.as_deref(),
            Some("socks5://127.0.0.1:1080")
        );
    }

    #[test]
    fn pac_alone_is_reported_not_applied() {
        let pac = parse_windows_proxy("", "", true).effective();
        assert_eq!(pac.source, Source::None);
        assert!(pac.pac_ignored);
        // With a static proxy as well, the static one is used.
        let both = parse_windows_proxy("127.0.0.1:7890", "", true).effective();
        assert!(!both.pac_ignored);
        assert!(both.route.has_proxy());
    }

    #[test]
    fn clash_style_bypass_list_becomes_cidr() {
        // What Clash for Windows and v2rayN write into ProxyOverride.
        let bypass =
            normalized_bypass("localhost;127.*;10.*;172.16.*;172.31.*;192.168.*;*.local;<local>");
        assert_eq!(
            bypass,
            "localhost,127.0.0.0/8,10.0.0.0/8,172.16.0.0/16,172.31.0.0/16,192.168.0.0/16,.local,::1"
        );
        // Both consumers agree that 127.0.0.1 and LAN hosts skip the proxy.
        let proxy = reqwest::NoProxy::from_string(&bypass);
        assert!(proxy.is_some());
    }

    #[test]
    fn macos_exceptions_and_short_cidr() {
        assert_eq!(
            normalized_bypass("*.local, 169.254/16, 10.0.0.0/8"),
            ".local,169.254.0.0/16,10.0.0.0/8,localhost,127.0.0.0/8,::1"
        );
    }

    #[test]
    fn bypass_entry_edge_cases() {
        assert_eq!(normalize_bypass_entry("*"), Some("*".into()));
        assert_eq!(
            normalize_bypass_entry("10.1.2.*"),
            Some("10.1.2.0/24".into())
        );
        assert_eq!(
            normalize_bypass_entry("10.*.*.*"),
            Some("10.0.0.0/8".into())
        );
        assert_eq!(
            normalize_bypass_entry("CORP.example"),
            Some("corp.example".into())
        );
        assert_eq!(
            normalize_bypass_entry("http://intranet/"),
            Some("intranet".into())
        );
        assert_eq!(normalize_bypass_entry("nas:5000"), Some("nas:5000".into()));
        assert_eq!(
            normalize_bypass_entry("fe80::/10"),
            Some("fe80::/10".into())
        );
        assert_eq!(
            normalize_bypass_entry("2001:db8::1"),
            Some("2001:db8::1".into())
        );
        assert_eq!(normalize_bypass_entry("10.0.0.0/33"), None);
        assert_eq!(normalize_bypass_entry("nas:abc"), None);
        assert_eq!(
            normalize_bypass_entry("*.corp.example:8443"),
            Some(".corp.example:8443".into())
        );
        assert_eq!(normalize_bypass_entry("<local>"), None);
        assert_eq!(normalize_bypass_entry("10.*.1.*"), None);
        assert_eq!(normalize_bypass_entry("a b"), None);
        assert_eq!(normalize_bypass_entry("user@host"), None);
    }

    #[test]
    fn addresses_are_canonical() {
        assert_eq!(
            parse_address("127.0.0.1:7890").as_deref(),
            Some("http://127.0.0.1:7890")
        );
        assert_eq!(
            parse_address("http://proxy.corp").as_deref(),
            Some("http://proxy.corp:80")
        );
        assert_eq!(
            parse_address("socks5://[::1]:1080").as_deref(),
            Some("socks5://[::1]:1080")
        );
        assert_eq!(parse_address("socks5://host"), None, "SOCKS needs a port");
        assert_eq!(parse_address("https://proxy:443"), None);
        assert_eq!(parse_address("http://u:p@proxy:8080"), None);
        assert_eq!(parse_address("http://proxy:8080/path"), None);
        assert_eq!(parse_address(""), None);
    }

    #[test]
    fn environment_wins_over_the_os_and_all_proxy_fills_gaps() {
        let env = |pairs: &'static [(&'static str, &'static str)]| {
            move |name: &str| {
                pairs
                    .iter()
                    .find(|(key, _)| *key == name)
                    .map(|(_, value)| value.to_string())
            }
        };
        assert_eq!(environment_route(env(&[])), None);
        let route = environment_route(env(&[
            ("https_proxy", "127.0.0.1:7890"),
            ("ALL_PROXY", "socks5://127.0.0.1:7891"),
            ("NO_PROXY", "corp.example,192.168.*"),
        ]))
        .unwrap();
        assert_eq!(route.http.as_deref(), Some("socks5://127.0.0.1:7891"));
        assert_eq!(route.https.as_deref(), Some("http://127.0.0.1:7890"));
        assert!(route.bypass.starts_with("corp.example,192.168.0.0/16,"));
    }

    #[test]
    fn precedence() {
        let os = || system("http://127.0.0.1:7890");
        let settings = manual("proxy.corp", Some(3128));

        // Policy beats everything.
        let policy = PolicyProxy::Direct;
        let effective = resolve(Some(&policy), &settings, None, os);
        assert_eq!(effective.source, Source::Policy);
        assert!(!effective.route.has_proxy());

        let policy = PolicyProxy::System;
        assert_eq!(
            resolve(Some(&policy), &settings, None, os).source,
            Source::System
        );

        // Then the app setting.
        let effective = resolve(None, &settings, None, os);
        assert_eq!(effective.source, Source::Manual);
        assert_eq!(
            effective.route.https.as_deref(),
            Some("http://proxy.corp:3128")
        );

        let direct = Settings {
            mode: Mode::Direct,
            ..settings.clone()
        };
        assert!(!resolve(None, &direct, None, os).route.has_proxy());

        // An incomplete manual address follows the OS instead of going direct.
        assert_eq!(
            resolve(None, &manual("", Some(1)), None, os).source,
            Source::System
        );
    }

    #[test]
    fn credentials_stay_out_of_the_route() {
        let settings = Settings {
            username: "alice".into(),
            ..manual("proxy.corp", Some(3128))
        };
        let effective = resolve(None, &settings, Some("p@ss:w/rd"), || system("http://x:1"));
        assert_eq!(
            effective.route.http.as_deref(),
            Some("http://proxy.corp:3128")
        );
        assert!(!format!("{effective:?}").contains("p@ss"));
        let env = child_env(&effective);
        let http = &env
            .iter()
            .find(|(name, _)| *name == "HTTP_PROXY")
            .unwrap()
            .1;
        assert_eq!(http, "http://alice:p%40ss%3Aw%2Frd@proxy.corp:3128/");
    }

    #[test]
    fn child_env_overrides_every_inherited_spelling() {
        let env = child_env(&Effective::direct());
        for name in [
            "HTTP_PROXY",
            "http_proxy",
            "HTTPS_PROXY",
            "https_proxy",
            "NO_PROXY",
            "no_proxy",
            "ALL_PROXY",
            "all_proxy",
        ] {
            let value = &env.iter().find(|(key, _)| *key == name).unwrap().1;
            assert!(value.is_empty(), "{name} must be cleared");
        }
        let env = child_env(&system("http://127.0.0.1:7890"));
        let no_proxy = &env.iter().find(|(key, _)| *key == "NO_PROXY").unwrap().1;
        assert_eq!(no_proxy, "localhost,127.0.0.0/8,::1");
    }

    #[test]
    fn validation() {
        assert!(validate(&Settings::default()).is_ok());
        // Fields of an unselected mode are kept as typed.
        let parked = Settings {
            mode: Mode::Direct,
            host: " not a host ".into(),
            ..Settings::default()
        };
        assert_eq!(validate(&parked).unwrap().host, "not a host");

        assert_eq!(
            validate(&manual("proxy", None)).unwrap_err().field,
            Some("port")
        );
        for host in ["", "http://proxy", "proxy/path", "user@proxy", "a b"] {
            assert_eq!(
                validate(&manual(host, Some(8080))).unwrap_err().field,
                Some("host"),
                "{host:?}"
            );
        }
        let ipv6 = validate(&manual("::1", Some(8080))).unwrap();
        assert_eq!(ipv6.endpoint().as_deref(), Some("http://[::1]:8080"));

        let bypass = Settings {
            bypass: "corp.example; 10.*; <local>".into(),
            ..manual("proxy", Some(8080))
        };
        assert!(validate(&bypass).is_ok());
        let bad = Settings {
            bypass: "corp.example, a b".into(),
            ..manual("proxy", Some(8080))
        };
        // Split on spaces: "a" and "b" are both valid names.
        assert!(validate(&bad).is_ok());
        let bad = Settings {
            bypass: "corp.example; user@host".into(),
            ..manual("proxy", Some(8080))
        };
        let error = validate(&bad).unwrap_err();
        assert_eq!(error.field, Some("bypass"));
        assert_eq!(error.value.as_deref(), Some("user@host"));
    }

    #[test]
    fn policy_values() {
        assert_eq!(policy_from_raw("", "", ""), None);
        assert_eq!(policy_from_raw("Direct", "", ""), Some(PolicyProxy::Direct));
        assert_eq!(
            policy_from_raw("system", "x", ""),
            Some(PolicyProxy::System)
        );
        assert_eq!(
            policy_from_raw("fixed_servers", "proxy.corp:3128", "*.corp;10.*"),
            Some(PolicyProxy::Fixed {
                address: "http://proxy.corp:3128".into(),
                bypass: "*.corp;10.*".into(),
            })
        );
        assert_eq!(policy_from_raw("fixed_servers", "", ""), None);
        assert_eq!(
            policy_from_raw("fixed_servers", "http://u:p@proxy:1", ""),
            None
        );
        assert_eq!(policy_from_raw("pac_script", "", ""), None);
    }

    #[test]
    fn webview_configuration() {
        assert_eq!(
            WebviewProxy::resolve(None, &Settings::default()),
            WebviewProxy::System
        );
        let settings = Settings {
            bypass: "*.corp.example;192.168.*".into(),
            username: "alice".into(),
            ..manual("proxy.corp", Some(3128))
        };
        let webview = WebviewProxy::resolve(None, &settings);
        assert_eq!(
            webview,
            WebviewProxy::Fixed {
                address: "http://proxy.corp:3128".into(),
                bypass: ".corp.example,192.168.0.0/16,localhost,127.0.0.0/8,::1".into(),
            }
        );
        // The loopback entries drop out: Chromium bypasses them on its own.
        assert_eq!(webview.bypass_list(), ".corp.example;192.168.0.0/16");
        let args = webview.chromium_args();
        assert!(args.contains("--proxy-server=http://proxy.corp:3128"));
        assert!(args.contains("--proxy-bypass-list=.corp.example;192.168.0.0/16"));
        // The credentials are not in the arguments; the engine asks for them.
        assert!(!args.contains("alice"));
        assert!(WebviewProxy::System.chromium_args().is_empty());
        assert!(WebviewProxy::Direct
            .chromium_args()
            .contains("--no-proxy-server"));
        // A proxy that needs a password stays out of the webviews everywhere.
        assert_eq!(webview.clone().for_platform(true), WebviewProxy::System);
        let expected = match cfg!(windows) {
            true => webview.clone(),
            false => WebviewProxy::System,
        };
        assert_eq!(webview.for_platform(false), expected);
        assert_eq!(
            WebviewProxy::System.for_platform(false),
            WebviewProxy::System
        );
        assert_eq!(
            WebviewProxy::resolve(Some(&PolicyProxy::Direct), &settings),
            WebviewProxy::Direct
        );
    }

    /// HTTP server on loopback that records each request head.
    fn stub(reply: &'static str) -> (String, std::sync::mpsc::Receiver<String>) {
        use std::io::{Read, Write};
        let listener = std::net::TcpListener::bind("127.0.0.1:0").unwrap();
        let address = listener.local_addr().unwrap();
        let (tx, rx) = std::sync::mpsc::channel();
        std::thread::spawn(move || {
            for stream in listener.incoming() {
                let Ok(mut stream) = stream else { break };
                let mut buffer = [0u8; 2048];
                let read = stream.read(&mut buffer).unwrap_or(0);
                let _ = tx.send(String::from_utf8_lossy(&buffer[..read]).into_owned());
                let _ = stream.write_all(reply.as_bytes());
            }
        });
        (format!("http://{address}"), rx)
    }

    const OK: &str = "HTTP/1.1 200 OK\r\nContent-Length: 2\r\nConnection: close\r\n\r\nok";

    #[tokio::test]
    async fn apply_sends_through_the_proxy_and_honours_the_bypass_list() {
        crate::session::ensure_tls_provider();
        let (proxy, proxied) = stub(OK);
        // A destination that only the proxy may reach: nothing listens there.
        let target = "http://remote.example.invalid/api";
        let route = |bypass: &str| Effective {
            credentials: Some(Credentials {
                username: "alice".into(),
                password: "secret".into(),
            }),
            ..Effective::new(Source::Manual, Route::both(proxy.clone(), bypass))
        };

        let client = apply(reqwest::Client::builder(), &route(""))
            .build()
            .unwrap();
        let body = client
            .get(target)
            .send()
            .await
            .unwrap()
            .text()
            .await
            .unwrap();
        assert_eq!(body, "ok");
        let head = proxied.recv().unwrap();
        assert!(head.starts_with("GET http://remote.example.invalid/api HTTP/1.1\r\n"));
        // alice:secret, from the keychain password, never from the address.
        assert!(head.contains("proxy-authorization: Basic YWxpY2U6c2VjcmV0\r\n"));

        // Bypassed: the client connects directly and fails to resolve the host,
        // and the proxy sees nothing.
        let client = apply(reqwest::Client::builder(), &route("*.example.invalid"))
            .build()
            .unwrap();
        assert!(client.get(target).send().await.is_err());
        assert!(proxied.try_recv().is_err());
    }

    #[tokio::test]
    async fn apply_keeps_a_separate_proxy_per_scheme() {
        crate::session::ensure_tls_provider();
        let (http_proxy, http_seen) = stub(OK);
        let (https_proxy, https_seen) = stub("HTTP/1.1 403 Forbidden\r\nContent-Length: 0\r\n\r\n");
        let effective = Effective::new(
            Source::System,
            Route {
                http: Some(http_proxy),
                https: Some(https_proxy),
                bypass: normalized_bypass(""),
            },
        );
        let client = apply(reqwest::Client::builder(), &effective)
            .build()
            .unwrap();
        // The tunnel is refused, so the request fails; what matters is who saw it.
        let _ = client.get("https://remote.example.invalid/").send().await;
        assert!(https_seen
            .recv()
            .unwrap()
            .starts_with("CONNECT remote.example.invalid:443 "));
        assert!(http_seen.try_recv().is_err());
    }

    #[test]
    fn socks_addresses_build_a_client() {
        crate::session::ensure_tls_provider();
        let effective = Effective::new(
            Source::Manual,
            Route::both("socks5://127.0.0.1:1080".into(), ""),
        );
        assert!(apply(reqwest::Client::builder(), &effective)
            .build()
            .is_ok());
    }

    /// The dynamic-store reader, against whatever this machine has configured:
    /// the shapes Apple stores must survive the translation.
    #[cfg(target_os = "macos")]
    #[test]
    fn reads_the_macos_settings() {
        let Some(os) = read_os_proxy() else {
            return;
        };
        eprintln!("[test] os proxy: {os:?}");
        let effective = os.effective();
        for entry in split_list(&effective.route.bypass) {
            assert!(
                LOOPBACK_BYPASS.contains(&entry) || normalize_bypass_entry(entry).is_some(),
                "{entry} did not survive translation"
            );
        }
        for address in [&effective.route.http, &effective.route.https]
            .into_iter()
            .flatten()
        {
            assert_eq!(parse_address(address).as_deref(), Some(address.as_str()));
        }
    }

    #[test]
    fn loopback() {
        for local in [
            "http://127.0.0.1:53817",
            "http://127.1.2.3:8080",
            "http://localhost:3000",
            "http://LOCALHOST:3000",
            "http://[::1]:8080",
        ] {
            assert!(is_loopback(local), "{local} is loopback");
        }
        for remote in [
            "https://chat.example.com",
            "http://192.168.1.10:8080",
            "http://localhost.example.com",
            "http://[::2]:8080",
            "not a url",
        ] {
            assert!(!is_loopback(remote), "{remote} is not loopback");
        }
    }
}
