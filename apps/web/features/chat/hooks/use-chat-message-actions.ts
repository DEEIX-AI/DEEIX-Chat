"use client";

import { useTranslations } from "next-intl";
import * as React from "react";
import { toast } from "sonner";
import { buildChildrenIndex, toBranchKey } from "@/features/chat/model/chat-thread";
import {
  resolvePersistedPublicID,
  toPendingAttachments,
} from "@/features/chat/model/message-submit";
import type { QueuedChatSubmission } from "@/features/chat/model/message-submit-branching";
import { withChatReasoningEffortOverride } from "@/features/chat/model/chat-reasoning-effort";
import type { PendingAttachment } from "@/features/chat/types/chat-runtime";
import type { ChatAreaMessage, UserMessageEditMode } from "@/features/chat/types/messages";
import { resolveErrorMessage } from "@/features/chat/utils/chat-runtime";
import { deleteConversationMessage, forkConversationFromMessage, updateMessage } from "@/shared/api/conversation";
import type { ConversationDTO, ConversationOptions, MessageDTO } from "@/shared/api/conversation-types";
import { resolveAccessToken } from "@/shared/auth/resolve-access-token";
import type { ReasoningEffortLevel } from "@/entities/model";

export type SubmitChatMessageInput = {
  content: string;
  currentAttachments: PendingAttachment[];
  resetComposer: boolean;
  parentMessagePublicID?: string | null;
  sourceMessagePublicID?: string | null;
  branchReason?: "default" | "retry" | "edit";
  queuedSubmission?: QueuedChatSubmission;
  optionsOverride?: ConversationOptions;
};

function buildContinueGenerationPrompt(t: ReturnType<typeof useTranslations>): string {
  return t("continueGenerationPrompt");
}

/**
 * Message-level actions: retry user/assistant messages, continue interrupted generations, edit user/assistant messages,
 * fork a new conversation from a message, and switch between sibling branches.
 */
