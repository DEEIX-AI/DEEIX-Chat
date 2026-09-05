"use client";

import { useTranslations } from "next-intl";
import * as React from "react";
import { toast } from "sonner";
import { useChatExchangeSync } from "@/features/chat/hooks/use-chat-exchange-sync";
import { useChatHiddenRuns } from "@/features/chat/hooks/use-chat-hidden-runs";
import { useChatMessageActions } from "@/features/chat/hooks/use-chat-message-actions";
import { useChatQueueDispatch } from "@/features/chat/hooks/use-chat-queue-dispatch";
import { useChatRunStream } from "@/features/chat/hooks/use-chat-run-stream";
import { useChatStopMessage } from "@/features/chat/hooks/use-chat-stop-message";
import { useChatSubmissionQueue } from "@/features/chat/hooks/use-chat-submission-queue";
import { toBranchKey } from "@/features/chat/model/chat-thread";
import {
  conversationTitleFromFirstUserMessage,
  isPlaceholderConversationTitle,
  refreshGeneratedConversationMetadata,
  shouldPollGeneratedConversationMetadata,
} from "@/features/chat/model/conversation-metadata-refresh";
import { sanitizeConversationOptions } from "@/features/chat/model/conversation-options";
import {
  resolveDefaultSubmissionParentMessage,
  resolvePersistedPublicID,
} from "@/features/chat/model/message-submit";
import {
  type ActiveStream,
  type BranchScope,
  branchRunIsVisible,
  branchScopeID,
  branchScopeIsVisible,
  branchScopesEqual,
  buildBranchScopePath,
  clearCancelSettlementTimer,
  createClientRunID,
  findLastVisibleActiveStream,
  findVisibleActiveStreamByRunID,
  MAX_CONCURRENT_RUNS,
  type QueuedChatSubmission,
  replaceCompletedBranchSelection,
} from "@/features/chat/model/message-submit-branching";
import {
  abortPendingExchange,
  createInitialPendingExchange,
  failPendingExchange,
} from "@/features/chat/model/message-submit-exchange";
import { resolveSubmitBlockDescription } from "@/features/chat/model/message-submit-media";
import { planChatSubmission } from "@/features/chat/model/message-submit-plan";
import type {
  ChatModelOption,
  PendingAttachment,
  PendingExchange,
  PendingExchangeMap,
} from "@/features/chat/types/chat-runtime";
import type { ChatAreaMessage } from "@/features/chat/types/messages";
import {
  resolveErrorDetails,
  resolveErrorMessage,
  resolveErrorSummary,
} from "@/features/chat/utils/chat-runtime";
import { getConversation } from "@/shared/api/conversation";
import type {
  ConversationDTO,
  ConversationOptions,
  MessageDTO,
  SendMessageResult,
  StreamMessageEvent,
} from "@/shared/api/conversation.types";
import { ApiError } from "@/shared/api/http-client";
import type { SkillSummaryDTO } from "@/shared/api/skills.types";
import { resolveAccessToken } from "@/shared/auth/resolve-access-token";
import { notifyResponseCompletion } from "@/shared/lib/browser-notifications";

