"use client";

import * as React from "react";

import type {
  AdminOfficialPricingCatalogData,
  AdminOfficialPricingCatalogItemDTO,
} from "@/features/admin/api/billing-types";
import { getAdminOpenRouterOfficialPricing } from "@/features/admin/api/billing";
import { resolveAccessToken } from "@/shared/auth/resolve-access-token";

export type OpenRouterCatalogState = {
  status: "idle" | "loaded" | "unavailable";
  items: AdminOfficialPricingCatalogItemDTO[];
  /** When the data was fetched from OpenRouter (or the builtin snapshot date); set once loaded. */
  fetchedAt?: string;
};

// OpenRouter's catalog is the fallback source of automatic context windows in the model sheet (after models.dev).
// Failures are silent: the sheet falls back to backend defaults.
export function useAdminModelsOpenrouterCatalog(open: boolean) {
  const [openRouterCatalog, setOpenRouterCatalog] = React.useState<OpenRouterCatalogState>({
    status: "idle",
    items: [],
  });
  // Shares one in-flight request between the auto-load effect and submit.
  const openRouterCatalogRequestRef = React.useRef<Promise<AdminOfficialPricingCatalogItemDTO[] | null> | null>(null);

  const loadOpenRouterCatalog = React.useCallback(async (
    accessToken?: string,
  ): Promise<AdminOfficialPricingCatalogItemDTO[] | null> => {
    if (openRouterCatalog.status === "loaded") {
      return openRouterCatalog.items;
    }
    if (openRouterCatalogRequestRef.current) {
      return openRouterCatalogRequestRef.current;
    }
    const request = (async () => {
      try {
        const token = accessToken ?? await resolveAccessToken();
        if (!token) {
          setOpenRouterCatalog({ status: "unavailable", items: [] });
          return null;
        }
        const result = await getAdminOpenRouterOfficialPricing(token);
        setOpenRouterCatalog({ status: "loaded", items: result.items, fetchedAt: result.fetchedAt });
        return result.items;
      } catch {
        setOpenRouterCatalog({ status: "unavailable", items: [] });
        return null;
      } finally {
        openRouterCatalogRequestRef.current = null;
      }
    })();
    openRouterCatalogRequestRef.current = request;
    return request;
  }, [openRouterCatalog]);

  React.useEffect(() => {
    if (!open || openRouterCatalog.status !== "idle") {
      return;
    }
    void loadOpenRouterCatalog();
  }, [loadOpenRouterCatalog, open, openRouterCatalog.status]);

  /** Replaces the catalog with freshly synced data (see the model catalog sync button). */
  const replaceOpenRouterCatalog = React.useCallback((data: AdminOfficialPricingCatalogData) => {
    setOpenRouterCatalog({ status: "loaded", items: data.items, fetchedAt: data.fetchedAt });
  }, []);

  return { openRouterCatalog, loadOpenRouterCatalog, replaceOpenRouterCatalog };
}
