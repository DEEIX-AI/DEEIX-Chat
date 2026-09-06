import { authedRequest } from "@/shared/api/authed-client";
import { pathParam } from "@/shared/api/http-client";

export type ResourceACLEntryDTO = {
  granteeUserID: number;
  granteeUsername: string;
  role: "viewer" | "editor";
  createdAt: string;
  updatedAt: string;
};

export async function listConversationACL(
  accessToken: string,
  conversationPublicID: string,
): Promise<ResourceACLEntryDTO[]> {
  return authedRequest<ResourceACLEntryDTO[]>(
    `/api/v1/conversations/${pathParam(conversationPublicID)}/acl`,
    { method: "GET", accessToken },
  );
}

export async function grantConversationACL(
  accessToken: string,
  conversationPublicID: string,
  username: string,
  role: "viewer" | "editor",
): Promise<ResourceACLEntryDTO> {
  return authedRequest<ResourceACLEntryDTO>(
    `/api/v1/conversations/${pathParam(conversationPublicID)}/acl`,
    { method: "PUT", accessToken, body: { username, role } },
  );
}

export async function revokeConversationACL(
  accessToken: string,
  conversationPublicID: string,
  granteeUserID: number,
): Promise<{ revoked: boolean }> {
  return authedRequest<{ revoked: boolean }>(
    `/api/v1/conversations/${pathParam(conversationPublicID)}/acl/${granteeUserID}`,
    { method: "DELETE", accessToken },
  );
}

export async function listKnowledgeBaseACL(
  accessToken: string,
  knowledgeBasePublicID: string,
): Promise<ResourceACLEntryDTO[]> {
  return authedRequest<ResourceACLEntryDTO[]>(
    `/api/v1/knowledge-bases/mine/${pathParam(knowledgeBasePublicID)}/acl`,
    { method: "GET", accessToken },
  );
}

export async function grantKnowledgeBaseACL(
  accessToken: string,
  knowledgeBasePublicID: string,
  username: string,
  role: "viewer" | "editor",
): Promise<ResourceACLEntryDTO> {
  return authedRequest<ResourceACLEntryDTO>(
    `/api/v1/knowledge-bases/mine/${pathParam(knowledgeBasePublicID)}/acl`,
    { method: "PUT", accessToken, body: { username, role } },
  );
}

export async function revokeKnowledgeBaseACL(
  accessToken: string,
  knowledgeBasePublicID: string,
  granteeUserID: number,
): Promise<{ revoked: boolean }> {
  return authedRequest<{ revoked: boolean }>(
    `/api/v1/knowledge-bases/mine/${pathParam(knowledgeBasePublicID)}/acl/${granteeUserID}`,
    { method: "DELETE", accessToken },
  );
}
