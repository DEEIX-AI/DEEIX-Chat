import type { UserStorageQuotaDTO } from "@/shared/api/file.types";

export type ConversationDTO = {
  userID: number;
  publicID: string;
  projectID: string;
  projectName: string;
  title: string;
  labelsJSON: string;
  model: string;
  provider: string;
  sessionKey: string;
  isStarred: boolean;
  starredAt: string | null;
  messageCount: number;
  status: string;
  contextPolicyJSON: string;
  lastCompactedAt: string | null;
  lastResponseID: string;
  shareStatus: "none" | "active" | "revoked" | "expired" | string;
  shareID: string;
  sharedAt: string | null;
  lastShareAccessedAt: string | null;
  createdAt: string;
  updatedAt: string;
};

export type ConversationStatusFilter = "active" | "archived" | "all";
export type ConversationStarredFilter = "all" | "starred" | "unstarred";
export type ConversationShareFilter = "all" | "shared" | "unshared";
export type ConversationProjectFilter = "all" | "unassigned" | string;
export type ConversationProjectStatusFilter = "active" | "archived" | "all";

export type ConversationProjectDTO = {
  publicID: string;
  name: string;
  description: string;
  systemPrompt: string;
  color: string;
  icon: string;
  sortOrder: number;
  status: string;
  createdAt: string;
  updatedAt: string;
};

export type MessageDTO = {
  id: number;
  conversationID: number;
  userID: number;
  publicID: string;
  parentMessageID: number | null;
  parentPublicID: string;
  runID: string;
  role: string;
  contentType: string;
  content: string;
  branchReason: "default" | "retry" | "edit";
  sourceMessageID: number | null;
  sourcePublicID: string;
  tokenUsage: number;
  inputTokens: number;
  outputTokens: number;
  cacheReadTokens: number;
  cacheWriteTokens: number;
  reasoningTokens: number;
  latencyMS: number;
  status: string;
  errorCode: string;
  errorMessage: string;
  attachments: string;
  platformModelName?: string;
  upstreamModelName?: string;
  modelVendor?: string;
  modelIcon?: string;
  processTrace?: MessageProcessTraceDTO;
  myFeedback: "up" | "down" | "";
  thumbsUpCount: number;
  thumbsDownCount: number;
  billingCost?: MessageBillingCostDTO;
  editedAt: string | null;
  createdAt: string;
  updatedAt: string;
};

export type ConversationRunDTO = {
  id: number;
  runID: string;
  requestID: string;
  userID: number;
  conversationID: number;
  endpoint: string;
  provider: string;
  providerProtocol: string;
  upstreamID: number;
  upstreamModelID: number;
  requestedModelName: string;
  platformModelName: string;
  routedBindingCode: string;
  modelVendor: string;
  modelIcon: string;
  upstreamModelName: string;
  inputTokens: number;
  outputTokens: number;
  cacheReadTokens: number;
  cacheWriteTokens: number;
  reasoningTokens: number;
  toolCallsCount: number;
  firstTokenLatencyMS: number;
  totalLatencyMS: number;
  status: string;
  errorCode: string;
  errorMessage: string;
  startedAt: string;
  endedAt: string | null;
  createdAt: string;
  updatedAt: string;
};

export type ConversationExportDTO = {
  version: number;
  exportScope: string;
  exportedAt: string;
  conversation: ConversationDTO;
  messages: MessageDTO[];
  runs: ConversationRunDTO[];
  totalMessages: number;
  totalRuns: number;
  defaultMessagePublicIDs: string[];
  compatibility: {
    format: string;
    notes: string;
  };
};

export type MessageBillingCostDTO = {
  billingMode: string;
  billedCurrency: string;
  billedNanousd: number;
  billedUSD: number;
  pricingSnapshotJSON: string;
};

export type TraceBlockDTO = {
  title: string;
  summary: string;
  contentMarkdown: string;
  status: string;
  stage?: string;
  roundID?: string;
  parentEventID?: string;
  updatedAt: string;
  payloadJSON?: string;
};

