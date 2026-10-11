import * as React from "react";
import { useTranslations } from "next-intl";
import { toast } from "sonner";

import {
  type AdminEmbeddingTask,
  type AdminEmbeddingTaskBucket,
  listAdminEmbeddingTasks,
  retryAdminEmbeddingTasks,
} from "@/features/admin/api";
import { resolveAdminErrorMessage } from "@/features/admin/utils/admin-error";
import { resolveAccessToken } from "@/shared/auth/resolve-access-token";
import { useDebouncedValue } from "@/shared/hooks/use-debounced-value";
import { runBulkActionInChunks } from "@/shared/lib/bulk-action";

const PAGE_SIZE = 50;
const SEARCH_DEBOUNCE_MS = 250;
// Matches the backend cap on files per targeted embedding submission.
const RETRY_CHUNK_SIZE = 100;

type UseAdminFilesEmbeddingTasksOptions = {
  open: boolean;
  bucket: AdminEmbeddingTaskBucket;
  /** Called after a retry was submitted so the index status counts can refresh. */
  onTasksChanged: () => void;
};

/**
 * Backs the platform-wide embedding task dialog: bucketed, searchable paged
 * loading plus single and bulk retries. List requests are versioned and
 * abortable so switching buckets or typing never lets an older response
 * overwrite newer state.
 */
