import type { SettingsGrouped } from "@/shared/api/settings.types";

export type ToolSettingsFieldType = "int" | "bool" | "textarea";

export type ToolSettingsField = {
  namespace: "mcp";
  key:
    | "mcp_enable"
    | "mcp_tool_timeout_seconds"
    | "mcp_tool_retry_count"
    | "mcp_max_concurrent_calls"
    | "mcp_max_selected_tools_per_message"
    | "mcp_max_llm_calls_per_run"
    | "mcp_max_tool_calls_per_run"
    | "mcp_tool_prompt";
  labelKey: string;
  descriptionKey: string;
  type: ToolSettingsFieldType;
  placeholder?: string;
  placeholderKey?: string;
};

export const TOOL_SETTINGS_FIELDS: ToolSettingsField[] = [
  {
    namespace: "mcp",
    key: "mcp_enable",
    labelKey: "mcpEnable.label",
    descriptionKey: "mcpEnable.description",
    type: "bool",
  },
  {
    namespace: "mcp",
    key: "mcp_tool_prompt",
    labelKey: "toolPrompt.label",
    descriptionKey: "toolPrompt.description",
    type: "textarea",
    placeholderKey: "defaultPromptPlaceholder",
  },
  {
    namespace: "mcp",
    key: "mcp_max_selected_tools_per_message",
    labelKey: "maxSelectedTools.label",
    descriptionKey: "maxSelectedTools.description",
    type: "int",
    placeholder: "32",
  },
  {
    namespace: "mcp",
    key: "mcp_max_llm_calls_per_run",
    labelKey: "maxLLMCalls.label",
    descriptionKey: "maxLLMCalls.description",
    type: "int",
    placeholder: "5",
  },
  {
    namespace: "mcp",
    key: "mcp_max_tool_calls_per_run",
    labelKey: "maxToolCalls.label",
    descriptionKey: "maxToolCalls.description",
    type: "int",
    placeholder: "8",
  },
  {
    namespace: "mcp",
    key: "mcp_max_concurrent_calls",
    labelKey: "maxConcurrentCalls.label",
    descriptionKey: "maxConcurrentCalls.description",
    type: "int",
    placeholder: "8",
  },
  {
    namespace: "mcp",
    key: "mcp_tool_timeout_seconds",
    labelKey: "toolTimeout.label",
    descriptionKey: "toolTimeout.description",
    type: "int",
    placeholder: "10",
  },
  {
    namespace: "mcp",
    key: "mcp_tool_retry_count",
    labelKey: "toolRetry.label",
    descriptionKey: "toolRetry.description",
    type: "int",
    placeholder: "0",
  },
];

export function toolFieldID(field: ToolSettingsField): string {
  return `${field.namespace}.${field.key}`;
}

export function flattenToolSettings(grouped: SettingsGrouped): Record<string, string> {
  const result: Record<string, string> = {};
  for (const item of grouped.mcp ?? []) {
    result[`mcp.${item.key}`] = item.value ?? "";
  }
  return applyToolSettingsDefaults(result);
}

export function applyToolSettingsDefaults(settings: Record<string, string>): Record<string, string> {
  return {
    ...settings,
    "mcp.mcp_enable": settings["mcp.mcp_enable"] || "false",
    "mcp.mcp_tool_prompt": settings["mcp.mcp_tool_prompt"] ?? "",
    "mcp.mcp_max_selected_tools_per_message": settings["mcp.mcp_max_selected_tools_per_message"] || "32",
    "mcp.mcp_max_llm_calls_per_run": settings["mcp.mcp_max_llm_calls_per_run"] || "5",
    "mcp.mcp_max_tool_calls_per_run": settings["mcp.mcp_max_tool_calls_per_run"] || "8",
    "mcp.mcp_max_concurrent_calls": settings["mcp.mcp_max_concurrent_calls"] || "8",
    "mcp.mcp_tool_timeout_seconds": settings["mcp.mcp_tool_timeout_seconds"] || "10",
    "mcp.mcp_tool_retry_count": settings["mcp.mcp_tool_retry_count"] || "0",
  };
}

export function toToolEditorField(
  field: ToolSettingsField,
  translate: (key: string) => string,
) {
  return {
    id: toolFieldID(field),
    label: translate(field.labelKey),
    description: translate(field.descriptionKey),
    type: field.type,
    placeholder: field.placeholderKey ? translate(field.placeholderKey) : field.placeholder,
  } as const;
}
