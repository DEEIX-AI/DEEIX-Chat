import { authedRequest } from "@/shared/api/authed-client";
import { pathParam } from "@/shared/api/http-client";
import type {
  DiscussionTemplateDTO,
  DiscussionTemplatePayload,
} from "@/shared/api/discussion-types";

type ListResponse = { results?: DiscussionTemplateDTO[] };
type DataResponse = { data: DiscussionTemplateDTO };

export async function listDiscussionTemplates(
  accessToken: string,
  signal?: AbortSignal,
): Promise<DiscussionTemplateDTO[]> {
  const data = await authedRequest<ListResponse>(
    "/api/v1/discussion-templates",
    { accessToken, signal },
    true,
  );
  return data.results ?? [];
}

export async function listAdminDiscussionTemplates(
  accessToken: string,
  signal?: AbortSignal,
): Promise<DiscussionTemplateDTO[]> {
  const data = await authedRequest<ListResponse>(
    "/api/v1/admin/discussion-templates",
    { accessToken, signal },
    true,
  );
  return data.results ?? [];
}

export async function createAdminDiscussionTemplate(
  accessToken: string,
  payload: DiscussionTemplatePayload,
): Promise<DiscussionTemplateDTO> {
  const data = await authedRequest<DataResponse>(
    "/api/v1/admin/discussion-templates",
    { method: "POST", accessToken, body: payload },
    true,
  );
  return data.data;
}

export async function updateAdminDiscussionTemplate(
  accessToken: string,
  publicID: string,
  payload: DiscussionTemplatePayload,
): Promise<DiscussionTemplateDTO> {
  const data = await authedRequest<DataResponse>(
    `/api/v1/admin/discussion-templates/${pathParam(publicID)}`,
    { method: "PATCH", accessToken, body: payload },
    true,
  );
  return data.data;
}

export async function deleteAdminDiscussionTemplate(
  accessToken: string,
  publicID: string,
): Promise<void> {
  await authedRequest<{ deleted: boolean }>(
    `/api/v1/admin/discussion-templates/${pathParam(publicID)}`,
    { method: "DELETE", accessToken },
    true,
  );
}
