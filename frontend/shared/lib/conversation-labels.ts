export const MAX_CONVERSATION_LABELS = 6;
export const MAX_CONVERSATION_LABEL_LENGTH = 24;

export function normalizeConversationLabel(value: string): string {
  return value.trim().replace(/^#+/u, "").trim().replace(/\s+/gu, " ");
}

export function normalizeConversationLabels(values: readonly string[]): string[] {
  const labels: string[] = [];
  const seen = new Set<string>();
  for (const item of values) {
    const label = normalizeConversationLabel(item);
    const key = label.toLowerCase();
    if (!label || Array.from(label).length > MAX_CONVERSATION_LABEL_LENGTH || seen.has(key)) {
      continue;
    }
    seen.add(key);
    labels.push(label);
    if (labels.length >= MAX_CONVERSATION_LABELS) {
      break;
    }
  }
  return labels;
}

export function parseConversationLabelsJSON(labelsJSON: string): string[] {
  const source = labelsJSON.trim();
  if (!source || source === "null" || source === "[]") {
    return [];
  }
  try {
    const parsed = JSON.parse(source) as unknown;
    return Array.isArray(parsed)
      ? normalizeConversationLabels(parsed.filter((item): item is string => typeof item === "string"))
      : [];
  } catch {
    return normalizeConversationLabels([source]);
  }
}
