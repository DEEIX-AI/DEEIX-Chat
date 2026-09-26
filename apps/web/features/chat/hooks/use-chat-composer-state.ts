"use client";

import * as React from "react";

import type { PendingAttachment } from "@/features/chat/types/chat-runtime";

const LEGACY_CHAT_COMPOSER_STORAGE_KEY = "deeix-chat:chat-composer:v1";
const CHAT_COMPOSER_STORAGE_KEY_PREFIX = "deeix-chat:chat-composer:v2:";
const NEW_CONVERSATION_COMPOSER_KEY = "__new__";
const TRANSIENT_COMPOSER_KEY = "__transient__";
const COMPOSER_ENTRY_MAX_AGE_MS = 30 * 24 * 60 * 60 * 1000;
const COMPOSER_STORE_MAX_ENTRIES = 50;
const COMPOSER_WRITE_DEBOUNCE_MS = 250;

type PersistedAttachment = Pick<
  PendingAttachment,
  | "fileID"
  | "fileName"
  | "mimeType"
  | "sizeBytes"
  | "detectedMime"
  | "fileCategory"
  | "processingStatus"
  | "processingReady"
  | "processingErrorCode"
  | "processingErrorMessage"
  | "extractStatus"
  | "embedStatus"
  | "ragReady"
  | "ragReason"
  | "ocrUsed"
>;

type PersistedComposerEntry = {
  draft: string;
  attachments: PersistedAttachment[];
  updatedAt: string;
};

type PersistedComposerStore = Record<string, PersistedComposerEntry>;

type ComposerState = {
  conversationKey: string;
  draft: string;
  attachments: PendingAttachment[];
};

type PendingComposerPersistence = Pick<ComposerState, "conversationKey" | "draft" | "attachments"> & {
  storageKey: string;
};

const composerStoreCache = new Map<string, PersistedComposerStore>();

const useIsomorphicLayoutEffect = typeof window === "undefined" ? React.useEffect : React.useLayoutEffect;

function sanitizeAttachments(items: PendingAttachment[]): PersistedAttachment[] {
  return items.map((item) => ({
    fileID: item.fileID,
    fileName: item.fileName,
    mimeType: item.mimeType,
    sizeBytes: item.sizeBytes,
    detectedMime: item.detectedMime,
    fileCategory: item.fileCategory,
    processingStatus: item.processingStatus,
    processingReady: item.processingReady,
    processingErrorCode: item.processingErrorCode,
    processingErrorMessage: item.processingErrorMessage,
    extractStatus: item.extractStatus,
    embedStatus: item.embedStatus,
    ragReady: item.ragReady,
    ragReason: item.ragReason,
    ocrUsed: item.ocrUsed,
  }));
}

function mergeAttachmentsByFileID<T extends Pick<PendingAttachment, "fileID">>(current: T[], incoming: T[]): T[] {
  if (incoming.length === 0) {
    return current;
  }
  const seen = new Set(current.map((item) => item.fileID));
  const next = [...current];
  for (const item of incoming) {
    if (seen.has(item.fileID)) {
      continue;
    }
    seen.add(item.fileID);
    next.push(item);
  }
  return next;
}

function restoreAttachments(items: PersistedAttachment[]): PendingAttachment[] {
  return items.map((item): PendingAttachment => ({
    ...item,
    previewURL: undefined,
  }));
}

function isPersistedAttachment(value: unknown): value is PersistedAttachment {
  if (!value || typeof value !== "object") {
    return false;
  }

  const item = value as Record<string, unknown>;
  return (
      typeof item.fileID === "string" &&
      typeof item.fileName === "string" &&
      typeof item.mimeType === "string" &&
      typeof item.sizeBytes === "number"
  );
}

function resolveComposerStorageKey(storageScope: string): string {
  const normalizedScope = storageScope.trim();
  return normalizedScope ? `${CHAT_COMPOSER_STORAGE_KEY_PREFIX}${encodeURIComponent(normalizedScope)}` : "";
}

