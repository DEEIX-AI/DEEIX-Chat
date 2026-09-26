"use client";

import { check } from "@tauri-apps/plugin-updater";

// Auto-update, desktop only. The updater plugin verifies the signature of every
// artifact against the public key in tauri.conf.json before installing, so a
// compromised release endpoint cannot push arbitrary code.
//
// Release channels: the endpoint points at a single channel today. A beta channel
// means a second endpoint plus a second signing key, not a second code path.

export type UpdateCheckResult =
  | { kind: "unsupported" }
  | { kind: "up-to-date" }
  | { kind: "available"; version: string; install: () => Promise<void> }
  | { kind: "failed"; message: string };

const isAvailable = () => typeof window !== "undefined" && "isTauri" in window;

/** Ask the release endpoint whether a newer version exists. */
export async function checkForUpdate(): Promise<UpdateCheckResult> {
  if (!isAvailable()) {
    return { kind: "unsupported" };
  }
  try {
    const update = await check();
    if (!update) {
      return { kind: "up-to-date" };
    }
    return {
      kind: "available",
      version: update.version,
      install: async () => {
        await update.downloadAndInstall();
      },
    };
  } catch (error) {
    return { kind: "failed", message: error instanceof Error ? error.message : String(error) };
  }
}

/**
 * Relaunch into the installed update. Kept separate from `install` because the
 * caller decides when it is safe to interrupt the user.
 */
export async function relaunchApp(): Promise<void> {
  if (!isAvailable()) {
    return;
  }
  const { relaunch } = await import("@tauri-apps/plugin-process");
  await relaunch();
}
