import { authedRequest } from "@/shared/api/authed-client";
import { pathParam } from "@/shared/api/http-client";
import type { PagePayload } from "@/shared/api/common.types";
import type {
  AdminBatchDeleteData,
  AdminBatchDeleteRequest,
  AdminLLMSetting,
  AdminLLMModelData,
  AdminLLMModelDTO,
  AdminLLMModelDisplayGroupData,
  AdminLLMModelDisplayGroupDTO,
  AdminLLMModelIconAsset,
  AdminLLMModelIconAssetListItem,
  AdminLLMModelVendorData,
  AdminLLMModelVendorDTO,
  AdminLLMModelProbeBatchData,
  AdminLLMModelProbeData,
  AdminLLMModelUpstreamSourceData,
  AdminLLMModelUpstreamSourceDTO,
  AdminLLMUpstreamData,
  AdminLLMUpstreamModelData,
  AdminLLMUpstreamModelDTO,
  AdminLLMUpstreamView,
  BindAdminLLMModelUpstreamSourceRequest,
  CreateAdminLLMModelRequest,
  CreateAdminLLMModelDisplayGroupRequest,
  CreateAdminLLMModelVendorRequest,
  CreateAdminLLMUpstreamRequest,
  ImportAdminLLMUpstreamModelsData,
  ImportAdminLLMUpstreamModelsRequest,
  ListAdminLLMRemoteModelsData,
  ReorderAdminLLMModelsRequest,
  SetAdminLLMModelProtocolsRequest,
  SetAdminLLMModelsDisplayGroupRequest,
  ResetAdminLLMCircuitData,
  SyncAdminLLMUpstreamModelsData,
  UpdateAdminLLMModelRequest,
  UpdateAdminLLMModelDisplayGroupRequest,
  UpdateAdminLLMModelVendorRequest,
  UpdateAdminLLMModelUpstreamSourceRequest,
  UpdateAdminLLMUpstreamRequest,
  UpsertAdminLLMUpstreamModelRequest,
} from "@/features/admin/api/llm.types";

import { normalizeAdminPagePayload, resolveAdminPage, type AdminListQueryOptions, type AdminPageOptions } from "./shared";

type ListAdminLLMUpstreamsOptions = AdminListQueryOptions & {
  compatible?: string;
};

type ListAdminLLMModelsOptions = AdminListQueryOptions & {
  onlyActive?: boolean;
  onlyAvailable?: boolean;
  vendor?: string;
  protocol?: string;
  upstream?: string;
};

type ListAdminLLMUpstreamModelsOptions = AdminPageOptions & {
  query?: string;
  routeStatus?: string;
  upstreamStatus?: string;
  protocol?: string;
  sort?: string;
};

// ---------------------------------------------------------------------------
// Upstream
// ---------------------------------------------------------------------------

export async function listAdminLLMUpstreams(
  accessToken: string,
  options: ListAdminLLMUpstreamsOptions = {},
): Promise<PagePayload<AdminLLMUpstreamView>> {
  const { page, pageSize } = resolveAdminPage(options);
  const params = new URLSearchParams({
    page: String(page),
    page_size: String(pageSize),
  });
  if (options.query?.trim()) {
    params.set("q", options.query.trim());
  }
  if (options.status?.trim()) {
    params.set("status", options.status.trim());
  }
  if (options.compatible?.trim()) {
    params.set("compatible", options.compatible.trim());
  }
  if (options.sort?.trim()) {
    params.set("sort", options.sort.trim());
  }
  const data = await authedRequest<PagePayload<AdminLLMUpstreamView>>(
    `/api/v1/admin/llm/upstreams?${params.toString()}`,
    { accessToken },
    true,
  );
  return normalizeAdminPagePayload(data);
}

