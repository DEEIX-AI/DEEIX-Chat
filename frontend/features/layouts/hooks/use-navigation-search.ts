"use client"

import * as React from "react"
import { useRouter } from "next/navigation"

import { filterConversationSearchResults } from "@/features/layouts/utils/navigation-search"
import { hasPlatformModifierKey } from "@/shared/lib/platform-shortcuts"
import type { ConversationDTO } from "@/shared/api/conversation.types"

type UseNavigationSearchOptions = {
  items: readonly ConversationDTO[]
  maxResults?: number
}

export function useNavigationSearch({ items, maxResults }: UseNavigationSearchOptions) {
  const router = useRouter()
  const [open, setOpen] = React.useState(false)
  const [query, setQuery] = React.useState("")

  React.useEffect(() => {
    if (!open) {
      setQuery("")
    }
  }, [open])

  const results = React.useMemo(
    () => filterConversationSearchResults(items, query, maxResults),
    [items, maxResults, query],
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
    openSearch,
    selectResult,
  }
}

export function useNavigationShortcuts({
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