function readComposerStore(storageKey: string): PersistedComposerStore {
  if (typeof window === "undefined" || !storageKey) {
    return {};
  }

  const cached = composerStoreCache.get(storageKey);
  if (cached) {
    return cached;
  }

  try {
    window.localStorage.removeItem(LEGACY_CHAT_COMPOSER_STORAGE_KEY);
    const raw = window.localStorage.getItem(storageKey);
    if (!raw) {
      const emptyStore = {};
      composerStoreCache.set(storageKey, emptyStore);
      return emptyStore;
    }

    const parsed = JSON.parse(raw) as unknown;
    if (!parsed || typeof parsed !== "object") {
      const emptyStore = {};
      composerStoreCache.set(storageKey, emptyStore);
      return emptyStore;
    }

    const entries: Array<[string, PersistedComposerEntry, number]> = [];
    const expiresBefore = Date.now() - COMPOSER_ENTRY_MAX_AGE_MS;
    for (const [key, value] of Object.entries(parsed as Record<string, unknown>)) {
      if (!value || typeof value !== "object") {
        continue;
      }
      const entry = value as Record<string, unknown>;
      const draft = typeof entry.draft === "string" ? entry.draft : "";
      const attachments = Array.isArray(entry.attachments)
        ? entry.attachments.filter(isPersistedAttachment)
        : [];
      const updatedAt = typeof entry.updatedAt === "string" ? entry.updatedAt : new Date(0).toISOString();
      const updatedAtMs = Date.parse(updatedAt);
      if (!Number.isFinite(updatedAtMs) || updatedAtMs < expiresBefore) {
        continue;
      }
      entries.push([key, {
        draft,
        attachments,
        updatedAt,
      }, updatedAtMs]);
    }
    entries.sort((left, right) => right[2] - left[2]);
    const nextStore = Object.fromEntries(
      entries.slice(0, COMPOSER_STORE_MAX_ENTRIES).map(([key, entry]) => [key, entry]),
    );
    composerStoreCache.set(storageKey, nextStore);
    return nextStore;
  } catch {
    const emptyStore = {};
    composerStoreCache.set(storageKey, emptyStore);
    return emptyStore;
  }
}

function writeComposerStore(storageKey: string, store: PersistedComposerStore) {
  if (typeof window === "undefined" || !storageKey) {
    return;
  }

  const expiresBefore = Date.now() - COMPOSER_ENTRY_MAX_AGE_MS;
  const boundedStore = Object.fromEntries(
    Object.entries(store)
      .map(([key, entry]) => [key, entry, Date.parse(entry.updatedAt)] as const)
      .filter(([, , updatedAt]) => Number.isFinite(updatedAt) && updatedAt >= expiresBefore)
      .sort((left, right) => right[2] - left[2])
      .slice(0, COMPOSER_STORE_MAX_ENTRIES)
      .map(([key, entry]) => [key, entry]),
  );
  composerStoreCache.set(storageKey, boundedStore);
  try {
    if (Object.keys(boundedStore).length === 0) {
      window.localStorage.removeItem(storageKey);
      return;
    }
    window.localStorage.setItem(storageKey, JSON.stringify(boundedStore));
  } catch {
    // Ignore storage quota / serialization issues and keep runtime state usable.
  }
}

function createEmptyComposerState(conversationKey: string): ComposerState {
  return {
    conversationKey,
    draft: "",
    attachments: [],
  };
}

function hasComposerContent(state: Pick<ComposerState, "draft" | "attachments">): boolean {
  return state.draft.trim().length > 0 || state.attachments.length > 0;
}

const ComposerStorageOps = {
  readEntry(storageKey: string, conversationKey: string): ComposerState {
    const entry = readComposerStore(storageKey)[conversationKey];
    return {
      conversationKey,
      draft: entry?.draft ?? "",
      attachments: restoreAttachments(entry?.attachments ?? []),
    };
  },

  writeEntry(storageKey: string, conversationKey: string, draft: string, attachments: PendingAttachment[]) {
    const store = readComposerStore(storageKey);
    const normalizedAttachments = sanitizeAttachments(attachments);

    if (!draft.trim() && normalizedAttachments.length === 0) {
      delete store[conversationKey];
    } else {
      store[conversationKey] = {
        draft,
        attachments: normalizedAttachments,
        updatedAt: new Date().toISOString(),
      };
    }
    writeComposerStore(storageKey, store);
  },

  removeEntry(storageKey: string, conversationKey: string) {
    const store = readComposerStore(storageKey);
    delete store[conversationKey];
    writeComposerStore(storageKey, store);
  },

  appendAttachments(storageKey: string, conversationKey: string, items: PendingAttachment[]) {
    if (items.length === 0) {
      return;
    }
    const store = readComposerStore(storageKey);
    const existing = store[conversationKey];
    const attachments = mergeAttachmentsByFileID(existing?.attachments ?? [], sanitizeAttachments(items));
    store[conversationKey] = {
      draft: existing?.draft ?? "",
      attachments,
      updatedAt: new Date().toISOString(),
    };
    writeComposerStore(storageKey, store);
  },
};

