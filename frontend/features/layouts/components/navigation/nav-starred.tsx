"use client"

import * as React from "react"
import { useRouter } from "next/navigation"
import { motion } from "motion/react"
import { StarOff } from "lucide-react"
import { useTranslations } from "next-intl"

import { List } from "@/components/animate-ui/icons/list"
import {
  AlertDialog,
  AlertDialogAction,
  AlertDialogCancel,
  AlertDialogContent,
  AlertDialogDescription as AlertDialogBody,
  AlertDialogFooter,
  AlertDialogHeader,
  AlertDialogTitle,
} from "@/components/ui/alert-dialog"
import {
  SidebarGroup,
  SidebarGroupLabel,
  SidebarMenu,
  SidebarMenuButton,
  useSidebar,
} from "@/components/ui/sidebar"
import { LoadingReveal } from "@/features/layouts/components/sections/loading-reveal"
import { NavigationSearch } from "@/features/layouts/components/navigation/navigation-search"
import { SidebarConversationItem } from "@/features/layouts/components/navigation/sidebar-conversation-item"
import { SidebarConversationSkeleton } from "@/features/layouts/components/navigation/sidebar-conversation-skeleton"
import {
  ConversationShareDialog,
  sharePatchFromDTO,
} from "@/features/chat/components/sections/conversation-share-dialog"
import { useActiveSidebarConversation } from "@/features/layouts/hooks/use-active-sidebar-conversation"
import { useSidebarListFlip } from "@/features/layouts/hooks/use-sidebar-list-flip"
import { SIDEBAR_OVERFLOW_ROW_TRANSITION } from "@/features/layouts/model/sidebar-motion"
import type {
  SidebarConversationDeleteTarget,
  SidebarConversationItem as SidebarConversationItemModel,
  SidebarConversationRenameTarget,
} from "@/features/layouts/types/navigation"
import { filterConversationSearchResults } from "@/features/layouts/utils/navigation-search"
import { useSidebarRecents } from "@/features/recent/context/sidebar-recents-context"
import type { ConversationDTO } from "@/shared/api/conversation.types"
import { cn } from "@/lib/utils"

const STARRED_SKELETON_WIDTHS = ["71%", "59%", "66%", "54%", "70%"] as const
const MAX_VISIBLE_STARRED = 5

function toSidebarConversationItem(item: ConversationDTO, untitled: string): SidebarConversationItemModel {
  return {
    publicID: item.publicID,
    title: item.title || untitled,
    url: `/chat?conversation_id=${item.publicID}`,
    starred: true,
  }
}

