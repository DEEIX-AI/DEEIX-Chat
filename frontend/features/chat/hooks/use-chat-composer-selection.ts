"use client";

import * as React from "react";

import type { SkillSummaryDTO } from "@/shared/api/skills.types";

const LEGACY_CHAT_COMPOSER_SELECTION_STORAGE_KEY = "deeix-chat:chat-composer-selection:v1";
const CHAT_COMPOSER_SELECTION_STORAGE_KEY_PREFIX = "deeix-chat:chat-composer-selection:v2:";
const COMPOSER_SELECTION_MAX_AGE_MS = 30 * 24 * 60 * 60 * 1000;
const COMPOSER_SELECTION_MAX_ENTRIES = 50;
const useIsomorphicLayoutEffect = typeof window === "undefined" ? React.useEffect : React.useLayoutEffect;

type ComposerSelection = {
  selectedToolIDs: number[];
  selectedSkills: SkillSummaryDTO[];
  selectedKnowledgeBaseIDs: string[];
};

type PersistedComposerSelection = ComposerSelection & {
  updatedAt: string;
};

type PersistedComposerSelectionStore = Record<string, PersistedComposerSelection>;

function emptySelection(): ComposerSelection {
  return {
    selectedToolIDs: [],
    selectedSkills: [],
    selectedKnowledgeBaseIDs: [],
  };
}

function cloneSelection(selection: ComposerSelection): ComposerSelection {
  return {
    selectedToolIDs: selection.selectedToolIDs.slice(),
    selectedSkills: selection.selectedSkills.slice(),
    selectedKnowledgeBaseIDs: selection.selectedKnowledgeBaseIDs.slice(),
  };
}

function hasSelection(selection: ComposerSelection): boolean {
  return selection.selectedToolIDs.length > 0 || selection.selectedSkills.length > 0 || selection.selectedKnowledgeBaseIDs.length > 0;
}

function isSkillSummary(value: unknown): value is SkillSummaryDTO {
  if (!value || typeof value !== "object") {
    return false;
  }
  const item = value as Record<string, unknown>;
  return (
    typeof item.id === "number" &&
    (item.scope === "builtin" || item.scope === "user") &&
    typeof item.title === "string" &&
    typeof item.trigger === "string" &&
    typeof item.description === "string" &&
    typeof item.enabled === "boolean" &&
    typeof item.sortOrder === "number" &&
    typeof item.createdAt === "string" &&
    typeof item.updatedAt === "string"
  );
}

function normalizeToolIDs(value: unknown): number[] {
  if (!Array.isArray(value)) {
    return [];
  }
  return Array.from(
    new Set(
      value.filter((item): item is number => Number.isInteger(item) && item > 0),
    ),
  );
}

function normalizeSkills(value: unknown): SkillSummaryDTO[] {
  if (!Array.isArray(value)) {
    return [];
  }
  const seen = new Set<number>();
  const result: SkillSummaryDTO[] = [];
  for (const item of value) {
    if (!isSkillSummary(item) || seen.has(item.id)) {
      continue;
    }
    seen.add(item.id);
    result.push(item);
  }
  return result;
}

function normalizeKnowledgeBaseIDs(value: unknown): string[] {
  if (!Array.isArray(value)) return [];
  return Array.from(new Set(value.filter((item): item is string => typeof item === "string" && item.trim().length > 0)))
    .map((item) => item.trim())
    .slice(0, 8);
}

function resolveSelectionStorageKey(storageScope: string): string {
  const normalizedScope = storageScope.trim();
  return normalizedScope
    ? `${CHAT_COMPOSER_SELECTION_STORAGE_KEY_PREFIX}${encodeURIComponent(normalizedScope)}`
    : "";
}

