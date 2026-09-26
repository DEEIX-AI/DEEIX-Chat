"use client";

// Server selection for the desktop shell. The choice (local sidecar or a pinned
// remote origin) lives on the Rust side; this module keeps a synchronous copy
// of the live origin for the API client.

import { normalizeApiBaseUrl } from "@deeix/core";

import { getServer, type ServerInfo, setLocalServer, setRemoteServer } from "./desktop-shell";

let current: ServerInfo | null = null;

/** Synchronous read for the API client; empty until `loadServer` ran. */
export function readServerOrigin(): string {
  return current?.origin ?? "";
}

export function readServerMode(): ServerInfo["mode"] | null {
  return current?.mode ?? null;
}

export async function loadServer(): Promise<ServerInfo | null> {
  current = await getServer();
  return current;
}

/** Normalise user input; "" when it is not an absolute http(s) URL. */
export function validateApiBaseUrl(raw: string): string {
  return normalizeApiBaseUrl(raw);
}

export async function commitRemoteServer(origin: string): Promise<ServerInfo> {
  current = await setRemoteServer(origin);
  return current;
}

export async function commitLocalServer(): Promise<ServerInfo> {
  current = await setLocalServer();
  return current;
}
