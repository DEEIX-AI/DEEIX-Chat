import type {
  BatchDeleteRequest,
  BatchDeleteResponse,
  BatchDeleteResultResponse,
  BindModelUpstreamSourceRequest,
  CircuitResetResponse,
  CreateModelRequest,
  CreateUpstreamRequest,
  ImportUpstreamModelsRequest,
  ImportUpstreamModelsResponse,
  ModelDataResponse,
  ModelProbeBatchResponse,
  ModelProbeDebugRequestResponse,
  ModelProbeDebugResponse,
  ModelProbeDebugResponseResponse,
  ModelProbeResponse,
  ModelResponse,
  ModelUpstreamSourceDataResponse,
  ModelUpstreamSourceResponse,
  ReorderModelsRequest,
  UpdateModelRequest,
  UpdateModelUpstreamSourceRequest,
  UpdateUpstreamRequest,
  UpsertUpstreamModelRequest,
  UpstreamAPIKeyResponse,
  UpstreamDataResponse,
  UpstreamModelDataResponse,
  UpstreamModelResponse,
  UpstreamRemoteModelResponse,
  UpstreamRemoteModelsResponse,
  UpstreamResponse,
} from "@deeix/api-contract";
import type { PagePayload } from "@/shared/api/common.types";

export type AdminLLMStatus = "active" | "inactive";
export type AdminLLMModelAccessScope = "public" | "internal";
export type AdminLLMAdapter =
  | "openai_responses"
  | "openrouter_chat_completions"
  | "openrouter_responses"
  | "openai_chat_completions"
  | "openai_image_generations"
  | "openai_image_edits"
  | "openai_video_generations"
  | "anthropic_messages"
  | "google_generate_content"
  | "google_image_generation"
  | "gemini_interactions"
  | "xai_responses"
  | "xai_image"
  | "xai_image_edits";
export type AdminLLMModelVendor = string;
export type AdminLLMCompatible =
  | "openai"
  | "anthropic"
  | "google"
  | "xai"
  | "openrouter"
  | "custom";
export type AdminLLMCbLogic = "or" | "and";
export type AdminLLMModelCbPolicyMode = "default" | "enforced";

// ---------------------------------------------------------------------------
// Upstream views
// ---------------------------------------------------------------------------

export type AdminLLMUpstreamView = Omit<
  UpstreamResponse,
  "apiKeyItems" | "cbThresholdLogic" | "compatible" | "status"
> & {
  compatible: AdminLLMCompatible | "";
  apiKeyItems?: AdminLLMUpstreamAPIKey[];
  status: AdminLLMStatus;
  cbThresholdLogic: AdminLLMCbLogic;
};

export type AdminLLMUpstreamAPIKey = UpstreamAPIKeyResponse;

export type AdminLLMModelDTO = Omit<
  ModelResponse,
  "accessScope" | "cbPolicyMode" | "status" | "vendor"
> & {
  vendor: AdminLLMModelVendor;
  accessScope: AdminLLMModelAccessScope;
  status: AdminLLMStatus;
  cbPolicyMode: AdminLLMModelCbPolicyMode;
};

export type AdminLLMUpstreamModelDTO = Omit<
  UpstreamModelResponse,
  "modelVendor" | "protocol" | "routeStatus" | "suggestedProtocol" | "upstreamModelStatus" | "upstreamModelVendor"
> & {
  modelVendor: AdminLLMModelVendor;
  upstreamModelVendor: AdminLLMModelVendor;
  suggestedProtocol: AdminLLMAdapter | "";
  protocol: AdminLLMAdapter | "";
  upstreamModelStatus: AdminLLMStatus;
  routeStatus: AdminLLMStatus | "";
};

export type AdminLLMModelUpstreamSourceDTO = Omit<
  ModelUpstreamSourceResponse,
  | "circuitScope"
  | "protocol"
  | "status"
  | "suggestedProtocol"
  | "upstreamModelStatus"
  | "upstreamModelVendor"
  | "upstreamStatus"
> & {
  upstreamStatus: AdminLLMStatus;
  upstreamModelVendor: AdminLLMModelVendor;
  suggestedProtocol: AdminLLMAdapter | "";
  upstreamModelStatus: AdminLLMStatus;
  protocol: AdminLLMAdapter | "";
  status: AdminLLMStatus;
  circuitScope: "upstream" | "source" | "";
};

export type AdminLLMUpstreamHealthView = {
  upstreamID: number;
  upstreamName: string;
  status: string;
  failureCount: number;
  circuitOpen: boolean;
  circuitUntil: string;
  lastError: string;
  lastFailureAt: string;
  lastSuccessAt: string;
};

export type AdminLLMModelProbeDebug = Omit<ModelProbeDebugResponse, "request" | "response"> & {
  request: ModelProbeDebugRequestResponse;
  response: ModelProbeDebugResponseResponse;
};

export type AdminLLMModelProbeResult = Omit<ModelProbeResponse, "debug" | "protocol" | "status"> & {
  status: "success" | "failed" | "unsupported";
  protocol: AdminLLMAdapter | "";
  upstreamID: number;
  upstreamName: string;
  upstreamModelID: number;
  upstreamModelName: string;
  upstreamStatusCode?: number;
  debug?: AdminLLMModelProbeDebug;
};

export type AdminLLMModelProbeBatchResult = Omit<ModelProbeBatchResponse, "results"> & {
  results: AdminLLMModelProbeResult[];
};

export type AdminLLMRemoteModelItem = Omit<
  UpstreamRemoteModelResponse,
  "suggestedProtocol" | "suggestedProtocols" | "upstreamModelStatus"