export async function createAdminLLMUpstream(
  accessToken: string,
  payload: CreateAdminLLMUpstreamRequest,
): Promise<AdminLLMUpstreamData> {
  return authedRequest<AdminLLMUpstreamData>(
    "/api/v1/admin/llm/upstreams",
    { method: "POST", accessToken, body: payload },
    true,
  );
}

export async function updateAdminLLMUpstream(
  accessToken: string,
  upstreamID: number,
  payload: UpdateAdminLLMUpstreamRequest,
): Promise<AdminLLMUpstreamData> {
  return authedRequest<AdminLLMUpstreamData>(
    `/api/v1/admin/llm/upstreams/${upstreamID}`,
    { method: "PATCH", accessToken, body: payload },
    true,
  );
}

export async function deleteAdminLLMUpstream(
  accessToken: string,
  upstreamID: number,
): Promise<void> {
  return authedRequest<void>(
    `/api/v1/admin/llm/upstreams/${upstreamID}`,
    { method: "DELETE", accessToken },
    true,
  );
}

export async function batchDeleteAdminLLMUpstreams(
  accessToken: string,
  payload: AdminBatchDeleteRequest,
): Promise<AdminBatchDeleteData> {
  return authedRequest<AdminBatchDeleteData>(
    "/api/v1/admin/llm/upstreams/batch-delete",
    { method: "POST", accessToken, body: payload },
    true,
  );
}

export async function openAdminLLMUpstreamCircuit(
  accessToken: string,
  upstreamID: number,
): Promise<void> {
  return authedRequest<void>(
    `/api/v1/admin/llm/upstreams/${upstreamID}/circuit/open`,
    { method: "POST", accessToken },
    true,
  );
}

export async function resetAdminLLMUpstreamCircuit(
  accessToken: string,
  upstreamID: number,
): Promise<ResetAdminLLMCircuitData> {
  return authedRequest<ResetAdminLLMCircuitData>(
    `/api/v1/admin/llm/upstreams/${upstreamID}/circuit/reset`,
    { method: "POST", accessToken },
    true,
  );
}

// ---------------------------------------------------------------------------
// Upstream route bindings
// ---------------------------------------------------------------------------

export async function listAdminLLMUpstreamModels(
  accessToken: string,
  upstreamID: number,
  options: ListAdminLLMUpstreamModelsOptions = {},
): Promise<PagePayload<AdminLLMUpstreamModelDTO>> {
  const { page, pageSize } = resolveAdminPage(options);
  const params = new URLSearchParams({
    page: String(page),
    page_size: String(pageSize),
  });
  if (options.query?.trim()) {
    params.set("q", options.query.trim());
  }
  if (options.routeStatus?.trim()) {
    params.set("route_status", options.routeStatus.trim());
  }
  if (options.upstreamStatus?.trim()) {
    params.set("upstream_status", options.upstreamStatus.trim());
  }
  if (options.protocol?.trim()) {
    params.set("protocol", options.protocol.trim());
  }
  if (options.sort?.trim()) {
    params.set("sort", options.sort.trim());
  }
  const data = await authedRequest<PagePayload<AdminLLMUpstreamModelDTO>>(
    `/api/v1/admin/llm/upstreams/${upstreamID}/models?${params.toString()}`,
    { accessToken },
    true,
  );
  return normalizeAdminPagePayload(data);
}

export async function upsertAdminLLMUpstreamModel(
  accessToken: string,
  upstreamID: number,
  payload: UpsertAdminLLMUpstreamModelRequest,
): Promise<AdminLLMUpstreamModelData> {
  return authedRequest<AdminLLMUpstreamModelData>(
    `/api/v1/admin/llm/upstreams/${upstreamID}/models`,
    { method: "POST", accessToken, body: payload },
    true,
  );
}

export async function deleteAdminLLMUpstreamModel(
  accessToken: string,
  upstreamID: number,
  routeID: number,
): Promise<void> {
  return authedRequest<void>(
    `/api/v1/admin/llm/upstreams/${upstreamID}/models/${routeID}`,
    { method: "DELETE", accessToken },
    true,
  );
}

