import type { ConversationDTO } from "@/shared/api/conversation.types";

type ConversationSorter = (items: ConversationDTO[]) => ConversationDTO[];

function toTimestamp(value: string | null | undefined): number {
  if (!value) {
    return 0;
  }
  const parsed = Date.parse(value);
  return Number.isNaN(parsed) ? 0 : parsed;
}

export function sortByUpdatedAtDesc(items: ConversationDTO[]): ConversationDTO[] {
  return [...items].sort((a, b) => {
    const diff = toTimestamp(b.updatedAt) - toTimestamp(a.updatedAt);
    if (diff !== 0) {
      return diff;
    }
    return b.publicID.localeCompare(a.publicID);
  });
}

export function sortByStarredAtDesc(items: ConversationDTO[]): ConversationDTO[] {
  return [...items].sort((a, b) => {
    const diff = toTimestamp(b.starredAt) - toTimestamp(a.starredAt);
    if (diff !== 0) {
      return diff;
    }
    const updatedDiff = toTimestamp(b.updatedAt) - toTimestamp(a.updatedAt);
    if (updatedDiff !== 0) {
      return updatedDiff;
    }
    return b.publicID.localeCompare(a.publicID);
  });
}

export function mergeUniqueByPublicID(
  base: ConversationDTO[],
  incoming: ConversationDTO[],
  sorter: ConversationSorter = sortByUpdatedAtDesc,
): ConversationDTO[] {
  const index = new Map<string, ConversationDTO>();
  for (const item of base) {
    index.set(item.publicID, item);
  }
  for (const item of incoming) {
    index.set(item.publicID, item);
  }
  return sorter(Array.from(index.values()));
}

export function upsertByPublicID(
  items: ConversationDTO[],
  incoming: ConversationDTO,
  sorter: ConversationSorter = sortByUpdatedAtDesc,
): ConversationDTO[] {
  return mergeUniqueByPublicID(items, [incoming], sorter);
}

export function removeByPublicID(items: ConversationDTO[], publicID: string): ConversationDTO[] {
  return items.filter((item) => item.publicID !== publicID);
}

export function isArchivedConversation(item: ConversationDTO): boolean {
  return item.status === "archived";
}
