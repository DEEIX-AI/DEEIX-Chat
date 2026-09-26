// Client platform runtime. Not a security boundary: the backend decides how to
// deliver the refresh token from the X-Client-Platform header plus the request
// Origin, and never trusts client-side flags.

/**
 * True when the page runs inside the Tauri shell.
 * `isTauri` is injected by Tauri at document start, so a browser loading the
 * same bundle reports false — one build serves both targets.
 */
export function isDesktopApp(): boolean {
  return typeof window !== "undefined" && window.isTauri === true;
}

/** Value for the X-Client-Platform header, or "" for browsers. */
export function resolveClientPlatform(): "" | "desktop" {
  return isDesktopApp() ? "desktop" : "";
}
