"use client"

import * as React from "react"
import { useSearchParams } from "next/navigation"

export function useLayoutActiveConversation() {
  const searchParams = useSearchParams()
  const activeConversationID = searchParams.get("conversation_id")
  const previousActiveConversationIDRef = React.useRef<string | null>(null)

  React.useEffect(() => {
    if (!activeConversationID || previousActiveConversationIDRef.current === activeConversationID) {
      return
    }
    previousActiveConversationIDRef.current = activeConversationID

    const activeItem = document.querySelector<HTMLElement>(
      `[data-sidebar-conversation-id="${activeConversationID}"][data-sidebar-active="true"]`,
    )
    if (!activeItem) {
      return
    }

    requestAnimationFrame(() => {
      activeItem.scrollIntoView({
        block: "nearest",
        inline: "nearest",
      })
    })
  }, [activeConversationID])

  return activeConversationID
}
