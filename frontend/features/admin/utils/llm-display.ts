import {
  AudioLines,
  Bot,
  ImageIcon,
  Paintbrush,
  Video,
} from "lucide-react";
import type { LucideIcon } from "lucide-react";
import type { AdminLLMAdapter } from "@/features/admin/api/llm.types";

export const MODEL_KIND_META: Record<
  string,
  { label: string; shortLabel: string; icon: LucideIcon }
> = {
  chat: { label: "Chat", shortLabel: "Chat", icon: Bot },
  audio: { label: "Audio", shortLabel: "Audio", icon: AudioLines },
  image_gen: { label: "Image generation", shortLabel: "Image generation", icon: ImageIcon },
  image_edit: { label: "Image editing", shortLabel: "Image editing", icon: Paintbrush },
  video_gen: { label: "Video generation", shortLabel: "Video generation", icon: Video },
};

export const COMPATIBLE_OPTIONS = [
  { label: "OpenAI", value: "openai" },
  { label: "Anthropic", value: "anthropic" },
  { label: "Google", value: "google" },
  { label: "xAI", value: "xai" },
  { label: "OpenRouter", value: "openrouter" },
  { label: "Custom", value: "custom" },
] as const;

export const PROTOCOL_OPTIONS: ReadonlyArray<{ value: AdminLLMAdapter; label: string; kind: string }> = [
  { value: "openai_responses", label: "Responses API (OpenAI)", kind: "chat" },
  { value: "openai_chat_completions", label: "Chat Completions (OpenAI)", kind: "chat" },
  { value: "openai_image_generations", label: "Image Generations (OpenAI)", kind: "image_gen" },
  { value: "openai_image_edits", label: "Image Edits (OpenAI)", kind: "image_edit" },
  { value: "openai_video_generations", label: "Video Generations (OpenAI)", kind: "video_gen" },
  { value: "anthropic_messages", label: "Messages (Anthropic)", kind: "chat" },
  { value: "google_generate_content", label: "Generate Content (Google)", kind: "chat" },
  { value: "google_image_generation", label: "Image Generation (Google)", kind: "image_gen" },
  { value: "xai_responses", label: "Responses (xAI)", kind: "chat" },
  { value: "xai_image", label: "Image Generation (xAI)", kind: "image_gen" },
] as const;

const PROTOCOL_LABELS: Record<string, string> = {
  ...Object.fromEntries(PROTOCOL_OPTIONS.map((item) => [item.value, item.label])),
};

const PROTOCOL_KINDS: Record<string, string> = {
  ...Object.fromEntries(PROTOCOL_OPTIONS.map((item) => [item.value, item.kind])),
};

const IMAGE_ROUTE_PROTOCOL_PAIR: ReadonlySet<AdminLLMAdapter> = new Set([
  "openai_image_generations",
  "openai_image_edits",
]);

const LLM_STATUS_LABELS: Record<string, string> = {
  active: "Enabled",
  inactive: "Disabled",
};

const BINDING_STATUS_LABELS: Record<string, string> = {
  available: "Ready to import",
  mapped: "Bound",
  existing: "Existing",
  created: "Created",
  failed: "Failed",
};

export function resolveKindLabel(kind: string): string {
  return MODEL_KIND_META[kind]?.shortLabel ?? kind;
}

export function resolveProtocolLabel(protocol: string): string {
  return PROTOCOL_LABELS[protocol] ?? protocol;
}

export function isSupportedRouteProtocolSelection(protocols: readonly AdminLLMAdapter[]): boolean {
  const uniqueProtocols = Array.from(new Set(protocols));
  if (uniqueProtocols.length <= 1) {
    return true;
  }
  return uniqueProtocols.length === 2 && uniqueProtocols.every((protocol) => IMAGE_ROUTE_PROTOCOL_PAIR.has(protocol));
}

export function resolveNextRouteProtocolSelection(
  currentProtocols: readonly AdminLLMAdapter[],
  protocol: AdminLLMAdapter,
): AdminLLMAdapter[] {
  const current = Array.from(new Set(currentProtocols));
  if (current.includes(protocol)) {
    return current.filter((item) => item !== protocol);
  }
  const candidate = [...current, protocol];
  if (isSupportedRouteProtocolSelection(candidate)) {
    return candidate;
  }
  return [protocol];
}

export function resolveKindsDisplayForProtocols(
  protocols: readonly AdminLLMAdapter[],
  fallbackDisplay = "chat",
): string {
  const kinds = Array.from(new Set(protocols.map((protocol) => PROTOCOL_KINDS[protocol]).filter(Boolean)));
  return kinds.length > 0 ? kinds.join(",") : fallbackDisplay;
}

export function resolveCompatibleLabel(compatible: string): string {
  return COMPATIBLE_OPTIONS.find((item) => item.value === compatible)?.label ?? (compatible || "-");
}

export function resolveLLMStatusLabel(status: string | null | undefined): string {
  const key = status?.trim() ?? "";
  return LLM_STATUS_LABELS[key] ?? (status?.trim() || "-");
}

export function resolveBindingStatusLabel(status: string | null | undefined, alreadyBound = false): string {
  const key = status?.trim() ?? "";
  if (!key && alreadyBound) {
    return "Bound";
  }
  return BINDING_STATUS_LABELS[key] ?? (key || "Ready to import");
}
