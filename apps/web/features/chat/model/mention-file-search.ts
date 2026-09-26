import { listFiles } from "@/shared/api/file";
import type { FileObjectDTO } from "@/shared/api/file.types";

type MentionFileSearchCacheKey = string;

export type MentionFileSearchPage = {
  files: FileObjectDTO[];
  total: number;
};

type MentionFileSearchCacheEntry = {
  expiresAt: number;
  page: MentionFileSearchPage;
};

type MentionFileSearchRequest = {
  accessToken: string;
  query: string;
  page: number;
  sessionRevision: number;
  signal?: AbortSignal;
};

const cache = new Map<MentionFileSearchCacheKey, MentionFileSearchCacheEntry>();

function normalizeQuery(query: string): string {
  return query.trim().toLowerCase();
}

function cacheKey(sessionRevision: number, query: string, page: number): MentionFileSearchCacheKey {
  return `${sessionRevision}:${page}:${normalizeQuery(query)}`;
}

function pruneCache() {
  const now = Date.now();
  for (const [key, entry] of cache) {
    if (entry.expiresAt <= now) {
      cache.delete(key);
    }
  }
  while (cache.size > 80) {
    const oldestKey = cache.keys().next().value;
    if (!oldestKey) {
      break;
    }
    cache.delete(oldestKey);
  }
}

export function readMentionFileSearchCache(
  sessionRevision: number,
  query: string,
  page: number,
): MentionFileSearchPage | null {
  pruneCache();
  const key = cacheKey(sessionRevision, query, page);
  const entry = cache.get(key);
  if (!entry) {
    return null;
  }
  cache.delete(key);
  cache.set(key, entry);
  return entry.page;
}

export function clearMentionFileSearchCache() {
  cache.clear();
}

export async function searchMentionFiles({
  accessToken,
  query,
  page,
  sessionRevision,
  signal,
}: MentionFileSearchRequest): Promise<MentionFileSearchPage> {
  const key = cacheKey(sessionRevision, query, page);
  const cached = readMentionFileSearchCache(sessionRevision, query, page);
  if (cached) {
    return cached;
  }

  const data = await listFiles(accessToken, {
    page,
    pageSize: 20,
    query,
    sort: "last_used",
  }, signal);
  const result: MentionFileSearchPage = {
    files: data.results ?? [],
    total: data.total ?? 0,
  };
  cache.set(key, {
    expiresAt: Date.now() + 60_000,
    page: result,
  });
  pruneCache();
  return result;
}