export type PromptTraceBlockDTO = {
  kind: string;
  title: string;
  tokenEstimate: number;
  cacheable: boolean;
  sourceCount: number;
  sourceRefs?: PromptTraceSourceDTO[];
};

export type PromptTraceSourceDTO = {
  sourceType: string;
  sourceID: string;
  title: string;
  artifactID?: number;
};

export type ContextArtifactDTO = {
  id: number;
  messageID: number;
  runID: string;
  kind: string;
  sourceType: string;
  sourceID: string;
  sourceTitle: string;
  content: string;
  tokenEstimate: number;
  score: number;
  metadataJSON: string;
  expiresAt?: string | null;
  createdAt: string;
};

export type PromptTraceDTO = {
  mode: string;
  promptFingerprint: string;
  statefulUsed: boolean;
  statefulDisabledReason: string;
  totalTokenEstimate: number;
  sentTokenEstimate: number;
  fullMessageCount: number;
  sentMessageCount: number;
  statefulSavedMessages: number;
  statefulSavedTokens: number;
  blocks: PromptTraceBlockDTO[];
};

export type ReasoningDeltaDTO = {
  event_type: string;
  item_id?: string;
  status?: string;
  kind: "summary_text" | "content_text" | "signature";
  signature?: string;
  encrypted_content?: string;
};

export type MessageProcessTraceDTO = {
  enabled: boolean;
  status: string;
  process?: TraceBlockDTO;
  tools?: TraceBlockDTO;
  upstreamThink?: TraceBlockDTO;
  promptTrace?: PromptTraceDTO;
  events?: TraceEventDTO[];
};

export type TraceEventDTO = {
  eventID: string;
  eventType: string;
  phase: string;
  stage?: string;
  roundID?: string;
  parentEventID?: string;
  title: string;
  summary: string;
  contentMarkdown: string;
  status: string;
  seq: number;
  startedAt: string;
  endedAt?: string;
  updatedAt: string;
  payloadJSON?: string;
};

export type CreateConversationRequest = {
  title?: string;
  model?: string;
  projectID?: string;
};

export type CreateConversationProjectRequest = {
  name: string;
  description?: string;
  systemPrompt?: string;
  color?: string;
  icon?: string;
};

export type UpdateConversationProjectRequest = {
  name?: string;
  description?: string;
  systemPrompt?: string;
  color?: string;
  icon?: string;
  status?: "active" | "archived";
};

export type ReorderConversationProjectsRequest = {
  projectIDs: string[];
};

export type SetConversationProjectRequest = {
  projectID?: string;
};

export type BatchSetConversationProjectRequest = {
  conversationPublicIDs: string[];
  projectID?: string;
};

export type BatchSetConversationProjectResult = {
  updated: number;
};

export type ConversationOptions = Record<string, unknown>;

export type UpstreamDebugInfo = {
  request?: {
    method?: string;
    path?: string;
    headers?: Record<string, string>;
    body?: string;
  };
  response?: {
    statusCode?: number;
    headers?: Record<string, string>;
    body?: string;
  };
};

export type RenameConversationRequest = {
  title: string;
};

export type SetConversationStarRequest = {
  starred: boolean;
};

export type SetConversationArchiveRequest = {
  archived: boolean;
};

export type DeleteConversationData = {
  deleted: boolean;
  deletedFileCount?: number;
  quota?: UserStorageQuotaDTO;
};

export type CreateConversationShareRequest = {
  defaultMessagePublicIDs?: string[];
};

export type ConversationShareDTO = {
  shareID: string;
  status: "none" | "active" | "revoked" | "expired" | string;
  titleSnapshot: string;
  modelSnapshot: string;
  messageCount: number;
  createdAt: string;
  updatedAt: string;
  revokedAt: string | null;
  lastAccessedAt: string | null;
};

export type RevokeConversationSharesRequest = {
  conversationPublicIDs: string[];
};