export function resolveConversationComposerKey(conversationID: string | null): string {
  return conversationID?.trim() || NEW_CONVERSATION_COMPOSER_KEY;
}

// 新对话草稿与历史会话草稿同属一个 store（key 为 __new__），"新对话"按钮只切换会话 key，
// 不删除已持久化的条目；切回新对话时由 hydration 从 storage 恢复，提交时由 setDraft("") 写空清除。
export function useChatComposerState(
  conversationID: string | null,
  {
    preserveDrafts = true,
    storageScope = "",
    transient = false,
  }: {
    preserveDrafts?: boolean;
    storageScope?: string;
    transient?: boolean;
  } = {},
) {
  const conversationKey = React.useMemo(
    () => transient ? TRANSIENT_COMPOSER_KEY : resolveConversationComposerKey(conversationID),
    [conversationID, transient],
  );
  const storageKey = React.useMemo(() => resolveComposerStorageKey(storageScope), [storageScope]);
  const persistenceEnabled = preserveDrafts && !transient && Boolean(storageKey);
  const [state, setState] = React.useState<ComposerState>(() => createEmptyComposerState(conversationKey));
  const [hydratedConversationKey, setHydratedConversationKey] = React.useState<string | null>(null);
  const [hydratedStorageKey, setHydratedStorageKey] = React.useState<string | null>(null);
  const activeStorageKeyRef = React.useRef(storageKey);
  const persistenceTimerRef = React.useRef<number | null>(null);
  const pendingPersistenceRef = React.useRef<PendingComposerPersistence | null>(null);

  const flushPendingPersistence = React.useCallback(() => {
    if (persistenceTimerRef.current !== null) {
      window.clearTimeout(persistenceTimerRef.current);
      persistenceTimerRef.current = null;
    }
    const pending = pendingPersistenceRef.current;
    pendingPersistenceRef.current = null;
    if (!pending) {
      return;
    }
    ComposerStorageOps.writeEntry(
      pending.storageKey,
      pending.conversationKey,
      pending.draft,
      pending.attachments,
    );
  }, []);

  const discardPendingPersistence = React.useCallback((targetConversationKey: string) => {
    if (pendingPersistenceRef.current?.conversationKey !== targetConversationKey) {
      return;
    }
    pendingPersistenceRef.current = null;
    if (persistenceTimerRef.current !== null) {
      window.clearTimeout(persistenceTimerRef.current);
      persistenceTimerRef.current = null;
    }
  }, []);

  React.useEffect(
    () => () => flushPendingPersistence(),
    [conversationKey, flushPendingPersistence, storageKey],
  );

  React.useEffect(() => {
    const handleStorage = (event: StorageEvent) => {
      if (event.key?.startsWith(CHAT_COMPOSER_STORAGE_KEY_PREFIX)) {
        composerStoreCache.delete(event.key);
      }
    };
    window.addEventListener("storage", handleStorage);
    return () => window.removeEventListener("storage", handleStorage);
  }, []);

  useIsomorphicLayoutEffect(() => {
    const previousStorageKey = activeStorageKeyRef.current;
    const storageAccountChanged = Boolean(previousStorageKey) && previousStorageKey !== storageKey;
    activeStorageKeyRef.current = storageKey;
    if (!persistenceEnabled) {
      discardPendingPersistence(conversationKey);
      if (!transient && storageKey && !preserveDrafts) {
        ComposerStorageOps.removeEntry(storageKey, conversationKey);
      }
      setState((prev) => (prev.conversationKey === conversationKey ? prev : createEmptyComposerState(conversationKey)));
      setHydratedConversationKey(conversationKey);
      setHydratedStorageKey(storageKey);
      return;
    }

    const nextState = ComposerStorageOps.readEntry(storageKey, conversationKey);
    setState((prev) => {
      const nextHasContent = hasComposerContent(nextState);
      const prevMatchesConversation = prev.conversationKey === conversationKey;
      const prevHasContent = !storageAccountChanged && prevMatchesConversation && hasComposerContent(prev);

      if (!nextHasContent && !prevHasContent) {
        return prevMatchesConversation && !storageAccountChanged
          ? prev
          : createEmptyComposerState(conversationKey);
      }

      if (
        !storageAccountChanged &&
        prevMatchesConversation &&
        prev.draft === nextState.draft &&
        prev.attachments.length === nextState.attachments.length &&
        prev.attachments.every(
          (item, index) =>
            item.fileID === nextState.attachments[index]?.fileID &&
            item.fileName === nextState.attachments[index]?.fileName &&
            item.mimeType === nextState.attachments[index]?.mimeType &&
            item.sizeBytes === nextState.attachments[index]?.sizeBytes &&
            item.processingStatus === nextState.attachments[index]?.processingStatus &&
            item.processingReady === nextState.attachments[index]?.processingReady,
        )
      ) {
        return prev;
      }

      return nextHasContent ? nextState : createEmptyComposerState(conversationKey);
    });
    setHydratedConversationKey(conversationKey);
    setHydratedStorageKey(storageKey);
  }, [conversationKey, discardPendingPersistence, persistenceEnabled, preserveDrafts, storageKey, transient]);

  React.useEffect(() => {
    if (
      hydratedConversationKey !== state.conversationKey ||
      hydratedStorageKey !== storageKey ||
      state.conversationKey !== conversationKey
    ) {
      return;
    }
    if (!persistenceEnabled) {
      return;
    }
    pendingPersistenceRef.current = {
      conversationKey: state.conversationKey,
      draft: state.draft,
      attachments: state.attachments,
      storageKey,
    };
    if (persistenceTimerRef.current !== null) {
      window.clearTimeout(persistenceTimerRef.current);
    }
    persistenceTimerRef.current = window.setTimeout(
      flushPendingPersistence,
      COMPOSER_WRITE_DEBOUNCE_MS,
    );
  }, [
    conversationKey,
    flushPendingPersistence,
    hydratedConversationKey,
    hydratedStorageKey,
    persistenceEnabled,
    state,
    storageKey,
  ]);

  const visibleState = state.conversationKey === conversationKey ? state : createEmptyComposerState(conversationKey);

  const setDraft = React.useCallback((value: React.SetStateAction<string>) => {
    setHydratedConversationKey(conversationKey);
    setHydratedStorageKey(storageKey);
    setState((prev) => ({
      ...(prev.conversationKey === conversationKey ? prev : createEmptyComposerState(conversationKey)),
      draft:
        typeof value === "function"
          ? value(prev.conversationKey === conversationKey ? prev.draft : "")
          : value,
    }));
  }, [conversationKey, storageKey]);

  const setAttachments = React.useCallback((value: React.SetStateAction<PendingAttachment[]>) => {
    setHydratedConversationKey(conversationKey);
    setHydratedStorageKey(storageKey);
    setState((prev) => ({
      ...(prev.conversationKey === conversationKey ? prev : createEmptyComposerState(conversationKey)),
      attachments:
        typeof value === "function"
          ? value(prev.conversationKey === conversationKey ? prev.attachments : [])
          : value,
    }));
  }, [conversationKey, storageKey]);

  const appendAttachmentsForKey = React.useCallback((targetConversationKey: string, items: PendingAttachment[]) => {
    if (items.length === 0) {
      return;
    }

    if (conversationKey === targetConversationKey) {
      setHydratedConversationKey(targetConversationKey);
      setHydratedStorageKey(storageKey);
      setState((prev) => ({
        ...(prev.conversationKey === targetConversationKey ? prev : createEmptyComposerState(targetConversationKey)),
        attachments: mergeAttachmentsByFileID(
          prev.conversationKey === targetConversationKey ? prev.attachments : [],
          items,
        ),
      }));
      return;
    }

    if (!persistenceEnabled) {
      return;
    }

    ComposerStorageOps.appendAttachments(storageKey, targetConversationKey, items);
  }, [conversationKey, persistenceEnabled, storageKey]);

  return {
    conversationKey: visibleState.conversationKey,
    draft: visibleState.draft,
    attachments: visibleState.attachments,
    setDraft,
    setAttachments,
    appendAttachmentsForKey,
  };
}
