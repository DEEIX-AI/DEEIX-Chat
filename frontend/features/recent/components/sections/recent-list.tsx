"use client";

import * as React from "react";
import Link from "next/link";
import { Archive, Check, PencilLine, Share2, Star, Trash } from "lucide-react";
import { useTranslations } from "next-intl";

import { Ellipsis } from "@/components/animate-ui/icons/ellipsis";
import { AnimatedText } from "@/components/ui/animated-text";
import { LoadingReveal } from "@/features/layouts/components/sections/loading-reveal";
import type { RecentRowState } from "@/features/recent/types/recent";
import { isArchivedConversation } from "@/features/recent/utils/conversation-list";
import {
  formatRelativeUpdatedAt,
  recentEmptyStateTitle,
} from "@/features/recent/utils/recent-display";
import {
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuItem,
  DropdownMenuItemIcon,
  DropdownMenuSeparator,
  DropdownMenuTrigger,
} from "@/components/ui/dropdown-menu";
import { Checkbox } from "@/components/ui/checkbox";
import { CenteredEmptyState } from "@/components/ui/empty-state";
import { Skeleton } from "@/components/ui/skeleton";
import { ConversationProjectSubmenu } from "@/shared/components/conversation-project-submenu";
import { ConversationShareExportSubmenu } from "@/shared/components/conversation-share-export-menu";
import { cn } from "@/lib/utils";
import { useAppLocale } from "@/i18n/app-i18n-provider";
import type {
  ConversationDTO,
  ConversationProjectDTO,
  ConversationShareFilter,
  ConversationStarredFilter,
  ConversationStatusFilter,
} from "@/shared/api/conversation.types";

function RecentRowSkeleton({
  showCheckbox = true,
}: {
  showCheckbox?: boolean;
}) {
  return (
    <div className="relative flex w-full items-stretch">
      <div
        aria-hidden="true"
        className="pointer-events-none absolute left-0 right-0 top-0 border-t border-border/60 md:left-13"
      />

      <div className="hidden w-13 shrink-0 items-center justify-center md:flex">
        {showCheckbox ? <Skeleton className="size-4 rounded-[5px] bg-muted/55" /> : null}
      </div>

      <div className="relative z-10 flex flex-1 items-center gap-3 px-2 py-3 md:px-3 md:py-3.5">
        <div className="min-w-0 flex-1">
          <div className="flex items-center gap-2">
            <Skeleton className="h-4 w-[min(15rem,42vw)] rounded-full bg-muted/58" />
            <Skeleton className="h-4 w-10 rounded-md bg-muted/44" />
          </div>
          <div className="mt-2">
            <Skeleton className="h-3 w-[min(12rem,34vw)] rounded-full bg-muted/42" />
          </div>
        </div>

        <Skeleton className="size-8 shrink-0 rounded-md bg-muted/46" />
      </div>
    </div>
  );
}

function RecentListSkeleton({
  count,
}: {
  count: number;
}) {
  return (
    <div className="pt-px">
      {Array.from({ length: count }).map((_, index) => (
        <RecentRowSkeleton key={`recent-skeleton-${index}`} />
      ))}
    </div>
  );
}

