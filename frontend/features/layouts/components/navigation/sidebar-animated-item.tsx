"use client"

import type * as React from "react"
import { motion, type Transition } from "motion/react"

type SidebarAnimatedItemProps = React.PropsWithChildren<{
  enabled: boolean
  className?: string
  layoutId?: string
  transition?: Transition
  conversationId?: string
  active?: boolean
}>

export function SidebarAnimatedItem({
  enabled,
  className,
  layoutId,
  transition,
  conversationId,
  active = false,
  children,
}: SidebarAnimatedItemProps) {
  const commonProps = {
    className,
    "data-sidebar-conversation-id": conversationId,
    "data-sidebar-active": active ? "true" : "false",
  } as const

  if (!enabled) {
    return <li {...commonProps}>{children}</li>
  }

  return (
    <motion.li
      layout="position"
      layoutId={layoutId}
      initial={false}
      transition={transition}
      style={{ willChange: "transform" }}
      {...commonProps}
    >
      {children}
    </motion.li>
  )
}
