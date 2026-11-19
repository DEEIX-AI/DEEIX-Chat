import { authedRequest } from "@/shared/api/authed-client";
import { pathParam } from "@/shared/api/http-client";
import type {
  DiscussionConversationSnapshot,
  DiscussionSnapshotRequest,
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

function discussionSnapshotPath(conversationPublicID: string, templatePublicID: string): string {
  return `/api/v1/conversations/${pathParam(conversationPublicID)}/discussion-snapshots/${pathParam(templatePublicID)}`;
}

export function getDiscussionSnapshot(
  accessToken: string,
  conversationPublicID: string,
  templatePublicID: string,
  signal?: AbortSignal,
): Promise<DiscussionConversationSnapshot> {
  return authedRequest<DiscussionConversationSnapshot>(
    discussionSnapshotPath(conversationPublicID, templatePublicID),
    { accessToken, signal },
    true,
  );
}

function mutateDiscussionSnapshot(
  accessToken: string,
  conversationPublicID: string,
  templatePublicID: string,
  action: "refresh" | "rollback" | "adopt",
  payload: DiscussionSnapshotRequest,
): Promise<DiscussionConversationSnapshot> {
  return authedRequest<DiscussionConversationSnapshot>(
    `${discussionSnapshotPath(conversationPublicID, templatePublicID)}/${action}`,
    { method: "POST", accessToken, body: payload },
    true,
  );
}

export function refreshDiscussionSnapshot(
  accessToken: string,
  conversationPublicID: string,
  templatePublicID: string,
  payload: DiscussionSnapshotRequest,
): Promise<DiscussionConversationSnapshot> {
  return mutateDiscussionSnapshot(accessToken, conversationPublicID, templatePublicID, "refresh", payload);
}

export function rollbackDiscussionSnapshot(
  accessToken: string,
  conversationPublicID: string,
  templatePublicID: string,
  expectedPendingVersion: number,
): Promise<DiscussionConversationSnapshot> {
  return mutateDiscussionSnapshot(accessToken, conversationPublicID, templatePublicID, "rollback", { expectedPendingVersion });
}

export function adoptDiscussionSnapshot(
  accessToken: string,
  conversationPublicID: string,
  templatePublicID: string,
  expectedPendingVersion: number,
): Promise<DiscussionConversationSnapshot> {
  return mutateDiscussionSnapshot(accessToken, conversationPublicID, templatePublicID, "adopt", { expectedPendingVersion });
}
