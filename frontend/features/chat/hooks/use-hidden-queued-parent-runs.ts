"use client";

import * as React from "react";

import type { PendingExchangeMap } from "@/features/chat/types/chat-runtime";
import { listConversationRuns } from "@/shared/api/conversation";
import { resolveAccessToken } from "@/shared/auth/resolve-access-token";

const POLL_INTERVAL_MS = 1_500;

type QueuedParentRun = {
  conversationScopeKey: string;
  conversationPublicID: string | null;
  parentRunID: string | null;
};

function isTerminalRunStatus(status: string | null | undefined): boolean {
  const normalized = status?.trim().toLowerCase() || "";
  return ["success", "interrupted", "error", "canceled", "cancelled"].includes(normalized);
}

export function useHiddenQueuedParentRuns({
  currentConversationScopeKey,
  queuedParents,
  getPendingExchanges,
  isRunActive,
}: {
  currentConversationScopeKey: string;
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
    const runsByConversation = new Map<string, Set<string>>();
    for (const parent of queuedParents) {
      const parentRunID = parent.parentRunID?.trim() || "";
      const conversationPublicID = parent.conversationPublicID?.trim() || "";
      if (!parentRunID || !conversationPublicID) {
        continue;
      }
      watchedRunIDs.add(parentRunID);
      if (
        parent.conversationScopeKey === currentConversationScopeKey ||
        isRunActive(parentRunID) ||
        locallyTrackedRunIDs.has(parentRunID) ||
        isTerminalRunStatus(statusesRef.current.get(parentRunID))
      ) {
        continue;
      }
      const runIDs = runsByConversation.get(conversationPublicID) ?? new Set<string>();
      runIDs.add(parentRunID);
      runsByConversation.set(conversationPublicID, runIDs);
    }
    for (const runID of statusesRef.current.keys()) {
      if (!watchedRunIDs.has(runID)) {
        statusesRef.current.delete(runID);
      }
    }
    if (runsByConversation.size === 0) {
      return;
    }

    let cancelled = false;
    let pollTimer: number | null = null;
    const poll = async () => {
      const token = await resolveAccessToken();
      if (!token || cancelled) {
        return;
      }
      let changed = false;
      let shouldPollAgain = false;
      await Promise.all(
        Array.from(runsByConversation.entries()).map(async ([conversationPublicID, runIDs]) => {
          try {
            const page = await listConversationRuns(token, conversationPublicID, {
              page: 1,
              pageSize: 20,
            });
            if (cancelled) {
              return;
            }
            const statuses = new Map(
              page.results.map((run) => [run.runID.trim(), run.status.trim().toLowerCase()]),
            );
            for (const runID of runIDs) {
              const status = statuses.get(runID) || "";
              if (!status || !isTerminalRunStatus(status)) {
                shouldPollAgain = true;
              }
              if (status && statusesRef.current.get(runID) !== status) {
                statusesRef.current.set(runID, status);
                changed = true;
              }
            }
          } catch {
            shouldPollAgain = true;
          }
        }),
      );
      if (cancelled) {
        return;
      }
      if (changed) {
        setRevision((current) => current + 1);
      }
      if (shouldPollAgain) {
        pollTimer = window.setTimeout(poll, POLL_INTERVAL_MS);
      }
    };

    void poll();
    return () => {
      cancelled = true;
      if (pollTimer !== null) {
        window.clearTimeout(pollTimer);
      }
    };
  }, [currentConversationScopeKey, getPendingExchanges, isRunActive, queuedParents]);

  const getStatus = React.useCallback((runID: string) => statusesRef.current.get(runID.trim()) || "", []);

  return {
    getStatus,
    revision,
  };
}
