import { authedFetch, authedRequest } from "@/shared/api/authed-client";
import type { PagePayload } from "@/shared/api/common.types";
import { readFileContentResponse, type FileContentResult } from "@/shared/api/file";
import { pathParam } from "@/shared/api/http-client";
import type {
  AddKnowledgeBaseFilesRequest,
  KnowledgeBaseDTO,
  KnowledgeBaseData,
  KnowledgeBaseDeleteData,
  KnowledgeBaseFileData,
  KnowledgeBaseFileDTO,
  KnowledgeBaseFileMutationData,
  KnowledgeBaseFilePage,
  KnowledgeBaseFileProcessingSnapshotDTO,
  KnowledgeBaseFileProcessingStatusDTO,
  KnowledgeBasePage,
  PatchMyKnowledgeBaseRequest,
  PatchKnowledgeBaseRequest,
  WriteMyKnowledgeBaseRequest,
  WriteKnowledgeBaseRequest,
} from "@/shared/api/knowledge-bases.types";

type KnowledgeBaseListOptions = {
  query?: string;
  sort?: "default" | "name" | "created" | "updated" | "files";
  ids?: string[];
  enabled?: boolean;
  page?: number;
  pageSize?: number;
};

type DeleteKnowledgeBaseOptions = {
  deleteFiles?: boolean;
};

function listPath(basePath: string, options: KnowledgeBaseListOptions = {}): string {
  const params = new URLSearchParams({
    page: String(options.page ?? 1),
    page_size: String(options.pageSize ?? 50),
  });
  if (options.query?.trim()) params.set("q", options.query.trim());
  if (options.sort && options.sort !== "default") params.set("sort", options.sort);
  for (const id of options.ids ?? []) {
    if (id.trim()) params.append("id", id.trim());
  }
  if (typeof options.enabled === "boolean") params.set("enabled", String(options.enabled));
  return `${basePath}?${params.toString()}`;
}

function normalizePage(data: PagePayload<KnowledgeBaseDTO>): KnowledgeBasePage {
  return { results: data.results ?? [], total: data.total ?? 0 };
}

export async function listVisibleKnowledgeBases(
  accessToken: string,
  options: KnowledgeBaseListOptions = {},
  signal?: AbortSignal,
): Promise<KnowledgeBasePage> {
  const data = await authedRequest<PagePayload<KnowledgeBaseDTO>>(
    listPath("/api/v1/knowledge-bases", options),
    { accessToken, signal },
    true,
  );
  return normalizePage(data);
}

export async function getKnowledgeBase(
  accessToken: string,
  id: string,
  admin = false,
  signal?: AbortSignal,
): Promise<KnowledgeBaseDTO> {
  const basePath = admin ? "/api/v1/admin/knowledge-bases" : "/api/v1/knowledge-bases";
  const data = await authedRequest<KnowledgeBaseData>(
    `${basePath}/${pathParam(id)}`,
    { accessToken, signal },
    true,
  );
  return data.knowledgeBase;
}

export async function createMyKnowledgeBase(accessToken: string, payload: WriteMyKnowledgeBaseRequest): Promise<KnowledgeBaseData> {
  return authedRequest<KnowledgeBaseData>("/api/v1/knowledge-bases/mine", { method: "POST", accessToken, body: payload }, true);
}

export async function updateMyKnowledgeBase(accessToken: string, id: string, payload: PatchMyKnowledgeBaseRequest): Promise<KnowledgeBaseData> {
  return authedRequest<KnowledgeBaseData>(`/api/v1/knowledge-bases/mine/${pathParam(id)}`, { method: "PATCH", accessToken, body: payload }, true);
}

export async function deleteMyKnowledgeBase(accessToken: string, id: string, options: DeleteKnowledgeBaseOptions = {}): Promise<KnowledgeBaseDeleteData> {
  const params = new URLSearchParams();
  if (options.deleteFiles) params.set("delete_files", "true");
  const query = params.size > 0 ? `?${params.toString()}` : "";
  return authedRequest<KnowledgeBaseDeleteData>(`/api/v1/knowledge-bases/mine/${pathParam(id)}${query}`, { method: "DELETE", accessToken }, true);
}

export async function listKnowledgeBaseFiles(
  accessToken: string,
  id: string,
  page = 1,
  pageSize = 100,
  signal?: AbortSignal,
): Promise<KnowledgeBaseFilePage> {
  const params = new URLSearchParams({ page: String(page), page_size: String(pageSize) });
  const data = await authedRequest<PagePayload<KnowledgeBaseFileDTO>>(
    `/api/v1/knowledge-bases/${pathParam(id)}/files?${params.toString()}`,
    { accessToken, signal },
    true,
  );
  return { results: data.results ?? [], total: data.total ?? 0 };
}

