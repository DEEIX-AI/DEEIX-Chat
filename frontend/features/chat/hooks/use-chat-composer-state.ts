"use client";

import * as React from "react";

import type { PendingAttachment } from "@/features/chat/types/chat-runtime";

const CHAT_COMPOSER_STORAGE_KEY = "deeix-chat:chat-composer:v1";
const NEW_CONVERSATION_COMPOSER_KEY = "__new__";

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
  return items.map((item) => ({
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

function readComposerStore(): PersistedComposerStore {
  if (typeof window === "undefined") {
    return {};
  }

  try {
    const raw = window.localStorage.getItem(CHAT_COMPOSER_STORAGE_KEY);
    if (!raw) {
      return {};
    }

    const parsed = JSON.parse(raw) as unknown;
    if (!parsed || typeof parsed !== "object") {
      return {};
    }

    const nextStore: PersistedComposerStore = {};
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
      nextStore[key] = {
        draft,
        attachments,
        updatedAt,
      };
    }
    return nextStore;
  } catch {
    return {};
  }
}

function writeComposerStore(store: PersistedComposerStore) {
  if (typeof window === "undefined") {
    return;
  }

  try {
    if (Object.keys(store).length === 0) {
      window.localStorage.removeItem(CHAT_COMPOSER_STORAGE_KEY);
      return;
    }
    window.localStorage.setItem(CHAT_COMPOSER_STORAGE_KEY, JSON.stringify(store));
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

// ComposerStorageOps centralizes localStorage access and avoids repeated readComposerStore() calls.
const ComposerStorageOps = {
  readEntry(conversationKey: string): ComposerState {
    const entry = readComposerStore()[conversationKey];
    return {
      conversationKey,
      draft: entry?.draft ?? "",
      attachments: restoreAttachments(entry?.attachments ?? []),
    };
  },

  writeEntry(conversationKey: string, draft: string, attachments: PendingAttachment[]) {
    const store = readComposerStore();
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
    writeComposerStore(store);
  },

  removeEntry(conversationKey: string) {
    const store = readComposerStore();
    delete store[conversationKey];
    writeComposerStore(store);
  },

  appendAttachments(conversationKey: string, items: PendingAttachment[]) {
    if (items.length === 0) {
      return;
    }
    const store = readComposerStore();
    const existing = store[conversationKey];
    const attachments = mergeAttachmentsByFileID(existing?.attachments ?? [], sanitizeAttachments(items));
    store[conversationKey] = {
      draft: existing?.draft ?? "",
      attachments,
      updatedAt: new Date().toISOString(),
    };
    writeComposerStore(store);
  },
};

export function resolveConversationComposerKey(conversationID: string | null): string {
  return conversationID?.trim() || NEW_CONVERSATION_COMPOSER_KEY;
}

export function useChatComposerState(
  conversationID: string | null,
  { preserveDrafts = true, resetToken = 0 }: { preserveDrafts?: boolean; resetToken?: number } = {},
) {
  const conversationKey = React.useMemo(() => resolveConversationComposerKey(conversationID), [conversationID]);
  const [state, setState] = React.useState<ComposerState>(() => createEmptyComposerState(conversationKey));
  const [hydratedConversationKey, setHydratedConversationKey] = React.useState<string | null>(null);

  React.useEffect(() => {
    if (resetToken <= 0 || conversationID) {
      return;
    }
    ComposerStorageOps.removeEntry(conversationKey);
    setHydratedConversationKey(conversationKey);
    setState(createEmptyComposerState(conversationKey));
  }, [conversationID, conversationKey, resetToken]);

  useIsomorphicLayoutEffect(() => {
    if (!preserveDrafts) {
      ComposerStorageOps.removeEntry(conversationKey);
      setState((prev) => (prev.conversationKey === conversationKey ? prev : createEmptyComposerState(conversationKey)));
      setHydratedConversationKey(conversationKey);
      return;
    }

    const nextState = ComposerStorageOps.readEntry(conversationKey);
    setState((prev) => {
      const nextHasContent = hasComposerContent(nextState);
      const prevMatchesConversation = prev.conversationKey === conversationKey;
      const prevHasContent = prevMatchesConversation && hasComposerContent(prev);

      if (!nextHasContent && !prevHasContent) {
        return prevMatchesConversation ? prev : createEmptyComposerState(conversationKey);
      }

      if (
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
  }, [conversationKey, preserveDrafts]);

  React.useEffect(() => {
    if (hydratedConversationKey !== state.conversationKey) {
      return;
    }
    if (!preserveDrafts) {
      ComposerStorageOps.removeEntry(state.conversationKey);
      return;
    }
    ComposerStorageOps.writeEntry(state.conversationKey, state.draft, state.attachments);
  }, [hydratedConversationKey, preserveDrafts, state]);

  const visibleState = state.conversationKey === conversationKey ? state : createEmptyComposerState(conversationKey);

  const setDraft = React.useCallback((value: React.SetStateAction<string>) => {
    setHydratedConversationKey(conversationKey);
    setState((prev) => ({
      ...(prev.conversationKey === conversationKey ? prev : createEmptyComposerState(conversationKey)),
      draft:
        typeof value === "function"
          ? value(prev.conversationKey === conversationKey ? prev.draft : "")
          : value,
    }));
  }, [conversationKey]);

  const setAttachments = React.useCallback((value: React.SetStateAction<PendingAttachment[]>) => {
    setHydratedConversationKey(conversationKey);
    setState((prev) => ({
      ...(prev.conversationKey === conversationKey ? prev : createEmptyComposerState(conversationKey)),
      attachments:
        typeof value === "function"
          ? value(prev.conversationKey === conversationKey ? prev.attachments : [])
          : value,
    }));
  }, [conversationKey]);

  const appendAttachmentsForKey = React.useCallback((targetConversationKey: string, items: PendingAttachment[]) => {
    if (items.length === 0) {
      return;
    }

    if (conversationKey === targetConversationKey) {
      setHydratedConversationKey(targetConversationKey);
      setState((prev) => ({
        ...(prev.conversationKey === targetConversationKey ? prev : createEmptyComposerState(targetConversationKey)),
        attachments: mergeAttachmentsByFileID(
          prev.conversationKey === targetConversationKey ? prev.attachments : [],
          items,
        ),
      }));
      return;
    }

    if (!preserveDrafts) {
      return;
    }

    ComposerStorageOps.appendAttachments(targetConversationKey, items);
  }, [conversationKey, preserveDrafts]);

  return {
    conversationKey: visibleState.conversationKey,
    draft: visibleState.draft,
    attachments: visibleState.attachments,
    setDraft,
    setAttachments,
    appendAttachmentsForKey,
  };
}
