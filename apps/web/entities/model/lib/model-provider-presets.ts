import type { ProviderImportProtocol } from "@deeix/core";

export type ModelProviderPreset = {
  id: string;
  name: string;
  /** Canonical base URL; adapters add the API version segment themselves. */
  baseURL: string;
  protocol: ProviderImportProtocol;
  /** Bundled LobeHub icon slug. */
  icon: string;
};

/**
 * Well-known providers, matched by address host: a recognised address picks
 * the protocol and the default name and icon.
 */
const MODEL_PROVIDER_PRESETS: readonly ModelProviderPreset[] = [
  { id: "openai", name: "OpenAI", baseURL: "https://api.openai.com/v1", protocol: "openai_responses", icon: "openai" },
  { id: "anthropic", name: "Anthropic", baseURL: "https://api.anthropic.com", protocol: "anthropic_messages", icon: "anthropic" },
  { id: "google", name: "Google Gemini", baseURL: "https://generativelanguage.googleapis.com", protocol: "google_generate_content", icon: "gemini" },
  { id: "xai", name: "xAI", baseURL: "https://api.x.ai/v1", protocol: "xai_responses", icon: "xai" },
  { id: "deepseek", name: "DeepSeek", baseURL: "https://api.deepseek.com/v1", protocol: "openai_chat_completions", icon: "deepseek" },
  { id: "openrouter", name: "OpenRouter", baseURL: "https://openrouter.ai/api/v1", protocol: "openrouter_chat_completions", icon: "openrouter" },
];

function hostOf(url: string): string {
  const match = /^https?:\/\/([^/:?#]+)/i.exec(url.trim());
  return match?.[1]?.toLowerCase() ?? "";
}

/** The preset whose host matches `baseURL`, so an imported link shows a familiar name and icon. */
export function findModelProviderPreset(baseURL: string): ModelProviderPreset | null {
  const host = hostOf(baseURL);
  if (!host) return null;
  return MODEL_PROVIDER_PRESETS.find((preset) => hostOf(preset.baseURL) === host) ?? null;
}

/**
 * Icon slug for a user's provider: their own pick first, else the preset
 * matched by address, else empty (callers fall back to the name's initial).
 * Accepts a bare host as well as a full URL.
 */
export function resolveModelProviderIcon(icon: string | null | undefined, address: string): string {
  const picked = icon?.trim() ?? "";
  if (picked) return picked;
  const base = /^https?:\/\//i.test(address) ? address : `https://${address}`;
  return findModelProviderPreset(base)?.icon ?? "";
}

