"use client";

import * as React from "react";

import type { SkillSummaryDTO } from "@/shared/api/skills.types";

const CHAT_COMPOSER_SELECTION_STORAGE_KEY = "deeix-chat:chat-composer-selection:v1";
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

function readSelectionStore(): PersistedComposerSelectionStore {
  if (typeof window === "undefined") {
    return {};
  }
  try {
    const raw = window.localStorage.getItem(CHAT_COMPOSER_SELECTION_STORAGE_KEY);
    if (!raw) {
      return {};
    }
    const parsed = JSON.parse(raw) as unknown;
    if (!parsed || typeof parsed !== "object" || Array.isArray(parsed)) {
      return {};
    }

    const nextStore: PersistedComposerSelectionStore = {};
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
      nextStore[key] = {
        ...selection,
        updatedAt: typeof entry.updatedAt === "string" ? entry.updatedAt : new Date(0).toISOString(),
      };
    }
    return nextStore;
  } catch {
    return {};
  }
}

function writeSelectionStore(store: PersistedComposerSelectionStore) {
  if (typeof window === "undefined") {
    return;
  }
  try {
    if (Object.keys(store).length === 0) {
      window.localStorage.removeItem(CHAT_COMPOSER_SELECTION_STORAGE_KEY);
      return;
    }
    window.localStorage.setItem(CHAT_COMPOSER_SELECTION_STORAGE_KEY, JSON.stringify(store));
  } catch {
    // Keep runtime selection usable when localStorage is unavailable.
  }
}

function readSelectionEntry(conversationKey: string): ComposerSelection {
  const entry = readSelectionStore()[conversationKey];
  return entry ? cloneSelection(entry) : emptySelection();
}

function writeSelectionEntry(conversationKey: string, selection: ComposerSelection) {
  const store = readSelectionStore();
  if (!hasSelection(selection)) {
    delete store[conversationKey];
  } else {
    store[conversationKey] = {
      ...cloneSelection(selection),
      updatedAt: new Date().toISOString(),
    };
  }
  writeSelectionStore(store);
}

function removeSelectionEntry(conversationKey: string) {
  const store = readSelectionStore();
  delete store[conversationKey];
  writeSelectionStore(store);
}

export function useChatComposerSelection({
  conversationKey,
  createdConversationID,
  resetToken = 0,
  hasConversation = false,
}: {
  conversationKey: string;
  createdConversationID: string | null;
  resetToken?: number;
  hasConversation?: boolean;
}) {
  const [selectedToolIDs, setSelectedToolIDs] = React.useState<number[]>([]);
  const [selectedSkills, setSelectedSkills] = React.useState<SkillSummaryDTO[]>([]);
  const [selectedKnowledgeBaseIDs, setSelectedKnowledgeBaseIDs] = React.useState<string[]>([]);
  const [hydratedKey, setHydratedKey] = React.useState<string | null>(null);
  const cacheRef = React.useRef(new Map<string, ComposerSelection>());
  const activeKeyRef = React.useRef(conversationKey);
  const selectedToolIDsRef = React.useRef(selectedToolIDs);
  const selectedSkillsRef = React.useRef(selectedSkills);
  const selectedKnowledgeBaseIDsRef = React.useRef(selectedKnowledgeBaseIDs);
  const resetTokenRef = React.useRef(resetToken);

  useIsomorphicLayoutEffect(() => {
    const previousKey = activeKeyRef.current;
    if (previousKey === conversationKey) {
      if (!cacheRef.current.has(conversationKey)) {
        const nextSelection = readSelectionEntry(conversationKey);
        cacheRef.current.set(conversationKey, cloneSelection(nextSelection));
        selectedToolIDsRef.current = nextSelection.selectedToolIDs;
        selectedSkillsRef.current = nextSelection.selectedSkills;
        selectedKnowledgeBaseIDsRef.current = nextSelection.selectedKnowledgeBaseIDs;
        setSelectedToolIDs(nextSelection.selectedToolIDs);
        setSelectedSkills(nextSelection.selectedSkills);
        setSelectedKnowledgeBaseIDs(nextSelection.selectedKnowledgeBaseIDs);
        setHydratedKey(conversationKey);
      }
      return;
    }

    const previousSelection: ComposerSelection = {
      selectedToolIDs: selectedToolIDsRef.current,
      selectedSkills: selectedSkillsRef.current,
      selectedKnowledgeBaseIDs: selectedKnowledgeBaseIDsRef.current,
    };
    cacheRef.current.set(previousKey, cloneSelection(previousSelection));
    writeSelectionEntry(previousKey, previousSelection);

    const createdKey = createdConversationID?.trim() || "";
    const shouldCarryNewConversationSelection =
      createdKey.length > 0 &&
      conversationKey === createdKey &&
      !cacheRef.current.has(conversationKey);
    const nextSelection = shouldCarryNewConversationSelection
      ? previousSelection
      : cacheRef.current.get(conversationKey) ?? readSelectionEntry(conversationKey);

    if (shouldCarryNewConversationSelection) {
      cacheRef.current.set(conversationKey, cloneSelection(nextSelection));
      writeSelectionEntry(conversationKey, nextSelection);
      cacheRef.current.delete(previousKey);
      removeSelectionEntry(previousKey);
    }

    activeKeyRef.current = conversationKey;
    selectedToolIDsRef.current = nextSelection.selectedToolIDs;
    selectedSkillsRef.current = nextSelection.selectedSkills;
    selectedKnowledgeBaseIDsRef.current = nextSelection.selectedKnowledgeBaseIDs;
    setSelectedToolIDs(nextSelection.selectedToolIDs);
    setSelectedSkills(nextSelection.selectedSkills);
    setSelectedKnowledgeBaseIDs(nextSelection.selectedKnowledgeBaseIDs);
    setHydratedKey(conversationKey);
  }, [conversationKey, createdConversationID]);

  useIsomorphicLayoutEffect(() => {
    if (resetTokenRef.current === resetToken) {
      return;
    }
    resetTokenRef.current = resetToken;
    if (hasConversation) {
      return;
    }

    const nextSelection = emptySelection();
    cacheRef.current.delete(conversationKey);
    removeSelectionEntry(conversationKey);
    activeKeyRef.current = conversationKey;
    selectedToolIDsRef.current = nextSelection.selectedToolIDs;
    selectedSkillsRef.current = nextSelection.selectedSkills;
    selectedKnowledgeBaseIDsRef.current = nextSelection.selectedKnowledgeBaseIDs;
    setSelectedToolIDs(nextSelection.selectedToolIDs);
    setSelectedSkills(nextSelection.selectedSkills);
    setSelectedKnowledgeBaseIDs(nextSelection.selectedKnowledgeBaseIDs);
    setHydratedKey(conversationKey);
  }, [conversationKey, hasConversation, resetToken]);

  React.useEffect(() => {
    if (hydratedKey !== activeKeyRef.current) {
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
    writeSelectionEntry(activeKeyRef.current, selection);
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
