"use client";

// Server selection for the desktop shell. The choice (local sidecar or a pinned
// remote origin) lives on the Rust side; this module keeps a synchronous copy
// of the live origin for the API client.

import { normalizeApiBaseUrl } from "@deeix/core";

import { getServer, type ServerInfo, setLocalServer, setRemoteServer } from "./desktop-shell";

let current: ServerInfo | null = null;
const originHandlers = new Set<(origin: string) => void>();

function setCurrent(next: ServerInfo | null): void {
  const origin = next?.origin ?? "";
  const changed = origin !== readServerOrigin();
  current = next;
  if (changed) {
    for (const handler of originHandlers) {
      handler(origin);
    }
  }
}

/**
 * Subscribe to origin changes (a server was bound, the local sidecar moved to a
 * new port). The handler receives the new origin, "" when the tab has none;
 * returns an unsubscribe function.
 */
export function onServerOriginChanged(handler: (origin: string) => void): () => void {
  originHandlers.add(handler);
  return () => {
    originHandlers.delete(handler);
  };
}

/** Synchronous read for the API client; empty until `loadServer` ran. */
export function readServerOrigin(): string {
  return current?.origin ?? "";
}

export function readServerMode(): ServerInfo["mode"] | null {
  return current?.mode ?? null;
}

export async function loadServer(): Promise<ServerInfo | null> {
  const server = await getServer();
  setCurrent(server);
  return server;
}

/** Normalise user input; "" when it is not an absolute http(s) URL. */
export function validateApiBaseUrl(raw: string): string {
  return normalizeApiBaseUrl(raw);
}

export async function commitRemoteServer(origin: string): Promise<ServerInfo> {
  const server = await setRemoteServer(origin);
  setCurrent(server);
  return server;
}

export async function commitLocalServer(): Promise<ServerInfo> {
  const server = await setLocalServer();
  setCurrent(server);
  return server;
}
