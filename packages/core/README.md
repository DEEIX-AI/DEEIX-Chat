# @deeix/core

Platform-agnostic client logic shared by `apps/web`, `apps/desktop` and `apps/mobile`.

## Modules

| Module | Exports | Purpose |
|---|---|---|
| `auth/errors` | `classifyAuthError`, `SESSION_TERMINATING_ERROR_CODES` | Map `{status, errorCode}` to `unauthorized` / `session_terminated` / `other`. Contract with `backend/.../auth/errors.go`. |
| `auth/refresh` | `createAuthClient(host)` | Refresh state machine: in-flight dedup, optional cross-tab lock, revision guard, 401 → refresh → single retry → clear on terminal error. |
| `server/url` | `normalizeApiBaseUrl`, `resolveApiBaseUrl` | Strict http(s) base URL validation and runtime → build-time → same-origin resolution. |

A host implements `AuthHost` (`SessionStore`, `refreshSession`, `classifyError`, optional `lock`).
Web: `apps/web/shared/auth/auth-client.ts`. Desktop/mobile add their own host; the policy is never copied.

## What belongs here

Logic that would otherwise be written once per client: auth policy, server address handling,
SSE parsing and message stream reducers (when a second client needs them).

## What does not belong here

- Any UI (React, React Native components, styling)
- Any platform I/O (`fetch`, `window`, `localStorage`, `SecureStore`, Tauri commands)

Hosts inject I/O through interfaces defined here. `tsconfig.json` has no `DOM` lib and
`biome.jsonc` blocks framework imports, so violations fail `pnpm check`.

## Testing

Tests run with Node's built-in runner and must not need a browser or device:

```bash
pnpm --filter @deeix/core test
```
