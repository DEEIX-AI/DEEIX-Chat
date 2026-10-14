// Enterprise policy. Administrators set it through Group Policy / Intune (ADMX
// templates in deploy/windows/policies) or plain registry values:
//
//   HKLM\SOFTWARE\Policies\DEEIX\Chat   (machine; wins value by value)
//   HKCU\SOFTWARE\Policies\DEEIX\Chat   (user)
//
//   DefaultServerUrl   REG_SZ     prefilled server address
//   LockServerUrl      REG_DWORD  1 = only DefaultServerUrl may be used
//   DisableLocalMode   REG_DWORD  1 = no bundled local server
//   DisableAutoUpdate  REG_DWORD  1 = no update checks at all
//   ProxyMode          REG_SZ     system | direct | fixed_servers (Chrome's names)
//   ProxyServer        REG_SZ     host:port, http://host:port or socks5://host:port
//   ProxyBypassList    REG_SZ     hosts, addresses and ranges that skip the proxy
//
// Policy is read once at startup (like most desktop apps; a change applies on
// the next launch) and enforced here in Rust. The web UI only reflects it.
// Other platforms have no policy source yet and always get the defaults.

use serde::Serialize;

use crate::proxy::{self, PolicyProxy};
use crate::session::normalize_origin;
use crate::tabs::Server;

/// Registry path below HKLM / HKCU.
#[cfg_attr(not(windows), allow(dead_code))]
pub const REGISTRY_KEY: &str = r"SOFTWARE\Policies\DEEIX\Chat";

pub const LOCAL_MODE_DISABLED: &str = "policy: local mode is disabled by your administrator";
pub const SERVER_LOCKED: &str = "policy: server address is locked by your administrator";
pub const UPDATES_DISABLED: &str = "policy: update checks are disabled by your administrator";

/// Values as found in one registry hive; `None` = not configured there.
#[derive(Debug, Clone, Default, PartialEq, Eq)]
pub struct RawPolicy {
    pub default_server_url: Option<String>,
    pub lock_server_url: Option<bool>,
    pub disable_local_mode: Option<bool>,
    pub disable_auto_update: Option<bool>,
    pub proxy_mode: Option<String>,
    pub proxy_server: Option<String>,
    pub proxy_bypass: Option<String>,
}

impl RawPolicy {
    /// Per-value precedence: a value configured in `self` (machine) wins over
    /// the same value in `lower` (user); unset values fall through.
    pub fn over(self, lower: RawPolicy) -> RawPolicy {
        // The proxy values are one setting spread over three values, so the
        // machine's ProxyMode decides the whole group: mixing a mode from one
        // hive with an address from the other would apply neither.
        let proxy = match self.proxy_mode {
            Some(_) => (self.proxy_mode, self.proxy_server, self.proxy_bypass),
            None => (
                lower.proxy_mode,
                lower.proxy_server.or(self.proxy_server),
                lower.proxy_bypass.or(self.proxy_bypass),
            ),
        };
        RawPolicy {
            default_server_url: self.default_server_url.or(lower.default_server_url),
            lock_server_url: self.lock_server_url.or(lower.lock_server_url),
            disable_local_mode: self.disable_local_mode.or(lower.disable_local_mode),
            disable_auto_update: self.disable_auto_update.or(lower.disable_auto_update),
            proxy_mode: proxy.0,
            proxy_server: proxy.1,
            proxy_bypass: proxy.2,
        }
    }
}

/// Effective policy. Serialised as `DistributionInfo.policy`.
#[derive(Debug, Clone, PartialEq, Eq, Serialize)]
#[serde(rename_all = "camelCase")]
pub struct Policy {
    /// Normalised origin (scheme://host[:port]); `None` when unset or invalid.
    pub default_server_url: Option<String>,
    /// True only when LockServerUrl=1 AND a valid DefaultServerUrl is set.
    pub server_url_locked: bool,
    /// False when DisableLocalMode=1, and also while the address is locked:
    /// a lock means "only this server", which excludes the bundled one.
    pub local_mode_allowed: bool,
    pub auto_update_disabled_by_policy: bool,
    /// Enterprise proxy policy; `None` leaves the choice to the user.
    #[serde(skip)]
    pub proxy: Option<PolicyProxy>,
    /// Serialised counterpart of `proxy`, which the web app only needs as a flag.
    pub proxy_managed_by_policy: bool,
}

impl Default for Policy {
    fn default() -> Self {
        Self::from_raw(RawPolicy::default())
    }
}

