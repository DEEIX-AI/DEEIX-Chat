import { authedRequest } from "@/shared/api/authed-client";
import { pathParam } from "@/shared/api/http-client";
import type { PagePayload } from "@/shared/api/common.types";
import type {
  PatchPromptPresetRequest,
  PromptPresetDTO,
  PromptPresetData,
  PromptPresetDeleteData,
  PromptPresetPage,
  WritePromptPresetRequest,
} from "@/shared/api/prompt-presets.types";

type PromptPresetListOptions = {
  query?: string;
  enabled?: boolean;
  page?: number;
  pageSize?: number;
};

function promptPresetListPath(basePath: string, options: PromptPresetListOptions = {}): string {
  const params = new URLSearchParams({
    page: String(options.page ?? 1),
    page_size: String(options.pageSize ?? 50),
  });
  if (options.query?.trim()) params.set("q", options.query.trim());
  if (typeof options.enabled === "boolean") params.set("enabled", String(options.enabled));
  return `${basePath}?${params.toString()}`;
}

function normalizePagePayload(data: PagePayload<PromptPresetDTO>): PromptPresetPage {
  return {
    results: data.results ?? [],
    total: data.total ?? 0,
  };
}

export async function listVisiblePromptPresets(
  accessToken: string,
  options: PromptPresetListOptions = {},
): Promise<PromptPresetPage> {
  const data = await authedRequest<PagePayload<PromptPresetDTO>>(
    promptPresetListPath("/api/v1/prompt-presets", options),
    { accessToken },
    true,
  );
  return normalizePagePayload(data);
}

export async function listMyPromptPresets(
  accessToken: string,
  options: PromptPresetListOptions = {},
): Promise<PromptPresetPage> {
  const data = await authedRequest<PagePayload<PromptPresetDTO>>(
    promptPresetListPath("/api/v1/prompt-presets/mine", options),
    { accessToken },
    true,
  );
  return normalizePagePayload(data);
}

export async function createMyPromptPreset(
  accessToken: string,
  payload: WritePromptPresetRequest,
): Promise<PromptPresetData> {
  return authedRequest<PromptPresetData>(
    "/api/v1/prompt-presets/mine",
    { method: "POST", accessToken, body: payload },
    true,
  );
}

export async function updateMyPromptPreset(
  accessToken: string,
  id: number,
  payload: PatchPromptPresetRequest,
): Promise<PromptPresetData> {
  return authedRequest<PromptPresetData>(
    `/api/v1/prompt-presets/mine/${pathParam(id)}`,
    { method: "PATCH", accessToken, body: payload },
    true,
  );
}

export async function deleteMyPromptPreset(accessToken: string, id: number): Promise<PromptPresetDeleteData> {
  return authedRequest<PromptPresetDeleteData>(
    `/api/v1/prompt-presets/mine/${pathParam(id)}`,
    { method: "DELETE", accessToken },
    true,
  );
}

export async function listAdminPromptPresets(
  accessToken: string,
  options: PromptPresetListOptions = {},
  signal?: AbortSignal,
): Promise<PromptPresetPage> {
  const data = await authedRequest<PagePayload<PromptPresetDTO>>(
    promptPresetListPath("/api/v1/admin/prompt-presets", options),
    { accessToken, signal },
    true,
  );
  return normalizePagePayload(data);
}

export async function createAdminPromptPreset(
  accessToken: string,
  payload: WritePromptPresetRequest,
): Promise<PromptPresetData> {
  return authedRequest<PromptPresetData>(
    "/api/v1/admin/prompt-presets",
    { method: "POST", accessToken, body: payload },
    true,
  );
}

export async function updateAdminPromptPreset(
  accessToken: string,
  id: number,
  payload: PatchPromptPresetRequest,
): Promise<PromptPresetData> {
  return authedRequest<PromptPresetData>(
    `/api/v1/admin/prompt-presets/${pathParam(id)}`,
    { method: "PATCH", accessToken, body: payload },
    true,
  );
}

export async function deleteAdminPromptPreset(accessToken: string, id: number): Promise<PromptPresetDeleteData> {
  return authedRequest<PromptPresetDeleteData>(
    `/api/v1/admin/prompt-presets/${pathParam(id)}`,
    { method: "DELETE", accessToken },
    true,
  );
}
