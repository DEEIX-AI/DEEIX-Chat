"use client";

import * as React from "react";
import { useTranslations } from "next-intl";

import type { ChatAreaMessage, MessageAttachment } from "@/features/chat/types/messages";
import type { PendingExchange } from "@/features/chat/types/chat-runtime";
import {
  buildVisibleMessages,
  mapServerMessage,
  reconcileBranchSelections,
} from "@/features/chat/model/chat-thread";
import type { MessageDTO } from "@/shared/api/conversation.types";

function buildPendingMessages({
  conversationID,
  pendingExchange,
  serverTreeMessages,
  serverMessagePublicIDs,
}: {
  conversationID: string | null;
  pendingExchange: PendingExchange | null;
  serverTreeMessages: ChatAreaMessage[];
  serverMessagePublicIDs: Set<string>;
}) {
  const nextMessages = [...serverTreeMessages];
  const activePublicID = conversationID?.trim() || null;
  const pendingConversationPublicID = pendingExchange?.conversationPublicID?.trim() || null;
  if (!pendingExchange || (activePublicID && pendingConversationPublicID !== activePublicID)) {
    return nextMessages;
  }
  const pendingRunID = pendingExchange.runID?.trim() || "";
  if (
    pendingRunID &&
    serverTreeMessages.some((item) => item.role === "assistant" && item.runID === pendingRunID)
  ) {
    return mergePendingAssistantState(nextMessages, pendingExchange);
  }

  const userPublicID = pendingExchange.userPublicID || pendingExchange.tempUserPublicID;
  const assistantPublicID = pendingExchange.assistantPublicID || pendingExchange.tempAssistantPublicID;

  if (!serverMessagePublicIDs.has(userPublicID)) {
    const pendingAttachments = pendingExchange.userAttachments;
    const attachments: MessageAttachment[] | undefined =
      pendingAttachments && pendingAttachments.length > 0
        ? pendingAttachments.map((att) => ({
            fileID: att.fileID,
            fileName: att.fileName,
            mimeType: att.mimeType,
            sizeBytes: att.sizeBytes,
            kind: att.mimeType.startsWith("image/") ? ("image" as const) : ("file" as const),
            previewURL: att.previewURL,
          }))
        : undefined;
    nextMessages.push({
      key: `${pendingExchange.key}-user`,
      publicID: userPublicID,
      parentPublicID: pendingExchange.parentPublicID,
      sourcePublicID: pendingExchange.sourcePublicID,
      role: "user",
      content: pendingExchange.userContent,
      branchReason: pendingExchange.branchReason,
      status: pendingExchange.assistantPending ? "pending" : "success",
      runID: pendingExchange.runID,
      serverMessageID: pendingExchange.userServerMessageID,
      createdAt: pendingExchange.userCreatedAt,
      isPending: pendingExchange.assistantPending,
      attachments,
    });
  }

  if (
    pendingExchange.assistantPending ||
    pendingExchange.assistantText.length > 0 ||
    !serverMessagePublicIDs.has(assistantPublicID)
  ) {
    nextMessages.push({
      key: `${pendingExchange.key}-assistant`,
      publicID: assistantPublicID,
      parentPublicID: userPublicID,
      sourcePublicID: null,
      role: "assistant",
      contentType: pendingExchange.assistantContentType,
      content: pendingExchange.assistantText,
      branchReason: pendingExchange.branchReason,
      status: pendingExchange.assistantPending ? "pending" : pendingExchange.assistantStatus ?? "success",
      runID: pendingExchange.runID,
      platformModelName: pendingExchange.platformModelName,
      serverMessageID: pendingExchange.assistantServerMessageID,
      createdAt: pendingExchange.assistantCreatedAt,
      updatedAt: pendingExchange.assistantUpdatedAt,
      isPending: pendingExchange.assistantPending,
      isStreaming: pendingExchange.assistantStreaming,
      isFileProc: Boolean(pendingExchange.assistantFileProc && !pendingExchange.assistantText),
      activityLabel: pendingExchange.assistantActivityLabel,
      imageAspectRatio: pendingExchange.assistantImageAspectRatio,
      processTrace: pendingExchange.assistantProcessTrace,
      inlineAlert: pendingExchange.assistantInlineAlert,
      inputTokens: pendingExchange.assistantInputTokens,
      outputTokens: pendingExchange.assistantOutputTokens,
      cacheReadTokens: pendingExchange.assistantCacheReadTokens,
      cacheWriteTokens: pendingExchange.assistantCacheWriteTokens,
      reasoningTokens: pendingExchange.assistantReasoningTokens,
      latencyMS: pendingExchange.assistantLatencyMS,
      compactDone: pendingExchange.compactDone,
    });
  }

  return nextMessages;
}