export async function getKnowledgeBaseFileProcessingStatuses(
  accessToken: string,
  id: string,
  fileIDs: string[],
  admin = false,
  signal?: AbortSignal,
): Promise<KnowledgeBaseFileProcessingStatusDTO[]> {
  if (fileIDs.length === 0) return [];
  const basePath = admin ? "/api/v1/admin/knowledge-bases" : "/api/v1/knowledge-bases";
  const batches = Array.from(
    { length: Math.ceil(fileIDs.length / 100) },
    (_, index) => fileIDs.slice(index * 100, (index + 1) * 100),
  );
  const statuses = await Promise.all(batches.map((batch) =>
    authedRequest<KnowledgeBaseFileProcessingStatusDTO[]>(
      `${basePath}/${pathParam(id)}/files/processing/statuses`,
      {
        method: "POST",
        accessToken,
        body: { fileIDs: batch },
        signal,
      },
      true,
    ),
  ));
  return statuses.flat();
}

export async function getKnowledgeBaseFileProcessingSnapshot(
  accessToken: string,
  id: string,
  fileIDs: string[],
  admin = false,
  signal?: AbortSignal,
): Promise<KnowledgeBaseFileProcessingSnapshotDTO> {
  const basePath = admin ? "/api/v1/admin/knowledge-bases" : "/api/v1/knowledge-bases";
  return authedRequest<KnowledgeBaseFileProcessingSnapshotDTO>(
    `${basePath}/${pathParam(id)}/files/processing/snapshot`,
    {
      method: "POST",
      accessToken,
      body: { fileIDs: fileIDs.slice(0, 100) },
      signal,
    },
    true,
  );
}

export async function listAvailableMyKnowledgeBaseFiles(
  accessToken: string,
  id: string,
  options: KnowledgeBaseListOptions = {},
  signal?: AbortSignal,
): Promise<KnowledgeBaseFilePage> {
  const data = await authedRequest<PagePayload<KnowledgeBaseFileDTO>>(
    listPath(`/api/v1/knowledge-bases/mine/${pathParam(id)}/available-files`, options),
    { accessToken, signal },
    true,
  );
  return { results: data.results ?? [], total: data.total ?? 0 };
}

export async function fetchKnowledgeBaseFileContent(
  accessToken: string,
  id: string,
  fileID: string,
  admin = false,
  signal?: AbortSignal,
): Promise<FileContentResult> {
  const basePath = admin ? "/api/v1/admin/knowledge-bases" : "/api/v1/knowledge-bases";
  const response = await authedFetch(
    `${basePath}/${pathParam(id)}/files/${pathParam(fileID)}/content`,
    { method: "GET", accessToken, cache: "no-store", signal },
    true,
  );
  return readFileContentResponse(response);
}

export async function addMyKnowledgeBaseFiles(accessToken: string, id: string, payload: AddKnowledgeBaseFilesRequest): Promise<KnowledgeBaseFileMutationData> {
  return authedRequest<KnowledgeBaseFileMutationData>(`/api/v1/knowledge-bases/mine/${pathParam(id)}/files`, { method: "POST", accessToken, body: payload }, true);
}

export async function removeMyKnowledgeBaseFile(accessToken: string, id: string, fileID: string): Promise<KnowledgeBaseFileMutationData> {
  return authedRequest<KnowledgeBaseFileMutationData>(
    `/api/v1/knowledge-bases/mine/${pathParam(id)}/files/${pathParam(fileID)}`,
    { method: "DELETE", accessToken },
    true,
  );
}

export async function listAdminKnowledgeBases(
  accessToken: string,
  options: KnowledgeBaseListOptions = {},
  signal?: AbortSignal,
): Promise<KnowledgeBasePage> {
  const data = await authedRequest<PagePayload<KnowledgeBaseDTO>>(
    listPath("/api/v1/admin/knowledge-bases", options),
    { accessToken, signal },
    true,
  );
  return normalizePage(data);
}

export async function createAdminKnowledgeBase(accessToken: string, payload: WriteKnowledgeBaseRequest): Promise<KnowledgeBaseData> {
  return authedRequest<KnowledgeBaseData>("/api/v1/admin/knowledge-bases", { method: "POST", accessToken, body: payload }, true);
}

