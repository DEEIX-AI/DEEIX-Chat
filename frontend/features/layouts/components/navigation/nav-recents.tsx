"use client";

import * as React from "react";
import { useRouter } from "next/navigation";
import { ChevronDown, Star } from "lucide-react";
import { useTranslations } from "next-intl";

import {
  AlertDialog,
  AlertDialogAction,
  AlertDialogCancel,
  AlertDialogContent,
  AlertDialogDescription,
  AlertDialogFooter,
  AlertDialogHeader,
  AlertDialogTitle,
} from "@/components/ui/alert-dialog";
import { Button } from "@/components/ui/button";
import { Collapsible } from "@/components/ui/collapsible";
import { Spinner } from "@/components/ui/spinner";
import {
  SidebarGroup,
  SidebarGroupLabel,
  SidebarMenu,
  useSidebar,
} from "@/components/ui/sidebar";
import {
  ConversationShareDialog,
  sharePatchFromDTO,
  useConversationExport,
  useSidebarConversations,
} from "@/entities/conversation";
import { LoadingReveal } from "@/shared/components/loading-reveal";
import { SidebarConversationItem } from "@/features/layouts/components/navigation/sidebar-conversation-item";
import { SidebarConversationSkeleton } from "@/features/layouts/components/navigation/sidebar-conversation-skeleton";
import { DeleteFilesOption } from "@/shared/components/delete-files-option";
import { CollapsibleMotionContent } from "@/shared/components/collapsible-motion-content";
import { useDialogSnapshot } from "@/shared/hooks/use-dialog-snapshot";
import { useSettingsChatPreferences } from "@/features/settings";
import { useLayoutActiveConversation } from "@/features/layouts/hooks/use-layout-active-conversation";
import { useLayoutSidebarListFlip } from "@/features/layouts/hooks/use-layout-sidebar-list-flip";
import { useSidebarConversationNavigation } from "@/features/layouts/hooks/use-sidebar-conversation-navigation";
import { groupConversationsByTime } from "@/features/layouts/model/conversation-time-groups";
import type {
  SidebarConversationDeleteTarget,
  SidebarConversationRenameTarget,
} from "@/features/layouts/types/navigation";
import { useLoadMoreSentinel } from "@/shared/hooks/use-load-more-sentinel";
import { useStoredBoolean } from "@/shared/hooks/use-stored-boolean";
import { cn } from "@/lib/utils";

const RECENT_SKELETON_WIDTHS = ["74%", "61%", "69%", "57%", "72%"] as const;
const RECENTS_OPEN_STORAGE_KEY = "deeix.sidebar.recents.open";

