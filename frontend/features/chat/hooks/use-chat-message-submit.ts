"use client";

import * as React from "react";
import { useTranslations } from "next-intl";
import { toast } from "sonner";

import type { ChatAreaMessage, ImageLoadingAspectRatio } from "@/features/chat/types/messages";
import type {
  ChatModelOption,
  PendingAttachment,
  PendingExchange,
} from "@/features/chat/types/chat-runtime";
import type { ChatSubmitBlockReason } from "@/features/chat/model/chat-task";
import { resolveChatSubmitDecision } from "@/features/chat/model/chat-task";
import {
  resolveDefaultSubmissionParentMessage,
  resolvePersistedPublicID,
  toPendingAttachments,
  toPendingProcessTrace,
} from "@/features/chat/model/message-submit";
import {
  resolveErrorDetails,
  resolveErrorMessage,
  resolveErrorSummary,
  toConversationPatch,
} from "@/features/chat/utils/chat-runtime";
import {
  applyBranchSelectionPath,
  buildChildrenIndex,
  resolveBranchSelectionPath,
  toBranchKey,
} from "@/features/chat/model/chat-thread";
import { sanitizeConversationOptions } from "@/features/chat/model/conversation-options";
import { buildMediaImagePreviewMarkdown } from "@/features/chat/model/media-image-preview";
import { resolveAccessToken } from "@/shared/auth/resolve-access-token";
import { notifyResponseCompletion } from "@/shared/lib/browser-notifications";
import {
  cancelMessageGeneration,
  getConversation,
  streamImageEdit,
  streamImageGeneration,
  streamMessage as streamConversationMessage,
  updateMessage,
  type ConversationStreamOptions,
} from "@/shared/api/conversation";
import type {
  ConversationDTO,
  ConversationOptions,
  MediaImageRequest,
  MessageDTO,
  SendMessageRequest,
  SendMessageResult,
  StreamMessageEvent,
} from "@/shared/api/conversation.types";
import { ApiError } from "@/shared/api/http-client";
import type { SkillSummaryDTO } from "@/shared/api/skills.types";

const CONVERSATION_METADATA_REFRESH_MAX_WAIT_MS = 45_000;
const CONVERSATION_METADATA_REFRESH_INITIAL_DELAY_MS = 800;
const CONVERSATION_METADATA_REFRESH_MAX_DELAY_MS = 5_000;
const CONVERSATION_METADATA_REFRESH_BACKOFF = 1.5;

function resolveSubmitBlockDescription(
  reason: ChatSubmitBlockReason,
  t: (key: string) => string,
): string {
  return t(`mediaInputBlocked.${reason}`);
}

function resolveImageLoadingAspectRatio(options: ConversationOptions): ImageLoadingAspectRatio {
  const rawSize = typeof options.size === "string" ? options.size.trim() : "";
  const match = rawSize.match(/^(\d+)\s*x\s*(\d+)$/i);
  if (!match) {
    return "wide";
  }
  const width = Number(match[1]);
  const height = Number(match[2]);
  if (!Number.isFinite(width) || !Number.isFinite(height) || width <= 0 || height <= 0) {
    return "wide";
  }
  if (width > height) {
    return "wide";
  }
  if (height > width) {
    return "portrait";
  }
  return "square";
}

function streamEventErrorToApiError(
  event: Extract<StreamMessageEvent, { type: "error" }>,
  fallback: string,
): ApiError {
  return new ApiError(event.message || fallback, 502, event.debug, event.errorCode);
}

function resolveInputSideUsageValue(...values: Array<number | null | undefined>): number {
  for (const value of values) {
    if (typeof value === "number" && Number.isFinite(value) && value > 0) {
      return value;
    }
  }
  return 0;
}

function resolveMediaStatusLabel(
  status: string,
  fallbackMessage: string,
  t: ReturnType<typeof useTranslations>,
): string {
  switch (status.trim()) {
    case "queued":
      return t("mediaStatus.queued");
    case "running":
      return t("mediaStatus.running");
    case "saving_artifact":
      return t("mediaStatus.savingArtifact");
    default:
      return fallbackMessage.trim() || status.trim();
  }
}

type ActiveStream = {
  controller: AbortController;
  runID: string;
  accessToken: string | null;
};

type QueuedChatSubmission = {
  id: string;
  content: string;
  attachments: PendingAttachment[];
  platformModelName: string;
  options: ConversationOptions;
  selectedToolIDs: number[];
  selectedSkills: SkillSummaryDTO[];
  htmlVisualPromptEnabled: boolean;
  htmlVisualColorMode: "light" | "dark";
};

function sleep(ms: number): Promise<void> {
  return new Promise((resolve) => {
    window.setTimeout(resolve, ms);
  });
}

function createClientRunID(): string {
  const randomID =
    typeof window.crypto?.randomUUID === "function"
      ? window.crypto.randomUUID().replaceAll("-", "")
      : Math.random().toString(36).slice(2) + Date.now().toString(36);
  return `run_${randomID}`.slice(0, 64);
}

function buildContinueGenerationPrompt(t: ReturnType<typeof useTranslations>): string {
  return t("continueGenerationPrompt");
}

