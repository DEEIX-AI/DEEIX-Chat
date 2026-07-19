import type {
  CreatePermissionGroupRequest as ContractCreatePermissionGroupRequest,
  DeletePermissionGroupResponse,
  GroupModelsResponse,
  GroupUsersResponse,
  ModelPermissionGroupsResponse,
  PermissionGroupDataResponse,
  PermissionGroupDeleteSummaryResponse,
  PermissionGroupListResponse,
  PermissionGroupModelRuleResponse,
  PermissionGroupResponse,
  UpdatePermissionGroupRequest as ContractUpdatePermissionGroupRequest,
} from "@deeix/api-contract";
import { authedRequest } from "@/shared/api/authed-client";
import { pathParam } from "@/shared/api/http-client";

export type PermissionGroup = PermissionGroupResponse;

export type PermissionGroupModelRuleType = "all" | "vendor" | "protocol" | "upstream";

export type PermissionGroupModelRule = Omit<PermissionGroupModelRuleResponse, "type"> & {
  type: PermissionGroupModelRuleType;
};

export type CreatePermissionGroupRequest = ContractCreatePermissionGroupRequest;

export type UpdatePermissionGroupRequest = ContractUpdatePermissionGroupRequest;

type PermissionGroupListData = Omit<PermissionGroupListResponse, "results"> & {
  results: PermissionGroup[];
};

type PermissionGroupData = Omit<PermissionGroupDataResponse, "group"> & {
  group: PermissionGroup;
};

type GroupModelsData = Omit<GroupModelsResponse, "rules"> & { rules: PermissionGroupModelRule[] };

type GroupUsersData = GroupUsersResponse;

type ModelPermissionGroupsData = ModelPermissionGroupsResponse;

export type DeletePermissionGroupResult = Omit<DeletePermissionGroupResponse, "summary"> & {
  summary: PermissionGroupDeleteSummaryResponse;
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

export async function deletePermissionGroup(accessToken: string, id: number): Promise<DeletePermissionGroupResult> {
  return authedRequest<DeletePermissionGroupResult>(
    `/api/v1/admin/permission-groups/${pathParam(id)}`,
    { method: "DELETE", accessToken },
    true,
  );
}

export async function listGroupModels(
  accessToken: string,
  groupID: number,
): Promise<{ modelIDs: number[]; rules: PermissionGroupModelRule[] }> {
  const data = await authedRequest<GroupModelsData>(
    `/api/v1/admin/permission-groups/${pathParam(groupID)}/models`,
    { accessToken },
    true,
  );
  return {
    modelIDs: data.modelIDs ?? [],
    rules: data.rules ?? [],
  };
}

export async function setGroupModels(
  accessToken: string,
  groupID: number,
  modelIDs: number[],
  rules: PermissionGroupModelRule[] = [],
): Promise<void> {
  await authedRequest<GroupModelsData>(
    `/api/v1/admin/permission-groups/${pathParam(groupID)}/models`,
    { method: "PUT", accessToken, body: { modelIDs, rules } },
    true,
  );
}

export async function listModelPermissionGroups(
  accessToken: string,
  modelID: number,
): Promise<ModelPermissionGroupsData> {
  const data = await authedRequest<ModelPermissionGroupsData>(
    `/api/v1/admin/models/${pathParam(modelID)}/permission-groups`,
    { accessToken },
    true,
  );
  return {
    manualGroupIDs: data.manualGroupIDs ?? [],
    matchedGroupIDs: data.matchedGroupIDs ?? [],
    effectiveGroupIDs: data.effectiveGroupIDs ?? [],
    unassigned: data.unassigned ?? false,
  };
}

export async function setModelPermissionGroups(
  accessToken: string,
  modelID: number,
  groupIDs: number[],
): Promise<ModelPermissionGroupsData> {
  const data = await authedRequest<ModelPermissionGroupsData>(
    `/api/v1/admin/models/${pathParam(modelID)}/permission-groups`,
    { method: "PUT", accessToken, body: { groupIDs } },
    true,
  );
  return {
    manualGroupIDs: data.manualGroupIDs ?? [],
    matchedGroupIDs: data.matchedGroupIDs ?? [],
    effectiveGroupIDs: data.effectiveGroupIDs ?? [],
    unassigned: data.unassigned ?? false,
  };
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
