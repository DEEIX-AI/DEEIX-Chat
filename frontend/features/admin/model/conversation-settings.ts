import type { SettingsGrouped } from "@/shared/api/settings.types";

export type ConversationFieldType = "int" | "bool" | "string" | "password" | "textarea" | "json" | "select" | "tabs" | "button";

export type ConversationVisibilityRule =
  | { field: string; equals: string }
  | { all: ConversationVisibilityRule[] };

export type ConversationSettingsSection = "conversation" | "contextCompression" | "optionPassthrough";

export type ConversationSettingsField = {
  section: ConversationSettingsSection;
  namespace: "chat";
  key:
    | "conversation_task_model"
    | "default_system_prompt"
    | "conversation_title_prompt"
    | "conversation_labels_prompt"
    | "context_compact_enabled"
    | "context_token_budget_enabled"
    | "context_max_turns"
    | "context_compact_trigger_tokens"
    | "context_compact_preserve_recent_turns"
    | "context_compact_highlights_per_role"
    | "context_compact_snippet_chars"
    | "context_artifact_retention_days"
    | "compact_async_enabled"
    | "compact_llm_enabled"
    | "compact_task_model"
    | "compact_max_failures"
    | "compact_system_prompt"
    | "compact_light_prompt"
    | "model_option_policy_mode"
    | "model_option_allowed_paths"
    | "model_option_denied_paths";
  label: string;
  description: string;
  type: ConversationFieldType;
  placeholder?: string;
  options?: Array<{ label: string; value: string }>;
  visibleWhen?: ConversationVisibilityRule;
  subgroupKey?: string;
};

export const CONVERSATION_TASK_MODEL_FOLLOW = "follow";

export const CONTEXT_COMPACT_ENABLED_RULE: ConversationVisibilityRule = {
  field: "chat.context_compact_enabled",
  equals: "true",
};

export const COMPACT_LLM_ENABLED_RULE: ConversationVisibilityRule = {
  all: [
    CONTEXT_COMPACT_ENABLED_RULE,
    { field: "chat.compact_llm_enabled", equals: "true" },
  ],
};

export const DEFAULT_MODEL_OPTION_ALLOWED_PATHS = `{
  "default": [
    "temperature",
    "top_p",
    "max_tokens",
    "max_output_tokens",
    "max_completion_tokens",
    "stop",
    "tools",
    "response_format.type"
  ],
  "openai_chat_completions": [
    "service_tier",
    "presence_penalty",
    "frequency_penalty",
    "reasoning_effort",
    "verbosity",
    "thinking.type",
    "stream_options.include_usage"
  ],
  "openai_responses": [
    "service_tier",
    "store",
    "reasoning.effort",
    "reasoning.summary",
    "text.verbosity"
  ],
  "openai_image_generations": [
    "background",
    "moderation",
    "n",
    "output_compression",
    "output_format",
    "partial_images",
    "quality",
    "response_format",
    "size",
    "style",
    "user"
  ],
  "openai_image_edits": [
    "background",
    "input_fidelity",
    "n",
    "output_compression",
    "output_format",
    "partial_images",
    "quality",
    "response_format",
    "size",
    "user"
  ],
  "google_image_generation": [
    "generationConfig.responseModalities",
    "generationConfig.imageConfig.aspectRatio",
    "generationConfig.imageConfig.imageSize"
  ],
  "anthropic_messages": [
    "speed",
    "top_k",
    "cache_control",
    "thinking.type",
    "thinking.budget_tokens"
  ],
  "xai_responses": [
    "store",
    "reasoning.effort"
  ],
  "xai_image": [
    "aspect_ratio",
    "n",
    "resolution",
    "response_format"
  ],
  "xai_image_edits": [
    "aspect_ratio",
    "n",
    "resolution",
    "response_format"
  ],
  "gemini_generate_content": [
    "generationConfig.temperature",
    "generationConfig.topP",
    "generationConfig.maxOutputTokens",
    "generationConfig.responseMimeType"
  ]
}`;

export const DEFAULT_MODEL_OPTION_DENIED_PATHS = `{
  "default": [
    "model",
    "messages",
    "input",
    "instructions",
    "prompt",
    "system",
    "systemInstruction",
    "headers",
    "api_key",
    "apiKey",
    "base_url",
    "baseURL",
    "stream",
    "previous_response_id"
  ]
}`;

type ConversationSettingsTranslator = (key: string) => string;

