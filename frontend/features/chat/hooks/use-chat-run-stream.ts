"use client";

import { useTranslations } from "next-intl";
import * as React from "react";
import { toast } from "sonner";
import type { ChatSubmitTask } from "@/features/chat/model/chat-task";
import { buildMediaImagePreviewMarkdown } from "@/features/chat/model/media-image-preview";
import { toPendingProcessTrace } from "@/features/chat/model/message-submit";
import { settleCompletedExchange } from "@/features/chat/model/message-submit-exchange";
import {
  resolveMediaStatusLabel,
  resolveVideoExtensionOptions,
} from "@/features/chat/model/message-submit-media";
import type { PendingAttachment, PendingExchange } from "@/features/chat/types/chat-runtime";
import {
  type ConversationStreamOptions,
  streamMessage as streamConversationMessage,
  streamImageEdit,
  streamImageGeneration,
  streamVideoExtension,
  streamVideoGeneration,
} from "@/shared/api/conversation";
import type {
  ConversationOptions,
  MediaImageRequest,
  MediaVideoExtensionRequest,
  MediaVideoRequest,
  SendMessageRequest,
  SendMessageResult,
  StreamMessageEvent,
} from "@/shared/api/conversation.types";
import type { SkillSummaryDTO } from "@/shared/api/skills.types";

