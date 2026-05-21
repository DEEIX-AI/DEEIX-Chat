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
import { resolveChatSubmitTask } from "@/features/chat/model/chat-task";
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
import { buildChildrenIndex, toBranchKey } from "@/features/chat/model/chat-thread";
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
  type ConversationStreamOptions,
} from "@/shared/api/conversation";
import type {
  ConversationDTO,
  ConversationOptions,
  MediaImageRequest,
  SendMessageRequest,
  SendMessageResult,
} from "@/shared/api/conversation.types";

const CONVERSATION_METADATA_REFRESH_DELAYS = [800, 1200, 1800, 2600, 3500, 5000] as const;

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

function normalizeLabelsJSON(value: string | null | undefined): string {
  const normalized = value?.trim();
  return normalized && normalized !== "null" ? normalized : "[]";
}

function shouldRefreshGeneratedConversationMetadata(item: ConversationDTO | null): boolean {
  return item !== null && item.messageCount === 0;
}

function hasGeneratedConversationMetadataChanged(
  previous: ConversationDTO | null,
  next: ConversationDTO,
): boolean {
  const previousTitle = previous?.title?.trim() ?? "";
  const nextTitle = next.title.trim();
  if (nextTitle && nextTitle !== previousTitle) {
    return true;
  }
  return normalizeLabelsJSON(next.labelsJSON) !== normalizeLabelsJSON(previous?.labelsJSON);
}

async function refreshGeneratedConversationMetadata(
  accessToken: string,
  conversationPublicID: string,
  previous: ConversationDTO | null,
  touchByPublicID: (publicID: string, patch?: Partial<ConversationDTO>) => void,
): Promise<void> {
  for (const delay of CONVERSATION_METADATA_REFRESH_DELAYS) {
    await sleep(delay);
    let latest: ConversationDTO;
    try {
      latest = await getConversation(accessToken, conversationPublicID);
    } catch {
      continue;
    }
    if (hasGeneratedConversationMetadataChanged(previous, latest)) {
      touchByPublicID(conversationPublicID, latest);
      return;
    }
  }
}

