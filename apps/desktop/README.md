# apps/desktop

Tauri 2 shell around the `@deeix/web` build. It contains **no business logic**:
windows, tray, deep links, auto-update and OS keychain access only. If a feature
needs code here, the right fix is a missing abstraction in `packages/core` or
`apps/web` — see [docs/ARCHITECTURE.md](../../docs/ARCHITECTURE.md).

## How the pieces fit

```
apps/web (Next.js static export — the same bundle the Go server serves)
        │  frontendDist: ../../web/out
        ▼
apps/desktop/src-tauri (Rust)
        ├── tabs               src/tabs.rs       one window, one webview per server, tab strip webview
        ├── sidecar            src/sidecar.rs    bundled Go server for local mode (loopback, SQLite)
        ├── session            src/session.rs    server choice + keychain + refresh (token never exposed to JS)
        ├── tray               src/tray.rs       show / quit, left click restores the window
        ├── OAuth loopback     src/oauth_loopback.rs  RFC 8252 receiver on 127.0.0.1:<ephemeral>
        ├── updater            tauri-plugin-updater + apps/web/shared/platform/desktop-updater.ts
        └── CSP                no inline/remote scripts; network open to http(s) (user picks the server)
```

The desktop app loads the exact web bundle; there is no desktop-specific build.
At runtime the page detects the shell through `window.isTauri` and shows the
first-run server setup screen. Token delivery is decided server-side: requests
carrying `X-Client-Platform: desktop` from a non-web Origin get the refresh
token in the response body instead of a `SameSite` cookie, which a cross-origin
webview would never receive.

## Tabs: one webview per server

The window is a plain `Window` with child webviews (Tauri `unstable`):

```
┌─ chrome (40px) ── /desktop/tabs ── tab strip, drag region ───────────┐
├─ tab-1 ── /  ── bound to local ──────────────────────────────────────┤
├─ tab-2 ── /  ── bound to https://chat.example.com  (hidden)          │
└─ tab-3 ── /  ── unbound → setup screen             (hidden)          ┘
```

Every tab is a full instance of the web app with its own DOM, caches, SSE
connections and in-memory session, so the app never has to model "several
servers at once". A tab is bound to at most one server; two tabs never share a
server (opening one that is already open activates its tab). Closing a tab
forgets that server: its refresh token is deleted and the sidecar stops when
the last local tab goes. Bound tabs are restored on launch from `tabs.json`.

Session commands resolve their server from the *calling webview's label*, so a
tab can only ever touch its own credential. `build.rs` declares the app ACL
manifest, so every app command must be granted per webview in
`capabilities/`: content tabs get the session commands, the strip gets
`tabs_*`, and neither can call the other's (Tauri only enforces the ACL on app
commands once a manifest exists). Webviews may only navigate within the app;
external links and `window.open` are routed to the system browser. The `BroadcastChannel` the browser
build uses to sync tokens between same-origin documents is disabled on desktop
for the same reason — tabs share an origin but not a server.

The strip is a page of the same web build (`/desktop/tabs`) under its own root
layout (`app/(shell)`), which mounts only the theme and the strip's strings —
no branding fetch, no app i18n bundles, no guards.

Tabs are lazy: on launch only the active tab gets a webview, the others are
created on first click. A tab hidden for 30 minutes has its webview discarded
(browser "memory saver"); it reloads on the next activation and signs back in
from the keychain. Tabs can be reordered by dragging; order is persisted.

`pnpm dev` runs with `tauri.dev.conf.json`, which swaps the identifier to
`com.deeix.chat.desktop.dev`: the dev build gets its own config dir, keychain
service and local database, so it never disturbs an installed copy.

Storage (localStorage) is shared across tabs on purpose: it holds UI
preferences such as theme and fonts, which should follow the user, not the
server.

## Two ways to run

| | Local | Remote |
| --- | --- | --- |
| Server | the Go server bundled as a sidecar, started by the shell | a DEEIX Chat deployment the user points the app at |
| Data | SQLite + local files under the app data dir (`local/`) | on that server |
| Account | one passwordless owner, signed in via a one-time grant from the sidecar handshake | the server's accounts, password or OAuth |
| Network | `127.0.0.1:<ephemeral>` only; port changes each launch | whatever the user pinned |
| Keychain key | `refresh-token:local` | `refresh-token:<origin>` |

Both modes run **the same server code with the same security policy**: local
mode is the SQLite deployment profile with per-install secrets, production
validation, and a CORS allowlist limited to the webview. Nothing in the API is
weaker locally. The only local-only endpoint is `POST /api/v1/auth/local/exchange`,
which is mounted solely in local mode and accepts a grant that (a) is printed once
on the sidecar's stdout, (b) is single-use, (c) expires in two minutes, and
(d) is consumed by Rust — the webview never sees it.

Sign-out in a local tab has no login page to return to, so it maps to
"leave server": the tab drops its credential and returns to the setup screen.

### Sidecar lifecycle

