import type {
  ConversationShareFilter,
  ConversationStarredFilter,
  ConversationStatusFilter,
} from "@/shared/api/conversation.types";

export const RECENT_PAGE_SIZE = 50;

export const RECENT_STATUS_FILTER_OPTIONS: Array<{
  value: ConversationStatusFilter;
}> = [
  { value: "all" },
  { value: "active" },
  { value: "archived" },
];

export const RECENT_STARRED_FILTER_OPTIONS: Array<{
  value: ConversationStarredFilter;
}> = [
  { value: "all" },
  { value: "starred" },
  { value: "unstarred" },
];

export const RECENT_SHARE_FILTER_OPTIONS: Array<{
  value: ConversationShareFilter;
}> = [
  { value: "all" },
  { value: "shared" },
  { value: "unshared" },
];

type RecentEmptyStateLabels = {
  archived: string;
  active: string;
  starred: string;
  unstarred: string;
  shared: string;
  unshared: string;
  conjunction: string;
  all: string;
  filtered: (filters: string) => string;
};

export function recentEmptyStateTitle(
  statusFilter: ConversationStatusFilter,
  starredFilter: ConversationStarredFilter,
  shareFilter: ConversationShareFilter,
  labels?: RecentEmptyStateLabels,
) {
  const parts: string[] = [];

  if (statusFilter === "archived") {
    parts.push(labels?.archived ?? "archived");
  } else if (statusFilter === "active") {
    parts.push(labels?.active ?? "active");
  }
  if (starredFilter === "starred") {
    parts.push(labels?.starred ?? "starred");
  } else if (starredFilter === "unstarred") {
    parts.push(labels?.unstarred ?? "unstarred");
  }
  if (shareFilter === "shared") {
    parts.push(labels?.shared ?? "shared");
  } else if (shareFilter === "unshared") {
    parts.push(labels?.unshared ?? "unshared");
  }
  if (parts.length > 0) {
    const filters = parts.join(labels?.conjunction ?? " and ");
    return labels?.filtered(filters) ?? `No ${filters} conversations`;
  }
  return labels?.all ?? "No conversations";
}

export function formatRelativeUpdatedAt(value: string, locale = "en-US", justNow = "Updated just now") {
  const date = new Date(value);
  if (Number.isNaN(date.getTime())) {
    return justNow;
  }

  const diffMs = date.getTime() - Date.now();
  const diffMinutes = Math.round(diffMs / 60000);
  const formatter = new Intl.RelativeTimeFormat(locale, { numeric: "auto" });

  const ranges = [
    { limit: 60, unit: "minute" as const, value: diffMinutes },
    { limit: 24, unit: "hour" as const, value: Math.round(diffMinutes / 60) },
    { limit: 30, unit: "day" as const, value: Math.round(diffMinutes / 1440) },
    { limit: 12, unit: "month" as const, value: Math.round(diffMinutes / 43200) },
  ];

  for (const range of ranges) {
    if (Math.abs(range.value) < range.limit) {
      return formatter.format(range.value, range.unit);
    }
  }

  return formatter.format(Math.round(diffMinutes / 525600), "year");
}
