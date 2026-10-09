// Hand-off for one-click import links (`/import#v=1&url=…&key=…`).
//
// The fragment is read once and immediately removed from the address bar so
// the key does not linger in history, screenshots or a copied URL. When the
// user must sign in first, the login redirect would drop the fragment, so it
// waits in sessionStorage: tab-scoped, never sent to the server, and deleted
// once the signed-in import screen shows it.

import { isFiniteNumber, isRecord, isString } from "@/shared/lib/type-guards";

const STORAGE_KEY = "deeix.provider-import.pending";
/** A pending import older than this is discarded rather than shown. */
const PENDING_TTL_MS = 10 * 60 * 1000;

type PendingImport = { fragment: string; savedAt: number };

/**
 * Take the import fragment from the current URL, stash it for this tab and
 * scrub it from the address bar. Returns the fragment, or "" when none.
 */
export function captureProviderImportFragment(): string {
  if (typeof window === "undefined") return "";
  const fragment = window.location.hash.replace(/^#/, "");
  if (!fragment) return "";
  // replaceState keeps the page but drops the key from history and the visible URL.
  window.history.replaceState(window.history.state, "", `${window.location.pathname}${window.location.search}`);
  try {
    const pending: PendingImport = { fragment, savedAt: Date.now() };
    window.sessionStorage.setItem(STORAGE_KEY, JSON.stringify(pending));
  } catch {
    // Storage may be unavailable (private mode quotas); the in-memory fragment still works for this render.
  }
  return fragment;
}

/**
 * Read the stashed fragment without deleting it: the page may mount more than
 * once before sign-in (React re-runs effects in development), and deleting
 * here would lose the link on the way to the login page. The import screen
 * clears it once the user is signed in; expired entries are ignored.
 */
export function readPendingProviderImport(): string {
  if (typeof window === "undefined") return "";
  let raw: string | null = null;
  try {
    raw = window.sessionStorage.getItem(STORAGE_KEY);
  } catch {
    return "";
  }
  if (!raw) return "";
  let pending: unknown;
  try {
    pending = JSON.parse(raw);
  } catch {
    return "";
  }
  if (!isRecord(pending) || !isString(pending.fragment) || !isFiniteNumber(pending.savedAt)) return "";
  if (Date.now() - pending.savedAt > PENDING_TTL_MS) return "";
  return pending.fragment;
}

/** Forget any stashed import, e.g. when the user cancels. */
export function clearPendingProviderImport(): void {
  if (typeof window === "undefined") return;
  try {
    window.sessionStorage.removeItem(STORAGE_KEY);
  } catch {
    // Nothing to clean up when storage is unavailable.
  }
}