export async function disableAdminLLMUpstreamModel(
  accessToken: string,
  upstreamID: number,
  routeID: number,
): Promise<AdminLLMUpstreamModelData> {
  return authedRequest<AdminLLMUpstreamModelData>(
    `/api/v1/admin/llm/upstreams/${upstreamID}/models/${routeID}/disable`,
    { method: "PATCH", accessToken },
    true,
  );
}

export async function enableAdminLLMUpstreamModel(
  accessToken: string,
  upstreamID: number,
  routeID: number,
): Promise<AdminLLMUpstreamModelData> {
  return authedRequest<AdminLLMUpstreamModelData>(
    `/api/v1/admin/llm/upstreams/${upstreamID}/models/${routeID}/enable`,
    { method: "PATCH", accessToken },
    true,
  );
}

export async function batchDeleteAdminLLMUpstreamModels(
  accessToken: string,
  upstreamID: number,
  payload: AdminBatchDeleteRequest,
): Promise<AdminBatchDeleteData> {
  return authedRequest<AdminBatchDeleteData>(
    `/api/v1/admin/llm/upstreams/${upstreamID}/models/batch-delete`,
    { method: "POST", accessToken, body: payload },
    true,
  );
}

export async function openAdminLLMUpstreamModelCircuit(
  accessToken: string,
  upstreamID: number,
  routeID: number,
): Promise<void> {
  return authedRequest<void>(
    `/api/v1/admin/llm/upstreams/${upstreamID}/models/${routeID}/circuit/open`,
    { method: "POST", accessToken },
    true,
  );
}

export async function resetAdminLLMUpstreamModelCircuit(
  accessToken: string,
  upstreamID: number,
  routeID: number,
): Promise<ResetAdminLLMCircuitData> {
  return authedRequest<ResetAdminLLMCircuitData>(
    `/api/v1/admin/llm/upstreams/${upstreamID}/models/${routeID}/circuit/reset`,
    { method: "POST", accessToken },
    true,
  );
}

// ---------------------------------------------------------------------------
// Remote model discovery
// ---------------------------------------------------------------------------

export async function listAdminLLMRemoteModels(
  accessToken: string,
  upstreamID: number,
  signal?: AbortSignal,
): Promise<ListAdminLLMRemoteModelsData> {
  return authedRequest<ListAdminLLMRemoteModelsData>(
    `/api/v1/admin/llm/upstreams/${upstreamID}/models/remote`,
    { accessToken, signal },
    true,
  );
}

export async function syncAdminLLMUpstreamModels(
  accessToken: string,
  upstreamID: number,
  options: { allowEmpty?: boolean; expectedSnapshot?: string; signal?: AbortSignal } = {},
): Promise<SyncAdminLLMUpstreamModelsData> {
  const params = new URLSearchParams();
  if (options.allowEmpty) {
    params.set("allow_empty", "true");
  }
  if (options.expectedSnapshot) {
    params.set("expected_snapshot", options.expectedSnapshot);
  }
  const query = params.size > 0 ? `?${params.toString()}` : "";
  return authedRequest<SyncAdminLLMUpstreamModelsData>(
    `/api/v1/admin/llm/upstreams/${upstreamID}/models/sync${query}`,
    { method: "POST", accessToken, signal: options.signal },
    true,
  );
}

export async function importAdminLLMUpstreamModels(
  accessToken: string,
  upstreamID: number,
  payload: ImportAdminLLMUpstreamModelsRequest,
  signal?: AbortSignal,
): Promise<ImportAdminLLMUpstreamModelsData> {
  return authedRequest<ImportAdminLLMUpstreamModelsData>(
    `/api/v1/admin/llm/upstreams/${upstreamID}/models/import`,
    { method: "POST", accessToken, body: payload, signal },
    true,
  );
}

// ---------------------------------------------------------------------------
// Model CRUD
// ---------------------------------------------------------------------------

