"use client";

import * as React from "react";
import { useTranslations } from "next-intl";
import { toast } from "sonner";

import type { ChatAreaMessage, ImageLoadingAspectRatio } from "@/features/chat/types/messages";
import type {
  ChatModelOption,
  PendingAttachment,
  PendingExchange,
  PendingExchangeMap,
} from "@/features/chat/types/chat-runtime";
import type { ChatSubmitBlockReason } from "@/features/chat/model/chat-task";
import { resolveChatSubmitDecision } from "@/features/chat/model/chat-task";
import { useHiddenQueuedParentRuns } from "@/features/chat/hooks/use-hidden-queued-parent-runs";
import {
  resolveDefaultSubmissionParentMessage,
  resolvePersistedPublicID,
  toPendingAttachments,
  toPendingProcessTrace,
} from "@/features/chat/model/message-submit";
import {
  preserveRicherLiveUpstreamThinkTrace,
  readLiveUpstreamThinkTrace,
} from "@/features/chat/model/upstream-think-store";
import {
  resolveErrorDetails,
  resolveErrorMessage,
  resolveErrorSummary,
} from "@/features/chat/utils/chat-runtime";
import {
  buildChildrenIndex,
  parseAttachments,
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
  streamVideoGeneration,
  updateMessage,
  type ConversationStreamOptions,
} from "@/shared/api/conversation";
import type {
  ConversationDTO,
  ConversationOptions,
  MediaImageRequest,
  MediaVideoRequest,
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
const MAX_CONCURRENT_RUNS = 5;
const GENERATION_CANCEL_SETTLEMENT_TIMEOUT_MS = 25_000;

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
  contentType: string | undefined,
  t: ReturnType<typeof useTranslations>,
): string {
  switch (status.trim()) {
    case "queued":
      if (contentType === "video") {
        return t("mediaStatus.videoQueued");
      }
      return t("mediaStatus.queued");
    case "running":
      if (contentType === "video") {
        return t("mediaStatus.videoRunning");
      }
      return t("mediaStatus.running");
    case "saving_artifact":
      if (contentType === "video") {
        return t("mediaStatus.videoSavingArtifact");
      }
      return t("mediaStatus.savingArtifact");
    default:
      return fallbackMessage.trim() || status.trim();
  }
}

type BranchScope = {
  conversationScopeKey: string;
  branchScopePath: string[];
  branchScopeRunID: string;
};

type ActiveStream = BranchScope & {
  controller: AbortController;
  runID: string;
  accessToken: string | null;
  cancelRequested: boolean;
  cancelSettlementTimer: number | null;
};

function clearCancelSettlementTimer(active: ActiveStream) {
  if (active.cancelSettlementTimer === null) {
    return;
  }
  window.clearTimeout(active.cancelSettlementTimer);
  active.cancelSettlementTimer = null;
}

function replaceCompletedBranchSelection(
  previous: Record<string, string>,
  branch: Pick<
    PendingExchange,
    "parentPublicID" | "tempUserPublicID" | "tempAssistantPublicID" | "reuseUserMessage"
  >,
  userPublicID: string,
  assistantPublicID: string,
): Record<string, string> {
  const next = { ...previous };
  let changed = false;
  const parentKey = toBranchKey(branch.parentPublicID);
  const tempUserPublicID = branch.tempUserPublicID;
  const tempAssistantPublicID = branch.tempAssistantPublicID;

  if (!branch.reuseUserMessage && next[parentKey] === tempUserPublicID) {
    next[parentKey] = userPublicID;
    changed = true;
  }
  if (next[tempUserPublicID] === tempAssistantPublicID) {
    delete next[tempUserPublicID];
    if (!branch.reuseUserMessage && next[parentKey] === userPublicID) {
      next[userPublicID] = assistantPublicID;
    }
    changed = true;
  }
  if (branch.reuseUserMessage && next[toBranchKey(userPublicID)] === tempAssistantPublicID) {
    next[toBranchKey(userPublicID)] = assistantPublicID;
    changed = true;
  }
  return changed ? next : previous;
}

type QueuedChatSubmission = BranchScope & {
  id: string;
  clientRunID: string;
  parentRunID: string | null;
  conversationPublicID: string | null;
  conversation: ConversationDTO | null;
  parentMessagePublicID: string | null;
  content: string;
  attachments: PendingAttachment[];
  platformModelName: string;
  options: ConversationOptions;
  selectedToolIDs: number[];
  selectedSkills: SkillSummaryDTO[];
  htmlVisualPromptEnabled: boolean;
};

function buildBranchScopePath(messages: ChatAreaMessage[]): string[] {
  return messages.map((message) => message.publicID.trim()).filter(Boolean);
}

function buildSubmissionBranchScopePath(
  messages: ChatAreaMessage[],
  parentMessagePublicID: string | null | undefined,
): string[] {
  const visiblePath = buildBranchScopePath(messages);
  const parentPublicID = parentMessagePublicID?.trim() || "";
  if (!parentPublicID) {
    return [];
  }
  const parentIndex = visiblePath.indexOf(parentPublicID);
  return parentIndex >= 0 ? visiblePath.slice(0, parentIndex + 1) : visiblePath;
}

function branchScopePathsEqual(left: readonly string[], right: readonly string[]): boolean {
  return left.length === right.length && left.every((publicID, index) => publicID === right[index]);
}

function branchScopesEqual(left: BranchScope, right: BranchScope): boolean {
  return (
    left.conversationScopeKey === right.conversationScopeKey &&
    left.branchScopeRunID === right.branchScopeRunID &&
    branchScopePathsEqual(left.branchScopePath, right.branchScopePath)
  );
}

function branchScopeID(scope: BranchScope): string {
  return JSON.stringify([
    scope.conversationScopeKey,
    scope.branchScopeRunID,
    ...scope.branchScopePath,
  ]);
}

function isSuccessfulBranchParentStatus(status: string | null | undefined): boolean {
  const normalized = status?.trim().toLowerCase() || "";
  return normalized === "success" || normalized === "interrupted";
}

