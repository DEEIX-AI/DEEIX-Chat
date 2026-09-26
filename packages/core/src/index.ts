// @deeix/core — platform-agnostic client logic.
//
// Rules (enforced by tsconfig `lib: ["ES2022"]` and biome `noRestrictedImports`):
//   - No React, Next.js, React Native, Expo or Tauri imports.
//   - No `window`, `document`, `localStorage`, `fetch` globals. Hosts inject I/O.
//   - Everything here must run under `node --test`.

export { SESSION_TERMINATING_ERROR_CODES, classifyAuthError } from "./auth/errors.ts";
export type { AuthErrorKind, AuthErrorShape } from "./auth/errors.ts";
export { createAuthClient } from "./auth/refresh.ts";
export type {
  AuthClient,
  AuthHost,
  RefreshLock,
  SessionCredentials,
  SessionStore,
  WithAuthRetryOptions,
} from "./auth/refresh.ts";
export { normalizeApiBaseUrl, resolveApiBaseUrl } from "./server/url.ts";
export type { ResolveApiBaseUrlInput } from "./server/url.ts";