export function useChatMessageSubmit({
  conversationID,
  conversationScopeKey,
  activeConversation,
  selectedPlatformModelName,
  modelOptions,
  selectedToolIDs,
  selectedSkills,
  selectedKnowledgeBaseIDs,
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
  onConversationForked,
  touchByPublicID,
  reload,
  replaceMessage,
  setDraft,
  setAttachments,
  releaseAttachments,
  transferAttachments,
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
  setStreamTextSnapshot,
  startStream,
  activeGenerationRunsRef,
  activeGenerationRunsRevision,
  onActiveGenerationRunsChange,
  onConversationRunDetached,
  onConversationRunFinished,
  onConversationRunStarted,
  resumeGenerationActive = false,
}: {
  conversationID: string | null;
  conversationScopeKey: string;
  activeConversation: ConversationDTO | null;
  selectedPlatformModelName: string;
  modelOptions: ChatModelOption[];
  selectedToolIDs: number[];
  selectedSkills: SkillSummaryDTO[];
  selectedKnowledgeBaseIDs: string[];
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
  onConversationForked?: (conversation: ConversationDTO) => Promise<void> | void;
  touchByPublicID: (publicID: string, patch: Partial<ConversationDTO>) => void;
  reload: () => void;
  replaceMessage: (message: MessageDTO) => void;
  setDraft: React.Dispatch<React.SetStateAction<string>>;
  setAttachments: React.Dispatch<React.SetStateAction<PendingAttachment[]>>;
  releaseAttachments: (items: PendingAttachment[]) => void;
  transferAttachments: (items: PendingAttachment[]) => void;
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
  setStreamTextSnapshot: (exchangeKey: string, content: string) => void;
  startStream: (exchangeKey: string, runID?: string) => void;
  activeGenerationRunsRef?: React.RefObject<Set<string>>;
  activeGenerationRunsRevision: number;
  onActiveGenerationRunsChange?: () => void;
  onConversationRunDetached?: (runID: string) => void;
  onConversationRunFinished?: (runID: string) => void;
  onConversationRunStarted?: (runID: string, conversationPublicID: string) => void;
  resumeGenerationActive?: boolean;
}) {
  const t = useTranslations("chat.submit");
  const activeStreamsRef = React.useRef(new Map<string, ActiveStream>());
  const conversationIDRef = React.useRef(conversationID);
  const conversationScopeKeyRef = React.useRef(conversationScopeKey);
  const activeConversationRef = React.useRef(activeConversation);
  const nextModelRunSequenceRef = React.useRef(new Map<string, number>());
  const latestCompletedModelRunSequenceRef = React.useRef(new Map<string, number>());
  const optimisticMessageCountsRef = React.useRef(new Map<string, number>());
  const {
    queuedSubmissions,
    setQueuedSubmissions,
    queuedSubmissionsRef,
    sendQueuedAfterCurrentRef,
    dispatchingQueuedSubmissionIDsRef,
    settledQueuedSubmissionIDsRef,
    onDeleteQueuedMessage,
    onEditQueuedMessage,
    onGuideQueuedMessage,
  } = useChatSubmissionQueue({ releaseAttachments });
  const isRunActive = React.useCallback((runID: string) => activeStreamsRef.current.has(runID), []);
  const {
    getStatus: getHiddenParentRunStatus,
    revision: hiddenParentRunStatusRevision,
  } = useChatHiddenRuns({
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
    [activeGenerationRunsRevision, conversationScopeKey, visibleBranchScopePath, visibleMessages],
  );

  const syncActiveRuns = React.useCallback(() => {
    onActiveGenerationRunsChange?.();
  }, [onActiveGenerationRunsChange]);

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

  useChatExchangeSync({
    conversationScopeKey,
    pendingExchanges,
    setPendingExchanges,
    serverMessagePublicIDs,
    combinedMessages,
    setBranchSelections,
  });

  const { runStream } = useChatRunStream({
    updatePendingExchange,
    enqueueUpstreamThinkDelta,
    enqueueStreamText,
    flushStreamTextNow,
    flushUpstreamThinkNow,
    resetStreamBuffer,
    setStreamTextSnapshot,
    onConversationRunFinished,
  });

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
      const planResult = planChatSubmission({
        content,
        currentAttachments,
        parentMessagePublicID,
        sourceMessagePublicID,
        branchReason,
        queuedSubmission,
        attachmentFallbackContent: t("attachmentOnlyContent"),
        uploading,
        maxFilesPerMessage,
        modelOptions,
        selectedPlatformModelName,
        options,
        selectedToolIDs,
        selectedSkills,
        selectedKnowledgeBaseIDs,
        htmlVisualPromptEnabled,
        visibleConversationScopeKey: conversationScopeKeyRef.current,
        visibleBranchScopePath: visibleBranchScopePathRef.current,
        visibleMessages: visibleMessagesRef.current,
        combinedMessages,
        activeStreams: Array.from(activeStreamsRef.current.values()),
      });
      if (planResult.attachmentsTruncated) {
        toast(t("attachmentsTruncated"), {
          description: t("attachmentsTruncatedDescription", { count: maxFilesPerMessage }),
        });
      }
      if (!planResult.ok) {
        const { block } = planResult;
        if (block.kind === "concurrent_limit") {
          toast.error(t("concurrentGenerationLimit", { count: MAX_CONCURRENT_RUNS }));
        } else if (block.kind === "media_unsupported") {
          toast.error(t("mediaInputUnsupported"), {
            description: resolveSubmitBlockDescription(block.reason, t),
          });
        } else if (block.kind === "no_model") {
          toast.error(t("noModel"), { description: t("selectModelFirst") });
        }
        return false;
      }
      const { plan } = planResult;
      const {
        payloadContent,
        platformModelName,
        clientRunID,
        exchangeKey,
        shouldFollowSubmittedBranch,
        effectiveAttachments,
        resolvedParentPublicID,
        assistantOnlyBranch,
        tempUserPublicID,
        tempAssistantPublicID,
        pendingUserPublicID,
      } = plan;
      let targetConversationScopeKey = plan.targetConversationScopeKey;
      let targetBranchScope = plan.targetBranchScope;
      const wasConversationMode = showConversationLayout || visibleMessageCount > 0;
      const createdAt = new Date().toISOString();
      let terminalResultReceived = false;
      let shouldKeepConversationLayout = false;
      const streamAbortController = new AbortController();
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
      if (targetConversationID) {
        onConversationRunStarted?.(clientRunID, targetConversationID);
      }
      syncActiveRuns();
      if (resetComposer) {
        setDraft("");
        transferAttachments(currentAttachments);
        setAttachments([]);
      }
      startStream(exchangeKey, clientRunID);
      setPendingExchanges((current) => ({
        ...current,
        [exchangeKey]: createInitialPendingExchange(plan, targetConversationID?.trim() || null, createdAt),
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
          const created = await prependNewConversation(platformModelName);
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
          onConversationRunStarted?.(clientRunID, created.publicID);
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
        modelRunSequence = (nextModelRunSequenceRef.current.get(targetConversationScopeKey) ?? 0) + 1;
        nextModelRunSequenceRef.current.set(targetConversationScopeKey, modelRunSequence);
        const completed = await runStream({
          token,
          conversationID: targetConversationID,
          submitTask: plan.submitTask,
          exchangeKey,
          clientRunID,
          content: payloadContent,
          options: plan.sanitizedOptions,
          effectiveAttachments,
          platformModelName,
          selectedToolIDs: plan.selectedToolIDs,
          selectedSkills: plan.selectedSkills,
          selectedKnowledgeBaseIDs: plan.selectedKnowledgeBaseIDs,
          htmlVisualPromptEnabled: plan.htmlVisualPromptEnabled,
          parentMessagePublicID: resolvedParentPublicID,
          sourceMessagePublicID: plan.resolvedSourcePublicID,
          branchReason: plan.branchReason,
          assistantOnlyBranch,
          signal: streamAbortController.signal,
        });

        terminalResultReceived = true;
        const assistantMessageSucceeded = (completed.assistantMessage.status || "success") === "success";
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
          ...(shouldUpdateConversationModel ? { model: platformModelName } : {}),
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
        releaseAttachments(currentAttachments);
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
          releaseAttachments(currentAttachments);
          updatePendingExchange(exchangeKey, (current) => abortPendingExchange(current, clientRunID));
          return false;
        }
        if (error instanceof ApiError && error.errorCode === "content_moderation.blocked") {
          // UI already updated via onModerationBlocked; settle as a soft block with retry.
          shouldKeepConversationLayout = true;
          releaseAttachments(currentAttachments);
          if (conversationScopeKeyRef.current === targetConversationScopeKey) {
            reload();
          }
          return false;
        }
        const errorMessage = resolveErrorMessage(error, t("retryLater"));
        const errorDetails = resolveErrorDetails(error);
        const errorSummary = resolveErrorSummary(error, t("retryLater"));
        shouldKeepConversationLayout = true;
        const shouldRestoreAttachments =
          resetComposer &&
          restoreDraftOnFailure &&
          branchRunIsVisible(
            targetBranchScope,
            clientRunID,
            conversationScopeKeyRef.current,
            visibleBranchScopePathRef.current,
            visibleMessagesRef.current,
          );
        if (shouldRestoreAttachments) {
          setDraft(content);
          setAttachments(currentAttachments);
        } else {
          releaseAttachments(currentAttachments);
        }
        updatePendingExchange(exchangeKey, (current) =>
          failPendingExchange(current, {
            clientRunID,
            title: t("generationInterrupted"),
            errorMessage,
            errorDetails,
          }),
        );
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
        if (terminalResultReceived) {
          // A resolved stream already has an authoritative terminal result.
          // Settle locally as a fallback even if the final SSE callback was
          // missed; only uncertain disconnects should remain detached.
          onConversationRunFinished?.(clientRunID);
        } else {
          onConversationRunDetached?.(clientRunID);
        }
        if (
          branchRunIsVisible(
            targetBranchScope,
            clientRunID,
            conversationScopeKeyRef.current,
            visibleBranchScopePathRef.current,
            visibleMessagesRef.current,
          ) &&
          !terminalResultReceived &&
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
      combinedMessages,
      flushStreamTextNow,
      flushUpstreamThinkNow,
      htmlVisualPromptEnabled,
      maxFilesPerMessage,
      modelOptions,
      onConversationCreated,
      onConversationRunDetached,
      onConversationRunFinished,
      onConversationRunStarted,
      options,
      prependNewConversation,
      releaseAttachments,
      reload,
      resetStreamBuffer,
      restoreDraftOnFailure,
      runStream,
      selectedKnowledgeBaseIDs,
      selectedPlatformModelName,
      selectedSkills,
      selectedToolIDs,
      sendQueuedAfterCurrentRef,
      setAttachments,
      setBranchSelections,
      setDraft,
      setPendingExchanges,
      setQueuedSubmissions,
      setShowConversationLayout,
      showConversationLayout,
      startStream,
      syncActiveRuns,
      t,
      touchByPublicID,
      transferAttachments,
      updatePendingExchange,
      uploading,
      visibleMessageCount,
      queuedSubmissionsRef,
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
    const visibleActive =
      findVisibleActiveStreamByRunID(
        activeStreamsRef.current,
        visibleRunID,
        targetConversationScopeKey,
        currentBranchScopePath,
        visibleMessagesRef.current,
      ) ??
      findLastVisibleActiveStream(
        activeStreamsRef.current,
        targetConversationScopeKey,
        currentBranchScopePath,
        visibleMessagesRef.current,
      );
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
          selectedKnowledgeBaseIDs: selectedKnowledgeBaseIDs.slice(),
          htmlVisualPromptEnabled,
        },
      ];
    });
    setDraft("");
    transferAttachments(currentAttachments);
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
    selectedKnowledgeBaseIDs,
    selectedToolIDs,
    setAttachments,
    setDraft,
    transferAttachments,
    uploading,
    visibleMessages,
    setQueuedSubmissions,
  ]);

  const onStopMessage = useChatStopMessage({
    activeStreamsRef,
    currentLeafMessage,
    conversationScopeKeyRef,
    visibleBranchScopePathRef,
    visibleMessagesRef,
    reload,
  });

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

  useChatQueueDispatch({
    queuedSubmissions,
    setQueuedSubmissions,
    sendQueuedAfterCurrentRef,
    dispatchingQueuedSubmissionIDsRef,
    settledQueuedSubmissionIDsRef,
    activeStreamsRef,
    getPendingExchanges,
    pendingExchanges,
    combinedMessages,
    visibleMessages,
    visibleBranchScopePath,
    conversationScopeKey,
    currentLeafMessage,
    getHiddenParentRunStatus,
    hiddenParentRunStatusRevision,
    resumeGenerationActive,
    activeGenerationRunsRevision,
    releaseAttachments,
    submitMessage,
  });

  const {
    onRetryUserMessage,
    onRetryAssistantMessage,
    onContinueAssistantMessage,
    onEditUserMessage,
    onEditAssistantMessage,
    onForkMessage,
    onCycleMessageBranch,
  } = useChatMessageActions({
    submitMessage,
    combinedMessages,
    replaceMessage,
    onConversationForked,
    conversationIDRef,
    setBranchSelections,
  });

  return {
    onCycleMessageBranch,
    onEditAssistantMessage,
    onEditUserMessage,
    onContinueAssistantMessage,
    onForkMessage,
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