function branchScopeIsVisible(
  scope: BranchScope,
  visibleConversationScopeKey: string,
  visibleMessages: ChatAreaMessage[],
): boolean {
  return (
    scope.conversationScopeKey === visibleConversationScopeKey &&
    visibleMessages.some((message) => message.runID === scope.branchScopeRunID)
  );
}

function findSuccessfulBranchParentMessage(
  messages: ChatAreaMessage[],
  runID: string | null | undefined,
): ChatAreaMessage | undefined {
  const normalizedRunID = runID?.trim() || "";
  if (!normalizedRunID) {
    return undefined;
  }
  return messages.find(
    (message) =>
      message.role === "assistant" &&
      message.runID === normalizedRunID &&
      Boolean(resolvePersistedPublicID(message.publicID)) &&
      !message.isPending &&
      !message.isStreaming &&
      isSuccessfulBranchParentStatus(message.status),
  );
}

function branchRunIsVisible(
  scope: BranchScope,
  runID: string | null | undefined,
  visibleConversationScopeKey: string,
  visibleBranchScopePath: readonly string[],
  visibleMessages: ChatAreaMessage[],
): boolean {
  const normalizedRunID = runID?.trim() || "";
  if (scope.conversationScopeKey !== visibleConversationScopeKey) {
    return false;
  }
  if (normalizedRunID && visibleMessages.some((message) => message.runID === normalizedRunID)) {
    return true;
  }
  return (
    branchScopePathsEqual(scope.branchScopePath, visibleBranchScopePath) &&
    (scope.branchScopeRunID === normalizedRunID ||
      branchScopeIsVisible(scope, visibleConversationScopeKey, visibleMessages))
  );
}

function rechainQueuedSubmissions(
  submissions: QueuedChatSubmission[],
  scope: BranchScope,
  rootParentRunID: string | null,
  rootParentMessagePublicID: string | null,
): QueuedChatSubmission[] {
  let parentRunID = rootParentRunID;
  let firstSubmission = true;
  return submissions.map((submission) => {
    if (!branchScopesEqual(submission, scope)) {
      return submission;
    }
    const parentMessagePublicID = firstSubmission
      ? rootParentMessagePublicID
      : submission.parentMessagePublicID;
    const nextSubmission =
      submission.parentRunID === parentRunID &&
      submission.parentMessagePublicID === parentMessagePublicID
        ? submission
        : { ...submission, parentRunID, parentMessagePublicID };
    parentRunID = submission.clientRunID;
    firstSubmission = false;
    return nextSubmission;
  });
}

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
  return ["new chat", "新对话"].includes(value);
}

function isFallbackConversationTitle(title: string, fallbackTitle: string): boolean {
  const normalizedFallback = fallbackTitle.trim();
  return normalizedFallback !== "" && title.trim() === normalizedFallback;
}

