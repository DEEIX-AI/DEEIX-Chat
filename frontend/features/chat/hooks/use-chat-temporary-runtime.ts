"use client";

import { useTranslations } from "next-intl";
import * as React from "react";
import { toast } from "sonner";

import { mapServerMessage } from "@/features/chat/model/chat-thread";
import { toPendingProcessTrace } from "@/features/chat/model/message-submit";
import {
  clearLiveUpstreamThinkTrace,
  upsertLiveUpstreamThinkTrace,
} from "@/features/chat/model/upstream-think-store";
import type { PendingAttachment } from "@/features/chat/types/chat-runtime";
import type { ChatAreaMessage, MessageAttachment } from "@/features/chat/types/messages";
import {
  resolveErrorDetails,
  resolveErrorMessage,
  resolveErrorSummary,
} from "@/features/chat/utils/chat-runtime";
import {
  streamTemporaryChatMessage,
  TEMPORARY_CHAT_MAX_ATTACHMENTS,
  TEMPORARY_CHAT_MAX_IMAGE_ATTACHMENTS,
} from "@/shared/api/conversation";
import type { TemporaryChatRequestAttachment } from "@/shared/api/conversation";
import type { ConversationOptions, TemporaryChatHistoryMessage } from "@/shared/api/conversation.types";
import type { FileContentLoader } from "@/shared/components/file-preview/preview-dialog";
import { resolveAccessToken } from "@/shared/auth/resolve-access-token";
import { createSecureUUID } from "@/shared/lib/secure-id";

type TemporaryMessage = TemporaryChatHistoryMessage & {
  id: string;
  historyOffset: number;
  parentID?: string;
  model?: string;
  runID?: string;
  streaming?: boolean;
  failed?: boolean;
  inputTokens?: number;
  outputTokens?: number;
  latencyMS?: number;
  activityLabel?: string;
  processTrace?: ChatAreaMessage["processTrace"];
  knowledgeSources?: ChatAreaMessage["knowledgeSources"];
  inlineAlert?: ChatAreaMessage["inlineAlert"];
  attachments?: MessageAttachment[];
  localAttachments?: PendingAttachment[];
};

type TemporaryHistoryMessage = TemporaryChatHistoryMessage & {
  attachments?: PendingAttachment[];
};

type TemporaryChatRuntimeInput = {
  active: boolean;
  draft: string;
  model: string;
  options: ConversationOptions;
  selectedToolIDs: number[];
  selectedSkillIDs: number[];
  selectedKnowledgeBaseIDs: string[];
  htmlVisualPromptEnabled: boolean;
  attachments: PendingAttachment[];
  onDraftChange: (value: string) => void;
  onAttachmentsConsumed: (items: PendingAttachment[]) => void;
  releaseAttachments: (items: PendingAttachment[]) => void;
};

type SubmitTemporaryTurnInput = {
  content: string;
  localAttachments: PendingAttachment[];
  baseHistory: TemporaryHistoryMessage[];
  replaceFromIndex?: number;
  consumeComposer: boolean;
};

function toMessageAttachment(item: PendingAttachment): MessageAttachment {
  return {
    fileID: item.fileID,
    fileName: item.fileName,
    mimeType: item.mimeType,
    detectedMime: item.detectedMime,
    fileCategory: item.fileCategory,
    sizeBytes: item.sizeBytes,
    kind: item.fileCategory === "image" || item.mimeType.startsWith("image/") ? "image" : "file",
    previewURL: item.previewURL,
    processingStatus: item.processingStatus,
    processingReady: item.processingReady,
    extractStatus: item.extractStatus,
    embedStatus: item.embedStatus,
    ragReady: item.ragReady,
    ragReason: item.ragReason,
    ocrUsed: item.ocrUsed,
  };
}

function isImageAttachment(item: PendingAttachment): boolean {
  return item.fileCategory === "image" || item.mimeType.startsWith("image/");
}

