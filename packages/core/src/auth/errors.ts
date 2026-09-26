// Error classification for the auth flow. Pure; hosts map their own error
// types (ApiError, Response, ...) onto AuthErrorKind via AuthHost.classifyError.

/**
 * Backend error codes that mean the current session is gone for good and the
 * client must drop its credentials instead of retrying.
 *
 * Contract with backend/internal/transport/http/auth/errors.go.
 */
export const SESSION_TERMINATING_ERROR_CODES: ReadonlySet<string> = new Set([
  "auth.invalid_token",
  "auth.invalid_refresh_token",
  "auth.session_invalid",
]);

export type AuthErrorKind =
  /** HTTP 401 that may be recovered by refreshing the access token. */
  | "unauthorized"
  /** HTTP 401 with a session-terminating code; credentials must be cleared. */
  | "session_terminated"
  /** Anything else (network, 4xx/5xx, validation). Never triggers a refresh. */
  | "other";

export type AuthErrorShape = {
  status?: number;
  errorCode?: string;
};

/** Classify a normalised {status, errorCode} pair. */
export function classifyAuthError(error: AuthErrorShape | null | undefined): AuthErrorKind {
  if (error?.status !== 401) {
    return "other";
  }
  if (typeof error.errorCode === "string" && SESSION_TERMINATING_ERROR_CODES.has(error.errorCode)) {
    return "session_terminated";
  }
  return "unauthorized";
}
