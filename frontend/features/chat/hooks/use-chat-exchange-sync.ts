"use client";

import * as React from "react";
import { replaceCompletedBranchSelection } from "@/features/chat/model/message-submit-branching";
import { collectSettledExchanges } from "@/features/chat/model/message-submit-exchange";
import type { PendingExchangeMap } from "@/features/chat/types/chat-runtime";
import type { ChatAreaMessage } from "@/features/chat/types/messages";

// 本地乐观 exchange 与服务端消息对账:清理已落库或已在其他会话完成的记录,并把临时分支选择替换为服务端 ID。
export function useChatExchangeSync({
  conversationScopeKey,
  pendingExchanges,
  setPendingExchanges,
  serverMessagePublicIDs,
  combinedMessages,
  setBranchSelections,
}: {
  conversationScopeKey: string;
  pendingExchanges: PendingExchangeMap;
  setPendingExchanges: React.Dispatch<React.SetStateAction<PendingExchangeMap>>;
  serverMessagePublicIDs: Set<string>;
  combinedMessages: ChatAreaMessage[];
  setBranchSelections: React.Dispatch<React.SetStateAction<Record<string, string>>>;
}) {
  React.useEffect(() => {
    setPendingExchanges((current) => {
      const completedBackgroundKeys = Object.entries(current)
        .filter(
          ([, exchange]) =>
            exchange.conversationScopeKey !== conversationScopeKey &&
            Boolean(exchange.assistantPublicID) &&
            !exchange.assistantPending &&
            !exchange.assistantStreaming,
        )
        .map(([exchangeKey]) => exchangeKey);
      if (completedBackgroundKeys.length === 0) {
        return current;
      }
      const next = { ...current };
      for (const exchangeKey of completedBackgroundKeys) {
        delete next[exchangeKey];
      }
      return next;
    });
  }, [conversationScopeKey, setPendingExchanges]);

  React.useEffect(() => {
    const { completedKeys, completedBranches } = collectSettledExchanges(
      pendingExchanges,
      serverMessagePublicIDs,
      combinedMessages,
    );
    if (completedBranches.length > 0) {
      setBranchSelections((current) =>
        completedBranches.reduce(
          (next, completed) =>
            replaceCompletedBranchSelection(
              next,
              {
                parentPublicID: completed.exchange.parentPublicID,
                tempUserPublicID: completed.exchange.tempUserPublicID,
                tempAssistantPublicID: completed.exchange.tempAssistantPublicID,
                reuseUserMessage: completed.exchange.reuseUserMessage,
              },
              completed.userPublicID,
              completed.assistantPublicID,
            ),
          current,
        ),
      );
    }
    if (completedKeys.length > 0) {
      setPendingExchanges((current) => {
        const next = { ...current };
        for (const key of completedKeys) {
          delete next[key];
        }
        return next;
      });
    }
  }, [
    combinedMessages,
    pendingExchanges,
    serverMessagePublicIDs,
    setBranchSelections,
    setPendingExchanges,
  ]);
}