export function buildConversationSettingsFields(t: ConversationSettingsTranslator): ConversationSettingsField[] {
  return [
    {
      section: "conversation",
      namespace: "chat",
      key: "conversation_task_model",
      label: t("fields.taskModel.label"),
      description: t("fields.taskModel.description"),
      type: "select",
      options: [{ label: t("taskModel.follow"), value: CONVERSATION_TASK_MODEL_FOLLOW }],
    },
    {
      section: "optionPassthrough",
      namespace: "chat",
      key: "model_option_policy_mode",
      label: t("fields.optionPolicyMode.label"),
      description: t("fields.optionPolicyMode.description"),
      type: "select",
      options: [
        { label: t("policy.allowlist"), value: "allowlist" },
        { label: t("policy.denylist"), value: "denylist" },
        { label: t("policy.disabled"), value: "disabled" },
      ],
    },
    {
      section: "optionPassthrough",
      namespace: "chat",
      key: "model_option_allowed_paths",
      label: t("fields.allowedPaths.label"),
      description: t("fields.allowedPaths.description"),
      type: "json",
      placeholder: DEFAULT_MODEL_OPTION_ALLOWED_PATHS,
    },
    {
      section: "optionPassthrough",
      namespace: "chat",
      key: "model_option_denied_paths",
      label: t("fields.deniedPaths.label"),
      description: t("fields.deniedPaths.description"),
      type: "json",
      placeholder: DEFAULT_MODEL_OPTION_DENIED_PATHS,
    },
    {
      section: "conversation",
      namespace: "chat",
      key: "conversation_title_prompt",
      label: t("fields.titlePrompt.label"),
      description: t("fields.titlePrompt.description"),
      type: "textarea",
      placeholder: t("fields.defaultPromptPlaceholder"),
    },
    {
      section: "conversation",
      namespace: "chat",
      key: "conversation_labels_prompt",
      label: t("fields.labelsPrompt.label"),
      description: t("fields.labelsPrompt.description"),
      type: "textarea",
      placeholder: t("fields.defaultPromptPlaceholder"),
    },
    {
      section: "conversation",
      namespace: "chat",
      key: "default_system_prompt",
      label: t("fields.defaultSystemPrompt.label"),
      description: t("fields.defaultSystemPrompt.description"),
      type: "textarea",
      placeholder: t("fields.defaultSystemPrompt.placeholder"),
    },
    {
      section: "contextCompression",
      namespace: "chat",
      key: "context_compact_enabled",
      label: t("fields.contextCompactEnabled.label"),
      description: t("fields.contextCompactEnabled.description"),
      type: "bool",
    },
    {
      section: "contextCompression",
      namespace: "chat",
      key: "context_token_budget_enabled",
      label: t("fields.contextTokenBudget.label"),
      description: t("fields.contextTokenBudget.description"),
      type: "bool",
      visibleWhen: CONTEXT_COMPACT_ENABLED_RULE,
    },
    {
      section: "contextCompression",
      namespace: "chat",
      key: "context_max_turns",
      label: t("fields.contextMaxTurns.label"),
      description: t("fields.contextMaxTurns.description"),
      type: "int",
      placeholder: t("fields.contextMaxTurns.placeholder"),
      visibleWhen: CONTEXT_COMPACT_ENABLED_RULE,
    },
    {
      section: "contextCompression",
      namespace: "chat",
      key: "context_compact_trigger_tokens",
      label: t("fields.contextCompactTriggerTokens.label"),
      description: t("fields.contextCompactTriggerTokens.description"),
      type: "int",
      placeholder: t("fields.contextCompactTriggerTokens.placeholder"),
      visibleWhen: CONTEXT_COMPACT_ENABLED_RULE,
    },
    {
      section: "contextCompression",
      namespace: "chat",
      key: "context_compact_preserve_recent_turns",
      label: t("fields.contextCompactPreserveTurns.label"),
      description: t("fields.contextCompactPreserveTurns.description"),
      type: "int",
      placeholder: t("fields.contextCompactPreserveTurns.placeholder"),
      visibleWhen: CONTEXT_COMPACT_ENABLED_RULE,
    },
    {
      section: "contextCompression",
      namespace: "chat",
      key: "context_compact_highlights_per_role",
      label: t("fields.contextCompactHighlightsPerRole.label"),
      description: t("fields.contextCompactHighlightsPerRole.description"),
      type: "int",
      placeholder: t("fields.contextCompactHighlightsPerRole.placeholder"),
      visibleWhen: CONTEXT_COMPACT_ENABLED_RULE,
    },
    {
      section: "contextCompression",
      namespace: "chat",
      key: "context_compact_snippet_chars",
      label: t("fields.contextCompactSnippetChars.label"),
      description: t("fields.contextCompactSnippetChars.description"),
      type: "int",
      placeholder: t("fields.contextCompactSnippetChars.placeholder"),
      visibleWhen: CONTEXT_COMPACT_ENABLED_RULE,
    },
    {
      section: "contextCompression",
      namespace: "chat",
      key: "context_artifact_retention_days",
      label: t("fields.contextArtifactRetentionDays.label"),
      description: t("fields.contextArtifactRetentionDays.description"),
      type: "int",
      placeholder: t("fields.contextArtifactRetentionDays.placeholder"),
      visibleWhen: CONTEXT_COMPACT_ENABLED_RULE,
    },
    {
      section: "contextCompression",
      namespace: "chat",
      key: "compact_async_enabled",
      label: t("fields.compactAsync.label"),
      description: t("fields.compactAsync.description"),
      type: "bool",
      visibleWhen: CONTEXT_COMPACT_ENABLED_RULE,
    },
    {
      section: "contextCompression",
      namespace: "chat",
      key: "compact_llm_enabled",
      label: t("fields.compactLLM.label"),
      description: t("fields.compactLLM.description"),
      type: "bool",
      visibleWhen: CONTEXT_COMPACT_ENABLED_RULE,
    },
    {
      section: "contextCompression",
      namespace: "chat",
      key: "compact_task_model",
      label: t("fields.compactTaskModel.label"),
      description: t("fields.compactTaskModel.description"),
      type: "select",
      options: [{ label: t("taskModel.follow"), value: CONVERSATION_TASK_MODEL_FOLLOW }],
      visibleWhen: COMPACT_LLM_ENABLED_RULE,
      subgroupKey: "compact_llm",
    },
    {
      section: "contextCompression",
      namespace: "chat",
      key: "compact_max_failures",
      label: t("fields.compactMaxFailures.label"),
      description: t("fields.compactMaxFailures.description"),
      type: "int",
      placeholder: t("fields.compactMaxFailures.placeholder"),
      visibleWhen: COMPACT_LLM_ENABLED_RULE,
      subgroupKey: "compact_llm",
    },
    {
      section: "contextCompression",
      namespace: "chat",
      key: "compact_system_prompt",
      label: t("fields.compactSystemPrompt.label"),
      description: t("fields.compactSystemPrompt.description"),
      type: "textarea",
      placeholder: t("fields.defaultPromptPlaceholder"),
      visibleWhen: COMPACT_LLM_ENABLED_RULE,
      subgroupKey: "compact_llm",
    },
    {
      section: "contextCompression",
      namespace: "chat",
      key: "compact_light_prompt",
      label: t("fields.compactLightPrompt.label"),
      description: t("fields.compactLightPrompt.description"),
      type: "textarea",
      placeholder: t("fields.defaultPromptPlaceholder"),
      visibleWhen: COMPACT_LLM_ENABLED_RULE,
      subgroupKey: "compact_llm",
    },
  ];
}

