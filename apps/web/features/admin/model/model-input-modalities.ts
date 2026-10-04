import { parseCapabilitiesObject, stringifyCapabilitiesObject } from "@/features/admin/model/capabilities-json";

// Keep in sync with the backend (domain/channel/model_input_modalities.go).
export const MODEL_MODALITIES = ["text", "image", "pdf", "audio", "video"] as const;

export type ModelModality = (typeof MODEL_MODALITIES)[number];

// Modalities an administrator can toggle; text is always accepted.
export const CONFIGURABLE_INPUT_MODALITIES = ["image", "pdf", "audio", "video"] as const satisfies readonly ModelModality[];

const INPUT_MODALITIES_KEY = "inputModalities";

export function normalizeModalities(values: readonly unknown[] | null | undefined): ModelModality[] {
  if (!values) {
    return [];
  }
  return MODEL_MODALITIES.filter((modality) => values.includes(modality));
}

/** Explicit `inputModalities` declared in the capabilities JSON, or null when following the catalog. */
export function modelInputModalitiesOverride(value: string | null | undefined): ModelModality[] | null {
  const payload = parseCapabilitiesObject(value);
  const raw = payload?.[INPUT_MODALITIES_KEY];
  if (!Array.isArray(raw)) {
    return null;
  }
  const modalities = normalizeModalities([...raw, "text"]);
  return modalities.length > 0 ? modalities : null;
}

/**
 * Writes or removes the `inputModalities` override; null restores automatic detection.
 * Returns null when the capabilities JSON is not an object (the caller reports it).
 */
export function setModelInputModalitiesInCapabilities(
  value: string | null | undefined,
  modalities: readonly ModelModality[] | null,
): string | null {
  const payload = parseCapabilitiesObject(value);
  if (!payload) {
    return null;
  }
  if (modalities === null) {
    delete payload[INPUT_MODALITIES_KEY];
  } else {
    payload[INPUT_MODALITIES_KEY] = normalizeModalities([...modalities, "text"]);
  }
  return stringifyCapabilitiesObject(payload);
}
