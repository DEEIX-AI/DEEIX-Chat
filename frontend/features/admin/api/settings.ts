import { authedRequest, authedFetch } from "@/shared/api/authed-client";
import { pathParam } from "@/shared/api/http-client";
import type {
  AdminDoclingRuntimeView,
  AdminEmbeddingRuntimeView,
  AdminMinerURuntimeView,
  AdminRapidOCRRuntimeView,
  AdminTesseractRuntimeView,
  AdminTikaRuntimeView,
} from "@/features/admin/api/admin.types";
import type { PatchSettingsRequest, SettingItem, SettingsGrouped } from "@/shared/api/settings.types";

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

export interface AdminEmbeddingIndexStatus {
  modelSignature: string;
  readyCount: number;
  staleCount: number;
  pendingCount: number;
  failedCount: number;
  needsReindex: boolean;
}

export async function getAdminEmbeddingStatus(accessToken: string): Promise<AdminEmbeddingIndexStatus> {
  return authedRequest<AdminEmbeddingIndexStatus>(
    "/api/v1/admin/settings/embedding/status",
    { accessToken },
    true,
  );
}

export async function triggerAdminEmbeddingReindex(accessToken: string): Promise<{ submitted: number; message: string }> {
  return authedRequest<{ submitted: number; message: string }>(
    "/api/v1/admin/settings/embedding/reindex",
    { method: "POST", accessToken },
    true,
  );
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