function RecentConversationRow({
  item,
  projects,
  hovered,
  selected,
  highlighted,
  selectionMode,
  hideTopDivider,
  mergeSelectedWithPrevious,
  mergeSelectedWithNext,
  onHoverChange,
  onToggleSelected,
  onToggleStar,
  onRename,
  onArchive,
  onShare,
  onRevokeShare,
  onSetProject,
  onExport,
  onDelete,
}: {
  item: ConversationDTO;
  projects: ConversationProjectDTO[];
  hovered: boolean;
  selected: boolean;
  highlighted: boolean;
  selectionMode: boolean;
  hideTopDivider: boolean;
  mergeSelectedWithPrevious: boolean;
  mergeSelectedWithNext: boolean;
  onHoverChange: (publicID: string | null) => void;
  onToggleSelected: (publicID: string) => void;
  onToggleStar: (publicID: string, nextStarred: boolean) => void;
  onRename: (item: ConversationDTO) => void;
  onArchive: (publicID: string, archived: boolean) => void;
  onShare: (item: ConversationDTO) => void;
  onRevokeShare: (publicID: string) => void | Promise<void>;
  onSetProject: (publicID: string, projectID?: string) => void | Promise<void>;
  onExport: (item: ConversationDTO) => void | Promise<void>;
  onDelete: (item: ConversationDTO) => void;
}) {
  const t = useTranslations("recent");
  const { locale } = useAppLocale();
  const [menuOpen, setMenuOpen] = React.useState(false);
  const archived = isArchivedConversation(item);
  const shared = item.shareStatus === "active" && Boolean(item.shareID?.trim());
  const title = item.title?.trim() || t("untitled");
  const updatedText = t("updatedAt", {
    time: formatRelativeUpdatedAt(item.updatedAt, locale, t("justNow")),
  });

  return (
    <div
      className="group relative flex w-full items-stretch"
      onMouseEnter={() => onHoverChange(item.publicID)}
      onMouseLeave={() => onHoverChange(null)}
    >
      <div
        aria-hidden="true"
        className={cn(
          "pointer-events-none absolute left-0 right-0 top-0 border-t border-border/60 transition-opacity duration-150 md:left-13",
          hideTopDivider && "opacity-0",
        )}
      />

      <div className="hidden w-13 shrink-0 items-center justify-center md:flex">
        <Checkbox
          checked={selected}
          aria-label={t("selectConversation", { title })}
          className={cn(
            "transition-opacity duration-150",
            selectionMode || highlighted ? "opacity-100" : "opacity-0 group-hover:opacity-100",
          )}
          onPointerDown={(event) => {
            event.stopPropagation();
          }}
          onCheckedChange={() => onToggleSelected(item.publicID)}
        />
      </div>

      <div
        className={cn(
          "relative z-10 flex flex-1 items-center gap-3 px-2 py-3 transition-[background-color,border-radius] duration-150 md:px-3 md:py-3.5",
          hovered && !selected && "rounded-2xl bg-accent/60",
          selected && !hovered && "bg-muted/60",
          selected && hovered && "bg-accent",
          selected && !mergeSelectedWithPrevious && "rounded-t-2xl",
          selected && mergeSelectedWithPrevious && "rounded-t-none",
          selected && !mergeSelectedWithNext && "rounded-b-2xl",
          selected && mergeSelectedWithNext && "rounded-b-none",
          !highlighted && "rounded-none bg-transparent",
        )}
      >
        {selectionMode ? (
          <div
            role="button"
            tabIndex={0}
            className="min-w-0 flex-1 cursor-pointer"
            onClick={() => onToggleSelected(item.publicID)}
            onKeyDown={(event) => {
              if (event.key === "Enter" || event.key === " ") {
                event.preventDefault();
                onToggleSelected(item.publicID);
              }
            }}
          >
            <div className="flex items-center gap-2">
              <AnimatedText
                text={title}
                className="min-w-0 flex-1"
                textClassName="text-sm font-medium text-foreground"
              />
              {archived ? <Archive className="size-3.5 fill-current text-foreground/45 self-center" /> : null}
              {item.isStarred ? <Star className="size-3.5 fill-current text-foreground/45 self-center" /> : null}
              {shared ? <Share2 className="size-3.5 fill-current text-foreground/45 self-center" /> : null}
            </div>
            <div className="mt-1 flex flex-wrap items-center gap-x-2 gap-y-1 text-[11px] text-muted-foreground">
              <span>{updatedText}</span>
            </div>
          </div>
        ) : (
          <Link href={`/chat?conversation_id=${item.publicID}`} prefetch={false} className="min-w-0 flex-1">
            <div className="flex items-center gap-2">
              <AnimatedText
                text={title}
                className="min-w-0 flex-1"
                textClassName="text-sm font-medium text-foreground"
              />
              {archived ? <Archive className="size-3.5 fill-current text-foreground/45 self-center" /> : null}
              {item.isStarred ? <Star className="size-3.5 fill-current text-foreground/45 self-center" /> : null}
              {shared ? <Share2 className="size-3.5 fill-current text-foreground/45 self-center" /> : null}
            </div>
            <div className="mt-1 flex flex-wrap items-center gap-x-2 gap-y-1 text-[11px] text-muted-foreground">
              <span>{updatedText}</span>
            </div>
          </Link>
        )}
 

        <DropdownMenu modal={false} open={menuOpen} onOpenChange={setMenuOpen}>
          <DropdownMenuTrigger asChild>
            <button
              id={`recent-page-item-menu-trigger-${item.publicID}`}
              className={cn(
                "flex size-8 shrink-0 items-center justify-center rounded-md text-muted-foreground opacity-0 transition-all duration-200 hover:bg-accent hover:text-foreground",
                highlighted && "opacity-100",
              )}
              onClick={(event) => {
                event.preventDefault();
                event.stopPropagation();
              }}
            >
              <Ellipsis size={16} strokeWidth={1.4} animate={hovered ? "pulse" : undefined} />
            </button>
          </DropdownMenuTrigger>
          <DropdownMenuContent align="end" className="w-max min-w-40 max-w-[calc(100vw-2rem)]">
            <DropdownMenuItem
              onSelect={(event) => {
                event.preventDefault();
                onToggleSelected(item.publicID);
              }}
            >
              <DropdownMenuItemIcon icon={Check} />
              {selected ? t("row.cancelSelect") : t("row.selectItem")}
            </DropdownMenuItem>
            <DropdownMenuSeparator />
            <DropdownMenuItem
              onSelect={(event) => {
                event.preventDefault();
                onToggleStar(item.publicID, !item.isStarred);
              }}
            >
              <DropdownMenuItemIcon icon={Star} />
              {item.isStarred ? t("row.unstar") : t("row.star")}
            </DropdownMenuItem>
            <DropdownMenuItem
              onSelect={(event) => {
                event.preventDefault();
                onRename(item);
              }}
            >
              <DropdownMenuItemIcon icon={PencilLine} />
              {t("row.rename")}
            </DropdownMenuItem>
            <ConversationProjectSubmenu
              label={t("row.moveToProject")}
              unassignedLabel={t("projects.unassigned")}
              currentProjectID={item.projectID}
              projects={projects}
              onSelect={(projectID) => onSetProject(item.publicID, projectID)}
            />
            <ConversationShareExportSubmenu
              label={t("row.shareAndExport")}
              shareLabel={shared ? t("row.manageShare") : t("row.share")}
              exportLabel={t("row.exportJSON")}
              onShare={() => onShare(item)}
              onExport={() => onExport(item)}
              onCloseMenu={() => setMenuOpen(false)}
            />
            <DropdownMenuItem
              onSelect={(event) => {
                event.preventDefault();
                onArchive(item.publicID, !archived);
              }}
            >
              <DropdownMenuItemIcon icon={Archive} />
              {archived ? t("row.unarchive") : t("row.archive")}
            </DropdownMenuItem>
            <DropdownMenuSeparator />
            <DropdownMenuItem
              variant="destructive"
              onSelect={(event) => {
                event.preventDefault();
                onDelete(item);
              }}
            >
              <DropdownMenuItemIcon icon={Trash} className="text-current" />
              {t("row.delete")}
            </DropdownMenuItem>
          </DropdownMenuContent>
        </DropdownMenu>
      </div>
    </div>
  );
}

