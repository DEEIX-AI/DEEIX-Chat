import type { MCPToolDTO } from "@/shared/api/mcp.types";

export const DEFAULT_MCP_TOOLS_SETTING_KEY = "chat.default_mcp_tool_ids";

export function parseDefaultMCPToolIDs(raw: string | null | undefined): number[] {
  const value = raw?.trim();
  if (!value) {
    return [];
  }
  try {
    const parsed = JSON.parse(value) as unknown;
    if (!Array.isArray(parsed)) {
      return [];
    }
    const seen = new Set<number>();
    const result: number[] = [];
    for (const item of parsed) {
      const id = typeof item === "number" ? item : Number(item);
      if (Number.isSafeInteger(id) && id > 0 && !seen.has(id)) {
        seen.add(id);
        result.push(id);
      }
    }
    return result;
  } catch {
    return [];
  }
}

export function normalizeAvailableMCPTools(tools: MCPToolDTO[]): MCPToolDTO[] {
  const seen = new Set<number>();
  return tools.filter((tool) => {
    if (!Number.isSafeInteger(tool.id) || tool.id <= 0 || seen.has(tool.id)) {
      return false;
    }
    const status = typeof tool.status === "string" ? tool.status.trim() : "";
    if (status && status !== "active") {
      return false;
    }
    seen.add(tool.id);
    return true;
  });
}

export function filterAvailableMCPToolIDs(
  toolIDs: number[],
  tools: MCPToolDTO[],
  limit?: number,
): number[] {
  const availableIDs = new Set(tools.map((tool) => tool.id));
  const result = toolIDs.filter((id) => availableIDs.has(id));
  return typeof limit === "number" && limit >= 0 ? result.slice(0, limit) : result;
}
