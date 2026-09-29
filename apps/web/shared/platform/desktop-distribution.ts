"use client";

import { invoke } from "@tauri-apps/api/core";

import { isBoolean, isNonEmptyString, isOneOf, isRecord, isString } from "@/shared/lib/type-guards";

import { isDesktopApp } from "./runtime";

// How this desktop build was distributed and what enterprise policy allows.
// The shell (apps/desktop/src-tauri) decides and enforces both; the web app only
// reflects them. Payloads are validated at runtime because they cross the IPC
// boundary as untyped JSON.

export const DISTRIBUTION_KINDS = ["installer", "msi", "portable", "dev"] as const;
export const UPDATE_MODES = ["auto", "notify", "disabled"] as const;

export type DistributionKind = (typeof DISTRIBUTION_KINDS)[number];
export type UpdateMode = (typeof UPDATE_MODES)[number];

export type DistributionPolicy = {
  defaultServerUrl: string | null;
  /** True only when the address is locked AND a default address is configured. */
  serverUrlLocked: boolean;
  localModeAllowed: boolean;
  autoUpdateDisabledByPolicy: boolean;
};

export type DistributionInfo = {
  kind: DistributionKind;
  updateMode: UpdateMode;
  /** Download page for manual updates. */
  releasesUrl: string;
  policy: DistributionPolicy;
};

/** A newer release found in `notify` mode; nothing is downloaded. */
export type UpdateNotice = { version: string; currentVersion: string };

const isDistributionKind = isOneOf(DISTRIBUTION_KINDS);
const isUpdateMode = isOneOf(UPDATE_MODES);

function isDistributionPolicy(value: unknown): value is DistributionPolicy {
  return (
    isRecord(value) &&
    (value.defaultServerUrl === null || isString(value.defaultServerUrl)) &&
    isBoolean(value.serverUrlLocked) &&
    isBoolean(value.localModeAllowed) &&
    isBoolean(value.autoUpdateDisabledByPolicy)
  );
}

export function isDistributionInfo(value: unknown): value is DistributionInfo {
  return (
    isRecord(value) &&
    isDistributionKind(value.kind) &&
    isUpdateMode(value.updateMode) &&
    isString(value.releasesUrl) &&
    isDistributionPolicy(value.policy)
  );
}

export function isUpdateNotice(value: unknown): value is UpdateNotice {
  return isRecord(value) && isNonEmptyString(value.version) && isString(value.currentVersion);
}

/**
 * What a browser (or a shell that cannot report its distribution) gets: no
 * update checks and no policy. Rust still enforces the real policy.
 */
export const FALLBACK_DISTRIBUTION: DistributionInfo = {
  kind: "dev",
  updateMode: "disabled",
  releasesUrl: "",
  policy: {
    defaultServerUrl: null,
    serverUrlLocked: false,
    localModeAllowed: true,
    autoUpdateDisabledByPolicy: false,
  },
};

/** Ask the shell for its distribution. Browsers resolve with the fallback. */
export async function getDistribution(): Promise<DistributionInfo> {
  if (!isDesktopApp()) {
    return FALLBACK_DISTRIBUTION;
  }
  const payload: unknown = await invoke("get_distribution");
  if (!isDistributionInfo(payload)) {
    throw new Error("Unexpected get_distribution payload");
  }
  return payload;
}

let cached: Promise<DistributionInfo> | null = null;

/** Memoised `getDistribution`: the answer cannot change while the app runs. */
export function loadDistribution(): Promise<DistributionInfo> {
  cached ??= getDistribution().catch((error: unknown) => {
    // Let a later caller retry instead of caching the failure.
    cached = null;
    throw error;
  });
  return cached;
}

/**
 * `notify` mode: ask the release endpoint whether a newer version exists without
 * downloading anything. Null when up to date (and always in a browser).
 */
export async function checkUpdateNotice(): Promise<UpdateNotice | null> {
  if (!isDesktopApp()) {
    return null;
  }
  const payload: unknown = await invoke("check_update_notice");
  if (payload === null) {
    return null;
  }
  if (!isUpdateNotice(payload)) {
    throw new Error("Unexpected check_update_notice payload");
  }
  return payload;
}

/** True when any enterprise policy value is in effect. */
export function hasActivePolicy(policy: DistributionPolicy): boolean {
  return (
    policy.defaultServerUrl !== null ||
    policy.serverUrlLocked ||
    !policy.localModeAllowed ||
    policy.autoUpdateDisabledByPolicy
  );
}

const POLICY_ERROR_PREFIX = "policy:";

/**
 * True when `set_remote_server` / `set_local_server` rejected because enterprise
 * policy forbids the choice. The shell rejects with a string starting with
 * `policy:`; an object carrying such a message is accepted too.
 */
export function isPolicyError(error: unknown): boolean {
  const text = isString(error) ? error : isRecord(error) && isString(error.message) ? error.message : "";
  return text.trimStart().startsWith(POLICY_ERROR_PREFIX);
}
