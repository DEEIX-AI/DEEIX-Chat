import type { PagePayload } from "@/shared/api/common.types";

export type AdminLLMStatus = "active" | "inactive";
export type AdminLLMModelAccessScope = "public" | "internal";
export type AdminLLMAdapter =
  | "openai_responses"
  | "openai_chat_completions"
  | "openai_image_generations"
  | "openai_image_edits"
  | "openai_video_generations"
  | "anthropic_messages"
  | "google_generate_content"
  | "google_image_generation"
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

// ---------------------------------------------------------------------------
// Upstream views
// ---------------------------------------------------------------------------

export type AdminLLMUpstreamView = {
  id: number;
  name: string;
  baseURL: string;
  compatible: AdminLLMCompatible | "";
  protocolDefaultsJSON: string;
  apiKeysMasked: string;
  apiKeyItems?: AdminLLMUpstreamAPIKey[];
  status: AdminLLMStatus;
  connectTimeoutMS: number;
  readTimeoutMS: number;
  streamIdleTimeoutMS: number;
  cbFailureThreshold: number;
  cbModelThreshold: number;
  cbThresholdLogic: AdminLLMCbLogic;
  cbDurationMin: number;
  cbWindowMin: number;
  headersJSON: string;
  modelsCount: number;
  activeModelsCount: number;
  circuitOpen: boolean;
  circuitUntil: string;
  createdAt: string;
  updatedAt: string;
};

export type AdminLLMUpstreamAPIKey = {
  id: string;
  index: number;
  keyMasked: string;
  status: string;
  note: string;
};

export type AdminLLMModelDTO = {
  id: number;
  platformModelName: string;
  vendor: AdminLLMModelVendor;
  kindsJSON: string;
  icon: string;
  capabilitiesJSON: string;
  systemPrompt: string;
  accessScope: AdminLLMModelAccessScope;
  status: AdminLLMStatus;
  description: string;
  sortOrder: number;
  sourceCount: number;
  activeSourceCount: number;
  protocolsJSON: string;
  createdAt: string;
  updatedAt: string;
};

export type AdminLLMUpstreamModelDTO = {
  id: number;
  routeID: number;
  upstreamID: number;
  bindingCode: string;
  platformModelID: number;
  platformModelName: string;
  modelVendor: AdminLLMModelVendor;
  modelKindsJSON: string;
  modelIcon: string;
  upstreamModelName: string;
  upstreamModelVendor: AdminLLMModelVendor;
  upstreamModelIcon: string;
  upstreamModelKindsJSON: string;
  suggestedProtocol: AdminLLMAdapter | "";
  protocol: AdminLLMAdapter | "";
  upstreamModelStatus: AdminLLMStatus;
  routeStatus: AdminLLMStatus | "";
  priority: number;
  weight: number;
  source: string;
  cbFailureThreshold: number;
  cbDurationMin: number;
  cbWindowMin: number;
  headersJSON: string;
  circuitOpen: boolean;
  circuitUntil: string;
  createdAt: string;
  updatedAt: string;
};

