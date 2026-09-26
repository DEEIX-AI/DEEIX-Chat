"use client";

import { useSearchParams } from "next/navigation";
import { useTranslations } from "next-intl";
import * as React from "react";
import { toast } from "sonner";
import { useFileExtract } from "@/features/files/hooks/use-file-extract";
import { useFileInvalidation } from "@/features/files/hooks/use-file-invalidation";
import { useFilePreview } from "@/features/files/hooks/use-file-preview";
import type { FileFilterValue, FileSortKey } from "@/features/files/types/files";
import { useLocalizedErrorMessage } from "@/i18n/use-localized-error";
import {
  deleteFile,
  listFiles,
  renameFile,
  submitFileEmbeddings,
  updateFileRagOptOut,
  uploadFile,
} from "@/shared/api/file";
import type {
  FileObjectDTO,
  FileProcessingStatusDTO,
  UploadFileResult,
  UserStorageQuotaDTO,
} from "@/shared/api/file.types";
import { resolveAccessToken } from "@/shared/auth/resolve-access-token";
import {
  type FileStatusPollingResult,
  useFileProcessingStatusPolling,
} from "@/shared/hooks/use-file-processing-status-polling";
import { useDebouncedValue } from "@/shared/hooks/use-debounced-value";
import { runBulkActionInChunks, runSettledItemsWithConcurrency } from "@/shared/lib/bulk-action";
import { resolveFileFilter } from "@/shared/lib/file-display";
import { canManuallyVectorizeFile, isFileProcessing } from "@/shared/lib/file-processing";
import { patchByID, replaceByID, upsertByID } from "@/shared/lib/optimistic-list";

const FILES_PAGE_SIZE = 100;
const FILE_SEARCH_DEBOUNCE_MS = 250;

type FilesMobileView = "list" | "detail";
type FileContentTab = "preview" | "extract";

type LoadFilesOptions = {
  preferredFileID?: string | null;
  ensurePreferred?: boolean;
  silent?: boolean;
  background?: boolean;
  page?: number;
  append?: boolean;
};

type UseFilesPageResult = {
  fileInputRef: React.RefObject<HTMLInputElement | null>;
  mobileView: FilesMobileView;
  files: FileObjectDTO[];
  total: number;
  selectedFile: FileObjectDTO | null;
  selectedFileID: string | null;
  quota: UserStorageQuotaDTO | null;
  loading: boolean;
  syncing: boolean;
  loadingMore: boolean;
  uploading: boolean;
  deletingFileID: string | null;
  selectedFileIDs: string[];
  bulkDeleteOpen: boolean;
  bulkDeleting: boolean;
  vectorizing: boolean;
  vectorizingFileIDs: string[];
  hasMore: boolean;
  query: string;
  sortKey: FileSortKey;
  filterKeys: FileFilterValue[];
  isSidebarCollapsed: boolean;
  isSearchOpen: boolean;
  renamingFileID: string | null;
  renameValue: string;
  deleteTarget: FileObjectDTO | null;
  preview: ReturnType<typeof useFilePreview>["preview"];
  extract: ReturnType<typeof useFileExtract>;
  contentTab: FileContentTab;
  openPreview: ReturnType<typeof useFilePreview>["open"];
  downloadPreview: ReturnType<typeof useFilePreview>["download"];
  onContentTabChange: (value: FileContentTab) => void;
  onOpenUploadPicker: () => void;
  onFilesPicked: (event: React.ChangeEvent<HTMLInputElement>) => Promise<void>;
  onLoadMore: () => Promise<void>;
  onSelectFile: (fileID: string) => void;
  onToggleSidebarCollapsed: () => void;
  onToggleSearch: () => void;
  onQueryChange: (value: string) => void;
  onFilterToggle: (value: FileFilterValue | "all") => void;
  onSortChange: (value: FileSortKey) => void;
  onRenameStart: (item: FileObjectDTO) => void;
  onRenameValueChange: (value: string) => void;
  onRenameCommit: (fileID: string, currentFileName: string) => Promise<void>;
  onRenameCancel: () => void;
  onDeleteRequest: (item: FileObjectDTO) => void;
  onClearDeleteTarget: () => void;
  onConfirmDeleteTarget: () => Promise<void>;
  onToggleFileSelection: (fileID: string, checked: boolean) => void;
  onSelectLoadedFiles: () => void;
  onClearFileSelection: () => void;
  onBulkDeleteRequest: () => void;
  onClearBulkDelete: () => void;
  onConfirmBulkDelete: () => Promise<void>;
  onVectorizeFile: (fileID: string) => Promise<void>;
  onVectorizeSelected: () => Promise<void>;
  onBackToList: () => void;
  onToggleRagOptOut: (fileID: string, current: boolean) => Promise<void>;
};

