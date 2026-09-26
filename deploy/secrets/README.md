# deploy/secrets

Local-only credentials. Everything here except this file is git-ignored.

| File | What | Where else it lives |
| --- | --- | --- |
| `deeix-chat-updater.key` | Tauri updater minisign **private** key, password-protected. Signs `latest.json` and every desktop package; an installed app only accepts updates signed with it. | GitHub Actions secrets `TAURI_SIGNING_PRIVATE_KEY` + `TAURI_SIGNING_PRIVATE_KEY_PASSWORD` |
| `deeix-chat-updater.key.pub` | Matching public key. | Embedded in `apps/desktop/src-tauri/tauri.conf.json` (`plugins.updater.pubkey`) |

Losing the private key means no installed desktop client can ever receive an
update again (they would have to reinstall), so keep a copy somewhere durable
outside this machine as well.

| `desktop-signing.env` | Signing inputs for a local release build (copy from `desktop-signing.env.example`). | GitHub Actions secrets of the same names (`*_PATH` entries become the file's base64) |
| `apple.p12` | Developer ID Application certificate with private key. Signing identity and team id are read from it. | `APPLE_CERTIFICATE` (base64) + `APPLE_CERTIFICATE_PASSWORD` |

Signed local build (loads `desktop-signing.env`, builds, verifies with Gatekeeper on macOS):

```bash
pnpm --filter @deeix/desktop build:signed
```
