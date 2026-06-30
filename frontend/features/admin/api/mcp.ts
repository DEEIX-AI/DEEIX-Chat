import { authedRequest } from "@/shared/api/authed-client";
import { pathParam } from "@/shared/api/http-client";
import type { MCPToolDTO } from "@/shared/api/mcp.types";
import type {
  AdminMCPServerDTO,
  AdminMCPServerDataResponse,
  AdminMCPServerListResponse,
  AdminMCPOrderItemPayload,
  AdminMCPOrderListResponse,
  AdminMCPOrderGroupDTO,
  AdminMCPServerPayload,
  AdminMCPToolListResponse,
} from "@/features/admin/api/mcp.types";

export async function listAdminMCPServers(accessToken: string): Promise<AdminMCPServerDTO[]> {
  const data = await authedRequest<AdminMCPServerListResponse>(
    "/api/v1/admin/mcp/servers",
    {
      method: "GET",
      accessToken,
    },
    true,
  );
  return data.results ?? [];
}

export async function createAdminMCPServer(
  accessToken: string,
  payload: AdminMCPServerPayload,
): Promise<AdminMCPServerDTO> {
  const data = await authedRequest<AdminMCPServerDataResponse>(
    "/api/v1/admin/mcp/servers",
    {
      method: "POST",
      accessToken,
      body: payload,
    },
    true,
  );
  return data.server;
}

export async function updateAdminMCPServer(
  accessToken: string,
  serverID: number,
  payload: AdminMCPServerPayload,
): Promise<AdminMCPServerDTO> {
  const data = await authedRequest<AdminMCPServerDataResponse>(
    `/api/v1/admin/mcp/servers/${pathParam(String(serverID))}`,
    {
      method: "PATCH",
      accessToken,
      body: payload,
    },
    true,
  );
  return data.server;
}

export async function deleteAdminMCPServer(accessToken: string, serverID: number): Promise<{ deleted: boolean }> {
  return authedRequest<{ deleted: boolean }>(
    `/api/v1/admin/mcp/servers/${pathParam(String(serverID))}`,
    {
      method: "DELETE",
      accessToken,
    },
    true,
  );
}

export async function listAdminMCPServerTools(accessToken: string, serverID: number): Promise<MCPToolDTO[]> {
  const data = await authedRequest<AdminMCPToolListResponse>(
    `/api/v1/admin/mcp/servers/${pathParam(String(serverID))}/tools`,
    {
      method: "GET",
      accessToken,
    },
    true,
  );
  return data.results ?? [];
}

export async function syncAdminMCPServerTools(accessToken: string, serverID: number): Promise<MCPToolDTO[]> {
  const data = await authedRequest<AdminMCPToolListResponse>(
    `/api/v1/admin/mcp/servers/${pathParam(String(serverID))}/sync`,
    {
      method: "POST",
      accessToken,
    },
    true,
  );
  return data.results ?? [];
}

export type AdminMCPToolPayload = {
  displayName?: string;
  description?: string;
  status?: "active" | "inactive";
};

export async function updateAdminMCPTool(
  accessToken: string,
  toolID: number,
  payload: AdminMCPToolPayload,
): Promise<MCPToolDTO> {
  return authedRequest<MCPToolDTO>(
    `/api/v1/admin/mcp/tools/${pathParam(String(toolID))}`,
    {
      method: "PATCH",
      accessToken,
      body: payload,
    },
    true,
  );
}

export async function updateAdminMCPServerToolsStatus(
  accessToken: string,
  serverID: number,
  status: "active" | "inactive",
  toolIDs: number[],
): Promise<MCPToolDTO[]> {
  const data = await authedRequest<AdminMCPToolListResponse>(
    `/api/v1/admin/mcp/servers/${pathParam(String(serverID))}/tools/status`,
    {
      method: "PATCH",
      accessToken,
      body: { status, toolIDs },
    },
    true,
  );
  return data.results ?? [];
}

export async function reorderAdminMCPServers(
  accessToken: string,
  servers: AdminMCPOrderItemPayload[],
): Promise<AdminMCPOrderGroupDTO[]> {
  const data = await authedRequest<AdminMCPOrderListResponse>(
    "/api/v1/admin/mcp/servers/order",
    {
      method: "PATCH",
      accessToken,
      body: { servers },
    },
    true,
  );
  return data.results ?? [];
}
