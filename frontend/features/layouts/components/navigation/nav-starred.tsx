"use client";

import * as React from "react";
import { useRouter } from "next/navigation";
import { motion } from "motion/react";
import { ChevronDown, StarOff } from "lucide-react";
import { useTranslations } from "next-intl";

import { List } from "@/components/animate-ui/icons/list";
import { Button } from "@/components/ui/button";
import { Collapsible } from "@/components/ui/collapsible";
import {
  AlertDialog,
  AlertDialogAction,
  AlertDialogCancel,
  AlertDialogContent,
  AlertDialogDescription as AlertDialogBody,
  AlertDialogFooter,
  AlertDialogHeader,
  AlertDialogTitle,
} from "@/components/ui/alert-dialog";
import {
  SidebarGroup,
  SidebarGroupLabel,
  SidebarMenu,
  SidebarMenuButton,
  useSidebar,
} from "@/components/ui/sidebar";
import {
  ConversationShareDialog,
  sharePatchFromDTO,
  useConversationExport,
  useSidebarConversations,
} from "@/entities/conversation";
import { LoadingReveal } from "@/shared/components/loading-reveal";
import { NavigationSearch } from "@/features/layouts/components/navigation/navigation-search";
import { SidebarConversationItem } from "@/features/layouts/components/navigation/sidebar-conversation-item";
import { SidebarConversationSkeleton } from "@/features/layouts/components/navigation/sidebar-conversation-skeleton";
import { CollapsibleMotionContent } from "@/shared/components/collapsible-motion-content";
import { DeleteFilesOption } from "@/shared/components/delete-files-option";
import { useDialogSnapshot } from "@/shared/hooks/use-dialog-snapshot";
import { useSettingsChatPreferences } from "@/features/settings";
import { useLayoutActiveConversation } from "@/features/layouts/hooks/use-layout-active-conversation";
import { useLayoutSidebarListFlip } from "@/features/layouts/hooks/use-layout-sidebar-list-flip";
import { useSidebarConversationNavigation } from "@/features/layouts/hooks/use-sidebar-conversation-navigation";
import { SIDEBAR_OVERFLOW_ROW_TRANSITION } from "@/features/layouts/model/sidebar-motion";
import type {
  SidebarConversationDeleteTarget,
  SidebarConversationRenameTarget,
} from "@/features/layouts/types/navigation";
import { filterConversationSearchResults } from "@/features/layouts/model/navigation-search";
import type { ConversationDTO } from "@/shared/api/conversation.types";
import { useStoredBoolean } from "@/shared/hooks/use-stored-boolean";
import { cn } from "@/lib/utils";

const STARRED_SKELETON_WIDTHS = ["71%", "59%", "66%", "54%", "70%"] as const;
const MAX_VISIBLE_STARRED = 5;
const STARRED_OPEN_STORAGE_KEY = "deeix.sidebar.starred.open";