function normalizeContentTab(value: string | null): FileContentTab {
  return value === "extract" ? "extract" : "preview";
}

export function useFilesPage(): UseFilesPageResult {
  const t = useTranslations("files");
  const resolveErrorMessage = useLocalizedErrorMessage();
  const searchParams = useSearchParams();
  const requestedFileID = searchParams.get("file")?.trim() || null;
  const requestedContentTab = normalizeContentTab(searchParams.get("tab"));
  const [mobileView, setMobileView] = React.useState<FilesMobileView>("list");
  const fileInputRef = React.useRef<HTMLInputElement | null>(null);
  const filesRef = React.useRef<FileObjectDTO[]>([]);
  const totalRef = React.useRef(0);
  const isMountedRef = React.useRef(false);
  const loadRequestSeqRef = React.useRef(0);
  const loadRequestControllerRef = React.useRef<AbortController | null>(null);
  const uploadRequestControllerRef = React.useRef<AbortController | null>(null);
  const hasLoadedOnceRef = React.useRef(false);

  const [files, setFiles] = React.useState<FileObjectDTO[]>([]);
  const [total, setTotal] = React.useState(0);
  const [selectedFileID, setSelectedFileID] = React.useState<string | null>(requestedFileID);
  const [quota, setQuota] = React.useState<UserStorageQuotaDTO | null>(null);
  const [loading, setLoading] = React.useState(true);
  const [syncing, setSyncing] = React.useState(false);
  const [loadingMore, setLoadingMore] = React.useState(false);
  const [uploading, setUploading] = React.useState(false);
  const [deletingFileID, setDeletingFileID] = React.useState<string | null>(null);
  const [selectedFileIDs, setSelectedFileIDs] = React.useState<string[]>([]);
  const [bulkDeleteOpen, setBulkDeleteOpen] = React.useState(false);
  const [bulkDeleting, setBulkDeleting] = React.useState(false);
  const [vectorizingFileIDs, setVectorizingFileIDs] = React.useState<string[]>([]);
  const [nextPage, setNextPage] = React.useState(2);
  const [hasMore, setHasMore] = React.useState(false);
  const [query, setQuery] = React.useState("");
  const debouncedQuery = useDebouncedValue(query.trim(), FILE_SEARCH_DEBOUNCE_MS);
  const [sortKey, setSortKey] = React.useState<FileSortKey>("created");
  const [filterKeys, setFilterKeys] = React.useState<FileFilterValue[]>([]);
  const [isSidebarCollapsed, setIsSidebarCollapsed] = React.useState(false);
  const [isSearchOpen, setIsSearchOpen] = React.useState(false);
  const [renamingFileID, setRenamingFileID] = React.useState<string | null>(null);
  const [renameValue, setRenameValue] = React.useState("");
  const [deleteTarget, setDeleteTarget] = React.useState<FileObjectDTO | null>(null);
  const [contentTab, setContentTab] = React.useState<FileContentTab>(requestedContentTab);

  const ensureAccessToken = React.useCallback(async () => {
    return resolveAccessToken();
  }, []);

  React.useEffect(() => {
    isMountedRef.current = true;
    return () => {
      isMountedRef.current = false;
      loadRequestControllerRef.current?.abort();
      loadRequestControllerRef.current = null;
      uploadRequestControllerRef.current?.abort();
      uploadRequestControllerRef.current = null;
    };
  }, []);

  React.useEffect(() => {
    filesRef.current = files;
  }, [files]);

  React.useEffect(() => {
    const visibleFileIDs = new Set(files.map((item) => item.fileID));
    setSelectedFileIDs((current) => current.filter((fileID) => visibleFileIDs.has(fileID)));
  }, [files]);

  React.useEffect(() => {
    totalRef.current = total;
  }, [total]);

  React.useEffect(() => {
    setContentTab(requestedContentTab);
  }, [requestedContentTab]);

  React.useEffect(() => {
    if (!requestedFileID) {
      return;
    }
    setSelectedFileID(requestedFileID);
    setMobileView("detail");
  }, [requestedFileID]);

  const loadFiles = React.useCallback(
    async (options: LoadFilesOptions = {}) => {
      const requestSeq = loadRequestSeqRef.current + 1;
      loadRequestSeqRef.current = requestSeq;
      loadRequestControllerRef.current?.abort();
      const requestController = new AbortController();
      loadRequestControllerRef.current = requestController;
      const page = options.page ?? 1;
      const isLatestRequest = () => loadRequestSeqRef.current === requestSeq;
      let token = "";
      try {
        token = await ensureAccessToken();
      } catch (error) {
        if (loadRequestControllerRef.current === requestController) {
          loadRequestControllerRef.current = null;
        }
        if (!requestController.signal.aborted && isMountedRef.current && isLatestRequest()) {
          setLoading(false);
          setLoadingMore(false);
          setSyncing(false);
          toast.error(t("toasts.listLoadFailed"), {
            id: "files-list-load-error",
            description: resolveErrorMessage(error, t("toasts.listLoadFailed")),
          });
        }
        return;
      }

      if (requestController.signal.aborted) {
        return;
      }

      if (!token) {
        if (loadRequestControllerRef.current === requestController) {
          loadRequestControllerRef.current = null;
        }
        if (!isMountedRef.current || !isLatestRequest()) {
          return;
        }
        setFiles([]);
        setTotal(0);
        setSelectedFileID(null);
        setHasMore(false);
        setNextPage(2);
        setLoading(false);
        setLoadingMore(false);
        setSyncing(false);
        hasLoadedOnceRef.current = true;
        toast.error(t("toasts.sessionExpired"), { description: t("toasts.viewAfterLogin") });
        return;
      }

      if (options.append) {
        setLoadingMore(true);
      } else if (options.silent || hasLoadedOnceRef.current) {
        if (!options.background) {
          setSyncing(true);
        }
      } else {
        setLoading(true);
      }

      try {
        const data = await listFiles(token, {
          page,
          pageSize: FILES_PAGE_SIZE,
          query: debouncedQuery,
          kind: filterKeys,
          sort: sortKey,
        }, requestController.signal);
        if (!isMountedRef.current || !isLatestRequest()) {
          return;
        }

        const currentFiles = filesRef.current;
        let nextItems = data.results;
        if (options.append) {
          const currentFileIDs = new Set(currentFiles.map((item) => item.fileID));
          nextItems = [...currentFiles, ...data.results.filter((item) => !currentFileIDs.has(item.fileID))];
        }
        const explicitPreferredFileID = options.preferredFileID?.trim() || "";
        if (options.ensurePreferred && explicitPreferredFileID && !nextItems.some((item) => item.fileID === explicitPreferredFileID)) {
          const preferredData = await listFiles(token, {
            page: 1,
            pageSize: 1,
            query: explicitPreferredFileID,
            sort: "created",
          }, requestController.signal);
          if (!isMountedRef.current || !isLatestRequest()) {
            return;
          }
          const preferredFile = preferredData.results.find((item) => item.fileID === explicitPreferredFileID);
          if (preferredFile) {
            nextItems = [preferredFile, ...nextItems.filter((item) => item.fileID !== preferredFile.fileID)];
          }
        }

        filesRef.current = nextItems;
        setFiles(nextItems);
        setTotal(data.total);
        setQuota(data.quota);
        setSelectedFileID((current) => {
          const preferredFileID = options.preferredFileID ?? current;
          if (preferredFileID && nextItems.some((item) => item.fileID === preferredFileID)) {
            return preferredFileID;
          }
          return nextItems[0]?.fileID ?? null;
        });
        setHasMore(page * FILES_PAGE_SIZE < data.total);
        setNextPage(page + 1);
      } catch (error) {
        if (requestController.signal.aborted) {
          return;
        }
        if (!isMountedRef.current || !isLatestRequest()) {
          return;
        }
        if (!options.background) {
          const description = resolveErrorMessage(error, t("toasts.listLoadFailed"));
          toast.error(t("toasts.listLoadFailed"), { id: "files-list-load-error", description });
        }
      } finally {
        if (loadRequestControllerRef.current === requestController) {
          loadRequestControllerRef.current = null;
        }
        if (isMountedRef.current && isLatestRequest()) {
          hasLoadedOnceRef.current = true;
          setLoading(false);
          setLoadingMore(false);
          setSyncing(false);
        }
      }
    },
    [debouncedQuery, ensureAccessToken, filterKeys, resolveErrorMessage, sortKey, t],
  );

  React.useEffect(() => {
    void loadFiles({
      preferredFileID: requestedFileID,
      ensurePreferred: Boolean(requestedFileID),
    });
  }, [loadFiles, requestedFileID]);

  const processingFileIDs = React.useMemo(
    () => files
      .filter(isFileProcessing)
      .map((item) => item.fileID),
    [files],
  );

  const onProcessingResult = React.useCallback(({
    statuses,
    missingFileIDs,
  }: FileStatusPollingResult<FileProcessingStatusDTO>) => {
    const statusesByID = new Map(statuses.map((status) => [status.fileID, status]));
    const missingFileIDSet = new Set(missingFileIDs);
    const currentFiles = filesRef.current;
    let changed = false;
    let removedCount = 0;
    const nextFiles: FileObjectDTO[] = [];
    for (const item of currentFiles) {
      if (missingFileIDSet.has(item.fileID)) {
        changed = true;
        removedCount += 1;
        continue;
      }
      const status = statusesByID.get(item.fileID);
      if (!status || (
        item.detectedMIME === status.detectedMIME &&
        item.fileCategory === status.fileCategory &&
        item.processingStatus === status.processingStatus &&
        item.processingReady === status.processingReady &&
        item.processingErrorCode === status.errorCode &&
        item.processingErrorMessage === status.errorMessage &&
        item.extractStatus === status.extractStatus &&
        item.embedStatus === status.embedStatus &&
        item.embedError === status.embedError &&
        item.chunkCount === status.chunkCount &&
        item.canVectorize === status.canVectorize &&
        item.vectorizationReason === status.vectorizationReason &&
        item.updatedAt === status.updatedAt
      )) {
        nextFiles.push(item);
        continue;
      }
      changed = true;
      nextFiles.push({
        ...item,
        detectedMIME: status.detectedMIME,
        fileCategory: status.fileCategory,
        processingStatus: status.processingStatus,
        processingReady: status.processingReady,
        processingErrorCode: status.errorCode,
        processingErrorMessage: status.errorMessage,
        extractStatus: status.extractStatus,
        embedStatus: status.embedStatus,
        embedError: status.embedError,
        chunkCount: status.chunkCount,
        canVectorize: status.canVectorize,
        vectorizationReason: status.vectorizationReason,
        updatedAt: status.updatedAt,
      });
    }
    if (changed) {
      filesRef.current = nextFiles;
      setFiles(nextFiles);
    }
    if (removedCount > 0) {
      setTotal((current) => Math.max(0, current - removedCount));
      setSelectedFileID((current) =>
        current && missingFileIDSet.has(current) ? (nextFiles[0]?.fileID ?? null) : current);
    }
  }, []);

  useFileProcessingStatusPolling({
    fileIDs: loading || loadingMore || uploading ? [] : processingFileIDs,
    intervalMs: 2000,
    onResult: onProcessingResult,
  });

  useFileInvalidation(
    React.useCallback((detail) => {
      if (detail.quota) {
        setQuota(detail.quota);
      }
      void loadFiles({ preferredFileID: selectedFileID, silent: true, background: true });
    }, [loadFiles, selectedFileID]),
  );

  const selectedFile = React.useMemo(
    () => files.find((item) => item.fileID === selectedFileID) ?? null,
    [files, selectedFileID],
  );
  const vectorizing = vectorizingFileIDs.length > 0;

  const { preview, open, download } = useFilePreview({
    file: selectedFile,
    getAccessToken: ensureAccessToken,
  });
  const extract = useFileExtract({
    file: selectedFile,
    enabled: contentTab === "extract",
    getAccessToken: ensureAccessToken,
  });

  const onOpenUploadPicker = React.useCallback(() => {
    fileInputRef.current?.click();
  }, []);

  const onFilesPicked = React.useCallback(
    async (event: React.ChangeEvent<HTMLInputElement>) => {
      const nextFiles = Array.from(event.target.files ?? []);
      event.target.value = "";
      if (nextFiles.length === 0) {
        return;
      }

      uploadRequestControllerRef.current?.abort();
      const requestController = new AbortController();
      uploadRequestControllerRef.current = requestController;
      const token = await ensureAccessToken();
      if (requestController.signal.aborted || !isMountedRef.current) {
        return;
      }
      if (!token) {
        if (uploadRequestControllerRef.current === requestController) {
          uploadRequestControllerRef.current = null;
        }
        toast.error(t("toasts.sessionExpired"), { description: t("toasts.uploadAfterLogin") });
        return;
      }

      setUploading(true);
      try {
        const results = await runSettledItemsWithConcurrency({
          items: nextFiles,
          signal: requestController.signal,
          runItem: (file) => uploadFile(token, file, { signal: requestController.signal }),
        });
        if (requestController.signal.aborted || !isMountedRef.current) {
          return;
        }
        const successResults: UploadFileResult[] = [];
        let failedCount = 0;

        for (const result of results) {
          if (result.status === "fulfilled") {
            successResults.push(result.value);
          } else {
            failedCount += 1;
          }
        }

        if (successResults.length > 0) {
          const latest = successResults[successResults.length - 1];
          const reusedCount = successResults.filter((item) => item.reused).length;
          const uploadedCount = successResults.length - reusedCount;
          const currentFileIDs = new Set(filesRef.current.map((item) => item.fileID));
          const normalizedQuery = debouncedQuery.toLowerCase();
          const seenUploadedFileIDs = new Set<string>();
          const nextUploadedFiles = successResults
            .map((item) => item.file)
            .filter((item) => {
              if (currentFileIDs.has(item.fileID) || seenUploadedFileIDs.has(item.fileID)) {
                return false;
              }
              seenUploadedFileIDs.add(item.fileID);
              if (normalizedQuery && !item.fileName.toLowerCase().includes(normalizedQuery)) {
                return false;
              }
              return filterKeys.length === 0 || filterKeys.includes(resolveFileFilter(item) as FileFilterValue);
            });
          setFiles((current) => {
            const next = nextUploadedFiles.reduce(
              (list, item) => upsertByID(list, item, (file) => file.fileID),
              current,
            );
            filesRef.current = next;
            return next;
          });
          setTotal((current) => current + nextUploadedFiles.length);
          if (nextUploadedFiles.some((item) => item.fileID === latest.file.fileID)) {
            setSelectedFileID(latest.file.fileID);
          }
          setQuota(latest.quota);
          void loadFiles({ preferredFileID: latest.file.fileID, silent: true, background: true });
          if (uploadedCount === 0 && reusedCount > 0) {
            toast.success(t("toasts.duplicateReused"));
          } else if (reusedCount > 0) {
            toast.success(
              uploadedCount === 1 ? t("toasts.uploadedOne") : t("toasts.uploadedMany", { count: uploadedCount }),
              { description: t("toasts.duplicateReused") },
            );
          } else {
            toast.success(successResults.length === 1 ? t("toasts.uploadedOne") : t("toasts.uploadedMany", { count: successResults.length }));
          }
        }

        if (failedCount > 0) {
          toast.error(t("toasts.partialUploadFailed"), {
            description: t("toasts.partialUploadDescription", { success: successResults.length, failed: failedCount }),
          });
        }
      } catch (error) {
        if (requestController.signal.aborted || !isMountedRef.current) {
          return;
        }
        const description = resolveErrorMessage(error, t("toasts.uploadFailed"));
        toast.error(t("toasts.uploadFailed"), { description });
      } finally {
        if (uploadRequestControllerRef.current === requestController) {
          uploadRequestControllerRef.current = null;
          if (isMountedRef.current) {
            setUploading(false);
          }
        }
      }
    },
    [debouncedQuery, ensureAccessToken, filterKeys, loadFiles, resolveErrorMessage, t],
  );

  const onDeleteFile = React.useCallback(
    async (fileID: string) => {
      const token = await ensureAccessToken();
      if (!token) {
        toast.error(t("toasts.sessionExpired"), { description: t("toasts.deleteAfterLogin") });
        return;
      }

      setDeletingFileID(fileID);
      try {
        const result = await deleteFile(token, fileID);
        const currentFiles = filesRef.current;
        const deletedIndex = currentFiles.findIndex((item) => item.fileID === fileID);
        const nextFiles = currentFiles.filter((item) => item.fileID !== fileID);
        const nextSelectedFileID = nextFiles[deletedIndex]?.fileID ?? nextFiles[deletedIndex - 1]?.fileID ?? nextFiles[0]?.fileID ?? null;

        filesRef.current = nextFiles;
        setFiles(nextFiles);
        setTotal((current) => Math.max(0, current - (deletedIndex >= 0 ? 1 : 0)));
        if (selectedFileID === fileID) {
          setSelectedFileID(nextSelectedFileID);
        }
        setQuota(result.quota);
        void loadFiles({ preferredFileID: selectedFileID === fileID ? nextSelectedFileID : selectedFileID, silent: true, background: true });
        toast.success(t("toasts.deleteSucceeded"));
      } catch (error) {
        const description = resolveErrorMessage(error, t("toasts.deleteFailed"));
        toast.error(t("toasts.deleteFailed"), { description });
      } finally {
        setDeletingFileID(null);
      }
    },
    [ensureAccessToken, loadFiles, resolveErrorMessage, selectedFileID, t],
  );

  const onToggleFileSelection = React.useCallback((fileID: string, checked: boolean) => {
    setSelectedFileIDs((current) => {
      const next = new Set(current);
      if (checked) {
        next.add(fileID);
      } else {
        next.delete(fileID);
      }
      return Array.from(next);
    });
  }, []);

  const onSelectLoadedFiles = React.useCallback(() => {
    setSelectedFileIDs(filesRef.current.map((item) => item.fileID));
  }, []);

  const onClearFileSelection = React.useCallback(() => {
    setSelectedFileIDs([]);
  }, []);

  const onBulkDeleteRequest = React.useCallback(() => {
    if (selectedFileIDs.length === 0) {
      return;
    }
    setBulkDeleteOpen(true);
  }, [selectedFileIDs.length]);

  const onClearBulkDelete = React.useCallback(() => {
    if (!bulkDeleting) {
      setBulkDeleteOpen(false);
    }
  }, [bulkDeleting]);

  const onConfirmBulkDelete = React.useCallback(async () => {
    const visibleFileIDs = new Set(filesRef.current.map((item) => item.fileID));
    const fileIDs = selectedFileIDs.filter((fileID) => visibleFileIDs.has(fileID));
    if (fileIDs.length === 0) {
      setBulkDeleteOpen(false);
      return;
    }
    const token = await ensureAccessToken();
    if (!token) {
      toast.error(t("toasts.sessionExpired"), { description: t("toasts.deleteAfterLogin") });
      return;
    }

    setBulkDeleting(true);
    let successCount = 0;
    let failedCount = 0;
    let latestQuota: UserStorageQuotaDTO | null = null;
    await runBulkActionInChunks({
      chunkSize: 10,
      items: fileIDs,
      title: t("bulkDeleteDialog.title"),
      runChunk: async (chunk) => {
        for (const fileID of chunk) {
          try {
            const result = await deleteFile(token, fileID);
            latestQuota = result.quota;
            successCount += 1;
          } catch {
            failedCount += 1;
          }
        }
      }
    });

    if (latestQuota) {
      setQuota(latestQuota);
    }
    setSelectedFileIDs([]);
    setBulkDeleteOpen(false);
    setBulkDeleting(false);
    const selectedFileDeleted = selectedFileID ? fileIDs.includes(selectedFileID) : false;
    if (selectedFileDeleted) {
      setSelectedFileID(null);
    }
    await loadFiles({ preferredFileID: selectedFileDeleted ? null : selectedFileID, silent: true, background: true });

    if (failedCount > 0) {
      toast.error(t("toasts.bulkDeletePartialFailed"), {
        description: t("toasts.bulkDeletePartialDescription", { success: successCount, failed: failedCount }),
      });
      return;
    }
    toast.success(t("toasts.bulkDeleteSucceeded", { count: successCount }));
  }, [ensureAccessToken, loadFiles, selectedFileID, selectedFileIDs, t]);

  const submitVectorization = React.useCallback(async (fileIDs: string[], clearSelection: boolean) => {
    if (vectorizing) {
      return;
    }
    const normalizedFileIDs = Array.from(new Set(fileIDs.map((fileID) => fileID.trim()).filter(Boolean)));
    if (normalizedFileIDs.length === 0) {
      return;
    }
    if (normalizedFileIDs.length > 100) {
      toast.error(t("toasts.vectorizeLimit"));
      return;
    }

    setVectorizingFileIDs(normalizedFileIDs);
    try {
      const token = await ensureAccessToken();
      if (!token) {
        toast.error(t("toasts.sessionExpired"), { description: t("toasts.operateAfterLogin") });
        return;
      }
      const result = await submitFileEmbeddings(token, normalizedFileIDs);
      const submittedFileIDSet = new Set(result.submittedFileIDs);
      const failedCount = result.skipped.filter(({ reason }) => ["queue_busy", "submit_failed"].includes(reason)).length;
      if (submittedFileIDSet.size > 0) {
        const nextFiles = filesRef.current.map((item) => submittedFileIDSet.has(item.fileID)
          ? {
            ...item,
            embedStatus: "queued",
            embedError: "",
            canVectorize: false,
            vectorizationReason: "processing",
          }
          : item);
        filesRef.current = nextFiles;
        setFiles(nextFiles);
      }
      if (clearSelection) {
        setSelectedFileIDs((current) => current.filter((fileID) => !submittedFileIDSet.has(fileID)));
      }
      if (failedCount > 0) {
        toast.warning(t("toasts.vectorizePartial", { submitted: result.submittedFileIDs.length, failed: failedCount }));
      } else if (result.submittedFileIDs.length === 0) {
        toast.info(t("toasts.vectorizeNoChanges"));
      } else {
        toast.success(t("toasts.vectorizeSubmitted", { count: result.submittedFileIDs.length }), {
          description: result.skipped.length > 0
            ? t("toasts.vectorizeSkipped", { count: result.skipped.length })
            : undefined,
        });
      }
    } catch (error) {
      toast.error(t("toasts.vectorizeFailed"), {
        description: resolveErrorMessage(error, t("toasts.vectorizeFailed")),
      });
    } finally {
      setVectorizingFileIDs([]);
    }
  }, [ensureAccessToken, resolveErrorMessage, t, vectorizing]);

  const onVectorizeFile = React.useCallback(async (fileID: string) => {
    await submitVectorization([fileID], false);
  }, [submitVectorization]);

  const onVectorizeSelected = React.useCallback(async () => {
    const selectedFileIDSet = new Set(selectedFileIDs);
    const fileIDs = filesRef.current
      .filter((file) => selectedFileIDSet.has(file.fileID) && canManuallyVectorizeFile(file))
      .map((file) => file.fileID);
    await submitVectorization(fileIDs, true);
  }, [selectedFileIDs, submitVectorization]);

  const onRenameCommit = React.useCallback(
    async (fileID: string, currentFileName: string) => {
      const nextFileName = renameValue.trim();
      if (!nextFileName) {
        toast.error(t("toasts.renameEmpty"));
        return;
      }
      if (nextFileName === currentFileName) {
        setRenamingFileID(null);
        setRenameValue("");
        return;
      }

      const token = await ensureAccessToken();
      if (!token) {
        toast.error(t("toasts.sessionExpired"), { description: t("toasts.renameAfterLogin") });
        return;
      }

      const previousFiles = filesRef.current;
      const optimisticFile = previousFiles.find((item) => item.fileID === fileID);
      if (optimisticFile) {
        const nextFiles = patchByID(previousFiles, fileID, (item) => item.fileID, { fileName: nextFileName });
        filesRef.current = nextFiles;
        setFiles(nextFiles);
      }
      try {
        const updated = await renameFile(token, fileID, nextFileName);
        setFiles((current) => {
          const next = replaceByID(current, fileID, (item) => item.fileID, updated);
          filesRef.current = next;
          return next;
        });
        toast.success(t("toasts.renameSucceeded"));
      } catch (error) {
        if (optimisticFile) {
          setFiles((current) => {
            const restored = replaceByID(current, fileID, (item) => item.fileID, optimisticFile);
            filesRef.current = restored;
            return restored;
          });
        }
        const description = resolveErrorMessage(error, t("toasts.renameFailed"));
        toast.error(t("toasts.renameFailed"), { description });
      } finally {
        setRenamingFileID(null);
        setRenameValue("");
      }
    },
    [ensureAccessToken, renameValue, resolveErrorMessage, t],
  );

  const onLoadMore = React.useCallback(async () => {
    if (loading || syncing || loadingMore || uploading || !hasMore) {
      return;
    }
    await loadFiles({ page: nextPage, append: true, silent: true });
  }, [hasMore, loadFiles, loading, loadingMore, nextPage, syncing, uploading]);

  const onFilterToggle = React.useCallback((value: FileFilterValue | "all") => {
    setFilterKeys((current) => {
      if (value === "all") {
        return [];
      }

      if (current.includes(value)) {
        return current.filter((item) => item !== value);
      }

      return [...current, value];
    });
  }, []);

  const onSelectFile = React.useCallback((fileID: string) => {
    setSelectedFileID(fileID);
    setContentTab("preview");
    setMobileView("detail");
  }, []);

  const onToggleSidebarCollapsed = React.useCallback(() => {
    setIsSidebarCollapsed((current) => !current);
  }, []);

  const onToggleSearch = React.useCallback(() => {
    setIsSearchOpen((current) => {
      if (current) {
        setQuery("");
      }
      return !current;
    });
  }, []);

  const onRenameStart = React.useCallback((item: FileObjectDTO) => {
    setRenamingFileID(item.fileID);
    setRenameValue(item.fileName);
  }, []);

  const onRenameCancel = React.useCallback(() => {
    setRenamingFileID(null);
    setRenameValue("");
  }, []);

  const onClearDeleteTarget = React.useCallback(() => {
    setDeleteTarget(null);
  }, []);

  const onConfirmDeleteTarget = React.useCallback(async () => {
    if (!deleteTarget) {
      return;
    }
    const fileID = deleteTarget.fileID;
    setDeleteTarget(null);
    await onDeleteFile(fileID);
  }, [deleteTarget, onDeleteFile]);

  const onBackToList = React.useCallback(() => {
    setMobileView("list");
  }, []);

  const onToggleRagOptOut = React.useCallback(
    async (fileID: string, current: boolean) => {
      const next = !current;
      const previousFiles = filesRef.current;
      const previousFile = previousFiles.find((item) => item.fileID === fileID) ?? null;
      const nextFiles = patchByID(previousFiles, fileID, (item) => item.fileID, { ragOptOut: next });
      filesRef.current = nextFiles;
      setFiles(nextFiles);

      const token = await ensureAccessToken();
      if (!token) {
        if (previousFile) {
          setFiles((items) => {
            const restored = replaceByID(items, fileID, (item) => item.fileID, previousFile);
            filesRef.current = restored;
            return restored;
          });
        }
        toast.error(t("toasts.sessionExpired"), { description: t("toasts.operateAfterLogin") });
        return;
      }

      try {
        const updated = await updateFileRagOptOut(token, fileID, next);
        setFiles((prev) => {
          const reconciled = replaceByID(prev, fileID, (item) => item.fileID, updated);
          filesRef.current = reconciled;
          return reconciled;
        });
        toast.success(next ? t("rag.disabledToast") : t("rag.enabledToast"));
      } catch (error) {
        if (previousFile) {
          setFiles((items) => {
            const restored = replaceByID(items, fileID, (item) => item.fileID, previousFile);
            filesRef.current = restored;
            return restored;
          });
        }
        const description = resolveErrorMessage(error, t("toasts.settingFailed"));
        toast.error(t("toasts.settingFailed"), { description });
      }
    },
    [ensureAccessToken, resolveErrorMessage, t],
  );

  return {
    fileInputRef,
    mobileView,
    files,
    total,
    selectedFile,
    selectedFileID,
    quota,
    loading,
    syncing,
    loadingMore,
    uploading,
    deletingFileID,
    selectedFileIDs,
    bulkDeleteOpen,
    bulkDeleting,
    vectorizing,
    vectorizingFileIDs,
    hasMore,
    query,
    sortKey,
    filterKeys,
    isSidebarCollapsed,
    isSearchOpen,
    renamingFileID,
    renameValue,
    deleteTarget,
    preview,
    extract,
    contentTab,
    openPreview: open,
    downloadPreview: download,
    onContentTabChange: setContentTab,
    onOpenUploadPicker,
    onFilesPicked,
    onLoadMore,
    onSelectFile,
    onToggleSidebarCollapsed,
    onToggleSearch,
    onQueryChange: setQuery,
    onFilterToggle,
    onSortChange: setSortKey,
    onRenameStart,
    onRenameValueChange: setRenameValue,
    onRenameCommit,
    onRenameCancel,
    onDeleteRequest: setDeleteTarget,
    onClearDeleteTarget,
    onConfirmDeleteTarget,
    onToggleFileSelection,
    onSelectLoadedFiles,
    onClearFileSelection,
    onBulkDeleteRequest,
    onClearBulkDelete,
    onConfirmBulkDelete,
    onVectorizeFile,
    onVectorizeSelected,
    onBackToList,
    onToggleRagOptOut,
  };
}
