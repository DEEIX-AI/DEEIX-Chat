import type { MCPToolDTO } from "@/shared/api/mcp.types";

export function hasMultipleImageAttachmentProcessors(toolIDs: number[], tools: MCPToolDTO[]): boolean {
  const selectedIDs = new Set(toolIDs);
  let processorCount = 0;
  for (const tool of tools) {
    if (selectedIDs.has(tool.id) && tool.attachmentInputMode === "image") {
      processorCount += 1;
      if (processorCount > 1) {
        return true;
      }
    }
  }
  return false;
}

export function normalizeImageAttachmentProcessorSelection(
  toolIDs: number[],
  tools: MCPToolDTO[],
): number[] {
  const toolsByID = new Map(tools.map((tool) => [tool.id, tool]));
  let processorSelected = false;
  return toolIDs.filter((toolID) => {
    const tool = toolsByID.get(toolID);
    if (tool?.attachmentInputMode !== "image") {
      return true;
    }
    if (processorSelected) {
      return false;
    }
    processorSelected = true;
    return true;
  });
}
