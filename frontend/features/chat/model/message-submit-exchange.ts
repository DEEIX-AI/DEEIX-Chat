import type { useTranslations } from "next-intl";
import { parseAttachments } from "@/features/chat/model/chat-thread";
import {
  resolveAssistantInputSideUsageValue,
  toPendingProcessTrace,
} from "@/features/chat/model/message-submit";
import { streamEventErrorToApiError } from "@/features/chat/model/message-submit-media";
import type { ChatSubmissionPlan } from "@/features/chat/model/message-submit-plan";
import {
  preserveRicherLiveUpstreamThinkTrace,
  readLiveUpstreamThinkTrace,
} from "@/features/chat/model/upstream-think-store";
import type { PendingExchange, PendingExchangeMap } from "@/features/chat/types/chat-runtime";
import type { ChatAreaMessage } from "@/features/chat/types/messages";
import { resolveErrorMessage } from "@/features/chat/utils/chat-runtime";
import type {
  SendMessageResult,
  StreamMessageEvent,
  UpstreamDebugInfo,
} from "@/shared/api/conversation.types";
import { ApiError } from "@/shared/api/http-client";

export function createInitialPendingExchange(
  plan: ChatSubmissionPlan,
  conversationPublicID: string | null,
  createdAt: string,
): PendingExchange {
  return {
    key: plan.exchangeKey,
    ...plan.targetBranchScope,
    conversationPublicID,
    userPublicID: plan.assistantOnlyBranch ? plan.pendingUserPublicID : undefined,
    tempUserPublicID: plan.tempUserPublicID,
    tempAssistantPublicID: plan.tempAssistantPublicID,
    runID: plan.clientRunID,
    platformModelName: plan.platformModelName,
    parentPublicID: plan.pendingParentPublicID,
    sourcePublicID: plan.resolvedSourcePublicID,
    branchReason: plan.branchReason,
    reuseUserMessage: plan.assistantOnlyBranch,
    userContent: plan.payloadContent,
    userAttachments: plan.effectiveAttachments.length > 0 ? plan.effectiveAttachments : undefined,
    userCreatedAt: createdAt,
    assistantText: "",
    assistantPending: true,
    assistantStreaming: true,
    assistantContentType: plan.assistantContentType,
    assistantImageAspectRatio: plan.assistantImageAspectRatio,
    assistantInlineAlert: undefined,
    assistantCreatedAt: createdAt,
    assistantProcessTrace: undefined,
  };
}

export function settleCompletedExchange(
  current: PendingExchange,
  completed: SendMessageResult,
  params: {
    assistantOnlyBranch: boolean;
    clientRunID: string;
    terminalStreamError: Extract<StreamMessageEvent, { type: "error" }> | null;
    t: ReturnType<typeof useTranslations>;
  },
): PendingExchange {
  const { assistantOnlyBranch, clientRunID, terminalStreamError, t } = params;
  const assistantMessageStatus = completed.assistantMessage.status || "success";
  const streamedText = current.assistantText;
  const assistantMessageBlocked =
    assistantMessageStatus.trim().toLowerCase() === "blocked" ||
    completed.assistantMessage.errorCode === "content_moderation.blocked";
  const terminalErrorMessage = terminalStreamError
    ? resolveErrorMessage(
        streamEventErrorToApiError(terminalStreamError, t("retryLater")),
        terminalStreamError.message || t("retryLater"),
      )
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
    ...current,
    userPublicID: completed.userMessage.publicID,
    assistantPublicID: completed.assistantMessage.publicID,
    platformModelName: completed.assistantMessage.platformModelName?.trim() || current.platformModelName,
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
    assistantContentType: completed.assistantMessage.contentType || current.assistantContentType,
    assistantAttachments: parseAttachments(completed.assistantMessage.attachments),
    assistantInputTokens: resolveAssistantInputSideUsageValue(
      assistantOnlyBranch,
      completed.assistantMessage.inputTokens,
      completed.userMessage.inputTokens,
      current.assistantInputTokens,
    ),
    assistantOutputTokens: completed.assistantMessage.outputTokens,
    assistantCacheReadTokens: resolveAssistantInputSideUsageValue(
      assistantOnlyBranch,
      completed.assistantMessage.cacheReadTokens,
      completed.userMessage.cacheReadTokens,
      current.assistantCacheReadTokens,
    ),
    assistantCacheWriteTokens: resolveAssistantInputSideUsageValue(
      assistantOnlyBranch,
      completed.assistantMessage.cacheWriteTokens,
      completed.userMessage.cacheWriteTokens,
      current.assistantCacheWriteTokens,
    ),
    assistantReasoningTokens: completed.assistantMessage.reasoningTokens,
    assistantLatencyMS: completed.assistantMessage.latencyMS,
    assistantProcessTrace:
      assistantMessageStatus === "interrupted"
        ? preserveRicherLiveUpstreamThinkTrace(
            toPendingProcessTrace(completed.assistantMessage.processTrace),
            readLiveUpstreamThinkTrace(clientRunID),
          )
        : toPendingProcessTrace(completed.assistantMessage.processTrace),
    assistantStatus: assistantMessageStatus,
    assistantErrorCode: completed.assistantMessage.errorCode,
    assistantErrorMessage: completed.assistantMessage.errorMessage,
    assistantInlineAlert: assistantMessageBlocked
      ? current.assistantInlineAlert ?? {
          title: t("moderationBlocked"),
          message: t("moderationBlockedDescription"),
        }
      : completed.assistantMessage.status === "error" || completed.assistantMessage.status === "interrupted"
        ? {
            title: t("generationInterrupted"),
            message: terminalErrorMessage || completedErrorMessage || t("retryLater"),
            details: terminalStreamError?.debug,
          }
        : undefined,
    assistantText: assistantMessageBlocked
      ? ""
      : streamedText === completed.assistantMessage.content
        ? current.assistantText
        : completed.assistantMessage.content,
  };
}

