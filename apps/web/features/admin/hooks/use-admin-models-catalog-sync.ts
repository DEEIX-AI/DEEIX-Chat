"use client";

import * as React from "react";
import { useTranslations } from "next-intl";
import { toast } from "sonner";

import {
  getAdminLLMModelCatalog,
  getAdminOpenRouterOfficialPricing,
  refreshAdminLLMModelCatalog,
} from "@/features/admin/api";
import type { AdminOfficialPricingCatalogData } from "@/features/admin/api/billing-types";
import type { AdminLLMModelCatalogStatus } from "@/features/admin/api/llm-types";
import { resolveAdminErrorMessage } from "@/features/admin/utils/admin-error";
import { resolveAccessToken } from "@/shared/auth/resolve-access-token";

type UseAdminModelsCatalogSyncOptions = {
  open: boolean;
  /** Receives the freshly synced OpenRouter catalog. */
  onOpenRouterSynced: (data: AdminOfficialPricingCatalogData) => void;
  /** Runs after at least one catalog was synced, so automatic detection can be resolved again. */
  onSynced: () => void;
};

/**
 * Syncs both external catalogs behind the model sheet's automatic detection at once: models.dev
 * (modalities, context window, reasoning) and OpenRouter (context window fallback, official pricing).
 * Opening the sheet loads the models.dev status, which also lets the server refresh data older than 24 hours.
 */
export function useAdminModelsCatalogSync({ open, onOpenRouterSynced, onSynced }: UseAdminModelsCatalogSyncOptions) {
  const t = useTranslations("adminModels");
  const [modelsDevStatus, setModelsDevStatus] = React.useState<AdminLLMModelCatalogStatus | null>(null);
  const [syncing, setSyncing] = React.useState(false);

  React.useEffect(() => {
    if (!open) return;
    let cancelled = false;
    void (async () => {
      try {
        const token = await resolveAccessToken();
        if (!token) return;
        const status = await getAdminLLMModelCatalog(token);
        if (!cancelled) setModelsDevStatus(status);
      } catch {
        // The status only feeds the tooltip; the next open retries.
      }
    })();
    return () => {
      cancelled = true;
    };
  }, [open]);

  const sync = React.useCallback(async () => {
    if (syncing) return;
    setSyncing(true);
    try {
      const token = await resolveAccessToken();
      if (!token) {
        toast.error(t("toast.sessionExpired"), { description: t("toast.signInAgain") });
        return;
      }
      const [modelsDev, openRouter] = await Promise.allSettled([
        refreshAdminLLMModelCatalog(token),
        getAdminOpenRouterOfficialPricing(token, { refresh: true }),
      ]);

      const failures: string[] = [];
      if (modelsDev.status === "fulfilled") {
        setModelsDevStatus(modelsDev.value);
      } else {
        failures.push(t("sheet.catalogSync.failedSource", { source: "models.dev", error: resolveAdminErrorMessage(modelsDev.reason) }));
        // The backend records the failure on the status; reload it so the tooltip is current.
        await getAdminLLMModelCatalog(token).then(setModelsDevStatus, () => undefined);
      }
      // A stale response means OpenRouter was unreachable and the server kept its previous data.
      const openRouterSynced = openRouter.status === "fulfilled" && !openRouter.value.stale;
      if (openRouter.status === "fulfilled") {
        onOpenRouterSynced(openRouter.value);
      }
      if (!openRouterSynced) {
        const error = openRouter.status === "rejected" ? resolveAdminErrorMessage(openRouter.reason) : t("sheet.catalogSync.unreachable");
        failures.push(t("sheet.catalogSync.failedSource", { source: "OpenRouter", error }));
      }

      if (modelsDev.status === "fulfilled" || openRouterSynced) {
        onSynced();
      }
      if (failures.length === 0) {
        toast.success(t("sheet.catalogSync.synced"), {
          description: t("sheet.catalogSync.syncedDescription", {
            modelsDev: modelsDev.status === "fulfilled" ? modelsDev.value.modelCount : 0,
            openRouter: openRouter.status === "fulfilled" ? openRouter.value.items.length : 0,
          }),
        });
      } else if (failures.length === 2) {
        toast.error(t("sheet.catalogSync.failed"), { description: failures.join(" · ") });
      } else {
        toast.warning(t("sheet.catalogSync.partial"), { description: failures.join(" · ") });
      }
    } finally {
      setSyncing(false);
    }
  }, [onOpenRouterSynced, onSynced, syncing, t]);

  return { modelsDevStatus, syncing, sync };
}
