"use client";

import * as React from "react";
import {
  branchScopeID,
  branchScopesEqual,
  type QueuedChatSubmission,
  rechainQueuedSubmissions,
} from "@/features/chat/model/message-submit-branching";
import type { PendingAttachment } from "@/features/chat/types/chat-runtime";

/**
 * State store and user actions for queued messages: delete (relinking parents of subsequent messages), edit content,
 * mark to send first after the current generation; also maintains dedupe sets such as dispatching/settled.
 */
export function useChatSubmissionQueue({
  releaseAttachments,
}: {
  releaseAttachments: (items: PendingAttachment[]) => void;
}) {
  const sendQueuedAfterCurrentRef = React.useRef(new Set<string>());
  const dispatchingQueuedSubmissionIDsRef = React.useRef(new Set<string>());
  const settledQueuedSubmissionIDsRef = React.useRef(new Set<string>());
  const [queuedSubmissions, setQueuedSubmissions] = React.useState<QueuedChatSubmission[]>([]);
  const queuedSubmissionsRef = React.useRef<QueuedChatSubmission[]>([]);

  React.useEffect(() => {
    const queuedIDs = new Set(queuedSubmissions.map((submission) => submission.id));
    for (const settledID of settledQueuedSubmissionIDsRef.current) {
      if (!queuedIDs.has(settledID)) settledQueuedSubmissionIDsRef.current.delete(settledID);
    }
  }, [queuedSubmissions]);

  React.useEffect(() => {
    queuedSubmissionsRef.current = queuedSubmissions;
  }, [queuedSubmissions]);

  const onDeleteQueuedMessage = React.useCallback(
    (id: string) => {
      const target = queuedSubmissionsRef.current.find((item) => item.id === id);
      if (target) {
        releaseAttachments(target.attachments);
      }
      setQueuedSubmissions((current) => {
        const currentTarget = current.find((item) => item.id === id);
        if (!currentTarget) {
          return current;
        }
        const firstScopeSubmission = current.find((item) => branchScopesEqual(item, currentTarget));
        return rechainQueuedSubmissions(
          current.filter((item) => item.id !== id),
          currentTarget,
          firstScopeSubmission?.parentRunID ?? null,
          firstScopeSubmission?.parentMessagePublicID ?? null,
        );
      });
    },
    [releaseAttachments],
  );

  const onEditQueuedMessage = React.useCallback((id: string, content: string) => {
    setQueuedSubmissions((current) =>
      current.map((item) => (item.id === id ? { ...item, content: content.trim() } : item)),
    );
  }, []);

  const onGuideQueuedMessage = React.useCallback((id: string) => {
    setQueuedSubmissions((current) => {
      const target = current.find((item) => item.id === id);
      if (!target) {
        return current;
      }
      sendQueuedAfterCurrentRef.current.add(branchScopeID(target));
      const firstScopeIndex = current.findIndex((item) => branchScopesEqual(item, target));
      const firstScopeSubmission = firstScopeIndex >= 0 ? current[firstScopeIndex] : undefined;
      const reordered = current.filter((item) => item.id !== id);
      reordered.splice(Math.max(firstScopeIndex, 0), 0, target);
      return rechainQueuedSubmissions(
        reordered,
        target,
        firstScopeSubmission?.parentRunID ?? null,
        firstScopeSubmission?.parentMessagePublicID ?? null,
      );
    });
  }, []);

  return {
    queuedSubmissions,
    setQueuedSubmissions,
    queuedSubmissionsRef,
    sendQueuedAfterCurrentRef,
    dispatchingQueuedSubmissionIDsRef,
    settledQueuedSubmissionIDsRef,
    onDeleteQueuedMessage,
    onEditQueuedMessage,
    onGuideQueuedMessage,
  };
}