> & {
  suggestedProtocol: AdminLLMAdapter | "";
  suggestedProtocols: AdminLLMAdapter[];
  upstreamModelStatus: AdminLLMStatus | "";
};

export type AdminLLMSetting = {
  id: number;
  key: string;
  value: string;
  description: string;
  createdAt: string;
  updatedAt: string;
};

// ---------------------------------------------------------------------------
// Request types
// ---------------------------------------------------------------------------

export type CreateAdminLLMUpstreamRequest = Omit<
  CreateUpstreamRequest,
  "cbThresholdLogic" | "compatible" | "status"
> & {
  compatible?: AdminLLMCompatible | "";
  status?: AdminLLMStatus;
  cbThresholdLogic?: AdminLLMCbLogic;
};

export type UpdateAdminLLMUpstreamRequest = Omit<
  UpdateUpstreamRequest,
  "cbThresholdLogic" | "compatible" | "status"
> & {
  compatible?: AdminLLMCompatible | "";
  status?: AdminLLMStatus;
  cbThresholdLogic?: AdminLLMCbLogic;
};

export type CreateAdminLLMModelRequest = Omit<
  CreateModelRequest,
  "accessScope" | "cbPolicyMode" | "status" | "vendor"
> & {
  vendor?: AdminLLMModelVendor;
  accessScope?: AdminLLMModelAccessScope;
  status?: AdminLLMStatus;
  cbPolicyMode?: AdminLLMModelCbPolicyMode;
};

export type UpdateAdminLLMModelRequest = Omit<
  UpdateModelRequest,
  "accessScope" | "cbPolicyMode" | "status" | "vendor"
> & {
  vendor?: AdminLLMModelVendor;
  accessScope?: AdminLLMModelAccessScope;
  status?: AdminLLMStatus;
  cbPolicyMode?: AdminLLMModelCbPolicyMode;
};

export type ReorderAdminLLMModelsRequest = ReorderModelsRequest;

export type UpsertAdminLLMUpstreamModelRequest = Omit<UpsertUpstreamModelRequest, "protocol" | "status"> & {
  protocol?: AdminLLMAdapter;
  status?: AdminLLMStatus;
};

export type UpdateAdminLLMModelUpstreamSourceRequest = Omit<
  UpdateModelUpstreamSourceRequest,
  "protocol" | "status"
> & {
  protocol?: AdminLLMAdapter;
  status?: AdminLLMStatus;
};

export type BindAdminLLMModelUpstreamSourceRequest = Omit<
  BindModelUpstreamSourceRequest,
  "protocol" | "status"
> & {
  protocol?: AdminLLMAdapter;
  status?: AdminLLMStatus;
};

export type ImportAdminLLMUpstreamModelsRequest = Omit<ImportUpstreamModelsRequest, "items"> & {
  items: Array<
    Omit<ImportUpstreamModelsRequest["items"][number], "protocol" | "protocols" | "status"> & {
      protocol?: AdminLLMAdapter;
      protocols?: AdminLLMAdapter[];
      status?: AdminLLMStatus;
    }
  >;
};

export type AdminBatchDeleteStatus = "deleted" | "not_found" | "failed";

export type AdminBatchDeleteRequest = BatchDeleteRequest;

// ---------------------------------------------------------------------------
// Response data wrappers
// ---------------------------------------------------------------------------

export type AdminLLMUpstreamData = Omit<UpstreamDataResponse, "upstream"> & {
  upstream: AdminLLMUpstreamView;
};

export type AdminLLMModelData = Omit<ModelDataResponse, "model"> & {
  model: AdminLLMModelDTO;
};

export type AdminLLMUpstreamModelData = Omit<UpstreamModelDataResponse, "binding"> & {
  binding: AdminLLMUpstreamModelDTO;
};

export type AdminLLMModelUpstreamSourceData = Omit<ModelUpstreamSourceDataResponse, "source"> & {
  source: AdminLLMModelUpstreamSourceDTO;
};

export type AdminLLMModelProbeData = AdminLLMModelProbeResult;
export type AdminLLMModelProbeBatchData = AdminLLMModelProbeBatchResult;

export type ResetAdminLLMCircuitData = CircuitResetResponse;

export type ListAdminLLMRemoteModelsData = Omit<UpstreamRemoteModelsResponse, "items"> & {
  items: AdminLLMRemoteModelItem[];
};

export type ImportAdminLLMUpstreamModelsData = Omit<ImportUpstreamModelsResponse, "results"> & {
  results: Array<
    Omit<ImportUpstreamModelsResponse["results"][number], "error" | "protocols" | "status"> & {
      status: "created" | "existing" | "failed";
      protocols: AdminLLMAdapter[];
      error?: string;
    }
  >;
};

export type AdminBatchDeleteResult = Omit<BatchDeleteResultResponse, "error" | "status"> & {
  status: AdminBatchDeleteStatus;
  error?: string;
};

export type AdminBatchDeleteData = Omit<BatchDeleteResponse, "results"> & {
  results: AdminBatchDeleteResult[];
};

// ---------------------------------------------------------------------------
// Page results
// ---------------------------------------------------------------------------

export type ListAdminLLMUpstreamsResult = PagePayload<AdminLLMUpstreamView>;
export type ListAdminLLMModelsResult = PagePayload<AdminLLMModelDTO>;
export type ListAdminLLMUpstreamModelsResult = PagePayload<AdminLLMUpstreamModelDTO>;
export type ListAdminLLMModelUpstreamSourcesResult = PagePayload<AdminLLMModelUpstreamSourceDTO>;
