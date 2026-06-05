"use client";

import * as React from "react";

import { useChatMessageSubmit } from "@/features/chat/hooks/use-chat-message-submit";
import { useChatStreamBuffer } from "@/features/chat/hooks/use-chat-stream-buffer";
import type { ChatAreaMessage } from "@/features/chat/types/messages";
import type {
  ChatModelOption,
  PendingAttachment,
  PendingExchange,
} from "@/features/chat/types/chat-runtime";
import type {
  ConversationDTO,
  ConversationOptions,
  MessageDTO,
} from "@/shared/api/conversation.types";

export function useChatSubmitStream({
  conversationID,
  resetToken,
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
  setBranchSelections,
  showConversationLayout,
  setShowConversationLayout,
  visibleMessageCount,
  currentLeafMessage,
  visibleMessages,
  combinedMessages,
  serverMessagePublicIDs,
  activeGenerationRunsRef,
}: {
  conversationID: string | null;
  resetToken: number;
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
  activeGenerationRunsRef?: React.RefObject<Set<string>>;
}) {
  const streamBuffer = useChatStreamBuffer({
    setPendingExchange,
  });

  const messageSubmit = useChatMessageSubmit({
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
    setBranchSelections,
    showConversationLayout,
    setShowConversationLayout,
    visibleMessageCount,
    currentLeafMessage,
    visibleMessages,
    combinedMessages,
    serverMessagePublicIDs,
    enqueueStreamText: streamBuffer.enqueueStreamText,
    flushStreamTextNow: streamBuffer.flushStreamTextNow,
    resetStreamBuffer: streamBuffer.resetStreamBuffer,
    startStream: streamBuffer.startStream,
    resetToken,
    activeGenerationRunsRef,
  });

  return {
    ...messageSubmit,
    pendingExchange,
    streamingText: pendingExchange?.assistantText ?? "",
  };
}
