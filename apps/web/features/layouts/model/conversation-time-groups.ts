import type { ConversationDTO } from "@/shared/api/conversation.types";

export type ConversationTimeGroup = {
  key: string;
  label: string;
  showLabel: boolean;
  items: ConversationDTO[];
};

type TimeGroupLabels = {
  yesterday: string;
  lastSevenDays: string;
  earlier: string;
};

function startOfDay(date: Date): Date {
  const d = new Date(date);
  d.setHours(0, 0, 0, 0);
  return d;
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

  const buckets = new Map<
    string,
    {
      key: string;
      label: string;
      showLabel: boolean;
      items: ConversationDTO[];
      order: number;
    }
  >();

  for (const item of items) {
    const updatedAt = new Date(item.updatedAt);
    if (Number.isNaN(updatedAt.getTime())) {
      continue;
    }

    let key: string;
    let label: string;
    let showLabel: boolean;
    let order: number;

    if (updatedAt >= todayStart) {
      key = "today";
      label = "";
      showLabel = false;
      order = 0;
    } else if (updatedAt >= yesterdayStart) {
      key = "yesterday";
      label = labels.yesterday;
      showLabel = true;
      order = 1;
    } else if (updatedAt >= sevenDaysAgo) {
      key = "last7days";
      label = labels.lastSevenDays;
      showLabel = true;
      order = 2;
    } else {
      key = "earlier";
      label = labels.earlier;
      showLabel = true;
      order = 3;
    }

    const bucket = buckets.get(key);
    if (bucket) {
      bucket.items.push(item);
    } else {
      buckets.set(key, { key, label, showLabel, items: [item], order });
    }
  }

  return Array.from(buckets.values())
    .sort((a, b) => a.order - b.order)
    .map(({ key, label, showLabel, items: groupItems }) => ({
      key,
      label,
      showLabel,
      items: groupItems,
    }));
}