export async function uploadAdminKnowledgeBaseFile(
  accessToken: string,
  file: File,
  signal?: AbortSignal,
): Promise<KnowledgeBaseFileData> {
  const formData = new FormData();
  formData.append("file", file);
  return authedRequest<KnowledgeBaseFileData>(
    "/api/v1/admin/knowledge-bases/files",
    { method: "POST", accessToken, body: formData, signal },
    true,
  );
}

export async function listAdminPlatformFiles(
  accessToken: string,
  options: KnowledgeBaseListOptions = {},
  signal?: AbortSignal,
): Promise<KnowledgeBaseFilePage> {
  const data = await authedRequest<PagePayload<KnowledgeBaseFileDTO>>(
    listPath("/api/v1/admin/knowledge-bases/files", options),
    { accessToken, signal },
    true,
  );
  return { results: data.results ?? [], total: data.total ?? 0 };
}

export async function fetchAdminPlatformFileContent(
  accessToken: string,
  fileID: string,
  signal?: AbortSignal,
): Promise<FileContentResult> {
  const response = await authedFetch(
    `/api/v1/admin/knowledge-bases/files/${pathParam(fileID)}/content`,
    { method: "GET", accessToken, cache: "no-store", signal },
    true,
  );
  return readFileContentResponse(response);
}

export async function deleteAdminKnowledgeBaseFile(accessToken: string, fileID: string): Promise<void> {
  await authedRequest<{ deleted: boolean }>(
    `/api/v1/admin/knowledge-bases/files/${pathParam(fileID)}`,
    { method: "DELETE", accessToken },
    true,
  );
}

export async function updateAdminKnowledgeBase(accessToken: string, id: string, payload: PatchKnowledgeBaseRequest): Promise<KnowledgeBaseData> {
  return authedRequest<KnowledgeBaseData>(`/api/v1/admin/knowledge-bases/${pathParam(id)}`, { method: "PATCH", accessToken, body: payload }, true);
}

export async function deleteAdminKnowledgeBase(accessToken: string, id: string, options: DeleteKnowledgeBaseOptions = {}): Promise<KnowledgeBaseDeleteData> {
  const params = new URLSearchParams();
  if (options.deleteFiles) params.set("delete_files", "true");
  const query = params.size > 0 ? `?${params.toString()}` : "";
  return authedRequest<KnowledgeBaseDeleteData>(`/api/v1/admin/knowledge-bases/${pathParam(id)}${query}`, { method: "DELETE", accessToken }, true);
}

export async function listAdminKnowledgeBaseFiles(
  accessToken: string,
  id: string,
  page = 1,
  pageSize = 100,
  signal?: AbortSignal,
): Promise<KnowledgeBaseFilePage> {
  const params = new URLSearchParams({ page: String(page), page_size: String(pageSize) });
  const data = await authedRequest<PagePayload<KnowledgeBaseFileDTO>>(
    `/api/v1/admin/knowledge-bases/${pathParam(id)}/files?${params.toString()}`,
    { accessToken, signal },
    true,
  );
  return { results: data.results ?? [], total: data.total ?? 0 };
}

export async function listAvailableAdminKnowledgeBaseFiles(
  accessToken: string,
  id: string,
  options: KnowledgeBaseListOptions = {},
  signal?: AbortSignal,
): Promise<KnowledgeBaseFilePage> {
  const data = await authedRequest<PagePayload<KnowledgeBaseFileDTO>>(
    listPath(`/api/v1/admin/knowledge-bases/${pathParam(id)}/available-files`, options),
    { accessToken, signal },
    true,
  );
  return { results: data.results ?? [], total: data.total ?? 0 };
}

export async function addAdminKnowledgeBaseFiles(accessToken: string, id: string, payload: AddKnowledgeBaseFilesRequest): Promise<KnowledgeBaseFileMutationData> {
  return authedRequest<KnowledgeBaseFileMutationData>(`/api/v1/admin/knowledge-bases/${pathParam(id)}/files`, { method: "POST", accessToken, body: payload }, true);
}

export async function removeAdminKnowledgeBaseFile(accessToken: string, id: string, fileID: string): Promise<KnowledgeBaseFileMutationData> {
  return authedRequest<KnowledgeBaseFileMutationData>(
    `/api/v1/admin/knowledge-bases/${pathParam(id)}/files/${pathParam(fileID)}`,
    { method: "DELETE", accessToken },
    true,
  );
}