`src/sidecar.rs` spawns `deeix-chat-server --local --data-dir <app-data>/local`
through `tauri-plugin-shell` (Rust side only; the webview has no shell
permission). The server binds a loopback port, prints one JSON line
(`{"type":"ready","origin":…,"grant":…}`) to stdout, and logs to stderr. The
shell waits up to 30 s for that line, forwards stderr to its own log, restarts
the process on demand if it exits, and kills it on app exit.

Binary: `scripts/build-sidecar.mjs` builds `backend/cmd/server` for the current
Rust target triple into `src-tauri/binaries/` (git-ignored). `pnpm dev` and
`pnpm build` run it first; CI runs it once per matrix leg because SQLite links
through cgo. The script downloads `go-sqlite3` itself and takes the SQLite
headers sqlite-vec needs from that module, so a fresh runner without a system
SQLite works. To check the Windows build from macOS:

```bash
brew install mingw-w64
CC=x86_64-w64-mingw32-gcc node scripts/build-sidecar.mjs --target x86_64-pc-windows-msvc
```

## Credential model

The browser build keeps the refresh token in an HttpOnly cookie, so page script
cannot read it. The desktop shell reproduces that property in Rust:

| | Browser | Desktop |
| --- | --- | --- |
| Access token | JS memory | JS memory |
| Refresh token at rest | HttpOnly cookie | OS keychain, account `refresh-token:<origin>` |
| Who sends it | browser, to the cookie's origin | `session.rs`, to the pinned origin only |
| Readable by page script | no | no — `store_session` is write-once, there is no read command |
| Server address | page origin | bound per tab, persisted in `tabs.json`; closing the tab drops that session |

The webview holds the refresh token exactly once, in the login response, and
hands it to `store_session` immediately. After that the only session commands
are `refresh_session` (returns an access token), `local_sign_in` (local tabs:
refresh or redeem the sidecar grant), `clear_session` and `leave_server`. A script injected into the page — from a malicious message,
a shared conversation, another user's profile — therefore gets the same
short-lived access it would get in a browser, not the long-lived credential,
and cannot redirect the credential to a server it controls.

## Prerequisites

- Rust toolchain (`rustup`) with a recent stable
- Platform dependencies for Tauri 2 (WebKitGTK on Linux, WebView2 on Windows,
  Xcode command line tools on macOS)
- `pnpm install` from the repository root

## Development

```bash
# from the repository root
pnpm --filter @deeix/desktop dev
```

`beforeDevCommand` starts `next dev` on port 3000 and the shell loads it. The Go
server must already be reachable at the address you enter on the setup screen
(default `http://127.0.0.1:8080`).

## Build

```bash
pnpm --filter @deeix/desktop build
```

Artifacts land in `apps/desktop/src-tauri/target/release/bundle/`.

Building is not enough to ship: macOS packages must be **notarized** and Windows
packages **code-signed**, otherwise users cannot install them. Signing keys and
the updater key live only in CI secrets — never in this repository.

## Third-party sign-in (OAuth / OIDC)

The webview cannot receive a provider redirect, so the desktop app follows the
RFC 8252 native-app flow using the server's provider-auth bridge:

1. The shell binds an ephemeral port on `127.0.0.1` and hands the web app
   `http://127.0.0.1:<port>/oauth/callback` as the redirect URI.
2. The web app starts the bridge with client id `com.deeix.chat.desktop`; the
   server validates that the redirect is a loopback address with a port and
   nothing else, then returns the provider authorization URL.
3. The authorization URL opens in the **system browser** (the user's existing
   provider session is reused; the webview never sees provider credentials).
4. The provider redirects to the server callback, which issues a one-time DEEIX
   grant and redirects the browser to the loopback URI.
5. The shell answers that request with a "you can close this tab" page and
   emits the callback URL to the webview, which navigates to `/auth/callback`
   and exchanges the grant with its PKCE verifier — the same code path the
   browser build uses.

Requirements on the server side: `PUBLIC_API_BASE_URL` set (this enables the
bridge) and the instance callback registered with each provider — see the
"OAuth callbacks for Web, App, and Desktop" section of the root README. No
custom URL scheme is registered and nothing is added to the provider allowlist
for the desktop app.

## Auto-update

The updater is configured in `tauri.conf.json` (`plugins.updater`). The app
checks on launch and every four hours (`desktop-update-notifier.tsx`) and
offers the update in a toast; nothing downloads until the user accepts.

Release assets are renamed by `scripts/rename-release-assets.mjs` to
`DEEIX-Chat-<version>-<os>-<arch>[-setup|-updater].<ext>`, e.g.
`DEEIX-Chat-0.4.4-beta.1-macos-arm64.dmg`, `-linux-x64.AppImage`,
`-windows-x64-setup.exe`, `-windows-x64-zh-CN.msi` (one MSI per
`bundle.windows.wix.language`), `-macos-arm64-updater.tar.gz`. The plan is
validated for collisions before anything is renamed, and the updater manifest,
which references assets by file name, is reconciled from its own URLs, so the
job can be re-run after an interruption. Only the artifact bytes are signed, so
verification is unaffected. `pnpm test` covers the mapping.

