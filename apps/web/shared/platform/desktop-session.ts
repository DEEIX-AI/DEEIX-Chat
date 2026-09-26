"use client";

// Desktop session bootstrap: connects the shared session snapshot and the API
// client to the Tauri shell. No-op in browsers.

import type { LoginData } from "@/shared/api/auth.types";
import { registerRuntimeApiBaseURLResolver } from "@/shared/api/http-client";
import { registerSessionClearedHandler, writeSessionSnapshot } from "@/shared/auth/session";
import { isDesktopApp } from "@/shared/platform";
import { clearSession, leaveServer, localSignIn, type ServerInfo, storeSession } from "@/shared/platform/desktop-shell";
import { loadServer, readServerMode, readServerOrigin } from "@/shared/platform/server-address";

let initialized: Promise<ServerInfo | null> | null = null;

/**
 * Install desktop hooks and resolve with the configured server (null on first
 * run). Idempotent; browsers resolve immediately with null.
 */
export function initializeDesktopSession(): Promise<ServerInfo | null> {
  if (!isDesktopApp()) {
    return Promise.resolve(null);
  }
  initialized ??= (async () => {
    registerRuntimeApiBaseURLResolver(readServerOrigin);
    // Sign-out: remote tabs drop the keychain token and land on the login page.
    // Local tabs have no login page to come back through, so leaving the
    // server is the equivalent — the tab returns to the setup screen.
    registerSessionClearedHandler(async () => {
      if (readServerMode() === "local") {
        await leaveServer();
      } else {
        await clearSession();
      }
    });
    return loadServer();
  })();
  return initialized;
}

/**
 * Record an authenticated session. Every sign-in path goes through here; on
 * desktop the refresh token is handed to the shell and never read back.
 */
export async function completeNativeSignIn(result: LoginData): Promise<void> {
  writeSessionSnapshot({ accessToken: result.accessToken, sessionID: result.sessionID });
  if (isDesktopApp() && result.refreshToken) {
    await storeSession(result.refreshToken);
  }
}

/**
 * Local mode has no login form: obtain a session from the shell, which
 * refreshes the stored token or, failing that, redeems the sidecar's one-time
 * grant. Returns true when a session is now available.
 */
export async function ensureLocalSession(): Promise<boolean> {
  if (!isDesktopApp() || readServerMode() !== "local") {
    return false;
  }
  const credentials = await localSignIn();
  // Sign-in may have restarted the sidecar on a new port.
  await loadServer();
  writeSessionSnapshot(credentials);
  return true;
}
