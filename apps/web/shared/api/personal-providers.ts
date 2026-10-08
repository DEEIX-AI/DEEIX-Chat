import { authedRequest } from "@/shared/api/authed-client";
import { pathParam } from "@/shared/api/http-client";
import type {
  AdminPersonalProviderPage,
  CreatePersonalProviderPayload,
  PersonalProviderAccessDTO,
  PersonalProviderAffectedData,
  PersonalProviderData,
  PersonalProviderDeleteData,
  PersonalProviderListData,
  PersonalProviderModelsData,
  PersonalProviderProbePayload,
  UpdatePersonalProviderPayload,
} from "@/shared/api/personal-providers-types";

const BASE_PATH = "/api/v1/me/model-providers";

export function getPersonalProviderAccess(accessToken: string, signal?: AbortSignal): Promise<PersonalProviderAccessDTO> {
  return authedRequest<PersonalProviderAccessDTO>(`${BASE_PATH}/access`, { accessToken, signal }, true);
}

export function listPersonalProviders(accessToken: string, signal?: AbortSignal): Promise<PersonalProviderListData> {
  return authedRequest<PersonalProviderListData>(BASE_PATH, { accessToken, signal }, true);
}

/** Fetches the provider's model list with a candidate key; nothing is saved. */
export function probePersonalProvider(
  accessToken: string,
  payload: PersonalProviderProbePayload,
): Promise<PersonalProviderModelsData> {
  return authedRequest<PersonalProviderModelsData>(`${BASE_PATH}/probe`, { method: "POST", accessToken, body: payload }, true);
}

export function createPersonalProvider(
  accessToken: string,
  payload: CreatePersonalProviderPayload,
): Promise<PersonalProviderData> {
  return authedRequest<PersonalProviderData>(BASE_PATH, { method: "POST", accessToken, body: payload }, true);
}

export function updatePersonalProvider(
  accessToken: string,
  id: string,
  payload: UpdatePersonalProviderPayload,
): Promise<PersonalProviderData> {
  return authedRequest<PersonalProviderData>(
    `${BASE_PATH}/${pathParam(id)}`,
    { method: "PATCH", accessToken, body: payload },
    true,
  );
}

export function deletePersonalProvider(accessToken: string, id: string): Promise<PersonalProviderDeleteData> {
  return authedRequest<PersonalProviderDeleteData>(`${BASE_PATH}/${pathParam(id)}`, { method: "DELETE", accessToken }, true);
}

/** Re-fetches the provider's model list with the saved key. */
export function listPersonalProviderModels(accessToken: string, id: string): Promise<PersonalProviderModelsData> {
  return authedRequest<PersonalProviderModelsData>(`${BASE_PATH}/${pathParam(id)}/models`, { accessToken }, true);
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
