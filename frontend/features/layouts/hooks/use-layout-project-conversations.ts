"use client";

import * as React from "react";

import {
  removeByPublicID,
  sortByUpdatedAtDesc,
  upsertByPublicID,
} from "@/entities/conversation";
import type { SidebarConversationChange } from "@/entities/conversation";
import { listConversations } from "@/shared/api/conversation";
import type { ConversationDTO, ConversationProjectDTO } from "@/shared/api/conversation.types";
import { resolveAccessToken } from "@/shared/auth/resolve-access-token";

export type ProjectConversationState = {
  items: ConversationDTO[];
  loading: boolean;
  loaded: boolean;
  error: boolean;
};

type ProjectConversationStateMap = Record<string, ProjectConversationState>;

const PROJECT_CONVERSATION_PAGE_SIZE = 30;
const PROJECT_EXPANDED_IDS_STORAGE_KEY = "deeix.sidebar.projects.expanded";

function readStoredProjectIDSet(): Set<string> {
  if (typeof window === "undefined") {
    return new Set();
  }

  try {
    const parsed = JSON.parse(window.localStorage.getItem(PROJECT_EXPANDED_IDS_STORAGE_KEY) ?? "[]") as unknown;
    return Array.isArray(parsed)
      ? new Set(parsed.filter((item): item is string => typeof item === "string" && item.trim().length > 0))
      : new Set();
  } catch {
    return new Set();
  }
}

function hasStoredProjectIDSet(): boolean {
  if (typeof window === "undefined") {
    return false;
  }

  try {
    return window.localStorage.getItem(PROJECT_EXPANDED_IDS_STORAGE_KEY) !== null;
  } catch {
    return false;
  }
}

function writeStoredProjectIDSet(value: Set<string>): void {
  try {
    window.localStorage.setItem(PROJECT_EXPANDED_IDS_STORAGE_KEY, JSON.stringify(Array.from(value)));
  } catch {
    // Sidebar persistence is optional when browser storage is unavailable.
  }
}

