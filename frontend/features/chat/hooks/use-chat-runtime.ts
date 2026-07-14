"use client";

import * as React from "react";

import type { PendingAttachment, PendingExchangeMap } from "@/features/chat/types/chat-runtime";
import type { ChatModelOption } from "@/features/chat/types/chat-runtime";
import { useChatBranchState } from "@/features/chat/hooks/use-chat-branch-state";
import { useChatSubmitStream } from "@/features/chat/hooks/use-chat-submit-stream";
import type {
  ConversationDTO,
  ConversationOptions,
  MessageDTO,
} from "@/shared/api/conversation.types";
import type { SkillSummaryDTO } from "@/shared/api/skills.types";

export function useChatRuntime({
  conversationID,
  resetToken,
  messages,
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
  releaseAttachments,
  activeGenerationRunsRef,
  failedGenerationRunsRef,
  resumingRunID = "",
}: {
  conversationID: string | null;
  resetToken: number;
  messages: MessageDTO[];
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
  releaseAttachments: (items: PendingAttachment[]) => void;
  activeGenerationRunsRef?: React.RefObject<Set<string>>;
  failedGenerationRunsRef?: React.RefObject<Set<string>>;
  resumingRunID?: string;
}) {
  const [showConversationLayout, setShowConversationLayout] = React.useState(false);
  const [pendingExchanges, setPendingExchanges] = React.useState<PendingExchangeMap>({});
  const previousResetTokenRef = React.useRef(resetToken);
  const liveServerRunIDs = React.useMemo(() => {
    const normalized = resumingRunID.trim();
    return normalized ? new Set([normalized]) : undefined;
  }, [resumingRunID]);

  const branchState = useChatBranchState({
    conversationID,
    resetToken,
    messages,
    pendingExchanges,
    liveRunIDs: liveServerRunIDs,
  });

  const submitState = useChatSubmitStream({
    conversationID,
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
    releaseAttachments,
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
    resetToken,
    activeGenerationRunsRef,
    failedGenerationRunsRef,
    resumeGenerationActive: Boolean(resumingRunID),
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
    onContinueAssistantMessage: submitState.onContinueAssistantMessage,
    onRetryAssistantMessage: submitState.onRetryAssistantMessage,
    onRetryUserMessage: submitState.onRetryUserMessage,
    onSendMessage: submitState.onSendMessage,
    onStopMessage: submitState.onStopMessage,
    onDeleteQueuedMessage: submitState.onDeleteQueuedMessage,
    onEditQueuedMessage: submitState.onEditQueuedMessage,
    onGuideQueuedMessage: submitState.onGuideQueuedMessage,
    queuedMessages: submitState.queuedMessages,
    sending: submitState.sending,
    visibleMessageCount: branchState.visibleMessageCount,
    visibleMessages: branchState.visibleMessages,
    isConversationMode: showConversationLayout || branchState.visibleMessageCount > 0,
  };
}
