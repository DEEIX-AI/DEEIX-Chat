import * as React from "react";
import { useTranslations } from "next-intl";
import { toast } from "sonner";

import { useLocalizedErrorMessage } from "@/i18n/use-localized-error";
import {
  deleteAdminKnowledgeBaseFile,
  fetchAdminPlatformFileContent,
  listAdminPlatformFiles,
  uploadAdminKnowledgeBaseFile,
} from "@/shared/api/knowledge-bases";
import type { KnowledgeBaseFileDTO } from "@/shared/api/knowledge-bases-types";
import { resolveAccessToken } from "@/shared/auth/resolve-access-token";
import type { PreviewDialogFile } from "@/entities/file";
import { runSettledBulkItems, runSettledItemsWithConcurrency } from "@/shared/lib/bulk-action";

const PAGE_SIZE = 50;
const UPLOAD_LIMIT = 100;
const SEARCH_DEBOUNCE_MS = 200;

/**
 * Backs the admin "platform files" dialog: debounced search with paged
 * loading, uploads, single/bulk deletion and preview content fetches. Every
 * list request is versioned and abortable so closing the dialog or typing a
 * new query never lets an older response overwrite newer state.
 */
export function useAdminKnowledgeBasesPlatformFiles({ open }: { open: boolean }) {
  const t = useTranslations("knowledgeBases");
  const resolveErrorMessage = useLocalizedErrorMessage();
  const requestVersionRef = React.useRef(0);
  const requestControllerRef = React.useRef<AbortController | null>(null);
  const uploadRequestControllerRef = React.useRef<AbortController | null>(null);
  const [query, setQuery] = React.useState("");
  const [files, setFiles] = React.useState<KnowledgeBaseFileDTO[]>([]);
  const [total, setTotal] = React.useState(0);
  const [page, setPage] = React.useState(1);
  const [loading, setLoading] = React.useState(false);
  const [loadingMore, setLoadingMore] = React.useState(false);
  const [uploading, setUploading] = React.useState(false);
  const [deletingFileID, setDeletingFileID] = React.useState("");
  const [selectedFileIDs, setSelectedFileIDs] = React.useState<string[]>([]);
  const [deleteTarget, setDeleteTarget] = React.useState<KnowledgeBaseFileDTO | null>(null);
  const [deleteDialogOpen, setDeleteDialogOpen] = React.useState(false);
  const [bulkDeleteTargetIDs, setBulkDeleteTargetIDs] = React.useState<string[]>([]);
  const [bulkDeleteDialogOpen, setBulkDeleteDialogOpen] = React.useState(false);
  const [bulkDeleting, setBulkDeleting] = React.useState(false);
  const selectedFileIDSet = React.useMemo(() => new Set(selectedFileIDs), [selectedFileIDs]);
  const allLoadedSelected = files.length > 0 && files.every((file) => selectedFileIDSet.has(file.fileID));
  const someLoadedSelected = files.some((file) => selectedFileIDSet.has(file.fileID));
  const busy = uploading || Boolean(deletingFileID) || bulkDeleting;

  const loadFirstPage = React.useCallback(async (
    requestVersion: number,
    searchQuery: string,
    requestController: AbortController,
  ) => {
    try {
      const token = await requireAccessToken();
      const result = await listAdminPlatformFiles(token, {
        page: 1,
        pageSize: PAGE_SIZE,
        query: searchQuery,
      }, requestController.signal);
      if (requestVersionRef.current !== requestVersion) return;
      setFiles(result.results);
      setTotal(result.total);
      setPage(1);
    } catch (error) {
      if (!requestController.signal.aborted && requestVersionRef.current === requestVersion) {
        toast.error(t("localFilesLoadFailed"), { description: resolveErrorMessage(error) });
      }
    } finally {
      if (requestControllerRef.current === requestController) {
        requestControllerRef.current = null;
      }
      if (requestVersionRef.current === requestVersion) setLoading(false);
    }
  }, [resolveErrorMessage, t]);

  React.useEffect(() => {
    if (!open) {
      requestControllerRef.current?.abort();
      requestControllerRef.current = null;
      uploadRequestControllerRef.current?.abort();
      uploadRequestControllerRef.current = null;
      setLoading(false);
      setLoadingMore(false);
      setUploading(false);
      return;
    }
    setSelectedFileIDs([]);
    const requestVersion = ++requestVersionRef.current;
    requestControllerRef.current?.abort();
    const requestController = new AbortController();
    requestControllerRef.current = requestController;
    setLoading(true);
    setLoadingMore(false);
    const timer = window.setTimeout(
      (): void => void loadFirstPage(requestVersion, query, requestController),
      SEARCH_DEBOUNCE_MS,
    );
    return () => {
      requestController.abort();
      window.clearTimeout(timer);
      if (requestVersionRef.current === requestVersion) requestVersionRef.current += 1;
    };
  }, [loadFirstPage, open, query]);

  React.useEffect(() => () => {
    uploadRequestControllerRef.current?.abort();
    uploadRequestControllerRef.current = null;
  }, []);

  const refresh = React.useCallback(() => {
    const requestVersion = ++requestVersionRef.current;
    requestControllerRef.current?.abort();
    const requestController = new AbortController();
    requestControllerRef.current = requestController;
    setSelectedFileIDs([]);
    setLoading(true);
    setLoadingMore(false);
    void loadFirstPage(requestVersion, query, requestController);
  }, [loadFirstPage, query]);

  const loadMore = React.useCallback(async () => {
    if (loading || loadingMore || page * PAGE_SIZE >= total) return;
    const requestVersion = requestVersionRef.current;
    requestControllerRef.current?.abort();
    const requestController = new AbortController();
    requestControllerRef.current = requestController;
    setLoadingMore(true);
    try {
      const token = await requireAccessToken();
      const nextPage = page + 1;
      const result = await listAdminPlatformFiles(token, {
        page: nextPage,
        pageSize: PAGE_SIZE,
        query,
      }, requestController.signal);
      if (requestVersionRef.current !== requestVersion) return;
      setFiles((current) => {
        const existingIDs = new Set(current.map((file) => file.fileID));
        return [...current, ...result.results.filter((file) => !existingIDs.has(file.fileID))];
      });
      setTotal(result.total);
      setPage(nextPage);
    } catch (error) {
      if (!requestController.signal.aborted && requestVersionRef.current === requestVersion) {
        toast.error(t("localFilesLoadFailed"), { description: resolveErrorMessage(error) });
      }
    } finally {
      if (requestControllerRef.current === requestController) {
        requestControllerRef.current = null;
      }
      if (requestVersionRef.current === requestVersion) setLoadingMore(false);
    }
  }, [loading, loadingMore, page, query, resolveErrorMessage, t, total]);

  const uploadFiles = React.useCallback(async (selectedFiles: File[]) => {
    if (selectedFiles.length === 0 || uploading) return;
    if (selectedFiles.length > UPLOAD_LIMIT) {
      toast.error(t("tooManyFiles", { max: UPLOAD_LIMIT }));
      return;
    }
    setUploading(true);
    uploadRequestControllerRef.current?.abort();
    const requestController = new AbortController();
    uploadRequestControllerRef.current = requestController;
    let uploaded = 0;
    let failed = 0;
    try {
      const token = await requireAccessToken();
      if (requestController.signal.aborted) return;
      const results = await runSettledItemsWithConcurrency({
        items: selectedFiles,
        signal: requestController.signal,
        runItem: (file) => uploadAdminKnowledgeBaseFile(token, file, requestController.signal),
      });
      if (requestController.signal.aborted) return;
      uploaded = results.filter((result) => result.status === "fulfilled").length;
      failed = results.length - uploaded;
      if (uploaded > 0) {
        toast.success(t("localFilesUploaded", { count: uploaded }));
        refresh();
      }
      if (failed > 0) {
        toast.error(t("partialUploadFailed"), {
          description: t("partialUploadDescription", { success: uploaded, failed }),
        });
      }
    } catch (error) {
      if (requestController.signal.aborted) return;
      toast.error(t("uploadFailed"), { description: resolveErrorMessage(error) });
    } finally {
      if (uploadRequestControllerRef.current === requestController) {
        uploadRequestControllerRef.current = null;
        if (!requestController.signal.aborted) setUploading(false);
      }
    }
  }, [refresh, resolveErrorMessage, t, uploading]);

  const deleteFile = React.useCallback(async () => {
    if (!deleteTarget || deletingFileID) return;
    setDeletingFileID(deleteTarget.fileID);
    try {
      const token = await requireAccessToken();
      await deleteAdminKnowledgeBaseFile(token, deleteTarget.fileID);
      setFiles((current) => current.filter((file) => file.fileID !== deleteTarget.fileID));
      setSelectedFileIDs((current) => current.filter((fileID) => fileID !== deleteTarget.fileID));
      setTotal((current) => Math.max(0, current - 1));
      setDeleteDialogOpen(false);
      refresh();
      toast.success(t("platformFileDeleted"));
    } catch (error) {
      toast.error(t("platformFileDeleteFailed"), { description: resolveErrorMessage(error) });
    } finally {
      setDeletingFileID("");
    }
  }, [deleteTarget, deletingFileID, refresh, resolveErrorMessage, t]);

  const toggleFileSelection = React.useCallback((fileID: string, selected: boolean) => {
    setSelectedFileIDs((current) => {
      if (selected) return current.includes(fileID) ? current : [...current, fileID];
      return current.filter((currentFileID) => currentFileID !== fileID);
    });
  }, []);

  const toggleAllLoadedFiles = React.useCallback((selected: boolean) => {
    setSelectedFileIDs(selected ? files.map((file) => file.fileID) : []);
  }, [files]);

  const requestBulkDelete = React.useCallback(() => {
    const loadedFileIDs = new Set(files.map((file) => file.fileID));
    const targets = selectedFileIDs.filter((fileID) => loadedFileIDs.has(fileID));
    if (targets.length === 0) return;
    setBulkDeleteTargetIDs(targets);
    setBulkDeleteDialogOpen(true);
  }, [files, selectedFileIDs]);

  const bulkDeleteFiles = React.useCallback(async () => {
    if (bulkDeleteTargetIDs.length === 0 || bulkDeleting) return;
    setBulkDeleting(true);
    try {
      const token = await requireAccessToken();
      const results = await runSettledBulkItems({
        chunkSize: 10,
        items: bulkDeleteTargetIDs,
        title: t("bulkDeletePlatformFilesTitle"),
        runItem: (fileID) => deleteAdminKnowledgeBaseFile(token, fileID),
      });
      const deletedIDs = new Set(
        results.filter((result) => result.status === "fulfilled").map((result) => result.item),
      );
      const successCount = deletedIDs.size;
      const failedCount = results.length - successCount;
      if (successCount > 0) {
        setFiles((current) => current.filter((file) => !deletedIDs.has(file.fileID)));
        setTotal((current) => Math.max(0, current - successCount));
        refresh();
      }
      setSelectedFileIDs([]);
      setBulkDeleteDialogOpen(false);
      if (failedCount > 0) {
        toast.error(t("bulkDeletePlatformFilesPartialFailed"), {
          description: t("bulkDeletePlatformFilesPartialDescription", {
            success: successCount,
            failed: failedCount,
          }),
        });
      } else {
        toast.success(t("bulkDeletePlatformFilesSucceeded", { count: successCount }));
      }
    } catch (error) {
      toast.error(t("platformFileDeleteFailed"), { description: resolveErrorMessage(error) });
    } finally {
      setBulkDeleting(false);
    }
  }, [bulkDeleteTargetIDs, bulkDeleting, refresh, resolveErrorMessage, t]);

  const loadPreviewContent = React.useCallback(async (file: PreviewDialogFile, signal: AbortSignal) => {
    const token = await requireAccessToken();
    return fetchAdminPlatformFileContent(token, file.fileID, signal);
  }, []);

  return {
    query,
    setQuery,
    files,
    total,
    hasMore: page * PAGE_SIZE < total,
    loading,
    loadingMore,
    uploading,
    deletingFileID,
    selectedFileIDs,
    selectedFileIDSet,
    allLoadedSelected,
    someLoadedSelected,
    deleteTarget,
    setDeleteTarget,
    deleteDialogOpen,
    setDeleteDialogOpen,
    bulkDeleteTargetIDs,
    setBulkDeleteTargetIDs,
    bulkDeleteDialogOpen,
    setBulkDeleteDialogOpen,
    bulkDeleting,
    busy,
    refresh,
    loadMore,
    uploadFiles,
    deleteFile,
    toggleFileSelection,
    toggleAllLoadedFiles,
    requestBulkDelete,
    bulkDeleteFiles,
    loadPreviewContent,
  };
}

async function requireAccessToken(): Promise<string> {
  const token = await resolveAccessToken();
  if (!token) throw new Error("missing access token");
  return token;
}
