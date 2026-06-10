import { listFiles } from "@/shared/api/file";
import type { FileObjectDTO } from "@/shared/api/file.types";

const MENTION_FILE_SEARCH_PAGE_SIZE = 8;
const MENTION_FILE_SEARCH_STALE_MS = 60_000;
const MENTION_FILE_SEARCH_MAX_CACHE_ENTRIES = 80;

type MentionFileSearchCacheKey = string;

type MentionFileSearchCacheEntry = {
  expiresAt: number;
  files: FileObjectDTO[];
};

type MentionFileSearchRequest = {
  accessToken: string;
  query: string;
  sessionRevision: number;
};

const cache = new Map<MentionFileSearchCacheKey, MentionFileSearchCacheEntry>();
const inFlight = new Map<MentionFileSearchCacheKey, Promise<FileObjectDTO[]>>();

function normalizeQuery(query: string): string {
  return query.trim().toLowerCase();
}

function cacheKey(sessionRevision: number, query: string): MentionFileSearchCacheKey {
  return `${sessionRevision}:${normalizeQuery(query)}`;
}

function pruneCache() {
  const now = Date.now();
  for (const [key, entry] of cache) {
    if (entry.expiresAt <= now) {
      cache.delete(key);
    }
  }
  while (cache.size > MENTION_FILE_SEARCH_MAX_CACHE_ENTRIES) {
    const oldestKey = cache.keys().next().value;
    if (!oldestKey) {
      break;
    }
    cache.delete(oldestKey);
  }
}

export function readMentionFileSearchCache(sessionRevision: number, query: string): FileObjectDTO[] | null {
  pruneCache();
  const key = cacheKey(sessionRevision, query);
  const entry = cache.get(key);
  if (!entry) {
    return null;
  }
  cache.delete(key);
  cache.set(key, entry);
  return entry.files;
}

export function clearMentionFileSearchCache() {
  cache.clear();
  inFlight.clear();
}

export async function searchMentionFiles({
  accessToken,
  query,
  sessionRevision,
}: MentionFileSearchRequest): Promise<FileObjectDTO[]> {
  const key = cacheKey(sessionRevision, query);
  const cached = readMentionFileSearchCache(sessionRevision, query);
  if (cached) {
    return cached;
  }

  const existingRequest = inFlight.get(key);
  if (existingRequest) {
    return existingRequest;
  }

  const request = listFiles(accessToken, {
    page: 1,
    pageSize: MENTION_FILE_SEARCH_PAGE_SIZE,
    query,
    sort: "last_used",
  })
    .then((data) => {
      const files = data.results ?? [];
      cache.set(key, {
        expiresAt: Date.now() + MENTION_FILE_SEARCH_STALE_MS,
        files,
      });
      pruneCache();
      return files;
    })
    .finally(() => {
      inFlight.delete(key);
    });

  inFlight.set(key, request);
  return request;
}
