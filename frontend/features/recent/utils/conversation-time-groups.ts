import type { ConversationDTO } from "@/shared/api/conversation.types";

export type ConversationTimeGroup = {
  key: string;
  label: string;
  items: ConversationDTO[];
};

type TimeGroupLabels = {
  today: string;
  yesterday: string;
  lastSevenDays: string;
  lastThirtyDays: string;
};

function startOfDay(date: Date): Date {
  const d = new Date(date);
  d.setHours(0, 0, 0, 0);
  return d;
}

function formatMonthKey(date: Date): string {
  const year = date.getFullYear();
  const month = String(date.getMonth() + 1).padStart(2, "0");
  return `${year}-${month}`;
}

export function groupConversationsByTime(
  items: ConversationDTO[],
  labels: TimeGroupLabels,
): ConversationTimeGroup[] {
  if (items.length === 0) {
    return [];
  }

  const now = new Date();
  const todayStart = startOfDay(now);
  const yesterdayStart = new Date(todayStart);
  yesterdayStart.setDate(yesterdayStart.getDate() - 1);
  const sevenDaysAgo = new Date(todayStart);
  sevenDaysAgo.setDate(sevenDaysAgo.getDate() - 7);
  const thirtyDaysAgo = new Date(todayStart);
  thirtyDaysAgo.setDate(thirtyDaysAgo.getDate() - 30);

  const buckets = new Map<string, { key: string; label: string; items: ConversationDTO[]; order: number }>();
  let monthOrder = 100;

  for (const item of items) {
    const updatedAt = new Date(item.updatedAt);
    if (Number.isNaN(updatedAt.getTime())) {
      continue;
    }

    let key: string;
    let label: string;
    let order: number;

    if (updatedAt >= todayStart) {
      key = "today";
      label = labels.today;
      order = 0;
    } else if (updatedAt >= yesterdayStart) {
      key = "yesterday";
      label = labels.yesterday;
      order = 1;
    } else if (updatedAt >= sevenDaysAgo) {
      key = "last7days";
      label = labels.lastSevenDays;
      order = 2;
    } else if (updatedAt >= thirtyDaysAgo) {
      key = "last30days";
      label = labels.lastThirtyDays;
      order = 3;
    } else {
      key = formatMonthKey(updatedAt);
      label = key;
      order = monthOrder++;
    }

    const bucket = buckets.get(key);
    if (bucket) {
      bucket.items.push(item);
    } else {
      buckets.set(key, { key, label, items: [item], order });
    }
  }

  return Array.from(buckets.values())
    .sort((a, b) => a.order - b.order)
    .map(({ key, label, items: groupItems }) => ({
      key,
      label,
      items: groupItems,
    }));
}
