"use client";

import * as React from "react";

import { useChatMessageSubmit } from "@/features/chat/hooks/use-chat-message-submit";
import { useChatStreamBuffer } from "@/features/chat/hooks/use-chat-stream-buffer";
import type {
  ChatModelOption,
  PendingAttachment,
  PendingExchangeMap,
} from "@/features/chat/types/chat-runtime";
import type { ChatAreaMessage } from "@/features/chat/types/messages";
import type {
  ConversationDTO,
  ConversationOptions,
  MessageDTO,
} from "@/shared/api/conversation.types";
import type { SkillSummaryDTO } from "@/shared/api/skills.types";

export function useChatSubmitStream({
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
  activeGenerationRunsRef,
  activeGenerationRunsRevision,
  onActiveGenerationRunsChange,
  onConversationRunDetached,
  onConversationRunFinished,
  onConversationRunStarted,
  resumeGenerationActive,
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
  activeGenerationRunsRef?: React.RefObject<Set<string>>;
  activeGenerationRunsRevision: number;
  onActiveGenerationRunsChange?: () => void;
  onConversationRunDetached?: (runID: string) => void;
  onConversationRunFinished?: (runID: string) => void;
  onConversationRunStarted?: (runID: string, conversationPublicID: string) => void;
  resumeGenerationActive?: boolean;
}) {
  const streamBuffer = useChatStreamBuffer({
    setPendingExchanges,
  });

  const messageSubmit = useChatMessageSubmit({
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
    enqueueUpstreamThinkDelta: streamBuffer.enqueueUpstreamThinkDelta,
    enqueueStreamText: streamBuffer.enqueueStreamText,
    flushStreamTextNow: streamBuffer.flushStreamTextNow,
    flushUpstreamThinkNow: streamBuffer.flushUpstreamThinkNow,
    resetStreamBuffer: streamBuffer.resetStreamBuffer,
    setStreamTextSnapshot: streamBuffer.setStreamTextSnapshot,
    startStream: streamBuffer.startStream,
    activeGenerationRunsRef,
    activeGenerationRunsRevision,
    onActiveGenerationRunsChange,
    onConversationRunDetached,
    onConversationRunFinished,
    onConversationRunStarted,
    resumeGenerationActive,
  });

  return messageSubmit;
}
