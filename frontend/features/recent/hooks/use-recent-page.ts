"use client";

import * as React from "react";
import { useRouter, useSearchParams } from "next/navigation";
import { useTranslations } from "next-intl";
import { toast } from "sonner";

import { useLocalizedErrorMessage } from "@/i18n/use-localized-error";
import { useLoadMoreSentinel } from "@/shared/hooks/use-load-more-sentinel";
import { useSidebarRecents } from "@/features/recent/context/sidebar-recents-context";
import { useSettingsChatPreferences } from "@/features/settings/hooks/use-settings-chat-preferences";
import { resolveAccessToken } from "@/shared/auth/resolve-access-token";
import { runBulkActionInChunks } from "@/shared/lib/bulk-action";
import {
  exportConversation,
  listConversations,
  revokeConversationShare,
  revokeConversationShares,
} from "@/shared/api/conversation";
import type {
  ConversationDTO,
  ConversationProjectFilter,
  ConversationShareDTO,
  ConversationShareFilter,
  ConversationStarredFilter,
  ConversationStatusFilter,
} from "@/shared/api/conversation.types";
import {
  mergeUniqueByPublicID,
  removeByPublicID,
  sortByUpdatedAtDesc,
  upsertByPublicID,
  isArchivedConversation,
} from "@/features/recent/utils/conversation-list";
import { RECENT_PAGE_SIZE } from "@/features/recent/utils/recent-display";
import type { RecentDeleteTarget, RecentRowState } from "@/features/recent/types/recent";
import { normalizeConversationSearchText } from "@/shared/lib/conversation-search";
import { downloadConversationExport } from "@/features/chat/model/conversation-export";

const RECENT_SEARCH_DEBOUNCE_MS = 250;

function isSharedConversation(item: ConversationDTO): boolean {
  return item.shareStatus === "active" && Boolean(item.shareID?.trim());
}

function conversationMatchesShareFilter(item: ConversationDTO, shareFilter: ConversationShareFilter): boolean {
  if (shareFilter === "shared") {
    return isSharedConversation(item);
  }
  if (shareFilter === "unshared") {
    return !isSharedConversation(item);
  }
  return true;
}

function conversationMatchesRecentFilters(
  item: ConversationDTO,
  statusFilter: ConversationStatusFilter,
  starredFilter: ConversationStarredFilter,
  shareFilter: ConversationShareFilter,
  projectFilter: ConversationProjectFilter,
): boolean {
  if (statusFilter === "archived" && !isArchivedConversation(item)) {
    return false;
  }
  if (statusFilter === "active" && isArchivedConversation(item)) {
    return false;
  }
  if (starredFilter === "starred" && !item.isStarred) {
    return false;
  }
  if (starredFilter === "unstarred" && item.isStarred) {
    return false;
  }
  if (!conversationMatchesShareFilter(item, shareFilter)) {
    return false;
  }
  if (projectFilter === "unassigned") {
    return !item.projectID;
  }
  if (projectFilter !== "all") {
    return item.projectID === projectFilter;
  }
  return true;
}

function sharePatchFromResult(share: ConversationShareDTO): Partial<ConversationDTO> {
  const active = share.status === "active" && Boolean(share.shareID.trim());
  return {
    shareStatus: share.status,
    shareID: active ? share.shareID : "",
    sharedAt: active ? share.createdAt : null,
    lastShareAccessedAt: share.lastAccessedAt,
  };
}

