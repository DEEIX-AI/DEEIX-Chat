"use client";

import * as React from "react";
import {
  type ActiveStream,
  branchRunIsVisible,
  clearCancelSettlementTimer,
  findLastVisibleActiveStream,
  findVisibleActiveStreamByRunID,
} from "@/features/chat/model/message-submit-branching";
import type { ChatAreaMessage } from "@/features/chat/types/messages";
import { cancelMessageGeneration } from "@/shared/api/conversation";
import { resolveAccessToken } from "@/shared/auth/resolve-access-token";

const GENERATION_CANCEL_SETTLEMENT_TIMEOUT_MS = 25_000;

// 停止当前可见分支的生成:优先取消可见活跃流,并在取消请求未及时结算时兜底中断连接。
export function useChatStopMessage({
  activeStreamsRef,
  currentLeafMessage,
  conversationScopeKeyRef,
  visibleBranchScopePathRef,
  visibleMessagesRef,
  reload,
}: {
  activeStreamsRef: React.RefObject<Map<string, ActiveStream>>;
  currentLeafMessage: ChatAreaMessage | null;
  conversationScopeKeyRef: React.RefObject<string>;
  visibleBranchScopePathRef: React.RefObject<string[]>;
  visibleMessagesRef: React.RefObject<ChatAreaMessage[]>;
  reload: () => void;
}) {
  return React.useCallback(() => {
    const visibleRunID = currentLeafMessage?.runID?.trim() || "";
    const visibleRunPending = Boolean(
      visibleRunID &&
        (currentLeafMessage?.isPending ||
          currentLeafMessage?.isStreaming ||
          currentLeafMessage?.status?.trim().toLowerCase() === "pending"),
    );
    const visibleActive = findVisibleActiveStreamByRunID(
      activeStreamsRef.current,
      visibleRunID,
      conversationScopeKeyRef.current,
      visibleBranchScopePathRef.current,
      visibleMessagesRef.current,
    );
    if (!visibleActive && visibleRunPending) {
      void resolveAccessToken().then(async (token) => {
        if (!token) {
          return;
        }
        await cancelMessageGeneration(token, visibleRunID).catch((): undefined => undefined);
        reload();
      });
      return true;
    }
    const active =
      visibleActive ??
      findLastVisibleActiveStream(
        activeStreamsRef.current,
        conversationScopeKeyRef.current,
        visibleBranchScopePathRef.current,
        visibleMessagesRef.current,
      );
    if (!active) {
      return false;
    }
    if (active.cancelRequested) {
      return true;
    }
    if (!active.accessToken) {
      active.controller.abort();
      return true;
    }

    active.cancelRequested = true;
    active.cancelSettlementTimer = window.setTimeout(() => {
      if (activeStreamsRef.current.get(active.runID) !== active) {
        return;
      }
      active.controller.abort();
      if (
        branchRunIsVisible(
          active,
          active.runID,
          conversationScopeKeyRef.current,
          visibleBranchScopePathRef.current,
          visibleMessagesRef.current,
        )
      ) {
        reload();
      }
    }, GENERATION_CANCEL_SETTLEMENT_TIMEOUT_MS);

    // Keep the stream connected so its terminal payload can replace optimistic IDs
    // and retain the final partial content/usage produced during cancellation.
    void cancelMessageGeneration(active.accessToken, active.runID).catch(() => {
      if (activeStreamsRef.current.get(active.runID) !== active) {
        return;
      }
      clearCancelSettlementTimer(active);
      active.controller.abort();
      if (
        branchRunIsVisible(
          active,
          active.runID,
          conversationScopeKeyRef.current,
          visibleBranchScopePathRef.current,
          visibleMessagesRef.current,
        )
      ) {
        reload();
      }
    });
    return true;
  }, [
    activeStreamsRef,
    conversationScopeKeyRef,
    currentLeafMessage?.isPending,
    currentLeafMessage?.isStreaming,
    currentLeafMessage?.runID,
    currentLeafMessage?.status,
    reload,
    visibleBranchScopePathRef,
    visibleMessagesRef,
  ]);
}