function normalizeLabelsJSON(value: string | null | undefined): string {
  const normalized = value?.trim();
  return normalized && normalized !== "null" ? normalized : "[]";
}

function isPlaceholderConversationTitle(title: string): boolean {
  const value = title.trim().toLowerCase();
  return ["", "new conversation", "new chat", "untitled", "新会话", "新对话", "新的对话"].includes(value);
}

function conversationTitleFromFirstUserMessage(content: string): string {
  const value = content.trim().replace(/\s+/g, " ").replace(/^[\s"'`“”‘’]+|[\s"'`“”‘’]+$/g, "");
  if (!value) {
    return "";
  }
  return Array.from(value).slice(0, 16).join("").trim();
}

function hasPendingGeneratedConversationMetadata(item: ConversationDTO | null): boolean {
  return (
    !item ||
    isPlaceholderConversationTitle(item.title) ||
    normalizeLabelsJSON(item.labelsJSON) === "[]"
  );
}

function hasGeneratedConversationMetadataChanged(
  previous: ConversationDTO | null,
  next: ConversationDTO,
): boolean {
  const previousTitle = previous?.title?.trim() ?? "";
  const nextTitle = next.title.trim();
  if (nextTitle && nextTitle !== previousTitle && !isPlaceholderConversationTitle(nextTitle)) {
    return true;
  }
  return normalizeLabelsJSON(next.labelsJSON) !== normalizeLabelsJSON(previous?.labelsJSON);
}

function shouldPollGeneratedConversationMetadata(
  item: ConversationDTO | null,
  result: SendMessageResult | null | undefined,
): boolean {
  if (!hasPendingGeneratedConversationMetadata(item)) {
    return false;
  }
  const hint = result?.metadataRefreshHint?.trim();
  if (!hint) {
    return true;
  }
  return hint === "pending";
}

async function refreshGeneratedConversationMetadata(
  accessToken: string,
  conversationPublicID: string,
  previous: ConversationDTO | null,
  touchByPublicID: (publicID: string, patch?: Partial<ConversationDTO>) => void,
): Promise<void> {
  let elapsedMS = 0;
  let delayMS = CONVERSATION_METADATA_REFRESH_INITIAL_DELAY_MS;
  let current = previous;

  while (elapsedMS < CONVERSATION_METADATA_REFRESH_MAX_WAIT_MS) {
    const nextDelayMS = Math.min(delayMS, CONVERSATION_METADATA_REFRESH_MAX_WAIT_MS - elapsedMS);
    await sleep(nextDelayMS);
    elapsedMS += nextDelayMS;

    let latest: ConversationDTO;
    try {
      latest = await getConversation(accessToken, conversationPublicID);
    } catch {
      continue;
    }
    if (hasGeneratedConversationMetadataChanged(current, latest)) {
      touchByPublicID(conversationPublicID, latest);
      current = latest;
      if (!hasPendingGeneratedConversationMetadata(latest)) {
        return;
      }
    }

    delayMS = Math.min(
      Math.round(delayMS * CONVERSATION_METADATA_REFRESH_BACKOFF),
      CONVERSATION_METADATA_REFRESH_MAX_DELAY_MS,
    );
  }
}

export function useChatMessageSubmit({
  conversationID,
  resetToken,
  activeConversation,
  selectedPlatformModelName,
  modelOptions,
  selectedToolIDs,
  selectedSkills,
  htmlVisualPromptEnabled,
  htmlVisualColorMode,
  options,
  draft,
  attachments,
  maxFilesPerMessage,
  uploading,
  restoreDraftOnFailure,
  prependNewConversation,
  onConversationCreated,
  touchByPublicID,
  reload,
  replaceMessage,
  setDraft,
  setAttachments,
  setSelectedSkills,
  releaseAttachments,
  pendingExchange,
  setPendingExchange,
  setBranchSelections,
  showConversationLayout,
  setShowConversationLayout,
  visibleMessageCount,
  currentLeafMessage,
  visibleMessages,
  combinedMessages,
  serverMessagePublicIDs,
  enqueueStreamText,
  flushStreamTextNow,
  resetStreamBuffer,
  startStream,
  activeGenerationRunsRef,
  failedGenerationRunsRef,
  resumeGenerationActive = false,
}: {
  conversationID: string | null;
  resetToken: number;
  activeConversation: ConversationDTO | null;
  selectedPlatformModelName: string;
  modelOptions: ChatModelOption[];
  selectedToolIDs: number[];
  selectedSkills: SkillSummaryDTO[];
  htmlVisualPromptEnabled: boolean;
  htmlVisualColorMode: "light" | "dark";
  options: ConversationOptions;
  draft: string;
  attachments: PendingAttachment[];
  maxFilesPerMessage: number;
  uploading: boolean;
  restoreDraftOnFailure: boolean;
  prependNewConversation: (platformModelName: string) => Promise<ConversationDTO | null | undefined>;
  onConversationCreated?: (conversationPublicID: string) => void;
  touchByPublicID: (publicID: string, patch?: Partial<ConversationDTO>) => void;
  reload: () => void;
  replaceMessage: (message: MessageDTO) => void;
  setDraft: React.Dispatch<React.SetStateAction<string>>;
  setAttachments: React.Dispatch<React.SetStateAction<PendingAttachment[]>>;
  setSelectedSkills: React.Dispatch<React.SetStateAction<SkillSummaryDTO[]>>;
  releaseAttachments: (items: PendingAttachment[]) => void;
  pendingExchange: PendingExchange | null;
  setPendingExchange: React.Dispatch<React.SetStateAction<PendingExchange | null>>;
  setBranchSelections: React.Dispatch<React.SetStateAction<Record<string, string>>>;
  showConversationLayout: boolean;
  setShowConversationLayout: React.Dispatch<React.SetStateAction<boolean>>;
  visibleMessageCount: number;
  currentLeafMessage: ChatAreaMessage | null;
  visibleMessages: ChatAreaMessage[];
  combinedMessages: ChatAreaMessage[];
  serverMessagePublicIDs: Set<string>;
  enqueueStreamText: (delta: string) => void;
  flushStreamTextNow: () => void;
  resetStreamBuffer: () => void;
  startStream: (exchangeKey: string) => void;
  activeGenerationRunsRef?: React.RefObject<Set<string>>;
  failedGenerationRunsRef?: React.RefObject<Set<string>>;
  resumeGenerationActive?: boolean;
}) {
  const t = useTranslations("chat.submit");
  const [sending, setSending] = React.useState(false);
  const activeStreamRef = React.useRef<ActiveStream | null>(null);
  const activeGenerationRunsRefRef = React.useRef(activeGenerationRunsRef);
  const previousResetTokenRef = React.useRef(resetToken);
  const conversationIDRef = React.useRef(conversationID);
  const activeConversationRef = React.useRef(activeConversation);
  const lastCompletedAssistantPublicIDRef = React.useRef<string | null>(null);
  const sendQueuedAfterCurrentRef = React.useRef(false);
  const [queuedSubmissions, setQueuedSubmissions] = React.useState<QueuedChatSubmission[]>([]);
  const queuedSubmissionsRef = React.useRef<QueuedChatSubmission[]>([]);

  React.useEffect(() => {
    conversationIDRef.current = conversationID;
  }, [conversationID]);

  React.useEffect(() => {
    activeConversationRef.current = activeConversation;
  }, [activeConversation]);

  React.useEffect(() => {
    queuedSubmissionsRef.current = queuedSubmissions;
  }, [queuedSubmissions]);

  React.useEffect(() => {
    activeGenerationRunsRefRef.current = activeGenerationRunsRef;
  }, [activeGenerationRunsRef]);

  React.useEffect(() => {
    if (previousResetTokenRef.current === resetToken) {
      return;
    }
    previousResetTokenRef.current = resetToken;

    const active = activeStreamRef.current;
    if (active) {
      // A new chat navigation should detach this view from the active stream without
      // canceling the server-side run. Reopening the conversation can resume it.
      active.controller.abort();
      activeGenerationRunsRefRef.current?.current.delete(active.runID);
      activeStreamRef.current = null;
    }

    resetStreamBuffer();
    setPendingExchange(null);
    setSending(false);
    lastCompletedAssistantPublicIDRef.current = null;
    sendQueuedAfterCurrentRef.current = false;
    releaseAttachments(queuedSubmissionsRef.current.flatMap((item) => item.attachments));
    setQueuedSubmissions([]);
  }, [releaseAttachments, resetStreamBuffer, resetToken, setPendingExchange]);

  React.useEffect(() => {
    if (!pendingExchange) {
      return;
    }
    const userPublicID = pendingExchange.userPublicID || pendingExchange.tempUserPublicID;
    const assistantPublicID = pendingExchange.assistantPublicID || pendingExchange.tempAssistantPublicID;
    if (serverMessagePublicIDs.has(userPublicID) && serverMessagePublicIDs.has(assistantPublicID)) {
      const serverPath = resolveBranchSelectionPath(combinedMessages, assistantPublicID);
      if (serverPath.length > 0) {
        setBranchSelections((prev) =>
          applyBranchSelectionPath(
            prev,
            serverPath,
            [pendingExchange.tempUserPublicID, pendingExchange.tempAssistantPublicID],
          ),
        );
      }
      setPendingExchange(null);
      return;
    }

    const pendingRunID = pendingExchange.runID?.trim();
    if (!pendingRunID || pendingExchange.assistantPending) {
      return;
    }
    const serverAssistant = combinedMessages.find(
      (item) =>
        item.role === "assistant" &&
        item.runID === pendingRunID &&
        serverMessagePublicIDs.has(item.publicID) &&
        resolvePersistedPublicID(item.publicID) &&
        !item.isPending &&
        !item.isStreaming &&
        item.status !== "pending",
    );
    if (serverAssistant) {
      const serverPath = resolveBranchSelectionPath(combinedMessages, serverAssistant.publicID);
      if (serverPath.length > 0) {
        setBranchSelections((prev) =>
          applyBranchSelectionPath(
            prev,
            serverPath,
            [pendingExchange.tempUserPublicID, pendingExchange.tempAssistantPublicID],
          ),
        );
      }
      setPendingExchange(null);
    }
  }, [combinedMessages, pendingExchange, serverMessagePublicIDs, setBranchSelections, setPendingExchange]);

  const submitMessage = React.useCallback(
    async ({
      content,
      currentAttachments,
      resetComposer,
      parentMessagePublicID,
      sourceMessagePublicID,
      branchReason,
      queuedSubmission,
    }: {
      content: string;
      currentAttachments: PendingAttachment[];
      resetComposer: boolean;
      parentMessagePublicID?: string | null;
      sourceMessagePublicID?: string | null;
      branchReason?: "default" | "retry" | "edit";
      queuedSubmission?: QueuedChatSubmission;
    }) => {
      const payloadContent = content || t("attachmentOnlyContent");
      const requestPlatformModelName = (queuedSubmission?.platformModelName ?? selectedPlatformModelName).trim();
      const requestOptions = queuedSubmission?.options ?? options;
      const requestSelectedToolIDs = queuedSubmission?.selectedToolIDs ?? selectedToolIDs;
      const requestSelectedSkills = queuedSubmission?.selectedSkills ?? selectedSkills;
      const requestHTMLVisualPromptEnabled = queuedSubmission?.htmlVisualPromptEnabled ?? htmlVisualPromptEnabled;
      const requestHTMLVisualColorMode = queuedSubmission?.htmlVisualColorMode ?? htmlVisualColorMode;
      const selectedModel = modelOptions.find((item) => item.platformModelName === requestPlatformModelName) ?? null;
      if ((!content && currentAttachments.length === 0) || uploading || activeStreamRef.current) {
        return false;
      }
      const effectiveAttachments =
        maxFilesPerMessage > 0 && currentAttachments.length > maxFilesPerMessage
          ? currentAttachments.slice(0, maxFilesPerMessage)
          : currentAttachments;
      if (effectiveAttachments.length < currentAttachments.length) {
        toast(t("attachmentsTruncated"), {
          description: t("attachmentsTruncatedDescription", { count: maxFilesPerMessage }),
        });
      }
      const submitDecision = resolveChatSubmitDecision(selectedModel, effectiveAttachments);
      if (submitDecision.blockedReason) {
        toast.error(t("mediaInputUnsupported"), {
          description: resolveSubmitBlockDescription(submitDecision.blockedReason, t),
        });
        return false;
      }
      const submitTask = submitDecision.task;
      if (!requestPlatformModelName) {
        toast.error(t("noModel"), { description: t("selectModelFirst") });
        return false;
      }

      const wasConversationMode = showConversationLayout || visibleMessageCount > 0;
      const exchangeKey = `local-exchange-${Date.now()}`;
      const resolvedParentPublicID = resolvePersistedPublicID(parentMessagePublicID);
      const resolvedSourcePublicID = resolvePersistedPublicID(sourceMessagePublicID);
      const resolvedBranchReason = branchReason ?? "default";
      const tempUserPublicID = `${exchangeKey}-user`;
      const tempAssistantPublicID = `${exchangeKey}-assistant`;
      const createdAt = new Date().toISOString();
      let sentSuccessfully = false;
      let shouldKeepConversationLayout = false;
      const streamAbortController = new AbortController();
      const clientRunID = createClientRunID();
      const sanitizedOptions = sanitizeConversationOptions(requestOptions);
      const assistantImageAspectRatio =
        submitTask === "chat" ? undefined : resolveImageLoadingAspectRatio(sanitizedOptions);
      let targetConversationID = conversationIDRef.current;
      let targetConversation = activeConversationRef.current;
      let metadataRefreshInFlight = false;

      activeGenerationRunsRef?.current.add(clientRunID);
      setShowConversationLayout(true);
      setSending(true);
      activeStreamRef.current = {
        controller: streamAbortController,
        runID: clientRunID,
        accessToken: null,
      };
      if (resetComposer) {
        setDraft("");
        setAttachments([]);
        setSelectedSkills([]);
      }
      startStream(exchangeKey);
      setPendingExchange({
        key: exchangeKey,
        conversationPublicID: targetConversationID?.trim() || null,
        tempUserPublicID,
        tempAssistantPublicID,
        runID: clientRunID,
        platformModelName: requestPlatformModelName,
        parentPublicID: resolvedParentPublicID,
        sourcePublicID: resolvedSourcePublicID,
        branchReason: resolvedBranchReason,
        userContent: payloadContent,
        userAttachments: effectiveAttachments.length > 0 ? effectiveAttachments : undefined,
        userCreatedAt: createdAt,
        assistantText: "",
        assistantPending: true,
        assistantStreaming: true,
        assistantContentType: submitTask === "chat" ? "markdown" : "image",
        assistantImageAspectRatio,
        assistantInlineAlert: undefined,
        assistantCreatedAt: createdAt,
        assistantProcessTrace: undefined,
      });
      setBranchSelections((prev) => ({
        ...prev,
        [toBranchKey(resolvedParentPublicID)]: tempUserPublicID,
        [tempUserPublicID]: tempAssistantPublicID,
      }));

      try {
        const token = await resolveAccessToken();
        if (streamAbortController.signal.aborted) {
          throw new DOMException("Aborted", "AbortError");
        }
        if (!token) {
          throw new Error(t("signInRequired"));
        }
        if (activeStreamRef.current?.controller === streamAbortController) {
          activeStreamRef.current = {
            controller: streamAbortController,
            runID: clientRunID,
            accessToken: token,
          };
        }
        const startMetadataRefresh = (result?: SendMessageResult | null) => {
          if (
            !targetConversationID ||
            metadataRefreshInFlight ||
            !shouldPollGeneratedConversationMetadata(targetConversation, result)
          ) {
            return;
          }
          metadataRefreshInFlight = true;
          void refreshGeneratedConversationMetadata(
            token,
            targetConversationID,
            targetConversation,
            touchByPublicID,
          )
            .catch(() => {
              // Metadata refresh failure does not affect this turn; the next list load will fetch server state.
            })
            .finally(() => {
              metadataRefreshInFlight = false;
            });
        };

        if (!targetConversationID) {
          const created = await prependNewConversation(requestPlatformModelName);
          if (streamAbortController.signal.aborted) {
            throw new DOMException("Aborted", "AbortError");
          }
          if (!created?.publicID) {
            throw new Error(t("createConversationFailed"));
          }
          targetConversationID = created.publicID;
          targetConversation = created;
          conversationIDRef.current = created.publicID;
          activeConversationRef.current = created;
          setPendingExchange((prev) =>
            prev && prev.key === exchangeKey
              ? {
                  ...prev,
                  conversationPublicID: created.publicID,
                }
              : prev,
          );
          // Update the URL without triggering Next.js RSC navigation, which can interrupt an active stream.
          window.history.replaceState(null, "", `/chat?conversation_id=${created.publicID}`);
          onConversationCreated?.(created.publicID);
        }
        const optimisticTitle = conversationTitleFromFirstUserMessage(payloadContent);
        if (
          targetConversationID &&
          optimisticTitle &&
          (!targetConversation || isPlaceholderConversationTitle(targetConversation.title))
        ) {
          if (targetConversation) {
            targetConversation = {
              ...targetConversation,
              title: optimisticTitle,
            };
            activeConversationRef.current = targetConversation;
          }
          touchByPublicID(targetConversationID, { title: optimisticTitle });
        }
        startMetadataRefresh(null);
        const commonStreamPayload = {
          model: requestPlatformModelName,
          options: Object.keys(sanitizedOptions).length > 0 ? sanitizedOptions : undefined,
          clientRunID: clientRunID,
          fileIDs: effectiveAttachments.length > 0 ? effectiveAttachments.map((item) => item.fileID) : undefined,
          parentMessagePublicID: resolvedParentPublicID || undefined,
          sourceMessagePublicID: resolvedSourcePublicID || undefined,
          branchReason: resolvedBranchReason,
        };
        let terminalStreamError: Extract<StreamMessageEvent, { type: "error" }> | null = null;
        const streamOptions: ConversationStreamOptions = {
          signal: streamAbortController.signal,
          onInterrupted: (event) => {
            terminalStreamError = event;
          },
          onFileProc: (message) => {
            setPendingExchange((prev) =>
              prev && prev.key === exchangeKey
                ? { ...prev, assistantFileProc: true, assistantActivityLabel: message.trim() || t("processingAttachments") }
                : prev,
            );
          },
          onRagSearch: (message) => {
            setPendingExchange((prev) =>
              prev && prev.key === exchangeKey
                ? { ...prev, assistantFileProc: true, assistantActivityLabel: message.trim() || t("retrievingContent") }
                : prev,
            );
          },
          onMediaStatus: (event) => {
            const activityLabel = resolveMediaStatusLabel(event.status, event.message, t);
            setPendingExchange((prev) =>
              prev && prev.key === exchangeKey
                ? { ...prev, assistantFileProc: true, assistantActivityLabel: activityLabel }
                : prev,
            );
          },
          onMediaImageDelta: (event) => {
            const previewMarkdown = buildMediaImagePreviewMarkdown(event, t("imagePreviewAlt"));
            if (!previewMarkdown) {
              return;
            }
            setPendingExchange((prev) =>
              prev && prev.key === exchangeKey
                ? {
                    ...prev,
                    assistantPending: false,
                    assistantStreaming: true,
                    assistantFileProc: false,
                    assistantActivityLabel: undefined,
                    assistantText: previewMarkdown,
                  }
                : prev,
            );
          },
          onCompactDone: (event) => {
            setPendingExchange((prev) =>
              prev && prev.key === exchangeKey
                ? { ...prev, compactDone: { method: event.method, freed_tokens: event.freed_tokens, summary_preview: event.summary_preview } }
                : prev,
            );
          },
          onProcessUpdate: (event) => {
            setPendingExchange((prev) =>
              prev && prev.key === exchangeKey
                ? {
                    ...prev,
                    assistantFileProc: false,
                    assistantActivityLabel: undefined,
                    assistantProcessTrace: event.trace ? toPendingProcessTrace(event.trace) : prev.assistantProcessTrace,
                  }
                : prev,
            );
          },
          onUpstreamThinkDelta: (event) => {
            setPendingExchange((prev) =>
              prev && prev.key === exchangeKey
                ? {
                    ...prev,
                    assistantProcessTrace: event.trace ? toPendingProcessTrace(event.trace) : prev.assistantProcessTrace,
                  }
                : prev,
            );
          },
          onDelta: (delta) => {
            // Always clear assistantFileProc so batched React updates cannot keep the file_proc spinner alive.
            setPendingExchange((prev) =>
              prev && prev.key === exchangeKey && prev.assistantFileProc
                ? { ...prev, assistantFileProc: false, assistantActivityLabel: undefined }
                : prev,
            );
            enqueueStreamText(delta);
          },
          onUsage: (event) => {
            setPendingExchange((prev) =>
              prev && prev.key === exchangeKey
                ? {
                    ...prev,
                    assistantInputTokens: event.input_tokens > 0 ? event.input_tokens : prev.assistantInputTokens,
                    assistantOutputTokens: event.output_tokens > 0 ? event.output_tokens : prev.assistantOutputTokens,
                    assistantCacheReadTokens:
                      event.cache_read_tokens > 0 ? event.cache_read_tokens : prev.assistantCacheReadTokens,
                    assistantCacheWriteTokens:
                      event.cache_write_tokens > 0 ? event.cache_write_tokens : prev.assistantCacheWriteTokens,
                    assistantReasoningTokens:
                      event.reasoning_tokens > 0 ? event.reasoning_tokens : prev.assistantReasoningTokens,
                  }
                : prev,
            );
          },
        };
        let completed: SendMessageResult;
        if (submitTask === "chat") {
          const chatPayload: SendMessageRequest = {
            ...commonStreamPayload,
            contentType: effectiveAttachments.length > 0 ? "mixed" : "text",
            content: payloadContent,
            selectedToolIDs: requestSelectedToolIDs.length > 0 ? requestSelectedToolIDs : undefined,
            skillIDs: requestSelectedSkills.length > 0 ? requestSelectedSkills.map((skill) => skill.id) : undefined,
            htmlVisualPrompt: requestHTMLVisualPromptEnabled || undefined,
            htmlVisualColorMode: requestHTMLVisualPromptEnabled ? requestHTMLVisualColorMode : undefined,
          };
          completed = await streamConversationMessage(token, targetConversationID, chatPayload, streamOptions);
        } else {
          const mediaPayload: MediaImageRequest = {
            ...commonStreamPayload,
            prompt: payloadContent,
          };
          completed =
            submitTask === "image_generation"
              ? await streamImageGeneration(token, targetConversationID, mediaPayload, streamOptions)
              : await streamImageEdit(token, targetConversationID, mediaPayload, streamOptions);
        }

        failedGenerationRunsRef?.current.delete(clientRunID);
        sentSuccessfully = true;
        lastCompletedAssistantPublicIDRef.current = completed.assistantMessage.publicID;
        flushStreamTextNow();
        resetStreamBuffer();
        const assistantMessageStatus = completed.assistantMessage.status || "success";
        const assistantMessageSucceeded = assistantMessageStatus === "success";
        setPendingExchange((prev) => {
          if (!prev || prev.key !== exchangeKey) {
            return prev;
          }
          const streamedText = prev.assistantText;
          const terminalErrorMessage = terminalStreamError
            ? resolveErrorMessage(streamEventErrorToApiError(terminalStreamError, t("retryLater")), terminalStreamError.message || t("retryLater"))
            : "";
          const completedErrorMessage = completed.assistantMessage.errorCode
            ? resolveErrorMessage(
                new ApiError(
                  completed.assistantMessage.errorMessage || t("retryLater"),
                  502,
                  terminalStreamError?.debug,
                  completed.assistantMessage.errorCode,
                ),
                completed.assistantMessage.errorMessage || t("retryLater"),
              )
            : completed.assistantMessage.errorMessage;
          return {
            ...prev,
            userPublicID: completed.userMessage.publicID,
            assistantPublicID: completed.assistantMessage.publicID,
            platformModelName: completed.assistantMessage.platformModelName?.trim() || prev.platformModelName,
            userContent: completed.userMessage.content,
            userServerMessageID: completed.userMessage.id,
            userCreatedAt: completed.userMessage.createdAt,
            assistantPending: false,
            assistantStreaming: false,
            assistantFileProc: false,
            assistantActivityLabel: undefined,
            assistantServerMessageID: completed.assistantMessage.id,
            assistantCreatedAt: completed.assistantMessage.createdAt,
            assistantUpdatedAt: completed.assistantMessage.updatedAt,
            assistantContentType: completed.assistantMessage.contentType || prev.assistantContentType,
            assistantInputTokens: resolveInputSideUsageValue(
              completed.assistantMessage.inputTokens,
              completed.userMessage.inputTokens,
              prev.assistantInputTokens,
            ),
            assistantOutputTokens: completed.assistantMessage.outputTokens,
            assistantCacheReadTokens: resolveInputSideUsageValue(
              completed.assistantMessage.cacheReadTokens,
              completed.userMessage.cacheReadTokens,
              prev.assistantCacheReadTokens,
            ),
            assistantCacheWriteTokens: resolveInputSideUsageValue(
              completed.assistantMessage.cacheWriteTokens,
              completed.userMessage.cacheWriteTokens,
              prev.assistantCacheWriteTokens,
            ),
            assistantReasoningTokens: completed.assistantMessage.reasoningTokens,
            assistantLatencyMS: completed.assistantMessage.latencyMS,
            assistantProcessTrace: toPendingProcessTrace(completed.assistantMessage.processTrace),
            assistantStatus: assistantMessageStatus,
            assistantErrorCode: completed.assistantMessage.errorCode,
            assistantErrorMessage: completed.assistantMessage.errorMessage,
            assistantInlineAlert:
              completed.assistantMessage.status === "error" || completed.assistantMessage.status === "interrupted"
                ? {
                    title: t("generationInterrupted"),
                    message: terminalErrorMessage || completedErrorMessage || t("retryLater"),
                    details: terminalStreamError?.debug,
                  }
                : undefined,
            assistantText:
              streamedText === completed.assistantMessage.content
                ? prev.assistantText
                : completed.assistantMessage.content,
          };
        });
        setBranchSelections((prev) =>
          applyBranchSelectionPath(
            prev,
            [
              {
                parentPublicID: completed.userMessage.parentPublicID || resolvedParentPublicID,
                publicID: completed.userMessage.publicID,
              },
              {
                parentPublicID: completed.userMessage.publicID,
                publicID: completed.assistantMessage.publicID,
              },
            ],
            [tempUserPublicID, tempAssistantPublicID],
          ),
        );
        touchByPublicID(
          targetConversationID,
          toConversationPatch(targetConversation, requestPlatformModelName),
        );
        if (assistantMessageSucceeded) {
          startMetadataRefresh(completed);
        }
        releaseAttachments(effectiveAttachments);
        if (assistantMessageSucceeded) {
          notifyResponseCompletion({
            content: completed.assistantMessage.content,
            conversationPublicID: targetConversationID,
            conversationTitle: targetConversation?.title || "DEEIX Chat",
          });
        }
        reload();
      } catch (error) {
        flushStreamTextNow();
        resetStreamBuffer();
        if (streamAbortController.signal.aborted) {
          shouldKeepConversationLayout = true;
          releaseAttachments(effectiveAttachments);
          setPendingExchange((prev) =>
            prev && prev.key === exchangeKey
              ? {
                  ...prev,
                  assistantPending: false,
                  assistantStreaming: false,
                  assistantFileProc: false,
                  assistantActivityLabel: undefined,
                  assistantInlineAlert: undefined,
                }
              : prev,
          );
          return false;
        }
        const errorMessage = resolveErrorMessage(error, t("retryLater"));
        const errorDetails = resolveErrorDetails(error);
        const errorSummary = resolveErrorSummary(error, t("retryLater"));
        failedGenerationRunsRef?.current.add(clientRunID);
        shouldKeepConversationLayout = true;
        if (resetComposer && restoreDraftOnFailure) {
          setDraft(content);
          setAttachments(currentAttachments);
          setSelectedSkills(requestSelectedSkills);
        }
        setPendingExchange((prev) =>
          prev && prev.key === exchangeKey
            ? {
                ...prev,
                assistantPending: false,
                assistantStreaming: false,
                assistantFileProc: false,
                assistantActivityLabel: undefined,
                assistantStatus: "error",
                assistantErrorMessage: errorMessage,
                assistantInlineAlert: {
                  title: t("generationInterrupted"),
                  message: errorMessage,
                  details: errorDetails,
                },
              }
            : prev,
        );
        toast.error(t("sendFailed"), { description: errorSummary });
        if (targetConversationID) {
          reload();
        }
        return false;
      } finally {
        if (activeStreamRef.current?.controller === streamAbortController) {
          activeStreamRef.current = null;
        }
        activeGenerationRunsRef?.current.delete(clientRunID);
        if (!sentSuccessfully && !wasConversationMode && !shouldKeepConversationLayout) {
          setShowConversationLayout(false);
        }
        setSending(false);
      }
      return true;
    },
    [
      activeGenerationRunsRef,
      failedGenerationRunsRef,
      enqueueStreamText,
      flushStreamTextNow,
      options,
      onConversationCreated,
      prependNewConversation,
      releaseAttachments,
      reload,
      resetStreamBuffer,
      restoreDraftOnFailure,
      modelOptions,
      selectedToolIDs,
      selectedSkills,
      htmlVisualPromptEnabled,
      htmlVisualColorMode,
      selectedPlatformModelName,
      setAttachments,
      setSelectedSkills,
      setBranchSelections,
      setDraft,
      setPendingExchange,
      setShowConversationLayout,
      showConversationLayout,
      startStream,
      touchByPublicID,
      uploading,
      maxFilesPerMessage,
      t,
      visibleMessageCount,
    ],
  );

  const enqueueSubmission = React.useCallback(() => {
    const content = draft.trim();
    const currentAttachments = attachments.slice();
    if ((!content && currentAttachments.length === 0) || uploading) {
      return false;
    }
    setQueuedSubmissions((current) => [
      ...current,
      {
        id: createClientRunID().replace("run_", "queue_"),
        content,
        attachments: currentAttachments,
        platformModelName: selectedPlatformModelName,
        options: sanitizeConversationOptions(options),
        selectedToolIDs: selectedToolIDs.slice(),
        selectedSkills: selectedSkills.slice(),
        htmlVisualPromptEnabled,
        htmlVisualColorMode,
      },
    ]);
    setDraft("");
    setAttachments([]);
    return true;
  }, [
    attachments,
    draft,
    htmlVisualColorMode,
    htmlVisualPromptEnabled,
    options,
    selectedPlatformModelName,
    selectedSkills,
    selectedToolIDs,
    setAttachments,
    setDraft,
    uploading,
  ]);

  const onStopMessage = React.useCallback(() => {
    const active = activeStreamRef.current;
    if (!active) {
      return;
    }
    if (active.accessToken) {
      void cancelMessageGeneration(active.accessToken, active.runID).catch(() => undefined);
    }
    active.controller.abort();
  }, []);

  const onDeleteQueuedMessage = React.useCallback((id: string) => {
    const target = queuedSubmissionsRef.current.find((item) => item.id === id);
    if (target) {
      releaseAttachments(target.attachments);
    }
    setQueuedSubmissions((current) => current.filter((item) => item.id !== id));
  }, [releaseAttachments]);

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
      return [target, ...current.filter((item) => item.id !== id)];
    });
    sendQueuedAfterCurrentRef.current = true;
  }, []);

  const onSendMessage = React.useCallback(async () => {
    if (activeStreamRef.current || sending || resumeGenerationActive) {
      enqueueSubmission();
      return;
    }
    const content = draft.trim();
    const parentMessagePublicID =
      resolvePersistedPublicID(currentLeafMessage?.publicID) ??
      resolveDefaultSubmissionParentMessage(visibleMessages)?.publicID ??
      null;
    await submitMessage({
      content,
      currentAttachments: attachments,
      resetComposer: true,
      parentMessagePublicID,
      branchReason: "default",
    });
  }, [attachments, currentLeafMessage?.publicID, draft, enqueueSubmission, resumeGenerationActive, sending, submitMessage, visibleMessages]);

  React.useEffect(() => {
    if (
      sending ||
      resumeGenerationActive ||
      activeStreamRef.current ||
      (pendingExchange && !sendQueuedAfterCurrentRef.current) ||
      queuedSubmissions.length === 0 ||
      uploading
    ) {
      return;
    }
    const queuedSubmission = queuedSubmissions[0];
    if (!queuedSubmission) {
      return;
    }
    sendQueuedAfterCurrentRef.current = false;
    setQueuedSubmissions((current) => current.filter((item) => item.id !== queuedSubmission.id));
    const parentMessagePublicID =
      lastCompletedAssistantPublicIDRef.current ??
      resolvePersistedPublicID(currentLeafMessage?.publicID) ??
      resolveDefaultSubmissionParentMessage(visibleMessages)?.publicID ??
      null;
    void submitMessage({
      content: queuedSubmission.content,
      currentAttachments: queuedSubmission.attachments,
      resetComposer: false,
      parentMessagePublicID,
      branchReason: "default",
      queuedSubmission,
    });
  }, [currentLeafMessage?.publicID, pendingExchange, queuedSubmissions, resumeGenerationActive, sending, submitMessage, uploading, visibleMessages]);

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
    async (message: ChatAreaMessage) => {
      const parentUser = combinedMessages.find((item) => item.publicID === message.parentPublicID && item.role === "user");
      if (!parentUser) {
        toast.error(t("retryReplyFailed"), { description: t("retryReplyMissingUser") });
        return;
      }
      const sourceMessagePublicID = resolvePersistedPublicID(parentUser.publicID);
      if (!sourceMessagePublicID) {
        toast.error(t("retryReplyFailed"), { description: t("continueReplyUnavailable") });
        return;
      }
      await submitMessage({
        content: parentUser.content.trim(),
        currentAttachments: toPendingAttachments(parentUser),
        resetComposer: false,
        parentMessagePublicID: parentUser.parentPublicID,
        sourceMessagePublicID,
        branchReason: "retry",
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

  const onEditUserMessage = React.useCallback(
    async (message: ChatAreaMessage, content: string) => {
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
    [submitMessage, t],
  );

  const onEditAssistantMessage = React.useCallback(
    async (message: ChatAreaMessage, content: string) => {
      const messagePublicID = resolvePersistedPublicID(message.publicID);
      const nextContent = content.trim();
      if (!messagePublicID || !nextContent) {
        toast.error(t("editReplyFailed"), { description: t("continueReplyUnavailable") });
        return false;
      }
      const token = await resolveAccessToken();
      if (!token) {
        toast.error(t("editReplyFailed"), { description: t("signInRequired") });
        return false;
      }
      try {
        const updated = await updateMessage(token, messagePublicID, { content: nextContent });
        replaceMessage(updated);
        return true;
      } catch {
        toast.error(t("editReplyFailed"), { description: t("retryLater") });
        return false;
      }
    },
    [replaceMessage, t],
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
    onCycleMessageBranch,
    onEditAssistantMessage,
    onEditUserMessage,
    onContinueAssistantMessage,
    onRetryAssistantMessage,
    onRetryUserMessage,
    onSendMessage,
    onStopMessage,
    onDeleteQueuedMessage,
    onEditQueuedMessage,
    onGuideQueuedMessage,
    queuedMessages: queuedSubmissions.map((item) => ({
      id: item.id,
      content: item.content,
      attachmentCount: item.attachments.length,
    })),
    sending,
  };
}
