"use client";

import * as React from "react";
import { useTranslations } from "next-intl";

import type { ChatAreaMessage, MessageAttachment } from "@/features/chat/types/messages";
import type { PendingExchange, PendingExchangeMap } from "@/features/chat/types/chat-runtime";
import {
  buildVisibleMessages,
  mapServerMessage,
  reconcileBranchSelections,
} from "@/features/chat/model/chat-thread";
import type { MessageDTO } from "@/shared/api/conversation.types";
import type { UpstreamDebugInfo } from "@/shared/api/conversation.types";
import { ApiError } from "@/shared/api/http-client";
import { useLocalizedErrorMessage } from "@/i18n/use-localized-error";

function appendPendingExchangeMessages({
  conversationID,
  pendingExchange,
  messages,
  serverMessagePublicIDs,
}: {
  conversationID: string | null;
  pendingExchange: PendingExchange;
  messages: ChatAreaMessage[];
  serverMessagePublicIDs: Set<string>;
}) {
  const nextMessages = [...messages];
  const activePublicID = conversationID?.trim() || null;
  const pendingConversationPublicID = pendingExchange.conversationPublicID?.trim() || null;
  if (pendingConversationPublicID && pendingConversationPublicID !== activePublicID) {
    return nextMessages;
  }
  if (!pendingConversationPublicID && activePublicID) {
    return nextMessages;
  }
  const pendingRunID = pendingExchange.runID?.trim() || "";
  if (
    pendingRunID &&
    messages.some((item) => item.role === "assistant" && item.runID === pendingRunID)
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
    const assistantAttachments =
      pendingExchange.assistantAttachments && pendingExchange.assistantAttachments.length > 0
        ? pendingExchange.assistantAttachments
        : undefined;
    nextMessages.push({
      key: `${pendingExchange.key}-assistant`,
      publicID: assistantPublicID,
      parentPublicID: userPublicID,
      sourcePublicID: pendingExchange.reuseUserMessage ? pendingExchange.sourcePublicID : null,
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
      attachments: assistantAttachments,
    });
  }

  return nextMessages;
}

function buildPendingMessages({
  conversationID,
  pendingExchanges,
  serverTreeMessages,
  serverMessagePublicIDs,
}: {
  conversationID: string | null;
  pendingExchanges: PendingExchangeMap;
  serverTreeMessages: ChatAreaMessage[];
  serverMessagePublicIDs: Set<string>;
}) {
  return Object.values(pendingExchanges).reduce(
    (messages, pendingExchange) =>
      appendPendingExchangeMessages({
        conversationID,
        pendingExchange,
        messages,
        serverMessagePublicIDs,
      }),
    serverTreeMessages,
  );
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
      attachments:
        pendingExchange.assistantAttachments && pendingExchange.assistantAttachments.length > 0
          ? pendingExchange.assistantAttachments
          : item.attachments,
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
  pendingExchanges,
  liveRunIDs,
}: {
  conversationID: string | null;
  resetToken: number;
  messages: MessageDTO[];
  pendingExchanges: PendingExchangeMap;
  liveRunIDs?: ReadonlySet<string>;
}) {
  const t = useTranslations("chat.messages");
  const resolveErrorMessage = useLocalizedErrorMessage();
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
            resolveErrorMessage: (errorCode: string, fallback: string, details?: UpstreamDebugInfo) =>
              resolveErrorMessage(new ApiError(fallback, 502, details, errorCode), fallback),
          },
          { liveRunIDs },
        ),
      ),
    [liveRunIDs, messages, resolveErrorMessage, t],
  );
  const serverMessagePublicIDs = React.useMemo(
    () => new Set(serverTreeMessages.map((item) => item.publicID).filter(Boolean)),
    [serverTreeMessages],
  );

  const combinedMessages = React.useMemo(
    () =>
      buildPendingMessages({
        conversationID,
        pendingExchanges,
        serverTreeMessages,
        serverMessagePublicIDs,
      }),
    [conversationID, pendingExchanges, serverMessagePublicIDs, serverTreeMessages],
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

  return {
    branchSelections,
    setBranchSelections,
    combinedMessages,
    currentLeafMessage,
    serverMessagePublicIDs,
    visibleMessageCount,
    visibleMessages,
  };
}