export type AdminLLMModelUpstreamSourceDTO = {
  id: number;
  upstreamID: number;
  upstreamName: string;
  baseURL: string;
  bindingCode: string;
  upstreamModelName: string;
  upstreamModelKindsJSON: string;
  upstreamModelVendor: AdminLLMModelVendor;
  upstreamModelIcon: string;
  suggestedProtocol: AdminLLMAdapter | "";
  upstreamModelStatus: AdminLLMStatus;
  protocol: AdminLLMAdapter | "";
  status: AdminLLMStatus;
  priority: number;
  weight: number;
  source: string;
  headersJSON: string;
  circuitOpen: boolean;
  circuitUntil: string;
  createdAt: string;
  updatedAt: string;
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

export type AdminLLMModelProbeDebug = {
  request: {
    method: string;
    path: string;
    headers?: Record<string, string>;
    body: string;
  };
  response: {
    statusCode: number;
    headers?: Record<string, string>;
    body: string;
  };
};

export type AdminLLMModelProbeResult = {
  success: boolean;
  status: "success" | "failed" | "unsupported";
  errorCode?: string;
  errorMessage?: string;
  latencyMS: number;
  protocol: AdminLLMAdapter | "";
  endpoint: string;
  platformModelID: number;
  platformModelName: string;
  upstreamID: number;
  upstreamName: string;
  upstreamModelID: number;
  upstreamModelName: string;
  routeID: number;
  bindingCode: string;
  upstreamStatusCode?: number;
  debug?: AdminLLMModelProbeDebug;
};

export type AdminLLMModelProbeBatchResult = {
  totalCount: number;
  successCount: number;
  failedCount: number;
  unsupportedCount: number;
  results: AdminLLMModelProbeResult[];
};

export type AdminLLMRemoteModelItem = {
  upstreamModelName: string;
  suggestedPlatformModelName: string;
  suggestedKindsJSON: string;
  suggestedProtocol: AdminLLMAdapter | "";
  suggestedProtocols: AdminLLMAdapter[];
  bindingCode: string;
  boundPlatformModels: string[];
  upstreamModelStatus: AdminLLMStatus | "";
  alreadySynced: boolean;
  alreadyBound: boolean;
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

export type CreateAdminLLMUpstreamRequest = {
  name: string;
  baseURL: string;
  compatible?: AdminLLMCompatible | "";
  protocolDefaultsJSON?: string;
  apiKeys: string;
  status?: AdminLLMStatus;
  connectTimeoutMS?: number;
  readTimeoutMS?: number;
  streamIdleTimeoutMS?: number;
  cbFailureThreshold?: number;
  cbModelThreshold?: number;
  cbThresholdLogic?: AdminLLMCbLogic;
  cbDurationMin?: number;
  cbWindowMin?: number;
  headersJSON?: string;
};

export type UpdateAdminLLMUpstreamRequest = {
  name?: string;
  baseURL?: string;
  compatible?: AdminLLMCompatible | "";
  protocolDefaultsJSON?: string;
  apiKeys?: string;
  addAPIKeys?: string;
  deleteAPIKeyIDs?: string[];
  status?: AdminLLMStatus;
  connectTimeoutMS?: number;
  readTimeoutMS?: number;
  streamIdleTimeoutMS?: number;
  cbFailureThreshold?: number;
  cbModelThreshold?: number;
  cbThresholdLogic?: AdminLLMCbLogic;
  cbDurationMin?: number;
  cbWindowMin?: number;
  headersJSON?: string;
};

export type CreateAdminLLMModelRequest = {
  platformModelName: string;
  vendor?: AdminLLMModelVendor;
  kindsJSON?: string;
  icon?: string;
  capabilitiesJSON?: string;
  systemPrompt?: string;
  accessScope?: AdminLLMModelAccessScope;
  status?: AdminLLMStatus;
  description?: string;
};

export type UpdateAdminLLMModelRequest = {
  platformModelName?: string;
  vendor?: AdminLLMModelVendor;
  kindsJSON?: string;
  icon?: string;
  capabilitiesJSON?: string;
  systemPrompt?: string;
  accessScope?: AdminLLMModelAccessScope;
  status?: AdminLLMStatus;
  description?: string;
};

export type ReorderAdminLLMModelsRequest = {
  modelIDs: number[];
};

export type UpsertAdminLLMUpstreamModelRequest = {
  routeID?: number;
  platformModelName: string;
  upstreamModelName: string;
  protocol?: AdminLLMAdapter;
  kindsJSON?: string;
  status?: AdminLLMStatus;
  priority?: number;
  weight?: number;
  source?: string;
  cbFailureThreshold?: number;
  cbDurationMin?: number;
  cbWindowMin?: number;
  headersJSON?: string;
};

export type UpdateAdminLLMModelUpstreamSourceRequest = {
  protocol?: AdminLLMAdapter;
  status?: AdminLLMStatus;
  priority?: number;
  weight?: number;
};

export type ImportAdminLLMUpstreamModelsRequest = {
  items: Array<{
    platformModelName: string;
    upstreamModelName: string;
    protocol?: AdminLLMAdapter;
    protocols?: AdminLLMAdapter[];
    kindsJSON?: string;
    status?: AdminLLMStatus;
    priority?: number;
  }>;
};

export type AdminBatchDeleteStatus = "deleted" | "not_found" | "failed";

export type AdminBatchDeleteRequest = {
  ids: number[];
};

// ---------------------------------------------------------------------------
// Response data wrappers
// ---------------------------------------------------------------------------

export type AdminLLMUpstreamData = {
  upstream: AdminLLMUpstreamView;
};

export type AdminLLMModelData = {
  model: AdminLLMModelDTO;
};

export type AdminLLMUpstreamModelData = {
  binding: AdminLLMUpstreamModelDTO;
};

export type AdminLLMModelUpstreamSourceData = {
  source: AdminLLMModelUpstreamSourceDTO;
};

export type AdminLLMModelProbeData = AdminLLMModelProbeResult;
export type AdminLLMModelProbeBatchData = AdminLLMModelProbeBatchResult;

export type ResetAdminLLMCircuitData = {
  reset: boolean;
};

export type ListAdminLLMRemoteModelsData = {
  total: number;
  items: AdminLLMRemoteModelItem[];
};

export type ImportAdminLLMUpstreamModelsData = {
  total: number;
  importedCount: number;
  failedCount: number;
  createdRoutes: number;
  existingRoutes: number;
  createdPlatform: number;
  results: Array<{
    upstreamModelName: string;
    platformModelName: string;
    bindingCode: string;
    status: "created" | "existing" | "failed";
    createdRoute: boolean;
    createdRoutes: number;
    existingRoutes: number;
    protocols: AdminLLMAdapter[];
    createdPlatform: boolean;
    error?: string;
  }>;
};

export type AdminBatchDeleteResult = {
  id: number;
  status: AdminBatchDeleteStatus;
  error?: string;
};

export type AdminBatchDeleteData = {
  total: number;
  successCount: number;
  notFoundCount: number;
  failedCount: number;
  results: AdminBatchDeleteResult[];
};

// ---------------------------------------------------------------------------
// Page results
// ---------------------------------------------------------------------------

export type ListAdminLLMUpstreamsResult = PagePayload<AdminLLMUpstreamView>;
export type ListAdminLLMModelsResult = PagePayload<AdminLLMModelDTO>;
export type ListAdminLLMUpstreamModelsResult = PagePayload<AdminLLMUpstreamModelDTO>;
export type ListAdminLLMModelUpstreamSourcesResult = PagePayload<AdminLLMModelUpstreamSourceDTO>;
