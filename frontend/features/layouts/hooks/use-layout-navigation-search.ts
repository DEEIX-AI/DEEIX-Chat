"use client"

import * as React from "react"
import { useRouter } from "next/navigation"

import {
  filterConversationSearchResults,
  toConversationSearchResult,
} from "@/features/layouts/utils/navigation-search"
import { hasPlatformModifierKey } from "@/shared/lib/platform-shortcuts"
import { normalizeConversationSearchText } from "@/shared/lib/conversation-search"
import { listConversations } from "@/shared/api/conversation"
import type { ConversationDTO } from "@/shared/api/conversation.types"
import { resolveAccessToken } from "@/shared/auth/resolve-access-token"

type UseLayoutNavigationSearchOptions = {
  items: readonly ConversationDTO[]
  maxResults?: number
}

const NAVIGATION_SEARCH_DEBOUNCE_MS = 250
const DEFAULT_NAVIGATION_SEARCH_LIMIT = 8

export function useLayoutNavigationSearch({ items, maxResults }: UseLayoutNavigationSearchOptions) {
  const router = useRouter()
  const [open, setOpen] = React.useState(false)
  const [query, setQuery] = React.useState("")
  const [remoteResults, setRemoteResults] = React.useState<ConversationDTO[]>([])
  const [loading, setLoading] = React.useState(false)
  const requestVersionRef = React.useRef(0)

  React.useEffect(() => {
    if (!open) {
      setQuery("")
      setRemoteResults([])
      setLoading(false)
      requestVersionRef.current += 1
    }
  }, [open])

  const normalizedQuery = React.useMemo(() => normalizeConversationSearchText(query), [query])

  React.useEffect(() => {
    if (!open || !normalizedQuery) {
      setRemoteResults([])
      setLoading(false)
      requestVersionRef.current += 1
      return
    }

    requestVersionRef.current += 1
    const requestVersion = requestVersionRef.current
    setRemoteResults([])
    setLoading(true)
    const timer = window.setTimeout(() => {
      void (async () => {
        try {
          const token = await resolveAccessToken()
          if (!token || requestVersion !== requestVersionRef.current) {
            return
          }
          const data = await listConversations(token, {
            page: 1,
            pageSize: maxResults ?? DEFAULT_NAVIGATION_SEARCH_LIMIT,
            status: "all",
            starred: "all",
            share: "all",
            project: "all",
            query: normalizedQuery,
          })
          if (requestVersion !== requestVersionRef.current) {
            return
          }
          setRemoteResults(data.results ?? [])
        } catch {
          if (requestVersion === requestVersionRef.current) {
            setRemoteResults([])
          }
        } finally {
          if (requestVersion === requestVersionRef.current) {
            setLoading(false)
          }
        }
      })()
    }, NAVIGATION_SEARCH_DEBOUNCE_MS)

    return () => window.clearTimeout(timer)
  }, [maxResults, normalizedQuery, open])

  const results = React.useMemo(
    () => {
      if (!normalizedQuery) {
        return filterConversationSearchResults(items, query, maxResults)
      }
      return remoteResults.map((item) => toConversationSearchResult(item))
    },
    [items, maxResults, normalizedQuery, query, remoteResults],
  )

  const openSearch = React.useCallback(() => {
    React.startTransition(() => {
      setOpen(true)
    })
  }, [])

  const selectResult = React.useCallback((href: string) => {
    setOpen(false)
    router.push(href)
  }, [router])

  return {
    open,
    setOpen,
    query,
    setQuery,
    results,
    loading,
    openSearch,
    selectResult,
  }
}

export function useLayoutNavigationShortcuts({
  onCreateConversation,
  onOpenSearch,
}: {
  onCreateConversation: () => void
  onOpenSearch: () => void
}) {
  React.useEffect(() => {
    const handleKeyDown = (event: KeyboardEvent) => {
      if (event.isComposing || event.key === "Process") {
        return
      }

      if (!hasPlatformModifierKey(event)) {
        return
      }

      const normalizedKey = event.key.toLowerCase()
      if (event.shiftKey && normalizedKey === "o") {
        event.preventDefault()
        onCreateConversation()
        return
      }

      if (!event.shiftKey && normalizedKey === "k") {
        event.preventDefault()
        onOpenSearch()
      }
    }

    window.addEventListener("keydown", handleKeyDown)
    return () => window.removeEventListener("keydown", handleKeyDown)
  }, [onCreateConversation, onOpenSearch])
}
