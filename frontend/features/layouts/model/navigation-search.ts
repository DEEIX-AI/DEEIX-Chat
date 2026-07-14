import {
  conversationMatchesSearch,
  conversationSearchText,
  normalizeConversationSearchText,
} from "@/shared/lib/conversation-search";
import type { ConversationSearchResult } from "@/features/layouts/types/navigation";
import type { ConversationDTO } from "@/shared/api/conversation.types";

type ConversationSearchResultGroup = {
  key: string;
  label: string;
  items: ConversationSearchResult[];
};

export function toConversationSearchResult(item: ConversationDTO, untitled: string): ConversationSearchResult {
  return {
    publicID: item.publicID,
    title: item.title?.trim() || untitled,
    searchText: conversationSearchText(item),
    href: `/chat?conversation_id=${item.publicID}`,
    updatedAt: item.updatedAt,
  };
}

export function filterConversationSearchResults(
  items: readonly ConversationDTO[],
  query: string,
  {
    maxResults,
    untitled,
  }: {
    maxResults?: number;
    untitled: string;
  },
) {
  const normalizedQuery = normalizeConversationSearchText(query);
  const results = items
    .filter((item) => conversationMatchesSearch(item, normalizedQuery))
    .map((item) => toConversationSearchResult(item, untitled));

  return typeof maxResults === "number" ? results.slice(0, maxResults) : results;
}

function isSameCalendarDay(left: Date, right: Date) {
  return (
    left.getFullYear() === right.getFullYear() &&
    left.getMonth() === right.getMonth() &&
    left.getDate() === right.getDate()
  );
}

export function groupConversationSearchResultsByDate(
  items: readonly ConversationSearchResult[],
  {
    locale,
    todayLabel,
  }: {
    locale: string;
    todayLabel: string;
  },
): ConversationSearchResultGroup[] {
  const now = new Date();
  const currentYear = now.getFullYear();
  const currentYearFormatter = new Intl.DateTimeFormat(locale, { month: "long" });
  const otherYearFormatter = new Intl.DateTimeFormat(locale, { year: "numeric", month: "long" });
  const groups = new Map<
    string,
    ConversationSearchResultGroup & {
      order: number;
    }
  >();

  for (const item of items) {
    const updatedAt = new Date(item.updatedAt);
    if (Number.isNaN(updatedAt.getTime())) {
      continue;
    }

    const isToday = isSameCalendarDay(updatedAt, now);
    const key = isToday
      ? "today"
      : `${updatedAt.getFullYear()}-${updatedAt.getMonth()}`;
    const label = isToday
      ? todayLabel
      : (updatedAt.getFullYear() === currentYear ? currentYearFormatter : otherYearFormatter).format(updatedAt);
    const order = isToday
      ? Number.MAX_SAFE_INTEGER
      : updatedAt.getFullYear() * 12 + updatedAt.getMonth();
    const group = groups.get(key);

    if (group) {
      group.items.push(item);
    } else {
      groups.set(key, { key, label, items: [item], order });
    }
  }

  return Array.from(groups.values())
    .sort((left, right) => right.order - left.order)
    .map(({ key, label, items: groupItems }) => ({
      key,
      label,
      items: groupItems,
    }));
}

type UpdatedAtLabelValues = {
  year: number;
  month: number;
  day: number;
  time: string;
};

type UpdatedAtLabelFormatter = (
  key: "todayTime" | "thisYearDateTime" | "fullDateTime",
  values: UpdatedAtLabelValues,
) => string;

export function formatUpdatedAtLabel(value: string, formatLabel: UpdatedAtLabelFormatter) {
  const date = new Date(value);
  if (Number.isNaN(date.getTime())) {
    return "";
  }

  const now = new Date();
  const isToday =
    date.getFullYear() === now.getFullYear() &&
    date.getMonth() === now.getMonth() &&
    date.getDate() === now.getDate();
  const isCurrentYear = date.getFullYear() === now.getFullYear();
  const timeLabel = [date.getHours(), date.getMinutes(), date.getSeconds()]
    .map((part) => String(part).padStart(2, "0"))
    .join(":");
  const values = {
    year: date.getFullYear(),
    month: date.getMonth() + 1,
    day: date.getDate(),
    time: timeLabel,
  };

  if (isToday) {
    return formatLabel("todayTime", values);
  }

  return formatLabel(isCurrentYear ? "thisYearDateTime" : "fullDateTime", values);
}
