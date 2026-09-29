"use client";

import * as React from "react";

import { DEFAULT_DESKTOP_DOWNLOAD_URL, type FeaturePolicy, getFeaturePolicy } from "@/shared/api/settings";
import { resolveAccessToken } from "@/shared/auth/resolve-access-token";

const DEFAULT_FEATURE_POLICY: FeaturePolicy = {
  knowledgeBaseEnabled: true,
  processTraceEnabled: true,
  desktopDownloadEnabled: true,
  desktopDownloadURL: DEFAULT_DESKTOP_DOWNLOAD_URL,
};

let cachedFeaturePolicy: FeaturePolicy | null = null;
let inflightFeaturePolicy: Promise<void> | null = null;
const featurePolicyListeners = new Set<() => void>();

function setCachedFeaturePolicy(policy: FeaturePolicy) {
  cachedFeaturePolicy = policy;
  for (const listener of featurePolicyListeners) listener();
}

/** Call after the admin successfully changes feature toggles so consumers in the current tab take effect immediately. */
export function overrideFeaturePolicy(patch: Partial<FeaturePolicy>) {
  setCachedFeaturePolicy({ ...(cachedFeaturePolicy ?? DEFAULT_FEATURE_POLICY), ...patch });
}

function loadFeaturePolicy(): Promise<void> {
  inflightFeaturePolicy ??= (async () => {
    try {
      const token = await resolveAccessToken();
      if (!token) {
        setCachedFeaturePolicy(DEFAULT_FEATURE_POLICY);
        return;
      }
      // Fields added after a deployment may be absent from an older server; unknown means enabled.
      setCachedFeaturePolicy({ ...DEFAULT_FEATURE_POLICY, ...(await getFeaturePolicy(token)) });
    } catch {
      // On fetch failure, treat everything as enabled (fail-open) to avoid breaking healthy deployments or blocking route guards.
      setCachedFeaturePolicy(DEFAULT_FEATURE_POLICY);
    } finally {
      inflightFeaturePolicy = null;
    }
  })();
  return inflightFeaturePolicy;
}

function subscribeFeaturePolicy(listener: () => void): () => void {
  featurePolicyListeners.add(listener);
  return () => featurePolicyListeners.delete(listener);
}

function getFeaturePolicySnapshot(): FeaturePolicy | null {
  return cachedFeaturePolicy;
}

/**
 * Reads the admin feature toggle policy (session-level cache, fetched once on first mount, all subscribers notified on cache update).
 * Returns all-enabled defaults until loaded; `loaded` is for cases that must wait for a definitive result (e.g. route guards).
 */
export function useFeaturePolicy(): FeaturePolicy & { loaded: boolean } {
  const policy = React.useSyncExternalStore(
    subscribeFeaturePolicy,
    getFeaturePolicySnapshot,
    getFeaturePolicySnapshot,
  );

  React.useEffect(() => {
    if (!cachedFeaturePolicy) void loadFeaturePolicy();
  }, []);

  return policy ? { ...policy, loaded: true } : { ...DEFAULT_FEATURE_POLICY, loaded: false };
}
