"use client"

import * as React from "react"
import Link from "next/link"
import { Archive, PencilLine, Trash } from "lucide-react"
import { useTranslations } from "next-intl"

import { Ellipsis } from "@/components/animate-ui/icons/ellipsis"
import { AnimatedText } from "@/components/ui/animated-text"
import {
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuItem,
  DropdownMenuItemIcon,
  DropdownMenuSeparator,
  DropdownMenuTrigger,
} from "@/components/ui/dropdown-menu"
import { SidebarAnimatedItem } from "@/features/layouts/components/navigation/sidebar-animated-item"
import { SIDEBAR_TRANSFER_TRANSITION } from "@/features/layouts/model/sidebar-motion"
import { ConversationProjectSubmenu } from "@/shared/components/conversation-project-submenu"
import { ConversationShareExportSubmenu } from "@/shared/components/conversation-share-export-menu"
import type {
  SidebarConversationItem as SidebarConversationItemModel,
  SidebarConversationProjectMenu,
  SidebarConversationStarAction,
} from "@/features/layouts/types/navigation"
import { cn } from "@/lib/utils"

type SidebarConversationItemProps = {
  item: SidebarConversationItemModel
  active: boolean
  isTransferring: boolean
  isRenaming: boolean
  renameValue: string
  menuTriggerID: string
  starAction: SidebarConversationStarAction
  projectMenu?: SidebarConversationProjectMenu
  rowClassName?: string
  linkClassName?: string
  onRenameValueChange: (value: string) => void
  onRenameCommit: (publicID: string, currentTitle: string) => void
  onRenameCancel: () => void
  onRename: (publicID: string, currentTitle: string) => void
  onArchive: (publicID: string) => void
  onShare?: (publicID: string, title: string) => void
  onExport?: (publicID: string) => void | Promise<void>
  onDelete: (publicID: string, title: string) => void
  onNavigate?: () => void
}

export function SidebarConversationItem({
  item,
  active,
  isTransferring,
  isRenaming,
  renameValue,
  menuTriggerID,
  starAction,
  projectMenu,
  rowClassName,
  linkClassName,
  onRenameValueChange,
  onRenameCommit,
  onRenameCancel,
  onRename,
  onArchive,
  onShare,
  onExport,
  onDelete,
  onNavigate,
}: SidebarConversationItemProps) {
  const t = useTranslations("recent.row")
  const [isRowHovered, setIsRowHovered] = React.useState(false)
  const [isMenuHovered, setIsMenuHovered] = React.useState(false)
  const [isMenuOpen, setIsMenuOpen] = React.useState(false)
  const showMenuButton = isRowHovered || isMenuHovered || isMenuOpen || active

  return (
    <SidebarAnimatedItem
      enabled={isTransferring}
      layoutId={`sidebar-conversation-${item.publicID}`}
      transition={SIDEBAR_TRANSFER_TRANSITION}
      className="group/menu-item relative"
      conversationId={item.publicID}
      active={active}
    >
      {isRenaming ? (
        <div className="relative flex h-8 items-center rounded-md bg-sidebar-accent text-sm text-sidebar-accent-foreground">
          <input
            autoFocus
            value={renameValue}
            className="h-8 w-full bg-transparent pl-2 pr-2 outline-none"
            onChange={(event) => onRenameValueChange(event.target.value)}
            onClick={(event) => event.stopPropagation()}
            onMouseDown={(event) => event.stopPropagation()}
            onKeyDown={(event) => {
              event.stopPropagation()
              if (event.key === "Enter") {
                event.preventDefault()
                onRenameCommit(item.publicID, item.title)
              } else if (event.key === "Escape") {
                event.preventDefault()
                onRenameCancel()
              }
            }}
            onBlur={() => onRenameCommit(item.publicID, item.title)}
          />
        </div>
      ) : (
        <div
          className={cn(
            "group relative flex h-8 items-center rounded-md text-sm transition-colors",
            active || isRowHovered
              ? "bg-sidebar-accent text-sidebar-accent-foreground"
              : "text-sidebar-foreground",
            rowClassName,
          )}
        >
          <Link
            href={item.url}
            prefetch={false}
            className={cn("flex h-full min-w-0 flex-1 items-center pl-2 pr-9", linkClassName)}
            onClick={onNavigate}
            onMouseEnter={() => setIsRowHovered(true)}
            onMouseLeave={() => setIsRowHovered(false)}
          >
            <AnimatedText
              text={item.title}
              className="flex-1"
              textClassName="text-current"
            />
          </Link>

          <DropdownMenu modal={false} open={isMenuOpen} onOpenChange={setIsMenuOpen}>
            <DropdownMenuTrigger asChild>
              <button
                id={menuTriggerID}
                className={cn(
                  "absolute right-0 flex h-8 w-8 items-center justify-center rounded-md text-sidebar-foreground opacity-0 transition-[background-color,color,opacity] duration-150 hover:bg-sidebar-accent hover:text-sidebar-accent-foreground",
                  showMenuButton && "opacity-100",
                )}
                onMouseEnter={() => setIsMenuHovered(true)}
                onMouseLeave={() => setIsMenuHovered(false)}
                onClick={(event) => {
                  event.preventDefault()
                  event.stopPropagation()
                }}
              >
                <Ellipsis size={16} strokeWidth={1.4} animate={isMenuHovered ? "pulse" : undefined} />
              </button>
            </DropdownMenuTrigger>
            <DropdownMenuContent align="end" className="w-max min-w-36 max-w-[calc(100vw-2rem)]">
              <DropdownMenuItem
                onSelect={(event) => {
                  event.preventDefault()
                  starAction.onSelect(item.publicID)
                }}
              >
                <DropdownMenuItemIcon icon={starAction.icon} />
                {starAction.label}
              </DropdownMenuItem>
              <DropdownMenuItem
                onSelect={(event) => {
                  event.preventDefault()
                  onRename(item.publicID, item.title)
                }}
              >
                <DropdownMenuItemIcon icon={PencilLine} />
                {t("rename")}
              </DropdownMenuItem>
              {projectMenu ? (
                <ConversationProjectSubmenu
                  label={projectMenu.label}
                  unassignedLabel={projectMenu.unassignedLabel}
                  currentProjectID={projectMenu.currentProjectID}
                  projects={projectMenu.projects}
                  onSelect={(projectID) => projectMenu.onSelect(item.publicID, projectID)}
                />
              ) : null}
              <ConversationShareExportSubmenu
                label={t("shareAndExport")}
                shareLabel={item.shareActive ? t("manageShare") : t("share")}
                exportLabel={t("exportJSON")}
                onShare={onShare ? () => onShare(item.publicID, item.title) : undefined}
                onExport={onExport ? () => onExport(item.publicID) : undefined}
                onCloseMenu={() => setIsMenuOpen(false)}
              />
              <DropdownMenuItem
                onSelect={(event) => {
                  event.preventDefault()
                  onArchive(item.publicID)
                }}
              >
                <DropdownMenuItemIcon icon={Archive} />
                {t("archive")}
              </DropdownMenuItem>
              <DropdownMenuSeparator />
              <DropdownMenuItem
                variant="destructive"
                onSelect={(event) => {
                  event.preventDefault()
                  onDelete(item.publicID, item.title)
                }}
              >
                <DropdownMenuItemIcon icon={Trash} className="text-current" />
                {t("delete")}
              </DropdownMenuItem>
            </DropdownMenuContent>
          </DropdownMenu>
        </div>
      )}
    </SidebarAnimatedItem>
  )
}
