export const MODEL_OPTION_POLICY_PROTOCOLS = [
  "default",
  "openai_chat_completions",
  "openai_responses",
  "openai_image_generations",
  "openai_image_edits",
  "anthropic_messages",
  "gemini_generate_content",
  "google_image_generation",
  "xai_responses",
  "xai_image",
] as const;

export type ModelOptionPolicyProtocol = (typeof MODEL_OPTION_POLICY_PROTOCOLS)[number];
export type ModelOptionPolicyMode = "allowlist" | "denylist" | "disabled" | string;

export type ModelOptionPolicy = {
  mode: ModelOptionPolicyMode;
  allowedPathsJSON: string;
  deniedPathsJSON: string;
  nativeToolAllowedTypesJSON: string;
};

export type ModelOptionRuleMap = Partial<Record<ModelOptionPolicyProtocol | string, string[]>>;

export const DEFAULT_NATIVE_TOOL_ALLOWED_TYPES = `{
  "openai_chat_completions": [
    "web_search",
    "web_search_preview"
  ],
  "openai_responses": [
    "web_search",
    "web_search_preview",
    "shell",
    "image_generation",
    "code_interpreter"
  ],
  "anthropic_messages": [
    "web_search_20250305",
    "web_search_20260209",
    "web_fetch_20250910",
    "web_fetch_20260209",
    "code_execution_20250825",
    "code_execution_20260120",
    "advisor_20260301",
    "tool_search_tool_regex_20251119",
    "tool_search_tool_bm25_20251119"
  ],
  "xai_responses": [
    "web_search",
    "x_search",
    "code_interpreter"
  ]
}`;

const DEFAULT_NATIVE_TOOL_ALLOWED_TYPES_MAP = parseModelOptionRuleMap(DEFAULT_NATIVE_TOOL_ALLOWED_TYPES).value;

export const MODEL_OPTION_POLICY_PROTOCOL_LABELS: Record<ModelOptionPolicyProtocol, string> = {
  default: "Default",
  openai_chat_completions: "OpenAI（Chat Completions）",
  openai_responses: "OpenAI（Responses）",
  openai_image_generations: "OpenAI（Image Generation）",
  openai_image_edits: "OpenAI（Image Edits）",
  anthropic_messages: "Anthropic（Messages）",
  gemini_generate_content: "Google（Generate Content）",
  google_image_generation: "Google（Image Generation）",
  xai_responses: "xAI（Responses）",
  xai_image: "xAI（Image Generation）",
};

export const HARD_DENIED_MODEL_OPTION_PATHS = [
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
  "previous_response_id",
];

export function parseModelOptionRuleMap(raw: string): { value: ModelOptionRuleMap; error: string } {
  try {
    const parsed = JSON.parse(raw || "{}") as unknown;
    if (!parsed || typeof parsed !== "object" || Array.isArray(parsed)) {
      return { value: {}, error: "Configuration must be a JSON object" };
    }
    const value: ModelOptionRuleMap = {};
    for (const [key, paths] of Object.entries(parsed as Record<string, unknown>)) {
      if (!Array.isArray(paths)) {
        return { value: {}, error: `${key} must be a string array` };
      }
      value[key] = paths.map((path) => (typeof path === "string" ? path.trim() : "")).filter(Boolean);
    }
    return { value, error: "" };
  } catch {
    return { value: {}, error: "Invalid JSON format" };
  }
}

export function uniqueModelOptionPaths(paths: string[]): string[] {
  return [...new Set(paths.map((path) => path.trim()).filter(Boolean))];
}

export function resolveModelOptionPolicyProtocol(protocol: string): ModelOptionPolicyProtocol {
  switch (protocol.trim()) {
    case "openai_chat_completions":
      return "openai_chat_completions";
    case "openai_image_generations":
      return "openai_image_generations";
    case "openai_image_edits":
      return "openai_image_edits";
    case "anthropic_messages":
      return "anthropic_messages";
    case "xai_responses":
      return "xai_responses";
    case "xai_image":
      return "xai_image";
    case "google_generate_content":
    case "gemini_generate_content":
      return "gemini_generate_content";
    case "google_image_generation":
      return "google_image_generation";
    case "openai_responses":
    default:
      return "openai_responses";
  }
}

export function effectiveModelOptionPaths(rules: ModelOptionRuleMap, protocol: string): string[] {
  if (protocol === "default") {
    return uniqueModelOptionPaths(rules.default ?? []);
  }
  return uniqueModelOptionPaths([...(rules.default ?? []), ...(rules[protocol] ?? [])]);
}

export function isNativeToolTypeAllowed(policy: ModelOptionPolicy | null, protocol: string, toolType: string): boolean {
  if ((policy?.mode?.trim() || "allowlist") === "disabled") {
    return false;
  }
  const policyProtocol = resolveModelOptionPolicyProtocol(protocol);
  const defaults = uniqueModelOptionPaths(DEFAULT_NATIVE_TOOL_ALLOWED_TYPES_MAP[policyProtocol] ?? []);
  if (defaults.length === 0 || !defaults.includes(toolType)) {
    return false;
  }
  const raw = policy?.nativeToolAllowedTypesJSON?.trim();
  if (!raw) {
    return true;
  }
  const configured = parseModelOptionRuleMap(raw).value;
  const configuredTypes = configured[policyProtocol];
  if (!configuredTypes) {
    return true;
  }
  return uniqueModelOptionPaths(configuredTypes).includes(toolType);
}

function pathSegments(path: string): string[] {
  return path.split(".").map((item) => item.trim()).filter(Boolean);
}

function ruleAffectsPath(rule: string, path: string): boolean {
  const ruleParts = pathSegments(rule);
  const pathParts = pathSegments(path);
  if (ruleParts.length === 0 || pathParts.length === 0 || ruleParts.length > pathParts.length) {
    return false;
  }
  return ruleParts.every((part, index) => part === pathParts[index]);
}

export function isModelOptionPathFiltered({
  policy,
  protocol,
  path,
}: {
  policy: ModelOptionPolicy;
  protocol: string;
  path: string;
}): boolean {
  const mode = policy.mode?.trim() || "allowlist";
  if (mode === "disabled") {
    return true;
  }

  const policyProtocol = resolveModelOptionPolicyProtocol(protocol);
  const allowed = parseModelOptionRuleMap(policy.allowedPathsJSON).value;
  const denied = parseModelOptionRuleMap(policy.deniedPathsJSON).value;
  const deniedPaths = uniqueModelOptionPaths([
    ...HARD_DENIED_MODEL_OPTION_PATHS,
    ...(mode === "denylist" ? effectiveModelOptionPaths(denied, policyProtocol) : []),
  ]);
  if (deniedPaths.some((rule) => ruleAffectsPath(rule, path))) {
    return true;
  }

  if (mode === "denylist") {
    return false;
  }

  const allowedPaths = effectiveModelOptionPaths(allowed, policyProtocol);
  return !allowedPaths.some((rule) => ruleAffectsPath(rule, path));
}
