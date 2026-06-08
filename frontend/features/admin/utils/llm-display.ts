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

type ProtocolOption = {
  value: AdminLLMAdapter;
  label: string;
  kinds: readonly string[];
};

export const PROTOCOL_OPTIONS: ReadonlyArray<ProtocolOption> = [
  { value: "openai_responses", label: "Responses API (OpenAI)", kinds: ["chat"] },
  { value: "openrouter_responses", label: "Responses (OpenRouter)", kinds: ["chat"] },
  { value: "openai_chat_completions", label: "Chat Completions (OpenAI)", kinds: ["chat"] },
  { value: "openai_image_generations", label: "Images Generations (OpenAI)", kinds: ["image_gen"] },
  { value: "openai_image_edits", label: "Images Edits (OpenAI)", kinds: ["image_edit"] },
  { value: "openai_video_generations", label: "Video Generations (OpenAI)", kinds: ["video_gen"] },
  { value: "anthropic_messages", label: "Messages (Anthropic)", kinds: ["chat"] },
  { value: "google_generate_content", label: "Generate Content (Google)", kinds: ["chat"] },
  { value: "google_image_generation", label: "Image Generation (Google)", kinds: ["image_gen", "image_edit"] },
  { value: "xai_responses", label: "Responses (xAI)", kinds: ["chat"] },
  { value: "xai_image", label: "Images Generations (xAI)", kinds: ["image_gen"] },
  { value: "xai_image_edits", label: "Images Edits (xAI)", kinds: ["image_edit"] },
] as const;

const PROTOCOL_LABELS: Record<string, string> = {
  ...Object.fromEntries(PROTOCOL_OPTIONS.map((item) => [item.value, item.label])),
};

const PROTOCOL_KINDS: Record<string, readonly string[]> = {
  ...Object.fromEntries(PROTOCOL_OPTIONS.map((item) => [item.value, item.kinds])),
};

const PROTOCOL_DISPLAY_ORDER = new Map<string, number>(
  PROTOCOL_OPTIONS.map((item, index) => [item.value, index]),
);

const IMAGE_ROUTE_PROTOCOL_PAIRS: ReadonlyArray<readonly [AdminLLMAdapter, AdminLLMAdapter]> = [
  ["openai_image_generations", "openai_image_edits"],
  ["xai_image", "xai_image_edits"],
];

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

export function sortProtocolsForDisplay<T extends string>(protocols: readonly T[]): T[] {
  const seen = new Set<string>();
  return protocols
    .map((protocol, index) => ({ protocol, index }))
    .filter(({ protocol }) => {
      const key = String(protocol || "").trim();
      if (!key || seen.has(key)) {
        return false;
      }
      seen.add(key);
      return true;
    })
    .sort((a, b) => {
      const orderA = PROTOCOL_DISPLAY_ORDER.get(a.protocol);
      const orderB = PROTOCOL_DISPLAY_ORDER.get(b.protocol);
      if (orderA !== undefined && orderB !== undefined) {
        return orderA - orderB;
      }
      if (orderA !== undefined) {
        return -1;
      }
      if (orderB !== undefined) {
        return 1;
      }
      return a.index - b.index;
    })
    .map(({ protocol }) => protocol);
}

export function isSupportedRouteProtocolSelection(protocols: readonly AdminLLMAdapter[]): boolean {
  const uniqueProtocols = Array.from(new Set(protocols));
  if (uniqueProtocols.length <= 1) {
    return true;
  }
  return uniqueProtocols.length === 2 && IMAGE_ROUTE_PROTOCOL_PAIRS.some(([generationProtocol, editProtocol]) =>
    uniqueProtocols.includes(generationProtocol) && uniqueProtocols.includes(editProtocol),
  );
}

export function resolveNextRouteProtocolSelection(
  currentProtocols: readonly AdminLLMAdapter[],
  protocol: AdminLLMAdapter,
): AdminLLMAdapter[] {
  const current = sortProtocolsForDisplay(currentProtocols);
  if (current.includes(protocol)) {
    return sortProtocolsForDisplay(current.filter((item) => item !== protocol));
  }
  const candidate = sortProtocolsForDisplay([...current, protocol]);
  if (isSupportedRouteProtocolSelection(candidate)) {
    return candidate;
  }
  return [protocol];
}

export function resolveKindsDisplayForProtocols(
  protocols: readonly AdminLLMAdapter[],
  fallbackDisplay = "chat",
): string {
  const kinds = Array.from(new Set(protocols.flatMap((protocol) => PROTOCOL_KINDS[protocol] ?? [])));
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
