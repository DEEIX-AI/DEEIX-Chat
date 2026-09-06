import { authedRequest } from "@/shared/api/authed-client";
import { pathParam } from "@/shared/api/http-client";

export type UserAPIKeyDTO = {
  publicId: string;
  name: string;
  keyPrefix: string;
  lastUsedAt?: string | null;
  revokedAt?: string | null;
  expiresAt?: string | null;
  createdAt: string;
};

export type CreateUserAPIKeyResult = UserAPIKeyDTO & {
  key: string;
};

export async function listUserAPIKeys(accessToken: string): Promise<UserAPIKeyDTO[]> {
  return authedRequest<UserAPIKeyDTO[]>("/api/v1/me/api-keys", {
    method: "GET",
    accessToken,
  });
}

export async function createUserAPIKey(
  accessToken: string,
  name: string,
): Promise<CreateUserAPIKeyResult> {
  return authedRequest<CreateUserAPIKeyResult>("/api/v1/me/api-keys", {
    method: "POST",
    accessToken,
    body: { name },
  });
}

export async function revokeUserAPIKey(accessToken: string, publicId: string): Promise<{ revoked: boolean }> {
  return authedRequest<{ revoked: boolean }>(`/api/v1/me/api-keys/${pathParam(publicId)}`, {
    method: "DELETE",
    accessToken,
  });
}