function readSelectionStore(storageKey: string): PersistedComposerSelectionStore {
  if (typeof window === "undefined" || !storageKey) {
    return {};
  }
  try {
    window.localStorage.removeItem(LEGACY_CHAT_COMPOSER_SELECTION_STORAGE_KEY);
    const raw = window.localStorage.getItem(storageKey);
    if (!raw) {
      return {};
    }
    const parsed = JSON.parse(raw) as unknown;
    if (!parsed || typeof parsed !== "object" || Array.isArray(parsed)) {
      return {};
    }

    const entries: Array<[string, PersistedComposerSelection, number]> = [];
    const expiresBefore = Date.now() - COMPOSER_SELECTION_MAX_AGE_MS;
    for (const [key, value] of Object.entries(parsed as Record<string, unknown>)) {
      if (!value || typeof value !== "object" || Array.isArray(value)) {
        continue;
      }
      const entry = value as Record<string, unknown>;
      const selection = {
        selectedToolIDs: normalizeToolIDs(entry.selectedToolIDs),
        selectedSkills: normalizeSkills(entry.selectedSkills),
        selectedKnowledgeBaseIDs: normalizeKnowledgeBaseIDs(entry.selectedKnowledgeBaseIDs),
      };
      if (!hasSelection(selection)) {
        continue;
      }
      const updatedAt = typeof entry.updatedAt === "string" ? entry.updatedAt : "";
      const updatedAtMs = Date.parse(updatedAt);
      if (!Number.isFinite(updatedAtMs) || updatedAtMs < expiresBefore) {
        continue;
      }
      entries.push([key, {
        ...selection,
        updatedAt,
      }, updatedAtMs]);
    }
    entries.sort((left, right) => right[2] - left[2]);
    return Object.fromEntries(
      entries.slice(0, COMPOSER_SELECTION_MAX_ENTRIES).map(([key, entry]) => [key, entry]),
    );
  } catch {
    return {};
  }
}

function writeSelectionStore(storageKey: string, store: PersistedComposerSelectionStore) {
  if (typeof window === "undefined" || !storageKey) {
    return;
  }
  try {
    const expiresBefore = Date.now() - COMPOSER_SELECTION_MAX_AGE_MS;
    const boundedStore = Object.fromEntries(
      Object.entries(store)
        .map(([key, entry]) => [key, entry, Date.parse(entry.updatedAt)] as const)
        .filter(([, , updatedAt]) => Number.isFinite(updatedAt) && updatedAt >= expiresBefore)
        .sort((left, right) => right[2] - left[2])
        .slice(0, COMPOSER_SELECTION_MAX_ENTRIES)
        .map(([key, entry]) => [key, entry]),
    );
    if (Object.keys(boundedStore).length === 0) {
      window.localStorage.removeItem(storageKey);
      return;
    }
    window.localStorage.setItem(storageKey, JSON.stringify(boundedStore));
  } catch {
    // Keep runtime selection usable when localStorage is unavailable.
  }
}

function readSelectionEntry(storageKey: string, conversationKey: string): ComposerSelection {
  const entry = readSelectionStore(storageKey)[conversationKey];
  return entry ? cloneSelection(entry) : emptySelection();
}

function writeSelectionEntry(storageKey: string, conversationKey: string, selection: ComposerSelection) {
  const store = readSelectionStore(storageKey);
  if (!hasSelection(selection)) {
    delete store[conversationKey];
  } else {
    store[conversationKey] = {
      ...cloneSelection(selection),
      updatedAt: new Date().toISOString(),
    };
  }
  writeSelectionStore(storageKey, store);
}

function removeSelectionEntry(storageKey: string, conversationKey: string) {
  const store = readSelectionStore(storageKey);
  delete store[conversationKey];
  writeSelectionStore(storageKey, store);
}