export function NavStarred() {
  const t = useTranslations("recent")
  const { isMobile, setOpenMobile } = useSidebar()
  const router = useRouter()
  const activeConversationID = useActiveSidebarConversation()

  const {
    starredItems,
    starredTotal,
    loadingInitial,
    transferringStarPublicID,
    setStarByPublicID,
    renameByPublicID,
    loadAllStarred,
    archiveByPublicID,
    deleteByPublicID,
    touchByPublicID,
  } = useSidebarRecents()

  const [showAllStarredDialog, setShowAllStarredDialog] = React.useState(false)
  const [dialogStarredItems, setDialogStarredItems] = React.useState<ConversationDTO[] | null>(null)
  const [dialogLoading, setDialogLoading] = React.useState(false)
  const [searchQuery, setSearchQuery] = React.useState("")
  const [deleteTarget, setDeleteTarget] = React.useState<SidebarConversationDeleteTarget>(null)
  const [renameTarget, setRenameTarget] = React.useState<SidebarConversationRenameTarget>(null)
  const [shareTarget, setShareTarget] = React.useState<{ publicID: string; title: string } | null>(null)
  const [renameValue, setRenameValue] = React.useState("")
  const listContainerRef = React.useRef<HTMLDivElement | null>(null)

  const starredConversationItems = React.useMemo(
    () => starredItems.map((item) => toSidebarConversationItem(item, t("untitled"))),
    [starredItems, t],
  )
  const visibleStarredItems = React.useMemo(
    () => starredConversationItems.slice(0, MAX_VISIBLE_STARRED),
    [starredConversationItems],
  )
  const hasOverflowButton = starredTotal > MAX_VISIBLE_STARRED
  const visibleStarredSignature = React.useMemo(
    () => `${visibleStarredItems.map((item) => item.publicID).join("|")}::overflow:${hasOverflowButton ? "1" : "0"}`,
    [hasOverflowButton, visibleStarredItems],
  )
  const commandResults = React.useMemo(
    () => filterConversationSearchResults(dialogStarredItems ?? starredItems, searchQuery, undefined, t("untitled")),
    [dialogStarredItems, searchQuery, starredItems, t],
  )
  const showInitialSkeleton = loadingInitial && starredConversationItems.length === 0

  useSidebarListFlip(listContainerRef, {
    enabled: Boolean(transferringStarPublicID),
    signature: visibleStarredSignature,
    excludeKey: transferringStarPublicID,
  })

  React.useEffect(() => {
    if (!showAllStarredDialog) {
      setDialogLoading(false)
      setDialogStarredItems(null)
      setSearchQuery("")
      return
    }

    if (starredTotal <= starredItems.length) {
      setDialogLoading(false)
      setDialogStarredItems(starredItems)
      return
    }

    let cancelled = false
    setDialogLoading(true)
    void loadAllStarred()
      .then((items) => {
        if (!cancelled) {
          setDialogStarredItems(items)
        }
      })
      .finally(() => {
        if (!cancelled) {
          setDialogLoading(false)
        }
      })

    return () => {
      cancelled = true
    }
  }, [loadAllStarred, showAllStarredDialog, starredItems, starredTotal])

  const onRename = React.useCallback((publicID: string, currentTitle: string) => {
    setRenameTarget({ publicID, currentTitle })
    setRenameValue(currentTitle)
  }, [])

  const onRenameCancel = React.useCallback(() => {
    setRenameTarget(null)
    setRenameValue("")
  }, [])

  const onRenameCommit = React.useCallback(
    async (publicID: string, currentTitle: string) => {
      const nextTitle = renameValue.trim()
      if (!nextTitle || nextTitle === currentTitle) {
        onRenameCancel()
        return
      }
      await renameByPublicID(publicID, nextTitle)
      onRenameCancel()
    },
    [onRenameCancel, renameByPublicID, renameValue],
  )

  const onUnstar = React.useCallback(
    (publicID: string) => {
      void setStarByPublicID(publicID, false)
    },
    [setStarByPublicID],
  )

  const onArchive = React.useCallback(
    async (publicID: string) => {
      await archiveByPublicID(publicID, true)
      if (activeConversationID === publicID) {
        router.push("/chat")
      }
    },
    [activeConversationID, archiveByPublicID, router],
  )

  const onDelete = React.useCallback((publicID: string, title: string) => {
    setDeleteTarget({ publicID, title })
  }, [])

  const onShare = React.useCallback((publicID: string, title: string) => {
    setShareTarget({ publicID, title })
  }, [])

  const confirmDelete = React.useCallback(async () => {
    if (!deleteTarget) {
      return
    }
    const ok = await deleteByPublicID(deleteTarget.publicID)
    if (ok && activeConversationID === deleteTarget.publicID) {
      router.push("/chat")
    }
    setDeleteTarget(null)
  }, [activeConversationID, deleteByPublicID, deleteTarget, router])

  const onSelectSearchResult = React.useCallback((href: string) => {
    setShowAllStarredDialog(false)
    if (isMobile) {
      setOpenMobile(false)
    }
    router.push(href)
  }, [isMobile, router, setOpenMobile])

  if (!loadingInitial && starredTotal === 0 && starredConversationItems.length === 0) {
    return null
  }

  return (
    <>
      <div
        className={cn(
          "relative z-10 overflow-hidden group-data-[collapsible=icon]:pointer-events-none group-data-[collapsible=icon]:opacity-0",
        )}
      >
        <SidebarGroup>
          <SidebarGroupLabel>{t("starred")}</SidebarGroupLabel>
          <div ref={listContainerRef}>
            <LoadingReveal
              loading={showInitialSkeleton}
              skeleton={<SidebarConversationSkeleton count={3} widths={STARRED_SKELETON_WIDTHS} prefix="sidebar-starred" />}
              className="min-h-0"
            >
              <SidebarMenu>
                {visibleStarredItems.map((item) => (
                  <SidebarConversationItem
                    key={item.publicID}
                    item={{
                      ...item,
                      shareActive: starredItems.some(
                        (conversation) =>
                          conversation.publicID === item.publicID &&
                          conversation.shareStatus === "active" &&
                          Boolean(conversation.shareID?.trim()),
                      ),
                    }}
                    active={activeConversationID === item.publicID}
                    isTransferring={transferringStarPublicID === item.publicID}
                    starAction={{
                      label: t("row.unstar"),
                      icon: StarOff,
                      onSelect: onUnstar,
                    }}
                    onRename={onRename}
                    isRenaming={renameTarget?.publicID === item.publicID}
                    renameValue={renameTarget?.publicID === item.publicID ? renameValue : item.title}
                    onRenameValueChange={setRenameValue}
                    onRenameCommit={onRenameCommit}
                    onRenameCancel={onRenameCancel}
                    onArchive={onArchive}
                    onShare={onShare}
                    onDelete={onDelete}
                    onNavigate={isMobile ? () => setOpenMobile(false) : undefined}
                    menuTriggerID={`starred-item-menu-trigger-${item.publicID}`}
                  />
                ))}

                <motion.li
                  data-sidebar-motion-key="starred-overflow"
                  layout="position"
                  initial={false}
                  transition={SIDEBAR_OVERFLOW_ROW_TRANSITION}
                  className={cn(
                    "group/menu-item relative overflow-hidden",
                    hasOverflowButton ? "" : "pointer-events-none",
                  )}
                  animate={{
                    height: hasOverflowButton ? 32 : 0,
                    opacity: hasOverflowButton ? 1 : 0,
                  }}
                >
                  <SidebarMenuButton
                    tabIndex={hasOverflowButton ? 0 : -1}
                    onClick={() => {
                      if (hasOverflowButton) {
                        setShowAllStarredDialog(true)
                      }
                    }}
                  >
                    <List size={16} strokeWidth={1.4} />
                    <span className="text-xs text-sidebar-foreground/75">{t("allConversations")}</span>
                  </SidebarMenuButton>
                </motion.li>
              </SidebarMenu>
            </LoadingReveal>
          </div>
        </SidebarGroup>
      </div>

      <NavigationSearch
        open={showAllStarredDialog}
        onOpenChange={setShowAllStarredDialog}
        query={searchQuery}
        onQueryChange={setSearchQuery}
        results={commandResults}
        title={t("starredSearch.title")}
        description={t("starredSearch.description")}
        placeholder={t("starredSearch.placeholder")}
        loading={dialogLoading}
        loadingText={t("starredSearch.loading")}
        emptyText={t("starredSearch.empty")}
        onSelect={onSelectSearchResult}
      />

      <AlertDialog open={Boolean(deleteTarget)} onOpenChange={(open) => !open && setDeleteTarget(null)}>
        <AlertDialogContent>
          <AlertDialogHeader>
            <AlertDialogTitle>{t("dialogs.deleteTitle")}</AlertDialogTitle>
            <AlertDialogBody>
              {t("dialogs.deleteDescription", { label: t("deleteConversationLabel", { title: deleteTarget?.title || t("untitled") }) })}
            </AlertDialogBody>
          </AlertDialogHeader>
          <AlertDialogFooter>
            <AlertDialogCancel>{t("dialogs.cancel")}</AlertDialogCancel>
            <AlertDialogAction variant="destructive" onClick={() => void confirmDelete()}>
              {t("dialogs.delete")}
            </AlertDialogAction>
          </AlertDialogFooter>
        </AlertDialogContent>
      </AlertDialog>

      {shareTarget ? (
        <ConversationShareDialog
          open={Boolean(shareTarget)}
          onOpenChange={(open) => !open && setShareTarget(null)}
          conversationPublicID={shareTarget.publicID}
          conversationTitle={shareTarget.title}
          onShareChange={(share) => {
            touchByPublicID(shareTarget.publicID, sharePatchFromDTO(share))
          }}
        />
      ) : null}
    </>
  )
}
