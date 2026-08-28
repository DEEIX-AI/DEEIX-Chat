"use client";

import * as React from "react";
import { useChatBranchState } from "@/features/chat/hooks/use-chat-branch-state";
import { useChatSubmitStream } from "@/features/chat/hooks/use-chat-submit-stream";
import type { ChatModelOption, PendingAttachment, PendingExchangeMap } from "@/features/chat/types/chat-runtime";
import type {
  ConversationDTO,
  ConversationOptions,
  MessageDTO,
} from "@/shared/api/conversation.types";
import type { SkillSummaryDTO } from "@/shared/api/skills.types";

function selectPendingExchangesByScope(
  exchanges: PendingExchangeMap,
  conversationScopeKey: string,
): PendingExchangeMap {
  const scopedEntries = Object.entries(exchanges).filter(
    ([, exchange]) => exchange.conversationScopeKey === conversationScopeKey,
  );
  return Object.fromEntries(scopedEntries);
}

function pendingExchangeMapsEqual(previous: PendingExchangeMap, next: PendingExchangeMap): boolean {
  const previousKeys = Object.keys(previous);
  const nextKeys = Object.keys(next);
  if (previousKeys.length !== nextKeys.length) {
    return false;
  }
  return previousKeys.every((key) => previous[key] === next[key]);
}

function useScopedPendingExchanges(conversationScopeKey: string) {
  const allPendingExchangesRef = React.useRef<PendingExchangeMap>({});
  const conversationScopeKeyRef = React.useRef(conversationScopeKey);
  conversationScopeKeyRef.current = conversationScopeKey;
  const [pendingExchanges, setVisiblePendingExchanges] = React.useState<PendingExchangeMap>({});
  const visiblePendingExchangesRef = React.useRef(pendingExchanges);
  const visiblePendingScopeKeyRef = React.useRef(conversationScopeKey);

  const publishVisiblePendingExchanges = React.useCallback((allPendingExchanges: PendingExchangeMap) => {
    const currentScopeKey = conversationScopeKeyRef.current;
    const nextVisible = selectPendingExchangesByScope(
      allPendingExchanges,
      currentScopeKey,
    );
    if (
      visiblePendingScopeKeyRef.current === currentScopeKey &&
      pendingExchangeMapsEqual(visiblePendingExchangesRef.current, nextVisible)
    ) {
      return;
    }
    visiblePendingScopeKeyRef.current = currentScopeKey;
    visiblePendingExchangesRef.current = nextVisible;
    setVisiblePendingExchanges(nextVisible);
  }, []);

  const setPendingExchanges = React.useCallback<
    React.Dispatch<React.SetStateAction<PendingExchangeMap>>
  >((update) => {
    const current = allPendingExchangesRef.current;
    const next = typeof update === "function" ? update(current) : update;
    if (next === current) {
      return;
    }
    allPendingExchangesRef.current = next;
    publishVisiblePendingExchanges(next);
  }, [publishVisiblePendingExchanges]);

  React.useEffect(() => {
    publishVisiblePendingExchanges(allPendingExchangesRef.current);
  }, [conversationScopeKey, publishVisiblePendingExchanges]);

  const getPendingExchanges = React.useCallback(() => allPendingExchangesRef.current, []);
  const scopedPendingExchanges =
    visiblePendingScopeKeyRef.current === conversationScopeKey
      ? pendingExchanges
      : selectPendingExchangesByScope(allPendingExchangesRef.current, conversationScopeKey);

  return {
    getPendingExchanges,
    pendingExchanges: scopedPendingExchanges,
    setPendingExchanges,
  };
}

