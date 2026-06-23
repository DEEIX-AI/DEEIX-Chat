import { authedRequest } from "@/shared/api/authed-client";
import type {
  AdminUserData,
  PatchAdminUserRequest,
  CreateAdminUserRequest,
  DeleteAdminUserData,
  ResetAdminUserPasswordData,
  ResetAdminUserPasswordRequest,
  ResetAdminUserTwoFactorData,
  RevokeAdminUserSessionsData,
  UpdateAdminUserStatusRequest,
  ImportOpenWebUIUsersData,
  ImportOpenWebUIUsersRequest,
} from "@/features/admin/api/admin.types";
import type { PagePayload } from "@/shared/api/common.types";
import type { UserDTO } from "@/shared/api/auth.types";

import { normalizeAdminPagePayload, resolveAdminPage, type AdminListQueryOptions } from "./shared";

export async function listAdminUsers(
  accessToken: string,
  options: AdminListQueryOptions = {},
): Promise<PagePayload<UserDTO>> {
  const { page, pageSize } = resolveAdminPage(options);
  const params = new URLSearchParams({
    page: String(page),
    page_size: String(pageSize),
  });
  if (options.query?.trim()) {
    params.set("q", options.query.trim());
  }
  const data = await authedRequest<PagePayload<UserDTO>>(
    `/api/v1/admin/users?${params.toString()}`,
    { accessToken },
    true,
  );

  return normalizeAdminPagePayload(data);
}

export async function createAdminUser(
  accessToken: string,
  payload: CreateAdminUserRequest,
): Promise<AdminUserData> {
  return authedRequest<AdminUserData>(
    "/api/v1/admin/users",
    {
      method: "POST",
      accessToken,
      body: payload,
    },
    true,
  );
}

export async function patchAdminUser(
  accessToken: string,
  userID: number,
  payload: PatchAdminUserRequest,
): Promise<AdminUserData> {
  return authedRequest<AdminUserData>(
    `/api/v1/admin/users/${userID}`,
    {
      method: "PATCH",
      accessToken,
      body: payload,
    },
    true,
  );
}

export async function updateAdminUserStatus(
  accessToken: string,
  userID: number,
  payload: UpdateAdminUserStatusRequest,
): Promise<AdminUserData> {
  return authedRequest<AdminUserData>(
    `/api/v1/admin/users/${userID}/status`,
    {
      method: "PATCH",
      accessToken,
      body: payload,
    },
    true,
  );
}

export async function resetAdminUserPassword(
  accessToken: string,
  userID: number,
  payload: ResetAdminUserPasswordRequest,
): Promise<ResetAdminUserPasswordData> {
  return authedRequest<ResetAdminUserPasswordData>(
    `/api/v1/admin/users/${userID}/reset-password`,
    {
      method: "POST",
      accessToken,
      body: payload,
    },
    true,
  );
}

export async function resetAdminUserTwoFactor(
  accessToken: string,
  userID: number,
): Promise<ResetAdminUserTwoFactorData> {
  return authedRequest<ResetAdminUserTwoFactorData>(
    `/api/v1/admin/users/${userID}/reset-2fa`,
    {
      method: "POST",
      accessToken,
    },
    true,
  );
}

export async function revokeAdminUserSessions(
  accessToken: string,
  userID: number,
): Promise<RevokeAdminUserSessionsData> {
  return authedRequest<RevokeAdminUserSessionsData>(
    `/api/v1/admin/users/${userID}/revoke-sessions`,
    {
      method: "POST",
      accessToken,
    },
    true,
  );
}

export async function deleteAdminUser(
  accessToken: string,
  userID: number,
): Promise<DeleteAdminUserData> {
  return authedRequest<DeleteAdminUserData>(
    `/api/v1/admin/users/${userID}`,
    {
      method: "DELETE",
      accessToken,
    },
    true,
  );
}

export async function importOpenWebUIUsers(
  accessToken: string,
  payload: ImportOpenWebUIUsersRequest,
): Promise<ImportOpenWebUIUsersData> {
  return authedRequest<ImportOpenWebUIUsersData>(
    "/api/v1/admin/users/import/openwebui",
    {
      method: "POST",
      accessToken,
      body: payload,
    },
    true,
  );
}
