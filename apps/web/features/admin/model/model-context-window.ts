import { parseCapabilitiesObject, stringifyCapabilitiesObject } from "@/features/admin/model/capabilities-json";

export const MODEL_CONTEXT_WINDOW_MIN = 4_096;
export const MODEL_CONTEXT_WINDOW_MAX = 16_000_000;

export const MODEL_CONTEXT_WINDOW_PRESETS = [
  { label: "64K", value: 64_000 },
  { label: "128K", value: 128_000 },
  { label: "256K", value: 256_000 },
  { label: "500K", value: 500_000 },
  { label: "1M", value: 1_000_000 },
] as const;

function positiveIntegerProperty(
  payload: Record<string, unknown>,
  keys: readonly string[],
): number | null {
  for (const key of keys) {
    const raw = payload[key];
    const parsed = typeof raw === "number" ? raw : typeof raw === "string" ? Number(raw.trim()) : Number.NaN;
    if (Number.isSafeInteger(parsed) && parsed > 0) {
      return parsed;
    }
  }
  return null;
}

export function isValidModelContextWindow(value: number): boolean {
  return Number.isSafeInteger(value)
    && value >= MODEL_CONTEXT_WINDOW_MIN
    && value <= MODEL_CONTEXT_WINDOW_MAX;
}

/** Context window declared by a catalog, or null when absent or outside the range the platform accepts. */
export function normalizeCatalogContextWindow(value: number | null | undefined): number | null {
  return typeof value === "number" && isValidModelContextWindow(value) ? value : null;
}

export function modelContextWindowOverride(value: string | null | undefined): number | null {
  const payload = parseCapabilitiesObject(value);
  if (!payload || payload._deeixContextWindowMode === "auto") {
    return null;
  }
  return positiveIntegerProperty(
    payload,
    ["contextWindow", "context_window", "contextWindowTokens", "context_window_tokens"],
  );
}

export function modelMaxOutputTokensOverride(value: string | null | undefined): number | null {
  const payload = parseCapabilitiesObject(value);
  return payload
    ? positiveIntegerProperty(payload, ["maxOutputTokens", "max_output_tokens"])
    : null;
}

export function setModelContextWindowInCapabilities(
  value: string | null | undefined,
  contextWindow: number | null,
): string | null {
  const payload = parseCapabilitiesObject(value);
  if (!payload) {
    return null;
  }
  delete payload._deeixContextWindowMode;
  delete payload.context_window;
  delete payload.contextWindowTokens;
  delete payload.context_window_tokens;
  if (contextWindow === null) {
    delete payload.contextWindow;
  } else {
    payload.contextWindow = contextWindow;
  }
  return stringifyCapabilitiesObject(payload);
}

export function setAutomaticModelContextWindowInCapabilities(
  value: string | null | undefined,
  contextWindow: number | null,
): string | null {
  const payload = parseCapabilitiesObject(value);
  if (!payload) {
    return null;
  }
  delete payload.context_window;
  delete payload.contextWindowTokens;
  delete payload.context_window_tokens;
  if (contextWindow === null) {
    if (payload._deeixContextWindowMode === "auto") {
      delete payload.contextWindow;
      delete payload._deeixContextWindowMode;
    }
  } else {
    payload.contextWindow = contextWindow;
    payload._deeixContextWindowMode = "auto";
  }
  return stringifyCapabilitiesObject(payload);
}
