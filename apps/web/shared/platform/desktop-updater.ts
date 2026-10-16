"use client";

import { invoke } from "@tauri-apps/api/core";
import { Update } from "@tauri-apps/plugin-updater";

import { isFiniteNumber, isNonEmptyString, isRecord, isString } from "@/shared/lib/type-guards";

// Auto-update, desktop only. The updater plugin verifies the signature of every
// artifact against the public key in tauri.conf.json before installing, so a
// compromised release endpoint cannot push arbitrary code.
//
// Release channels: the endpoint points at a single channel today. A beta channel
// means a second endpoint plus a second signing key, not a second code path.
//
// The check goes through the shell's `check_update`, not the plugin's `check`,
// so it uses the app's network proxy; download and install then run on the
// plugin's resource, which keeps that proxy.

export type UpdateCheckResult =
  | { kind: "unsupported" }
  | { kind: "up-to-date" }
  | { kind: "available"; version: string; install: () => Promise<void> }
  | { kind: "failed"; message: string };

const isAvailable = () => typeof window !== "undefined" && "isTauri" in window;

type UpdateMetadata = ConstructorParameters<typeof Update>[0];

function isUpdateMetadata(value: unknown): value is UpdateMetadata {
  return (
    isRecord(value) &&
    isFiniteNumber(value.rid) &&
    isString(value.currentVersion) &&
    isNonEmptyString(value.version) &&
    isRecord(value.rawJson)
  );
}

/** Ask the release endpoint whether a newer version exists. */
export async function checkForUpdate(): Promise<UpdateCheckResult> {
  if (!isAvailable()) {
    return { kind: "unsupported" };
  }
  try {
    const metadata: unknown = await invoke("check_update");
    if (metadata === null) {
      return { kind: "up-to-date" };
    }
    if (!isUpdateMetadata(metadata)) {
      throw new Error("Unexpected check_update payload");
    }
    const update = new Update(metadata);
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
