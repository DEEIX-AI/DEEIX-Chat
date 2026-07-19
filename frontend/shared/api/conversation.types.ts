import type {
  BatchSetConversationProjectRequest as ContractBatchSetConversationProjectRequest,
  BatchSetConversationProjectResponse,
  ContextArtifactResponse,
  ConversationDefaultModelCandidateResponse,
  ConversationDeleteResponse,
  ConversationExportResponse,
  ConversationProjectResponse,
  ConversationResponse,
  ConversationShareResponse,
  CreateConversationProjectRequest as ContractCreateConversationProjectRequest,
  CreateConversationRequest as ContractCreateConversationRequest,
  CreateConversationShareRequest as ContractCreateConversationShareRequest,
  MessageBillingCostResponse,
  MessageFeedbackResponse,
  MessageProcessTraceResponse,
  MessagePromptTraceBlockResponse,
  MessagePromptTraceResponse,
  MessagePromptTraceSourceResponse,
  MessageResponse,
  MessageTraceBlockResponse,
  MessageTraceEventResponse,
  ModelProbeDebugResponse,
  PublicSharedConversationResponse,
  PublicSharedMessageResponse,
  RenameConversationRequest as ContractRenameConversationRequest,
  ReorderConversationProjectsRequest as ContractReorderConversationProjectsRequest,
  RevokeConversationSharesRequest as ContractRevokeConversationSharesRequest,
  RevokeConversationSharesResponse,
  RunResponse,
  SendMessageRequest as ContractSendMessageRequest,
  SendMessageResponse,
  SetConversationArchiveRequest as ContractSetConversationArchiveRequest,
  SetConversationProjectRequest as ContractSetConversationProjectRequest,
  SetConversationStarRequest as ContractSetConversationStarRequest,
  SetMessageFeedbackRequest as ContractSetMessageFeedbackRequest,
  UpdateConversationProjectRequest as ContractUpdateConversationProjectRequest,
  UpdateMessageRequest as ContractUpdateMessageRequest,
} from "@deeix/api-contract";
import type { UserStorageQuotaDTO } from "@/shared/api/file.types";

export type ConversationDTO = ConversationResponse;

export type ConversationDefaultModelCandidateDTO = ConversationDefaultModelCandidateResponse;

export type ConversationStatusFilter = "active" | "archived" | "all";
export type ConversationStarredFilter = "all" | "starred" | "unstarred";
export type ConversationShareFilter = "all" | "shared" | "unshared";
export type ConversationProjectFilter = "all" | "unassigned" | string;
export type ConversationProjectStatusFilter = "active" | "archived" | "all";
export type ConversationProjectMCPDefaultMode = "inherit" | "custom";

export type ConversationProjectDTO = Omit<ConversationProjectResponse, "mcpDefaultMode"> & {
  mcpDefaultMode: ConversationProjectMCPDefaultMode;
};

export type MessageDTO = Omit<
  MessageResponse,
  | "billingCost"
  | "modelIcon"
  | "modelVendor"
  | "platformModelName"
  | "processTrace"
  | "upstreamModelName"
> & {
  branchReason: "default" | "retry" | "edit";
  platformModelName?: string;
  upstreamModelName?: string;
  modelVendor?: string;
  modelIcon?: string;
  processTrace?: MessageProcessTraceDTO;
  myFeedback: "up" | "down" | "";
  billingCost?: MessageBillingCostDTO;
};

export type ConversationRunDTO = Omit<RunResponse, "taskType">;

export type ConversationExportDTO = Omit<
  ConversationExportResponse,
  "compatibility" | "conversation" | "messages" | "runs"
> & {
  conversation: ConversationDTO;
  messages: MessageDTO[];
  runs: ConversationRunDTO[];
  compatibility: ConversationExportResponse["compatibility"];
};

export type MessageBillingCostDTO = MessageBillingCostResponse;

export type TraceBlockDTO = MessageTraceBlockResponse;

