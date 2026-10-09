"use client";

import * as React from "react";

import { getPersonalProviderAccess } from "@/shared/api/personal-providers";
import type { PersonalProviderAccessDTO } from "@/shared/api/personal-providers-types";
import { resolveAccessToken } from "@/shared/auth/resolve-access-token";

const DISABLED_ACCESS: PersonalProviderAccessDTO = {
  enabled: false,
  maxPerUser: 0,
  protocols: [],
  modelProtocols: [],
};

let cachedAccess: PersonalProviderAccessDTO | null = null;
let inflightAccess: Promise<void> | null = null;
const accessListeners = new Set<() => void>();

function setCachedAccess(access: PersonalProviderAccessDTO) {
  cachedAccess = access;
  for (const listener of accessListeners) listener();
}

/** Drop the cache so the next reader refetches, e.g. after an admin changes the policy. */
export function invalidatePersonalProviderAccess() {
  cachedAccess = null;
  void loadAccess();
}

function loadAccess(): Promise<void> {
  inflightAccess ??= (async () => {
    try {
      const token = await resolveAccessToken();
      // Unlike feature toggles this fails closed: a user-key feature must not appear on a guess.
      setCachedAccess(token ? await getPersonalProviderAccess(token) : DISABLED_ACCESS);
    } catch {
      setCachedAccess(DISABLED_ACCESS);
    } finally {
      inflightAccess = null;
    }
  })();
  return inflightAccess;
}

function subscribe(listener: () => void): () => void {
  accessListeners.add(listener);
  return () => accessListeners.delete(listener);
}

function getSnapshot(): PersonalProviderAccessDTO | null {
  return cachedAccess;
}

/**
 * Whether the current user may bring their own model provider keys
 * (session-level cache shared by the settings sidebar, page and import screen).
 * Reports disabled until loaded; `loaded` lets route guards wait for the answer.
 */
export function usePersonalProviderAccess(): PersonalProviderAccessDTO & { loaded: boolean } {
  const access = React.useSyncExternalStore(subscribe, getSnapshot, getSnapshot);

  React.useEffect(() => {
    if (!cachedAccess) void loadAccess();
  }, []);

  return access ? { ...access, loaded: true } : { ...DISABLED_ACCESS, loaded: false };
}
