# Windows deployment

For IT administrators deploying DEEIX Chat Desktop to managed Windows machines. For how the app is built and how each distribution form behaves, see [`apps/desktop/README.md`](../../apps/desktop/README.md#windows-distribution).

## Choose a package

| Package | Use when | Scope | Updates |
| --- | --- | --- | --- |
| `DEEIX-Chat-<v>-windows-x64.msi` | Deploying through Intune, Configuration Manager (SCCM) or Group Policy | Per machine (`Program Files`) | None in-app; you deploy new versions |
| `DEEIX-Chat-<v>-windows-x64-setup.exe` | Users install it themselves | Per user, no admin rights | Built-in updater |
| `DEEIX-Chat-<v>-windows-x64-portable.zip` | Installers are blocked, or a USB / no-install setup is needed | Unzip and run | Notifies only; replace the folder to update |
| `DEEIX-Chat-<v>-windows-x64-portable-offline.zip` | As above, on machines without WebView2 and without internet access (published only when built) | Unzip and run | Same as portable |

## Prerequisites

- Windows 10 or later (x64). Windows Server 2016+ works but needs the WebView2 runtime.
- **Microsoft Edge WebView2 Runtime.** Built into Windows 10 21H1+ and Windows 11. The setup and MSI download it when it is missing, which fails offline — deploy the [Evergreen Standalone Installer](https://developer.microsoft.com/microsoft-edge/webview2/) first on offline machines, or use the offline portable zip.
- A DEEIX Chat server reachable from the client (unless users run the built-in local server).

## Verify what you deploy

Each release publishes `SHA256SUMS` and a GitHub build provenance attestation:

```powershell
# Checksum
(Get-FileHash .\DEEIX-Chat-<v>-windows-x64.msi -Algorithm SHA256).Hash
# Compare with the matching line in SHA256SUMS

# Authenticode signature (msi, setup.exe, and the .exe files inside the portable zip)
Get-AuthenticodeSignature .\DEEIX-Chat-<v>-windows-x64.msi

# Build provenance (GitHub CLI)
gh attestation verify .\DEEIX-Chat-<v>-windows-x64.msi --repo DEEIX-AI/DEEIX-Chat
```

AppLocker / WDAC: allow by **publisher** (the certificate on the signed binaries) rather than by path; the portable edition runs from wherever it is unzipped.

## Install silently

```powershell
# Per machine (MSI)
msiexec /i DEEIX-Chat-<v>-windows-x64.msi /qn /l*v "$env:TEMP\deeix-chat-install.log"

# Uninstall (MSI)
msiexec /x DEEIX-Chat-<v>-windows-x64.msi /qn

# Per user (setup)
.\DEEIX-Chat-<v>-windows-x64-setup.exe /S
```

Newer MSIs upgrade older ones in place (fixed upgrade code), so Intune / SCCM supersedence and Group Policy upgrades work across versions, including prereleases.

## Configure with policy

The app reads `HKLM\SOFTWARE\Policies\DEEIX\Chat` and `HKCU\SOFTWARE\Policies\DEEIX\Chat` at startup; a value under HKLM wins. Changes take effect the next time the app starts.

| Policy | Registry value | Effect |
| --- | --- | --- |
| Default server address | `DefaultServerUrl` (REG_SZ) | Prefills the server address on the setup screen. Use an `http(s)` origin without a path, e.g. `https://chat.example.com`. |
| Allow only the default server | `LockServerUrl` (REG_DWORD 1) | Users can only connect to `DefaultServerUrl` (ignored when that is not set). Also disables the built-in local server. Previously saved tabs for other servers are removed together with their stored sign-in. |
| Disable the built-in local server | `DisableLocalMode` (REG_DWORD 1) | Users cannot run the bundled local server. |
| Disable update checks | `DisableAutoUpdate` (REG_DWORD 1) | No update checks or notifications in any package. |

The app enforces these itself; the UI only shows them (read-only address, hidden options, "managed by your organization").

### Group Policy (ADMX)

1. Copy `policies/DEEIXChat.admx` to `C:\Windows\PolicyDefinitions` — or to the central store `\\<domain>\SYSVOL\<domain>\Policies\PolicyDefinitions` — and the matching `policies/<language>/DEEIXChat.adml` (`en-US`, `zh-CN`) into the language subfolder next to it.
2. Open the Group Policy Management Editor: **Computer Configuration** (or **User Configuration**) → **Administrative Templates** → **DEEIX Chat**.

### Intune

Import the ADMX under **Devices → Configuration → Import ADMX**, then create an *Imported Administrative templates* profile. Alternatively, set the registry values with a remediation or configuration script.

### Registry file

[`policies/DEEIXChat.example.reg`](policies/DEEIXChat.example.reg) sets all four values under HKLM. Edit it, then run `reg import DEEIXChat.example.reg` from an elevated prompt.

## Where data lives

| Package | App data | Stored sign-in |
| --- | --- | --- |
| MSI / setup | `%APPDATA%\com.deeix.chat.desktop`, `%LOCALAPPDATA%\com.deeix.chat.desktop` | Windows Credential Manager, service `com.deeix.chat.desktop` |
| Portable | `data\` next to `DEEIX Chat.exe` | Windows Credential Manager, service `com.deeix.chat.desktop.portable-<id>` (id in `data\instance-id`) |

Refresh tokens are never written to disk in plain text. Removing a portable copy: sign out in the app, then delete the folder.
