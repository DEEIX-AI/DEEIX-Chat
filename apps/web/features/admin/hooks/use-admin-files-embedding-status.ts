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

// While a rebuild or queued tasks are in flight, poll quietly so the counts follow the
// background work instead of freezing at the moment the button was clicked.
const STATUS_POLL_INTERVAL_MS = 3000;
// A few consecutive failed polls usually mean the backend is down; stop instead of hammering it.
const MAX_FAILED_POLLS = 5;

function shouldPollStatus(status: AdminEmbeddingIndexStatus): boolean {
  return status.reindexRunning || status.activeCount > 0;
}

/**
 * Embedding index status for the files settings page. Loads are versioned and
 * abortable; while background work is running the status refreshes itself on
 * an interval, and every newer load or a clear drops the pending refresh.
 */
export function useAdminFilesEmbeddingStatus() {
  const t = useTranslations("adminFiles");
  const [embeddingStatus, setEmbeddingStatus] = React.useState<AdminEmbeddingIndexStatus | null>(null);
  const [embeddingStatusLoading, setEmbeddingStatusLoading] = React.useState(false);
  const [reindexing, setReindexing] = React.useState(false);
  const embeddingRefreshTimerRef = React.useRef<number | null>(null);
  const embeddingStatusRequestRef = React.useRef<AbortController | null>(null);
  const embeddingStatusRequestVersionRef = React.useRef(0);
  const failedPollsRef = React.useRef(0);

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

  // Silent loads keep the current numbers and controls in place: they back the polling and
  // the refreshes after a submission, where a skeleton or disabled buttons would only flicker.
  const fetchEmbeddingStatus = React.useCallback(async (silent: boolean) => {
    cancelEmbeddingStatusRefresh();
    const requestVersion = embeddingStatusRequestVersionRef.current;
    const requestController = new AbortController();
    embeddingStatusRequestRef.current = requestController;
    if (!silent) setEmbeddingStatusLoading(true);
    const schedulePoll = () => {
      embeddingRefreshTimerRef.current = window.setTimeout(function poll() {
        embeddingRefreshTimerRef.current = null;
        // Hidden tabs keep the poll armed without hitting the aggregate query; counts catch up on return.
        if (document.visibilityState === "hidden") {
          embeddingRefreshTimerRef.current = window.setTimeout(poll, STATUS_POLL_INTERVAL_MS);
          return;
        }
        void fetchEmbeddingStatus(true);
      }, STATUS_POLL_INTERVAL_MS);
    };
    try {
      const token = await resolveAccessToken();
      if (!token || requestController.signal.aborted) return;
      const status = await getAdminEmbeddingStatus(token, requestController.signal);
      if (embeddingStatusRequestVersionRef.current !== requestVersion) return;
      failedPollsRef.current = 0;
      setEmbeddingStatus(status);
      if (shouldPollStatus(status)) schedulePoll();
    } catch {
      if (requestController.signal.aborted || embeddingStatusRequestVersionRef.current !== requestVersion) return;
      if (!silent) {
        setEmbeddingStatus(null);
        return;
      }
      // A failed silent refresh keeps the last known numbers and retries a bounded number of times.
      failedPollsRef.current += 1;
      if (failedPollsRef.current < MAX_FAILED_POLLS) schedulePoll();
    } finally {
      if (embeddingStatusRequestRef.current === requestController) {
        embeddingStatusRequestRef.current = null;
      }
      if (embeddingStatusRequestVersionRef.current === requestVersion) {
        setEmbeddingStatusLoading(false);
      }
    }
  }, [cancelEmbeddingStatusRefresh]);

  // Explicit loads and refreshes come from user actions, so each starts with a fresh retry budget.
  const loadEmbeddingStatus = React.useCallback(() => {
    failedPollsRef.current = 0;
    return fetchEmbeddingStatus(false);
  }, [fetchEmbeddingStatus]);
  const refreshEmbeddingStatus = React.useCallback(() => {
    failedPollsRef.current = 0;
    return fetchEmbeddingStatus(true);
  }, [fetchEmbeddingStatus]);

  const handleReindex = React.useCallback(async (includeEmpty = false) => {
    setReindexing(true);
    try {
      const token = await resolveAccessToken();
      if (!token) {
        toast.error(t("toast.sessionExpired"));
        return;
      }
      const result = await triggerAdminEmbeddingReindex(token, { includeEmpty });
      if (result.submitted > 0) {
        toast.success(t("toast.reindexSubmitted"), {
          description: t("toast.reindexSubmittedDescription", { count: result.submitted }),
        });
      } else {
        toast.info(t("toast.reindexNothing"));
      }
      // The rebuild reports itself as running, so this refresh keeps polling until it finishes.
      void refreshEmbeddingStatus();
    } catch (error) {
      toast.error(t("toast.reindexFailed"), { description: resolveAdminErrorMessage(error, t("toast.unknownError")) });
    } finally {
      setReindexing(false);
    }
  }, [refreshEmbeddingStatus, t]);

  return {
    embeddingStatus,
    embeddingStatusLoading,
    reindexing,
    loadEmbeddingStatus,
    refreshEmbeddingStatus,
    clearEmbeddingStatus,
    handleReindex,
  };
}
