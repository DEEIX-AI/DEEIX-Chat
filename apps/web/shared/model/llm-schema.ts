export type LLMKind =
  | "chat"
  | "audio"
  | "image_gen"
  | "image_edit"
  | "video_gen";

function normalizeKinds(kinds: string[]): string[] {
  const seen = new Set<string>();
  const result: string[] = [];
  for (const item of kinds) {
    const kind = item.trim().toLowerCase();
    if (!kind || seen.has(kind)) continue;
    seen.add(kind);
    result.push(kind);
  }
  return result;
}

export function parseKindsJSON(raw: string | null | undefined): string[] {
  if (!raw?.trim()) return [];
  try {
    const parsed = JSON.parse(raw) as unknown;
    if (!Array.isArray(parsed)) return [];
    return normalizeKinds(parsed.filter((item): item is string => typeof item === "string"));
  } catch {
    return [];
  }
}

export function stringifyKinds(kinds: string[]): string {
  const cleaned = normalizeKinds(kinds);
  return cleaned.length > 0 ? JSON.stringify(cleaned) : "";
}
