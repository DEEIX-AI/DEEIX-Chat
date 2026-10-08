import type {
  AdminPersonalProviderPageResponseDoc,
  AdminPersonalProviderResponse,
  CreatePersonalProviderRequest,
  PersonalProviderAccessResponse,
  PersonalProviderAffectedResponse,
  PersonalProviderDataResponse,
  PersonalProviderDeleteResponse,
  PersonalProviderListResponse,
  PersonalProviderModelsResponse,
  PersonalProviderProbeRequest,
  PersonalProviderResponse,
  UpdatePersonalProviderRequest,
} from "@deeix/api-contract";

export type PersonalProviderAccessDTO = PersonalProviderAccessResponse;
export type PersonalProviderDTO = PersonalProviderResponse;
export type PersonalProviderListData = PersonalProviderListResponse;
export type PersonalProviderData = PersonalProviderDataResponse;
export type PersonalProviderModelsData = PersonalProviderModelsResponse;
export type PersonalProviderDeleteData = PersonalProviderDeleteResponse;
export type PersonalProviderProbePayload = PersonalProviderProbeRequest;
export type CreatePersonalProviderPayload = CreatePersonalProviderRequest;
export type UpdatePersonalProviderPayload = UpdatePersonalProviderRequest;

export type AdminPersonalProviderDTO = AdminPersonalProviderResponse;
export type AdminPersonalProviderPage = AdminPersonalProviderPageResponseDoc["data"];
export type PersonalProviderAffectedData = PersonalProviderAffectedResponse;
