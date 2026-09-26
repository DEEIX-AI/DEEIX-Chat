"use client";

import { invoke } from "@tauri-apps/api/core";
import { listen } from "@tauri-apps/api/event";
import { openUrl } from "@tauri-apps/plugin-opener";

import { isDesktopApp } from "@/shared/platform/runtime";

// Desktop OAuth transport (RFC 8252 native-app flow).
//
// The browser build lets the provider redirect the whole page back to
// /auth/callback. Inside the Tauri webview that is neither possible (the
// provider cannot redirect into tauri://) nor desirable (the user should sign
// in with their system browser session). So the shell:
//   1. binds an ephemeral loopback port and hands back the redirect URI,
//   2. opens the provider's authorization URL in the system browser,
//   3. emits the loopback callback URL as a Tauri event once the redirect lands.
// The web app then navigates itself to /auth/callback with those parameters and
// the shared callback page handles state verification and the grant exchange.

/** Client id registered with the provider-auth bridge for the desktop shell. */
export const DESKTOP_OAUTH_CLIENT_ID = "com.deeix.chat.desktop";
export const WEB_OAUTH_CLIENT_ID = "deeix-web";

const CALLBACK_EVENT = "oauth-loopback-callback";

export function resolveOAuthClientId(): string {
  return isDesktopApp() ? DESKTOP_OAUTH_CLIENT_ID : WEB_OAUTH_CLIENT_ID;
}

/**
 * Start listening for the provider redirect. Returns the redirect URI to send
 * to the server. The listener lives until a request arrives, `stopOAuthLoopback`
 * is called, or a 10 minute timeout elapses.
 */
export async function startOAuthLoopback(): Promise<string> {
  return invoke<string>("start_oauth_loopback");
}

export async function stopOAuthLoopback(): Promise<void> {
  try {
    await invoke("stop_oauth_loopback");
  } catch {
    // Nothing to stop.
  }
}

/** Open the authorization URL in the user's default browser. */
export async function openInSystemBrowser(url: string): Promise<void> {
  await openUrl(url);
}

/**
 * Resolve with the loopback callback URL when the provider redirects back.
 * Rejects when the listener is cancelled or times out.
 */
export function waitForOAuthCallback(signal?: AbortSignal): Promise<URL> {
  return new Promise((resolve, reject) => {
    let unlisten: (() => void) | null = null;
    const cleanup = () => {
      unlisten?.();
      signal?.removeEventListener("abort", onAbort);
    };
    const onAbort = () => {
      cleanup();
      void stopOAuthLoopback();
      reject(new DOMException("OAuth sign-in cancelled", "AbortError"));
    };
    signal?.addEventListener("abort", onAbort, { once: true });

    void listen<string>(CALLBACK_EVENT, (event) => {
      cleanup();
      try {
        resolve(new URL(event.payload));
      } catch (error) {
        reject(error);
      }
    }).then((fn) => {
      unlisten = fn;
      if (signal?.aborted) {
        onAbort();
      }
    }, reject);
  });
}
