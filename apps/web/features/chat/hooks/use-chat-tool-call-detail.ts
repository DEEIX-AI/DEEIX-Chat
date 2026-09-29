"use client";

import * as React from "react";

import { getConversationToolCallDetail } from "@/shared/api/conversation";
import type { ConversationToolCallDetailDTO } from "@/shared/api/conversation-types";
import { resolveAccessToken } from "@/shared/auth/resolve-access-token";

/** Lazily loads the full tool-call result the first time its dialog opens. */
export function useChatToolCallDetail({
  open,
  runID,
  toolCallID,
}: {
  open: boolean;
  runID: string;
  toolCallID: string;
}) {
  const [detail, setDetail] = React.useState<ConversationToolCallDetailDTO | null>(null);
  const [loading, setLoading] = React.useState(false);
  const [loadFailed, setLoadFailed] = React.useState(false);

  React.useEffect(() => {
    setDetail(null);
    setLoadFailed(false);
  }, [runID, toolCallID]);

  React.useEffect(() => {
    if (!open || detail) {
      return;
    }

    const controller = new AbortController();
    setLoading(true);
    setLoadFailed(false);
    void (async () => {
      try {
        const accessToken = await resolveAccessToken();
        if (!accessToken) {
          throw new Error("missing access token");
        }
        const result = await getConversationToolCallDetail(
          accessToken,
          runID,
          toolCallID,
          controller.signal,
        );
        if (!controller.signal.aborted) {
          setDetail(result);
        }
      } catch {
        if (!controller.signal.aborted) {
          setLoadFailed(true);
        }
      } finally {
        if (!controller.signal.aborted) {
          setLoading(false);
        }
      }
    })();

    return () => controller.abort();
  }, [detail, open, runID, toolCallID]);

  // Called once the dialog finished closing, so the next open re-fetches a fresh result.
  const reset = React.useCallback(() => {
    setDetail(null);
    setLoading(false);
    setLoadFailed(false);
  }, []);

  return { detail, loading, loadFailed, reset };
}