export function useChatRuntime({
  conversationID,
  resetToken,
  messages,
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
  activeGenerationRunsRef,
  activeGenerationRunsRevision,
  onActiveGenerationRunsChange,
  onConversationRunDetached,
  onConversationRunFinished,
  onConversationRunStarted,
  resumingRunID = "",
  resumingActivityLabel = "",
}: {
  conversationID: string | null;
  resetToken: number;
  messages: MessageDTO[];
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
  activeGenerationRunsRef?: React.RefObject<Set<string>>;
  activeGenerationRunsRevision: number;
  onActiveGenerationRunsChange?: () => void;
  onConversationRunDetached?: (runID: string) => void;
  onConversationRunFinished?: (runID: string) => void;
  onConversationRunStarted?: (runID: string, conversationPublicID: string) => void;
  resumingRunID?: string;
  resumingActivityLabel?: string;
}) {
  const [showConversationLayout, setShowConversationLayout] = React.useState(false);
  const previousResetTokenRef = React.useRef(resetToken);
  const conversationScopeKey = React.useMemo(
    () => conversationID?.trim() ? `conversation:${conversationID.trim()}` : `draft:${resetToken}`,
    [conversationID, resetToken],
  );
  const { getPendingExchanges, pendingExchanges, setPendingExchanges } =
    useScopedPendingExchanges(conversationScopeKey);
  const liveServerRunIDs = React.useMemo(() => {
    const normalized = resumingRunID.trim();
    return normalized ? new Set([normalized]) : undefined;
  }, [resumingRunID]);
  const liveActivityLabels = React.useMemo(() => {
    const runID = resumingRunID.trim();
    const label = resumingActivityLabel.trim();
    return runID && label ? new Map([[runID, label]]) : undefined;
  }, [resumingActivityLabel, resumingRunID]);

  const branchState = useChatBranchState({
    conversationID,
    conversationScopeKey,
    resetToken,
    messages,
    pendingExchanges,
    liveActivityLabels,
    liveRunIDs: liveServerRunIDs,
  });
  const visibleResumeGenerationActive = React.useMemo(() => {
    const normalizedRunID = resumingRunID.trim();
    if (!normalizedRunID) {
      return false;
    }
    return branchState.visibleMessages.some(
      (message) =>
        message.role === "assistant" &&
        message.runID === normalizedRunID &&
        (message.isPending ||
          message.isStreaming ||
          message.status?.trim().toLowerCase() === "pending"),
    );
  }, [branchState.visibleMessages, resumingRunID]);

  const submitState = useChatSubmitStream({
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
    setBranchSelections: branchState.setBranchSelections,
    showConversationLayout,
    setShowConversationLayout,
    visibleMessageCount: branchState.visibleMessageCount,
    currentLeafMessage: branchState.currentLeafMessage,
    visibleMessages: branchState.visibleMessages,
    combinedMessages: branchState.combinedMessages,
    serverMessagePublicIDs: branchState.serverMessagePublicIDs,
    activeGenerationRunsRef,
    activeGenerationRunsRevision,
    onActiveGenerationRunsChange,
    onConversationRunDetached,
    onConversationRunFinished,
    onConversationRunStarted,
    resumeGenerationActive: visibleResumeGenerationActive,
  });

  React.useEffect(() => {
    if (branchState.visibleMessageCount > 0) {
      setShowConversationLayout(true);
      return;
    }
    if (!conversationID && !submitState.sending) {
      setShowConversationLayout(false);
    }
  }, [branchState.visibleMessageCount, conversationID, submitState.sending]);

  React.useEffect(() => {
    if (previousResetTokenRef.current === resetToken) {
      return;
    }
    previousResetTokenRef.current = resetToken;
    setShowConversationLayout(false);
  }, [resetToken]);

  return {
    currentLeafMessage: branchState.currentLeafMessage,
    onCycleMessageBranch: submitState.onCycleMessageBranch,
    onEditAssistantMessage: submitState.onEditAssistantMessage,
    onEditUserMessage: submitState.onEditUserMessage,
    onForkMessage: submitState.onForkMessage,
    onContinueAssistantMessage: submitState.onContinueAssistantMessage,
    onRetryAssistantMessage: submitState.onRetryAssistantMessage,
    onRetryUserMessage: submitState.onRetryUserMessage,
    onSendMessage: submitState.onSendMessage,
    onStopMessage: submitState.onStopMessage,
    onDeleteQueuedMessage: submitState.onDeleteQueuedMessage,
    onEditQueuedMessage: submitState.onEditQueuedMessage,
    onGuideQueuedMessage: submitState.onGuideQueuedMessage,
    queuedMessages: submitState.queuedMessages,
    sending: submitState.sending || visibleResumeGenerationActive,
    visibleMessageCount: branchState.visibleMessageCount,
    visibleMessages: branchState.visibleMessages,
    isConversationMode: showConversationLayout || branchState.visibleMessageCount > 0,
  };
}
