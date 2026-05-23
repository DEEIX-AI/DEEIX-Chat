import type { AdminLLMModelDTO, AdminLLMModelVendor, AdminLLMStatus } from "@/features/admin/api/llm.types";
import { resolveLocalizedErrorMessage } from "@/i18n/resolve-error-message";
import { MODEL_KIND_META, resolveProtocolLabel } from "@/features/admin/utils/llm-display";
import { parseKindsJSON, stringifyKinds } from "@/shared/model/llm-schema";
import { KNOWN_VENDOR_OPTIONS } from "@/shared/lib/model-identity";

// ---------------------------------------------------------------------------
// Constants
// ---------------------------------------------------------------------------

export const PAGE_SIZE_DEFAULT = 25;

export const MODEL_VENDOR_OPTIONS: AdminLLMModelVendor[] = KNOWN_VENDOR_OPTIONS.map((item) => item.value);

export const MODEL_STATUS_OPTIONS: AdminLLMStatus[] = ["active", "inactive"];

export const MODEL_SORT_OPTIONS = [
  { labelKey: "sort.sortOrderAsc", value: "sortOrder_asc" },
  { labelKey: "sort.updatedDesc", value: "updated_desc" },
  { labelKey: "sort.idDesc", value: "id_desc" },
  { labelKey: "sort.platformModelNameAsc", value: "platformModelName_asc" },
  { labelKey: "sort.sourceCountDesc", value: "sourceCount_desc" },
] as const;

export type ModelSortValue = (typeof MODEL_SORT_OPTIONS)[number]["value"];

export const ADAPTER_LABELS: Record<string, string> = {
  openai_responses:         resolveProtocolLabel("openai_responses"),
  openai_chat_completions:  resolveProtocolLabel("openai_chat_completions"),
  openai_image_generations: resolveProtocolLabel("openai_image_generations"),
  openai_image_edits:       resolveProtocolLabel("openai_image_edits"),
  openai_video_generations: resolveProtocolLabel("openai_video_generations"),
  anthropic_messages:       resolveProtocolLabel("anthropic_messages"),
  google_generate_content:  resolveProtocolLabel("google_generate_content"),
  google_image_generation:  resolveProtocolLabel("google_image_generation"),
  xai_responses:            resolveProtocolLabel("xai_responses"),
  xai_image:                resolveProtocolLabel("xai_image"),
};

// ---------------------------------------------------------------------------
// Types
// ---------------------------------------------------------------------------

export const MODEL_KIND_OPTIONS = Object.entries(MODEL_KIND_META).map(([value, meta]) => ({
  value,
  label: meta.label,
}));

export type ModelFormPayload = {
  platformModelName: string;
  vendor: AdminLLMModelVendor | "";
  kinds: string[];
  icon: string;
  capabilitiesJSON: string;
  systemPrompt: string;
  status: AdminLLMStatus;
  description: string;
};

// ---------------------------------------------------------------------------
// Utilities
// ---------------------------------------------------------------------------

export function resolveErrorMessage(error: unknown): string {
  return resolveLocalizedErrorMessage(error);
}

export function formatDateTime(value: string | null | undefined, locale = "en-US"): string {
  if (!value) return "-";
  const date = new Date(value);
  if (Number.isNaN(date.getTime())) return "-";
  return new Intl.DateTimeFormat(locale, {
    year: "numeric",
    month: "2-digit",
    day: "2-digit",
    hour: "2-digit",
    minute: "2-digit",
  }).format(date);
}

export function resolveValue(value: string | null | undefined): string {
  return value?.trim() || "-";
}

export function kindsJsonToDisplay(kindsJson: string | null | undefined): string {
  return parseKindsJSON(kindsJson).join(",");
}

export function displayToKindsJson(display: string): string {
  return stringifyKinds(display
    .split(",")
    .map((s) => s.trim())
    .filter(Boolean));
}

export function mapModelToForm(model: AdminLLMModelDTO): ModelFormPayload {
  const kinds = parseKindsJSON(model.kindsJSON);
  return {
    platformModelName: model.platformModelName,
    vendor: model.vendor ?? "",
    kinds,
    icon: model.icon ?? "",
    capabilitiesJSON: model.capabilitiesJSON ?? "",
    systemPrompt: model.systemPrompt ?? "",
    status: model.status,
    description: model.description ?? "",
  };
}
