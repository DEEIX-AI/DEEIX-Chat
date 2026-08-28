"use client";

import { useTranslations } from "next-intl";
import * as React from "react";
import { toast } from "sonner";
import type { SubmitChatMessageInput } from "@/features/chat/hooks/use-chat-message-actions";
import { resolvePersistedPublicID } from "@/features/chat/model/message-submit";
import {
  type ActiveStream,
  type BranchScope,
  branchScopeID,
  branchScopeIsVisible,
  branchScopesEqual,
  findSuccessfulBranchParentMessage,
  isFailedBranchParentStatus,
  isSuccessfulBranchParentStatus,
  MAX_CONCURRENT_RUNS,
  type QueuedChatSubmission,
} from "@/features/chat/model/message-submit-branching";
import type { PendingAttachment, PendingExchangeMap } from "@/features/chat/types/chat-runtime";
import type { ChatAreaMessage } from "@/features/chat/types/messages";

/**
 * 排队消息的派发调度：父 run 失败时级联清理其后代排队消息并提示；
 * 当父 run 已成功落库且未超并发上限时，把符合条件的队首消息交给 submitMessage 发送。
 */
export function useChatQueueDispatch({
  queuedSubmissions,
  setQueuedSubmissions,
  sendQueuedAfterCurrentRef,
  dispatchingQueuedSubmissionIDsRef,
  settledQueuedSubmissionIDsRef,
  activeStreamsRef,
  getPendingExchanges,
  pendingExchanges,
  combinedMessages,
  visibleMessages,
  visibleBranchScopePath,
  conversationScopeKey,
  currentLeafMessage,
  getHiddenParentRunStatus,
  hiddenParentRunStatusRevision,
  resumeGenerationActive,
  activeGenerationRunsRevision,
  releaseAttachments,
  submitMessage,
}: {
  queuedSubmissions: QueuedChatSubmission[];
  setQueuedSubmissions: React.Dispatch<React.SetStateAction<QueuedChatSubmission[]>>;
  sendQueuedAfterCurrentRef: React.RefObject<Set<string>>;
  dispatchingQueuedSubmissionIDsRef: React.RefObject<Set<string>>;
  settledQueuedSubmissionIDsRef: React.RefObject<Set<string>>;
  activeStreamsRef: React.RefObject<Map<string, ActiveStream>>;
  getPendingExchanges: () => PendingExchangeMap;
  pendingExchanges: PendingExchangeMap;
  combinedMessages: ChatAreaMessage[];
  visibleMessages: ChatAreaMessage[];
  visibleBranchScopePath: string[];
  conversationScopeKey: string;
  currentLeafMessage: ChatAreaMessage | null;
  getHiddenParentRunStatus: (runID: string) => string;
  hiddenParentRunStatusRevision: number;
  resumeGenerationActive: boolean;
  activeGenerationRunsRevision: number;
  releaseAttachments: (items: PendingAttachment[]) => void;
  submitMessage: (input: SubmitChatMessageInput) => Promise<boolean>;
}) {
  const t = useTranslations("chat.submit");

  React.useEffect(() => {
    if (queuedSubmissions.length === 0) {
      return;
    }
    const pending = Object.values(getPendingExchanges());
    const failedRunIDs = new Set<string>();
    const failedSubmissionIDs = new Set<string>();
    let changed = true;
    while (changed) {
      changed = false;
      for (const submission of queuedSubmissions) {
        if (failedSubmissionIDs.has(submission.id) || !submission.parentRunID) {
          continue;
        }
        const parentStatus =
          pending.find((exchange) => exchange.runID === submission.parentRunID)?.assistantStatus ??
          combinedMessages.find(
            (message) => message.role === "assistant" && message.runID === submission.parentRunID,
          )?.status ??
          getHiddenParentRunStatus(submission.parentRunID);
        if (!failedRunIDs.has(submission.parentRunID) && !isFailedBranchParentStatus(parentStatus)) {
          continue;
        }
        failedSubmissionIDs.add(submission.id);
        failedRunIDs.add(submission.clientRunID);
        changed = true;
      }
    }
    const newlySettled = queuedSubmissions.filter(
      (submission) =>
        failedSubmissionIDs.has(submission.id) &&
        !settledQueuedSubmissionIDsRef.current.has(submission.id),
    );
    if (newlySettled.length === 0) {
      return;
    }
    for (const submission of newlySettled) {
      settledQueuedSubmissionIDsRef.current.add(submission.id);
      dispatchingQueuedSubmissionIDsRef.current.delete(submission.id);
      sendQueuedAfterCurrentRef.current.delete(branchScopeID(submission));
      releaseAttachments(submission.attachments);
    }
    setQueuedSubmissions((current) =>
      current.filter((submission) => !failedSubmissionIDs.has(submission.id)),
    );
    toast.error(t("queuedParentFailed"), {
      description: t("queuedParentFailedDescription"),
    });
  }, [
    getHiddenParentRunStatus,
    getPendingExchanges,
    hiddenParentRunStatusRevision,
    combinedMessages,
    queuedSubmissions,
    releaseAttachments,
    t,
    sendQueuedAfterCurrentRef,
    settledQueuedSubmissionIDsRef,
    dispatchingQueuedSubmissionIDsRef,
    setQueuedSubmissions,
  ]);

  React.useEffect(() => {
    const currentBranchHasPendingServerGeneration = visibleMessages.some(
      (message) =>
        message.role === "assistant" &&
        (message.isPending ||
          message.isStreaming ||
          message.status?.trim().toLowerCase() === "pending"),
    );
    if (queuedSubmissions.length === 0) {
      return;
    }
    if (activeStreamsRef.current.size >= MAX_CONCURRENT_RUNS) {
      return;
    }
    const allPendingExchanges = getPendingExchanges();
    const queuedSubmission = queuedSubmissions.find((item) => {
      if (dispatchingQueuedSubmissionIDsRef.current.has(item.id)) {
        return false;
      }
      const hasActiveStream = Array.from(activeStreamsRef.current.values()).some((active) =>
        branchScopesEqual(active, item),
      );
      if (hasActiveStream) {
        return false;
      }
      const isCurrentBranch = branchScopeIsVisible(item, conversationScopeKey, visibleMessages);
      if (isCurrentBranch && (resumeGenerationActive || currentBranchHasPendingServerGeneration)) {
        return false;
      }
      const hasUnresolvedDefaultExchange = Object.values(allPendingExchanges).some(
        (exchange) =>
          branchScopesEqual(exchange, item) &&
          exchange.branchReason === "default" &&
          !exchange.assistantPublicID,
      );
      if (
        hasUnresolvedDefaultExchange &&
        !sendQueuedAfterCurrentRef.current.has(branchScopeID(item))
      ) {
        return false;
      }
      if (!item.parentRunID) {
        return true;
      }
      const parentExchange = Object.values(allPendingExchanges).find(
        (exchange) => exchange.runID === item.parentRunID && branchScopesEqual(exchange, item),
      );
      const parentStatus =
        parentExchange?.assistantStatus ??
        combinedMessages.find(
          (message) => message.role === "assistant" && message.runID === item.parentRunID,
        )?.status ??
        getHiddenParentRunStatus(item.parentRunID);
      if (isFailedBranchParentStatus(parentStatus)) {
        return false;
      }
      if (resolvePersistedPublicID(parentExchange?.assistantPublicID)) {
        return true;
      }
      const serverParentMessage = findSuccessfulBranchParentMessage(
        combinedMessages,
        item.parentRunID,
      );
      if (serverParentMessage) {
        return true;
      }
      if (isSuccessfulBranchParentStatus(getHiddenParentRunStatus(item.parentRunID))) {
        return true;
      }
      return Boolean(
        isCurrentBranch &&
          currentLeafMessage?.runID === item.parentRunID &&
          resolvePersistedPublicID(currentLeafMessage.publicID),
      );
    });
    if (!queuedSubmission) {
      return;
    }
    const dispatchedBranchScope: BranchScope = {
      conversationScopeKey: queuedSubmission.conversationScopeKey,
      branchScopePath: queuedSubmission.branchScopePath,
      branchScopeRunID: queuedSubmission.clientRunID,
    };
    const dispatchedSubmission: QueuedChatSubmission = {
      ...queuedSubmission,
      ...dispatchedBranchScope,
    };
    dispatchingQueuedSubmissionIDsRef.current.add(queuedSubmission.id);
    sendQueuedAfterCurrentRef.current.delete(branchScopeID(queuedSubmission));
    setQueuedSubmissions((current) =>
      current
        .filter((item) => item.id !== queuedSubmission.id)
        .map((item) =>
          branchScopesEqual(item, queuedSubmission)
            ? {
                ...item,
                ...dispatchedBranchScope,
              }
            : item,
        ),
    );
    const parentExchange = queuedSubmission.parentRunID
      ? Object.values(allPendingExchanges).find(
          (exchange) =>
            exchange.runID === queuedSubmission.parentRunID &&
            branchScopesEqual(exchange, queuedSubmission),
        )
      : undefined;
    const serverParentMessage = findSuccessfulBranchParentMessage(
      combinedMessages,
      queuedSubmission.parentRunID,
    );
    const parentMessagePublicID =
      resolvePersistedPublicID(parentExchange?.assistantPublicID) ??
      resolvePersistedPublicID(serverParentMessage?.publicID) ??
      (branchScopeIsVisible(queuedSubmission, conversationScopeKey, visibleMessages) &&
      currentLeafMessage?.runID === queuedSubmission.parentRunID
        ? resolvePersistedPublicID(currentLeafMessage.publicID)
        : null) ??
      queuedSubmission.parentMessagePublicID;
    void submitMessage({
      content: queuedSubmission.content,
      currentAttachments: queuedSubmission.attachments,
      resetComposer: false,
      parentMessagePublicID,
      branchReason: "default",
      queuedSubmission: dispatchedSubmission,
    }).finally(() => {
      dispatchingQueuedSubmissionIDsRef.current.delete(queuedSubmission.id);
    });
  }, [
    activeGenerationRunsRevision,
    combinedMessages,
    conversationScopeKey,
    currentLeafMessage?.publicID,
    currentLeafMessage?.runID,
    getPendingExchanges,
    getHiddenParentRunStatus,
    hiddenParentRunStatusRevision,
    pendingExchanges,
    queuedSubmissions,
    resumeGenerationActive,
    submitMessage,
    visibleBranchScopePath,
    visibleMessages,
    activeStreamsRef,
    dispatchingQueuedSubmissionIDsRef,
    sendQueuedAfterCurrentRef,
    setQueuedSubmissions,
  ]);
}
