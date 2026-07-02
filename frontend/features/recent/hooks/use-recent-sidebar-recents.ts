"use client";

import * as React from "react";
import { useTranslations } from "next-intl";

import { resolveAccessToken } from "@/shared/auth/resolve-access-token";
import { readAccessToken } from "@/shared/auth/session";
import { dispatchFileLibraryInvalidated } from "@/shared/events/file-library-events";
import { runBulkActionInChunks } from "@/shared/lib/bulk-action";
import { resolveConversationDefaultModel } from "@/shared/model/conversation-default-model";
import {
  batchSetConversationProject,
  createConversation,
  createConversationProject,
  deleteConversation,
  deleteConversationProject,
  listConversationProjects,
  listConversations,
  regenerateConversationTitle,
  renameConversation,
  reorderConversationProjects,
  setConversationProject,
  setConversationArchive,
  setConversationStar,
  updateConversationProject,
} from "@/shared/api/conversation";
import type {
  ConversationDTO,
  ConversationProjectDTO,
  CreateConversationProjectRequest,
  UpdateConversationProjectRequest,
} from "@/shared/api/conversation.types";

import type {
  DeleteConversationOptions,
  DeleteConversationProjectOptions,
  SidebarConversationChange,
  SidebarRecentsControllerValue,
} from "@/features/recent/types/sidebar-recents";
import {
  mergeUniqueByPublicID,
  removeByPublicID,
  sortByStarredAtDesc,
  sortByUpdatedAtDesc,
  upsertByPublicID,
} from "@/features/recent/utils/conversation-list";

const RECENT_PAGE_SIZE = 50;
const STARRED_VISIBLE_LIMIT = 5;
const STARRED_BUFFER_SIZE = 3;
const STARRED_WINDOW_SIZE = STARRED_VISIBLE_LIMIT + STARRED_BUFFER_SIZE;
const STARRED_DIALOG_PAGE_SIZE = 100;

type SidebarRecentsCache = {
  accessToken: string;
  recentItems: ConversationDTO[];
  starredItems: ConversationDTO[];
  projects: ConversationProjectDTO[];
  starredTotal: number;
  hasMore: boolean;
  page: number;
};

let sidebarRecentsCache: SidebarRecentsCache | null = null;

function readSidebarRecentsCache(): SidebarRecentsCache | null {
  const accessToken = readAccessToken();
  if (!accessToken || sidebarRecentsCache?.accessToken !== accessToken) {
    return null;
  }
  return sidebarRecentsCache;
}

function writeSidebarRecentsCache(next: Omit<SidebarRecentsCache, "accessToken">): void {
  const accessToken = readAccessToken();
  if (!accessToken) {
    return;
  }
  sidebarRecentsCache = {
    accessToken,
    ...next,
  };
}

function waitForNextFrame(): Promise<void> {
  return new Promise((resolve) => {
    if (typeof window === "undefined") {
      resolve();
      return;
    }
    window.requestAnimationFrame(() => resolve());
  });
}

function patchConversationList(
  items: ConversationDTO[],
  publicID: string,
  patch: Partial<ConversationDTO>,
  sortItems: (items: ConversationDTO[]) => ConversationDTO[],
): ConversationDTO[] {
  const current = items.find((item) => item.publicID === publicID);
  if (!current) {
    return items;
  }

  return sortItems(
    items.map((item) =>
      item.publicID === publicID
        ? {
            ...item,
            ...patch,
          }
        : item,
    ),
  );
}

function activeShareFieldsMissing(item: ConversationDTO): boolean {
  return (item.shareStatus ?? "").trim() === "none" && !item.shareID?.trim();
}