export async function listAdminLLMModels(
  accessToken: string,
  options: ListAdminLLMModelsOptions = {},
): Promise<PagePayload<AdminLLMModelDTO>> {
  const { page, pageSize } = resolveAdminPage(options);
  const params = new URLSearchParams({
    page: String(page),
    page_size: String(pageSize),
    only_active: options.onlyActive ? "true" : "false",
  });
  if (options.onlyAvailable) {
    params.set("only_available", "true");
  }
  if (options.query?.trim()) {
    params.set("q", options.query.trim());
  }
  if (options.status?.trim()) {
    params.set("status", options.status.trim());
  }
  if (options.vendor?.trim()) {
    params.set("vendor", options.vendor.trim());
  }
  if (options.protocol?.trim()) {
    params.set("protocol", options.protocol.trim());
  }
  if (options.upstream?.trim()) {
    params.set("upstream", options.upstream.trim());
  }
  if (options.sort?.trim()) {
    params.set("sort", options.sort.trim());
  }
  const data = await authedRequest<PagePayload<AdminLLMModelDTO>>(
    `/api/v1/admin/llm/models?${params.toString()}`,
    { accessToken },
    true,
  );
  return normalizeAdminPagePayload(data);
}

export async function createAdminLLMModel(
  accessToken: string,
  payload: CreateAdminLLMModelRequest,
): Promise<AdminLLMModelData> {
  return authedRequest<AdminLLMModelData>(
    "/api/v1/admin/llm/models",
    { method: "POST", accessToken, body: payload },
    true,
  );
}

export async function updateAdminLLMModel(
  accessToken: string,
  modelID: number,
  payload: UpdateAdminLLMModelRequest,
): Promise<AdminLLMModelData> {
  return authedRequest<AdminLLMModelData>(
    `/api/v1/admin/llm/models/${modelID}`,
    { method: "PATCH", accessToken, body: payload },
    true,
  );
}

export async function setAdminLLMModelProtocols(
  accessToken: string,
  modelID: number,
  payload: SetAdminLLMModelProtocolsRequest,
): Promise<AdminLLMModelData> {
  return authedRequest<AdminLLMModelData>(
    `/api/v1/admin/llm/models/${modelID}/protocols`,
    { method: "PATCH", accessToken, body: payload },
    true,
  );
}

export async function reorderAdminLLMModels(
  accessToken: string,
  payload: ReorderAdminLLMModelsRequest,
): Promise<void> {
  return authedRequest<void>(
    "/api/v1/admin/llm/models/order",
    { method: "POST", accessToken, body: payload },
    true,
  );
}

export async function setAdminLLMModelsDisplayGroup(
  accessToken: string,
  payload: SetAdminLLMModelsDisplayGroupRequest,
): Promise<void> {
  return authedRequest<void>(
    "/api/v1/admin/llm/models/display-group",
    { method: "PATCH", accessToken, body: payload },
    true,
  );
}

export async function listAdminLLMModelVendors(
  accessToken: string,
  options: AdminListQueryOptions = {},
): Promise<PagePayload<AdminLLMModelVendorDTO>> {
  const { page, pageSize } = resolveAdminPage(options);
  const params = new URLSearchParams({ page: String(page), page_size: String(pageSize) });
  if (options.query?.trim()) {
    params.set("q", options.query.trim());
  }
  const data = await authedRequest<PagePayload<AdminLLMModelVendorDTO>>(
    `/api/v1/admin/llm/model-vendors?${params.toString()}`,
    { accessToken },
    true,
  );
  return normalizeAdminPagePayload(data);
}

export async function createAdminLLMModelVendor(
  accessToken: string,
  payload: CreateAdminLLMModelVendorRequest,
): Promise<AdminLLMModelVendorData> {
  return authedRequest<AdminLLMModelVendorData>(
    "/api/v1/admin/llm/model-vendors",
    { method: "POST", accessToken, body: payload },
    true,
  );
}