export function useRecentPage() {
  const t = useTranslations("recent");
  const resolveErrorMessage = useLocalizedErrorMessage();
  const router = useRouter();
  const {
    prependNewConversation,
    renameByPublicID,
    regenerateTitleByPublicID,
    setStarByPublicID,
    archiveByPublicID,
    deleteByPublicID,
    projects,
    setProjectByPublicID,
    touchByPublicID,
    lastChange,
  } = useSidebarRecents();
  const [items, setItems] = React.useState<ConversationDTO[]>([]);
  const [loadingInitial, setLoadingInitial] = React.useState(true);
  const [loadingMore, setLoadingMore] = React.useState(false);
  const [hasMore, setHasMore] = React.useState(true);
  const [loadMoreFailed, setLoadMoreFailed] = React.useState(false);
  const [statusFilter, setStatusFilter] = React.useState<ConversationStatusFilter>("all");
  const [starredFilter, setStarredFilter] = React.useState<ConversationStarredFilter>("all");
  const [shareFilter, setShareFilter] = React.useState<ConversationShareFilter>("all");
  const searchParams = useSearchParams();
  const [projectFilter, setProjectFilter] = React.useState<ConversationProjectFilter>(() => searchParams.get("project") || "all");
  const [query, setQuery] = React.useState("");
  const [debouncedQuery, setDebouncedQuery] = React.useState("");
  const [selectionMode, setSelectionMode] = React.useState(false);
  const [hoveredConversationID, setHoveredConversationID] = React.useState<string | null>(null);
  const [selectedConversationIDs, setSelectedConversationIDs] = React.useState<string[]>([]);
  const [renameTarget, setRenameTarget] = React.useState<ConversationDTO | null>(null);
  const [renameValue, setRenameValue] = React.useState("");
  const [renamingAutomatically, setRenamingAutomatically] = React.useState(false);
  const [deleteTarget, setDeleteTarget] = React.useState<RecentDeleteTarget>(null);
  const [deleteFiles, setDeleteFiles] = React.useState(false);
  const { deleteFilesByDefault } = useSettingsChatPreferences();
  const [shareTarget, setShareTarget] = React.useState<ConversationDTO | null>(null);
  const loadMoreRef = React.useRef<HTMLDivElement | null>(null);
  const pageRef = React.useRef(1);
  const requestVersionRef = React.useRef(0);
  const loadingMoreRef = React.useRef(false);
  const loadMoreFailedRef = React.useRef(false);
  const isSelectionMode = selectionMode || selectedConversationIDs.length > 0;

  React.useEffect(() => {
    setProjectFilter(searchParams.get("project") || "all");
  }, [searchParams]);

  React.useEffect(() => {
    loadingMoreRef.current = loadingMore;
  }, [loadingMore]);

  React.useEffect(() => {
    loadMoreFailedRef.current = loadMoreFailed;
  }, [loadMoreFailed]);

  const normalizedQuery = normalizeConversationSearchText(debouncedQuery);
  const filteredItems = items;

  React.useEffect(() => {
    const timer = window.setTimeout(() => {
      setDebouncedQuery(query);
    }, RECENT_SEARCH_DEBOUNCE_MS);

    return () => window.clearTimeout(timer);
  }, [query]);

  const lastAppliedChangeSequenceRef = React.useRef(0);

  React.useEffect(() => {
    if (!lastChange || lastChange.sequence <= lastAppliedChangeSequenceRef.current) {
      return;
    }
    lastAppliedChangeSequenceRef.current = lastChange.sequence;

    if (lastChange.type === "remove") {
      setItems((current) => removeByPublicID(current, lastChange.publicID));
      setSelectedConversationIDs((current) => current.filter((item) => item !== lastChange.publicID));
      return;
    }

    if (lastChange.type === "patch" && lastChange.patch) {
      setItems((current) =>
        current
          .map((item) => (item.publicID === lastChange.publicID ? { ...item, ...lastChange.patch } : item))
          .filter((item) => conversationMatchesRecentFilters(item, statusFilter, starredFilter, shareFilter, projectFilter)),
      );
      return;
    }

    if (!lastChange.item) {
      return;
    }

    if (!conversationMatchesRecentFilters(lastChange.item, statusFilter, starredFilter, shareFilter, projectFilter)) {
      setItems((current) => removeByPublicID(current, lastChange.publicID));
      setSelectedConversationIDs((current) => current.filter((item) => item !== lastChange.publicID));
      return;
    }

    if (normalizedQuery && !items.some((item) => item.publicID === lastChange.publicID)) {
      return;
    }

    setItems((current) => upsertByPublicID(current, lastChange.item!));
  }, [items, lastChange, normalizedQuery, projectFilter, shareFilter, starredFilter, statusFilter]);

  const loadPage = React.useCallback(
    async (page: number, options?: { replace?: boolean; version?: number }) => {
      const requestVersion = options?.version ?? requestVersionRef.current;
      const token = await resolveAccessToken();
      if (!token) {
        if (requestVersion === requestVersionRef.current) {
          setItems([]);
          setHasMore(false);
        }
        return;
      }

      const data = await listConversations(token, {
        page,
        pageSize: RECENT_PAGE_SIZE,
        status: statusFilter,
        starred: starredFilter,
        share: shareFilter,
        project: projectFilter,
        query: normalizedQuery,
      });
      if (requestVersion !== requestVersionRef.current) {
        return;
      }

      const nextResults = data.results ?? [];
      setItems((current) => (
        options?.replace ? sortByUpdatedAtDesc(nextResults) : mergeUniqueByPublicID(current, nextResults)
      ));

      const loaded = data.results?.length ?? 0;
      const total = data.total ?? 0;
      const mergedCount = (page - 1) * RECENT_PAGE_SIZE + loaded;
      setHasMore(loaded === RECENT_PAGE_SIZE && mergedCount < total);
      setLoadMoreFailed(false);
      loadMoreFailedRef.current = false;
      pageRef.current = page;
    },
    [normalizedQuery, projectFilter, shareFilter, starredFilter, statusFilter],
  );

  React.useEffect(() => {
    requestVersionRef.current += 1;
    const version = requestVersionRef.current;

    setLoadingInitial(true);
    setItems([]);
    setHasMore(true);
    setLoadMoreFailed(false);
    loadMoreFailedRef.current = false;
    setSelectionMode(false);
    setSelectedConversationIDs([]);
    setHoveredConversationID(null);
    pageRef.current = 1;

    let cancelled = false;
    (async () => {
      try {
        await loadPage(1, { replace: true, version });
      } finally {
        if (!cancelled && version === requestVersionRef.current) {
          setLoadingInitial(false);
        }
      }
    })();

    return () => {
      cancelled = true;
    };
  }, [loadPage]);

  const loadMore = React.useCallback(async () => {
    if (loadingInitial || loadingMoreRef.current || !hasMore || loadMoreFailedRef.current) {
      return;
    }

    loadingMoreRef.current = true;
    setLoadingMore(true);
    try {
      await loadPage(pageRef.current + 1, { version: requestVersionRef.current });
    } catch (error) {
      loadMoreFailedRef.current = true;
      setLoadMoreFailed(true);
      const description = resolveErrorMessage(error, t("loadMoreFailed"));
      toast.error(t("loadMoreFailed"), {
        id: "recent-load-more-error",
        description,
      });
    } finally {
      loadingMoreRef.current = false;
      setLoadingMore(false);
    }
  }, [hasMore, loadPage, loadingInitial, resolveErrorMessage, t]);

  useLoadMoreSentinel({
    enabled: hasMore && !loadingInitial && !loadMoreFailed,
    targetRef: loadMoreRef,
    rootMargin: "160px",
    onLoadMore: loadMore,
  });

  const retryLoadMore = React.useCallback(async () => {
    setLoadMoreFailed(false);
    loadMoreFailedRef.current = false;
    await loadMore();
  }, [loadMore]);

  const onCreateConversation = React.useCallback(async () => {
    const currentProjectID = projectFilter !== "all" && projectFilter !== "unassigned" ? projectFilter : "";
    const created = await prependNewConversation(undefined, currentProjectID);
    if (created?.publicID) {
      router.push(`/chat?conversation_id=${created.publicID}`);
      return;
    }
    router.push("/chat");
  }, [prependNewConversation, projectFilter, router]);

  const onProjectFilterChange = React.useCallback(
    (value: ConversationProjectFilter) => {
      setProjectFilter(value);
      const href = value === "all" ? "/recent" : `/recent?project=${encodeURIComponent(value)}`;
      router.replace(href);
    },
    [router],
  );

  const onToggleSelected = React.useCallback((publicID: string) => {
    setSelectionMode(true);
    setSelectedConversationIDs((current) =>
      current.includes(publicID) ? current.filter((item) => item !== publicID) : [...current, publicID],
    );
  }, []);

  const onToggleStar = React.useCallback(
    async (publicID: string, nextStarred: boolean) => {
      const updated = await setStarByPublicID(publicID, nextStarred);
      if (!updated) {
        return;
      }
      setItems((current) => (
        conversationMatchesRecentFilters(updated, statusFilter, starredFilter, shareFilter, projectFilter)
          ? upsertByPublicID(current, updated)
          : removeByPublicID(current, publicID)
      ));
    },
    [projectFilter, setStarByPublicID, shareFilter, starredFilter, statusFilter],
  );

  const onRename = React.useCallback((item: ConversationDTO) => {
    setRenameTarget(item);
    setRenameValue(item.title || t("untitled"));
  }, [t]);

  const onArchive = React.useCallback(
    async (publicID: string, archived: boolean) => {
      const updated = await archiveByPublicID(publicID, archived);
      setSelectedConversationIDs((current) => current.filter((item) => item !== publicID));
      if (!updated) {
        return;
      }

      setItems((current) => {
        if (!conversationMatchesRecentFilters(updated, statusFilter, starredFilter, shareFilter, projectFilter)) {
          return removeByPublicID(current, publicID);
        }
        return upsertByPublicID(current, updated);
      });
    },
    [archiveByPublicID, projectFilter, shareFilter, starredFilter, statusFilter],
  );

  const patchConversationShare = React.useCallback(
    (publicID: string, patch: Partial<ConversationDTO>) => {
      setItems((current) =>
        current
          .map((item) => (item.publicID === publicID ? { ...item, ...patch } : item))
          .filter((item) => conversationMatchesRecentFilters(item, statusFilter, starredFilter, shareFilter, projectFilter)),
      );
      const activeAfterPatch = patch.shareStatus === "active" && Boolean(patch.shareID?.trim());
      const keepSelected =
        shareFilter === "all" ||
        (shareFilter === "shared" && activeAfterPatch) ||
        (shareFilter === "unshared" && !activeAfterPatch);
      if (!keepSelected) {
        setSelectedConversationIDs((current) => current.filter((item) => item !== publicID));
      }
      touchByPublicID(publicID, patch);
      setShareTarget((current) => (current?.publicID === publicID ? { ...current, ...patch } : current));
    },
    [projectFilter, shareFilter, starredFilter, statusFilter, touchByPublicID],
  );

  const onShare = React.useCallback((item: ConversationDTO) => {
    setShareTarget(item);
  }, []);

  const onSetProject = React.useCallback(
    async (publicID: string, projectID?: string) => {
      const updated = await setProjectByPublicID(publicID, projectID);
      if (!updated) {
        return;
      }
      setItems((current) =>
        conversationMatchesRecentFilters(updated, statusFilter, starredFilter, shareFilter, projectFilter)
          ? upsertByPublicID(current, updated)
          : removeByPublicID(current, publicID),
      );
      setSelectedConversationIDs((current) => (
        conversationMatchesRecentFilters(updated, statusFilter, starredFilter, shareFilter, projectFilter)
          ? current
          : current.filter((item) => item !== publicID)
      ));
    },
    [projectFilter, setProjectByPublicID, shareFilter, starredFilter, statusFilter],
  );

  const closeShareDialog = React.useCallback(() => {
    setShareTarget(null);
  }, []);

  const onShareChange = React.useCallback(
    (share: ConversationShareDTO) => {
      if (!shareTarget) {
        return;
      }
      patchConversationShare(shareTarget.publicID, sharePatchFromResult(share));
    },
    [patchConversationShare, shareTarget],
  );

  const onRevokeShare = React.useCallback(
    async (publicID: string) => {
      const token = await resolveAccessToken();
      if (!token) {
        return;
      }
      const updated = await revokeConversationShare(token, publicID);
      patchConversationShare(publicID, sharePatchFromResult(updated));
      toast.success(t("shareClosed"));
    },
    [patchConversationShare, t],
  );

  const onDelete = React.useCallback((item: ConversationDTO) => {
    setDeleteFiles(deleteFilesByDefault);
    setDeleteTarget({
      ids: [item.publicID],
      label: t("deleteConversationLabel", { title: item.title || t("untitled") }),
    });
  }, [deleteFilesByDefault, t]);

  const onExport = React.useCallback(async (item: ConversationDTO) => {
    const token = await resolveAccessToken();
    if (!token) {
      return;
    }
    try {
      const data = await exportConversation(token, item.publicID);
      downloadConversationExport(data);
      toast.success(t("exported"));
    } catch (error) {
      toast.error(t("exportFailed"), {
        description: resolveErrorMessage(error, t("exportFailed")),
      });
    }
  }, [resolveErrorMessage, t]);

  const onRenameCommit = React.useCallback(async () => {
    if (!renameTarget) {
      return;
    }

    const nextTitle = renameValue.trim();
    if (!nextTitle || nextTitle === renameTarget.title) {
      setRenameTarget(null);
      setRenameValue("");
      return;
    }

    const updated = await renameByPublicID(renameTarget.publicID, nextTitle);
    if (updated) {
      setItems((current) => upsertByPublicID(current, updated));
    }
    setRenameTarget(null);
    setRenameValue("");
  }, [renameByPublicID, renameTarget, renameValue]);

  const onAutoRename = React.useCallback(async () => {
    if (!renameTarget || renamingAutomatically) {
      return;
    }

    setRenamingAutomatically(true);
    try {
      const updated = await regenerateTitleByPublicID(renameTarget.publicID);
      if (updated) {
        setItems((current) => upsertByPublicID(current, updated));
        setRenameTarget(null);
        setRenameValue("");
      }
    } catch (error) {
      toast.error(t("dialogs.autoRenameFailed"), {
        description: resolveErrorMessage(error, t("dialogs.autoRenameFailed")),
      });
    } finally {
      setRenamingAutomatically(false);
    }
  }, [regenerateTitleByPublicID, renameTarget, renamingAutomatically, resolveErrorMessage, t]);

  const confirmDelete = React.useCallback(async () => {
    if (!deleteTarget) {
      return;
    }

    await runBulkActionInChunks({
      chunkSize: 10,
      items: deleteTarget.ids,
      title: t("labelMenu.bulk.pending"),
      runChunk: async (ids) => {
        for (const id of ids) {
          await deleteByPublicID(id, deleteFiles ? { deleteFiles: true } : undefined);
        }
      },
    });
    setItems((current) => current.filter((item) => !deleteTarget.ids.includes(item.publicID)));
    setSelectedConversationIDs((current) => current.filter((item) => !deleteTarget.ids.includes(item)));
    if (deleteTarget.ids.length > 1) {
      setSelectionMode(false);
    }
    setDeleteTarget(null);
    setDeleteFiles(false);
  }, [deleteByPublicID, deleteFiles, deleteTarget, t]);

  const closeDeleteDialog = React.useCallback(() => {
    setDeleteTarget(null);
    setDeleteFiles(false);
  }, []);

  const exitSelectionMode = React.useCallback(() => {
    setSelectionMode(false);
    setSelectedConversationIDs([]);
  }, []);

  const toggleSelectionMode = React.useCallback(
    (checked: boolean | "indeterminate") => {
      const visibleConversationIDs = filteredItems.map((item) => item.publicID);

      if (checked) {
        setSelectionMode(true);
        setSelectedConversationIDs((current) => {
          const next = new Set(current);
          for (const id of visibleConversationIDs) {
            next.add(id);
          }
          return Array.from(next);
        });
        return;
      }

      exitSelectionMode();
    },
    [exitSelectionMode, filteredItems],
  );

  const selectedItems = React.useMemo(
    () => items.filter((item) => selectedConversationIDs.includes(item.publicID)),
    [items, selectedConversationIDs],
  );

  const selectedSharedItems = React.useMemo(
    () => selectedItems.filter(isSharedConversation),
    [selectedItems],
  );

  const allSelectedArchived = React.useMemo(
    () => selectedItems.length > 0 && selectedItems.every((item) => isArchivedConversation(item)),
    [selectedItems],
  );

  const archiveSelected = React.useCallback(async () => {
    if (selectedItems.length === 0) {
      return;
    }

    const nextArchived = !allSelectedArchived;
    const targets = selectedItems.filter((item) => isArchivedConversation(item) !== nextArchived);
    const updates = (await runBulkActionInChunks({
      chunkSize: 10,
      items: targets,
      title: t("labelMenu.bulk.pending"),
      runChunk: async (chunk) => {
        const updatedItems: Array<ConversationDTO | null> = [];
        for (const item of chunk) {
          updatedItems.push(await archiveByPublicID(item.publicID, nextArchived));
        }
        return updatedItems;
      },
    })).flat();

    setItems((current) => {
      let next = current;
      for (let index = 0; index < targets.length; index += 1) {
        const target = targets[index];
        const updated = updates[index];
        if (!updated) {
          continue;
        }
        if ((statusFilter === "active" && nextArchived) || (statusFilter === "archived" && !nextArchived)) {
          next = removeByPublicID(next, target.publicID);
          continue;
        }
        next = conversationMatchesRecentFilters(updated, statusFilter, starredFilter, shareFilter, projectFilter)
          ? upsertByPublicID(next, updated)
          : removeByPublicID(next, target.publicID);
      }
      return next;
    });
    setSelectedConversationIDs([]);
    setSelectionMode(false);
  }, [allSelectedArchived, archiveByPublicID, projectFilter, selectedItems, shareFilter, starredFilter, statusFilter, t]);

  const revokeSelectedShares = React.useCallback(async () => {
    if (selectedSharedItems.length === 0) {
      return;
    }
    const token = await resolveAccessToken();
    if (!token) {
      return;
    }
    const ids = selectedSharedItems.map((item) => item.publicID);
    await runBulkActionInChunks({
      items: ids,
      title: t("labelMenu.bulk.pending"),
      runChunk: (conversationPublicIDs) => revokeConversationShares(token, { conversationPublicIDs }),
    });
    const patch: Partial<ConversationDTO> = {
      shareStatus: "revoked",
      shareID: "",
      sharedAt: null,
      lastShareAccessedAt: null,
    };
    setItems((current) =>
      current
        .map((item) => (ids.includes(item.publicID) ? { ...item, ...patch } : item))
        .filter((item) => conversationMatchesRecentFilters(item, statusFilter, starredFilter, shareFilter, projectFilter)),
    );
    for (const id of ids) {
      touchByPublicID(id, patch);
    }
    setSelectedConversationIDs([]);
    setSelectionMode(false);
    toast.success(t("shareClosed"));
  }, [projectFilter, selectedSharedItems, shareFilter, starredFilter, statusFilter, t, touchByPublicID]);

  const requestDeleteSelected = React.useCallback(() => {
    if (selectedConversationIDs.length === 0) {
      return;
    }

    setDeleteFiles(deleteFilesByDefault);
    setDeleteTarget({
      ids: [...selectedConversationIDs],
      label: t("selectedConversationCountLabel", { count: selectedConversationIDs.length }),
    });
  }, [deleteFilesByDefault, selectedConversationIDs, t]);

  const rowStates = React.useMemo<RecentRowState[]>(
    () =>
      filteredItems.map((item) => {
        const hovered = hoveredConversationID === item.publicID;
        const selected = selectedConversationIDs.includes(item.publicID);
        return {
          publicID: item.publicID,
          hovered,
          selected,
          highlighted: hovered || selected,
        };
      }),
    [filteredItems, hoveredConversationID, selectedConversationIDs],
  );

  const visibleSelectedCount = React.useMemo(
    () => filteredItems.filter((item) => selectedConversationIDs.includes(item.publicID)).length,
    [filteredItems, selectedConversationIDs],
  );

  const pageSelectionState = React.useMemo<boolean | "indeterminate">(() => {
    if (filteredItems.length === 0 || visibleSelectedCount === 0) {
      return false;
    }

    if (visibleSelectedCount === filteredItems.length) {
      return true;
    }

    return "indeterminate";
  }, [filteredItems.length, visibleSelectedCount]);

  return {
    items,
    filteredItems,
    normalizedQuery,
    loadingInitial,
    loadingMore,
    hasMore,
    loadMoreFailed,
    statusFilter,
    starredFilter,
    shareFilter,
    projectFilter,
    projects,
    query,
    isSelectionMode,
    selectedConversationIDs,
    hoveredConversationID,
    renameTarget,
    renameValue,
    renamingAutomatically,
    deleteTarget,
    deleteFiles,
    shareTarget,
    rowStates,
    allSelectedArchived,
    selectedSharedCount: selectedSharedItems.length,
    pageSelectionState,
    loadMoreRef,
    onCreateConversation,
    setQuery,
    setStatusFilter,
    setStarredFilter,
    setShareFilter,
    setProjectFilter: onProjectFilterChange,
    setHoveredConversationID,
    onToggleSelected,
    onToggleStar,
    onRename,
    onArchive,
    onShare,
    onSetProject,
    onRevokeShare,
    onExport,
    onDelete,
    setRenameValue,
    onRenameCommit,
    onAutoRename,
    closeRenameDialog: () => {
      setRenameTarget(null);
      setRenameValue("");
    },
    confirmDelete,
    closeDeleteDialog,
    setDeleteFiles,
    closeShareDialog,
    onShareChange,
    toggleSelectionMode,
    archiveSelected,
    revokeSelectedShares,
    requestDeleteSelected,
    exitSelectionMode,
    enterSelectionMode: () => setSelectionMode(true),
    retryLoadMore,
  };
}
