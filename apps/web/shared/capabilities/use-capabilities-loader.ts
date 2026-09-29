"use client";

import { ALL_ENABLED, type CapabilityFlags } from "@deeix/core";
import * as React from "react";

import { getCapabilities } from "@/shared/api/capabilities";
import { registerFeatureDisabledListener, resolveApiBaseURL } from "@/shared/api/http-client";

export type CapabilitiesState = {
  flags: CapabilityFlags;
  /** False until the first response (or fallback) for the current server arrived. */
  loaded: boolean;
};

export const INITIAL_CAPABILITIES_STATE: CapabilitiesState = { flags: ALL_ENABLED, loaded: false };

// One in-flight request per server, shared by every mounted provider. Requests
// bypass the HTTP cache, so this map is the only cache and clearing it re-fetches.
const requests = new Map<string, Promise<CapabilityFlags>>();
const invalidationListeners = new Set<() => void>();

function requestCapabilities(server: string): Promise<CapabilityFlags> {
  let request = requests.get(server);
  if (!request) {
    request = getCapabilities();
    requests.set(server, request);
  }
  return request;
}

// A feature.disabled response means the server's capabilities differ from the
// loaded copy; every provider re-fetches.
function invalidateCapabilities(): void {
  requests.clear();
  for (const listener of invalidationListeners) {
    listener();
  }
}

/** Loads the current server's capability flags and re-loads them whenever the server reports a disabled feature. */
export function useCapabilitiesLoader(): CapabilitiesState {
  const server = resolveApiBaseURL();
  const [state, setState] = React.useState<CapabilitiesState>(INITIAL_CAPABILITIES_STATE);

  React.useEffect(() => {
    registerFeatureDisabledListener(invalidateCapabilities);
  }, []);

  React.useEffect(() => {
    let active = true;
    const load = () => {
      void requestCapabilities(server).then((flags) => {
        if (active) {
          setState({ flags, loaded: true });
        }
      });
    };
    setState((current) => ({ ...current, loaded: false }));
    load();
    invalidationListeners.add(load);
    return () => {
      active = false;
      invalidationListeners.delete(load);
    };
  }, [server]);

  return state;
}