function preserveKnownShareState(
  current: ConversationDTO | null | undefined,
  incoming: ConversationDTO,
): ConversationDTO {
  if (
    !current ||
    !current.shareID?.trim() ||
    current.shareStatus !== "active" ||
    !activeShareFieldsMissing(incoming)
  ) {
    return incoming;
  }

  return {
    ...incoming,
    shareStatus: current.shareStatus,
    shareID: current.shareID,
    sharedAt: current.sharedAt,
    lastShareAccessedAt: current.lastShareAccessedAt,
  };
}

function orderProjectsByPublicIDs(
  items: ConversationProjectDTO[],
  projectIDs: string[],
): ConversationProjectDTO[] {
  if (projectIDs.length === 0) {
    return items;
  }

  const byPublicID = new Map(items.map((item) => [item.publicID, item]));
  const seen = new Set<string>();
  const ordered: ConversationProjectDTO[] = [];

  for (const publicID of projectIDs) {
    const project = byPublicID.get(publicID);
    if (!project || seen.has(publicID)) {
      continue;
    }
    seen.add(publicID);
    ordered.push({
      ...project,
      sortOrder: ordered.length + 1,
    });
  }

  for (const project of items) {
    if (seen.has(project.publicID)) {
      continue;
    }
    ordered.push({
      ...project,
      sortOrder: ordered.length + 1,
    });
  }

  return ordered;
}

async function fetchRecentPage(accessToken: string, page: number) {
  return listConversations(accessToken, {
    page,
    pageSize: RECENT_PAGE_SIZE,
    status: "active",
    starred: "unstarred",
  });
}

async function fetchStarredWindow(accessToken: string) {
  return listConversations(accessToken, {
    page: 1,
    pageSize: STARRED_WINDOW_SIZE,
    status: "active",
    starred: "starred",
  });
}

async function fetchAllStarred(accessToken: string): Promise<ConversationDTO[]> {
  let page = 1;
  let hasMore = true;
  let items: ConversationDTO[] = [];

  while (hasMore) {
    const data = await listConversations(accessToken, {
      page,
      pageSize: STARRED_DIALOG_PAGE_SIZE,
      status: "active",
      starred: "starred",
    });

    const pageItems = data.results ?? [];
    items = mergeUniqueByPublicID(items, pageItems, sortByStarredAtDesc);

    const loaded = pageItems.length;
    const total = data.total ?? 0;
    const mergedCount = (page - 1) * STARRED_DIALOG_PAGE_SIZE + loaded;
    hasMore = loaded === STARRED_DIALOG_PAGE_SIZE && mergedCount < total;
    page += 1;
  }

  return sortByStarredAtDesc(items);
}

async function fetchActiveProjects(accessToken: string): Promise<ConversationProjectDTO[]> {
  return listConversationProjects(accessToken, { status: "active" });
}

