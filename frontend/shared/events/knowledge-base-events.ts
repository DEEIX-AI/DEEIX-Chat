"use client";

export type KnowledgeBaseInvalidatedDetail = {
  knowledgeBaseID?: string;
  timestamp: number;
  nonce: string;
};

const storageKey = "deeix-chat:knowledge-base-invalidated:payload";

function isInvalidationDetail(value: unknown): value is KnowledgeBaseInvalidatedDetail {
  if (!value || typeof value !== "object") {
    return false;
  }
  const detail = value as Partial<KnowledgeBaseInvalidatedDetail>;
  return (
    (detail.knowledgeBaseID === undefined || typeof detail.knowledgeBaseID === "string") &&
    typeof detail.timestamp === "number" &&
    typeof detail.nonce === "string"
  );
}

export function dispatchKnowledgeBaseInvalidated(knowledgeBaseID?: string): void {
  if (typeof window === "undefined") {
    return;
  }
  const detail: KnowledgeBaseInvalidatedDetail = {
    knowledgeBaseID: knowledgeBaseID?.trim() || undefined,
    timestamp: Date.now(),
    nonce: `${Date.now()}-${Math.random().toString(36).slice(2)}`,
  };
  try {
    window.localStorage.setItem(storageKey, JSON.stringify(detail));
  } catch {
    // localStorage can be unavailable in private browsing or strict environments.
  }
}

export function subscribeKnowledgeBaseInvalidated(
  handler: (detail: KnowledgeBaseInvalidatedDetail) => void,
): () => void {
  if (typeof window === "undefined") {
    return () => {};
  }

  const handleStorage = (event: StorageEvent) => {
    if (event.key !== storageKey || !event.newValue) {
      return;
    }
    try {
      const detail = JSON.parse(event.newValue) as unknown;
      if (isInvalidationDetail(detail)) {
        handler(detail);
      }
    } catch {
      // Ignore malformed payloads from external scripts or older builds.
    }
  };

  window.addEventListener("storage", handleStorage);
  return () => {
    window.removeEventListener("storage", handleStorage);
  };
}
