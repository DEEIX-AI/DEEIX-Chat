import type {
  AdminPersonalProviderPageResponseDoc,
  AdminPersonalProviderResponse,
  CreatePersonalProviderRequest,
  PersonalProviderAccessResponse,
  PersonalProviderAffectedResponse,
  PersonalProviderAvailableModelResponse,
  PersonalProviderDataResponse,
  PersonalProviderDeleteResponse,
  PersonalProviderListResponse,
  PersonalProviderModelRequest,
  PersonalProviderModelResponse,
  PersonalProviderModelsResponse,
  PersonalProviderProbeRequest,
  PersonalProviderResponse,
  UpdatePersonalProviderRequest,
} from "@deeix/api-contract";

export type PersonalProviderAccessDTO = PersonalProviderAccessResponse;
export type PersonalProviderDTO = PersonalProviderResponse;
export type PersonalProviderModelDTO = PersonalProviderModelResponse;
export type PersonalProviderListData = PersonalProviderListResponse;
export type PersonalProviderData = PersonalProviderDataResponse;
export type PersonalProviderModelsData = PersonalProviderModelsResponse;
/** A model the provider lists, with the protocols the server suggests for it. */
export type PersonalProviderAvailableModelDTO = PersonalProviderAvailableModelResponse;

export type PersonalProviderDeleteData = PersonalProviderDeleteResponse;
export type PersonalProviderProbePayload = PersonalProviderProbeRequest;
/** A model to enable: its name with protocols, or the bare name, the only form older servers take. */
type ModelSelectionPayload = Array<string | PersonalProviderModelRequest>;
export type CreatePersonalProviderPayload = Omit<CreatePersonalProviderRequest, "models"> & { models: ModelSelectionPayload };
export type UpdatePersonalProviderPayload = Omit<UpdatePersonalProviderRequest, "models"> & { models?: ModelSelectionPayload };

export type AdminPersonalProviderDTO = AdminPersonalProviderResponse;
export type AdminPersonalProviderPage = AdminPersonalProviderPageResponseDoc["data"];
export type PersonalProviderAffectedData = PersonalProviderAffectedResponse;
