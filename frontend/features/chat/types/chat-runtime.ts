import type {
  ChatInlineAlert,
  ImageLoadingAspectRatio,
  ChatMessageProcessTrace,
} from "@/features/chat/types/messages";
import type { ConversationOptions } from "@/shared/api/conversation.types";
import type { PublicModelPricingDTO } from "@/shared/api/model.types";

export type ViewerProfile = {
  name: string;
  timeZone: string;
};

export type ChatModelOption = {
  platformModelName: string;
  icon: string;
  vendor: string;
  kinds: string[];
  protocols: string[];
  defaultOptions: ConversationOptions;
  pricing: PublicModelPricingDTO | null;
};

export type PendingAttachment = {
  fileID: string;
  fileName: string;
  mimeType: string;
  detectedMime?: string;
  fileCategory?: string;
  sizeBytes: number;
  previewURL?: string;
  processingStatus?: string;
  processingReady?: boolean;
  processingErrorCode?: string;
  processingErrorMessage?: string;
  extractStatus?: string;
  embedStatus?: string;
  ragReady?: boolean;
  ragReason?: string;
  ocrUsed?: boolean;
  ragOptOut?: boolean;
};

export type UploadingAttachment = {
  tempID: string;
  fileName: string;
  sizeBytes: number;
};

export type PendingExchange = {
  key: string;
  conversationPublicID: string | null;
  tempUserPublicID: string;
  tempAssistantPublicID: string;
  userPublicID?: string;
  assistantPublicID?: string;
  runID?: string;
  platformModelName?: string;
  parentPublicID: string | null;
  sourcePublicID: string | null;
  branchReason: "default" | "retry" | "edit";
  userContent: string;
  userAttachments?: PendingAttachment[];
  userServerMessageID?: number;
  userCreatedAt: string;
  assistantText: string;
  assistantPending: boolean;
  assistantStreaming: boolean;
  assistantFileProc?: boolean; // Active file_proc stage.
  assistantActivityLabel?: string;
  assistantImageAspectRatio?: ImageLoadingAspectRatio;
  assistantProcessTrace?: ChatMessageProcessTrace;
  assistantInlineAlert?: ChatInlineAlert;
  assistantServerMessageID?: number;
  assistantCreatedAt: string;
  assistantUpdatedAt?: string;
  assistantContentType?: string;
  assistantInputTokens?: number;
  assistantOutputTokens?: number;
  assistantCacheReadTokens?: number;
  assistantCacheWriteTokens?: number;
  assistantReasoningTokens?: number;
  assistantLatencyMS?: number;
  compactDone?: { method: string; freed_tokens: number; summary_preview: string };
};
