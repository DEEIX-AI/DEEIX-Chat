"use client"

import * as React from "react"

type UseLayoutSidebarListFlipOptions = {
  enabled: boolean
  signature: string
  excludeKey?: string | null
}

export function useLayoutSidebarListFlip(
  containerRef: React.RefObject<HTMLElement | null>,
  { enabled, signature, excludeKey }: UseLayoutSidebarListFlipOptions,
) {
  const rectsRef = React.useRef<Map<string, DOMRect>>(new Map())

  React.useLayoutEffect(() => {
    const container = containerRef.current
    if (!container) {
      return
    }

    const nodes = Array.from(
      container.querySelectorAll<HTMLElement>("[data-sidebar-conversation-id], [data-sidebar-motion-key]"),
    )

    const nextRects = new Map<string, DOMRect>()
    for (const node of nodes) {
      const key = node.dataset.sidebarConversationId || node.dataset.sidebarMotionKey
      if (!key) {
        continue
      }
      nextRects.set(key, node.getBoundingClientRect())
    }

    if (enabled) {
      for (const node of nodes) {
        const key = node.dataset.sidebarConversationId || node.dataset.sidebarMotionKey
        if (!key || key === excludeKey) {
          continue
        }

        const previousRect = rectsRef.current.get(key)
        const nextRect = nextRects.get(key)
        if (!previousRect || !nextRect) {
          continue
        }

        const deltaY = previousRect.top - nextRect.top
        if (Math.abs(deltaY) < 0.5) {
          continue
        }

        node.animate(
          [
            { transform: `translateY(${deltaY}px)` },
            { transform: "translateY(0px)" },
          ],
          {
            duration: 280,
            easing: "cubic-bezier(0.16, 1, 0.3, 1)",
          },
        )
      }
    }

    rectsRef.current = nextRects
  }, [containerRef, enabled, excludeKey, signature])
}