export function abortPendingExchange(current: PendingExchange, clientRunID: string): PendingExchange {
  return {
    ...current,
    assistantPending: false,
    assistantStreaming: false,
    assistantFileProc: false,
    assistantActivityLabel: undefined,
    assistantProcessTrace: readLiveUpstreamThinkTrace(clientRunID) ?? current.assistantProcessTrace,
    assistantInlineAlert: undefined,
  };
}

export function failPendingExchange(
  current: PendingExchange,
  params: {
    clientRunID: string;
    title: string;
    errorMessage: string;
    errorDetails: UpstreamDebugInfo | undefined;
  },
): PendingExchange {
  return {
    ...current,
    assistantPending: false,
    assistantStreaming: false,
    assistantFileProc: false,
    assistantActivityLabel: undefined,
    assistantProcessTrace: readLiveUpstreamThinkTrace(params.clientRunID) ?? current.assistantProcessTrace,
    assistantStatus: "error",
    assistantErrorMessage: params.errorMessage,
    assistantInlineAlert: {
      title: params.title,
      message: params.errorMessage,
      details: params.errorDetails,
    },
  };
}

export function collectSettledExchanges(
  pendingExchanges: PendingExchangeMap,
  serverMessagePublicIDs: Set<string>,
  combinedMessages: ChatAreaMessage[],
): {
  completedKeys: string[];
  completedBranches: Array<{
    exchange: PendingExchange;
    userPublicID: string;
    assistantPublicID: string;
  }>;
} {
  const completedKeys: string[] = [];
  const completedBranches: Array<{
    exchange: PendingExchange;
    userPublicID: string;
    assistantPublicID: string;
  }> = [];
  for (const [exchangeKey, exchange] of Object.entries(pendingExchanges)) {
    const userPublicID = exchange.userPublicID || exchange.tempUserPublicID;
    const assistantPublicID = exchange.assistantPublicID || exchange.tempAssistantPublicID;
    if (serverMessagePublicIDs.has(userPublicID) && serverMessagePublicIDs.has(assistantPublicID)) {
      completedKeys.push(exchangeKey);
      continue;
    }
    if (exchange.assistantPending || !exchange.runID?.trim()) {
      continue;
    }
    const serverAssistant = combinedMessages.find(
      (item) =>
        item.role === "assistant" &&
        item.runID === exchange.runID &&
        serverMessagePublicIDs.has(item.publicID) &&
        !item.isPending &&
        !item.isStreaming &&
        item.status !== "pending",
    );
    if (!serverAssistant?.parentPublicID) {
      continue;
    }
    completedKeys.push(exchangeKey);
    completedBranches.push({
      exchange,
      userPublicID: serverAssistant.parentPublicID,
      assistantPublicID: serverAssistant.publicID,
    });
  }
  return { completedKeys, completedBranches };
}