export function fieldID(field: ConversationSettingsField): string {
  return `${field.namespace}.${field.key}`;
}

export function flattenConversationSettings(grouped: SettingsGrouped): Record<string, string> {
  const result: Record<string, string> = {};
  for (const item of grouped.chat ?? []) {
    result[`chat.${item.key}`] = item.value ?? "";
  }
  return applyConversationDefaults(result);
}

export function applyConversationDefaults(settings: Record<string, string>): Record<string, string> {
  const result = { ...settings };
  if (!(result["chat.conversation_task_model"] ?? "").trim()) {
    result["chat.conversation_task_model"] = CONVERSATION_TASK_MODEL_FOLLOW;
  }
  if (!(result["chat.model_option_policy_mode"] ?? "").trim()) {
    result["chat.model_option_policy_mode"] = "allowlist";
  }
  if (!(result["chat.model_option_allowed_paths"] ?? "").trim()) {
    result["chat.model_option_allowed_paths"] = DEFAULT_MODEL_OPTION_ALLOWED_PATHS;
  }
  if (!(result["chat.model_option_denied_paths"] ?? "").trim()) {
    result["chat.model_option_denied_paths"] = DEFAULT_MODEL_OPTION_DENIED_PATHS;
  }
  if (!(result["chat.compact_task_model"] ?? "").trim()) {
    result["chat.compact_task_model"] = CONVERSATION_TASK_MODEL_FOLLOW;
  }
  if (!(result["chat.context_artifact_retention_days"] ?? "").trim()) {
    result["chat.context_artifact_retention_days"] = "90";
  }
  result["chat.conversation_title_prompt"] = normalizeConversationPromptValue(result["chat.conversation_title_prompt"] ?? "");
  result["chat.conversation_labels_prompt"] = normalizeConversationPromptValue(result["chat.conversation_labels_prompt"] ?? "");
  result["chat.default_system_prompt"] = normalizeConversationPromptValue(result["chat.default_system_prompt"] ?? "");
  result["chat.compact_system_prompt"] = normalizeConversationPromptValue(result["chat.compact_system_prompt"] ?? "");
  result["chat.compact_light_prompt"] = normalizeConversationPromptValue(result["chat.compact_light_prompt"] ?? "");
  return result;
}

export function matchesConversationVisibilityRule(
  rule: ConversationVisibilityRule | undefined,
  settings: Record<string, string>,
): boolean {
  if (!rule) {
    return true;
  }
  if ("all" in rule) {
    return rule.all.every((item) => matchesConversationVisibilityRule(item, settings));
  }
  return (settings[rule.field] ?? "") === rule.equals;
}

export function resolveVisibleConversationFields(
  fields: ConversationSettingsField[],
  settings: Record<string, string>,
): ConversationSettingsField[] {
  return fields.filter((field) => matchesConversationVisibilityRule(field.visibleWhen, settings));
}

function normalizeConversationPromptValue(value: string): string {
  const normalized = value.trim();
  if (!normalized) return "";
  return value;
}

export function toEditorField(field: ConversationSettingsField) {
  return {
    id: fieldID(field),
    label: field.label,
    description: field.description,
    type: field.type,
    placeholder: field.placeholder,
    options: field.options,
  } as const;
}