export function useChatMessageSubmit({
  conversationID,
  resetToken,
  activeConversation,
  selectedPlatformModelName,
  modelOptions,
  selectedToolIDs,
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
  setDraft,
  setAttachments,
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
}: {
  conversationID: string | null;
  resetToken: number;
  activeConversation: ConversationDTO | null;
  selectedPlatformModelName: string;
  modelOptions: ChatModelOption[];
  selectedToolIDs: number[];
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
  setDraft: React.Dispatch<React.SetStateAction<string>>;
  setAttachments: React.Dispatch<React.SetStateAction<PendingAttachment[]>>;
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
}) {
  const t = useTranslations("chat.submit");
  const [sending, setSending] = React.useState(false);
  const activeStreamRef = React.useRef<ActiveStream | null>(null);
  const activeGenerationRunsRefRef = React.useRef(activeGenerationRunsRef);
  const previousResetTokenRef = React.useRef(resetToken);

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
      if (active.accessToken) {
        void cancelMessageGeneration(active.accessToken, active.runID).catch(() => undefined);
      }
      active.controller.abort();
      activeGenerationRunsRefRef.current?.current.delete(active.runID);
      activeStreamRef.current = null;
    }

    resetStreamBuffer();
    setPendingExchange(null);
    setSending(false);
  }, [resetStreamBuffer, resetToken, setPendingExchange]);

  React.useEffect(() => {
    if (!pendingExchange) {
      return;
    }
    const userPublicID = pendingExchange.userPublicID || pendingExchange.tempUserPublicID;
    const assistantPublicID = pendingExchange.assistantPublicID || pendingExchange.tempAssistantPublicID;
    if (!serverMessagePublicIDs.has(userPublicID) || !serverMessagePublicIDs.has(assistantPublicID)) {
      return;
    }
    setPendingExchange(null);
  }, [pendingExchange, serverMessagePublicIDs, setPendingExchange]);

  const submitMessage = React.useCallback(
    async ({
      content,
      currentAttachments,
      resetComposer,
      parentMessagePublicID,
      sourceMessagePublicID,
      branchReason,
    }: {
      content: string;
      currentAttachments: PendingAttachment[];
      resetComposer: boolean;
      parentMessagePublicID?: string | null;
      sourceMessagePublicID?: string | null;
      branchReason?: "default" | "retry" | "edit";
    }) => {
      const payloadContent = content || t("attachmentOnlyContent");
      const requestPlatformModelName = selectedPlatformModelName.trim();
      const selectedModel = modelOptions.find((item) => item.platformModelName === requestPlatformModelName) ?? null;
      if ((!content && currentAttachments.length === 0) || sending || uploading || activeStreamRef.current) {
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
      const submitTask = resolveChatSubmitTask(selectedModel, effectiveAttachments);
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
      const sanitizedOptions = sanitizeConversationOptions(options);
      const assistantImageAspectRatio =
        submitTask === "chat" ? undefined : resolveImageLoadingAspectRatio(sanitizedOptions);
      let targetConversationID = conversationID;
      let targetConversation = activeConversation;

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
      }
      startStream(exchangeKey);
      setPendingExchange({
        key: exchangeKey,
        conversationPublicID: conversationID?.trim() || null,
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
        const shouldRefreshConversationMetadata = shouldRefreshGeneratedConversationMetadata(targetConversation);

        const commonStreamPayload = {
          model: requestPlatformModelName,
          options: Object.keys(sanitizedOptions).length > 0 ? sanitizedOptions : undefined,
          clientRunID: clientRunID,
          fileIDs: effectiveAttachments.length > 0 ? effectiveAttachments.map((item) => item.fileID) : undefined,
          parentMessagePublicID: resolvedParentPublicID || undefined,
          sourceMessagePublicID: resolvedSourcePublicID || undefined,
          branchReason: resolvedBranchReason,
        };
        const streamOptions: ConversationStreamOptions = {
          signal: streamAbortController.signal,
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
            selectedToolIDs: selectedToolIDs.length > 0 ? selectedToolIDs : undefined,
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

        sentSuccessfully = true;
        flushStreamTextNow();
        resetStreamBuffer();
        setPendingExchange((prev) => {
          if (!prev || prev.key !== exchangeKey) {
            return prev;
          }
          const streamedText = prev.assistantText;
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
            assistantInputTokens: completed.assistantMessage.inputTokens,
            assistantOutputTokens: completed.assistantMessage.outputTokens,
            assistantCacheReadTokens: completed.assistantMessage.cacheReadTokens,
            assistantCacheWriteTokens: completed.assistantMessage.cacheWriteTokens,
            assistantReasoningTokens: completed.assistantMessage.reasoningTokens,
            assistantLatencyMS: completed.assistantMessage.latencyMS,
            assistantProcessTrace: toPendingProcessTrace(completed.assistantMessage.processTrace),
            assistantInlineAlert: undefined,
            assistantText:
              streamedText === completed.assistantMessage.content
                ? prev.assistantText
                : completed.assistantMessage.content,
          };
        });
        setBranchSelections((prev) => {
          const next = { ...prev };
          next[toBranchKey(resolvedParentPublicID)] = completed.userMessage.publicID;
          delete next[tempUserPublicID];
          next[completed.userMessage.publicID] = completed.assistantMessage.publicID;
          return next;
        });
        touchByPublicID(
          targetConversationID,
          toConversationPatch(targetConversation, requestPlatformModelName),
        );
        if (shouldRefreshConversationMetadata) {
          void refreshGeneratedConversationMetadata(
            token,
            targetConversationID,
            targetConversation,
            touchByPublicID,
          ).catch(() => {
            // Metadata refresh failure does not affect this turn; the next list load will fetch server state.
          });
        }
        releaseAttachments(effectiveAttachments);
        notifyResponseCompletion({
          content: completed.assistantMessage.content,
          conversationPublicID: targetConversationID,
          conversationTitle: targetConversation?.title || "DEEIX Chat",
        });
        reload();
      } catch (error) {
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
        shouldKeepConversationLayout = true;
        if (resetComposer && restoreDraftOnFailure) {
          setDraft(content);
          setAttachments(currentAttachments);
        }
        setPendingExchange((prev) =>
          prev && prev.key === exchangeKey
            ? {
                ...prev,
                assistantPending: false,
                assistantStreaming: false,
                assistantFileProc: false,
                assistantActivityLabel: undefined,
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
      activeConversation,
      activeGenerationRunsRef,
      conversationID,
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
      selectedPlatformModelName,
      sending,
      setAttachments,
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

  const onSendMessage = React.useCallback(async () => {
    const content = draft.trim();
    const parentMessage = resolveDefaultSubmissionParentMessage(visibleMessages);
    await submitMessage({
      content,
      currentAttachments: attachments,
      resetComposer: true,
      parentMessagePublicID: parentMessage?.publicID ?? currentLeafMessage?.publicID ?? null,
      branchReason: "default",
    });
  }, [attachments, currentLeafMessage?.publicID, draft, submitMessage, visibleMessages]);

  const onRetryUserMessage = React.useCallback(
    async (message: ChatAreaMessage) => {
      await submitMessage({
        content: message.content.trim(),
        currentAttachments: toPendingAttachments(message),
        resetComposer: false,
        parentMessagePublicID: message.parentPublicID,
        sourceMessagePublicID: message.publicID,
        branchReason: "retry",
      });
    },
    [submitMessage],
  );

  const onRetryAssistantMessage = React.useCallback(
    async (message: ChatAreaMessage) => {
      const parentUser = combinedMessages.find((item) => item.publicID === message.parentPublicID && item.role === "user");
      if (!parentUser) {
        toast.error(t("retryReplyFailed"), { description: t("retryReplyMissingUser") });
        return;
      }
      await submitMessage({
        content: parentUser.content.trim(),
        currentAttachments: toPendingAttachments(parentUser),
        resetComposer: false,
        parentMessagePublicID: parentUser.parentPublicID,
        sourceMessagePublicID: parentUser.publicID,
        branchReason: "retry",
      });
    },
    [combinedMessages, submitMessage, t],
  );

  const onEditUserMessage = React.useCallback(
    async (message: ChatAreaMessage, content: string) => {
      const ok = await submitMessage({
        content: content.trim(),
        currentAttachments: toPendingAttachments(message),
        resetComposer: false,
        parentMessagePublicID: message.parentPublicID,
        sourceMessagePublicID: message.publicID,
        branchReason: "edit",
      });
      return ok;
    },
    [submitMessage],
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
    onEditUserMessage,
    onRetryAssistantMessage,
    onRetryUserMessage,
    onSendMessage,
    onStopMessage,
    sending,
  };
}
