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

/**
 * Public address of the web client, for links meant to be opened elsewhere.
 * The browser is already on it; the desktop shell runs a bundled copy whose
 * own origin is useless to others, while the server it talks to also serves
 * the web client. Empty outside the browser.
 */
export function resolvePublicWebOrigin(apiBaseURL: string): string {
  if (typeof window === "undefined") return "";
  return isDesktopApp() ? apiBaseURL : window.location.origin;
}