function prepareTemporaryRequestHistory(fullHistory: TemporaryHistoryMessage[]): {
  attachments: TemporaryChatRequestAttachment[];
  messages: TemporaryChatHistoryMessage[];
} {
  const historyTruncated = fullHistory.length > 99;
  const history = historyTruncated ? fullHistory.slice(-99) : fullHistory;
  const selected: Array<TemporaryChatRequestAttachment & { fileID: string }> = [];
  let imageCount = 0;

  for (let messageIndex = history.length - 1; messageIndex >= 0; messageIndex -= 1) {
    const message = history[messageIndex];
    if (message.role !== "user") {
      continue;
    }
    const messageAttachments = message.attachments ?? [];
    for (let index = messageAttachments.length - 1; index >= 0; index -= 1) {
      const attachment = messageAttachments[index];
      if (!attachment.localFile || selected.length >= TEMPORARY_CHAT_MAX_ATTACHMENTS) {
        continue;
      }
      const image = isImageAttachment(attachment);
      if (image && imageCount >= TEMPORARY_CHAT_MAX_IMAGE_ATTACHMENTS) {
        continue;
      }
      if (image) {
        imageCount += 1;
      }
      selected.push({
        file: attachment.localFile,
        fileID: attachment.fileID,
        kind: image ? "image" : "file",
        messageIndex,
      });
    }
  }

  selected.reverse();
  const selectedIDs = new Set(selected.map((item) => item.fileID));
  const messages = history.map((message, messageIndex): TemporaryChatHistoryMessage => {
    const content = [
      historyTruncated && messageIndex === 0 ? "[Earlier temporary conversation omitted from this request]" : "",
      message.content.trim(),
    ].filter(Boolean).join("\n\n");
    if (message.role !== "user") {
      return { role: message.role, content };
    }
    const omittedNames = (message.attachments ?? [])
      .filter((item) => !selectedIDs.has(item.fileID))
      .map((item) => item.fileName);
    if (omittedNames.length === 0) {
      return { role: message.role, content };
    }
    const notice = `[Earlier temporary attachments omitted from this request: ${JSON.stringify(omittedNames)}]`;
    return { role: message.role, content: content ? `${content}\n\n${notice}` : notice };
  });

  return {
    attachments: selected.map(({ fileID: _fileID, ...item }) => item),
    messages,
  };
}

