"use client";

import type { UserStorageQuotaDTO } from "@/shared/api/file.types";

export type FileLibraryInvalidationReason =
  | "conversation_deleted"
  | "conversation_project_deleted";

export type FileLibraryInvalidatedDetail = {
  reason: FileLibraryInvalidationReason;
  deletedFileCount: number;
  quota?: UserStorageQuotaDTO;
  sourceID?: string;
  timestamp: number;
  nonce: string;
};

const FILE_LIBRARY_INVALIDATED_EVENT = "deeix-chat:file-library-invalidated";
const FILE_LIBRARY_INVALIDATED_STORAGE_KEY = "deeix-chat:file-library-invalidated:payload";
const FILE_LIBRARY_BROADCAST_CHANNEL = "deeix-chat:file-library";

type FileLibraryInvalidatedInput = {
  reason: FileLibraryInvalidationReason;
  deletedFileCount?: number;
  quota?: UserStorageQuotaDTO;
  sourceID?: string;
};

type FileLibraryInvalidatedHandler = (detail: FileLibraryInvalidatedDetail) => void;

function createInvalidatedDetail(input: FileLibraryInvalidatedInput): FileLibraryInvalidatedDetail {
  return {
    reason: input.reason,
    deletedFileCount: Math.max(0, input.deletedFileCount ?? 0),
    quota: input.quota,
    sourceID: input.sourceID?.trim() || undefined,
    timestamp: Date.now(),
    nonce: `${Date.now()}-${Math.random().toString(36).slice(2)}`,
  };
}

function isFileLibraryInvalidatedDetail(value: unknown): value is FileLibraryInvalidatedDetail {
  if (!value || typeof value !== "object") {
    return false;
  }
  const detail = value as Partial<FileLibraryInvalidatedDetail>;
  return (
    (detail.reason === "conversation_deleted" || detail.reason === "conversation_project_deleted") &&
    typeof detail.deletedFileCount === "number" &&
    typeof detail.timestamp === "number" &&
    typeof detail.nonce === "string"
  );
}

function notifyCurrentTab(detail: FileLibraryInvalidatedDetail): void {
  window.dispatchEvent(new CustomEvent<FileLibraryInvalidatedDetail>(FILE_LIBRARY_INVALIDATED_EVENT, { detail }));
}

function notifyOtherTabs(detail: FileLibraryInvalidatedDetail): void {
  try {
    const channel = new BroadcastChannel(FILE_LIBRARY_BROADCAST_CHANNEL);
    channel.postMessage(detail);
    channel.close();
  } catch {
    // BroadcastChannel is not available in every browser context.
  }

  try {
    window.localStorage.setItem(FILE_LIBRARY_INVALIDATED_STORAGE_KEY, JSON.stringify(detail));
  } catch {
    // localStorage can be unavailable in private browsing or strict environments.
  }
}

export function dispatchFileLibraryInvalidated(input: FileLibraryInvalidatedInput): void {
  if (typeof window === "undefined") {
    return;
  }
  const detail = createInvalidatedDetail(input);
  if (detail.deletedFileCount <= 0) {
    return;
  }
  notifyCurrentTab(detail);
  notifyOtherTabs(detail);
}

export function subscribeFileLibraryInvalidated(handler: FileLibraryInvalidatedHandler): () => void {
  if (typeof window === "undefined") {
    return () => {};
  }

  const seenNonces = new Set<string>();
  const emitOnce = (detail: FileLibraryInvalidatedDetail) => {
    if (seenNonces.has(detail.nonce)) {
      return;
    }
    seenNonces.add(detail.nonce);
    if (seenNonces.size > 50) {
      seenNonces.clear();
      seenNonces.add(detail.nonce);
    }
    handler(detail);
  };

  const handleWindowEvent = (event: Event) => {
    const detail = (event as CustomEvent<unknown>).detail;
    if (isFileLibraryInvalidatedDetail(detail)) {
      emitOnce(detail);
    }
  };

  const handleStorageEvent = (event: StorageEvent) => {
    if (event.key !== FILE_LIBRARY_INVALIDATED_STORAGE_KEY || !event.newValue) {
      return;
    }
    try {
      const parsed = JSON.parse(event.newValue) as unknown;
      if (isFileLibraryInvalidatedDetail(parsed)) {
        emitOnce(parsed);
      }
    } catch {
      // Ignore malformed storage payloads from external scripts or older builds.
    }
  };

  let channel: BroadcastChannel | null = null;
  try {
    channel = new BroadcastChannel(FILE_LIBRARY_BROADCAST_CHANNEL);
    channel.onmessage = (event: MessageEvent<unknown>) => {
      if (isFileLibraryInvalidatedDetail(event.data)) {
        emitOnce(event.data);
      }
    };
  } catch {
    channel = null;
  }

  window.addEventListener(FILE_LIBRARY_INVALIDATED_EVENT, handleWindowEvent);
  window.addEventListener("storage", handleStorageEvent);

  return () => {
    window.removeEventListener(FILE_LIBRARY_INVALIDATED_EVENT, handleWindowEvent);
    window.removeEventListener("storage", handleStorageEvent);
    if (channel) {
      channel.close();
    }
  };
}
