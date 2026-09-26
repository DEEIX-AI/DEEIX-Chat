export const MODEL_CONTEXT_WINDOW_MIN = 4_096;
export const MODEL_CONTEXT_WINDOW_MAX = 16_000_000;

export const MODEL_CONTEXT_WINDOW_PRESETS = [
  { label: "64K", value: 64_000 },
  { label: "128K", value: 128_000 },
  { label: "256K", value: 256_000 },
  { label: "500K", value: 500_000 },
  { label: "1M", value: 1_000_000 },
] as const;

function parseCapabilitiesObject(value: string | null | undefined): Record<string, unknown> | null {
  const normalized = value?.trim() ?? "";
  if (!normalized) {
    return {};
  }
  try {
    const parsed = JSON.parse(normalized) as unknown;
    return parsed !== null && typeof parsed === "object" && !Array.isArray(parsed)
      ? parsed as Record<string, unknown>
      : null;
  } catch {
    return null;
  }
}

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
  return Object.keys(payload).length > 0 ? JSON.stringify(payload, null, 2) : "";
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
  return Object.keys(payload).length > 0 ? JSON.stringify(payload, null, 2) : "";
}