impl Policy {
    pub fn from_raw(raw: RawPolicy) -> Self {
        let default_server_url = raw.default_server_url.as_deref().and_then(|url| {
            let normalized = normalize_origin(url);
            if normalized.is_none() && !url.trim().is_empty() {
                eprintln!(
                    "[policy] ignoring DefaultServerUrl {url:?}: not an http(s) origin without a path"
                );
            }
            normalized
        });
        let server_url_locked = raw.lock_server_url == Some(true) && default_server_url.is_some();
        if raw.lock_server_url == Some(true) && !server_url_locked {
            eprintln!("[policy] LockServerUrl has no effect without a valid DefaultServerUrl");
        }
        let proxy = proxy::policy_from_raw(
            raw.proxy_mode.as_deref().unwrap_or_default(),
            raw.proxy_server.as_deref().unwrap_or_default(),
            raw.proxy_bypass.as_deref().unwrap_or_default(),
        );
        Self {
            default_server_url,
            server_url_locked,
            local_mode_allowed: raw.disable_local_mode != Some(true) && !server_url_locked,
            auto_update_disabled_by_policy: raw.disable_auto_update == Some(true),
            proxy_managed_by_policy: proxy.is_some(),
            proxy,
        }
    }

    /// May a tab be bound to this remote origin? `origin` must already be normalised.
    pub fn check_remote(&self, origin: &str) -> Result<(), &'static str> {
        match (&self.default_server_url, self.server_url_locked) {
            (Some(locked), true) if locked != origin => Err(SERVER_LOCKED),
            _ => Ok(()),
        }
    }

    pub fn check_local(&self) -> Result<(), &'static str> {
        if self.local_mode_allowed {
            Ok(())
        } else if self.server_url_locked {
            Err(SERVER_LOCKED)
        } else {
            Err(LOCAL_MODE_DISABLED)
        }
    }

    /// Whether a server restored from disk may still be used.
    pub fn allows(&self, server: &Server) -> bool {
        match server.mode {
            crate::session::ServerMode::Local => self.check_local().is_ok(),
            crate::session::ServerMode::Remote => self.check_remote(&server.origin).is_ok(),
        }
    }

    pub fn is_active(&self) -> bool {
        *self != Policy::default()
    }
}

/// Read the effective policy for this machine and user.
pub fn load() -> Policy {
    let policy = Policy::from_raw(read_hive(Hive::Machine).over(read_hive(Hive::User)));
    if policy.is_active() {
        eprintln!("[policy] in effect: {policy:?}");
    }
    policy
}

#[cfg_attr(not(windows), allow(dead_code))]
#[derive(Clone, Copy)]
enum Hive {
    Machine,
    User,
}

#[cfg(windows)]
fn read_hive(hive: Hive) -> RawPolicy {
    use windows_registry::{CURRENT_USER, LOCAL_MACHINE};

    let root = match hive {
        Hive::Machine => LOCAL_MACHINE,
        Hive::User => CURRENT_USER,
    };
    // A missing key is the normal "no policy" case; anything else is logged
    // and treated as unset rather than blocking startup.
    let key = match root.open(REGISTRY_KEY) {
        Ok(key) => key,
        Err(_) => return RawPolicy::default(),
    };
    let flag = |name: &str| -> Option<bool> {
        match key.get_u64(name) {
            Ok(value) => Some(value != 0),
            Err(_) => None,
        }
    };
    let string = |name: &str| {
        key.get_string(name)
            .ok()
            .map(|value| value.trim().to_string())
            .filter(|value| !value.is_empty())
    };
    RawPolicy {
        default_server_url: string("DefaultServerUrl"),
        lock_server_url: flag("LockServerUrl"),
        disable_local_mode: flag("DisableLocalMode"),
        disable_auto_update: flag("DisableAutoUpdate"),
        proxy_mode: string("ProxyMode"),
        proxy_server: string("ProxyServer"),
        proxy_bypass: string("ProxyBypassList"),
    }
}

#[cfg(not(windows))]
fn read_hive(_hive: Hive) -> RawPolicy {
    RawPolicy::default()
}

#[cfg(test)]
mod tests {
    use super::*;

    fn raw(
        url: Option<&str>,
        lock: Option<bool>,
        local: Option<bool>,
        update: Option<bool>,
    ) -> RawPolicy {
        RawPolicy {
            default_server_url: url.map(str::to_string),
            lock_server_url: lock,
            disable_local_mode: local,
            disable_auto_update: update,
            ..RawPolicy::default()
        }
    }

    fn proxy_raw(mode: &str, server: &str, bypass: &str) -> RawPolicy {
        RawPolicy {
            proxy_mode: Some(mode.to_string()),
            proxy_server: Some(server.to_string()),
            proxy_bypass: Some(bypass.to_string()),
            ..RawPolicy::default()
        }
    }

