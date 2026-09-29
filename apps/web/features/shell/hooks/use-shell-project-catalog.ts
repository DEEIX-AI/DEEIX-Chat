import * as React from "react";

import { resolveAccessToken } from "@/shared/auth/resolve-access-token";

type ProjectCatalogPage<Item> = {
  results: Item[];
  total: number;
};

export type ProjectCatalogListOptions<ID extends string | number> = {
  ids?: ID[];
  query?: string;
  page?: number;
  pageSize?: number;
};

/**
 * Debounced, paginated catalog search for the project defaults pickers. The
 * first page also resolves the currently selected IDs so selections that fall
 * outside the visible page stay listed, and stale selections can be pruned.
 */
export function useShellProjectCatalog<Item, ID extends string | number>({
  open,
  selectedIDs,
  loadPage,
  getID,
  onSelectedIDsResolved,
  onError,
}: {
  open: boolean;
  selectedIDs: ID[];
  loadPage: (
    accessToken: string,
    options: ProjectCatalogListOptions<ID>,
    signal: AbortSignal,
  ) => Promise<ProjectCatalogPage<Item>>;
  getID: (item: Item) => ID;
  onSelectedIDsResolved: (requestedIDs: ID[], availableIDs: ReadonlySet<ID>) => void;
  onError: () => void;
}) {
  const [items, setItems] = React.useState<Item[]>([]);
  const [loading, setLoading] = React.useState(false);
  const [loadingMore, setLoadingMore] = React.useState(false);
  const [query, setQuery] = React.useState("");
  const [page, setPage] = React.useState(1);
  const [total, setTotal] = React.useState(0);
  const requestControllerRef = React.useRef<AbortController | null>(null);
  const selectedIDsRef = React.useRef(selectedIDs);
  selectedIDsRef.current = selectedIDs;

  const load = React.useCallback(async (nextQuery: string, nextPage = 1) => {
    requestControllerRef.current?.abort();
    const controller = new AbortController();
    requestControllerRef.current = controller;
    if (nextPage === 1) {
      setLoading(true);
    } else {
      setLoadingMore(true);
    }

    const requestedSelectedIDs = nextPage === 1
      ? Array.from(new Set(selectedIDsRef.current))
      : [];
    try {
      const accessToken = await resolveAccessToken();
      if (!accessToken || controller.signal.aborted) {
        if (!accessToken) {
          throw new Error("missing access token");
        }
        return;
      }

      const selectedPagePromise = requestedSelectedIDs.length > 0
        ? loadPage(
            accessToken,
            { ids: requestedSelectedIDs, page: 1, pageSize: requestedSelectedIDs.length },
            controller.signal,
          ).then((selectedPage) => ({ selectedPage, loaded: true })).catch((error: unknown): { selectedPage: null; loaded: boolean } => {
            if (controller.signal.aborted) {
              throw error;
            }
            return { selectedPage: null, loaded: false };
          })
        : Promise.resolve({ selectedPage: null, loaded: false });
      const [catalogPage, selectedResult] = await Promise.all([
        loadPage(
          accessToken,
          { query: nextQuery, page: nextPage, pageSize: 50 },
          controller.signal,
        ),
        selectedPagePromise,
      ]);
      if (controller.signal.aborted) {
        return;
      }

      setItems((current) => {
        const next = nextPage === 1 ? [] : current.slice();
        const seen = new Set(next.map(getID));
        for (const item of catalogPage.results) {
          const id = getID(item);
          if (!seen.has(id)) {
            seen.add(id);
            next.push(item);
          }
        }
        for (const item of selectedResult.selectedPage?.results ?? []) {
          const id = getID(item);
          if (!seen.has(id)) {
            seen.add(id);
            next.push(item);
          }
        }
        return next;
      });
      setPage(nextPage);
      setTotal(catalogPage.total);
      if (selectedResult.loaded && selectedResult.selectedPage) {
        onSelectedIDsResolved(
          requestedSelectedIDs,
          new Set(selectedResult.selectedPage.results.map(getID)),
        );
      }
    } catch {
      if (!controller.signal.aborted) {
        onError();
      }
    } finally {
      if (requestControllerRef.current === controller) {
        requestControllerRef.current = null;
        setLoading(false);
        setLoadingMore(false);
      }
    }
  }, [getID, loadPage, onError, onSelectedIDsResolved]);

  React.useEffect(() => {
    if (!open) {
      requestControllerRef.current?.abort();
      requestControllerRef.current = null;
      setItems([]);
      setLoading(false);
      setLoadingMore(false);
      setQuery("");
      setPage(1);
      setTotal(0);
      return;
    }

    requestControllerRef.current?.abort();
    setLoading(true);
    setLoadingMore(false);
    const timer = window.setTimeout((): void => void load(query.trim(), 1), 200);
    return () => window.clearTimeout(timer);
  }, [load, open, query]);

  React.useEffect(() => () => requestControllerRef.current?.abort(), []);

  const loadMore = React.useCallback(() => {
    if (!loading && !loadingMore && page * 50 < total) {
      void load(query.trim(), page + 1);
    }
  }, [load, loading, loadingMore, page, query, total]);

  return {
    items,
    loading,
    loadingMore,
    setQuery,
    hasMore: page * 50 < total,
    loadMore,
  };
}