function conversationTitleFromFirstUserMessage(content: string): string {
  const value = content.trim().replace(/\s+/g, " ").replace(/^[\s"'`“”‘’]+|[\s"'`“”‘’]+$/g, "");
  if (!value) {
    return "";
  }
  return Array.from(value).slice(0, 16).join("").trim();
}

function hasPendingGeneratedConversationMetadata(
  item: ConversationDTO | null,
  autoGenerateLabels: boolean,
  fallbackTitle = "",
): boolean {
  return (
    !item ||
    isPlaceholderConversationTitle(item.title) ||
    isFallbackConversationTitle(item.title, fallbackTitle) ||
    (autoGenerateLabels && normalizeLabelsJSON(item.labelsJSON) === "[]")
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
  autoGenerateLabels: boolean,
  fallbackTitle = "",
): boolean {
  if (!hasPendingGeneratedConversationMetadata(item, autoGenerateLabels, fallbackTitle)) {
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
  autoGenerateLabels: boolean,
  fallbackTitle: string,
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
      if (!hasPendingGeneratedConversationMetadata(latest, autoGenerateLabels, fallbackTitle)) {
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
  conversationScopeKey,
  activeConversation,
  selectedPlatformModelName,
  modelOptions,
  selectedToolIDs,
  selectedSkills,
  htmlVisualPromptEnabled,
  options,
  draft,
  attachments,
  maxFilesPerMessage,
  uploading,
  restoreDraftOnFailure,
  autoGenerateLabels,
  prependNewConversation,
  onConversationCreated,
  touchByPublicID,
  reload,
  replaceMessage,
  setDraft,
  setAttachments,
  releaseAttachments,
  getPendingExchanges,
  pendingExchanges,
  setPendingExchanges,
  setBranchSelections,
  showConversationLayout,
  setShowConversationLayout,
  visibleMessageCount,
  currentLeafMessage,
  visibleMessages,
  combinedMessages,
  serverMessagePublicIDs,
  enqueueUpstreamThinkDelta,
  enqueueStreamText,
  flushStreamTextNow,
  flushUpstreamThinkNow,
  resetStreamBuffer,
  startStream,
  activeGenerationRunsRef,
  failedGenerationRunsRef,
  resumeGenerationActive = false,
}: {
  conversationID: string | null;
  conversationScopeKey: string;
  activeConversation: ConversationDTO | null;
  selectedPlatformModelName: string;
  modelOptions: ChatModelOption[];
  selectedToolIDs: number[];
  selectedSkills: SkillSummaryDTO[];
  htmlVisualPromptEnabled: boolean;
  options: ConversationOptions;
  draft: string;
  attachments: PendingAttachment[];
  maxFilesPerMessage: number;
  uploading: boolean;
  restoreDraftOnFailure: boolean;
  autoGenerateLabels: boolean;
  prependNewConversation: (platformModelName: string) => Promise<ConversationDTO | null | undefined>;
  onConversationCreated?: (conversationPublicID: string) => void;
  touchByPublicID: (publicID: string, patch?: Partial<ConversationDTO>) => void;
  reload: () => void;
  replaceMessage: (message: MessageDTO) => void;
  setDraft: React.Dispatch<React.SetStateAction<string>>;
  setAttachments: React.Dispatch<React.SetStateAction<PendingAttachment[]>>;
  releaseAttachments: (items: PendingAttachment[]) => void;
  getPendingExchanges: () => PendingExchangeMap;
  pendingExchanges: PendingExchangeMap;
  setPendingExchanges: React.Dispatch<React.SetStateAction<PendingExchangeMap>>;
  setBranchSelections: React.Dispatch<React.SetStateAction<Record<string, string>>>;
  showConversationLayout: boolean;
  setShowConversationLayout: React.Dispatch<React.SetStateAction<boolean>>;
  visibleMessageCount: number;
  currentLeafMessage: ChatAreaMessage | null;
  visibleMessages: ChatAreaMessage[];
  combinedMessages: ChatAreaMessage[];
  serverMessagePublicIDs: Set<string>;
  enqueueUpstreamThinkDelta: (exchangeKey: string, event: Extract<StreamMessageEvent, { type: "upstream_think_delta" }>) => void;
  enqueueStreamText: (exchangeKey: string, delta: string) => void;
  flushStreamTextNow: (exchangeKey: string) => void;
  flushUpstreamThinkNow: (exchangeKey: string) => void;
  resetStreamBuffer: (exchangeKey?: string) => void;
  startStream: (exchangeKey: string, runID?: string) => void;
  activeGenerationRunsRef?: React.RefObject<Set<string>>;
  failedGenerationRunsRef?: React.RefObject<Set<string>>;
  resumeGenerationActive?: boolean;
}) {
  const t = useTranslations("chat.submit");
  const [activeRunRevision, setActiveRunRevision] = React.useState(0);
  const activeStreamsRef = React.useRef(new Map<string, ActiveStream>());
  const conversationIDRef = React.useRef(conversationID);
  const conversationScopeKeyRef = React.useRef(conversationScopeKey);
  const activeConversationRef = React.useRef(activeConversation);
  const nextModelRunSequenceRef = React.useRef(new Map<string, number>());
  const latestCompletedModelRunSequenceRef = React.useRef(new Map<string, number>());
  const optimisticMessageCountsRef = React.useRef(new Map<string, number>());
  const sendQueuedAfterCurrentRef = React.useRef(new Set<string>());
  const dispatchingQueuedSubmissionIDsRef = React.useRef(new Set<string>());
  const [queuedSubmissions, setQueuedSubmissions] = React.useState<QueuedChatSubmission[]>([]);
  const queuedSubmissionsRef = React.useRef<QueuedChatSubmission[]>([]);
  const isRunActive = React.useCallback((runID: string) => activeStreamsRef.current.has(runID), []);
  const {
    getStatus: getHiddenParentRunStatus,
    revision: hiddenParentRunStatusRevision,
  } = useHiddenQueuedParentRuns({
    currentConversationScopeKey: conversationScopeKey,
    queuedParents: queuedSubmissions,
    getPendingExchanges,
    isRunActive,
  });
  const visibleBranchScopePath = React.useMemo(
    () => buildBranchScopePath(visibleMessages),
    [visibleMessages],
  );
  const visibleBranchScopePathRef = React.useRef(visibleBranchScopePath);
  const visibleMessagesRef = React.useRef(visibleMessages);
  visibleBranchScopePathRef.current = visibleBranchScopePath;
  visibleMessagesRef.current = visibleMessages;
  const sending = React.useMemo(
    () =>
      Array.from(activeStreamsRef.current.values()).some((active) =>
        branchRunIsVisible(
          active,
          active.runID,
          conversationScopeKey,
          visibleBranchScopePath,
          visibleMessages,
        ),
      ),
    [activeRunRevision, conversationScopeKey, visibleBranchScopePath, visibleMessages],
  );

  const syncActiveRuns = React.useCallback(() => {
    setActiveRunRevision((current) => current + 1);
  }, []);

  const updatePendingExchange = React.useCallback(
    (exchangeKey: string, update: (current: PendingExchange) => PendingExchange) => {
      setPendingExchanges((current) => {
        const exchange = current[exchangeKey];
        if (!exchange) {
          return current;
        }
        const nextExchange = update(exchange);
        return nextExchange === exchange ? current : { ...current, [exchangeKey]: nextExchange };
      });
    },
    [setPendingExchanges],
  );

  React.useEffect(() => {
    conversationIDRef.current = conversationID;
  }, [conversationID]);

  React.useEffect(() => {
    conversationScopeKeyRef.current = conversationScopeKey;
  }, [conversationScopeKey]);

  React.useEffect(() => {
    activeConversationRef.current = activeConversation;
  }, [activeConversation]);

  React.useEffect(() => {
    queuedSubmissionsRef.current = queuedSubmissions;
  }, [queuedSubmissions]);

  React.useEffect(() => {
    setPendingExchanges((current) => {
      const completedBackgroundKeys = Object.entries(current)
        .filter(
          ([, exchange]) =>
            exchange.conversationScopeKey !== conversationScopeKey &&
            Boolean(exchange.assistantPublicID) &&
            !exchange.assistantPending &&
            !exchange.assistantStreaming,
        )
        .map(([exchangeKey]) => exchangeKey);
      if (completedBackgroundKeys.length === 0) {
        return current;
      }
      const next = { ...current };
      for (const exchangeKey of completedBackgroundKeys) {
        delete next[exchangeKey];
      }
      return next;
    });
  }, [conversationScopeKey, setPendingExchanges]);

  React.useEffect(() => {
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
    if (completedBranches.length > 0) {
      setBranchSelections((current) =>
        completedBranches.reduce(
          (next, completed) =>
            replaceCompletedBranchSelection(
              next,
              {
                parentPublicID: completed.exchange.parentPublicID,
                tempUserPublicID: completed.exchange.tempUserPublicID,
                tempAssistantPublicID: completed.exchange.tempAssistantPublicID,
                reuseUserMessage: completed.exchange.reuseUserMessage,
              },
              completed.userPublicID,
              completed.assistantPublicID,
            ),
          current,
        ),
      );
    }
    if (completedKeys.length > 0) {
      setPendingExchanges((current) => {
        const next = { ...current };
        for (const key of completedKeys) {
          delete next[key];
        }
        return next;
      });
    }
  }, [
    combinedMessages,
    pendingExchanges,
    serverMessagePublicIDs,
    setBranchSelections,
    setPendingExchanges,
  ]);

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
      let targetConversationScopeKey = queuedSubmission?.conversationScopeKey ?? conversationScopeKeyRef.current;
      const resolvedParentPublicID = resolvePersistedPublicID(parentMessagePublicID);
      const targetBranchScopePath = queuedSubmission?.branchScopePath.slice() ??
        buildSubmissionBranchScopePath(visibleMessagesRef.current, resolvedParentPublicID);
      const clientRunID = queuedSubmission?.clientRunID ?? createClientRunID();
      let targetBranchScope: BranchScope = {
        conversationScopeKey: targetConversationScopeKey,
        branchScopePath: targetBranchScopePath,
        branchScopeRunID: queuedSubmission?.branchScopeRunID ?? clientRunID,
      };
      const shouldFollowSubmittedBranch =
        !queuedSubmission ||
        branchRunIsVisible(
          targetBranchScope,
          clientRunID,
          conversationScopeKeyRef.current,
          visibleBranchScopePathRef.current,
          visibleMessagesRef.current,
        );
      const selectedModel = modelOptions.find((item) => item.platformModelName === requestPlatformModelName) ?? null;
      const resolvedBranchReason = branchReason ?? "default";
      const concurrentBranchRun = resolvedBranchReason === "retry" || resolvedBranchReason === "edit";
      const targetConversationHasActiveStream = Array.from(activeStreamsRef.current.values()).some(
        (active) =>
          queuedSubmission
            ? branchScopesEqual(active, targetBranchScope)
            : active.conversationScopeKey === targetConversationScopeKey &&
              branchScopePathsEqual(active.branchScopePath, targetBranchScopePath),
      );
      if (
        (!content && currentAttachments.length === 0) ||
        (!queuedSubmission && uploading) ||
        (!concurrentBranchRun && targetConversationHasActiveStream)
      ) {
        return false;
      }
      if (activeStreamsRef.current.size >= MAX_CONCURRENT_RUNS) {
        toast.error(t("concurrentGenerationLimit", { count: MAX_CONCURRENT_RUNS }));
        return false;
      }
      if (concurrentBranchRun) {
        const activeRunIDs = new Set(activeStreamsRef.current.keys());
        for (const message of combinedMessages) {
          const runID = message.runID?.trim() || "";
          if (
            message.role === "assistant" &&
            runID &&
            (message.isPending || message.isStreaming || message.status?.trim().toLowerCase() === "pending")
          ) {
            activeRunIDs.add(runID);
          }
        }
        if (activeRunIDs.size >= MAX_CONCURRENT_RUNS) {
          toast.error(t("concurrentGenerationLimit", { count: MAX_CONCURRENT_RUNS }));
          return false;
        }
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
      const sanitizedOptions = sanitizeConversationOptions(requestOptions);
      const submitDecision = resolveChatSubmitDecision(selectedModel, effectiveAttachments, sanitizedOptions);
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
      const exchangeKey = `local-exchange-${clientRunID}`;
      const resolvedSourcePublicID = resolvePersistedPublicID(sourceMessagePublicID);
      const assistantOnlyBranch =
        resolvedBranchReason === "retry" &&
        Boolean(resolvedParentPublicID && resolvedSourcePublicID) &&
        combinedMessages.some((item) => item.publicID === resolvedSourcePublicID && item.role === "assistant");
      const reusedUserMessage = assistantOnlyBranch
        ? combinedMessages.find(
            (item) => item.publicID === resolvedParentPublicID && item.role === "user",
          ) ?? null
        : null;
      const pendingParentPublicID = assistantOnlyBranch
        ? reusedUserMessage?.parentPublicID ?? null
        : resolvedParentPublicID;
      const tempUserPublicID = `${exchangeKey}-user`;
      const tempAssistantPublicID = `${exchangeKey}-assistant`;
      const pendingUserPublicID = assistantOnlyBranch && resolvedParentPublicID ? resolvedParentPublicID : tempUserPublicID;
      const createdAt = new Date().toISOString();
      let sentSuccessfully = false;
      let shouldKeepConversationLayout = false;
      const streamAbortController = new AbortController();
      const assistantImageAspectRatio =
        submitTask === "image_generation" || submitTask === "image_edit"
          ? resolveImageLoadingAspectRatio(sanitizedOptions)
          : undefined;
      const assistantContentType =
        submitTask === "chat" ? "markdown" : submitTask === "video_generation" ? "video" : "image";
      let targetConversationID = queuedSubmission?.conversationPublicID ?? conversationIDRef.current;
      let targetConversation = queuedSubmission?.conversation ?? activeConversationRef.current;
      let metadataRefreshInFlight = false;
      let modelRunSequence = 0;

      activeGenerationRunsRef?.current.add(clientRunID);
      if (shouldFollowSubmittedBranch) {
        setShowConversationLayout(true);
      }
      activeStreamsRef.current.set(clientRunID, {
        controller: streamAbortController,
        runID: clientRunID,
        ...targetBranchScope,
        accessToken: null,
        cancelRequested: false,
        cancelSettlementTimer: null,
      });
      syncActiveRuns();
      if (resetComposer) {
        setDraft("");
        setAttachments([]);
      }
      startStream(exchangeKey, clientRunID);
      setPendingExchanges((current) => ({
        ...current,
        [exchangeKey]: {
          key: exchangeKey,
          ...targetBranchScope,
          conversationPublicID: targetConversationID?.trim() || null,
          userPublicID: assistantOnlyBranch ? pendingUserPublicID : undefined,
          tempUserPublicID,
          tempAssistantPublicID,
          runID: clientRunID,
          platformModelName: requestPlatformModelName,
          parentPublicID: pendingParentPublicID,
          sourcePublicID: resolvedSourcePublicID,
          branchReason: resolvedBranchReason,
          reuseUserMessage: assistantOnlyBranch,
          userContent: payloadContent,
          userAttachments: effectiveAttachments.length > 0 ? effectiveAttachments : undefined,
          userCreatedAt: createdAt,
          assistantText: "",
          assistantPending: true,
          assistantStreaming: true,
          assistantContentType,
          assistantImageAspectRatio,
          assistantInlineAlert: undefined,
          assistantCreatedAt: createdAt,
          assistantProcessTrace: undefined,
        },
      }));
      if (shouldFollowSubmittedBranch) {
        setBranchSelections((prev) => ({
          ...prev,
          ...(assistantOnlyBranch ? {} : { [toBranchKey(resolvedParentPublicID)]: pendingUserPublicID }),
          [pendingUserPublicID]: tempAssistantPublicID,
        }));
      }

      try {
        const token = await resolveAccessToken();
        if (streamAbortController.signal.aborted) {
          throw new DOMException("Aborted", "AbortError");
        }
        if (!token) {
          throw new Error(t("signInRequired"));
        }
        const activeStream = activeStreamsRef.current.get(clientRunID);
        if (activeStream?.controller === streamAbortController) {
          activeStream.accessToken = token;
        }
        let metadataFallbackTitle = "";
        const startMetadataRefresh = (result?: SendMessageResult | null) => {
          if (
            !targetConversationID ||
            metadataRefreshInFlight ||
            !shouldPollGeneratedConversationMetadata(
              targetConversation,
              result,
              autoGenerateLabels,
              metadataFallbackTitle,
            )
          ) {
            return;
          }
          metadataRefreshInFlight = true;
          void refreshGeneratedConversationMetadata(
            token,
            targetConversationID,
            targetConversation,
            autoGenerateLabels,
            metadataFallbackTitle,
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
          const previousTargetBranchScope = targetBranchScope;
          const previousConversationScopeKey = previousTargetBranchScope.conversationScopeKey;
          targetConversationScopeKey = `conversation:${created.publicID}`;
          targetBranchScope = {
            ...previousTargetBranchScope,
            conversationScopeKey: targetConversationScopeKey,
          };
          targetConversationID = created.publicID;
          targetConversation = created;
          const createdActiveStream = activeStreamsRef.current.get(clientRunID);
          if (createdActiveStream) {
            createdActiveStream.conversationScopeKey = targetConversationScopeKey;
          }
          const migratedBranchScopes: BranchScope[] = [
            previousTargetBranchScope,
            ...queuedSubmissionsRef.current
              .filter((item) => item.conversationScopeKey === previousConversationScopeKey)
              .map((item) => item),
          ];
          for (const branchScope of migratedBranchScopes) {
            if (sendQueuedAfterCurrentRef.current.delete(branchScopeID(branchScope))) {
              sendQueuedAfterCurrentRef.current.add(
                branchScopeID({
                  ...branchScope,
                  conversationScopeKey: targetConversationScopeKey,
                }),
              );
            }
          }
          setQueuedSubmissions((current) =>
            current.map((item) =>
              item.conversationScopeKey === previousConversationScopeKey
                ? {
                    ...item,
                    conversationScopeKey: targetConversationScopeKey,
                    conversationPublicID: created.publicID,
                    conversation: created,
                  }
                : item,
            ),
          );
          updatePendingExchange(exchangeKey, (current) => ({
            ...current,
            conversationScopeKey: targetConversationScopeKey,
            conversationPublicID: created.publicID,
          }));
          if (
            branchRunIsVisible(
              previousTargetBranchScope,
              clientRunID,
              conversationScopeKeyRef.current,
              visibleBranchScopePathRef.current,
              visibleMessagesRef.current,
            )
          ) {
            conversationIDRef.current = created.publicID;
            conversationScopeKeyRef.current = targetConversationScopeKey;
            activeConversationRef.current = created;
            // Update the URL without triggering Next.js RSC navigation, which can interrupt an active stream.
            window.history.replaceState(null, "", `/chat?conversation_id=${created.publicID}`);
            onConversationCreated?.(created.publicID);
          }
          syncActiveRuns();
        }
        metadataFallbackTitle = conversationTitleFromFirstUserMessage(payloadContent);
        const optimisticTitle = metadataFallbackTitle;
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
            if (conversationScopeKeyRef.current === targetConversationScopeKey) {
              activeConversationRef.current = targetConversation;
            }
          }
          touchByPublicID(targetConversationID, { title: optimisticTitle });
        }
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
              compactDone: { method: event.method, freed_tokens: event.freed_tokens, summary_preview: event.summary_preview },
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
        };
        modelRunSequence = (nextModelRunSequenceRef.current.get(targetConversationScopeKey) ?? 0) + 1;
        nextModelRunSequenceRef.current.set(targetConversationScopeKey, modelRunSequence);
        let completed: SendMessageResult;
        if (submitTask === "chat") {
          const chatPayload: SendMessageRequest = {
            ...commonStreamPayload,
            contentType: effectiveAttachments.length > 0 ? "mixed" : "text",
            content: payloadContent,
            selectedToolIDs: requestSelectedToolIDs.length > 0 ? requestSelectedToolIDs : undefined,
            skillIDs: requestSelectedSkills.length > 0 ? requestSelectedSkills.map((skill) => skill.id) : undefined,
            htmlVisualPrompt: requestHTMLVisualPromptEnabled || undefined,
          };
          completed = await streamConversationMessage(token, targetConversationID, chatPayload, streamOptions);
        } else if (submitTask === "video_generation") {
          const mediaPayload: MediaVideoRequest = {
            ...commonStreamPayload,
            prompt: payloadContent,
          };
          completed = await streamVideoGeneration(token, targetConversationID, mediaPayload, streamOptions);
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
        flushStreamTextNow(exchangeKey);
        flushUpstreamThinkNow(exchangeKey);
        resetStreamBuffer(exchangeKey);
        const assistantMessageStatus = completed.assistantMessage.status || "success";
        const assistantMessageSucceeded = assistantMessageStatus === "success";
        updatePendingExchange(exchangeKey, (current) => {
          const streamedText = current.assistantText;
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
            assistantInputTokens: resolveInputSideUsageValue(
              completed.assistantMessage.inputTokens,
              completed.userMessage.inputTokens,
              current.assistantInputTokens,
            ),
            assistantOutputTokens: completed.assistantMessage.outputTokens,
            assistantCacheReadTokens: resolveInputSideUsageValue(
              completed.assistantMessage.cacheReadTokens,
              completed.userMessage.cacheReadTokens,
              current.assistantCacheReadTokens,
            ),
            assistantCacheWriteTokens: resolveInputSideUsageValue(
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
                ? current.assistantText
                : completed.assistantMessage.content,
          };
        });
        const completedBranchScope: BranchScope = {
          conversationScopeKey: targetConversationScopeKey,
          branchScopePath: assistantOnlyBranch
            ? [...targetBranchScope.branchScopePath, completed.assistantMessage.publicID]
            : [
                ...targetBranchScope.branchScopePath,
                completed.userMessage.publicID,
                completed.assistantMessage.publicID,
              ],
          branchScopeRunID: clientRunID,
        };
        if (conversationScopeKeyRef.current === targetConversationScopeKey) {
          setBranchSelections((current) =>
            replaceCompletedBranchSelection(
              current,
              {
                parentPublicID: resolvedParentPublicID,
                tempUserPublicID,
                tempAssistantPublicID,
                reuseUserMessage: assistantOnlyBranch,
              },
              completed.userMessage.publicID,
              completed.assistantMessage.publicID,
            ),
          );
        }
        const currentConversation =
          activeConversationRef.current?.publicID === targetConversationID
            ? activeConversationRef.current
            : targetConversation;
        const shouldUpdateConversationModel =
          modelRunSequence > (latestCompletedModelRunSequenceRef.current.get(targetConversationScopeKey) ?? 0);
        if (shouldUpdateConversationModel) {
          latestCompletedModelRunSequenceRef.current.set(targetConversationScopeKey, modelRunSequence);
        }
        const optimisticMessageCount =
          Math.max(
            currentConversation?.messageCount ?? 0,
            optimisticMessageCountsRef.current.get(targetConversationScopeKey) ?? 0,
          ) + (assistantOnlyBranch ? 1 : 2);
        optimisticMessageCountsRef.current.set(targetConversationScopeKey, optimisticMessageCount);
        const conversationPatch: Partial<ConversationDTO> = {
          ...(shouldUpdateConversationModel ? { model: requestPlatformModelName } : {}),
          updatedAt: new Date().toISOString(),
          messageCount: optimisticMessageCount,
        };
        const updatedConversation = currentConversation
          ? { ...currentConversation, ...conversationPatch }
          : null;
        if (updatedConversation && conversationScopeKeyRef.current === targetConversationScopeKey) {
          activeConversationRef.current = updatedConversation;
        }
        if (sendQueuedAfterCurrentRef.current.delete(branchScopeID(targetBranchScope))) {
          sendQueuedAfterCurrentRef.current.add(branchScopeID(completedBranchScope));
        }
        setQueuedSubmissions((current) => {
          if (!current.some((item) => item.conversationScopeKey === targetConversationScopeKey)) {
            return current;
          }
          return current.map((item) => {
            if (item.conversationScopeKey !== targetConversationScopeKey) {
              return item;
            }
            const sameBranch = branchScopesEqual(item, targetBranchScope);
            const isDirectChild = item.parentRunID === clientRunID;
            return {
              ...item,
              ...(updatedConversation ? { conversation: updatedConversation } : {}),
              ...(sameBranch
                ? {
                    branchScopePath: completedBranchScope.branchScopePath,
                    branchScopeRunID: completedBranchScope.branchScopeRunID,
                  }
                : {}),
              ...(isDirectChild
                ? {
                    parentRunID: null,
                    parentMessagePublicID: completed.assistantMessage.publicID,
                  }
                : {}),
            };
          });
        });
        if (conversationScopeKeyRef.current !== targetConversationScopeKey) {
          setPendingExchanges((current) => {
            if (!current[exchangeKey]) {
              return current;
            }
            const next = { ...current };
            delete next[exchangeKey];
            return next;
          });
        }
        touchByPublicID(targetConversationID, conversationPatch);
        if (assistantMessageSucceeded || completed.metadataRefreshHint?.trim() === "pending") {
          startMetadataRefresh(completed);
        }
        releaseAttachments(effectiveAttachments);
        if (assistantMessageSucceeded) {
          notifyResponseCompletion({
            content: completed.assistantMessage.content,
            conversationPublicID: targetConversationID,
            conversationTitle: targetConversation?.title,
          });
        }
        if (conversationScopeKeyRef.current === targetConversationScopeKey) {
          reload();
        }
      } catch (error) {
        flushStreamTextNow(exchangeKey);
        flushUpstreamThinkNow(exchangeKey);
        resetStreamBuffer(exchangeKey);
        if (streamAbortController.signal.aborted) {
          shouldKeepConversationLayout = true;
          releaseAttachments(effectiveAttachments);
          updatePendingExchange(exchangeKey, (current) => ({
            ...current,
            assistantPending: false,
            assistantStreaming: false,
            assistantFileProc: false,
            assistantActivityLabel: undefined,
            assistantProcessTrace: readLiveUpstreamThinkTrace(clientRunID) ?? current.assistantProcessTrace,
            assistantInlineAlert: undefined,
          }));
          return false;
        }
        const errorMessage = resolveErrorMessage(error, t("retryLater"));
        const errorDetails = resolveErrorDetails(error);
        const errorSummary = resolveErrorSummary(error, t("retryLater"));
        failedGenerationRunsRef?.current.add(clientRunID);
        shouldKeepConversationLayout = true;
        if (
          resetComposer &&
          restoreDraftOnFailure &&
          branchRunIsVisible(
            targetBranchScope,
            clientRunID,
            conversationScopeKeyRef.current,
            visibleBranchScopePathRef.current,
            visibleMessagesRef.current,
          )
        ) {
          setDraft(content);
          setAttachments(currentAttachments);
        }
        updatePendingExchange(exchangeKey, (current) => ({
          ...current,
          assistantPending: false,
          assistantStreaming: false,
          assistantFileProc: false,
          assistantActivityLabel: undefined,
          assistantProcessTrace: readLiveUpstreamThinkTrace(clientRunID) ?? current.assistantProcessTrace,
          assistantStatus: "error",
          assistantErrorMessage: errorMessage,
          assistantInlineAlert: {
            title: t("generationInterrupted"),
            message: errorMessage,
            details: errorDetails,
          },
        }));
        toast.error(t("sendFailed"), { description: errorSummary });
        if (targetConversationID) {
          const failedConversationID = targetConversationID;
          void resolveAccessToken()
            .then((latestToken) =>
              latestToken ? getConversation(latestToken, failedConversationID) : null,
            )
            .then((latestConversation) => {
              if (latestConversation) {
                touchByPublicID(failedConversationID, latestConversation);
              }
            })
            .catch(() => {
              // The next conversation list load will reconcile a failed refresh.
            });
        }
        if (targetConversationID && conversationScopeKeyRef.current === targetConversationScopeKey) {
          reload();
        }
        return false;
      } finally {
        const activeStream = activeStreamsRef.current.get(clientRunID);
        if (activeStream?.controller === streamAbortController) {
          clearCancelSettlementTimer(activeStream);
          activeStreamsRef.current.delete(clientRunID);
        }
        activeGenerationRunsRef?.current.delete(clientRunID);
        if (
          branchRunIsVisible(
            targetBranchScope,
            clientRunID,
            conversationScopeKeyRef.current,
            visibleBranchScopePathRef.current,
            visibleMessagesRef.current,
          ) &&
          !sentSuccessfully &&
          !wasConversationMode &&
          !shouldKeepConversationLayout
        ) {
          setShowConversationLayout(false);
        }
        syncActiveRuns();
      }
      return true;
    },
    [
      activeGenerationRunsRef,
      autoGenerateLabels,
      failedGenerationRunsRef,
      enqueueUpstreamThinkDelta,
      enqueueStreamText,
      flushStreamTextNow,
      flushUpstreamThinkNow,
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
      selectedPlatformModelName,
      setAttachments,
      setBranchSelections,
      setDraft,
      setPendingExchanges,
      setShowConversationLayout,
      showConversationLayout,
      startStream,
      touchByPublicID,
      uploading,
      maxFilesPerMessage,
      t,
      syncActiveRuns,
      updatePendingExchange,
      visibleMessageCount,
      combinedMessages,
    ],
  );

  const enqueueSubmission = React.useCallback(() => {
    const content = draft.trim();
    const currentAttachments = attachments.slice();
    if ((!content && currentAttachments.length === 0) || uploading) {
      return false;
    }
    const parentMessagePublicID =
      resolvePersistedPublicID(currentLeafMessage?.publicID) ??
      resolveDefaultSubmissionParentMessage(visibleMessages)?.publicID ??
      null;
    const targetConversationScopeKey = conversationScopeKeyRef.current;
    const targetConversationPublicID = conversationIDRef.current;
    const targetConversation = activeConversationRef.current;
    const currentBranchScopePath = visibleBranchScopePathRef.current;
    const visibleRunID = currentLeafMessage?.runID?.trim() || "";
    const visibleRunPending = Boolean(
      visibleRunID &&
        (currentLeafMessage?.isPending ||
          currentLeafMessage?.isStreaming ||
          currentLeafMessage?.status?.trim().toLowerCase() === "pending"),
    );
    const visibleActiveCandidate = visibleRunID ? activeStreamsRef.current.get(visibleRunID) : undefined;
    const visibleActive =
      visibleActiveCandidate &&
      branchRunIsVisible(
        visibleActiveCandidate,
        visibleActiveCandidate.runID,
        targetConversationScopeKey,
        currentBranchScopePath,
        visibleMessagesRef.current,
      )
        ? visibleActiveCandidate
        : Array.from(activeStreamsRef.current.values())
            .filter((item) =>
              branchRunIsVisible(
                item,
                item.runID,
                targetConversationScopeKey,
                currentBranchScopePath,
                visibleMessagesRef.current,
              ),
            )
            .at(-1);
    const targetBranchScopePath = visibleActive?.branchScopePath.slice() ?? currentBranchScopePath.slice();
    const targetBranchScopeRunID = visibleActive?.branchScopeRunID ?? visibleRunID;
    if (!targetBranchScopeRunID) {
      return false;
    }
    const targetBranchScope: BranchScope = {
      conversationScopeKey: targetConversationScopeKey,
      branchScopePath: targetBranchScopePath,
      branchScopeRunID: targetBranchScopeRunID,
    };
    const clientRunID = createClientRunID();
    setQueuedSubmissions((current) => {
      const previousQueuedSubmission = current
        .filter((item) => branchScopesEqual(item, targetBranchScope))
        .at(-1);
      return [
        ...current,
        {
          id: clientRunID.replace("run_", "queue_"),
          clientRunID,
          parentRunID:
            previousQueuedSubmission?.clientRunID ??
            (visibleRunPending ? visibleRunID : visibleActive?.runID) ??
            null,
          ...targetBranchScope,
          conversationPublicID: targetConversationPublicID,
          conversation: targetConversation,
          parentMessagePublicID,
          content,
          attachments: currentAttachments,
          platformModelName: selectedPlatformModelName,
          options: sanitizeConversationOptions(options),
          selectedToolIDs: selectedToolIDs.slice(),
          selectedSkills: selectedSkills.slice(),
          htmlVisualPromptEnabled,
        },
      ];
    });
    setDraft("");
    setAttachments([]);
    return true;
  }, [
    attachments,
    currentLeafMessage?.publicID,
    currentLeafMessage?.isPending,
    currentLeafMessage?.isStreaming,
    currentLeafMessage?.runID,
    currentLeafMessage?.status,
    draft,
    htmlVisualPromptEnabled,
    options,
    selectedPlatformModelName,
    selectedSkills,
    selectedToolIDs,
    setAttachments,
    setDraft,
    uploading,
    visibleMessages,
  ]);

  const onStopMessage = React.useCallback(() => {
    const visibleRunID = currentLeafMessage?.runID?.trim() || "";
    const visibleRunPending = Boolean(
      visibleRunID &&
        (currentLeafMessage?.isPending ||
          currentLeafMessage?.isStreaming ||
          currentLeafMessage?.status?.trim().toLowerCase() === "pending"),
    );
    const visibleActiveCandidate = visibleRunID ? activeStreamsRef.current.get(visibleRunID) : undefined;
    const visibleActive =
      visibleActiveCandidate &&
      branchRunIsVisible(
        visibleActiveCandidate,
        visibleActiveCandidate.runID,
        conversationScopeKeyRef.current,
        visibleBranchScopePathRef.current,
        visibleMessagesRef.current,
      )
        ? visibleActiveCandidate
        : undefined;
    if (!visibleActive && visibleRunPending) {
      void resolveAccessToken().then(async (token) => {
        if (!token) {
          return;
        }
        await cancelMessageGeneration(token, visibleRunID).catch(() => undefined);
        reload();
      });
      return true;
    }
    const active =
      visibleActive ??
      Array.from(activeStreamsRef.current.values())
        .filter((item) =>
          branchRunIsVisible(
            item,
            item.runID,
            conversationScopeKeyRef.current,
            visibleBranchScopePathRef.current,
            visibleMessagesRef.current,
          ),
        )
        .at(-1);
    if (!active) {
      return false;
    }
    if (active.cancelRequested) {
      return true;
    }
    if (!active.accessToken) {
      active.controller.abort();
      return true;
    }

    active.cancelRequested = true;
    active.cancelSettlementTimer = window.setTimeout(() => {
      if (activeStreamsRef.current.get(active.runID) !== active) {
        return;
      }
      active.controller.abort();
      if (
        branchRunIsVisible(
          active,
          active.runID,
          conversationScopeKeyRef.current,
          visibleBranchScopePathRef.current,
          visibleMessagesRef.current,
        )
      ) {
        reload();
      }
    }, GENERATION_CANCEL_SETTLEMENT_TIMEOUT_MS);

    // Keep the stream connected so its terminal payload can replace optimistic IDs
    // and retain the final partial content/usage produced during cancellation.
    void cancelMessageGeneration(active.accessToken, active.runID).catch(() => {
      if (activeStreamsRef.current.get(active.runID) !== active) {
        return;
      }
      clearCancelSettlementTimer(active);
      active.controller.abort();
      if (
        branchRunIsVisible(
          active,
          active.runID,
          conversationScopeKeyRef.current,
          visibleBranchScopePathRef.current,
          visibleMessagesRef.current,
        )
      ) {
        reload();
      }
    });
    return true;
  }, [
    currentLeafMessage?.isPending,
    currentLeafMessage?.isStreaming,
    currentLeafMessage?.runID,
    currentLeafMessage?.status,
    reload,
  ]);

  const onDeleteQueuedMessage = React.useCallback((id: string) => {
    const target = queuedSubmissionsRef.current.find((item) => item.id === id);
    if (target) {
      releaseAttachments(target.attachments);
    }
    setQueuedSubmissions((current) => {
      const currentTarget = current.find((item) => item.id === id);
      if (!currentTarget) {
        return current;
      }
      const firstScopeSubmission = current.find(
        (item) => branchScopesEqual(item, currentTarget),
      );
      return rechainQueuedSubmissions(
        current.filter((item) => item.id !== id),
        currentTarget,
        firstScopeSubmission?.parentRunID ?? null,
        firstScopeSubmission?.parentMessagePublicID ?? null,
      );
    });
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
      sendQueuedAfterCurrentRef.current.add(branchScopeID(target));
      const firstScopeIndex = current.findIndex(
        (item) => branchScopesEqual(item, target),
      );
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

  const onSendMessage = React.useCallback(async () => {
    if (sending || resumeGenerationActive) {
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
      const hasActiveStream = Array.from(activeStreamsRef.current.values()).some(
        (active) => branchScopesEqual(active, item),
      );
      if (hasActiveStream) {
        return false;
      }
      const isCurrentBranch =
        branchScopeIsVisible(item, conversationScopeKey, visibleMessages);
      if (
        isCurrentBranch &&
        (resumeGenerationActive || currentBranchHasPendingServerGeneration)
      ) {
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
        (exchange) =>
          exchange.runID === item.parentRunID &&
          branchScopesEqual(exchange, item),
      );
      if (resolvePersistedPublicID(parentExchange?.assistantPublicID)) {
        return true;
      }
      const serverParentMessage = findSuccessfulBranchParentMessage(combinedMessages, item.parentRunID);
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
    })
      .finally(() => {
        dispatchingQueuedSubmissionIDsRef.current.delete(queuedSubmission.id);
      });
  }, [
    activeRunRevision,
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
  ]);

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
    queuedMessages: queuedSubmissions
      .filter(
        (item) =>
          branchScopeIsVisible(item, conversationScopeKey, visibleMessages),
      )
      .map((item) => ({
        id: item.id,
        content: item.content,
        attachmentCount: item.attachments.length,
      })),
    sending,
  };
}
