import type {
  EmbeddingIndexStatusResponse,
  EmbeddingReindexResponse,
  EmbeddingTaskResponse,
  EmbeddingTaskRetryResponse,
} from "@deeix/api-contract";
import { authedRequest, authedFetch } from "@/shared/api/authed-client";
import type { PagePayload } from "@/shared/api/common-types";
import type { ADMIN_EMBEDDING_TASK_BUCKETS } from "@/features/admin/model/admin-unions";
import { pathParam } from "@/shared/api/http-client";
import type {
  AdminDoclingRuntimeView,
  AdminEmbeddingRuntimeView,
  AdminMinerURuntimeView,
  AdminRapidOCRRuntimeView,
  AdminTesseractRuntimeView,
  AdminTikaRuntimeView,
} from "@/features/admin/api/admin-types";
import type { PatchSettingsRequest, SettingItem, SettingsGrouped } from "@/shared/api/settings-types";

import { normalizeAdminPagePayload, resolveAdminPage, type AdminPageOptions } from "./shared";

export async function listAdminSettings(accessToken: string): Promise<SettingsGrouped> {
  return authedRequest<SettingsGrouped>(
    "/api/v1/admin/settings",
    { accessToken },
    true,
  );
}

export async function listAdminSettingsByNamespace(
  accessToken: string,
  namespace: string,
): Promise<SettingItem[]> {
  return authedRequest<SettingItem[]>(
    `/api/v1/admin/settings/${pathParam(namespace)}`,
    { accessToken },
    true,
  );
}

export async function patchAdminSettings(
  accessToken: string,
  payload: PatchSettingsRequest,
): Promise<SettingsGrouped> {
  return authedRequest<SettingsGrouped>(
    "/api/v1/admin/settings",
    {
      method: "PATCH",
      accessToken,
      body: payload,
    },
    true,
  );
}

export async function getAdminTikaRuntime(accessToken: string): Promise<AdminTikaRuntimeView> {
  return authedRequest<AdminTikaRuntimeView>(
    "/api/v1/admin/settings/tika/runtime",
    { accessToken },
    true,
  );
}

export async function getAdminDoclingRuntime(accessToken: string): Promise<AdminDoclingRuntimeView> {
  return authedRequest<AdminDoclingRuntimeView>(
    "/api/v1/admin/settings/docling/runtime",
    { accessToken },
    true,
  );
}

export async function getAdminTesseractRuntime(accessToken: string): Promise<AdminTesseractRuntimeView> {
  return authedRequest<AdminTesseractRuntimeView>(
    "/api/v1/admin/settings/tesseract/runtime",
    { accessToken },
    true,
  );
}

export async function getAdminRapidOCRRuntime(accessToken: string): Promise<AdminRapidOCRRuntimeView> {
  return authedRequest<AdminRapidOCRRuntimeView>(
    "/api/v1/admin/settings/rapidocr/runtime",
    { accessToken },
    true,
  );
}

export async function getAdminMinerURuntime(accessToken: string): Promise<AdminMinerURuntimeView> {
  return authedRequest<AdminMinerURuntimeView>(
    "/api/v1/admin/settings/mineru/runtime",
    { accessToken },
    true,
  );
}

export async function getAdminEmbeddingRuntime(accessToken: string): Promise<AdminEmbeddingRuntimeView> {
  return authedRequest<AdminEmbeddingRuntimeView>(
    "/api/v1/admin/settings/embedding/runtime",
    { accessToken },
    true,
  );
}

export type AdminEmbeddingIndexStatus = EmbeddingIndexStatusResponse;

export async function getAdminEmbeddingStatus(
  accessToken: string,
  signal?: AbortSignal,
): Promise<AdminEmbeddingIndexStatus> {
  return authedRequest<AdminEmbeddingIndexStatus>(
    "/api/v1/admin/settings/embedding/status",
    { accessToken, signal },
    true,
  );
}

export type AdminEmbeddingTaskBucket = (typeof ADMIN_EMBEDDING_TASK_BUCKETS)[number];
export type AdminEmbeddingTask = EmbeddingTaskResponse;
export type AdminEmbeddingTaskRetryResult = EmbeddingTaskRetryResponse;

type ListAdminEmbeddingTasksOptions = AdminPageOptions & {
  bucket: AdminEmbeddingTaskBucket;
  query?: string;
};

export async function listAdminEmbeddingTasks(
  accessToken: string,
  options: ListAdminEmbeddingTasksOptions,
  signal?: AbortSignal,
): Promise<PagePayload<AdminEmbeddingTask>> {
  const { page, pageSize } = resolveAdminPage(options);
  const params = new URLSearchParams();
  params.set("bucket", options.bucket);
  params.set("page", String(page));
  params.set("page_size", String(pageSize));
  if (options.query?.trim()) params.set("query", options.query.trim());
  const data = await authedRequest<PagePayload<AdminEmbeddingTask>>(
    `/api/v1/admin/settings/embedding/tasks?${params.toString()}`,
    { accessToken, signal },
    true,
  );
  return normalizeAdminPagePayload(data);
}

export async function retryAdminEmbeddingTasks(
  accessToken: string,
  fileIDs: string[],
): Promise<AdminEmbeddingTaskRetryResult> {
  return authedRequest<AdminEmbeddingTaskRetryResult>(
    "/api/v1/admin/settings/embedding/tasks/retry",
    { method: "POST", accessToken, body: { fileIDs } },
    true,
  );
}

export async function triggerAdminEmbeddingReindex(
  accessToken: string,
  options: { includeEmpty?: boolean } = {},
): Promise<EmbeddingReindexResponse> {
  const path = options.includeEmpty
    ? "/api/v1/admin/settings/embedding/reindex?include_empty=true"
    : "/api/v1/admin/settings/embedding/reindex";
  return authedRequest<EmbeddingReindexResponse>(path, { method: "POST", accessToken }, true);
}

export type AdminConversationExportFile = {
  blob: Blob;
  fileName: string;
};

function contentDispositionFileName(value: string | null): string | null {
  if (!value) {
    return null;
  }
  const utf8Match = value.match(/filename\*=UTF-8''([^;]+)/i);
  if (utf8Match?.[1]) {
    try {
      return decodeURIComponent(utf8Match[1]);
    } catch {
      return utf8Match[1];
    }
  }
  const asciiMatch = value.match(/filename="?([^";]+)"?/i);
  return asciiMatch?.[1] ?? null;
}

export async function exportAllConversations(accessToken: string): Promise<AdminConversationExportFile> {
  const response = await authedFetch("/api/v1/admin/conversations/export", { accessToken });
  if (!response.ok) {
    throw new Error(`export failed: ${response.status}`);
  }
  const timestamp = new Date().toISOString().replace(/[:.]/g, "-").slice(0, 19);
  return {
    blob: await response.blob(),
    fileName: contentDispositionFileName(response.headers.get("Content-Disposition")) ?? `conversations-export-${timestamp}.jsonl`,
  };
}
