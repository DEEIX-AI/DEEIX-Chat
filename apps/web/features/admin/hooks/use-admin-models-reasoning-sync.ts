import * as React from "react";
import { useTranslations } from "next-intl";
import { toast } from "sonner";

import { getAdminLLMModelCatalog, refreshAdminLLMModelCatalog } from "@/features/admin/api";
import type { AdminLLMModelCatalogStatus } from "@/features/admin/api/llm-types";
import { resolveAdminErrorMessage } from "@/features/admin/utils/admin-error";
import { resolveAccessToken } from "@/shared/auth/resolve-access-token";

// While the server syncs in the background, poll the status at this interval, for at most this long
// (the server-side fetch times out well before).
const STATUS_POLL_INTERVAL_MS = 2000;
const STATUS_POLL_MAX_MS = 2 * 60 * 1000;

/**
 * Reasoning capability sync. Loading the status (on mount, i.e. when the capabilities dialog opens)
 * lets the server sync in the background when its data is older than 24 hours, the same on-demand
 * policy as the OpenRouter official pricing; the status is followed until that sync finishes. The
 * button syncs immediately.
 */
export function useAdminModelsReasoningSync() {
  const t = useTranslations("adminModels");
  const [status, setStatus] = React.useState<AdminLLMModelCatalogStatus | null>(null);
  const [syncing, setSyncing] = React.useState(false);

  const withToken = React.useCallback(async (): Promise<string | null> => {
    const token = await resolveAccessToken();
    if (!token) {
      toast.error(t("toast.sessionExpired"), { description: t("toast.signInAgain") });
      return null;
    }
    return token;
  }, [t]);

  React.useEffect(() => {
    let cancelled = false;
    let timer: ReturnType<typeof setTimeout> | undefined;
    const startedAt = Date.now();
    const load = async () => {
      const token = await resolveAccessToken();
      if (!token || cancelled) return;
      try {
        const nextStatus = await getAdminLLMModelCatalog(token);
        if (cancelled) return;
        setStatus(nextStatus);
        if (nextStatus.refreshing && Date.now() - startedAt < STATUS_POLL_MAX_MS) {
          timer = setTimeout(() => void load(), STATUS_POLL_INTERVAL_MS);
        }
      } catch {
        // The status only feeds the button state and tooltip; the next dialog open retries.
      }
    };
    void load();
    return () => {
      cancelled = true;
      if (timer) clearTimeout(timer);
    };
  }, []);

  const sync = React.useCallback(async () => {
    if (syncing) return;
    setSyncing(true);
    try {
      const token = await withToken();
      if (!token) return;
      const nextStatus = await refreshAdminLLMModelCatalog(token);
      setStatus(nextStatus);
      toast.success(t("reasoningSync.toast.synced", { count: nextStatus.modelCount }));
    } catch (error) {
      toast.error(t("reasoningSync.toast.failed"), { description: resolveAdminErrorMessage(error) });
      // The backend records the failure on the status; reload it so the tooltip is current.
      const token = await resolveAccessToken();
      if (token) {
        await getAdminLLMModelCatalog(token).then(setStatus, () => undefined);
      }
    } finally {
      setSyncing(false);
    }
  }, [syncing, t, withToken]);

  return { status, syncing: syncing || Boolean(status?.refreshing), sync };
}
