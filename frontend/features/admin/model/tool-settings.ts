import type { SettingsGrouped } from "@/shared/api/settings.types";

export type ToolSettingsFieldType = "int" | "bool" | "textarea";

export type ToolSettingsField = {
  namespace: "mcp" | "programming";
  key:
    | "mcp_enable"
    | "mcp_tool_timeout_seconds"
    | "mcp_tool_retry_count"
    | "mcp_max_concurrent_calls"
    | "mcp_max_selected_tools_per_message"
    | "mcp_max_llm_calls_per_run"
    | "mcp_max_tool_calls_per_run"
    | "mcp_tool_prompt"
    | "programming_enable"
    | "programming_shell_enable"
    | "programming_tool_timeout_seconds"
    | "programming_max_file_bytes"
    | "programming_max_llm_calls_per_run"
    | "programming_max_tool_calls_per_run"
    | "programming_max_output_chars"
    | "programming_tool_prompt";
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

export const PROGRAMMING_SETTINGS_FIELDS: ToolSettingsField[] = [
  {
    namespace: "programming",
    key: "programming_enable",
    labelKey: "programmingEnable.label",
    descriptionKey: "programmingEnable.description",
    type: "bool",
  },
  {
    namespace: "programming",
    key: "programming_shell_enable",
    labelKey: "programmingShellEnable.label",
    descriptionKey: "programmingShellEnable.description",
    type: "bool",
  },
  {
    namespace: "programming",
    key: "programming_tool_prompt",
    labelKey: "programmingToolPrompt.label",
    descriptionKey: "programmingToolPrompt.description",
    type: "textarea",
    placeholderKey: "defaultPromptPlaceholder",
  },
  {
    namespace: "programming",
    key: "programming_max_llm_calls_per_run",
    labelKey: "programmingMaxLLMCalls.label",
    descriptionKey: "programmingMaxLLMCalls.description",
    type: "int",
    placeholder: "12",
  },
  {
    namespace: "programming",
    key: "programming_max_tool_calls_per_run",
    labelKey: "programmingMaxToolCalls.label",
    descriptionKey: "programmingMaxToolCalls.description",
    type: "int",
    placeholder: "32",
  },
  {
    namespace: "programming",
    key: "programming_tool_timeout_seconds",
    labelKey: "programmingToolTimeout.label",
    descriptionKey: "programmingToolTimeout.description",
    type: "int",
    placeholder: "30",
  },
  {
    namespace: "programming",
    key: "programming_max_file_bytes",
    labelKey: "programmingMaxFileBytes.label",
    descriptionKey: "programmingMaxFileBytes.description",
    type: "int",
    placeholder: "1048576",
  },
  {
    namespace: "programming",
    key: "programming_max_output_chars",
    labelKey: "programmingMaxOutputChars.label",
    descriptionKey: "programmingMaxOutputChars.description",
    type: "int",
    placeholder: "100000",
  },
];

export const ALL_TOOL_SETTINGS_FIELDS: ToolSettingsField[] = [
  ...TOOL_SETTINGS_FIELDS,
  ...PROGRAMMING_SETTINGS_FIELDS,
];

export function toolFieldID(field: ToolSettingsField): string {
  return `${field.namespace}.${field.key}`;
}

export function flattenToolSettings(grouped: SettingsGrouped): Record<string, string> {
  const result: Record<string, string> = {};
  for (const item of grouped.mcp ?? []) {
    result[`mcp.${item.key}`] = item.value ?? "";
  }
  for (const item of grouped.programming ?? []) {
    result[`programming.${item.key}`] = item.value ?? "";
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
    "programming.programming_enable": settings["programming.programming_enable"] || "true",
    "programming.programming_shell_enable": settings["programming.programming_shell_enable"] || "false",
    "programming.programming_tool_prompt": settings["programming.programming_tool_prompt"] ?? "",
    "programming.programming_max_llm_calls_per_run": settings["programming.programming_max_llm_calls_per_run"] || "12",
    "programming.programming_max_tool_calls_per_run": settings["programming.programming_max_tool_calls_per_run"] || "32",
    "programming.programming_tool_timeout_seconds": settings["programming.programming_tool_timeout_seconds"] || "30",
    "programming.programming_max_file_bytes": settings["programming.programming_max_file_bytes"] || "1048576",
    "programming.programming_max_output_chars": settings["programming.programming_max_output_chars"] || "100000",
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