export type PromptTraceBlockDTO = Omit<MessagePromptTraceBlockResponse, "sourceRefs"> & {
  sourceRefs?: PromptTraceSourceDTO[];
};

export type PromptTraceSourceDTO = MessagePromptTraceSourceResponse;

export type ContextArtifactDTO = ContextArtifactResponse;

export type PromptTraceDTO = Omit<MessagePromptTraceResponse, "blocks"> & {
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

export type MessageProcessTraceDTO = Omit<
  MessageProcessTraceResponse,
  "events" | "process" | "promptTrace" | "tools" | "upstreamThink"
> & {
  process?: TraceBlockDTO;
  tools?: TraceBlockDTO;
  upstreamThink?: TraceBlockDTO;
  promptTrace?: PromptTraceDTO;
  events?: TraceEventDTO[];
};

export type TraceEventDTO = MessageTraceEventResponse;

export type CreateConversationRequest = ContractCreateConversationRequest;

export type CreateConversationProjectRequest = Omit<ContractCreateConversationProjectRequest, "mcpDefaultMode"> & {
  mcpDefaultMode?: ConversationProjectMCPDefaultMode;
};

export type UpdateConversationProjectRequest = Omit<ContractUpdateConversationProjectRequest, "mcpDefaultMode"> & {
  mcpDefaultMode?: ConversationProjectMCPDefaultMode;
};

export type ReorderConversationProjectsRequest = ContractReorderConversationProjectsRequest;

export type SetConversationProjectRequest = ContractSetConversationProjectRequest;

export type BatchSetConversationProjectRequest = ContractBatchSetConversationProjectRequest;

export type BatchSetConversationProjectResult = BatchSetConversationProjectResponse;

export type ConversationOptions = Record<string, unknown>;

export type UpstreamDebugInfo = ModelProbeDebugResponse;

export type RenameConversationRequest = ContractRenameConversationRequest;

export type SetConversationStarRequest = ContractSetConversationStarRequest;

export type SetConversationArchiveRequest = ContractSetConversationArchiveRequest;

export type DeleteConversationData = Omit<ConversationDeleteResponse, "quota"> & {
  quota?: UserStorageQuotaDTO;
};

export type CreateConversationShareRequest = ContractCreateConversationShareRequest;

export type ConversationShareDTO = ConversationShareResponse;

export type RevokeConversationSharesRequest = ContractRevokeConversationSharesRequest;

export type RevokeConversationSharesResult = RevokeConversationSharesResponse;

export type PublicSharedMessageDTO = Omit<PublicSharedMessageResponse, "processTrace"> & {
  processTrace?: MessageProcessTraceDTO;
};

export type PublicSharedConversationDTO = Omit<PublicSharedConversationResponse, "messages"> & {
  messages: PublicSharedMessageDTO[];
};

export type SetMessageFeedbackRequest = ContractSetMessageFeedbackRequest;

export type UpdateMessageRequest = ContractUpdateMessageRequest;

export type MessageFeedbackResult = Omit<MessageFeedbackResponse, "myFeedback"> & {
  myFeedback: "up" | "down" | "";
};

export type SendMessageRequest = Omit<ContractSendMessageRequest, "options"> & {
  options?: ConversationOptions;
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

export type MediaVideoRequest = {
  prompt: string;
  model?: string;
  options?: ConversationOptions;
  clientRunID?: string;
  fileIDs?: string[];
  parentMessagePublicID?: string;
  sourceMessagePublicID?: string;
  branchReason?: "default" | "retry" | "edit";
};

export type SendMessageResult = Omit<SendMessageResponse, "assistantMessage" | "metadataRefreshHint" | "userMessage"> & {
  userMessage: MessageDTO;
  assistantMessage: MessageDTO;
  metadataRefreshHint?: "pending" | "not_needed" | "skipped_no_titleable_content" | string;
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
      title?: string;
      summary?: string;
      stage?: string;
      roundID?: string;
      eventID?: string;
      kind?: ReasoningDeltaDTO["kind"] | string;
      delta?: string;
      contentMarkdown?: string;
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
      content_type?: string;
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
