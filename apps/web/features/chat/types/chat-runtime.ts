import type {
  ChatInlineAlert,
  ChatMessageProcessTrace,
  ImageLoadingAspectRatio,
  MessageAttachment,
} from "@/features/chat/types/messages";
import type { ConversationOptions } from "@/shared/api/conversation-types";
import type { PublicModelPricingDTO } from "@/shared/api/model-types";
import type { ModelControl, ModelNativeToolConfig, ModelReasoningCapability, ReasoningEffortLevel } from "@/entities/model";

export type ViewerProfile = {
  name: string;
  timeZone: string;
};

export type ChatModelOption = {
  platformModelName: string;
  icon: string;
  vendor: string;
  vendorName: string;
  vendorIcon: string;
  displayGroupID: number | null;
  displayGroupName: string;
  displayGroupIcon: string;
  kinds: string[];
  protocols: string[];
  defaultOptions: ConversationOptions;
  optionControls: ModelOptionControl[];
  lockedOptionPaths: string[];
  nativeToolKeys: string[];
  nativeTools: ModelNativeToolConfig[];
  pricing: PublicModelPricingDTO | null;
  videoExtension: ModelMediaTaskConfig | null;
  // Normalized reasoning effort capability (explicit or inferred by the backend); null when unsupported.
  reasoning: ModelReasoningCapability | null;
  // User-facing controls declared by the administrator (the reasoning switcher is one of them).
  controls: ModelControl[];
  // Input modalities the model accepts (explicit declaration, else models.dev); empty when unknown.
  // Describes the model only: whether a file is sent natively also depends on the route, decided by the server.
  inputModalities: ModelInputModality[];
};

export const MODEL_INPUT_MODALITIES = ["text", "image", "pdf", "audio", "video"] as const;

export type ModelInputModality = (typeof MODEL_INPUT_MODALITIES)[number];

export type ModelMediaTaskConfig = {
  enabled: boolean;
  defaultOptions: ConversationOptions;
  optionControls: ModelOptionControl[];
};

export const MODEL_OPTION_CONTROL_TYPES = ["boolean", "number", "select", "text"] as const;

export type ModelOptionControlType = (typeof MODEL_OPTION_CONTROL_TYPES)[number];

export type ModelOptionControl = {
  path: string;
  label?: string;
  description?: string;
  type?: ModelOptionControlType;
  options?: string[];
  placeholder?: string;
  locked?: boolean;
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
  localFile?: File;
};

export type UploadingAttachment = {
  tempID: string;
  fileName: string;
  sizeBytes: number;
};

export type PendingExchange = {
  key: string;
  conversationScopeKey: string;
  branchScopePath: string[];
  branchScopeRunID: string;
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
  reuseUserMessage: boolean;
  userContent: string;
  userAttachments?: PendingAttachment[];
  userServerMessageID?: number;
  userCreatedAt: string;
  assistantText: string;
  assistantPending: boolean;
  assistantStreaming: boolean;
  assistantStatus?: string;
  assistantErrorCode?: string;
  assistantErrorMessage?: string;
  assistantFileProc?: boolean; // Active file_proc stage.
  assistantActivityLabel?: string;
  assistantImageAspectRatio?: ImageLoadingAspectRatio;
  assistantProcessTrace?: ChatMessageProcessTrace;
  assistantInlineAlert?: ChatInlineAlert;
  assistantServerMessageID?: number;
  assistantCreatedAt: string;
  assistantUpdatedAt?: string;
  assistantContentType?: string;
  assistantAttachments?: MessageAttachment[];
  assistantInputTokens?: number;
  assistantOutputTokens?: number;
  assistantCacheReadTokens?: number;
  assistantCacheWriteTokens?: number;
  assistantReasoningTokens?: number;
  assistantReasoningEffort?: ReasoningEffortLevel | null;
  assistantLatencyMS?: number;
  compactDone?: { method: string; freed_tokens: number; summary_preview: string };
};

export type PendingExchangeMap = Record<string, PendingExchange>;
