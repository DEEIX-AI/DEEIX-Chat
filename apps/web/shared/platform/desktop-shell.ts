"use client";

import { invoke } from "@tauri-apps/api/core";
import { listen } from "@tauri-apps/api/event";

// Typed bindings for the Tauri shell's session commands (apps/desktop/src-tauri/src/session.rs).
// This is the whole surface the webview has for the long-lived credential:
// it can hand a refresh token over once, ask for an access token, or sign out.
// It can never read the token back or point it at a different server.

export type ShellSessionError = {
  kind: "network" | "http" | "storage" | "no_session" | "invalid_origin" | "sidecar" | "no_server" | "tabs";
  message: string;
  status?: number;
  errorCode?: string;
};

export function isShellSessionError(value: unknown): value is ShellSessionError {
  return typeof value === "object" && value !== null && "kind" in value && "message" in value;
}

export type ShellCredentials = { accessToken: string; sessionID: string };

export type ServerMode = "local" | "remote";

/** The configured server. In local mode `origin` is the live sidecar address. */
export type ServerInfo = { mode: ServerMode; origin: string };

/** Configured server, or null on first run. Starts the local sidecar when needed. */
export function getServer(): Promise<ServerInfo | null> {
  return invoke<ServerInfo | null>("get_server");
}

/** Use a remote server. Switching servers drops the previous session. */
export function setRemoteServer(origin: string): Promise<ServerInfo> {
  return invoke<ServerInfo>("set_remote_server", { origin });
}

/** Use the bundled local server (SQLite + local storage). */
export function setLocalServer(): Promise<ServerInfo> {
  return invoke<ServerInfo>("set_local_server");
}

/** Local mode: sign in with the sidecar's one-time grant; the grant never reaches JS. */
export function localSignIn(): Promise<ShellCredentials> {
  return invoke<ShellCredentials>("local_sign_in");
}

/** Drop this tab's credential and return it to the setup screen. */
export function leaveServer(): Promise<void> {
  return invoke("leave_server");
}

/** Store the refresh token issued at login. One-way: there is no read. */
export function storeSession(refreshToken: string): Promise<void> {
  return invoke("store_session", { refreshToken });
}

export function clearSession(): Promise<void> {
  return invoke("clear_session");
}

/** Rotate the stored refresh token and return short-lived credentials. */
export function refreshSession(): Promise<ShellCredentials> {
  return invoke<ShellCredentials>("refresh_session");
}

// ---------- tabs (apps/desktop/src-tauri/src/tabs.rs) ----------
// Only the tab strip webview is granted these; content tabs never see them.

export type ShellTab = { id: string; server: ServerInfo | null; title: string };
export type ShellTabs = { tabs: ShellTab[]; active: string | null; platform: "macos" | "windows" | "linux" | string };

const TABS_CHANGED_EVENT = "tabs:changed";

/** Subscribe to tab changes; returns an unsubscribe function. */
export function onTabsChanged(handler: (tabs: ShellTabs) => void): () => void {
  const unlisten = listen<ShellTabs>(TABS_CHANGED_EVENT, (event) => handler(event.payload));
  return () => {
    void unlisten.then((fn) => fn());
  };
}

export function listTabs(): Promise<ShellTabs> {
  return invoke<ShellTabs>("tabs_list");
}

export function openTab(): Promise<ShellTab> {
  return invoke<ShellTab>("tabs_open");
}

/** Show a tab. `focus` moves keyboard focus into it (not while a press is in progress). */
export function activateTab(id: string, focus = true): Promise<void> {
  return invoke("tabs_activate", { id, focus });
}

export function closeTab(id: string): Promise<void> {
  return invoke("tabs_close", { id });
}

export function moveTab(id: string, index: number): Promise<void> {
  return invoke("tabs_move", { id, index });
}
