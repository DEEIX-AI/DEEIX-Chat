"use client";

import { type AuthHost, type SessionStore, classifyAuthError, createAuthClient } from "@deeix/core";
import type { LoginData } from "@/shared/api/auth.types";
import { ApiError, apiRequest } from "@/shared/api/http-client";
import { clearSessionSnapshot, readAccessToken, readSessionRevision, writeSessionSnapshot } from "@/shared/auth/session";
import { isDesktopApp } from "@/shared/platform";
import { isShellSessionError, refreshSession } from "@/shared/platform/desktop-shell";

// Runtime host for the shared auth state machine (@deeix/core).
//
// Browser and desktop share one request path; only the refresh transport differs:
//   - browser: POST /auth/refresh with the HttpOnly cookie, server rotates it.
//   - desktop: the Tauri shell performs the refresh against the pinned server
//     using the keychain-held token. The webview never sees that token.

const AUTH_REFRESH_LOCK_NAME = "deeix-chat:auth-refresh";

type NavigatorWithLocks = Navigator & {
  locks?: {
    request<T>(name: string, callback: () => Promise<T> | T): Promise<T>;
  };
};

const sessionStore: SessionStore = {
  readAccessToken,
  readRevision: readSessionRevision,
  write: (credentials) => writeSessionSnapshot(credentials),
  // Refresh-driven clears never fan out to peers: each tab's own refresh
  // will fail on the same server state, and a peer may already hold a newer session.
  clear: () => clearSessionSnapshot({ syncPeers: false }),
};

const host: AuthHost = {
  store: sessionStore,
  async refreshSession() {
    if (isDesktopApp()) {
      try {
        return await refreshSession();
      } catch (error) {
        if (isShellSessionError(error) && error.kind === "no_session") {
          return null;
        }
        throw error;
      }
    }
    const data = await apiRequest<LoginData>("/api/v1/auth/refresh", { method: "POST" });
    return data.accessToken ? { accessToken: data.accessToken, sessionID: data.sessionID } : null;
  },
  classifyError(error) {
    if (error instanceof ApiError) {
      return classifyAuthError(error);
    }
    if (isShellSessionError(error) && error.kind === "http") {
      return classifyAuthError({ status: error.status, errorCode: error.errorCode });
    }
    return "other";
  },
  lock: {
    run(fn) {
      const locks = typeof navigator === "undefined" ? undefined : (navigator as NavigatorWithLocks).locks;
      return locks ? locks.request(AUTH_REFRESH_LOCK_NAME, fn) : fn();
    },
  },
};

export const authClient = createAuthClient(host);