export function useChatTemporaryRuntime({
  active,
  draft,
  model,
  options,
  selectedToolIDs,
  selectedSkillIDs,
  selectedKnowledgeBaseIDs,
  htmlVisualPromptEnabled,
  attachments,
  onDraftChange,
  onAttachmentsConsumed,
  releaseAttachments,
}: TemporaryChatRuntimeInput) {
  const t = useTranslations("chat.temporary");
  const tSubmit = useTranslations("chat.submit");
  const [messages, setMessages] = React.useState<TemporaryMessage[]>([]);
  const [sending, setSending] = React.useState(false);
  const messagesRef = React.useRef<TemporaryMessage[]>([]);
  const historyRef = React.useRef<TemporaryHistoryMessage[]>([]);
  const retainedAttachmentsRef = React.useRef(new Map<string, PendingAttachment>());
  const sessionIDRef = React.useRef("");
  const abortControllerRef = React.useRef<AbortController | null>(null);
  const sendingRef = React.useRef(false);
  const liveRunIDsRef = React.useRef(new Set<string>());

  const clearLiveTraces = React.useCallback(() => {
    liveRunIDsRef.current.forEach(clearLiveUpstreamThinkTrace);
    liveRunIDsRef.current.clear();
  }, []);

  const clearRetainedAttachments = React.useCallback(() => {
    const retained = Array.from(retainedAttachmentsRef.current.values());
    retainedAttachmentsRef.current.clear();
    if (retained.length > 0) {
      releaseAttachments(retained);
    }
  }, [releaseAttachments]);

  const replaceMessages = React.useCallback((next: TemporaryMessage[]) => {
    messagesRef.current = next;
    setMessages(next);
  }, []);

  const updateMessage = React.useCallback(
    (messageID: string, update: (message: TemporaryMessage) => TemporaryMessage) => {
      replaceMessages(messagesRef.current.map((message) =>
        message.id === messageID ? update(message) : message
      ));
    },
    [replaceMessages],
  );

  const replaceMessageTail = React.useCallback((
    fromIndex: number,
    replacements: TemporaryMessage[],
    preservedAttachmentIDs: ReadonlySet<string> = new Set(),
  ) => {
    const current = messagesRef.current;
    const released: PendingAttachment[] = [];
    for (const message of current.slice(fromIndex)) {
      if (message.runID) {
        clearLiveUpstreamThinkTrace(message.runID);
        liveRunIDsRef.current.delete(message.runID);
      }
      for (const attachment of message.localAttachments ?? []) {
        if (preservedAttachmentIDs.has(attachment.fileID)) {
          continue;
        }
        const retained = retainedAttachmentsRef.current.get(attachment.fileID);
        if (retained) {
          retainedAttachmentsRef.current.delete(attachment.fileID);
          released.push(retained);
        }
      }
    }
    if (released.length > 0) {
      releaseAttachments(released);
    }
    replaceMessages([...current.slice(0, fromIndex), ...replacements]);
  }, [releaseAttachments, replaceMessages]);

  const finishSending = React.useCallback((controller: AbortController) => {
    if (abortControllerRef.current !== controller) {
      return;
    }
    abortControllerRef.current = null;
    setSending(false);
    sendingRef.current = false;
  }, []);

  React.useEffect(() => {
    if (active) {
      return;
    }
    abortControllerRef.current?.abort();
    abortControllerRef.current = null;
    sendingRef.current = false;
    sessionIDRef.current = "";
    historyRef.current = [];
    clearRetainedAttachments();
    clearLiveTraces();
    setSending(false);
    replaceMessages([]);
  }, [active, clearLiveTraces, clearRetainedAttachments, replaceMessages]);

  React.useEffect(() => {
    const abort = () => abortControllerRef.current?.abort();
    window.addEventListener("pagehide", abort);
    return () => {
      window.removeEventListener("pagehide", abort);
      abort();
      sessionIDRef.current = "";
      historyRef.current = [];
      clearRetainedAttachments();
      clearLiveTraces();
    };
  }, [clearLiveTraces, clearRetainedAttachments]);

  const stop = React.useCallback(() => {
    abortControllerRef.current?.abort();
  }, []);

  const loadAttachmentContent = React.useCallback<FileContentLoader>(async (file, signal) => {
    if (signal.aborted) {
      throw new DOMException("The operation was aborted", "AbortError");
    }
    const source = retainedAttachmentsRef.current.get(file.fileID)?.localFile;
    if (!source) {
      throw new Error("Temporary attachment is no longer available");
    }
    return {
      blob: source,
      contentType: source.type || "application/octet-stream",
      disposition: null,
      contentLength: source.size,
    };
  }, []);

  const submitTurn = React.useCallback(async ({
    content,
    localAttachments,
    baseHistory,
    replaceFromIndex,
    consumeComposer,
  }: SubmitTemporaryTurnInput): Promise<boolean> => {
    const normalizedContent = content.trim();
    const selectedModel = model.trim();
    if (
      !active ||
      (!normalizedContent && localAttachments.length === 0) ||
      !selectedModel ||
      sendingRef.current
    ) {
      return false;
    }
    if (localAttachments.some((item) => !(item.localFile instanceof File))) {
      toast.error(t("failed"));
      return false;
    }

    sendingRef.current = true;
    setSending(true);
    const controller = new AbortController();
    abortControllerRef.current = controller;
    const token = await resolveAccessToken().catch(() => "");
    if (controller.signal.aborted) {
      finishSending(controller);
      return false;
    }
    if (!token) {
      finishSending(controller);
      toast.error(t("sessionExpired"));
      return false;
    }
    if (!sessionIDRef.current) {
      sessionIDRef.current = createSecureUUID();
    }

    for (const attachment of localAttachments) {
      retainedAttachmentsRef.current.set(attachment.fileID, attachment);
    }
    if (consumeComposer) {
      onAttachmentsConsumed(localAttachments);
      onDraftChange("");
    }

    historyRef.current = baseHistory;
    const historyOffset = baseHistory.length;
    const currentMessages = messagesRef.current;
    const insertionIndex = replaceFromIndex ?? currentMessages.length;
    const userMessage: TemporaryMessage = {
      id: createSecureUUID(),
      role: "user",
      content: normalizedContent,
      historyOffset,
      parentID: currentMessages[insertionIndex - 1]?.id,
      attachments: localAttachments.map(toMessageAttachment),
      localAttachments,
    };
    const assistantID = createSecureUUID();
    const clientRunID = createSecureUUID();
    const replacements: TemporaryMessage[] = [
      userMessage,
      {
        id: assistantID,
        role: "assistant",
        content: "",
        historyOffset,
        parentID: userMessage.id,
        model: selectedModel,
        runID: clientRunID,
        streaming: true,
      },
    ];
    if (replaceFromIndex === undefined) {
      replaceMessages([...currentMessages, ...replacements]);
    } else {
      replaceMessageTail(
        replaceFromIndex,
        replacements,
        new Set(localAttachments.map((item) => item.fileID)),
      );
    }

    const userHistory: TemporaryHistoryMessage = {
      role: "user",
      content: normalizedContent,
      attachments: localAttachments,
    };
    const preparedRequest = prepareTemporaryRequestHistory([...baseHistory, userHistory]);
    let streamedAssistantText = "";
    let moderationBlocked = false;

    try {
      const completed = await streamTemporaryChatMessage(
        token,
        {
          sessionID: sessionIDRef.current,
          clientRunID,
          model: selectedModel,
          options,
          selectedToolIDs: selectedToolIDs.length > 0 ? selectedToolIDs : undefined,
          skillIDs: selectedSkillIDs.length > 0 ? selectedSkillIDs : undefined,
          knowledgeBaseIDs: selectedKnowledgeBaseIDs.length > 0 ? selectedKnowledgeBaseIDs : undefined,
          htmlVisualPrompt: htmlVisualPromptEnabled || undefined,
          messages: preparedRequest.messages,
        },
        {
          signal: controller.signal,
          onDelta: (delta) => {
            streamedAssistantText += delta;
            updateMessage(assistantID, (message) => ({
              ...message,
              content: `${message.content}${delta}`,
              activityLabel: undefined,
            }));
          },
          onRagSearch: (message) => {
            updateMessage(assistantID, (item) => ({ ...item, activityLabel: message }));
          },
          onProcessUpdate: (event) => {
            updateMessage(assistantID, (message) => ({
              ...message,
              activityLabel: undefined,
              processTrace: event.trace
                ? toPendingProcessTrace(event.trace)
                : message.processTrace,
            }));
          },
          onUpstreamThinkDelta: (event) => {
            liveRunIDsRef.current.add(clientRunID);
            upsertLiveUpstreamThinkTrace(clientRunID, event);
          },
          onUsage: (event) => {
            updateMessage(assistantID, (message) => ({
              ...message,
              inputTokens: event.input_tokens > 0 ? event.input_tokens : message.inputTokens,
              outputTokens: event.output_tokens > 0 ? event.output_tokens : message.outputTokens,
            }));
          },
          onModerationBlocked: (event) => {
            moderationBlocked = true;
            // 临时对话没有可回看的账单，拦截后仍计费的说明只能随实时事件一并展示。
            updateMessage(assistantID, (message) => ({
              ...message,
              content: [t("blocked"), event.billedReason ? tSubmit("moderationBlockedBilled") : ""]
                .filter(Boolean)
                .join("\n"),
              streaming: false,
              failed: true,
            }));
          },
        },
        preparedRequest.attachments,
      );
      if (controller.signal.aborted || abortControllerRef.current !== controller) {
        throw new DOMException("The operation was aborted", "AbortError");
      }
      const mappedAssistant = mapServerMessage(completed.assistantMessage);
      historyRef.current = [
        ...baseHistory,
        userHistory,
        { role: "assistant", content: completed.assistantMessage.content },
      ];
      updateMessage(assistantID, (message) => ({
        ...message,
        content: completed.assistantMessage.content,
        streaming: false,
        failed: false,
        inputTokens: completed.userMessage.inputTokens,
        outputTokens: completed.assistantMessage.outputTokens,
        latencyMS: completed.assistantMessage.latencyMS,
        activityLabel: undefined,
        processTrace: mappedAssistant.processTrace,
        knowledgeSources: mappedAssistant.knowledgeSources,
        inlineAlert: undefined,
      }));
    } catch (error) {
      if (abortControllerRef.current !== controller) {
        return false;
      }
      const aborted = controller.signal.aborted;
      if (!moderationBlocked && streamedAssistantText.trim()) {
        historyRef.current = [
          ...baseHistory,
          userHistory,
          { role: "assistant", content: streamedAssistantText },
        ];
      }
      if (moderationBlocked) {
        updateMessage(assistantID, (message) => ({
          ...message,
          streaming: false,
          failed: true,
          activityLabel: undefined,
          inlineAlert: undefined,
        }));
        return false;
      }
      const errorMessage = aborted ? "" : resolveErrorMessage(error, tSubmit("retryLater"));
      const errorDetails = aborted ? undefined : resolveErrorDetails(error);
      updateMessage(assistantID, (message) => ({
        ...message,
        content: aborted ? message.content || t("stopped") : message.content,
        streaming: false,
        failed: true,
        activityLabel: undefined,
        inlineAlert: aborted
          ? undefined
          : {
              title: tSubmit("generationInterrupted"),
              message: errorMessage,
              details: errorDetails,
            },
      }));
      if (!aborted) {
        toast.error(tSubmit("sendFailed"), {
          description: resolveErrorSummary(error, tSubmit("retryLater")),
        });
      }
    } finally {
      finishSending(controller);
    }
    return true;
  }, [
    active,
    finishSending,
    htmlVisualPromptEnabled,
    model,
    onAttachmentsConsumed,
    onDraftChange,
    options,
    replaceMessageTail,
    replaceMessages,
    selectedKnowledgeBaseIDs,
    selectedSkillIDs,
    selectedToolIDs,
    t,
    tSubmit,
    updateMessage,
  ]);

  const send = React.useCallback(async () => {
    const currentAttachments = attachments.filter((item) => item.localFile instanceof File);
    if (currentAttachments.length !== attachments.length) {
      toast.error(t("failed"));
      return;
    }
    await submitTurn({
      content: draft,
      localAttachments: currentAttachments,
      baseHistory: [...historyRef.current],
      consumeComposer: true,
    });
  }, [attachments, draft, submitTurn, t]);

  const resolveUserTurn = React.useCallback((message: ChatAreaMessage) => {
    const messageIndex = messagesRef.current.findIndex((item) => item.id === message.publicID);
    if (messageIndex < 0) {
      return null;
    }
    const target = messagesRef.current[messageIndex];
    if (target.role === "user") {
      return { message: target, index: messageIndex };
    }
    for (let index = messageIndex - 1; index >= 0; index -= 1) {
      const candidate = messagesRef.current[index];
      if (candidate.role === "user" && candidate.historyOffset === target.historyOffset) {
        return { message: candidate, index };
      }
    }
    return null;
  }, []);

  const retryMessage = React.useCallback(async (message: ChatAreaMessage) => {
    const turn = resolveUserTurn(message);
    if (!turn) {
      toast.error(t("failed"));
      return;
    }
    await submitTurn({
      content: turn.message.content,
      localAttachments: turn.message.localAttachments ?? [],
      baseHistory: historyRef.current.slice(0, turn.message.historyOffset),
      replaceFromIndex: turn.index,
      consumeComposer: false,
    });
  }, [resolveUserTurn, submitTurn, t]);

  const editUserMessage = React.useCallback(async (message: ChatAreaMessage, content: string) => {
    const turn = resolveUserTurn(message);
    if (!turn) {
      toast.error(t("failed"));
      return false;
    }
    return submitTurn({
      content,
      localAttachments: turn.message.localAttachments ?? [],
      baseHistory: historyRef.current.slice(0, turn.message.historyOffset),
      replaceFromIndex: turn.index,
      consumeComposer: false,
    });
  }, [resolveUserTurn, submitTurn, t]);

  const editAssistantMessage = React.useCallback(async (message: ChatAreaMessage, content: string) => {
    const nextContent = content.trim();
    const assistantIndex = messagesRef.current.findIndex(
      (item) => item.id === message.publicID && item.role === "assistant",
    );
    if (!nextContent || assistantIndex < 0 || sendingRef.current) {
      return false;
    }
    const assistant = messagesRef.current[assistantIndex];
    const turn = resolveUserTurn(message);
    if (!turn) {
      toast.error(t("failed"));
      return false;
    }
    replaceMessageTail(assistantIndex + 1, []);
    updateMessage(assistant.id, (current) => ({
      ...current,
      content: nextContent,
      streaming: false,
      failed: false,
      activityLabel: undefined,
      inlineAlert: undefined,
    }));
    historyRef.current = [
      ...historyRef.current.slice(0, turn.message.historyOffset),
      {
        role: "user",
        content: turn.message.content,
        attachments: turn.message.localAttachments ?? [],
      },
      { role: "assistant", content: nextContent },
    ];
    return true;
  }, [replaceMessageTail, resolveUserTurn, t, updateMessage]);

  const areaMessages = React.useMemo<ChatAreaMessage[]>(
    () => messages.map((message): ChatAreaMessage => ({
      key: message.id,
      publicID: message.id,
      parentPublicID: message.parentID ?? null,
      sourcePublicID: null,
      role: message.role,
      content: message.content,
      branchReason: "default",
      status: message.failed ? "failed" : message.streaming ? "processing" : "completed",
      isStreaming: message.streaming,
      platformModelName: message.model,
      runID: message.runID,
      inputTokens: message.inputTokens,
      outputTokens: message.outputTokens,
      latencyMS: message.latencyMS,
      activityLabel: message.activityLabel,
      processTrace: message.processTrace,
      knowledgeSources: message.knowledgeSources,
      inlineAlert: message.inlineAlert,
      attachments: message.attachments,
    })),
    [messages],
  );

  return {
    messages: areaMessages,
    sending,
    send,
    stop,
    loadAttachmentContent,
    onRetryUserMessage: retryMessage,
    onRetryAssistantMessage: retryMessage,
    onEditUserMessage: editUserMessage,
    onEditAssistantMessage: editAssistantMessage,
  };
}
