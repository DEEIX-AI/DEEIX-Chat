import type { AuthErrorKind } from "./errors.ts";

// Access-token refresh state machine shared by every client.
//
// Invariants (the reason this lives in core and not per app):
//   1. At most one refresh is in flight per client; concurrent callers share it.
//   2. A refresh started against session revision R must not clear a session
//      that was replaced by revision R+1 while the refresh was running.
//   3. A refresh that returns no token, or fails with a session-terminating
//      error, clears the session and resolves to "" — it never throws for that.
//   4. Other refresh failures (network, 5xx) propagate; the session is kept.
//   5. A request that gets 401 is retried exactly once with a fresh token.
//      If the retry fails with a session-terminating error, the session is cleared.

export type SessionCredentials = {
  accessToken: string;
  sessionID: string;
};

/**
 * Where the current session lives. Web keeps it in memory + BroadcastChannel,
 * desktop/mobile back it with secure storage. Core never persists anything itself.
 */
export interface SessionStore {
  readAccessToken(): string;
  /** Monotonic counter that changes whenever the stored session changes. */
  readRevision(): number;
  write(credentials: SessionCredentials): void;
  clear(): void;
}

/** Cross-context mutual exclusion (e.g. Web Locks across browser tabs). Optional. */
export interface RefreshLock {
  run<T>(fn: () => Promise<T>): Promise<T>;
}

export interface AuthHost {
  store: SessionStore;
  /**
   * Obtain fresh credentials using this platform's refresh transport
   * (HttpOnly cookie on web, native shell command on desktop/mobile). The
   * long-lived refresh token itself never passes through core.
   * Return `null` when the server answers without an access token.
   * Throw for transport/HTTP failures; they are classified via `classifyError`.
   */
  refreshSession(): Promise<SessionCredentials | null>;
  classifyError(error: unknown): AuthErrorKind;
  lock?: RefreshLock;
}

export type WithAuthRetryOptions<T> = {
  accessToken: string;
  /** Perform the request with the given token. Must throw on 401. */
  execute: (accessToken: string) => Promise<T>;
  /** Set to false to fail fast on 401 without refreshing. Default true. */
  allowRefresh?: boolean;
  /**
   * Abort detection. Called with the caught error after a failed attempt and
   * with no argument after the refresh. Return the error to throw, or null.
   */
  resolveAbort?: (error?: unknown) => Error | null;
};

export interface AuthClient {
  /** Refresh unconditionally (deduplicated). Resolves "" when the session is gone. */
  refreshAccessToken(failedToken?: string): Promise<string>;
  /** Return a token that differs from `failedToken`, refreshing only if needed. */
  recoverAccessToken(failedToken: string): Promise<string>;
  /** Run `execute`; on 401 refresh once and retry. */
  withAuthRetry<T>(options: WithAuthRetryOptions<T>): Promise<T>;
}

export function createAuthClient(host: AuthHost): AuthClient {
  const { store } = host;
  let inFlight: Promise<string> | null = null;

  async function performRefresh(): Promise<string> {
    const startedRevision = store.readRevision();
    const clearIfUnchanged = () => {
      if (store.readRevision() === startedRevision) {
        store.clear();
      }
    };

    let credentials: SessionCredentials | null;
    try {
      credentials = await host.refreshSession();
    } catch (error) {
      if (host.classifyError(error) === "session_terminated") {
        clearIfUnchanged();
        return "";
      }
      throw error;
    }

    if (!credentials?.accessToken) {
      clearIfUnchanged();
      return "";
    }

    store.write(credentials);
    return credentials.accessToken;
  }

  function refreshUnlessReplaced(failedToken: string): Promise<string> {
    const run = async () => {
      // Another tab/caller may already have rotated the token while we waited for the lock.
      const currentToken = store.readAccessToken();
      if (currentToken && currentToken !== failedToken) {
        return currentToken;
      }
      return performRefresh();
    };
    return host.lock ? host.lock.run(run) : run();
  }

  function refreshAccessToken(failedToken = ""): Promise<string> {
    if (!inFlight) {
      inFlight = refreshUnlessReplaced(failedToken).finally(() => {
        inFlight = null;
      });
    }
    return inFlight;
  }

  async function recoverAccessToken(failedToken: string): Promise<string> {
    const currentToken = store.readAccessToken();
    if (currentToken && currentToken !== failedToken) {
      return currentToken;
    }
    return refreshAccessToken(failedToken);
  }

  async function withAuthRetry<T>(options: WithAuthRetryOptions<T>): Promise<T> {
    const { execute, allowRefresh = true, resolveAbort } = options;
    try {
      return await execute(options.accessToken);
    } catch (error) {
      const abortError = resolveAbort?.(error);
      if (abortError) {
        throw abortError;
      }
      if (!allowRefresh || host.classifyError(error) === "other") {
        throw error;
      }

      const refreshedToken = await recoverAccessToken(options.accessToken);
      const refreshAbortError = resolveAbort?.();
      if (refreshAbortError) {
        throw refreshAbortError;
      }
      if (!refreshedToken) {
        throw error;
      }

      try {
        return await execute(refreshedToken);
      } catch (retryError) {
        if (host.classifyError(retryError) === "session_terminated") {
          store.clear();
        }
        throw retryError;
      }
    }
  }

  return { refreshAccessToken, recoverAccessToken, withAuthRetry };
}