// 单次生成的流式执行:注册流事件回调、按任务类型派发请求、完成后结算 exchange。
export function useChatRunStream({
  updatePendingExchange,
  enqueueUpstreamThinkDelta,
  enqueueStreamText,
  flushStreamTextNow,
  flushUpstreamThinkNow,
  resetStreamBuffer,
  setStreamTextSnapshot,
  onConversationRunFinished,
}: {
  updatePendingExchange: (
    exchangeKey: string,
    update: (current: PendingExchange) => PendingExchange,
  ) => void;
  enqueueUpstreamThinkDelta: (
    exchangeKey: string,
    event: Extract<StreamMessageEvent, { type: "upstream_think_delta" }>,
  ) => void;
  enqueueStreamText: (exchangeKey: string, delta: string) => void;
  flushStreamTextNow: (exchangeKey: string) => void;
  flushUpstreamThinkNow: (exchangeKey: string) => void;
  resetStreamBuffer: (exchangeKey?: string) => void;
  setStreamTextSnapshot: (exchangeKey: string, content: string) => void;
  onConversationRunFinished?: (runID: string) => void;
}) {
  const t = useTranslations("chat.submit");

  const runStream = React.useCallback(
    async ({
      token,
      conversationID,
      submitTask,
      exchangeKey,
      clientRunID,
      content,
      options,
      effectiveAttachments,
      platformModelName,
      selectedToolIDs,
      selectedSkills,
      selectedKnowledgeBaseIDs,
      htmlVisualPromptEnabled,
      parentMessagePublicID,
      sourceMessagePublicID,
      branchReason,
      assistantOnlyBranch,
      signal,
    }: {
      token: string;
      conversationID: string;
      submitTask: ChatSubmitTask;
      exchangeKey: string;
      clientRunID: string;
      content: string;
      options: ConversationOptions;
      effectiveAttachments: PendingAttachment[];
      platformModelName: string;
      selectedToolIDs: number[];
      selectedSkills: SkillSummaryDTO[];
      selectedKnowledgeBaseIDs: string[];
      htmlVisualPromptEnabled: boolean;
      parentMessagePublicID: string | null;
      sourceMessagePublicID: string | null;
      branchReason: "default" | "retry" | "edit";
      assistantOnlyBranch: boolean;
      signal: AbortSignal;
    }): Promise<SendMessageResult> => {
      let terminalStreamError: Extract<StreamMessageEvent, { type: "error" }> | null = null;
      const streamOptions: ConversationStreamOptions = {
        signal,
        onTerminal: () => {
          onConversationRunFinished?.(clientRunID);
        },
        onInterrupted: (event) => {
          terminalStreamError = event;
        },
        onFileProc: (message) => {
          updatePendingExchange(exchangeKey, (current) => ({
            ...current,
            assistantFileProc: true,
            assistantActivityLabel: message.trim() || t("processingAttachments"),
          }));
        },
        onRagSearch: (message) => {
          updatePendingExchange(exchangeKey, (current) => ({
            ...current,
            assistantFileProc: true,
            assistantActivityLabel: message.trim() || t("retrievingContent"),
          }));
        },
        onMediaStatus: (event) => {
          const activityLabel = resolveMediaStatusLabel(event.status, event.message, event.content_type, t);
          updatePendingExchange(exchangeKey, (current) => ({
            ...current,
            assistantFileProc: true,
            assistantActivityLabel: activityLabel,
          }));
        },
        onMediaImageDelta: (event) => {
          const previewMarkdown = buildMediaImagePreviewMarkdown(event, t("imagePreviewAlt"));
          if (!previewMarkdown) {
            return;
          }
          updatePendingExchange(exchangeKey, (current) => ({
            ...current,
            assistantPending: false,
            assistantStreaming: true,
            assistantFileProc: false,
            assistantActivityLabel: undefined,
            assistantText: previewMarkdown,
          }));
        },
        onCompactDone: (event) => {
          updatePendingExchange(exchangeKey, (current) => ({
            ...current,
            compactDone: {
              method: event.method,
              freed_tokens: event.freed_tokens,
              summary_preview: event.summary_preview,
            },
          }));
        },
        onProcessUpdate: (event) => {
          updatePendingExchange(exchangeKey, (current) => ({
            ...current,
            assistantFileProc: false,
            assistantActivityLabel: undefined,
            assistantProcessTrace: event.trace ? toPendingProcessTrace(event.trace) : current.assistantProcessTrace,
          }));
        },
        onUpstreamThinkDelta: (event) => {
          enqueueUpstreamThinkDelta(exchangeKey, event);
        },
        onDelta: (delta) => {
          // Always clear assistantFileProc so batched React updates cannot keep the file_proc spinner alive.
          updatePendingExchange(exchangeKey, (current) =>
            current.assistantFileProc
              ? { ...current, assistantFileProc: false, assistantActivityLabel: undefined }
              : current,
          );
          enqueueStreamText(exchangeKey, delta);
        },
        onTextSnapshot: (content) => {
          setStreamTextSnapshot(exchangeKey, content);
        },
        onUsage: (event) => {
          updatePendingExchange(exchangeKey, (current) => ({
            ...current,
            assistantInputTokens: event.input_tokens > 0 ? event.input_tokens : current.assistantInputTokens,
            assistantOutputTokens: event.output_tokens > 0 ? event.output_tokens : current.assistantOutputTokens,
            assistantCacheReadTokens:
              event.cache_read_tokens > 0 ? event.cache_read_tokens : current.assistantCacheReadTokens,
            assistantCacheWriteTokens:
              event.cache_write_tokens > 0 ? event.cache_write_tokens : current.assistantCacheWriteTokens,
            assistantReasoningTokens:
              event.reasoning_tokens > 0 ? event.reasoning_tokens : current.assistantReasoningTokens,
          }));
        },
        onModerationChecking: () => {
          updatePendingExchange(exchangeKey, (current) => ({
            ...current,
            assistantFileProc: true,
            assistantActivityLabel: t("moderationChecking"),
          }));
        },
        onModerationBlocked: (event) => {
          const categories = Array.isArray(event.categories) ? event.categories : [];
          updatePendingExchange(exchangeKey, (current) => ({
            ...current,
            assistantPending: false,
            assistantStreaming: false,
            assistantFileProc: false,
            assistantActivityLabel: undefined,
            assistantText: "",
            assistantAttachments: [],
            assistantProcessTrace: undefined,
            assistantStatus: "blocked",
            assistantErrorCode: "content_moderation.blocked",
            assistantErrorMessage: t("moderationBlocked"),
            assistantInlineAlert: {
              title: t("moderationBlocked"),
              message: [
                t("moderationBlockedDescription"),
                event.eventID ? t("moderationEventId", { id: event.eventID }) : "",
                categories.length > 0 ? t("moderationCategories", { categories: categories.join(", ") }) : "",
                event.billedReason ? t("moderationBlockedBilled") : "",
              ]
                .filter(Boolean)
                .join("\n"),
            },
          }));
          toast.error(t("moderationBlocked"), {
            description: t("moderationBlockedDescription"),
          });
        },
      };

      const effectiveOptions = submitTask === "video_extension" ? resolveVideoExtensionOptions(options) : options;
      const commonStreamPayload = {
        model: platformModelName,
        options: Object.keys(effectiveOptions).length > 0 ? effectiveOptions : undefined,
        clientRunID,
        fileIDs: effectiveAttachments.length > 0 ? effectiveAttachments.map((item) => item.fileID) : undefined,
        parentMessagePublicID: parentMessagePublicID || undefined,
        sourceMessagePublicID: sourceMessagePublicID || undefined,
        branchReason,
      };
      let completed: SendMessageResult;
      if (submitTask === "chat") {
        const chatPayload: SendMessageRequest = {
          ...commonStreamPayload,
          contentType: effectiveAttachments.length > 0 ? "mixed" : "text",
          content,
          selectedToolIDs: selectedToolIDs.length > 0 ? selectedToolIDs : undefined,
          skillIDs: selectedSkills.length > 0 ? selectedSkills.map((skill) => skill.id) : undefined,
          knowledgeBaseIDs: selectedKnowledgeBaseIDs,
          htmlVisualPrompt: htmlVisualPromptEnabled || undefined,
        };
        completed = await streamConversationMessage(token, conversationID, chatPayload, streamOptions);
      } else if (submitTask === "video_generation") {
        const mediaPayload: MediaVideoRequest = {
          ...commonStreamPayload,
          prompt: content,
        };
        completed = await streamVideoGeneration(token, conversationID, mediaPayload, streamOptions);
      } else if (submitTask === "video_extension") {
        const sourceVideoFileID = effectiveAttachments[0]?.fileID;
        if (!sourceVideoFileID) {
          throw new Error("video extension source is missing");
        }
        const mediaPayload: MediaVideoExtensionRequest = {
          model: commonStreamPayload.model,
          options: commonStreamPayload.options,
          clientRunID: commonStreamPayload.clientRunID,
          parentMessagePublicID: commonStreamPayload.parentMessagePublicID,
          sourceMessagePublicID: commonStreamPayload.sourceMessagePublicID,
          branchReason: commonStreamPayload.branchReason,
          prompt: content,
          sourceVideoFileID,
        };
        completed = await streamVideoExtension(token, conversationID, mediaPayload, streamOptions);
      } else {
        const mediaPayload: MediaImageRequest = {
          ...commonStreamPayload,
          prompt: content,
        };
        completed =
          submitTask === "image_generation"
            ? await streamImageGeneration(token, conversationID, mediaPayload, streamOptions)
            : await streamImageEdit(token, conversationID, mediaPayload, streamOptions);
      }

      flushStreamTextNow(exchangeKey);
      flushUpstreamThinkNow(exchangeKey);
      resetStreamBuffer(exchangeKey);
      updatePendingExchange(exchangeKey, (current) =>
        settleCompletedExchange(current, completed, {
          assistantOnlyBranch,
          clientRunID,
          terminalStreamError,
          t,
        }),
      );
      return completed;
    },
    [
      enqueueStreamText,
      enqueueUpstreamThinkDelta,
      flushStreamTextNow,
      flushUpstreamThinkNow,
      onConversationRunFinished,
      resetStreamBuffer,
      setStreamTextSnapshot,
      t,
      updatePendingExchange,
    ],
  );

  return { runStream };
}
