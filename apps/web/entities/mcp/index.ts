// Public entry of the mcp entity; code outside entities/mcp/ imports only this file.
// Unused re-exports are tree-shaken (package.json "sideEffects"), so UI and logic share one entry.
export {
  hasMultipleImageAttachmentProcessors,
  normalizeImageAttachmentProcessorSelection,
} from "@/entities/mcp/lib/mcp-tool-selection";