export function useLayoutProjectConversations({
  activeProjectID,
  items,
  lastChange,
  projects,
}: {
  activeProjectID: string;
  items: ConversationDTO[];
  lastChange: SidebarConversationChange | null;
  projects: ConversationProjectDTO[];
}) {
  const [expandedProjectIDs, setExpandedProjectIDs] = React.useState<Set<string>>(readStoredProjectIDSet);
  const [projectConversationState, setProjectConversationState] = React.useState<ProjectConversationStateMap>({});
  const projectConversationStateRef = React.useRef(projectConversationState);
  const projectConversationRequestVersionRef = React.useRef<Record<string, number>>({});
  const mountedRef = React.useRef(false);
  const expandedProjectIDsRef = React.useRef(expandedProjectIDs);
  const activeRevealedProjectIDsRef = React.useRef(new Set<string>());
  const hasStoredExpandedProjectIDsRef = React.useRef(hasStoredProjectIDSet());

  React.useEffect(() => {
    mountedRef.current = true;
    return () => {
      mountedRef.current = false;
    };
  }, []);

  const updateProjectConversationState = React.useCallback(
    (updater: (previous: ProjectConversationStateMap) => ProjectConversationStateMap) => {
      const next = updater(projectConversationStateRef.current);
      projectConversationStateRef.current = next;
      setProjectConversationState(next);
    },
    [],
  );

  const updateExpandedProjectIDs = React.useCallback(
    (updater: (previous: Set<string>) => Set<string>, persist = false) => {
      const next = updater(expandedProjectIDsRef.current);
      expandedProjectIDsRef.current = next;
      setExpandedProjectIDs(next);
      if (persist) {
        hasStoredExpandedProjectIDsRef.current = true;
        writeStoredProjectIDSet(
          new Set(Array.from(next).filter((projectID) => !activeRevealedProjectIDsRef.current.has(projectID))),
        );
      }
    },
    [],
  );

  const loadProjectConversations = React.useCallback(
    async (projectID: string, force = false) => {
      const current = projectConversationStateRef.current[projectID];
      if (!force && (current?.loading || current?.loaded)) {
        return;
      }

      const requestVersion = (projectConversationRequestVersionRef.current[projectID] ?? 0) + 1;
      projectConversationRequestVersionRef.current[projectID] = requestVersion;

      updateProjectConversationState((previous) => ({
        ...previous,
        [projectID]: {
          items: previous[projectID]?.items ?? [],
          loading: true,
          loaded: previous[projectID]?.loaded ?? false,
          error: false,
        },
      }));

      try {
        const token = await resolveAccessToken();
        if (!mountedRef.current || projectConversationRequestVersionRef.current[projectID] !== requestVersion) {
          return;
        }
        if (!token) {
          updateProjectConversationState((previous) => ({
            ...previous,
            [projectID]: {
              items: previous[projectID]?.items ?? [],
              loading: false,
              loaded: false,
              error: true,
            },
          }));
          return;
        }

        const data = await listConversations(token, {
          page: 1,
          pageSize: PROJECT_CONVERSATION_PAGE_SIZE,
          status: "active",
          starred: "all",
          project: projectID,
        });
        if (!mountedRef.current || projectConversationRequestVersionRef.current[projectID] !== requestVersion) {
          return;
        }
        updateProjectConversationState((previous) => ({
          ...previous,
          [projectID]: {
            items: sortByUpdatedAtDesc(data.results ?? []),
            loading: false,
            loaded: true,
            error: false,
          },
        }));
      } catch {
        if (!mountedRef.current || projectConversationRequestVersionRef.current[projectID] !== requestVersion) {
          return;
        }
        updateProjectConversationState((previous) => ({
          ...previous,
          [projectID]: {
            items: previous[projectID]?.items ?? [],
            loading: false,
            loaded: false,
            error: true,
          },
        }));
      }
    },
    [updateProjectConversationState],
  );

  React.useEffect(() => {
    const visibleProjectIDs = new Set(projects.map((project) => project.publicID));
    expandedProjectIDs.forEach((projectID) => {
      if (visibleProjectIDs.has(projectID)) {
        void loadProjectConversations(projectID);
      }
    });
  }, [expandedProjectIDs, loadProjectConversations, projects]);

  const ensureProjectExpanded = React.useCallback(
    (projectID: string, persist = false) => {
      const shouldLoad = !projectConversationStateRef.current[projectID]?.loaded;
      if (persist) {
        activeRevealedProjectIDsRef.current.delete(projectID);
      }
      updateExpandedProjectIDs((previous) => {
        if (previous.has(projectID)) {
          return previous;
        }
        const next = new Set(previous);
        next.add(projectID);
        return next;
      }, persist);
      if (shouldLoad) {
        void loadProjectConversations(projectID);
      }
    },
    [loadProjectConversations, updateExpandedProjectIDs],
  );

  const toggleProjectExpanded = React.useCallback(
    (projectID: string) => {
      const shouldLoad = !projectConversationStateRef.current[projectID]?.loaded;
      const expandedNext = !expandedProjectIDsRef.current.has(projectID);
      activeRevealedProjectIDsRef.current.delete(projectID);
      updateExpandedProjectIDs((previous) => {
        const next = new Set(previous);
        if (next.has(projectID)) {
          next.delete(projectID);
        } else {
          next.add(projectID);
        }
        return next;
      }, true);
      if (expandedNext && shouldLoad) {
        void loadProjectConversations(projectID);
      }
    },
    [loadProjectConversations, updateExpandedProjectIDs],
  );

  React.useEffect(() => {
    if (!activeProjectID || hasStoredExpandedProjectIDsRef.current || activeRevealedProjectIDsRef.current.has(activeProjectID)) {
      return;
    }
    activeRevealedProjectIDsRef.current.add(activeProjectID);
    ensureProjectExpanded(activeProjectID, false);
  }, [activeProjectID, ensureProjectExpanded]);

  React.useEffect(() => {
    if (!lastChange) {
      return;
    }

    updateProjectConversationState((previous) => {
      const projectIDs = Object.keys(previous);
      if (projectIDs.length === 0) {
        return previous;
      }

      let changed = false;
      const next = { ...previous };

      for (const projectID of projectIDs) {
        const state = previous[projectID];
        if (!state?.loaded) {
          continue;
        }

        if (lastChange.type === "remove") {
          const nextItems = removeByPublicID(state.items, lastChange.publicID);
          if (nextItems.length !== state.items.length) {
            next[projectID] = { ...state, items: nextItems };
            changed = true;
          }
          continue;
        }

        const existing = state.items.find((item) => item.publicID === lastChange.publicID);
        const base =
          lastChange.item ??
          (existing
            ? { ...existing, ...(lastChange.patch ?? {}) }
            : items.find((item) => item.publicID === lastChange.publicID));

        if (!base) {
          continue;
        }

        const updated = lastChange.patch ? { ...base, ...lastChange.patch } : base;
        const belongsToProject = updated.projectID === projectID && updated.status !== "archived";
        if (belongsToProject) {
          next[projectID] = { ...state, items: upsertByPublicID(state.items, updated, sortByUpdatedAtDesc) };
          changed = true;
        } else if (existing) {
          next[projectID] = { ...state, items: removeByPublicID(state.items, updated.publicID) };
          changed = true;
        }
      }

      return changed ? next : previous;
    });
  }, [items, lastChange, updateProjectConversationState]);

  const removeProject = React.useCallback(
    (projectID: string) => {
      projectConversationRequestVersionRef.current[projectID] =
        (projectConversationRequestVersionRef.current[projectID] ?? 0) + 1;
      activeRevealedProjectIDsRef.current.delete(projectID);
      updateExpandedProjectIDs((previous) => {
        const next = new Set(previous);
        next.delete(projectID);
        return next;
      }, true);
      updateProjectConversationState((previous) => {
        if (!(projectID in previous)) {
          return previous;
        }
        const next = { ...previous };
        delete next[projectID];
        return next;
      });
    },
    [updateExpandedProjectIDs, updateProjectConversationState],
  );

  return {
    ensureProjectExpanded,
    expandedProjectIDs,
    loadProjectConversations,
    projectConversationState,
    removeProject,
    toggleProjectExpanded,
  };
}
