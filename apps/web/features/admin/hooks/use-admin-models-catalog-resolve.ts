"use client";

import * as React from "react";

import { resolveAdminLLMModelCatalog } from "@/features/admin/api";
import type { AdminLLMModelCatalogResolution } from "@/features/admin/api/llm-types";
import { resolveAccessToken } from "@/shared/auth/resolve-access-token";
import { useDebouncedValue } from "@/shared/hooks/use-debounced-value";

const RESOLVE_DEBOUNCE_MS = 300;

export type AdminModelsCatalogResolveInput = {
  name: string;
  vendor: string;
  protocols: readonly string[];
  capabilitiesJSON: string;
};

export type AdminModelsCatalogResolveState = {
  status: "idle" | "loading" | "loaded" | "error";
  /** Last successful result; kept while a newer request is loading so the rows do not flicker. */
  result: AdminLLMModelCatalogResolution | null;
};

const IDLE_STATE: AdminModelsCatalogResolveState = { status: "idle", result: null };

/**
 * Asks the server what the model form will detect automatically (models.dev modalities and context
 * window, reasoning levels) for the current, possibly unsaved, values.
 */
export function useAdminModelsCatalogResolve(
  open: boolean,
  input: AdminModelsCatalogResolveInput,
  /** Bump to resolve again with unchanged input, e.g. after the catalogs were synced. */
  revision = 0,
): AdminModelsCatalogResolveState {
  const protocolsKey = input.protocols.join("\n");
  const payload = React.useMemo(
    () => ({
      name: input.name.trim(),
      vendor: input.vendor.trim(),
      protocols: protocolsKey ? protocolsKey.split("\n") : [],
      capabilitiesJSON: input.capabilitiesJSON,
    }),
    [input.capabilitiesJSON, input.name, input.vendor, protocolsKey],
  );
  const query = useDebouncedValue(payload, RESOLVE_DEBOUNCE_MS);
  const [state, setState] = React.useState<AdminModelsCatalogResolveState>(IDLE_STATE);

  React.useEffect(() => {
    if (!open || !query.name) {
      setState(IDLE_STATE);
      return;
    }
    let cancelled = false;
    setState((previous) => ({ status: "loading", result: previous.result }));
    void (async () => {
      try {
        const token = await resolveAccessToken();
        if (!token) {
          throw new Error("missing access token");
        }
        const result = await resolveAdminLLMModelCatalog(token, query);
        if (!cancelled) setState({ status: "loaded", result });
      } catch {
        // Keep the last result so a transient failure does not blank the rows.
        if (!cancelled) setState((previous) => ({ status: "error", result: previous.result }));
      }
    })();
    return () => {
      cancelled = true;
    };
  }, [open, query, revision]);

  return state;
}
