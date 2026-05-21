import {
  AudioLines,
  Bot,
  ImageIcon,
  Paintbrush,
  Video,
} from "lucide-react";
import type { LucideIcon } from "lucide-react";

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

export const PROTOCOL_OPTIONS = [
  { value: "openai_responses", label: "Responses API (OpenAI)" },
  { value: "openai_chat_completions", label: "Chat Completions (OpenAI)" },
  { value: "openai_image_generations", label: "Image Generations (OpenAI)" },
  { value: "anthropic_messages", label: "Messages (Anthropic)" },
  { value: "google_generate_content", label: "Generate Content (Google)" },
  { value: "xai_responses", label: "Responses (xAI)" },
] as const;

const PROTOCOL_LABELS: Record<string, string> = {
  ...Object.fromEntries(PROTOCOL_OPTIONS.map((item) => [item.value, item.label])),
  openai_image_edits: "Image Edits (OpenAI)",
  openai_video_generations: "Video Generations (OpenAI)",
  google_imagen: "Imagen (Google)",
};

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
