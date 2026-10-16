"use client";

import * as React from "react";

import { mapProcessTrace } from "@/features/chat/model/chat-thread";
import {
  readMessageTraceDetail,
  subscribeMessageTraceDetails,
  writeMessageTraceDetail,
} from "@/features/chat/model/message-trace-detail-store";
import type { ChatMessageProcessTrace } from "@/features/chat/types/messages";
import { getMessageProcessTrace } from "@/shared/api/conversation";
import { resolveAccessToken } from "@/shared/auth/resolve-access-token";

/**
 * The message list sends finished runs as a summary (thinking cut short, no tool inputs or
 * outputs). The full trace loads once per message, when its run is expanded.
 */
export function useChatMessageTraceDetail(messagePublicID: string | undefined, trace: ChatMessageProcessTrace | undefined) {
  const key = messagePublicID?.trim() ?? "";
  const omitted = Boolean(key && trace?.detailOmitted);
  const detail = React.useSyncExternalStore(
    subscribeMessageTraceDetails,
    () => (omitted ? readMessageTraceDetail(key) : undefined),
    (): undefined => undefined,
  );

  const loadDetail = React.useCallback(() => {
    const status = readMessageTraceDetail(key)?.status;
    if (!omitted || status === "loading" || status === "loaded") {
      return;
    }
    writeMessageTraceDetail(key, { status: "loading" });
    void (async () => {
      try {
        const token = await resolveAccessToken();
        if (!token) {
          writeMessageTraceDetail(key, { status: "failed" });
          return;
        }
        const full = await getMessageProcessTrace(token, key);
        writeMessageTraceDetail(key, { status: "loaded", trace: mapProcessTrace(full) });
      } catch {
        writeMessageTraceDetail(key, { status: "failed" });
      }
    })();
  }, [key, omitted]);

  return {
    trace: detail?.status === "loaded" ? (detail.trace ?? trace) : trace,
    detailPending: omitted && detail?.status !== "loaded",
    detailFailed: detail?.status === "failed",
    loadDetail,
  };
}