export function NavRecents() {
  const t = useTranslations("recent");
  const onNavigate = useSidebarConversationNavigation();
  const router = useRouter();
  const activeConversationID = useLayoutActiveConversation();
  const { deleteFilesByDefault } = useSettingsChatPreferences();

  const {
    recentItems,
    hasMore,
    loadingInitial,
    loadingMore,
    loadMoreFailed,
    loadMore,
    retryLoadMore,
    projects,
    transferringStarPublicID,
    renameByPublicID,
    regenerateTitleByPublicID,
    setStarByPublicID,
    archiveByPublicID,
    deleteByPublicID,
    touchByPublicID,
    setProjectByPublicID,
  } = useSidebarConversations();

  const [deleteTarget, setDeleteTarget] = React.useState<SidebarConversationDeleteTarget>(null);
  const [deleteFiles, setDeleteFiles] = React.useState(false);
  const [renameTarget, setRenameTarget] = React.useState<SidebarConversationRenameTarget>(null);
  const [shareTarget, setShareTarget] = React.useState<{
    publicID: string;
    title: string;
  } | null>(null);
  const [renameValue, setRenameValue] = React.useState("");
  const [autoRenamingPublicID, setAutoRenamingPublicID] = React.useState<string | null>(null);
  const [recentsOpen, setRecentsOpen] = useStoredBoolean(RECENTS_OPEN_STORAGE_KEY, true);
  const listContainerRef = React.useRef<HTMLDivElement | null>(null);
  const deleteFilesID = React.useId();
  const stableDeleteTarget = useDialogSnapshot(deleteTarget);
  const stableShareTarget = useDialogSnapshot(shareTarget);
  const recentsContentID = React.useId();
  const onExport = useConversationExport({
    successMessage: t("exported"),
    failureMessage: t("exportFailed"),
  });

  const loadMoreRef = useLoadMoreSentinel<HTMLLIElement>({
    enabled: recentsOpen && hasMore && !loadingInitial && !loadingMore && !loadMoreFailed,
    onLoadMore: loadMore,
  });

  const onRename = React.useCallback((publicID: string, currentTitle: string) => {
    setRenameTarget({ publicID, currentTitle });
    setRenameValue(currentTitle);
  }, []);

  const onRenameCancel = React.useCallback(() => {
    setRenameTarget(null);
    setRenameValue("");
  }, []);

  const onRenameCommit = React.useCallback(
    async (publicID: string, currentTitle: string) => {
      const nextTitle = renameValue.trim();
      if (!nextTitle || nextTitle === currentTitle) {
        onRenameCancel();
        return;
      }
      await renameByPublicID(publicID, nextTitle);
      onRenameCancel();
    },
    [onRenameCancel, renameByPublicID, renameValue],
  );

  const onAutoRename = React.useCallback(
    async (publicID: string) => {
      if (autoRenamingPublicID) {
        return;
      }
      setAutoRenamingPublicID(publicID);
      try {
        const updated = await regenerateTitleByPublicID(publicID);
        if (updated) {
          onRenameCancel();
        }
      } catch {
        // Keep the current rename input open so the user can retry or edit manually.
      } finally {
        setAutoRenamingPublicID(null);
      }
    },
    [autoRenamingPublicID, onRenameCancel, regenerateTitleByPublicID],
  );

  const onToggleStar = React.useCallback(
    (publicID: string, nextStarred: boolean) => {
      void setStarByPublicID(publicID, nextStarred);
    },
    [setStarByPublicID],
  );

  const onArchive = React.useCallback(
    async (publicID: string) => {
      await archiveByPublicID(publicID, true);
      if (activeConversationID === publicID) {
        router.push("/chat");
      }
    },
    [activeConversationID, archiveByPublicID, router],
  );

  const onDelete = React.useCallback((publicID: string, title: string) => {
    setDeleteFiles(deleteFilesByDefault);
    setDeleteTarget({ publicID, title });
  }, [deleteFilesByDefault]);

  const onShare = React.useCallback((publicID: string, title: string) => {
    setShareTarget({ publicID, title });
  }, []);

  const confirmDelete = React.useCallback(async () => {
    if (!deleteTarget) {
      return;
    }
    const ok = await deleteByPublicID(deleteTarget.publicID, { deleteFiles });
    if (ok && activeConversationID === deleteTarget.publicID) {
      router.push("/chat");
    }
    setDeleteTarget(null);
    setDeleteFiles(false);
  }, [activeConversationID, deleteByPublicID, deleteFiles, deleteTarget, router]);

  const visibleItemsSignature = React.useMemo(
    () => recentItems.filter((item) => !item.projectID).map((item) => item.publicID).join("|"),
    [recentItems],
  );
  const showInitialSkeleton = loadingInitial && recentItems.length === 0;
  const visibleRecentItems = React.useMemo(
    () => recentItems.filter((item) => !item.projectID),
    [recentItems],
  );
  const timeGroups = React.useMemo(
    () => groupConversationsByTime(visibleRecentItems, {
      yesterday: t("timeGroup.yesterday"),
      lastSevenDays: t("timeGroup.lastSevenDays"),
      earlier: t("timeGroup.earlier"),
    }),
    [visibleRecentItems, t],
  );

  useLayoutSidebarListFlip(listContainerRef, {
    enabled: recentsOpen && Boolean(transferringStarPublicID),
    signature: visibleItemsSignature,
    excludeKey: transferringStarPublicID,
  });

  return (
    <>
      <div className="relative z-0 group-data-[collapsible=icon]:pointer-events-none group-data-[collapsible=icon]:opacity-0">
        <Collapsible open={recentsOpen} onOpenChange={setRecentsOpen}>
          <SidebarGroup className="px-2 py-2">
            <SidebarGroupLabel
              asChild
              className="w-fit max-w-full self-start cursor-pointer gap-1 pr-1 transition-[color,margin,opacity] hover:text-sidebar-foreground"
            >
              <Button
                type="button"
                variant="ghost"
                className="h-8 gap-1 py-0 pl-2 pr-1 text-xs hover:bg-transparent has-[>svg]:pl-2 has-[>svg]:pr-1 dark:hover:bg-transparent"
                aria-controls={recentsContentID}
                aria-expanded={recentsOpen}
                aria-label={recentsOpen ? t("collapseSection") : t("expandSection")}
                onClick={() => setRecentsOpen((open) => !open)}
              >
                <span className="min-w-0 truncate text-left">{t("title")}</span>
                <ChevronDown
                  aria-hidden
                  className={cn(
                    "!size-3 stroke-1.5 transition-transform duration-200",
                    !recentsOpen && "-rotate-90",
                  )}
                />
              </Button>
            </SidebarGroupLabel>
            <CollapsibleMotionContent id={recentsContentID} open={recentsOpen}>
              <div ref={listContainerRef} className="relative">
                <LoadingReveal
                  loading={showInitialSkeleton}
                  skeleton={<SidebarConversationSkeleton count={6} widths={RECENT_SKELETON_WIDTHS} prefix="sidebar-recent" />}
                  className="min-h-0"
                >
                  <SidebarMenu className="gap-0.5">
                    {visibleRecentItems.length === 0 ? (
                      <li className="px-2 py-2 text-xs text-muted-foreground">
                        {t("empty")}
                      </li>
                    ) : null}

                    {timeGroups.map((group, groupIndex) => (
                      <React.Fragment key={group.key}>
                        {group.showLabel ? (
                          <li className={cn("px-2 pb-0.5 text-[11px] font-medium text-sidebar-foreground/40", groupIndex === 0 ? "pt-0" : "pt-3")}>
                            {group.label}
                          </li>
                        ) : null}
                        {group.items.map((item) => {
                          const title = item.title || t("untitled");
                          const publicID = item.publicID;

                          return (
                            <SidebarConversationItem
                              key={publicID}
                              active={activeConversationID === publicID}
                              item={{
                                publicID,
                                title,
                                url: `/chat?conversation_id=${publicID}`,
                                shareActive: item.shareStatus === "active" && Boolean(item.shareID?.trim()),
                              }}
                              starAction={{
                                label: item.isStarred ? t("row.unstar") : t("row.star"),
                                icon: Star,
                                onSelect: (targetPublicID) => onToggleStar(targetPublicID, !item.isStarred),
                              }}
                              projectMenu={{
                                label: t("row.moveToProject"),
                                unassignedLabel: t("projects.unassigned"),
                                currentProjectID: item.projectID,
                                projects,
                                onSelect: (targetPublicID, projectID) => {
                                  void setProjectByPublicID(targetPublicID, projectID);
                                },
                              }}
                              isTransferring={transferringStarPublicID === publicID}
                              onRename={onRename}
                              isRenaming={renameTarget?.publicID === publicID}
                              renameValue={renameTarget?.publicID === publicID ? renameValue : title}
                              onRenameValueChange={setRenameValue}
                              onRenameCommit={onRenameCommit}
                              onRenameCancel={onRenameCancel}
                              onAutoRename={onAutoRename}
                              isAutoRenaming={autoRenamingPublicID === publicID}
                              onArchive={onArchive}
                              onShare={onShare}
                              onExport={onExport}
                              onDelete={onDelete}
                              onNavigate={onNavigate}
                              menuTriggerID={`recent-item-menu-trigger-${publicID}`}
                            />
                          );
                        })}
                      </React.Fragment>
                    ))}

                    {loadingMore ? (
                      <li className="flex items-center gap-2 px-2 py-2 text-xs text-muted-foreground">
                        <Spinner className="size-3.5" />
                        <span>{t("loadingMore")}</span>
                      </li>
                    ) : null}
                    {hasMore && !loadMoreFailed ? (
                      <li aria-hidden="true" className="h-4 list-none" ref={loadMoreRef} />
                    ) : null}

                    {loadMoreFailed ? (
                      <li className="flex items-center gap-2 px-2 py-2 text-xs text-muted-foreground">
                        <span>{t("loadMoreFailed")}</span>
                        <Button
                          type="button"
                          variant="link"
                          size="xs"
                          className="h-auto p-0 text-xs font-normal text-muted-foreground underline hover:text-foreground"
                          onClick={() => void retryLoadMore()}
                        >
                          {t("retry")}
                        </Button>
                      </li>
                    ) : null}
                  </SidebarMenu>
                </LoadingReveal>
              </div>
            </CollapsibleMotionContent>
          </SidebarGroup>
        </Collapsible>
      </div>

      <AlertDialog
        open={Boolean(deleteTarget)}
        onOpenChange={(open) => {
          if (!open) {
            setDeleteTarget(null);
            setDeleteFiles(false);
          }
        }}
      >
        <AlertDialogContent>
          <AlertDialogHeader>
            <AlertDialogTitle>{t("dialogs.deleteTitle")}</AlertDialogTitle>
            <AlertDialogDescription>
              {t("dialogs.deleteDescription", { label: t("deleteConversationLabel", { title: stableDeleteTarget?.title || t("untitled") }) })}
            </AlertDialogDescription>
            <DeleteFilesOption
              id={deleteFilesID}
              checked={deleteFiles}
              onCheckedChange={setDeleteFiles}
            />
          </AlertDialogHeader>
          <AlertDialogFooter>
            <AlertDialogCancel>{t("dialogs.cancel")}</AlertDialogCancel>
            <AlertDialogAction variant="destructive" onClick={() => void confirmDelete()}>
              {t("dialogs.delete")}
            </AlertDialogAction>
          </AlertDialogFooter>
        </AlertDialogContent>
      </AlertDialog>

      {stableShareTarget ? (
        <ConversationShareDialog
          open={Boolean(shareTarget)}
          onOpenChange={(open) => !open && setShareTarget(null)}
          conversationPublicID={stableShareTarget.publicID}
          conversationTitle={stableShareTarget.title}
          onShareChange={(share) => {
            touchByPublicID(stableShareTarget.publicID, sharePatchFromDTO(share));
          }}
        />
      ) : null}
    </>
  );
}
