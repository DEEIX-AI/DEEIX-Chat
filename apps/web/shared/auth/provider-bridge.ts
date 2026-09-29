"use client";

import { startProviderAuthBridge, startProviderBindBridge } from "@/shared/api/auth";
import { isDesktopApp } from "@/shared/platform";
import { isRecord } from "@/shared/lib/type-guards";
import {
  openInSystemBrowser,
  resolveOAuthClientId,
  startOAuthLoopback,
  stopOAuthLoopback,
  waitForOAuthCallback,
} from "@/shared/platform/desktop-oauth";

export type ProviderAuthIntent = "login" | "register" | "bind";

/** The handoff request kept in sessionStorage until the provider redirects back. */
export type ProviderBridgeRequest = {
  verifier: string;
  state: string;
  intent: ProviderAuthIntent;
  next: string;
};

// The verifier is needed until the grant exchange, which can happen as late as the end of the
// backend transaction window (providerAuthTransactionTTL, 10m: the provider may call back at its
// very end) plus the grant window that callback opens (providerAuthGrantTTL, 90s), both in
// backend/internal/application/auth/provider_bridge.go. The 30s margin absorbs clock skew and
// redirect latency; past that the server has forgotten both and the stored verifier is useless.
const PROVIDER_BRIDGE_REQUEST_TTL_MS = 10 * 60 * 1000 + 90 * 1000 + 30 * 1000;

type StoredProviderBridgeRequest = {
  verifier: string;
  state: string;
  intent?: string;
  next?: string;
  expiresAt: number;
};

function isStoredProviderBridgeRequest(value: unknown): value is StoredProviderBridgeRequest {
  if (!isRecord(value)) return false;
  const record = value;
  return (
    typeof record.verifier === "string" &&
    typeof record.state === "string" &&
    typeof record.expiresAt === "number" &&
    (record.intent === undefined || typeof record.intent === "string") &&
    (record.next === undefined || typeof record.next === "string")
  );
}

export function providerBridgeStorageKey(slug: string): string {
  return `deeix-chat:oauth:${slug}:bridge`;
}

export function readProviderBridgeRequest(slug: string): ProviderBridgeRequest | null {
  try {
    const raw = window.sessionStorage.getItem(providerBridgeStorageKey(slug));
    if (!raw) return null;
    const parsed: unknown = JSON.parse(raw);
    if (!isStoredProviderBridgeRequest(parsed) || parsed.expiresAt <= Date.now()) {
      clearProviderBridgeRequest(slug);
      return null;
    }
    if (!parsed.verifier || !parsed.state) return null;
    return {
      verifier: parsed.verifier,
      state: parsed.state,
      intent: parsed.intent === "register" ? "register" : parsed.intent === "bind" ? "bind" : "login",
      next: parsed.next ?? "",
    };
  } catch {
    return null;
  }
}

export function clearProviderBridgeRequest(slug: string): void {
  window.sessionStorage.removeItem(providerBridgeStorageKey(slug));
}

function base64URL(bytes: Uint8Array): string {
  let binary = "";
  bytes.forEach((byte) => {
    binary += String.fromCharCode(byte);
  });
  return btoa(binary).replaceAll("+", "-").replaceAll("/", "_").replaceAll("=", "");
}

export async function createProviderPKCE() {
  const verifierBytes = new Uint8Array(48);
  window.crypto.getRandomValues(verifierBytes);
  const verifier = base64URL(verifierBytes);
  const digest = await window.crypto.subtle.digest("SHA-256", new TextEncoder().encode(verifier));
  return {
    verifier,
    challenge: base64URL(new Uint8Array(digest)),
  };
}

export function createProviderClientState(): string {
  const bytes = new Uint8Array(32);
  window.crypto.getRandomValues(bytes);
  return base64URL(bytes);
}

/**
 * Starts a provider authorization.
 *
 * In the browser the document navigates to the provider and the promise never
 * settles for the page. In the desktop shell the provider opens in the system
 * browser, the shell waits on its loopback listener (RFC 8252), and the resolved
 * value is the in-app callback path the caller should route to.
 */
export async function beginProviderAuthorization(
  input:
    | { slug: string; intent: "login" | "register"; next: string }
    | { slug: string; intent: "bind"; next: string; accessToken: string },
): Promise<string | null> {
  const desktop = isDesktopApp();
  const pkce = await createProviderPKCE();
  const clientState = createProviderClientState();
  const redirectURI = desktop
    ? await startOAuthLoopback()
    : `${window.location.origin}/auth/callback?provider=${encodeURIComponent(input.slug)}`;
  window.sessionStorage.setItem(
    providerBridgeStorageKey(input.slug),
    JSON.stringify({
      verifier: pkce.verifier,
      state: clientState,
      intent: input.intent,
      next: input.next,
      expiresAt: Date.now() + PROVIDER_BRIDGE_REQUEST_TTL_MS,
    } satisfies StoredProviderBridgeRequest),
  );

  const startInput = {
    clientID: resolveOAuthClientId(),
    redirectURI,
    codeChallenge: pkce.challenge,
    clientState,
    next: input.next,
  };
  try {
    const result = input.intent === "bind"
      ? await startProviderBindBridge(input.slug, { ...startInput, accessToken: input.accessToken })
      : await startProviderAuthBridge(input.slug, { ...startInput, intent: input.intent });
    if (!desktop) {
      // OAuth must use a full document navigation so the provider redirect can leave the app origin.
      window.location.href = result.authorizationURL;
      return null;
    }
    await openInSystemBrowser(result.authorizationURL);
    const callback = await waitForOAuthCallback();
    // The shared callback page verifies the state and exchanges the grant.
    return `/auth/callback${callback.search}`;
  } catch (error) {
    clearProviderBridgeRequest(input.slug);
    if (desktop) {
      void stopOAuthLoopback();
    }
    throw error;
  }
}
