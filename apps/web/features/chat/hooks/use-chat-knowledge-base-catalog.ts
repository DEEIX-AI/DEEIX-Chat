"use client";

import { useTranslations } from "next-intl";
import * as React from "react";
import { toast } from "sonner";

import { listVisibleKnowledgeBases } from "@/shared/api/knowledge-bases";
import type { KnowledgeBaseDTO } from "@/shared/api/knowledge-bases-types";
import { resolveAccessToken } from "@/shared/auth/resolve-access-token";

export const MAX_SELECTED_KNOWLEDGE_BASES = 8;
const KNOWLEDGE_BASE_PAGE_SIZE = 50;

/**
 * Knowledge-base picker catalog. The popover's open state and search query live here
 * because they drive the debounced load and abort in-flight requests on close.
 */
export function useChatKnowledgeBaseCatalog({
  selectedIDs,
  onChange,
}: {
  selectedIDs: string[];
  onChange: (ids: string[]) => void;
}) {
  const t = useTranslations("chat.composer");
  const [open, setOpen] = React.useState(false);
  const [loading, setLoading] = React.useState(false);
  const [loadingMore, setLoadingMore] = React.useState(false);
  const [items, setItems] = React.useState<KnowledgeBaseDTO[]>([]);
  const [page, setPage] = React.useState(1);
  const [total, setTotal] = React.useState(0);
  const [query, setQuery] = React.useState("");
  const openRef = React.useRef(open);
  const requestVersionRef = React.useRef(0);
  const requestControllerRef = React.useRef<AbortController | null>(null);
  const selectedIDsRef = React.useRef(selectedIDs);
  const onChangeRef = React.useRef(onChange);
  const translationRef = React.useRef(t);

  openRef.current = open;
  selectedIDsRef.current = selectedIDs;
  onChangeRef.current = onChange;
  translationRef.current = t;

  const loadCatalog = React.useCallback(async (nextQuery: string, nextPage = 1) => {
    const requestVersion = ++requestVersionRef.current;
    requestControllerRef.current?.abort();
    const requestController = new AbortController();
    requestControllerRef.current = requestController;
    if (nextPage === 1) setLoading(true);
    else setLoadingMore(true);
    try {
      const token = await resolveAccessToken();
      if (requestController.signal.aborted) return;
      if (!token) throw new Error("missing access token");
      const [catalog, selected] = await Promise.all([
        listVisibleKnowledgeBases(token, {
          query: nextQuery,
          page: nextPage,
          pageSize: KNOWLEDGE_BASE_PAGE_SIZE,
        }, requestController.signal),
        nextPage === 1 && selectedIDsRef.current.length > 0
          ? listVisibleKnowledgeBases(token, {
              ids: selectedIDsRef.current.slice(0, MAX_SELECTED_KNOWLEDGE_BASES),
              pageSize: MAX_SELECTED_KNOWLEDGE_BASES,
            }, requestController.signal)
          : Promise.resolve({ results: [], total: 0 }),
      ]);
      if (requestController.signal.aborted || requestVersionRef.current !== requestVersion) return;
      setItems((current) => {
        const next = nextPage === 1 ? catalog.results.slice() : [...current, ...catalog.results];
        const seen = new Set(next.map((item) => item.publicID));
        for (const item of selected.results) {
          if (!seen.has(item.publicID)) next.push(item);
        }
        return next;
      });
      setPage(nextPage);
      setTotal(catalog.total);

      const readyIDs = new Set(
        selected.results.filter((item) => item.readyFileCount > 0).map((item) => item.publicID),
      );
      const currentIDs = selectedIDsRef.current;
      if (nextPage === 1 && currentIDs.length > 0) {
        const nextIDs = currentIDs.filter((id) => readyIDs.has(id));
        if (nextIDs.length !== currentIDs.length) onChangeRef.current(nextIDs);
      }
    } catch {
      if (!requestController.signal.aborted && openRef.current && requestVersionRef.current === requestVersion) {
        toast.error(translationRef.current("knowledgeBaseLoadFailed"));
      }
    } finally {
      if (requestControllerRef.current === requestController) {
        requestControllerRef.current = null;
      }
      if (!requestController.signal.aborted && requestVersionRef.current === requestVersion) {
        setLoading(false);
        setLoadingMore(false);
      }
    }
  }, []);

  React.useEffect(() => {
    return () => requestControllerRef.current?.abort();
  }, []);

  React.useEffect(() => {
    if (!open) return;
    setLoading(true);
    const timer = window.setTimeout((): void => void loadCatalog(query.trim(), 1), 200);
    return () => {
      window.clearTimeout(timer);
      requestControllerRef.current?.abort();
    };
  }, [loadCatalog, open, query]);

  const handleOpenChange = React.useCallback((nextOpen: boolean) => {
    setOpen(nextOpen);
    openRef.current = nextOpen;
    if (!nextOpen) {
      requestControllerRef.current?.abort();
      requestControllerRef.current = null;
      setQuery("");
    }
  }, []);

  const loadMore = React.useCallback(() => {
    void loadCatalog(query.trim(), page + 1);
  }, [loadCatalog, page, query]);

  return {
    open,
    handleOpenChange,
    query,
    setQuery,
    items,
    loading,
    loadingMore,
    hasMore: page * KNOWLEDGE_BASE_PAGE_SIZE < total,
    loadMore,
  };
}
