"use client";

import * as React from "react";

import type { PendingExchangeMap } from "@/features/chat/types/chat-runtime";
import { getConversationRunStatuses } from "@/shared/api/conversation";
import { resolveAccessToken } from "@/shared/auth/resolve-access-token";

type QueuedParentRun = {
  parentRunID: string | null;
};

function isTerminalRunStatus(status: string | null | undefined): boolean {
  const normalized = status?.trim().toLowerCase() || "";
  return ["success", "interrupted", "error", "canceled", "cancelled", "blocked", "unavailable"].includes(normalized);
}

export function useChatHiddenRuns({
  queuedParents,
  getPendingExchanges,
  isRunActive,
}: {
  queuedParents: QueuedParentRun[];
  getPendingExchanges: () => PendingExchangeMap;
  isRunActive: (runID: string) => boolean;
}) {
  const statusesRef = React.useRef(new Map<string, string>());
  const [revision, setRevision] = React.useState(0);

  React.useEffect(() => {
    const locallyTrackedRunIDs = new Set(
      Object.values(getPendingExchanges())
        .map((exchange) => exchange.runID?.trim() || "")
        .filter(Boolean),
    );
    const watchedRunIDs = new Set<string>();
    const pendingRunIDs = new Set<string>();
    for (const parent of queuedParents) {
      const parentRunID = parent.parentRunID?.trim() || "";
      if (!parentRunID) {
        continue;
      }
      watchedRunIDs.add(parentRunID);
      if (
        isRunActive(parentRunID) ||
        locallyTrackedRunIDs.has(parentRunID) ||
        isTerminalRunStatus(statusesRef.current.get(parentRunID))
      ) {
        continue;
      }
      pendingRunIDs.add(parentRunID);
    }
    for (const runID of statusesRef.current.keys()) {
      if (!watchedRunIDs.has(runID)) {
        statusesRef.current.delete(runID);
      }
    }
    if (pendingRunIDs.size === 0) {
      return;
    }

    let cancelled = false;
    let failureCount = 0;
    const missingSinceByRunID = new Map<string, number>();
    let pollTimer: number | null = null;
    let requestController: AbortController | null = null;
    const schedule = () => {
      if (!cancelled && !document.hidden && pendingRunIDs.size > 0) {
        pollTimer = window.setTimeout(poll, 1500 * Math.min(2 ** failureCount, 4));
      }
    };
    const poll = async () => {
      if (cancelled || document.hidden || pendingRunIDs.size === 0 || requestController !== null) {
        return;
      }
      requestController = new AbortController();
      const controller = requestController;
      try {
        const token = await resolveAccessToken();
        if (!token || cancelled || controller.signal.aborted || document.hidden) {
          return;
        }
        const statuses = await getConversationRunStatuses(token, Array.from(pendingRunIDs), controller.signal);
        if (cancelled || controller.signal.aborted) {
          return;
        }
        failureCount = 0;
        let changed = false;
        const returnedRunIDs = new Set<string>();
        for (const item of statuses) {
          const runID = item.runID.trim();
          const status = item.status.trim().toLowerCase();
          if (!pendingRunIDs.has(runID) || !status) {
            continue;
          }
          returnedRunIDs.add(runID);
          missingSinceByRunID.delete(runID);
          if (statusesRef.current.get(runID) !== status) {
            statusesRef.current.set(runID, status);
            changed = true;
          }
          if (isTerminalRunStatus(status)) {
            pendingRunIDs.delete(runID);
          }
        }
        const now = Date.now();
        for (const runID of pendingRunIDs) {
          if (returnedRunIDs.has(runID)) {
            continue;
          }
          const missingSince = missingSinceByRunID.get(runID);
          if (missingSince === undefined) {
            missingSinceByRunID.set(runID, now);
            continue;
          }
          if (now - missingSince < 60_000) {
            continue;
          }
          statusesRef.current.set(runID, "unavailable");
          pendingRunIDs.delete(runID);
          missingSinceByRunID.delete(runID);
          changed = true;
        }
        if (changed) {
          setRevision((current) => current + 1);
        }
      } catch {
        if (!controller.signal.aborted) {
          failureCount += 1;
        }
      } finally {
        if (requestController === controller) {
          requestController = null;
          schedule();
        }
      }
    };

    const handleVisibilityChange = () => {
      if (pollTimer !== null) {
        window.clearTimeout(pollTimer);
        pollTimer = null;
      }
      if (document.hidden) {
        requestController?.abort();
      } else {
        void poll();
      }
    };

    document.addEventListener("visibilitychange", handleVisibilityChange);
    void poll();
    return () => {
      cancelled = true;
      requestController?.abort();
      document.removeEventListener("visibilitychange", handleVisibilityChange);
      if (pollTimer !== null) {
        window.clearTimeout(pollTimer);
      }
    };
  }, [getPendingExchanges, isRunActive, queuedParents]);

  const getStatus = React.useCallback((runID: string) => statusesRef.current.get(runID.trim()) || "", []);

  return {
    getStatus,
    revision,
  };
}