export function useChatComposerSelection({
  conversationKey,
  createdConversationID,
  resetToken = 0,
  hasConversation = false,
  storageScope = "",
}: {
  conversationKey: string;
  createdConversationID: string | null;
  resetToken?: number;
  hasConversation?: boolean;
  storageScope?: string;
}) {
  const storageKey = React.useMemo(() => resolveSelectionStorageKey(storageScope), [storageScope]);
  const [selectedToolIDs, setSelectedToolIDs] = React.useState<number[]>([]);
  const [selectedSkills, setSelectedSkills] = React.useState<SkillSummaryDTO[]>([]);
  const [selectedKnowledgeBaseIDs, setSelectedKnowledgeBaseIDs] = React.useState<string[]>([]);
  const [hydratedKey, setHydratedKey] = React.useState<string | null>(null);
  const cacheRef = React.useRef(new Map<string, ComposerSelection>());
  const activeKeyRef = React.useRef(conversationKey);
  const activeStorageKeyRef = React.useRef(storageKey);
  const selectedToolIDsRef = React.useRef(selectedToolIDs);
  const selectedSkillsRef = React.useRef(selectedSkills);
  const selectedKnowledgeBaseIDsRef = React.useRef(selectedKnowledgeBaseIDs);
  const resetTokenRef = React.useRef(resetToken);

  useIsomorphicLayoutEffect(() => {
    const previousStorageKey = activeStorageKeyRef.current;
    const previousKey = activeKeyRef.current;
    if (previousStorageKey === storageKey && previousKey === conversationKey) {
      if (!cacheRef.current.has(conversationKey)) {
        const nextSelection = readSelectionEntry(storageKey, conversationKey);
        cacheRef.current.set(conversationKey, cloneSelection(nextSelection));
        selectedToolIDsRef.current = nextSelection.selectedToolIDs;
        selectedSkillsRef.current = nextSelection.selectedSkills;
        selectedKnowledgeBaseIDsRef.current = nextSelection.selectedKnowledgeBaseIDs;
        setSelectedToolIDs(nextSelection.selectedToolIDs);
        setSelectedSkills(nextSelection.selectedSkills);
        setSelectedKnowledgeBaseIDs(nextSelection.selectedKnowledgeBaseIDs);
        setHydratedKey(`${storageKey}\u0000${conversationKey}`);
      }
      return;
    }

    const previousSelection: ComposerSelection = {
      selectedToolIDs: selectedToolIDsRef.current,
      selectedSkills: selectedSkillsRef.current,
      selectedKnowledgeBaseIDs: selectedKnowledgeBaseIDsRef.current,
    };
    if (previousStorageKey) {
      cacheRef.current.set(previousKey, cloneSelection(previousSelection));
      writeSelectionEntry(previousStorageKey, previousKey, previousSelection);
    }
    if (previousStorageKey !== storageKey) {
      cacheRef.current.clear();
    }

    const createdKey = createdConversationID?.trim() || "";
    const shouldCarryNewConversationSelection =
      previousStorageKey === storageKey &&
      createdKey.length > 0 &&
      conversationKey === createdKey &&
      !cacheRef.current.has(conversationKey);
    const nextSelection = shouldCarryNewConversationSelection
      ? previousSelection
      : cacheRef.current.get(conversationKey) ?? readSelectionEntry(storageKey, conversationKey);

    if (shouldCarryNewConversationSelection && storageKey) {
      cacheRef.current.set(conversationKey, cloneSelection(nextSelection));
      writeSelectionEntry(storageKey, conversationKey, nextSelection);
      cacheRef.current.delete(previousKey);
      removeSelectionEntry(storageKey, previousKey);
    }

    activeStorageKeyRef.current = storageKey;
    activeKeyRef.current = conversationKey;
    selectedToolIDsRef.current = nextSelection.selectedToolIDs;
    selectedSkillsRef.current = nextSelection.selectedSkills;
    selectedKnowledgeBaseIDsRef.current = nextSelection.selectedKnowledgeBaseIDs;
    setSelectedToolIDs(nextSelection.selectedToolIDs);
    setSelectedSkills(nextSelection.selectedSkills);
    setSelectedKnowledgeBaseIDs(nextSelection.selectedKnowledgeBaseIDs);
    setHydratedKey(`${storageKey}\u0000${conversationKey}`);
  }, [conversationKey, createdConversationID, storageKey]);

  useIsomorphicLayoutEffect(() => {
    if (resetTokenRef.current === resetToken || hasConversation) {
      return;
    }
    resetTokenRef.current = resetToken;

    const nextSelection = emptySelection();
    cacheRef.current.delete(conversationKey);
    removeSelectionEntry(storageKey, conversationKey);
    activeKeyRef.current = conversationKey;
    selectedToolIDsRef.current = nextSelection.selectedToolIDs;
    selectedSkillsRef.current = nextSelection.selectedSkills;
    selectedKnowledgeBaseIDsRef.current = nextSelection.selectedKnowledgeBaseIDs;
    setSelectedToolIDs(nextSelection.selectedToolIDs);
    setSelectedSkills(nextSelection.selectedSkills);
    setSelectedKnowledgeBaseIDs(nextSelection.selectedKnowledgeBaseIDs);
    setHydratedKey(`${storageKey}\u0000${conversationKey}`);
  }, [conversationKey, hasConversation, resetToken, storageKey]);

  React.useEffect(() => {
    if (hydratedKey !== `${activeStorageKeyRef.current}\u0000${activeKeyRef.current}`) {
      return;
    }
    const selection: ComposerSelection = {
      selectedToolIDs,
      selectedSkills,
      selectedKnowledgeBaseIDs,
    };
    selectedToolIDsRef.current = selectedToolIDs;
    selectedSkillsRef.current = selectedSkills;
    selectedKnowledgeBaseIDsRef.current = selectedKnowledgeBaseIDs;
    cacheRef.current.set(activeKeyRef.current, cloneSelection(selection));
    writeSelectionEntry(activeStorageKeyRef.current, activeKeyRef.current, selection);
  }, [hydratedKey, selectedKnowledgeBaseIDs, selectedToolIDs, selectedSkills]);

  return {
    selectedToolIDs,
    selectedSkills,
    selectedKnowledgeBaseIDs,
    setSelectedToolIDs,
    setSelectedSkills,
    setSelectedKnowledgeBaseIDs,
  };
}
