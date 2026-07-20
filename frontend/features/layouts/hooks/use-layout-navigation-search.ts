"use client";

import * as React from "react";
import { useRouter } from "next/navigation";

import { useConversationSearchPreview } from "@/features/layouts/hooks/use-conversation-search-preview";
import { toConversationSearchResult } from "@/features/layouts/model/navigation-search";
import { hasPlatformModifierKey } from "@/shared/lib/platform-shortcuts";
import { normalizeConversationSearchText } from "@/shared/lib/conversation-search";
import { searchConversations } from "@/shared/api/conversation";
import { resolveAccessToken } from "@/shared/auth/resolve-access-token";
import type { ConversationSearchResult } from "@/features/layouts/types/navigation";

type UseLayoutNavigationSearchOptions = {
  untitled: string;
};

const NAVIGATION_SEARCH_DEBOUNCE_MS = 250;
const NAVIGATION_SEARCH_PAGE_SIZE = 20;

function mergeSearchResults(
  current: ConversationSearchResult[],
  next: ConversationSearchResult[],
): ConversationSearchResult[] {
  const seen = new Set(current.map((item) => item.publicID));
  return [...current, ...next.filter((item) => !seen.has(item.publicID))];
}

export function useLayoutNavigationSearch({ untitled }: UseLayoutNavigationSearchOptions) {
  const router = useRouter();
  const [open, setOpen] = React.useState(false);
  const [query, setQuery] = React.useState("");
  const [results, setResults] = React.useState<ConversationSearchResult[]>([]);
  const [page, setPage] = React.useState(1);
  const [hasMore, setHasMore] = React.useState(false);
  const [loading, setLoading] = React.useState(false);
  const [loadingMore, setLoadingMore] = React.useState(false);
  const [loadFailed, setLoadFailed] = React.useState(false);
  const [loadMoreFailed, setLoadMoreFailed] = React.useState(false);
  const [refreshRevision, setRefreshRevision] = React.useState(0);
  const requestVersionRef = React.useRef(0);
  const loadingMoreRef = React.useRef(false);
  const loadMoreAbortRef = React.useRef<AbortController | null>(null);
  const conversationPreview = useConversationSearchPreview(open);

  React.useEffect(() => {
    if (!open) {
      setQuery("");
      setResults([]);
      setPage(1);
      setHasMore(false);
      setLoading(false);
      setLoadingMore(false);
      setLoadFailed(false);
      setLoadMoreFailed(false);
      loadingMoreRef.current = false;
      loadMoreAbortRef.current?.abort();
      loadMoreAbortRef.current = null;
      requestVersionRef.current += 1;
    }
  }, [open]);

  const normalizedQuery = normalizeConversationSearchText(query);

  React.useEffect(() => {
    if (!open) {
      return;
    }

    requestVersionRef.current += 1;
    const requestVersion = requestVersionRef.current;
    setResults([]);
    setPage(1);
    setHasMore(false);
    setLoading(true);
    setLoadingMore(false);
    setLoadFailed(false);
    setLoadMoreFailed(false);
    loadingMoreRef.current = false;
    loadMoreAbortRef.current?.abort();
    loadMoreAbortRef.current = null;
    const abortController = new AbortController();
    const timer = window.setTimeout(() => {
      void (async () => {
        try {
          const token = await resolveAccessToken();
          if (requestVersion !== requestVersionRef.current) {
            return;
          }
          if (!token) {
            setLoadFailed(true);
            return;
          }
          const data = await searchConversations(token, {
            page: 1,
            pageSize: NAVIGATION_SEARCH_PAGE_SIZE,
            query: normalizedQuery,
            signal: abortController.signal,
          });
          if (requestVersion !== requestVersionRef.current) {
            return;
          }
          const nextResults = data.results ?? [];
          setResults(nextResults.map((item) => toConversationSearchResult(item, untitled)));
          setHasMore(data.hasMore ?? false);
        } catch {
          if (requestVersion === requestVersionRef.current) {
            setResults([]);
            setHasMore(false);
            setLoadFailed(true);
          }
        } finally {
          if (requestVersion === requestVersionRef.current) {
            setLoading(false);
          }
        }
      })();
    }, normalizedQuery ? NAVIGATION_SEARCH_DEBOUNCE_MS : 0);

    return () => {
      window.clearTimeout(timer);
      if (requestVersionRef.current === requestVersion) {
        requestVersionRef.current += 1;
      }
      abortController.abort();
    };
  }, [normalizedQuery, open, refreshRevision, untitled]);

  const loadMore = React.useCallback(async () => {
    if (!open || loading || loadingMoreRef.current || !hasMore) {
      return;
    }
    const requestVersion = requestVersionRef.current;
    const nextPage = page + 1;
    const requestQuery = normalizedQuery;
    const abortController = new AbortController();
    loadMoreAbortRef.current?.abort();
    loadMoreAbortRef.current = abortController;
    loadingMoreRef.current = true;
    setLoadingMore(true);
    setLoadMoreFailed(false);
    try {
      const token = await resolveAccessToken();
      if (requestVersion !== requestVersionRef.current) {
        return;
      }
      if (!token) {
        setLoadMoreFailed(true);
        return;
      }
      const data = await searchConversations(token, {
        page: nextPage,
        pageSize: NAVIGATION_SEARCH_PAGE_SIZE,
        query: requestQuery,
        signal: abortController.signal,
      });
      if (requestVersion !== requestVersionRef.current) {
        return;
      }
      const nextResults = (data.results ?? []).map((item) => toConversationSearchResult(item, untitled));
      setResults((current) => mergeSearchResults(current, nextResults));
      setPage(nextPage);
      setHasMore(data.hasMore ?? false);
    } catch {
      if (requestVersion === requestVersionRef.current) {
        setLoadMoreFailed(true);
      }
    } finally {
      if (requestVersion === requestVersionRef.current) {
        loadingMoreRef.current = false;
        setLoadingMore(false);
        if (loadMoreAbortRef.current === abortController) {
          loadMoreAbortRef.current = null;
        }
      }
    }
  }, [hasMore, loading, normalizedQuery, open, page, untitled]);

  const retrySearch = React.useCallback(() => {
    setRefreshRevision((current) => current + 1);
  }, []);

  const openSearch = React.useCallback(() => {
    setOpen(true);
  }, []);

  const selectResult = React.useCallback((href: string) => {
    setOpen(false);
    router.push(href);
  }, [router]);

  return {
    open,
    setOpen,
    query,
    setQuery,
    results,
    loading,
    loadingMore,
    loadFailed,
    loadMoreFailed,
    hasMore,
    preview: conversationPreview.preview,
    loadMore,
    retrySearch,
    retryPreview: conversationPreview.retryPreview,
    previewResult: conversationPreview.selectPreview,
    openSearch,
    selectResult,
  };
}

export function useLayoutNavigationShortcuts({
  onCreateConversation,
  onOpenSearch,
}: {
  onCreateConversation: () => void;
  onOpenSearch: () => void;
}) {
  React.useEffect(() => {
    const handleKeyDown = (event: KeyboardEvent) => {
      if (event.isComposing || event.key === "Process") {
        return;
      }

      if (!hasPlatformModifierKey(event)) {
        return;
      }

      const normalizedKey = event.key.toLowerCase();
      if (event.shiftKey && normalizedKey === "o") {
        event.preventDefault();
        onCreateConversation();
        return;
      }

      if (!event.shiftKey && normalizedKey === "k") {
        event.preventDefault();
        onOpenSearch();
      }
    };

    window.addEventListener("keydown", handleKeyDown);
    return () => window.removeEventListener("keydown", handleKeyDown);
  }, [onCreateConversation, onOpenSearch]);
}
