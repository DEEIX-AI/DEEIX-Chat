import type { UpstreamDebugInfo } from "@/shared/api/conversation.types";

export type MessageAttachment = {
  fileID: string;
  fileName: string;
  mimeType: string;
  detectedMime?: string;
  fileCategory?: string;
  sizeBytes: number;
  kind: "file" | "image";
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
};

export type ChatMessageBranchNavigator = {
  parentPublicID: string | null;
  index: number;
  total: number;
  canPrevious: boolean;
  canNext: boolean;
};

export type RAGCitation = {
  file_name: string;
  file_id: string;
  chunk_index: number;
  score: number;
  preview: string;
};

export type ChatTraceBlock = {
  title: string;
  summary: string;
  contentMarkdown: string;
  contentSegments?: string[];
  status: string;
  stage?: string;
  roundID?: string;
  parentEventID?: string;
  updatedAt?: string;
  payloadJson?: string;
};

export type ChatTraceEvent = {
  eventID: string;
  eventType: "process" | "tool" | "think" | string;
  phase: "process" | "tools" | "upstream_think" | string;
  stage?: "process" | "think" | "tool" | "answer" | string;
  roundID?: string;
  parentEventID?: string;
  title: string;
  summary: string;
  contentMarkdown: string;
  status: string;
  seq: number;
  startedAt?: string;
  endedAt?: string;
  updatedAt?: string;
  payloadJson?: string;
};

export type ChatPromptTraceBlock = {
  kind: string;
  title: string;
  tokenEstimate: number;
  cacheable: boolean;
  sourceCount: number;
  sourceRefs?: ChatPromptTraceSource[];
};

export type ChatPromptTraceSource = {
  sourceType: string;
  sourceID: string;
  title: string;
  artifactID?: number;
};

export type ChatPromptTrace = {
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
  blocks: ChatPromptTraceBlock[];
};

export type ChatMessageProcessTrace = {
  enabled: boolean;
  status: string;
  process?: ChatTraceBlock;
  tools?: ChatTraceBlock;
  upstreamThink?: ChatTraceBlock;
  promptTrace?: ChatPromptTrace;
  events?: ChatTraceEvent[];
};

export type ChatInlineAlert = {
  title: string;
  message: string;
  details?: UpstreamDebugInfo;
};

export type ChatBillingCost = {
  billingMode: string;
  billedCurrency: string;
  billedNanousd: number;
  billedUSD: number;
  pricingSnapshotJSON: string;
};

export type ImageLoadingAspectRatio = "wide" | "portrait" | "square";

export type ChatAreaMessage = {
  key: string;
  publicID: string;
  parentPublicID: string | null;
  sourcePublicID: string | null;
  role: "user" | "assistant" | "system";
  contentType?: string;
  content: string;
  branchReason: "default" | "retry" | "edit";
  status?: string;
  runID?: string;
  platformModelName?: string;
  serverMessageID?: number;
  createdAt?: string;
  updatedAt?: string;
  editedAt?: string | null;
  isPending?: boolean;
  isStreaming?: boolean;
  isFileProc?: boolean; // Active file_proc stream stage.
  activityLabel?: string;
  imageAspectRatio?: ImageLoadingAspectRatio;
  myFeedback?: "up" | "down" | null;
  thumbsUpCount?: number;
  thumbsDownCount?: number;
  branchNavigator?: ChatMessageBranchNavigator;
  attachments?: MessageAttachment[];
  // Token usage for assistant messages.
  inputTokens?: number;
  outputTokens?: number;
  cacheReadTokens?: number;
  cacheWriteTokens?: number;
  reasoningTokens?: number;
  latencyMS?: number;
  billingCost?: ChatBillingCost;
  processTrace?: ChatMessageProcessTrace;
  inlineAlert?: ChatInlineAlert;
  compactDone?: { method: string; freed_tokens: number; summary_preview: string };
};
