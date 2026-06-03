import type { SettingsGrouped } from "@/shared/api/settings.types";
import { resolveLocalizedErrorMessage } from "@/i18n/resolve-error-message";

export type ConversationFieldType = "int" | "bool" | "string" | "password" | "textarea" | "json" | "select" | "tabs" | "button";

export type ConversationSettingsField = {
  namespace: "chat";
  key:
    | "conversation_task_model"
    | "default_system_prompt"
    | "conversation_title_prompt"
    | "conversation_labels_prompt"
    | "model_option_policy_mode"
    | "model_option_allowed_paths"
    | "model_option_denied_paths";
  label: string;
  description: string;
  type: ConversationFieldType;
  placeholder?: string;
  options?: Array<{ label: string; value: string }>;
};

export const CONVERSATION_TASK_MODEL_FOLLOW = "follow";

export const DEFAULT_MODEL_OPTION_ALLOWED_PATHS = `{
  "default": [
    "temperature",
    "top_p",
    "max_tokens",
    "max_output_tokens",
    "max_completion_tokens",
    "stop",
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
    "thinking.type",
    "thinking.budget_tokens"
  ],
  "xai_responses": [
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
      namespace: "chat",
      key: "conversation_task_model",
      label: t("fields.taskModel.label"),
      description: t("fields.taskModel.description"),
      type: "select",
      options: [{ label: t("taskModel.follow"), value: CONVERSATION_TASK_MODEL_FOLLOW }],
    },
    {
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
      namespace: "chat",
      key: "model_option_allowed_paths",
      label: t("fields.allowedPaths.label"),
      description: t("fields.allowedPaths.description"),
      type: "json",
      placeholder: DEFAULT_MODEL_OPTION_ALLOWED_PATHS,
    },
    {
      namespace: "chat",
      key: "model_option_denied_paths",
      label: t("fields.deniedPaths.label"),
      description: t("fields.deniedPaths.description"),
      type: "json",
      placeholder: DEFAULT_MODEL_OPTION_DENIED_PATHS,
    },
    {
      namespace: "chat",
      key: "conversation_title_prompt",
      label: t("fields.titlePrompt.label"),
      description: t("fields.titlePrompt.description"),
      type: "textarea",
      placeholder: t("fields.defaultPromptPlaceholder"),
    },
    {
      namespace: "chat",
      key: "conversation_labels_prompt",
      label: t("fields.labelsPrompt.label"),
      description: t("fields.labelsPrompt.description"),
      type: "textarea",
      placeholder: t("fields.defaultPromptPlaceholder"),
    },
    {
      namespace: "chat",
      key: "default_system_prompt",
      label: t("fields.defaultSystemPrompt.label"),
      description: t("fields.defaultSystemPrompt.description"),
      type: "textarea",
      placeholder: t("fields.defaultSystemPrompt.placeholder"),
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
  result["chat.conversation_title_prompt"] = normalizeConversationPromptValue(result["chat.conversation_title_prompt"] ?? "");
  result["chat.conversation_labels_prompt"] = normalizeConversationPromptValue(result["chat.conversation_labels_prompt"] ?? "");
  result["chat.default_system_prompt"] = normalizeConversationPromptValue(result["chat.default_system_prompt"] ?? "");
  return result;
}

function normalizeConversationPromptValue(value: string): string {
  const normalized = value.trim();
  if (!normalized) return "";
  return value;
}

export function resolveErrorMessage(error: unknown): string {
  return resolveLocalizedErrorMessage(error);
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
