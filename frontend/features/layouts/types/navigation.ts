import type * as React from "react"
import type { LucideIcon } from "lucide-react"

export type ShortcutKey = "shift" | "command" | string

export type NavigationIconProps = {
  size?: number
  strokeWidth?: number
  className?: string
  animate?: "default"
}

export type NavigationItem = {
  id: "newChat" | "search" | "recent" | "files"
  title: string
  url: string
  icon: React.ComponentType<NavigationIconProps>
  group: "primary" | "secondary"
  variant?: "primary"
  shortcut?: readonly ShortcutKey[]
  external?: boolean
}

export type ConversationSearchResult = {
  publicID: string
  title: string
  searchText: string
  href: string
  updatedAt: string
}

export type SidebarConversationItem = {
  publicID: string
  title: string
  url: string
  starred?: boolean
  shareActive?: boolean
}

export type SidebarConversationRenameTarget = {
  publicID: string
  currentTitle: string
} | null

export type SidebarConversationDeleteTarget = {
  publicID: string
  title: string
} | null

export type SidebarConversationStarAction = {
  label: string
  icon: LucideIcon
  onSelect: (publicID: string) => void
}