function mergePendingAssistantState(messages: ChatAreaMessage[], pendingExchange: PendingExchange) {
  const pendingAlert = pendingExchange.assistantInlineAlert;
  const pendingRunID = pendingExchange.runID?.trim() || "";
  const pendingAssistantID = pendingExchange.assistantPublicID || pendingExchange.tempAssistantPublicID;
  const pendingText = pendingExchange.assistantText;
  return messages.map((item) => {
    const sameAssistant =
      item.role === "assistant" &&
      ((pendingRunID && item.runID === pendingRunID) || item.publicID === pendingAssistantID);
    if (!sameAssistant) {
      return item;
    }
    const serverStatus = item.status?.trim().toLowerCase() || "success";
    const serverHasTerminalState = !item.isPending && !item.isStreaming && serverStatus !== "pending";
    const existingAlert = item.inlineAlert;
    const nextAlert = pendingAlert
      ? {
          title: existingAlert?.title || pendingAlert.title,
          message: existingAlert?.message || pendingAlert.message,
          details: existingAlert?.details?.request?.body ? existingAlert.details : pendingAlert.details,
        }
      : existingAlert;
    return {
      ...item,
      content: serverHasTerminalState && item.content ? item.content : pendingText ? pendingText : item.content,
      contentType: pendingExchange.assistantContentType ?? item.contentType,
      isPending: pendingExchange.assistantPending,
      isStreaming: pendingExchange.assistantStreaming,
      isFileProc: Boolean(pendingExchange.assistantFileProc && !pendingText),
      activityLabel: pendingExchange.assistantActivityLabel ?? item.activityLabel,
      imageAspectRatio: pendingExchange.assistantImageAspectRatio ?? item.imageAspectRatio,
      processTrace: pendingExchange.assistantProcessTrace ?? item.processTrace,
      inlineAlert: nextAlert,
      inputTokens: pendingExchange.assistantInputTokens ?? item.inputTokens,
      outputTokens: pendingExchange.assistantOutputTokens ?? item.outputTokens,
      cacheReadTokens: pendingExchange.assistantCacheReadTokens ?? item.cacheReadTokens,
      cacheWriteTokens: pendingExchange.assistantCacheWriteTokens ?? item.cacheWriteTokens,
      reasoningTokens: pendingExchange.assistantReasoningTokens ?? item.reasoningTokens,
      latencyMS: pendingExchange.assistantLatencyMS ?? item.latencyMS,
      compactDone: pendingExchange.compactDone ?? item.compactDone,
      platformModelName: pendingExchange.platformModelName ?? item.platformModelName,
      status: pendingExchange.assistantPending
        ? "pending"
        : serverHasTerminalState
          ? item.status
          : pendingExchange.assistantStatus ?? item.status,
    };
  });
}

export function useChatBranchState({
  conversationID,
  resetToken,
  messages,
  pendingExchange,
  liveRunIDs,
}: {
  conversationID: string | null;
  resetToken: number;
  messages: MessageDTO[];
  pendingExchange: PendingExchange | null;
  liveRunIDs?: ReadonlySet<string>;
}) {
  const t = useTranslations("chat.messages");
  const [branchSelections, setBranchSelections] = React.useState<Record<string, string>>({});

  React.useEffect(() => {
    setBranchSelections({});
  }, [conversationID, resetToken]);

  const serverTreeMessages = React.useMemo(
    () =>
      messages.map((item) =>
        mapServerMessage(
          item,
          {
            generationInterrupted: t("generationInterrupted"),
            streamInterrupted: t("streamInterrupted"),
            imageRunning: t("imageRunning"),
          },
          { liveRunIDs },
        ),
      ),
    [liveRunIDs, messages, t],
  );
  const serverMessagePublicIDs = React.useMemo(
    () => new Set(serverTreeMessages.map((item) => item.publicID).filter(Boolean)),
    [serverTreeMessages],
  );

  const combinedMessages = React.useMemo(
    () =>
      buildPendingMessages({
        conversationID,
        pendingExchange,
        serverTreeMessages,
        serverMessagePublicIDs,
      }),
    [conversationID, pendingExchange, serverMessagePublicIDs, serverTreeMessages],
  );
  const combinedMessagesRef = React.useRef(combinedMessages);
  React.useEffect(() => {
    combinedMessagesRef.current = combinedMessages;
  }, [combinedMessages]);
  const messageStructureKey = React.useMemo(
    () =>
      combinedMessages
        .map((item) => `${item.publicID}:${item.parentPublicID ?? ""}:${item.role}`)
        .join("|"),
    [combinedMessages],
  );

  React.useEffect(() => {
    setBranchSelections((prev) => reconcileBranchSelections(combinedMessagesRef.current, prev));
  }, [messageStructureKey]);

  const visibleMessages = React.useMemo(
    () => buildVisibleMessages(combinedMessages, branchSelections),
    [branchSelections, combinedMessages],
  );

  const visibleMessageCount = visibleMessages.length;
  const currentLeafMessage = visibleMessages.at(-1) ?? null;
  const showPendingAssistant = Boolean(
    pendingExchange &&
      visibleMessages.some(
        (item) =>
          item.role === "assistant" &&
          ((pendingExchange.runID && item.runID === pendingExchange.runID) ||
            item.publicID === (pendingExchange.assistantPublicID || pendingExchange.tempAssistantPublicID)) &&
          item.isStreaming,
      ),
  );

  return {
    branchSelections,
    setBranchSelections,
    combinedMessages,
    currentLeafMessage,
    serverMessagePublicIDs,
    showPendingAssistant,
    visibleMessageCount,
    visibleMessages,
  };
}