export function NavStarred() {
  const t = useTranslations("recent");
  const { isMobile, setOpenMobile } = useSidebar();
  const router = useRouter();
  const onNavigate = useSidebarConversationNavigation();
  const activeConversationID = useLayoutActiveConversation();
  const { deleteFilesByDefault } = useSettingsChatPreferences();

  const {
    starredItems,
    projects,
    starredTotal,
    loadingInitial,
    transferringStarPublicID,
    setStarByPublicID,
    renameByPublicID,
    regenerateTitleByPublicID,
    loadAllStarred,
    archiveByPublicID,
    deleteByPublicID,
    touchByPublicID,
    setProjectByPublicID,
  } = useSidebarConversations();

  const [showAllStarredDialog, setShowAllStarredDialog] = React.useState(false);
  const [dialogStarredItems, setDialogStarredItems] = React.useState<ConversationDTO[] | null>(null);
  const [dialogLoading, setDialogLoading] = React.useState(false);
  const [searchQuery, setSearchQuery] = React.useState("");
  const [deleteTarget, setDeleteTarget] = React.useState<SidebarConversationDeleteTarget>(null);
  const [deleteFiles, setDeleteFiles] = React.useState(false);
  const [renameTarget, setRenameTarget] = React.useState<SidebarConversationRenameTarget>(null);
  const [shareTarget, setShareTarget] = React.useState<{
    publicID: string;
    title: string;
  } | null>(null);
  const [renameValue, setRenameValue] = React.useState("");
  const [autoRenamingPublicID, setAutoRenamingPublicID] = React.useState<string | null>(null);
  const [starredOpen, setStarredOpen] = useStoredBoolean(STARRED_OPEN_STORAGE_KEY, true);
  const listContainerRef = React.useRef<HTMLDivElement | null>(null);
  const deleteFilesID = React.useId();
  const starredContentID = React.useId();
  const stableDeleteTarget = useDialogSnapshot(deleteTarget);
  const stableShareTarget = useDialogSnapshot(shareTarget);
  const onExport = useConversationExport({
    successMessage: t("exported"),
    failureMessage: t("exportFailed"),
  });

  const starredConversationItems = React.useMemo(
    () => starredItems.map((item) => ({
      publicID: item.publicID,
      title: item.title || t("untitled"),
      url: `/chat?conversation_id=${item.publicID}`,
    })),
    [starredItems, t],
  );
  const visibleStarredItems = React.useMemo(
    () => starredConversationItems.slice(0, MAX_VISIBLE_STARRED),
    [starredConversationItems],
  );
  const hasOverflowButton = starredTotal > MAX_VISIBLE_STARRED;
  const visibleStarredSignature = React.useMemo(
    () => `${visibleStarredItems.map((item) => item.publicID).join("|")}::overflow:${hasOverflowButton ? "1" : "0"}`,
    [hasOverflowButton, visibleStarredItems],
  );
  const commandResults = React.useMemo(
    () => filterConversationSearchResults(dialogStarredItems ?? starredItems, searchQuery, { untitled: t("untitled") }),
    [dialogStarredItems, searchQuery, starredItems, t],
  );
  const showInitialSkeleton = loadingInitial && starredConversationItems.length === 0;

  useLayoutSidebarListFlip(listContainerRef, {
    enabled: starredOpen && Boolean(transferringStarPublicID),
    signature: visibleStarredSignature,
    excludeKey: transferringStarPublicID,
  });

  React.useEffect(() => {
    if (!showAllStarredDialog) {
      setDialogLoading(false);
      setDialogStarredItems(null);
      setSearchQuery("");
      return;
    }

    if (starredTotal <= starredItems.length) {
      setDialogLoading(false);
      setDialogStarredItems(starredItems);
      return;
    }

    let cancelled = false;
    setDialogLoading(true);
    void loadAllStarred()
      .then((items) => {
        if (!cancelled) {
          setDialogStarredItems(items);
        }
      })
      .catch(() => {
        if (!cancelled) {
          setDialogStarredItems(starredItems);
        }
      })
      .finally(() => {
        if (!cancelled) {
          setDialogLoading(false);
        }
      });

    return () => {
      cancelled = true;
    };
  }, [loadAllStarred, showAllStarredDialog, starredItems, starredTotal]);

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

  const onUnstar = React.useCallback(
    (publicID: string) => {
      void setStarByPublicID(publicID, false);
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

  const onSelectSearchResult = React.useCallback((href: string) => {
    setShowAllStarredDialog(false);
    if (isMobile) {
      setOpenMobile(false);
    }
    router.push(href);
  }, [isMobile, router, setOpenMobile]);

  if (!loadingInitial && starredTotal === 0 && starredConversationItems.length === 0) {
    return null;
  }

  return (
    <>
      <div className="relative z-10 group-data-[collapsible=icon]:pointer-events-none group-data-[collapsible=icon]:opacity-0">
        <motion.div
          className="overflow-hidden"
          initial={showInitialSkeleton ? false : { height: 0, opacity: 0, y: -4 }}
          animate={{ height: "auto", opacity: 1, y: 0 }}
          transition={SIDEBAR_OVERFLOW_ROW_TRANSITION}
        >
          <Collapsible open={starredOpen} onOpenChange={setStarredOpen}>
            <SidebarGroup className="px-2 py-2">
            <SidebarGroupLabel
              asChild
              className="w-fit max-w-full self-start cursor-pointer gap-1 pr-1 transition-[color,margin,opacity] hover:text-sidebar-foreground"
            >
              <Button
                type="button"
                variant="ghost"
                className="h-8 gap-1 py-0 pl-2 pr-1 text-xs hover:bg-transparent has-[>svg]:pl-2 has-[>svg]:pr-1 dark:hover:bg-transparent"
                aria-controls={starredContentID}
                aria-expanded={starredOpen}
                aria-label={starredOpen ? t("collapseStarredSection") : t("expandStarredSection")}
                onClick={() => setStarredOpen((open) => !open)}
              >
                <span className="min-w-0 truncate text-left">{t("starred")}</span>
                <ChevronDown
                  aria-hidden
                  className={cn(
                    "!size-3 stroke-1.5 transition-transform duration-200",
                    !starredOpen && "-rotate-90",
                  )}
                />
              </Button>
            </SidebarGroupLabel>
            <CollapsibleMotionContent id={starredContentID} open={starredOpen}>
              <div ref={listContainerRef}>
                <LoadingReveal
                  loading={showInitialSkeleton}
                  skeleton={<SidebarConversationSkeleton count={3} widths={STARRED_SKELETON_WIDTHS} prefix="sidebar-starred" />}
                  className="min-h-0"
                >
                  <SidebarMenu className="gap-0.5">
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
                        projectMenu={{
                          label: t("row.moveToProject"),
                          unassignedLabel: t("projects.unassigned"),
                          currentProjectID: starredItems.find((conversation) => conversation.publicID === item.publicID)?.projectID,
                          projects,
                          onSelect: (targetPublicID, projectID) => {
                            void setProjectByPublicID(targetPublicID, projectID);
                          },
                        }}
                        onRename={onRename}
                        isRenaming={renameTarget?.publicID === item.publicID}
                        renameValue={renameTarget?.publicID === item.publicID ? renameValue : item.title}
                        onRenameValueChange={setRenameValue}
                        onRenameCommit={onRenameCommit}
                        onRenameCancel={onRenameCancel}
                        onAutoRename={onAutoRename}
                        isAutoRenaming={autoRenamingPublicID === item.publicID}
                        onArchive={onArchive}
                        onShare={onShare}
                        onExport={onExport}
                        onDelete={onDelete}
                        onNavigate={onNavigate}
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
                            setShowAllStarredDialog(true);
                          }
                        }}
                      >
                        <List aria-hidden size={16} strokeWidth={1.4} />
                        <span className="text-xs text-sidebar-foreground/75">{t("allConversations")}</span>
                      </SidebarMenuButton>
                    </motion.li>
                  </SidebarMenu>
                </LoadingReveal>
              </div>
            </CollapsibleMotionContent>
            </SidebarGroup>
          </Collapsible>
        </motion.div>
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
            <AlertDialogBody>
              {t("dialogs.deleteDescription", { label: t("deleteConversationLabel", { title: stableDeleteTarget?.title || t("untitled") }) })}
            </AlertDialogBody>
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
