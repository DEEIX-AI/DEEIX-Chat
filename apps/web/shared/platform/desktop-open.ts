"use client";

import { openUrl } from "@tauri-apps/plugin-opener";

import { isDesktopApp } from "./runtime";

/** Open an http(s) URL in the system browser (new tab in a browser build). */
export async function openExternal(url: string): Promise<void> {
  if (isDesktopApp()) {
    await openUrl(url);
    return;
  }
  window.open(url, "_blank", "noopener,noreferrer");
}