export function useAdminFilesEmbeddingTasks({ open, bucket, onTasksChanged }: UseAdminFilesEmbeddingTasksOptions) {
  const t = useTranslations("adminFiles");
  const [query, setQuery] = React.useState("");
  const debouncedQuery = useDebouncedValue(query.trim(), SEARCH_DEBOUNCE_MS);
  const [tasks, setTasks] = React.useState<AdminEmbeddingTask[]>([]);
  const [total, setTotal] = React.useState(0);
  const [page, setPage] = React.useState(1);
  const [loading, setLoading] = React.useState(false);
  const [loadingMore, setLoadingMore] = React.useState(false);
  const [selectedFileIDs, setSelectedFileIDs] = React.useState<string[]>([]);
  const [retryingFileIDs, setRetryingFileIDs] = React.useState<string[]>([]);
  const requestVersionRef = React.useRef(0);
  const requestControllerRef = React.useRef<AbortController | null>(null);
  // Ids already rendered, so appending stays duplicate-free and can detect a page that added nothing.
  const loadedFileIDsRef = React.useRef<Set<string>>(new Set());

  const loadPage = React.useCallback(async (nextPage: number, append: boolean) => {
    const requestVersion = append ? requestVersionRef.current : ++requestVersionRef.current;
    requestControllerRef.current?.abort();
    const requestController = new AbortController();
    requestControllerRef.current = requestController;
    if (append) {
      setLoadingMore(true);
    } else {
      setLoading(true);
      setSelectedFileIDs([]);
      loadedFileIDsRef.current = new Set();
    }
    try {
      const token = await resolveAccessToken();
      if (!token) {
        toast.error(t("toast.sessionExpired"), { description: t("toast.sessionExpiredDescription") });
        return;
      }
      if (requestController.signal.aborted) return;
      const result = await listAdminEmbeddingTasks(
        token,
        { bucket, query: debouncedQuery, page: nextPage, pageSize: PAGE_SIZE },
        requestController.signal,
      );
      if (requestVersionRef.current !== requestVersion) return;
      if (append) {
        const fresh = result.results.filter((task) => !loadedFileIDsRef.current.has(task.fileID));
        if (fresh.length === 0) {
          // Nothing new: stop paging instead of letting the sentinel request the same page forever.
          setTotal(loadedFileIDsRef.current.size);
        } else {
          for (const task of fresh) loadedFileIDsRef.current.add(task.fileID);
          setTasks((current) => [...current, ...fresh]);
          setTotal(result.total);
          setPage(nextPage);
        }
      } else {
        loadedFileIDsRef.current = new Set(result.results.map((task) => task.fileID));
        setTasks(result.results);
        setTotal(result.total);
        setPage(1);
      }
    } catch (error) {
      if (requestController.signal.aborted || requestVersionRef.current !== requestVersion) return;
      if (!append) {
        setTasks([]);
        setTotal(0);
      }
      toast.error(t("embeddingTasks.loadFailed"), {
        description: resolveAdminErrorMessage(error, t("toast.unknownError")),
      });
    } finally {
      if (requestControllerRef.current === requestController) {
        requestControllerRef.current = null;
      }
      if (requestVersionRef.current === requestVersion) {
        setLoading(false);
        setLoadingMore(false);
      }
    }
  }, [bucket, debouncedQuery, t]);

  React.useEffect(() => {
    if (!open) {
      requestVersionRef.current += 1;
      requestControllerRef.current?.abort();
      requestControllerRef.current = null;
      setLoading(false);
      setLoadingMore(false);
      // The next opening starts with a clean search; the list itself stays until then so the
      // closing animation does not flash an empty state.
      setQuery("");
      setSelectedFileIDs([]);
      return;
    }
    void loadPage(1, false);
  }, [loadPage, open]);

  React.useEffect(() => () => {
    requestControllerRef.current?.abort();
    requestControllerRef.current = null;
  }, []);

  const hasMore = tasks.length < total;
  const loadMore = React.useCallback(() => {
    if (loading || loadingMore || !hasMore) return;
    void loadPage(page + 1, true);
  }, [hasMore, loadPage, loading, loadingMore, page]);

  const retryableFileIDs = React.useMemo(
    () => tasks.filter((task) => task.retryable).map((task) => task.fileID),
    [tasks],
  );
  const selectedFileIDSet = React.useMemo(() => new Set(selectedFileIDs), [selectedFileIDs]);
  const retryingFileIDSet = React.useMemo(() => new Set(retryingFileIDs), [retryingFileIDs]);
  const allRetryableSelected =
    retryableFileIDs.length > 0 && retryableFileIDs.every((fileID) => selectedFileIDSet.has(fileID));
  const someRetryableSelected = retryableFileIDs.some((fileID) => selectedFileIDSet.has(fileID));
  const retrying = retryingFileIDs.length > 0;

  const toggleSelection = React.useCallback((fileID: string, checked: boolean) => {
    setSelectedFileIDs((current) => {
      if (checked) return current.includes(fileID) ? current : [...current, fileID];
      return current.filter((value) => value !== fileID);
    });
  }, []);

  const toggleAllRetryable = React.useCallback((checked: boolean) => {
    setSelectedFileIDs(checked ? retryableFileIDs : []);
  }, [retryableFileIDs]);

  const retry = React.useCallback(async (fileIDs: string[]) => {
    const targets = Array.from(new Set(fileIDs));
    if (targets.length === 0 || retrying) return;
    setRetryingFileIDs(targets);
    try {
      const token = await resolveAccessToken();
      if (!token) {
        toast.error(t("toast.sessionExpired"), { description: t("toast.sessionExpiredDescription") });
        return;
      }
      const results = await runBulkActionInChunks({
        items: targets,
        chunkSize: RETRY_CHUNK_SIZE,
        title: t("embeddingTasks.retrying"),
        runChunk: (chunk) => retryAdminEmbeddingTasks(token, chunk),
      });
      const submitted = results.reduce((count, result) => count + result.submittedFileIDs.length, 0);
      const skipped = results.reduce((count, result) => count + result.skipped.length, 0);
      if (submitted > 0) {
        toast.success(t("embeddingTasks.retrySubmitted", { count: submitted }), {
          description: skipped > 0 ? t("embeddingTasks.retrySkipped", { count: skipped }) : undefined,
        });
      } else {
        toast.warning(t("embeddingTasks.retryNothing"), {
          description: t("embeddingTasks.retrySkipped", { count: skipped }),
        });
      }
      onTasksChanged();
      void loadPage(1, false);
    } catch (error) {
      toast.error(t("embeddingTasks.retryFailed"), {
        description: resolveAdminErrorMessage(error, t("toast.unknownError")),
      });
    } finally {
      setRetryingFileIDs([]);
    }
  }, [loadPage, onTasksChanged, retrying, t]);

  return {
    query,
    setQuery,
    tasks,
    total,
    loading,
    loadingMore,
    hasMore,
    loadMore,
    selectedFileIDs,
    selectedFileIDSet,
    allRetryableSelected,
    someRetryableSelected,
    toggleSelection,
    toggleAllRetryable,
    retrying,
    retryingFileIDSet,
    retry,
  };
}