export function useRecentSidebarRecentsController(): SidebarRecentsControllerValue {
  const t = useTranslations("recent");
  const initialCache = React.useMemo(() => readSidebarRecentsCache(), []);
  const [recentItems, setRecentItems] = React.useState<ConversationDTO[]>(() => initialCache?.recentItems ?? []);
  const [starredItems, setStarredItems] = React.useState<ConversationDTO[]>(() => initialCache?.starredItems ?? []);
  const [projects, setProjects] = React.useState<ConversationProjectDTO[]>(() => initialCache?.projects ?? []);
  const [starredTotal, setStarredTotal] = React.useState(() => initialCache?.starredTotal ?? 0);
  const [loadingInitial, setLoadingInitial] = React.useState(() => !initialCache);
  const [loadingMore, setLoadingMore] = React.useState(false);
  const [hasMore, setHasMore] = React.useState(() => initialCache?.hasMore ?? true);
  const [loadMoreFailed, setLoadMoreFailed] = React.useState(false);
  const [transferringStarPublicID, setTransferringStarPublicID] = React.useState<string | null>(null);
  const [lastChange, setLastChange] = React.useState<SidebarConversationChange | null>(null);
  const pageRef = React.useRef(initialCache?.page ?? 1);
  const changeSequenceRef = React.useRef(0);
  const initialRequestVersionRef = React.useRef(0);
  const inFlightRef = React.useRef(false);
  const loadingMoreRef = React.useRef(false);
  const loadMoreFailedRef = React.useRef(false);
  const clearTransferTimerRef = React.useRef<ReturnType<typeof setTimeout> | null>(null);
  const recentItemsRef = React.useRef<ConversationDTO[]>([]);
  const starredItemsRef = React.useRef<ConversationDTO[]>([]);
  const projectsRef = React.useRef<ConversationProjectDTO[]>(initialCache?.projects ?? []);
  const starredTotalRef = React.useRef(0);
  const starredWindowRequestVersionRef = React.useRef(0);
  const hasHydratedInitialRef = React.useRef(Boolean(initialCache));

  React.useEffect(() => {
    loadingMoreRef.current = loadingMore;
  }, [loadingMore]);

  React.useEffect(() => {
    loadMoreFailedRef.current = loadMoreFailed;
  }, [loadMoreFailed]);

  React.useEffect(() => {
    recentItemsRef.current = recentItems;
  }, [recentItems]);

  React.useEffect(() => {
    starredItemsRef.current = starredItems;
  }, [starredItems]);

  React.useEffect(() => {
    projectsRef.current = projects;
  }, [projects]);

  React.useEffect(() => {
    starredTotalRef.current = starredTotal;
  }, [starredTotal]);

  const setProjectList = React.useCallback((updater: React.SetStateAction<ConversationProjectDTO[]>) => {
    setProjects((current) => {
      const next = typeof updater === "function"
        ? (updater as (value: ConversationProjectDTO[]) => ConversationProjectDTO[])(current)
        : updater;
      projectsRef.current = next;
      return next;
    });
  }, []);

  React.useEffect(() => {
    writeSidebarRecentsCache({
      recentItems,
      starredItems,
      projects,
      starredTotal,
      hasMore,
      page: pageRef.current,
    });
  }, [hasMore, projects, recentItems, starredItems, starredTotal]);

  const items = React.useMemo(
    () => mergeUniqueByPublicID(starredItems, recentItems, sortByUpdatedAtDesc),
    [recentItems, starredItems],
  );

  const publishChange = React.useCallback((change: Omit<SidebarConversationChange, "sequence">) => {
    changeSequenceRef.current += 1;
    setLastChange({
      sequence: changeSequenceRef.current,
      ...change,
    });
  }, []);

  const currentConversationSnapshot = React.useCallback((publicID: string) => {
    return (
      recentItemsRef.current.find((item) => item.publicID === publicID) ??
      starredItemsRef.current.find((item) => item.publicID === publicID) ??
      null
    );
  }, []);

  const applyConversationUpdate = React.useCallback((publicID: string, incoming: ConversationDTO) => {
    const updated = preserveKnownShareState(currentConversationSnapshot(publicID), incoming);
    if (updated.isStarred) {
      setRecentItems((prev) => removeByPublicID(prev, publicID));
      setStarredItems((prev) => upsertByPublicID(prev, updated, sortByStarredAtDesc));
    } else {
      setRecentItems((prev) => upsertByPublicID(prev, updated, sortByUpdatedAtDesc));
      setStarredItems((prev) => removeByPublicID(prev, publicID));
    }
    publishChange({ type: "upsert", publicID, item: updated });
    return updated;
  }, [currentConversationSnapshot, publishChange]);

  const refreshStarredWindow = React.useCallback(async (accessTokenOverride?: string) => {
    starredWindowRequestVersionRef.current += 1;
    const requestVersion = starredWindowRequestVersionRef.current;
    const token = accessTokenOverride ?? (await resolveAccessToken());
    if (!token) {
      if (requestVersion !== starredWindowRequestVersionRef.current) {
        return;
      }
      setStarredItems([]);
      setStarredTotal(0);
      return;
    }

    const data = await fetchStarredWindow(token);
    if (requestVersion !== starredWindowRequestVersionRef.current) {
      return;
    }

    const nextStarredItems = sortByStarredAtDesc(data.results ?? []);
    setStarredItems(nextStarredItems);
    setStarredTotal(data.total ?? nextStarredItems.length);
  }, []);

  const loadInitial = React.useCallback(async () => {
    initialRequestVersionRef.current += 1;
    const requestVersion = initialRequestVersionRef.current;

    const shouldShowInitialSkeleton =
      !hasHydratedInitialRef.current &&
      recentItemsRef.current.length === 0 &&
      starredItemsRef.current.length === 0;
    setLoadingInitial(shouldShowInitialSkeleton);
    setLoadMoreFailed(false);
    loadMoreFailedRef.current = false;
    pageRef.current = 1;

    const token = await resolveAccessToken();
    if (!token) {
      if (requestVersion !== initialRequestVersionRef.current) {
        return;
      }
      setRecentItems([]);
      setStarredItems([]);
      setProjectList([]);
      setStarredTotal(0);
      setHasMore(false);
      hasHydratedInitialRef.current = true;
      setLoadingInitial(false);
      return;
    }

    try {
      const [recentData, starredData, projectData] = await Promise.all([
        fetchRecentPage(token, 1),
        fetchStarredWindow(token),
        fetchActiveProjects(token),
      ]);

      if (requestVersion !== initialRequestVersionRef.current) {
        return;
      }

      const nextRecentItems = recentData.results ?? [];
      const loaded = nextRecentItems.length;
      const total = recentData.total ?? 0;
      const nextStarredItems = sortByStarredAtDesc(starredData.results ?? []);

      setRecentItems(sortByUpdatedAtDesc(nextRecentItems));
      setStarredItems(nextStarredItems);
      setProjectList(projectData);
      setStarredTotal(starredData.total ?? nextStarredItems.length);
      setHasMore(loaded === RECENT_PAGE_SIZE && loaded < total);
    } finally {
      if (requestVersion === initialRequestVersionRef.current) {
        hasHydratedInitialRef.current = true;
        setLoadingInitial(false);
      }
    }
  }, [setProjectList]);

  React.useEffect(() => {
    void loadInitial();
  }, [loadInitial]);

  React.useEffect(() => {
    return () => {
      if (clearTransferTimerRef.current !== null) {
        clearTimeout(clearTransferTimerRef.current);
        clearTransferTimerRef.current = null;
      }
    };
  }, []);

  const loadMore = React.useCallback(async () => {
    if (loadingInitial || loadingMoreRef.current || !hasMore || loadMoreFailedRef.current || inFlightRef.current) {
      return;
    }

    const token = await resolveAccessToken();
    if (!token) {
      setHasMore(false);
      return;
    }

    inFlightRef.current = true;
    loadingMoreRef.current = true;
    setLoadingMore(true);

    try {
      const nextPage = pageRef.current + 1;
      const data = await fetchRecentPage(token, nextPage);
      const nextRecentPageItems = data.results ?? [];
      const loaded = nextRecentPageItems.length;
      const total = data.total ?? 0;
      const mergedCount = (nextPage - 1) * RECENT_PAGE_SIZE + loaded;

      setRecentItems((current) => mergeUniqueByPublicID(current, nextRecentPageItems, sortByUpdatedAtDesc));
      setHasMore(loaded === RECENT_PAGE_SIZE && mergedCount < total);
      setLoadMoreFailed(false);
      loadMoreFailedRef.current = false;
      pageRef.current = nextPage;
    } catch {
      loadMoreFailedRef.current = true;
      setLoadMoreFailed(true);
    } finally {
      inFlightRef.current = false;
      loadingMoreRef.current = false;
      setLoadingMore(false);
    }
  }, [hasMore, loadingInitial]);

  const retryLoadMore = React.useCallback(async () => {
    loadMoreFailedRef.current = false;
    setLoadMoreFailed(false);
    await loadMore();
  }, [loadMore]);

  const prependNewConversation = React.useCallback(async (platformModelName?: string, projectID?: string): Promise<ConversationDTO | null> => {
    const token = await resolveAccessToken();
    if (!token) {
      return null;
    }
    const explicitModel = platformModelName?.trim() || "";
    const modelName = explicitModel || (await resolveConversationDefaultModel({ accessToken: token })).platformModelName;

    const item = await createConversation(token, {
      title: t("newChat"),
      model: modelName,
      projectID: projectID?.trim() || "",
    });
    setRecentItems((prev) => mergeUniqueByPublicID([item], prev, sortByUpdatedAtDesc));
    publishChange({ type: "upsert", publicID: item.publicID, item });
    return item;
  }, [publishChange, t]);

  const renameByPublicID = React.useCallback(
    async (publicID: string, title: string): Promise<ConversationDTO | null> => {
      const token = await resolveAccessToken();
      if (!token) {
        return null;
      }

      return applyConversationUpdate(publicID, await renameConversation(token, publicID, { title }));
    },
    [applyConversationUpdate],
  );

  const regenerateTitleByPublicID = React.useCallback(
    async (publicID: string): Promise<ConversationDTO | null> => {
      const token = await resolveAccessToken();
      if (!token) {
        return null;
      }

      return applyConversationUpdate(publicID, await regenerateConversationTitle(token, publicID));
    },
    [applyConversationUpdate],
  );

  const touchByPublicID = React.useCallback((publicID: string, patch: Partial<ConversationDTO>) => {
    setRecentItems((prev) => patchConversationList(prev, publicID, patch, sortByUpdatedAtDesc));
    setStarredItems((prev) => patchConversationList(prev, publicID, patch, sortByStarredAtDesc));
    publishChange({ type: "patch", publicID, patch });
  }, [publishChange]);

  const createProject = React.useCallback(async (payload: CreateConversationProjectRequest): Promise<ConversationProjectDTO | null> => {
    const token = await resolveAccessToken();
    if (!token) {
      return null;
    }
    const created = await createConversationProject(token, payload);
    setProjectList((current) => [...current, created].sort((a, b) => a.sortOrder - b.sortOrder || b.publicID.localeCompare(a.publicID)));
    return created;
  }, [setProjectList]);

  const updateProject = React.useCallback(
    async (projectID: string, payload: UpdateConversationProjectRequest): Promise<ConversationProjectDTO | null> => {
      const token = await resolveAccessToken();
      if (!token) {
        return null;
      }
      const updated = await updateConversationProject(token, projectID, payload);
      setProjectList((current) =>
        current
          .map((item) => (item.publicID === projectID ? updated : item))
          .filter((item) => item.status === "active")
          .sort((a, b) => a.sortOrder - b.sortOrder || b.publicID.localeCompare(a.publicID)),
      );
      if (updated.name.trim()) {
        const patch: Partial<ConversationDTO> = {
          projectID: updated.publicID,
          projectName: updated.name,
        };
        setRecentItems((current) => current.map((item) => (item.projectID === updated.publicID ? { ...item, ...patch } : item)));
        setStarredItems((current) => current.map((item) => (item.projectID === updated.publicID ? { ...item, ...patch } : item)));
      }
      return updated;
    },
    [setProjectList],
  );

  const deleteProject = React.useCallback(
    async (projectID: string, options: DeleteConversationProjectOptions = {}): Promise<boolean> => {
      const token = await resolveAccessToken();
      if (!token) {
        return false;
      }
      const deleteConversations = options.deleteConversations === true;
      const affectedItems = new Map<string, ConversationDTO>();
      for (const item of [...recentItemsRef.current, ...starredItemsRef.current]) {
        if (item.projectID === projectID) {
          affectedItems.set(item.publicID, item);
        }
      }

      const result = await deleteConversationProject(token, projectID, {
        deleteConversations,
        deleteFiles: deleteConversations && options.deleteFiles === true,
      });
      dispatchFileLibraryInvalidated({
        reason: "conversation_project_deleted",
        deletedFileCount: result.deletedFileCount ?? 0,
        quota: result.quota,
        sourceID: projectID,
      });
      setProjectList((current) => current.filter((item) => item.publicID !== projectID));
      if (deleteConversations) {
        setRecentItems((current) => current.filter((item) => item.projectID !== projectID));
        setStarredItems((current) => current.filter((item) => item.projectID !== projectID));
        setStarredTotal((current) => {
          const knownStarredCount = Array.from(affectedItems.values()).filter((item) => item.isStarred).length;
          return Math.max(0, current - knownStarredCount);
        });
        for (const item of affectedItems.values()) {
          publishChange({ type: "remove", publicID: item.publicID });
        }
        void refreshStarredWindow(token);
        return true;
      }

      const patch: Partial<ConversationDTO> = { projectID: "", projectName: "" };
      setRecentItems((current) => current.map((item) => (item.projectID === projectID ? { ...item, ...patch } : item)));
      setStarredItems((current) => current.map((item) => (item.projectID === projectID ? { ...item, ...patch } : item)));
      for (const item of affectedItems.values()) {
        publishChange({ type: "patch", publicID: item.publicID, patch });
      }
      return true;
    },
    [publishChange, refreshStarredWindow, setProjectList],
  );

  const reorderProjects = React.useCallback(async (projectIDs: string[]): Promise<void> => {
    const rollbackProjects = projectsRef.current;
    setProjectList(orderProjectsByPublicIDs(rollbackProjects, projectIDs));

    const token = await resolveAccessToken();
    if (!token) {
      setProjectList(rollbackProjects);
      throw new Error("errors.auth.unauthorized");
    }

    try {
      const reordered = await reorderConversationProjects(token, { projectIDs });
      setProjectList(reordered);
    } catch (error) {
      setProjectList(rollbackProjects);
      throw error;
    }
  }, [setProjectList]);

  const setProjectByPublicID = React.useCallback(
    async (publicID: string, projectID?: string): Promise<ConversationDTO | null> => {
      const token = await resolveAccessToken();
      if (!token) {
        return null;
      }
      const updated = preserveKnownShareState(
        currentConversationSnapshot(publicID),
        await setConversationProject(token, publicID, { projectID: projectID?.trim() || "" }),
      );
      if (updated.isStarred) {
        setStarredItems((prev) => upsertByPublicID(prev, updated, sortByStarredAtDesc));
        setRecentItems((prev) => removeByPublicID(prev, publicID));
      } else {
        setRecentItems((prev) => upsertByPublicID(prev, updated, sortByUpdatedAtDesc));
        setStarredItems((prev) => removeByPublicID(prev, publicID));
      }
      publishChange({ type: "upsert", publicID, item: updated });
      return updated;
    },
    [currentConversationSnapshot, publishChange],
  );

  const batchSetProjectByPublicIDs = React.useCallback(
    async (publicIDs: string[], projectID?: string): Promise<number> => {
      const token = await resolveAccessToken();
      if (!token || publicIDs.length === 0) {
        return 0;
      }
      const projectIDValue = projectID?.trim() || "";
      const results = await runBulkActionInChunks({
        items: publicIDs,
        title: t("labelMenu.bulk.pending"),
        runChunk: (conversationPublicIDs) => batchSetConversationProject(token, {
          conversationPublicIDs,
          projectID: projectIDValue,
        }),
      });
      const project = projects.find((item) => item.publicID === projectID);
      const patch: Partial<ConversationDTO> = {
        projectID: project?.publicID ?? "",
        projectName: project?.name ?? "",
      };
      setRecentItems((current) => current.map((item) => (publicIDs.includes(item.publicID) ? { ...item, ...patch } : item)));
      setStarredItems((current) => current.map((item) => (publicIDs.includes(item.publicID) ? { ...item, ...patch } : item)));
      for (const publicID of publicIDs) {
        publishChange({ type: "patch", publicID, patch });
      }
      return results.reduce((total, result) => total + result.updated, 0);
    },
    [projects, publishChange, t],
  );

  const setStarByPublicID = React.useCallback(
    async (publicID: string, starred: boolean): Promise<ConversationDTO | null> => {
      const rollbackRecentItems = recentItemsRef.current;
      const rollbackStarredItems = starredItemsRef.current;
      const rollbackStarredTotal = starredTotalRef.current;
      const targetItem =
        rollbackRecentItems.find((item) => item.publicID === publicID) ??
        rollbackStarredItems.find((item) => item.publicID === publicID) ??
        null;
      const optimisticStarredAt = new Date().toISOString();
      const wasStarred = Boolean(targetItem?.isStarred);

      if (clearTransferTimerRef.current !== null) {
        clearTimeout(clearTransferTimerRef.current);
        clearTransferTimerRef.current = null;
      }

      setTransferringStarPublicID(publicID);
      await waitForNextFrame();

      if (starred) {
        setRecentItems((prev) => removeByPublicID(prev, publicID));
        if (targetItem) {
          setStarredItems((prev) =>
            upsertByPublicID(
              prev,
              {
                ...targetItem,
                isStarred: true,
                starredAt: optimisticStarredAt,
              },
              sortByStarredAtDesc,
            ),
          );
        }
        if (!wasStarred) {
          setStarredTotal((prev) => prev + 1);
        }
      } else {
        setStarredItems((prev) => removeByPublicID(prev, publicID));
        if (targetItem) {
          setRecentItems((prev) =>
            upsertByPublicID(
              prev,
              {
                ...targetItem,
                isStarred: false,
                starredAt: null,
              },
              sortByUpdatedAtDesc,
            ),
          );
        }
        if (wasStarred) {
          setStarredTotal((prev) => Math.max(0, prev - 1));
        }
      }
      publishChange({
        type: "patch",
        publicID,
        patch: {
          isStarred: starred,
          starredAt: starred ? optimisticStarredAt : null,
        },
      });

      const token = await resolveAccessToken();
      if (!token) {
        setRecentItems(rollbackRecentItems);
        setStarredItems(rollbackStarredItems);
        setStarredTotal(rollbackStarredTotal);
        if (targetItem) {
          publishChange({ type: "upsert", publicID, item: targetItem });
        }
        clearTransferTimerRef.current = setTimeout(() => {
          setTransferringStarPublicID((current) => (current === publicID ? null : current));
          clearTransferTimerRef.current = null;
        }, 320);
        return null;
      }

      try {
        const updated = preserveKnownShareState(
          targetItem,
          await setConversationStar(token, publicID, { starred }),
        );

        if (starred) {
          setStarredItems((prev) => upsertByPublicID(prev, updated, sortByStarredAtDesc));
          setRecentItems((prev) => removeByPublicID(prev, publicID));
        } else {
          setStarredItems((prev) => removeByPublicID(prev, publicID));
          setRecentItems((prev) => upsertByPublicID(prev, updated, sortByUpdatedAtDesc));
        }
        publishChange({ type: "upsert", publicID, item: updated });
        void refreshStarredWindow(token);

        return updated;
      } catch (error) {
        setRecentItems(rollbackRecentItems);
        setStarredItems(rollbackStarredItems);
        setStarredTotal(rollbackStarredTotal);
        if (targetItem) {
          publishChange({ type: "upsert", publicID, item: targetItem });
        }
        throw error;
      } finally {
        clearTransferTimerRef.current = setTimeout(() => {
          setTransferringStarPublicID((current) => (current === publicID ? null : current));
          clearTransferTimerRef.current = null;
        }, 320);
      }
    },
    [publishChange, refreshStarredWindow],
  );

  const archiveByPublicID = React.useCallback(
    async (publicID: string, archived: boolean): Promise<ConversationDTO | null> => {
      const token = await resolveAccessToken();
      if (!token) {
        return null;
      }

      const updated = preserveKnownShareState(
        currentConversationSnapshot(publicID),
        await setConversationArchive(token, publicID, { archived }),
      );
      if (archived) {
        setRecentItems((prev) => removeByPublicID(prev, publicID));
        setStarredItems((prev) => removeByPublicID(prev, publicID));
        if (starredItemsRef.current.some((item) => item.publicID === publicID)) {
          setStarredTotal((prev) => Math.max(0, prev - 1));
        }
        void refreshStarredWindow(token);
      } else if (updated.isStarred) {
        setStarredItems((prev) => upsertByPublicID(prev, updated, sortByStarredAtDesc));
        void refreshStarredWindow(token);
      } else {
        setRecentItems((prev) => upsertByPublicID(prev, updated, sortByUpdatedAtDesc));
      }
      publishChange({ type: "upsert", publicID, item: updated });
      return updated;
    },
    [currentConversationSnapshot, publishChange, refreshStarredWindow],
  );

  const deleteByPublicID = React.useCallback(async (publicID: string, options: DeleteConversationOptions = {}): Promise<boolean> => {
    const token = await resolveAccessToken();
    if (!token) {
      return false;
    }

    const result = await deleteConversation(token, publicID, { deleteFiles: options.deleteFiles === true });
    dispatchFileLibraryInvalidated({
      reason: "conversation_deleted",
      deletedFileCount: result.deletedFileCount ?? 0,
      quota: result.quota,
      sourceID: publicID,
    });
    const removedStarred = starredItemsRef.current.some((item) => item.publicID === publicID);
    setRecentItems((prev) => removeByPublicID(prev, publicID));
    setStarredItems((prev) => removeByPublicID(prev, publicID));
    if (removedStarred) {
      setStarredTotal((prev) => Math.max(0, prev - 1));
    }
    publishChange({ type: "remove", publicID });
    void refreshStarredWindow(token);
    return true;
  }, [publishChange, refreshStarredWindow]);

  const loadAllStarred = React.useCallback(async (): Promise<ConversationDTO[]> => {
    const token = await resolveAccessToken();
    if (!token) {
      return [];
    }
    return fetchAllStarred(token);
  }, []);

  return React.useMemo(
    () => ({
      items,
      recentItems,
      starredItems,
      projects,
      starredTotal,
      loadingInitial,
      loadingMore,
      hasMore,
      loadMoreFailed,
      transferringStarPublicID,
      lastChange,
      loadMore,
      retryLoadMore,
      prependNewConversation,
      touchByPublicID,
      renameByPublicID,
      regenerateTitleByPublicID,
      createProject,
      updateProject,
      deleteProject,
      reorderProjects,
      setProjectByPublicID,
      batchSetProjectByPublicIDs,
      setStarByPublicID,
      loadAllStarred,
      archiveByPublicID,
      deleteByPublicID,
    }),
    [
      archiveByPublicID,
      batchSetProjectByPublicIDs,
      createProject,
      deleteByPublicID,
      deleteProject,
      hasMore,
      items,
      lastChange,
      loadAllStarred,
      loadMore,
      loadingInitial,
      loadingMore,
      loadMoreFailed,
      prependNewConversation,
      projects,
      regenerateTitleByPublicID,
      recentItems,
      reorderProjects,
      retryLoadMore,
      setProjectByPublicID,
      starredItems,
      starredTotal,
      touchByPublicID,
      updateProject,
      renameByPublicID,
      setStarByPublicID,
      transferringStarPublicID,
    ],
  );
}
