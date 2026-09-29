import * as React from "react";
import { useTranslations } from "next-intl";
import { toast } from "sonner";

import {
  type AdminEmbeddingIndexStatus,
  getAdminEmbeddingStatus,
  triggerAdminEmbeddingReindex,
} from "@/features/admin/api";
import { resolveAdminErrorMessage } from "@/features/admin/utils/admin-error";
import { resolveAccessToken } from "@/shared/auth/resolve-access-token";

/**
 * Embedding index status for the files settings page. Loads are versioned and
 * abortable; a reindex schedules one delayed status refresh, which is dropped
 * if a newer load or a clear happened meanwhile.
 */
export function useAdminFilesEmbeddingStatus() {
  const t = useTranslations("adminFiles");
  const [embeddingStatus, setEmbeddingStatus] = React.useState<AdminEmbeddingIndexStatus | null>(null);
  const [embeddingStatusLoading, setEmbeddingStatusLoading] = React.useState(false);
  const [reindexing, setReindexing] = React.useState(false);
  const embeddingRefreshTimerRef = React.useRef<number | null>(null);
  const embeddingStatusRequestRef = React.useRef<AbortController | null>(null);
  const embeddingStatusRequestVersionRef = React.useRef(0);

  const cancelEmbeddingStatusRefresh = React.useCallback(() => {
    if (embeddingRefreshTimerRef.current !== null) {
      window.clearTimeout(embeddingRefreshTimerRef.current);
      embeddingRefreshTimerRef.current = null;
    }
    embeddingStatusRequestVersionRef.current += 1;
    embeddingStatusRequestRef.current?.abort();
    embeddingStatusRequestRef.current = null;
  }, []);

  const clearEmbeddingStatus = React.useCallback(() => {
    cancelEmbeddingStatusRefresh();
    setEmbeddingStatus(null);
    setEmbeddingStatusLoading(false);
  }, [cancelEmbeddingStatusRefresh]);

  React.useEffect(
    () => cancelEmbeddingStatusRefresh,
    [cancelEmbeddingStatusRefresh],
  );

  const loadEmbeddingStatus = React.useCallback(async () => {
    cancelEmbeddingStatusRefresh();
    const requestVersion = embeddingStatusRequestVersionRef.current;
    const requestController = new AbortController();
    embeddingStatusRequestRef.current = requestController;
    setEmbeddingStatusLoading(true);
    try {
      const token = await resolveAccessToken();
      if (!token || requestController.signal.aborted) return;
      const status = await getAdminEmbeddingStatus(token, requestController.signal);
      if (embeddingStatusRequestVersionRef.current !== requestVersion) return;
      setEmbeddingStatus(status);
    } catch {
      if (
        !requestController.signal.aborted &&
        embeddingStatusRequestVersionRef.current === requestVersion
      ) setEmbeddingStatus(null);
    } finally {
      if (embeddingStatusRequestRef.current === requestController) {
        embeddingStatusRequestRef.current = null;
      }
      if (embeddingStatusRequestVersionRef.current === requestVersion) {
        setEmbeddingStatusLoading(false);
      }
    }
  }, [cancelEmbeddingStatusRefresh]);

  const handleReindex = React.useCallback(async (includeEmpty = false) => {
    const refreshVersion = embeddingStatusRequestVersionRef.current;
    setReindexing(true);
    try {
      const token = await resolveAccessToken();
      if (!token) {
        toast.error(t("toast.sessionExpired"));
        return;
      }
      const result = await triggerAdminEmbeddingReindex(token, { includeEmpty });
      toast.success(t("toast.reindexSubmitted"), {
        description: t("toast.reindexSubmittedDescription", { count: result.submitted }),
      });
      if (embeddingStatusRequestVersionRef.current !== refreshVersion) return;
      if (embeddingRefreshTimerRef.current !== null) {
        window.clearTimeout(embeddingRefreshTimerRef.current);
      }
      embeddingRefreshTimerRef.current = window.setTimeout(() => {
        embeddingRefreshTimerRef.current = null;
        void loadEmbeddingStatus();
      }, 1500);
    } catch (error) {
      toast.error(t("toast.reindexFailed"), { description: resolveAdminErrorMessage(error, t("toast.unknownError")) });
    } finally {
      setReindexing(false);
    }
  }, [loadEmbeddingStatus, t]);

  return {
    embeddingStatus,
    embeddingStatusLoading,
    reindexing,
    loadEmbeddingStatus,
    clearEmbeddingStatus,
    handleReindex,
  };
}
