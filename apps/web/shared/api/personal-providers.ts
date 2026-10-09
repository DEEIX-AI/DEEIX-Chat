import { authedRequest } from "@/shared/api/authed-client";
import { pathParam } from "@/shared/api/http-client";
import { isRecord, isString, isStringArray } from "@/shared/lib/type-guards";
import type {
  AdminPersonalProviderPage,
  CreatePersonalProviderPayload,
  PersonalProviderAccessDTO,
  PersonalProviderAffectedData,
  PersonalProviderAvailableModelDTO,
  PersonalProviderDTO,
  PersonalProviderData,
  PersonalProviderDeleteData,
  PersonalProviderListData,
  PersonalProviderModelDTO,
  PersonalProviderModelsData,
  PersonalProviderProbePayload,
  UpdatePersonalProviderPayload,
} from "@/shared/api/personal-providers-types";

const BASE_PATH = "/api/v1/me/model-providers";

// A server from before per-model protocols (the desktop app can point at one) sends model names as
// plain strings and no model protocol list. Responses are normalised here so the rest of the client
// sees one shape: such a model runs on its provider's protocol.

function normalizeModel(entry: unknown, providerProtocol: string): PersonalProviderModelDTO[] {
  if (isString(entry)) return entry ? [{ name: entry, protocols: [providerProtocol] }] : [];
  if (!isRecord(entry) || !isString(entry.name) || !entry.name) return [];
  const protocols = isStringArray(entry.protocols) && entry.protocols.length > 0 ? entry.protocols : [providerProtocol];
  return [{ name: entry.name, protocols }];
}

function normalizeProvider(provider: PersonalProviderDTO): PersonalProviderDTO {
  const models: unknown[] = Array.isArray(provider.models) ? provider.models : [];
  return { ...provider, models: models.flatMap((entry) => normalizeModel(entry, provider.protocol)) };
}

function normalizeAvailableModels(data: PersonalProviderModelsData, providerProtocol: string): PersonalProviderModelsData {
  const models: unknown[] = Array.isArray(data.models) ? data.models : [];
  return {
    models: models.flatMap((entry): PersonalProviderAvailableModelDTO[] => {
      if (isString(entry)) return entry ? [{ name: entry, suggestedProtocols: [providerProtocol] }] : [];
      if (!isRecord(entry) || !isString(entry.name) || !entry.name) return [];
      const suggested = isStringArray(entry.suggestedProtocols) && entry.suggestedProtocols.length > 0 ? entry.suggestedProtocols : [providerProtocol];
      return [{ name: entry.name, suggestedProtocols: suggested }];
    }),
  };
}

/** `modelProtocols` is empty on a server without per-model protocols; callers then offer no choice. */
export async function getPersonalProviderAccess(accessToken: string, signal?: AbortSignal): Promise<PersonalProviderAccessDTO> {
  const access = await authedRequest<PersonalProviderAccessDTO>(`${BASE_PATH}/access`, { accessToken, signal }, true);
  return {
    ...access,
    protocols: isStringArray(access.protocols) ? access.protocols : [],
    modelProtocols: isStringArray(access.modelProtocols) ? access.modelProtocols : [],
  };
}

export async function listPersonalProviders(accessToken: string, signal?: AbortSignal): Promise<PersonalProviderListData> {
  const data = await authedRequest<PersonalProviderListData>(BASE_PATH, { accessToken, signal }, true);
  return { providers: (data.providers ?? []).map(normalizeProvider) };
}

/** Fetches the provider's model list with a candidate key; nothing is saved. */
export async function probePersonalProvider(
  accessToken: string,
  payload: PersonalProviderProbePayload,
): Promise<PersonalProviderModelsData> {
  const data = await authedRequest<PersonalProviderModelsData>(`${BASE_PATH}/probe`, { method: "POST", accessToken, body: payload }, true);
  return normalizeAvailableModels(data, payload.protocol);
}

export async function createPersonalProvider(
  accessToken: string,
  payload: CreatePersonalProviderPayload,
): Promise<PersonalProviderData> {
  const data = await authedRequest<PersonalProviderData>(BASE_PATH, { method: "POST", accessToken, body: payload }, true);
  return { provider: normalizeProvider(data.provider) };
}

export async function updatePersonalProvider(
  accessToken: string,
  id: string,
  payload: UpdatePersonalProviderPayload,
): Promise<PersonalProviderData> {
  const data = await authedRequest<PersonalProviderData>(
    `${BASE_PATH}/${pathParam(id)}`,
    { method: "PATCH", accessToken, body: payload },
    true,
  );
  return { provider: normalizeProvider(data.provider) };
}

export function deletePersonalProvider(accessToken: string, id: string): Promise<PersonalProviderDeleteData> {
  return authedRequest<PersonalProviderDeleteData>(`${BASE_PATH}/${pathParam(id)}`, { method: "DELETE", accessToken }, true);
}

/** Re-fetches the provider's model list with the saved key; `providerProtocol` is the fallback suggestion. */
export async function listPersonalProviderModels(accessToken: string, id: string, providerProtocol: string): Promise<PersonalProviderModelsData> {
  const data = await authedRequest<PersonalProviderModelsData>(`${BASE_PATH}/${pathParam(id)}/models`, { accessToken }, true);
  return normalizeAvailableModels(data, providerProtocol);
}

type AdminPersonalProviderListOptions = {
  query?: string;
  status?: string;
  page?: number;
  pageSize?: number;
};

export function listAdminPersonalProviders(
  accessToken: string,
  options: AdminPersonalProviderListOptions = {},
  signal?: AbortSignal,
): Promise<AdminPersonalProviderPage> {
  const params = new URLSearchParams({
    page: String(options.page ?? 1),
    page_size: String(options.pageSize ?? 20),
  });
  if (options.query?.trim()) params.set("q", options.query.trim());
  if (options.status) params.set("status", options.status);
  return authedRequest<AdminPersonalProviderPage>(`/api/v1/admin/model-providers?${params.toString()}`, { accessToken, signal }, true);
}

export function setAdminPersonalProvidersSuspended(
  accessToken: string,
  ids: string[],
  suspended: boolean,
): Promise<PersonalProviderAffectedData> {
  return authedRequest<PersonalProviderAffectedData>(
    "/api/v1/admin/model-providers/suspend",
    { method: "POST", accessToken, body: { ids, suspended } },
    true,
  );
}

export function suspendAdminPersonalProviderHost(accessToken: string, host: string): Promise<PersonalProviderAffectedData> {
  return authedRequest<PersonalProviderAffectedData>(
    "/api/v1/admin/model-providers/suspend-host",
    { method: "POST", accessToken, body: { host } },
    true,
  );
}

export function deleteAdminPersonalProviders(accessToken: string, ids: string[]): Promise<PersonalProviderAffectedData> {
  return authedRequest<PersonalProviderAffectedData>(
    "/api/v1/admin/model-providers/batch-delete",
    { method: "POST", accessToken, body: { ids } },
    true,
  );
}