`scripts/sync-version.mjs` also derives `bundle.windows.wix.version` from
`VERSION`: MSI product versions are numeric-only, so `0.4.4-beta.1` becomes
`0.4.4.1` (a pre-release must therefore end in a number). Windows Installer
ignores the fourth field when comparing versions, so a beta still upgrades to
its stable; the in-app updater compares semver.

Release flow: merging a `VERSION` bump into `main` creates the tag
`v<VERSION>` (`release-tag.yml`), which builds every target and opens a
**draft** GitHub Release with the installers and a signed `latest.json`.
Publishing the draft is the step that ships the update; until then existing
installs see nothing. Pushing the tag by hand does the same.

### Channels

| Tag | Channel | What the app polls |
| --- | --- | --- |
| `v1.2.3` | stable | `releases/latest/download/latest.json` — GitHub resolves this to the newest published non-prerelease |
| `v1.2.3-beta.1` | beta | `releases/download/desktop-beta/latest.json` — a rolling tag refreshed by `desktop-channel.yml` whenever a prerelease is published |

A beta build is the same code with a different updater endpoint, injected at
build time via `tauri build --config`. Both channels are signed with the same
updater key: a channel is a distribution lane, not a trust boundary. Beta
installs keep receiving betas; to move a user back to stable, have them install
a stable build. Stable installs never see prereleases.

## Build times and sizes

What a local `pnpm build` costs once caches are warm, and where it goes:

| Step | Unchanged | Changed | Notes |
| --- | --- | --- | --- |
| Go sidecar | ~3 s | ~20 s | Go build cache; `-tags nopostgres,noredis,nos3,noswagger,nomsgpack` compiles out drivers local mode does not use. cgo needs a C compiler and SQLite headers; `build-sidecar.mjs` takes the headers from the `go-sqlite3` module so Windows works without a system SQLite |
| Web (`next build`) | **0.3 s** | ~35 s | runs through `turbo`, so an untouched frontend is a cache hit |
| Rust app crate | ~1 s | ~60 s | recompiles whenever `out/` changed (assets are embedded); fat LTO, the shipped profile |
| `.app` | ~1 s | | |
| `.dmg` | ~25 s | | `hdiutil` + Finder layout; `scripts/package-dmg.sh` then re-encodes with lzfse and (when signing is configured) re-signs and notarizes |

`pnpm build:signed` produces exactly what CI ships. Use `pnpm build:app` while
iterating: it relaxes LTO (~17 s link) and stops at the `.app`.

Every `no*` tag has an `*_off.go` counterpart that returns a clear error if the
config selects a driver that was compiled out.

## Icons

`src-tauri/icons/mark.png` is the bare "D" mark (black on transparent). From it:
`source.png` is the 1024² app icon on the macOS grid (824px rounded square in
the theme's `--background` #f8f8f6, mark in `--foreground` #3d3929 at 82%, transparent margin) —
regenerate every platform size with
`pnpm tauri icon src-tauri/icons/source.png --output src-tauri/icons` and
delete the generated `android/` and `ios/` folders. `tray.png` is the 44px
monochrome menu-bar glyph (macOS template image); Windows/Linux trays show the
app icon.

## Signing

| Platform | What is needed | CI secrets |
| --- | --- | --- |
| macOS | Developer ID Application certificate (.p12) + Apple ID app-specific password for notarization; identity and team id are derived from the certificate (`scripts/apple-signing-env.sh`) | `APPLE_CERTIFICATE`, `APPLE_CERTIFICATE_PASSWORD`, `APPLE_ID`, `APPLE_PASSWORD` |
| Windows | Code-signing certificate (.pfx) | `WINDOWS_CERTIFICATE`, `WINDOWS_CERTIFICATE_PASSWORD` |
| All | Updater keypair (`tauri signer generate`); the private key is kept git-ignored in `deploy/secrets/` (see its README) | `TAURI_SIGNING_PRIVATE_KEY`, `TAURI_SIGNING_PRIVATE_KEY_PASSWORD` |

`tauri.conf.json` already enables the hardened runtime on macOS and SHA-256 +
RFC 3161 timestamping on Windows; CI imports the Windows certificate into the
runner's store and passes its thumbprint to the bundler.

Only the updater key is mandatory. Platform signing switches on by itself once
its secrets exist. Without a Developer ID the macOS app is signed **ad-hoc**
(`APPLE_SIGNING_IDENTITY=-`); Tauri would otherwise skip `codesign` and leave
only the linker's signature, which macOS reports as *damaged* on Apple silicon.
Ad-hoc builds still need the user to allow the app under System Settings →
Privacy & Security and are for internal testing, not public releases. Signing
keys never enter the repository.

To obtain the .p12 on macOS: Keychain Access → My Certificates → right-click the
"Developer ID Application" cert → Export → base64 it (`base64 -i cert.p12 | pbcopy`).

## Keychain entries

| Field | Value |
| --- | --- |
| Service | `com.deeix.chat.desktop` |
| Account | `refresh-token:<origin>` (e.g. `refresh-token:https://chat.example.com`) |

One entry per origin; switching servers deletes the previous entry rather than
leaving dormant credentials behind.
