import type { ConversationDTO } from "@/shared/api/conversation.types"
import type { ConversationSearchResult } from "@/features/layouts/types/navigation"
import {
  conversationMatchesSearch,
  conversationSearchText,
  normalizeConversationSearchText,
} from "@/shared/lib/conversation-search"

export function toConversationSearchResult(item: ConversationDTO, untitled = "New chat"): ConversationSearchResult {
  return {
    publicID: item.publicID,
    title: item.title?.trim() || untitled,
    searchText: conversationSearchText(item),
    href: `/chat?conversation_id=${item.publicID}`,
    updatedAt: item.updatedAt,
  }
}

export function filterConversationSearchResults(
  items: readonly ConversationDTO[],
  query: string,
  maxResults?: number,
  untitled?: string,
) {
  const normalizedQuery = normalizeConversationSearchText(query)
  const results = items
    .filter((item) => conversationMatchesSearch(item, normalizedQuery))
    .map((item) => toConversationSearchResult(item, untitled))

  return typeof maxResults === "number" ? results.slice(0, maxResults) : results
}

type UpdatedAtLabelValues = {
  year: number
  month: number
  day: number
  time: string
}

type UpdatedAtLabelFormatter = (
  key: "todayTime" | "thisYearDateTime" | "fullDateTime",
  values: UpdatedAtLabelValues,
) => string

export function formatUpdatedAtLabel(value: string, formatLabel: UpdatedAtLabelFormatter) {
  const date = new Date(value)
  if (Number.isNaN(date.getTime())) {
    return ""
  }

  const now = new Date()
  const isToday =
    date.getFullYear() === now.getFullYear() &&
    date.getMonth() === now.getMonth() &&
    date.getDate() === now.getDate()
  const isCurrentYear = date.getFullYear() === now.getFullYear()
  const timeLabel = [date.getHours(), date.getMinutes(), date.getSeconds()]
    .map((part) => String(part).padStart(2, "0"))
    .join(":")
  const values = {
    year: date.getFullYear(),
    month: date.getMonth() + 1,
    day: date.getDate(),
    time: timeLabel,
  }

  if (isToday) {
    return formatLabel("todayTime", values)
  }

  return formatLabel(isCurrentYear ? "thisYearDateTime" : "fullDateTime", values)
}
