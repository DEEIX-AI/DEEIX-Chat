import type { ConversationDTO } from "@/shared/api/conversation.types";

export function normalizeConversationSearchText(value: string) {
  return value.trim().toLocaleLowerCase();
}

function parseConversationLabels(labelsJSON: string): string[] {
  const source = labelsJSON.trim();
  if (!source || source === "null" || source === "[]") {
    return [];
  }
  try {
    const parsed = JSON.parse(source) as unknown;
    if (Array.isArray(parsed)) {
      return parsed
        .map((item) => (typeof item === "string" ? item.trim() : ""))
        .filter(Boolean);
    }
  } catch {
    return [source];
  }
  return [source];
}

export function conversationSearchHaystacks(item: ConversationDTO): string[] {
  return [
    item.publicID,
    item.title,
    ...parseConversationLabels(item.labelsJSON),
    item.model,
  ].filter((value): value is string => Boolean(value?.trim()));
}

export function conversationSearchText(item: ConversationDTO): string {
  return conversationSearchHaystacks(item).join(" ");
}

export function conversationMatchesSearch(item: ConversationDTO, normalizedQuery: string): boolean {
  if (!normalizedQuery) {
    return true;
  }
  return conversationSearchHaystacks(item).some((value) =>
    value.toLocaleLowerCase().includes(normalizedQuery),
  );
}