type RecentListProps = {
  loadingInitial: boolean;
  filteredItems: ConversationDTO[];
  projects: ConversationProjectDTO[];
  normalizedQuery: string;
  statusFilter: ConversationStatusFilter;
  starredFilter: ConversationStarredFilter;
  shareFilter: ConversationShareFilter;
  rowStates: RecentRowState[];
  isSelectionMode: boolean;
  loadMoreRef: React.RefObject<HTMLDivElement | null>;
  hasMore: boolean;
  loadMoreFailed: boolean;
  loadingMore: boolean;
  onHoverChange: (publicID: string | null) => void;
  onToggleSelected: (publicID: string) => void;
  onToggleStar: (publicID: string, nextStarred: boolean) => void;
  onRename: (item: ConversationDTO) => void;
  onArchive: (publicID: string, archived: boolean) => void;
  onShare: (item: ConversationDTO) => void;
  onRevokeShare: (publicID: string) => void | Promise<void>;
  onSetProject: (publicID: string, projectID?: string) => void | Promise<void>;
  onExport: (item: ConversationDTO) => void | Promise<void>;
  onDelete: (item: ConversationDTO) => void;
  onRetryLoadMore: () => void | Promise<void>;
};

type RecentConversationGroup = {
  key: string;
  title: string;
  items: ConversationDTO[];
};

