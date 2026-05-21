import { authedRequest } from "@/shared/api/authed-client";
import type { MCPToolDTO, MCPToolListResponse } from "@/shared/api/mcp.types";

export async function listAvailableMCPTools(accessToken: string): Promise<MCPToolDTO[]> {
  const data = await authedRequest<MCPToolListResponse>(
    "/api/v1/mcp/tools",
    {
      method: "GET",
      accessToken,
    },
    true,
  );
  return data.results ?? [];
}