export async function updateAdminLLMModelVendor(
  accessToken: string,
  vendorKey: string,
  payload: UpdateAdminLLMModelVendorRequest,
): Promise<AdminLLMModelVendorData> {
  return authedRequest<AdminLLMModelVendorData>(
    `/api/v1/admin/llm/model-vendors/${pathParam(vendorKey)}`,
    { method: "PATCH", accessToken, body: payload },
    true,
  );
}

export async function deleteAdminLLMModelVendor(
  accessToken: string,
  vendorKey: string,
): Promise<void> {
  return authedRequest<void>(
    `/api/v1/admin/llm/model-vendors/${pathParam(vendorKey)}`,
    { method: "DELETE", accessToken },
    true,
  );
}

export async function uploadAdminLLMModelIcon(
  accessToken: string,
  file: File,
): Promise<AdminLLMModelIconAsset> {
  const form = new FormData();
  form.append("file", file);
  return authedRequest<AdminLLMModelIconAsset>(
    "/api/v1/admin/llm/icon-assets",
    { method: "POST", accessToken, body: form },
    true,
  );
}

export async function listAdminLLMModelIcons(
  accessToken: string,
  options: AdminListQueryOptions = {},
): Promise<PagePayload<AdminLLMModelIconAssetListItem>> {
  const { page, pageSize } = resolveAdminPage(options);
  const params = new URLSearchParams({ page: String(page), page_size: String(pageSize) });
  const data = await authedRequest<PagePayload<AdminLLMModelIconAssetListItem>>(
    `/api/v1/admin/llm/icon-assets?${params.toString()}`,
    { accessToken },
    true,
  );
  return normalizeAdminPagePayload(data);
}

export async function deleteAdminLLMModelIcon(
  accessToken: string,
  publicID: string,
): Promise<void> {
  return authedRequest<void>(
    `/api/v1/admin/llm/icon-assets/${pathParam(publicID)}`,
    { method: "DELETE", accessToken },
    true,
  );
}

export async function listAdminLLMModelDisplayGroups(
  accessToken: string,
  options: AdminListQueryOptions = {},
): Promise<PagePayload<AdminLLMModelDisplayGroupDTO>> {
  const { page, pageSize } = resolveAdminPage(options);
  const params = new URLSearchParams({ page: String(page), page_size: String(pageSize) });
  if (options.query?.trim()) {
    params.set("q", options.query.trim());
  }
  const data = await authedRequest<PagePayload<AdminLLMModelDisplayGroupDTO>>(
    `/api/v1/admin/llm/model-display-groups?${params.toString()}`,
    { accessToken },
    true,
  );
  return normalizeAdminPagePayload(data);
}

export async function createAdminLLMModelDisplayGroup(
  accessToken: string,
  payload: CreateAdminLLMModelDisplayGroupRequest,
): Promise<AdminLLMModelDisplayGroupData> {
  return authedRequest<AdminLLMModelDisplayGroupData>(
    "/api/v1/admin/llm/model-display-groups",
    { method: "POST", accessToken, body: payload },
    true,
  );
}

export async function updateAdminLLMModelDisplayGroup(
  accessToken: string,
  groupID: number,
  payload: UpdateAdminLLMModelDisplayGroupRequest,
): Promise<AdminLLMModelDisplayGroupData> {
  return authedRequest<AdminLLMModelDisplayGroupData>(
    `/api/v1/admin/llm/model-display-groups/${groupID}`,
    { method: "PATCH", accessToken, body: payload },
    true,
  );
}

export async function deleteAdminLLMModelDisplayGroup(
  accessToken: string,
  groupID: number,
): Promise<void> {
  return authedRequest<void>(
    `/api/v1/admin/llm/model-display-groups/${groupID}`,
    { method: "DELETE", accessToken },
    true,
  );
}

