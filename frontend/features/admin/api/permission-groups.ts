import { authedRequest } from "@/shared/api/authed-client";
import { pathParam } from "@/shared/api/http-client";

export type PermissionGroup = {
  id: number;
  code: string;
  name: string;
  description: string;
  isDefault: boolean;
  createdAt: string;
  updatedAt: string;
};

export type CreatePermissionGroupRequest = {
  code: string;
  name: string;
  description: string;
};

export type UpdatePermissionGroupRequest = {
  name: string;
  description: string;
};

type PermissionGroupListData = {
  results: PermissionGroup[];
};

type PermissionGroupData = {
  group: PermissionGroup;
};

type GroupModelsData = {
  modelIDs: number[];
};

type GroupUsersData = {
  userIDs: number[];
};

export async function listPermissionGroups(accessToken: string): Promise<PermissionGroup[]> {
  const data = await authedRequest<PermissionGroupListData>(
    "/api/v1/admin/permission-groups",
    { accessToken },
    true,
  );
  return data.results ?? [];
}

export async function createPermissionGroup(
  accessToken: string,
  req: CreatePermissionGroupRequest,
): Promise<PermissionGroup> {
  const data = await authedRequest<PermissionGroupData>(
    "/api/v1/admin/permission-groups",
    { method: "POST", accessToken, body: req },
    true,
  );
  return data.group;
}

export async function updatePermissionGroup(
  accessToken: string,
  id: number,
  req: UpdatePermissionGroupRequest,
): Promise<PermissionGroup> {
  const data = await authedRequest<PermissionGroupData>(
    `/api/v1/admin/permission-groups/${pathParam(id)}`,
    { method: "PATCH", accessToken, body: req },
    true,
  );
  return data.group;
}

export async function deletePermissionGroup(accessToken: string, id: number): Promise<void> {
  await authedRequest<{ deleted: boolean }>(
    `/api/v1/admin/permission-groups/${pathParam(id)}`,
    { method: "DELETE", accessToken },
    true,
  );
}

export async function listGroupModels(accessToken: string, groupID: number): Promise<number[]> {
  const data = await authedRequest<GroupModelsData>(
    `/api/v1/admin/permission-groups/${pathParam(groupID)}/models`,
    { accessToken },
    true,
  );
  return data.modelIDs ?? [];
}

export async function setGroupModels(
  accessToken: string,
  groupID: number,
  modelIDs: number[],
): Promise<void> {
  await authedRequest<GroupModelsData>(
    `/api/v1/admin/permission-groups/${pathParam(groupID)}/models`,
    { method: "PUT", accessToken, body: { modelIDs } },
    true,
  );
}

export async function listGroupUsers(accessToken: string, groupID: number): Promise<number[]> {
  const data = await authedRequest<GroupUsersData>(
    `/api/v1/admin/permission-groups/${pathParam(groupID)}/users`,
    { accessToken },
    true,
  );
  return data.userIDs ?? [];
}

export async function setGroupUsers(
  accessToken: string,
  groupID: number,
  userIDs: number[],
): Promise<void> {
  await authedRequest<GroupUsersData>(
    `/api/v1/admin/permission-groups/${pathParam(groupID)}/users`,
    { method: "PUT", accessToken, body: { userIDs } },
    true,
  );
}