function buildConversationGroups(
  items: ConversationDTO[],
  projects: ConversationProjectDTO[],
  unassignedTitle: string,
): RecentConversationGroup[] {
  const itemsByProjectID = new Map<string, ConversationDTO[]>();
  const unknownProjectItems = new Map<string, ConversationDTO[]>();
  const unassignedItems: ConversationDTO[] = [];
  const groups: RecentConversationGroup[] = [];
  const knownProjectIDs = new Set(projects.map((project) => project.publicID));

  for (const item of items) {
    if (!item.projectID) {
      unassignedItems.push(item);
      continue;
    }
    if (knownProjectIDs.has(item.projectID)) {
      itemsByProjectID.set(item.projectID, [...(itemsByProjectID.get(item.projectID) ?? []), item]);
      continue;
    }
    const unknownProjectTitle = item.projectName || item.projectID;
    unknownProjectItems.set(unknownProjectTitle, [...(unknownProjectItems.get(unknownProjectTitle) ?? []), item]);
  }

  for (const project of projects) {
    const projectItems = itemsByProjectID.get(project.publicID);
    if (!projectItems?.length) {
      continue;
    }
    groups.push({
      key: project.publicID,
      title: project.name,
      items: projectItems,
    });
  }

  for (const [title, projectItems] of unknownProjectItems) {
    groups.push({
      key: `project:${title}`,
      title,
      items: projectItems,
    });
  }

  if (unassignedItems.length > 0) {
    groups.push({
      key: "unassigned",
      title: unassignedTitle,
      items: unassignedItems,
    });
  }

  return groups;
}