export async function deleteAdminLLMModel(
  accessToken: string,
  modelID: number,
): Promise<void> {
  return authedRequest<void>(
    `/api/v1/admin/llm/models/${modelID}`,
    { method: "DELETE", accessToken },
    true,
  );
}

export async function batchDeleteAdminLLMModels(
  accessToken: string,
  payload: AdminBatchDeleteRequest,
): Promise<AdminBatchDeleteData> {
  return authedRequest<AdminBatchDeleteData>(
    "/api/v1/admin/llm/models/batch-delete",
    { method: "POST", accessToken, body: payload },
    true,
  );
}

export async function testAdminLLMModel(
  accessToken: string,
  modelID: number,
): Promise<AdminLLMModelProbeData> {
  return authedRequest<AdminLLMModelProbeData>(
    `/api/v1/admin/llm/models/${modelID}/test`,
    { method: "POST", accessToken, body: {} },
    true,
  );
}

export async function testAdminLLMModelAll(
  accessToken: string,
  modelID: number,
): Promise<AdminLLMModelProbeBatchData> {
  return authedRequest<AdminLLMModelProbeBatchData>(
    `/api/v1/admin/llm/models/${modelID}/test-all`,
    { method: "POST", accessToken, body: {} },
    true,
  );
}

// ---------------------------------------------------------------------------
// Model upstream sources
// ---------------------------------------------------------------------------

export async function listAdminLLMModelUpstreamSources(
  accessToken: string,
  modelID: number,
  options: AdminPageOptions = {},
): Promise<PagePayload<AdminLLMModelUpstreamSourceDTO>> {
  const { page, pageSize } = resolveAdminPage(options);
  const data = await authedRequest<PagePayload<AdminLLMModelUpstreamSourceDTO>>(
    `/api/v1/admin/llm/models/${modelID}/sources?page=${page}&page_size=${pageSize}`,
    { accessToken },
    true,
  );
  return normalizeAdminPagePayload(data);
}

export async function bindAdminLLMModelUpstreamSource(
  accessToken: string,
  modelID: number,
  payload: BindAdminLLMModelUpstreamSourceRequest,
): Promise<AdminLLMModelUpstreamSourceData> {
  return authedRequest<AdminLLMModelUpstreamSourceData>(
    `/api/v1/admin/llm/models/${modelID}/sources`,
    { method: "POST", accessToken, body: payload },
    true,
  );
}

export async function updateAdminLLMModelUpstreamSource(
  accessToken: string,
  modelID: number,
  routeID: number,
  payload: UpdateAdminLLMModelUpstreamSourceRequest,
): Promise<AdminLLMModelUpstreamSourceData> {
  return authedRequest<AdminLLMModelUpstreamSourceData>(
    `/api/v1/admin/llm/models/${modelID}/sources/${routeID}`,
    { method: "PATCH", accessToken, body: payload },
    true,
  );
}

export async function testAdminLLMUpstreamModelRoute(
  accessToken: string,
  upstreamID: number,
  routeID: number,
): Promise<AdminLLMModelProbeData> {
  return authedRequest<AdminLLMModelProbeData>(
    `/api/v1/admin/llm/upstreams/${upstreamID}/models/${routeID}/test`,
    { method: "POST", accessToken, body: {} },
    true,
  );
}

// ---------------------------------------------------------------------------
// Global settings
// ---------------------------------------------------------------------------

export async function listAdminLLMSettings(
  accessToken: string,
): Promise<AdminLLMSetting[]> {
  return authedRequest<AdminLLMSetting[]>(
    "/api/v1/admin/llm/settings",
    { accessToken },
    true,
  );
}

export async function updateAdminLLMSetting(
  accessToken: string,
  key: string,
  value: string,
): Promise<AdminLLMSetting> {
  return authedRequest<AdminLLMSetting>(
    `/api/v1/admin/llm/settings/${pathParam(key)}`,
    { method: "PATCH", accessToken, body: { value } },
    true,
  );
}
