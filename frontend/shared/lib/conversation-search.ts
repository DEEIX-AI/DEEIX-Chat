import type { ConversationDTO } from "@/shared/api/conversation.types";
import { parseConversationLabelsJSON } from "@/shared/lib/conversation-labels";

export function normalizeConversationSearchText(value: string) {
  return value.trim().toLocaleLowerCase();
}

export function conversationSearchHaystacks(item: ConversationDTO): string[] {
  return [
    item.publicID,
    item.title,
    ...parseConversationLabelsJSON(item.labelsJSON),
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