export type RevokeConversationSharesResult = {
  revoked: boolean;
};

export type PublicSharedMessageDTO = {
  publicID: string;
  parentPublicID: string;
  sourcePublicID: string;
  runID: string;
  role: "user" | "assistant" | "system" | string;
  contentType: string;
  content: string;
  branchReason: "default" | "retry" | "edit" | string;
  tokenUsage: number;
  inputTokens: number;
  outputTokens: number;
  cacheReadTokens: number;
  cacheWriteTokens: number;
  reasoningTokens: number;
  latencyMS: number;
  status: string;
  errorCode: string;
  errorMessage: string;
  attachments: string;
  platformModelName: string;
  upstreamModelName: string;
  modelVendor: string;
  modelIcon: string;
  processTrace?: MessageProcessTraceDTO;
  editedAt: string | null;
  createdAt: string;
  updatedAt: string;
};

export type PublicSharedConversationDTO = {
  shareID: string;
  title: string;
  model: string;
  createdAt: string;
  lastAccessedAt: string | null;
  defaultMessagePublicIDs: string[];
  messages: PublicSharedMessageDTO[];
};

export type SetMessageFeedbackRequest = {
  feedback?: "up" | "down";
};

export type UpdateMessageRequest = {
  content: string;
};

export type MessageFeedbackResult = {
  messageID: number;
  messagePublicID: string;
  myFeedback: "up" | "down" | "";
  thumbsUpCount: number;
  thumbsDownCount: number;
};

export type SendMessageRequest = {
  contentType: "text" | "markdown" | "image" | "file" | "mixed";
  content: string;
  model?: string;
  options?: ConversationOptions;
  clientRunID?: string;
  fileIDs?: string[];
  selectedToolIDs?: number[];
  htmlVisualPrompt?: boolean;
  htmlVisualColorMode?: "light" | "dark";
  parentMessagePublicID?: string;
  sourceMessagePublicID?: string;
  branchReason?: "default" | "retry" | "edit";
};

export type MediaImageRequest = {
  prompt: string;
  model?: string;
  options?: ConversationOptions;
  clientRunID?: string;
  fileIDs?: string[];
  maskFileID?: string;
  parentMessagePublicID?: string;
  sourceMessagePublicID?: string;
  branchReason?: "default" | "retry" | "edit";
};

export type SendMessageResult = {
  userMessage: MessageDTO;
  assistantMessage: MessageDTO;
};

export type StreamMessageEvent =
  | {
      type: "file_proc";
      seq?: number;
      message: string;
    }
  | {
      type: "rag_search";
      seq?: number;
      message: string;
    }
  | {
      type: "process_update";
      seq?: number;
      status: string;
      block?: TraceBlockDTO;
      trace?: MessageProcessTraceDTO;
    }
  | {
      type: "upstream_think_delta";
      seq?: number;
      status: string;
      block?: TraceBlockDTO;
      trace?: MessageProcessTraceDTO;
      reasoning?: ReasoningDeltaDTO;
    }
  | {
      type: "delta";
      seq?: number;
      delta: string;
    }
  | {
      type: "usage";
      seq?: number;
      input_tokens: number;
      output_tokens: number;
      cache_read_tokens: number;
      cache_write_tokens: number;
      reasoning_tokens: number;
    }
  | {
      type: "media_status";
      seq?: number;
      status: string;
      message: string;
    }
  | {
      type: "media_image_delta";
      seq?: number;
      index?: number;
      b64_json: string;
      mime_type?: string;
      revised_prompt?: string;
    }
  | {
      type: "completed";
      seq?: number;
      data: SendMessageResult;
    }
  | {
      type: "compact_done";
      seq?: number;
      method: string;
      freed_tokens: number;
      kept_turns: number;
      summary_preview: string;
    }
  | {
      type: "error";
      seq?: number;
      message: string;
      errorCode?: string;
      debug?: UpstreamDebugInfo;
      data?: SendMessageResult;
    };