export function useChatMessageActions({
  submitMessage,
  combinedMessages,
  replaceMessage,
  onConversationForked,
  conversationIDRef,
  setBranchSelections,
  reload,
}: {
  submitMessage: (input: SubmitChatMessageInput) => Promise<boolean>;
  combinedMessages: ChatAreaMessage[];
  replaceMessage: (message: MessageDTO) => void;
  onConversationForked?: (conversation: ConversationDTO) => Promise<void> | void;
  conversationIDRef: React.RefObject<string | null>;
  setBranchSelections: React.Dispatch<React.SetStateAction<Record<string, string>>>;
  reload: () => void;
}) {
  const t = useTranslations("chat.submit");

  const onRetryUserMessage = React.useCallback(
    async (message: ChatAreaMessage) => {
      const sourceMessagePublicID = resolvePersistedPublicID(message.publicID);
      if (!sourceMessagePublicID) {
        toast.error(t("retryReplyFailed"), { description: t("continueReplyUnavailable") });
        return;
      }
      await submitMessage({
        content: message.content.trim(),
        currentAttachments: toPendingAttachments(message),
        resetComposer: false,
        parentMessagePublicID: message.parentPublicID,
        sourceMessagePublicID,
        branchReason: "retry",
      });
    },
    [submitMessage, t],
  );

  const onRetryAssistantMessage = React.useCallback(
    // reasoningEffort overrides the level for this single regeneration; the composer selection is unchanged.
    async (message: ChatAreaMessage, reasoningEffort?: ReasoningEffortLevel) => {
      const parentUser = combinedMessages.find(
        (item) => item.publicID === message.parentPublicID && item.role === "user",
      );
      if (!parentUser) {
        toast.error(t("retryReplyFailed"), { description: t("retryReplyMissingUser") });
        return;
      }
      const parentUserPublicID = resolvePersistedPublicID(parentUser.publicID);
      const assistantSourceMessagePublicID = resolvePersistedPublicID(message.publicID);
      if (!parentUserPublicID || !assistantSourceMessagePublicID) {
        toast.error(t("retryReplyFailed"), { description: t("continueReplyUnavailable") });
        return;
      }
      await submitMessage({
        content: parentUser.content.trim(),
        currentAttachments: toPendingAttachments(parentUser),
        resetComposer: false,
        parentMessagePublicID: parentUserPublicID,
        sourceMessagePublicID: assistantSourceMessagePublicID,
        branchReason: "retry",
        optionsOverride: reasoningEffort ? withChatReasoningEffortOverride({}, reasoningEffort) : undefined,
      });
    },
    [combinedMessages, submitMessage, t],
  );

  const onContinueAssistantMessage = React.useCallback(
    async (message: ChatAreaMessage) => {
      const parentPublicID = resolvePersistedPublicID(message.publicID);
      const status = message.status?.trim().toLowerCase();
      if (!parentPublicID || message.role !== "assistant" || status !== "interrupted") {
        toast.error(t("continueReplyFailed"), { description: t("continueReplyUnavailable") });
        return;
      }
      await submitMessage({
        content: buildContinueGenerationPrompt(t),
        currentAttachments: [],
        resetComposer: false,
        parentMessagePublicID: parentPublicID,
        branchReason: "default",
      });
    },
    [submitMessage, t],
  );

  const saveMessageInPlace = React.useCallback(
    async (message: ChatAreaMessage, content: string, failureKey: "editReplyFailed" | "editMessageFailed") => {
      const messagePublicID = resolvePersistedPublicID(message.publicID);
      const nextContent = content.trim();
      if (!messagePublicID || !nextContent) {
        toast.error(t(failureKey), { description: t("continueReplyUnavailable") });
        return false;
      }
      const token = await resolveAccessToken();
      if (!token) {
        toast.error(t(failureKey), { description: t("signInRequired") });
        return false;
      }
      try {
        const updated = await updateMessage(token, messagePublicID, { content: nextContent });
        replaceMessage(updated);
        return true;
      } catch {
        toast.error(t(failureKey), { description: t("retryLater") });
        return false;
      }
    },
    [replaceMessage, t],
  );

  const onEditUserMessage = React.useCallback(
    async (message: ChatAreaMessage, content: string, mode: UserMessageEditMode) => {
      if (mode === "save") {
        return saveMessageInPlace(message, content, "editMessageFailed");
      }
      const sourceMessagePublicID = resolvePersistedPublicID(message.publicID);
      if (!sourceMessagePublicID) {
        toast.error(t("retryReplyFailed"), { description: t("continueReplyUnavailable") });
        return false;
      }
      const ok = await submitMessage({
        content: content.trim(),
        currentAttachments: toPendingAttachments(message),
        resetComposer: false,
        parentMessagePublicID: message.parentPublicID,
        sourceMessagePublicID,
        branchReason: "edit",
      });
      return ok;
    },
    [saveMessageInPlace, submitMessage, t],
  );

  const onEditAssistantMessage = React.useCallback(
    (message: ChatAreaMessage, content: string) => saveMessageInPlace(message, content, "editReplyFailed"),
    [saveMessageInPlace],
  );

  const onForkMessage = React.useCallback(
    async (message: ChatAreaMessage) => {
      const messagePublicID = resolvePersistedPublicID(message.publicID);
      const conversationPublicID = conversationIDRef.current?.trim() || "";
      if (!messagePublicID || !conversationPublicID) {
        toast.error(t("forkFailed"), { description: t("continueReplyUnavailable") });
        return;
      }
      const token = await resolveAccessToken();
      if (!token) {
        toast.error(t("forkFailed"), { description: t("signInRequired") });
        return;
      }
      try {
        const forked = await forkConversationFromMessage(token, conversationPublicID, messagePublicID);
        await onConversationForked?.(forked);
      } catch (error) {
        toast.error(t("forkFailed"), {
          description: resolveErrorMessage(error, t("retryLater")),
        });
      }
    },
    [conversationIDRef, onConversationForked, t],
  );

  const onDeleteMessage = React.useCallback(
    async (message: ChatAreaMessage) => {
      const messagePublicID = resolvePersistedPublicID(message.publicID);
      const conversationPublicID = conversationIDRef.current?.trim() || "";
      if (!messagePublicID || !conversationPublicID) {
        toast.error(t("deleteMessageFailed"), { description: t("continueReplyUnavailable") });
        return;
      }
      const token = await resolveAccessToken();
      if (!token) {
        toast.error(t("deleteMessageFailed"), { description: t("signInRequired") });
        return;
      }
      try {
        await deleteConversationMessage(token, conversationPublicID, messagePublicID);
        // splice reconnects subsequent messages and branch selection converges on its own, so a full refresh suffices.
        reload();
      } catch (error) {
        toast.error(t("deleteMessageFailed"), {
          description: resolveErrorMessage(error, t("retryLater")),
        });
      }
    },
    [conversationIDRef, reload, t],
  );

  const onCycleMessageBranch = React.useCallback(
    (parentPublicID: string | null, direction: "previous" | "next") => {
      const siblings = buildChildrenIndex(combinedMessages).get(toBranchKey(parentPublicID)) ?? [];
      if (siblings.length <= 1) {
        return;
      }
      setBranchSelections((prev) => {
        const parentKey = toBranchKey(parentPublicID);
        const selectedPublicID = prev[parentKey] || siblings[siblings.length - 1]?.publicID;
        const currentIndex = siblings.findIndex((item) => item.publicID === selectedPublicID);
        if (currentIndex < 0) {
          return prev;
        }
        const nextIndex = direction === "previous" ? currentIndex - 1 : currentIndex + 1;
        if (nextIndex < 0 || nextIndex >= siblings.length) {
          return prev;
        }
        return {
          ...prev,
          [parentKey]: siblings[nextIndex].publicID,
        };
      });
    },
    [combinedMessages, setBranchSelections],
  );

  return {
    onRetryUserMessage,
    onRetryAssistantMessage,
    onContinueAssistantMessage,
    onEditUserMessage,
    onEditAssistantMessage,
    onForkMessage,
    onDeleteMessage,
    onCycleMessageBranch,
  };
}