export function RecentList({
  loadingInitial,
  filteredItems,
  projects,
  normalizedQuery,
  statusFilter,
  starredFilter,
  shareFilter,
  rowStates,
  isSelectionMode,
  loadMoreRef,
  hasMore,
  loadMoreFailed,
  loadingMore,
  onHoverChange,
  onToggleSelected,
  onToggleStar,
  onRename,
  onArchive,
  onShare,
  onRevokeShare,
  onSetProject,
  onExport,
  onDelete,
  onRetryLoadMore,
}: RecentListProps) {
  const t = useTranslations("recent");
  const groups = React.useMemo(
    () => buildConversationGroups(filteredItems, projects, t("projects.unassigned")),
    [filteredItems, projects, t],
  );
  const rowStateByPublicID = React.useMemo(() => {
    const stateMap = new Map<string, RecentRowState>();
    for (const state of rowStates) {
      stateMap.set(state.publicID, state);
    }
    return stateMap;
  }, [rowStates]);
  const emptyState = (
    <CenteredEmptyState
      title={normalizedQuery ? t("emptyState.searchTitle") : recentEmptyStateTitle(statusFilter, starredFilter, shareFilter, {
        archived: t("archived"),
        active: t("active"),
        starred: t("starred"),
        unstarred: t("unstarred"),
        shared: t("shared"),
        unshared: t("unshared"),
        conjunction: t("emptyState.conjunction"),
        all: t("emptyState.allTitle"),
        filtered: (filters) => t("emptyState.filteredTitle", { filters }),
      })}
      description={normalizedQuery ? t("emptyState.searchDescription") : t("emptyState.allDescription")}
    />
  );

  const listContent = filteredItems.length === 0 ? (
    emptyState
  ) : (
    <div className="min-h-0 h-full overflow-y-auto pr-2">
      <div className="pt-px">
        {groups.map((group, groupIndex) => {
          return (
            <section key={group.key} className={cn(groupIndex > 0 && "mt-6")}>
              <div className="flex h-7 min-w-0 items-center justify-between gap-3 px-2 text-xs md:ml-13 md:px-3">
                <span className="min-w-0 truncate font-medium text-foreground/55">{group.title}</span>
                <span className="shrink-0 text-[11px] text-muted-foreground/55">
                  {t("conversationCount", { count: group.items.length })}
                </span>
              </div>

              {group.items.map((item, index) => {
                const currentState = rowStateByPublicID.get(item.publicID);
                const previousItem = index > 0 ? group.items[index - 1] : null;
                const nextItem = index < group.items.length - 1 ? group.items[index + 1] : null;
                const previousState = previousItem ? rowStateByPublicID.get(previousItem.publicID) : null;
                const nextState = nextItem ? rowStateByPublicID.get(nextItem.publicID) : null;

                return (
                  <RecentConversationRow
                    key={item.publicID}
                    item={item}
                    projects={projects}
                    hovered={currentState?.hovered ?? false}
                    selected={currentState?.selected ?? false}
                    highlighted={currentState?.highlighted ?? false}
                    selectionMode={isSelectionMode}
                    hideTopDivider={
                      (currentState?.highlighted ?? false) ||
                      (previousState?.highlighted ?? false)
                    }
                    mergeSelectedWithPrevious={(currentState?.selected ?? false) && (previousState?.selected ?? false)}
                    mergeSelectedWithNext={(currentState?.selected ?? false) && (nextState?.selected ?? false)}
                    onHoverChange={onHoverChange}
                    onToggleSelected={onToggleSelected}
                    onToggleStar={onToggleStar}
                    onRename={onRename}
                    onArchive={onArchive}
                    onShare={onShare}
                    onRevokeShare={onRevokeShare}
                    onSetProject={onSetProject}
                    onExport={onExport}
                    onDelete={onDelete}
                  />
                );
              })}
            </section>
          );
        })}

        {hasMore && !loadMoreFailed ? <div ref={loadMoreRef} className="h-4" aria-hidden="true" /> : null}

        {loadingMore ? (
          <div className="px-0 py-2">
            <RecentListSkeleton count={3} />
          </div>
        ) : null}

        {loadMoreFailed ? (
          <div className="flex items-center justify-center gap-3 px-3 py-4 text-xs text-muted-foreground">
            <span>{t("loadMoreFailed")}</span>
            <button
              type="button"
              className="underline underline-offset-4 transition-colors hover:text-foreground"
              onClick={() => {
                void onRetryLoadMore();
              }}
            >
              {t("retry")}
            </button>
          </div>
        ) : null}
      </div>
    </div>
  );

  return (
    <div className="mt-6 min-h-0 flex-1 overflow-hidden">
      <LoadingReveal
        loading={loadingInitial}
        className="h-full"
        skeletonClassName="h-full"
        contentClassName="h-full"
        skeleton={(
          <div className="min-h-0 h-full overflow-y-auto pr-2">
            <RecentListSkeleton count={8} />
          </div>
        )}
      >
        {listContent}
      </LoadingReveal>
    </div>
  );
}
