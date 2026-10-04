import { isRecord, parseJSON } from "@/shared/lib/type-guards";

/**
 * Parses the model capabilities JSON as an object. Empty input is an empty object; anything that is not
 * a JSON object returns null so callers can report the invalid JSON instead of overwriting it.
 */
export function parseCapabilitiesObject(value: string | null | undefined): Record<string, unknown> | null {
  const normalized = value?.trim() ?? "";
  if (!normalized) {
    return {};
  }
  const parsed = parseJSON(normalized);
  return isRecord(parsed) ? parsed : null;
}

/** Serializes the capabilities object for the editor; an empty object becomes an empty string. */
export function stringifyCapabilitiesObject(payload: Record<string, unknown>): string {
  return Object.keys(payload).length > 0 ? JSON.stringify(payload, null, 2) : "";
}