    #[test]
    fn proxy_policy_is_one_setting_across_two_hives() {
        // HKLM sets the mode, so it decides the whole group: an address from
        // HKCU would otherwise be paired with a mode that never chose it.
        let merged = proxy_raw("fixed_servers", "http://hklm.corp:3128", "").over(proxy_raw(
            "direct",
            "http://hkcu.corp:9999",
            "hkcu.example",
        ));
        assert_eq!(merged.proxy_mode.as_deref(), Some("fixed_servers"));
        let policy = Policy::from_raw(merged);
        assert_eq!(
            policy.proxy,
            Some(PolicyProxy::Fixed {
                address: "http://hklm.corp:3128".into(),
                bypass: String::new(),
            })
        );
        assert!(policy.proxy_managed_by_policy);
        assert!(policy.is_active());

        // Only HKCU configures it: HKCU applies.
        let merged = RawPolicy::default().over(proxy_raw("direct", "", ""));
        assert_eq!(Policy::from_raw(merged).proxy, Some(PolicyProxy::Direct));

        // An unusable combination is treated as no policy at all.
        let merged = RawPolicy::default().over(proxy_raw("fixed_servers", "", ""));
        let policy = Policy::from_raw(merged);
        assert_eq!(policy.proxy, None);
        assert!(!policy.proxy_managed_by_policy);
    }

    #[test]
    fn proxy_policy_is_not_serialised_but_its_flag_is() {
        let policy = Policy::from_raw(proxy_raw("direct", "", ""));
        let json = serde_json::to_value(&policy).unwrap();
        assert_eq!(json["proxyManagedByPolicy"], serde_json::json!(true));
        assert!(json.get("proxy").is_none());
    }

    #[test]
    fn no_policy_means_defaults() {
        let policy = Policy::from_raw(RawPolicy::default());
        assert_eq!(policy.default_server_url, None);
        assert!(!policy.server_url_locked);
        assert!(policy.local_mode_allowed);
        assert!(!policy.auto_update_disabled_by_policy);
        assert!(!policy.is_active());
    }

    #[test]
    fn machine_wins_value_by_value() {
        let machine = raw(Some("https://hklm.example"), None, Some(false), None);
        let user = raw(
            Some("https://hkcu.example"),
            Some(true),
            Some(true),
            Some(true),
        );
        let merged = machine.over(user);
        assert_eq!(
            merged.default_server_url.as_deref(),
            Some("https://hklm.example")
        );
        // Unset in HKLM: HKCU applies.
        assert_eq!(merged.lock_server_url, Some(true));
        assert_eq!(merged.disable_auto_update, Some(true));
        // Explicit 0 in HKLM overrides 1 in HKCU.
        assert_eq!(merged.disable_local_mode, Some(false));
    }

    #[test]
    fn lock_requires_a_valid_url() {
        assert!(!Policy::from_raw(raw(None, Some(true), None, None)).server_url_locked);
        assert!(
            !Policy::from_raw(raw(Some("chat.example.com"), Some(true), None, None))
                .server_url_locked
        );
        assert!(
            !Policy::from_raw(raw(Some("https://a.example/app"), Some(true), None, None))
                .server_url_locked
        );
        let locked = Policy::from_raw(raw(
            Some(" HTTPS://Chat.Example.com/ "),
            Some(true),
            None,
            None,
        ));
        assert!(locked.server_url_locked);
        assert_eq!(
            locked.default_server_url.as_deref(),
            Some("https://chat.example.com")
        );
        assert!(
            !Policy::from_raw(raw(Some("https://a.example"), Some(false), None, None))
                .server_url_locked
        );
    }

    #[test]
    fn lock_compares_normalised_origins() {
        let policy = Policy::from_raw(raw(
            Some("https://Chat.Example.com:443/"),
            Some(true),
            None,
            None,
        ));
        let ok = normalize_origin("https://chat.example.com").unwrap();
        assert_eq!(policy.check_remote(&ok), Ok(()));
        for other in [
            "http://chat.example.com",
            "https://chat.example.com:8443",
            "https://evil.example",
        ] {
            let other = normalize_origin(other).unwrap();
            assert_eq!(policy.check_remote(&other), Err(SERVER_LOCKED), "{other}");
        }
        // A lock also excludes the bundled local server.
        assert!(!policy.local_mode_allowed);
        assert_eq!(policy.check_local(), Err(SERVER_LOCKED));
    }

    #[test]
    fn unlocked_default_url_is_only_a_suggestion() {
        let policy = Policy::from_raw(raw(Some("https://a.example"), None, None, None));
        assert_eq!(policy.check_remote("https://b.example"), Ok(()));
        assert_eq!(policy.check_local(), Ok(()));
        assert!(policy.is_active());
    }

    #[test]
    fn local_mode_can_be_disabled_alone() {
        let policy = Policy::from_raw(raw(None, None, Some(true), None));
        assert_eq!(policy.check_local(), Err(LOCAL_MODE_DISABLED));
        assert!(policy.allows(&Server::remote("https://a.example".into())));
        assert!(!policy.allows(&Server::local()));
    }

    #[test]
    fn errors_carry_the_policy_prefix() {
        for message in [LOCAL_MODE_DISABLED, SERVER_LOCKED, UPDATES_DISABLED] {
            assert!(message.starts_with("policy: "), "{message}");
        }
    }
}
