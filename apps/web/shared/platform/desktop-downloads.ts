"use client";

import { getCurrentWebview } from "@tauri-apps/api/webview";

// The shell saves `<a download>` files to the Downloads folder (src-tauri/src/downloads.rs)
// and reports each one to the webview that started it; a webview shows no download UI.

export type DesktopDownloadFinished = { fileName: string; success: boolean };

const FINISHED_EVENT = "downloads:finished";

/** Subscribe to downloads this tab started; returns an unsubscribe function. */
export function onDesktopDownloadFinished(handler: (download: DesktopDownloadFinished) => void): () => void {
  const unlisten = getCurrentWebview().listen<DesktopDownloadFinished>(FINISHED_EVENT, (event) => handler(event.payload));
  return () => {
    void unlisten.then((fn) => fn());
  };
}
