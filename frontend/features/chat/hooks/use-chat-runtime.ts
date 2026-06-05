"use client";

import * as React from "react";

import type { PendingAttachment, PendingExchange } from "@/features/chat/types/chat-runtime";
import type { ChatModelOption } from "@/features/chat/types/chat-runtime";
import { useChatBranchState } from "@/features/chat/hooks/use-chat-branch-state";
import { useChatSubmitStream } from "@/features/chat/hooks/use-chat-submit-stream";
import type {
  ConversationDTO,
  ConversationOptions,
  MessageDTO,
} from "@/shared/api/conversation.types";

export function useChatRuntime({
  conversationID,
  resetToken,
  messages,
  activeConversation,
  selectedPlatformModelName,
  modelOptions,
  selectedToolIDs,
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
  resumingRunID = "",
}: {
  conversationID: string | null;
  resetToken: number;
  messages: MessageDTO[];
  activeConversation: ConversationDTO | null;
  selectedPlatformModelName: string;
  modelOptions: ChatModelOption[];
  selectedToolIDs: number[];
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
  resumingRunID?: string;
}) {
  const [showConversationLayout, setShowConversationLayout] = React.useState(false);
  const [pendingExchange, setPendingExchange] = React.useState<PendingExchange | null>(null);
  const previousResetTokenRef = React.useRef(resetToken);
  const liveServerRunIDs = React.useMemo(() => {
    const normalized = resumingRunID.trim();
    return normalized ? new Set([normalized]) : undefined;
  }, [resumingRunID]);

  const branchState = useChatBranchState({
    conversationID,
    resetToken,
    messages,
    pendingExchange,
    liveRunIDs: liveServerRunIDs,
  });

  const submitState = useChatSubmitStream({
    conversationID,
    activeConversation,
    selectedPlatformModelName,
    modelOptions,
    selectedToolIDs,
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
    pendingExchange,
    setPendingExchange,
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
    setPendingExchange(null);
    setShowConversationLayout(false);
  }, [resetToken]);

  const streamingTraceText = React.useMemo(() => {
    const trace = submitState.pendingExchange?.assistantProcessTrace;
    if (!trace) {
      return "";
    }

    const fragments = [
      trace.status,
      trace.upstreamThink?.summary,
      trace.upstreamThink?.contentMarkdown,
      trace.upstreamThink?.updatedAt,
      trace.process?.summary,
      trace.process?.contentMarkdown,
      trace.process?.updatedAt,
      trace.tools?.summary,
      trace.tools?.contentMarkdown,
      trace.tools?.updatedAt,
    ];

    return fragments.filter(Boolean).join("\n");
  }, [submitState.pendingExchange?.assistantProcessTrace]);

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
    sending: submitState.sending,
    showPendingAssistant: branchState.showPendingAssistant,
    streamingText: submitState.streamingText,
    streamingTraceText,
    visibleMessageCount: branchState.visibleMessageCount,
    visibleMessages: branchState.visibleMessages,
    isConversationMode: showConversationLayout || branchState.visibleMessageCount > 0,
  };
}
