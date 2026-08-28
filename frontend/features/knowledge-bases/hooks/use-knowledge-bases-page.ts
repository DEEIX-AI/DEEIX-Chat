"use client";

import { useTranslations } from "next-intl";
import * as React from "react";
import { toast } from "sonner";

import type {
  KnowledgeBaseDraft,
  KnowledgeBaseMode,
  KnowledgeBaseMobileView,
  KnowledgeBasePreviewTarget,
  KnowledgeBaseSortKey,
} from "@/features/knowledge-bases/types/knowledge-bases";
import { useLocalizedErrorMessage } from "@/i18n/use-localized-error";
import { uploadFile } from "@/shared/api/file";
import {
  addAdminKnowledgeBaseFiles,
  addMyKnowledgeBaseFiles,
  createAdminKnowledgeBase,
  createMyKnowledgeBase,
  deleteAdminKnowledgeBaseFile,
  deleteAdminKnowledgeBase,
  deleteMyKnowledgeBase,
  fetchKnowledgeBaseFileContent,
  getKnowledgeBase,
  getKnowledgeBaseFileProcessingSnapshot,
  listAdminKnowledgeBaseFiles,
  listAdminKnowledgeBases,
  listAvailableAdminKnowledgeBaseFiles,
  listAvailableMyKnowledgeBaseFiles,
  listKnowledgeBaseFiles,
  listVisibleKnowledgeBases,
  removeAdminKnowledgeBaseFile,
  removeMyKnowledgeBaseFile,
  updateAdminKnowledgeBase,
  updateMyKnowledgeBase,
  uploadAdminKnowledgeBaseFile,
} from "@/shared/api/knowledge-bases";
import type {
  KnowledgeBaseDTO,
  KnowledgeBaseFileDTO,
  KnowledgeBaseFileProcessingSnapshotDTO,
  KnowledgeBaseFileProcessingStatusDTO,
} from "@/shared/api/knowledge-bases.types";
import { resolveAccessToken } from "@/shared/auth/resolve-access-token";
import type { PreviewDialogFile } from "@/shared/components/file-preview/preview-dialog";
import {
  dispatchKnowledgeBaseInvalidated,
  subscribeKnowledgeBaseInvalidated,
} from "@/shared/events/knowledge-base-events";
import { useDialogSnapshot } from "@/shared/hooks/use-dialog-snapshot";
import {
  useFileStatusPolling,
  type FileStatusPollingResult,
} from "@/shared/hooks/use-file-processing-status-polling";
import { runSettledBulkItems, runSettledItemsWithConcurrency } from "@/shared/lib/bulk-action";
import { isFileProcessing } from "@/shared/lib/file-processing";

const FILE_ACTION_LIMIT = 100;
const FILE_PAGE_SIZE = 100;
const AVAILABLE_FILE_PAGE_SIZE = 50;
const AVAILABLE_FILE_SEARCH_DEBOUNCE_MS = 200;

type KnowledgeFileUploadResult = { file: { fileID: string } };

export function useKnowledgeBasesPage(mode: KnowledgeBaseMode) {
  const t = useTranslations("knowledgeBases");
  const resolveErrorMessage = useLocalizedErrorMessage();
  const [items, setItems] = React.useState<KnowledgeBaseDTO[]>([]);
  const [selectedID, setSelectedID] = React.useState("");
  const [selectedKnowledgeBaseIDs, setSelectedKnowledgeBaseIDs] = React.useState<string[]>([]);
  const [sortKey, setSortKey] = React.useState<KnowledgeBaseSortKey>("default");
  const [query, setQuery] = React.useState("");
  const [listQuery, setListQuery] = React.useState("");
  const [searchOpen, setSearchOpen] = React.useState(false);
  const [sidebarCollapsed, setSidebarCollapsed] = React.useState(false);
  const [mobileView, setMobileView] = React.useState<KnowledgeBaseMobileView>("list");
  const [files, setFiles] = React.useState<KnowledgeBaseFileDTO[]>([]);
  const [filesTotal, setFilesTotal] = React.useState(0);
  const [filesPage, setFilesPage] = React.useState(1);
  const [loading, setLoading] = React.useState(true);
  const [itemsTotal, setItemsTotal] = React.useState(0);
  const [itemsPage, setItemsPage] = React.useState(1);
  const [itemsLoadingMore, setItemsLoadingMore] = React.useState(false);
  const [filesLoading, setFilesLoading] = React.useState(false);
  const [filesLoadingMore, setFilesLoadingMore] = React.useState(false);
  const [removingFileID, setRemovingFileID] = React.useState("");
  const [draft, setDraft] = React.useState<KnowledgeBaseDraft | null>(null);
  const [saving, setSaving] = React.useState(false);
  const [toggling, setToggling] = React.useState(false);
  const [addFilesOpen, setAddFilesOpen] = React.useState(false);
  const [availableFiles, setAvailableFiles] = React.useState<KnowledgeBaseFileDTO[]>([]);
  const [availableFilesTotal, setAvailableFilesTotal] = React.useState(0);
  const [availableFilesPage, setAvailableFilesPage] = React.useState(1);
  const [availableFilesLoading, setAvailableFilesLoading] = React.useState(false);
  const [availableFilesLoadingMore, setAvailableFilesLoadingMore] = React.useState(false);
  const [fileQuery, setFileQuery] = React.useState("");
  const [selectedFileIDs, setSelectedFileIDs] = React.useState<string[]>([]);
  const [addingFiles, setAddingFiles] = React.useState(false);
  const [uploadingFiles, setUploadingFiles] = React.useState(false);
  const [deletingPlatformFileID, setDeletingPlatformFileID] = React.useState("");
  const [deleteTarget, setDeleteTarget] = React.useState<KnowledgeBaseDTO | null>(null);
  const [deleteFiles, setDeleteFiles] = React.useState(false);
  const [deleting, setDeleting] = React.useState(false);
  const [bulkDeleteOpen, setBulkDeleteOpen] = React.useState(false);
  const [bulkDeleteFiles, setBulkDeleteFiles] = React.useState(false);
  const [bulkDeleting, setBulkDeleting] = React.useState(false);
  const [previewTarget, setPreviewTarget] = React.useState<KnowledgeBasePreviewTarget | null>(null);

  // 选中知识库变化时在渲染期同步复位文件列表,使切换后的首帧即为加载态。
  const [filesSelectedID, setFilesSelectedID] = React.useState(selectedID);
  if (filesSelectedID !== selectedID) {
    setFilesSelectedID(selectedID);
    setFiles([]);
    setFilesTotal(0);
    setFilesPage(1);
    setFilesLoading(Boolean(selectedID));
    setFilesLoadingMore(false);
  }

  const itemsRequestVersionRef = React.useRef(0);
  const filesRequestVersionRef = React.useRef(0);
  const availableFilesRequestVersionRef = React.useRef(0);
  const itemsRequestControllerRef = React.useRef<AbortController | null>(null);
  const filesRequestControllerRef = React.useRef<AbortController | null>(null);
  const availableFilesRequestControllerRef = React.useRef<AbortController | null>(null);
  const uploadRequestControllerRef = React.useRef<AbortController | null>(null);
  const itemsPageRef = React.useRef(itemsPage);
  itemsPageRef.current = itemsPage;
  const filesRef = React.useRef(files);
  filesRef.current = files;
  const selectedIDRef = React.useRef(selectedID);
  selectedIDRef.current = selectedID;
  const previewSnapshot = useDialogSnapshot(previewTarget);

  React.useEffect(() => () => {
    itemsRequestControllerRef.current?.abort();
    filesRequestControllerRef.current?.abort();
    availableFilesRequestControllerRef.current?.abort();
    uploadRequestControllerRef.current?.abort();
  }, []);

  const selected = React.useMemo(
    () => items.find((item) => item.publicID === selectedID) ?? null,
    [items, selectedID],
  );
  const selectedProcessingFileCount = selected?.processingFileCount ?? 0;

  const selectableItems = React.useMemo(
    () => items.filter((item) => mode === "admin" || item.scope === "user"),
    [items, mode],
  );

  React.useEffect(() => {
    const timer = window.setTimeout(() => setListQuery(query.trim()), 200);
    return () => window.clearTimeout(timer);
  }, [query]);

  React.useEffect(() => {
    const selectableIDs = new Set(selectableItems.map((item) => item.publicID));
    setSelectedKnowledgeBaseIDs((current) => current.filter((id) => selectableIDs.has(id)));
  }, [selectableItems]);

  const listFilePage = React.useCallback((
    accessToken: string,
    knowledgeBaseID: string,
    page: number,
    signal?: AbortSignal,
  ) => mode === "admin"
    ? listAdminKnowledgeBaseFiles(accessToken, knowledgeBaseID, page, FILE_PAGE_SIZE, signal)
    : listKnowledgeBaseFiles(accessToken, knowledgeBaseID, page, FILE_PAGE_SIZE, signal), [mode]);

  const replaceFileList = React.useCallback(async (
    accessToken: string,
    knowledgeBaseID: string,
  ) => {
    const requestVersion = ++filesRequestVersionRef.current;
    filesRequestControllerRef.current?.abort();
    const requestController = new AbortController();
    filesRequestControllerRef.current = requestController;
    setFilesLoadingMore(false);
    try {
      const page = await listFilePage(accessToken, knowledgeBaseID, 1, requestController.signal);
      if (
        selectedIDRef.current !== knowledgeBaseID ||
        filesRequestVersionRef.current !== requestVersion
      ) return;
      setFiles(page.results);
      setFilesTotal(page.total);
      setFilesPage(1);
    } catch (error) {
      if (!requestController.signal.aborted) {
        throw error;
      }
    } finally {
      if (filesRequestControllerRef.current === requestController) {
        filesRequestControllerRef.current = null;
      }
      if (
        selectedIDRef.current === knowledgeBaseID &&
        filesRequestVersionRef.current === requestVersion
      ) setFilesLoading(false);
    }
  }, [listFilePage]);

  const loadPreviewContent = React.useCallback(async (file: PreviewDialogFile, signal: AbortSignal) => {
    if (!previewSnapshot) throw new Error("missing knowledge base preview target");
    const token = await requireAccessToken();
    return fetchKnowledgeBaseFileContent(
      token,
      previewSnapshot.knowledgeBaseID,
      file.fileID,
      previewSnapshot.admin,
      signal,
    );
  }, [previewSnapshot]);

  React.useEffect(() => {
    if (!loading && !selected) setMobileView("list");
  }, [loading, selected]);

  const loadItems = React.useCallback(async (preferredID?: string, silent = false) => {
    const requestVersion = ++itemsRequestVersionRef.current;
    itemsRequestControllerRef.current?.abort();
    const requestController = new AbortController();
    itemsRequestControllerRef.current = requestController;
    const targetID = preferredID || (silent ? selectedIDRef.current : "");
    setItemsLoadingMore(false);
    if (!silent) setLoading(true);
    try {
      const token = await requireAccessToken();
      const page = mode === "admin"
        ? await listAdminKnowledgeBases(token, {
            page: 1, pageSize: 50, query: listQuery, sort: sortKey,
          }, requestController.signal)
        : await listVisibleKnowledgeBases(token, {
            page: 1, pageSize: 50, query: listQuery, sort: sortKey,
          }, requestController.signal);
      if (itemsRequestVersionRef.current !== requestVersion) return;
      let results = page.results;
      if (targetID && !results.some((item) => item.publicID === targetID)) {
        try {
          const preferred = await getKnowledgeBase(token, targetID, mode === "admin", requestController.signal);
          if (itemsRequestVersionRef.current !== requestVersion) return;
          results = [preferred, ...results];
        } catch {
          if (requestController.signal.aborted) return;
          // The selected item may have been deleted between the mutation and refresh.
        }
      }
      setItems(results);
      setItemsTotal(page.total);
      setItemsPage(1);
      setSelectedID((current) => {
        const next = targetID || current;
        return results.some((item) => item.publicID === next) ? next : (results[0]?.publicID ?? "");
      });
    } catch {
      if (!requestController.signal.aborted && !silent && itemsRequestVersionRef.current === requestVersion) {
        toast.error(t("loadFailed"));
      }
    } finally {
      if (itemsRequestControllerRef.current === requestController) {
        itemsRequestControllerRef.current = null;
      }
      if (itemsRequestVersionRef.current === requestVersion) setLoading(false);
    }
  }, [listQuery, mode, sortKey, t]);

  const loadMoreItems = React.useCallback(async () => {
    if (itemsLoadingMore || items.length >= itemsTotal) return;
    const requestVersion = itemsRequestVersionRef.current;
    itemsRequestControllerRef.current?.abort();
    const requestController = new AbortController();
    itemsRequestControllerRef.current = requestController;
    setItemsLoadingMore(true);
    try {
      const token = await requireAccessToken();
      const nextPage = itemsPage + 1;
      const page = mode === "admin"
        ? await listAdminKnowledgeBases(token, {
            page: nextPage, pageSize: 50, query: listQuery, sort: sortKey,
          }, requestController.signal)
        : await listVisibleKnowledgeBases(token, {
            page: nextPage, pageSize: 50, query: listQuery, sort: sortKey,
          }, requestController.signal);
      if (itemsRequestVersionRef.current !== requestVersion) return;
      setItems((current) => {
        const existing = new Set(current.map((item) => item.publicID));
        return [...current, ...page.results.filter((item) => !existing.has(item.publicID))];
      });
      setItemsTotal(page.total);
      setItemsPage(nextPage);
    } catch {
      if (!requestController.signal.aborted && itemsRequestVersionRef.current === requestVersion) {
        toast.error(t("loadFailed"));
      }
    } finally {
      if (itemsRequestControllerRef.current === requestController) {
        itemsRequestControllerRef.current = null;
      }
      if (itemsRequestVersionRef.current === requestVersion) setItemsLoadingMore(false);
    }
  }, [items.length, itemsLoadingMore, itemsPage, itemsTotal, listQuery, mode, sortKey, t]);

  const refreshLoadedItems = React.useCallback(async () => {
    const requestVersion = ++itemsRequestVersionRef.current;
    itemsRequestControllerRef.current?.abort();
    const requestController = new AbortController();
    itemsRequestControllerRef.current = requestController;
    const loadedPages = Math.max(1, itemsPageRef.current);
    const targetCount = loadedPages * 50;
    setItemsLoadingMore(false);
    try {
      const token = await requireAccessToken();
      const merged: KnowledgeBaseDTO[] = [];
      const seen = new Set<string>();
      let total = 0;
      for (let pageNumber = 1; pageNumber <= Math.ceil(targetCount / 100); pageNumber += 1) {
        const page = mode === "admin"
          ? await listAdminKnowledgeBases(token, {
              page: pageNumber, pageSize: 100, query: listQuery, sort: sortKey,
            }, requestController.signal)
          : await listVisibleKnowledgeBases(token, {
              page: pageNumber, pageSize: 100, query: listQuery, sort: sortKey,
            }, requestController.signal);
        if (itemsRequestVersionRef.current !== requestVersion) return;
        total = page.total;
        for (const item of page.results) {
          if (!seen.has(item.publicID)) {
            seen.add(item.publicID);
            merged.push(item);
          }
        }
        if (merged.length >= total) break;
      }

      let results = merged.slice(0, Math.min(targetCount, total));
      const targetID = selectedIDRef.current;
      if (targetID && !results.some((item) => item.publicID === targetID)) {
        try {
          const preferred = await getKnowledgeBase(token, targetID, mode === "admin", requestController.signal);
          if (itemsRequestVersionRef.current !== requestVersion) return;
          results = [preferred, ...results];
        } catch {
          if (requestController.signal.aborted) return;
          // The selected item may have been deleted while the page was inactive.
        }
      }

      setItems(results);
      setItemsTotal(total);
      setItemsPage(Math.max(1, Math.min(loadedPages, Math.ceil(total / 50))));
      setSelectedID((current) =>
        results.some((item) => item.publicID === current) ? current : (results[0]?.publicID ?? ""),
      );
    } catch {
      // Focus refresh is best-effort; keep the current list on transient failures.
    } finally {
      if (itemsRequestControllerRef.current === requestController) {
        itemsRequestControllerRef.current = null;
      }
      if (itemsRequestVersionRef.current === requestVersion) setLoading(false);
    }
  }, [listQuery, mode, sortKey]);

  React.useEffect(() => {
    void loadItems();
  }, [loadItems]);

  React.useEffect(() => {
    let lastRefreshAt = 0;
    const refresh = () => {
      const now = Date.now();
      if (now - lastRefreshAt < 250) {
        return;
      }
      lastRefreshAt = now;
      void refreshLoadedItems();
    };
    const refreshWhenVisible = () => {
      if (!document.hidden) refresh();
    };
    const unsubscribe = subscribeKnowledgeBaseInvalidated(refresh);
    window.addEventListener("focus", refresh);
    document.addEventListener("visibilitychange", refreshWhenVisible);
    return () => {
      unsubscribe();
      window.removeEventListener("focus", refresh);
      document.removeEventListener("visibilitychange", refreshWhenVisible);
    };
  }, [refreshLoadedItems]);

  React.useEffect(() => {
    if (!selectedID) {
      filesRequestControllerRef.current?.abort();
      filesRequestControllerRef.current = null;
      filesRequestVersionRef.current += 1;
      setFiles([]);
      setFilesTotal(0);
      setFilesPage(1);
      setFilesLoading(false);
      setFilesLoadingMore(false);
      return;
    }
    let cancelled = false;
    const requestVersion = ++filesRequestVersionRef.current;
    filesRequestControllerRef.current?.abort();
    const requestController = new AbortController();
    filesRequestControllerRef.current = requestController;
    setFilesLoading(true);
    setFilesLoadingMore(false);
    void (async () => {
      try {
        const token = await requireAccessToken();
        const page = await listFilePage(token, selectedID, 1, requestController.signal);
        if (
          !cancelled &&
          selectedIDRef.current === selectedID &&
          filesRequestVersionRef.current === requestVersion
        ) {
          setFiles(page.results);
          setFilesTotal(page.total);
          setFilesPage(1);
        }
      } catch {
        if (
          !requestController.signal.aborted &&
          !cancelled &&
          selectedIDRef.current === selectedID &&
          filesRequestVersionRef.current === requestVersion
        ) toast.error(t("filesLoadFailed"));
      } finally {
        if (filesRequestControllerRef.current === requestController) {
          filesRequestControllerRef.current = null;
        }
        if (
          !cancelled &&
          selectedIDRef.current === selectedID &&
          filesRequestVersionRef.current === requestVersion
        ) setFilesLoading(false);
      }
    })();
    return () => {
      cancelled = true;
      requestController.abort();
    };
  }, [listFilePage, selectedID, t]);

  const processingFileIDs = React.useMemo(
    () => files.filter(isFileProcessing).map((file) => file.fileID),
    [files],
  );

  const loadProcessingStatuses = React.useCallback(
    (accessToken: string, fileIDs: string[], signal: AbortSignal) =>
      getKnowledgeBaseFileProcessingSnapshot(accessToken, selectedID, fileIDs, mode === "admin", signal),
    [mode, selectedID],
  );
  const selectProcessingStatuses = React.useCallback(
    (snapshot: KnowledgeBaseFileProcessingSnapshotDTO) => snapshot.statuses,
    [],
  );

  const onProcessingResult = React.useCallback(({
    statuses,
    missingFileIDs,
    snapshot,
  }: FileStatusPollingResult<KnowledgeBaseFileProcessingStatusDTO, KnowledgeBaseFileProcessingSnapshotDTO>) => {
    const statusesByID = new Map(statuses.map((status) => [status.fileID, status]));
    const missingFileIDSet = new Set(missingFileIDs);
    const currentFiles = filesRef.current;
    let filesChanged = false;
    const nextFiles: KnowledgeBaseFileDTO[] = [];
    for (const file of currentFiles) {
      if (missingFileIDSet.has(file.fileID)) {
        filesChanged = true;
        continue;
      }
      const status = statusesByID.get(file.fileID);
      if (!status) {
        nextFiles.push(file);
        continue;
      }
      if (
        file.detectedMIME === status.detectedMIME &&
        file.fileCategory === status.fileCategory &&
        file.processingStatus === status.processingStatus &&
        file.processing === status.processing &&
        file.processingReady === status.processingReady &&
        file.embedStatus === status.embedStatus &&
        file.chunkCount === status.chunkCount &&
        file.ragOptOut === status.ragOptOut &&
        file.updatedAt === status.updatedAt
      ) {
        nextFiles.push(file);
        continue;
      }
      const nextFile = { ...file, ...status };
      filesChanged = true;
      nextFiles.push(nextFile);
    }

    if (filesChanged) {
      filesRef.current = nextFiles;
      setFiles(nextFiles);
    }
    const latest = snapshot.knowledgeBase;
    const summaryChanged = !selected ||
      selected.revision !== latest.revision ||
      selected.fileCount !== latest.fileCount ||
      selected.readyFileCount !== latest.readyFileCount ||
      selected.processingFileCount !== latest.processingFileCount;
    setFilesTotal((current) => current === latest.fileCount ? current : latest.fileCount);
    if (summaryChanged) {
      setItems((current) => current.map((item) => item.publicID === selectedID ? latest : item));
      dispatchKnowledgeBaseInvalidated(selectedID);
    }
  }, [selected, selectedID]);

  useFileStatusPolling({
    fileIDs: !selectedID || filesLoading || filesLoadingMore ? [] : processingFileIDs,
    intervalMs: 2500,
    enabled: Boolean(
      selectedID &&
      !filesLoading &&
      !filesLoadingMore &&
      (selectedProcessingFileCount > 0 || processingFileIDs.length > 0),
    ),
    loadStatuses: loadProcessingStatuses,
    selectStatuses: selectProcessingStatuses,
    onResult: onProcessingResult,
  });

  React.useEffect(() => {
    if (!addFilesOpen || !selected) {
      availableFilesRequestControllerRef.current?.abort();
      availableFilesRequestControllerRef.current = null;
      return;
    }
    let cancelled = false;
    const requestVersion = ++availableFilesRequestVersionRef.current;
    availableFilesRequestControllerRef.current?.abort();
    const requestController = new AbortController();
    availableFilesRequestControllerRef.current = requestController;
    setAvailableFilesLoading(true);
    setAvailableFilesLoadingMore(false);
    const timer = window.setTimeout((): void => void (async () => {
      try {
        const token = await requireAccessToken();
        const result = mode === "admin"
          ? await listAvailableAdminKnowledgeBaseFiles(token, selected.publicID, {
              page: 1, pageSize: AVAILABLE_FILE_PAGE_SIZE, query: fileQuery,
            }, requestController.signal)
          : await listAvailableMyKnowledgeBaseFiles(token, selected.publicID, {
              page: 1, pageSize: AVAILABLE_FILE_PAGE_SIZE, query: fileQuery,
            }, requestController.signal);
        if (!cancelled && availableFilesRequestVersionRef.current === requestVersion) {
          setAvailableFiles(result.results);
          setAvailableFilesTotal(result.total);
          setAvailableFilesPage(1);
        }
      } catch {
        if (
          !requestController.signal.aborted &&
          !cancelled &&
          availableFilesRequestVersionRef.current === requestVersion
        ) toast.error(t("filesLoadFailed"));
      } finally {
        if (availableFilesRequestControllerRef.current === requestController) {
          availableFilesRequestControllerRef.current = null;
        }
        if (!cancelled && availableFilesRequestVersionRef.current === requestVersion) setAvailableFilesLoading(false);
      }
    })(), AVAILABLE_FILE_SEARCH_DEBOUNCE_MS);
    return () => {
      cancelled = true;
      requestController.abort();
      window.clearTimeout(timer);
      if (availableFilesRequestVersionRef.current === requestVersion) {
        availableFilesRequestVersionRef.current += 1;
      }
    };
  }, [addFilesOpen, fileQuery, mode, selected, t]);

  const loadMoreAvailableFiles = React.useCallback(async () => {
    if (!selected || availableFilesLoadingMore || availableFilesPage * AVAILABLE_FILE_PAGE_SIZE >= availableFilesTotal) return;
    availableFilesRequestControllerRef.current?.abort();
    const requestController = new AbortController();
    availableFilesRequestControllerRef.current = requestController;
    setAvailableFilesLoadingMore(true);
    const requestVersion = availableFilesRequestVersionRef.current;
    try {
      const token = await requireAccessToken();
      const nextPage = availableFilesPage + 1;
      const result = mode === "admin"
        ? await listAvailableAdminKnowledgeBaseFiles(token, selected.publicID, {
            page: nextPage, pageSize: AVAILABLE_FILE_PAGE_SIZE, query: fileQuery,
          }, requestController.signal)
        : await listAvailableMyKnowledgeBaseFiles(token, selected.publicID, {
            page: nextPage, pageSize: AVAILABLE_FILE_PAGE_SIZE, query: fileQuery,
          }, requestController.signal);
      if (availableFilesRequestVersionRef.current !== requestVersion) return;
      setAvailableFiles((current) => {
        const seen = new Set(current.map((file) => file.fileID));
        return [...current, ...result.results.filter((file) => !seen.has(file.fileID))];
      });
      setAvailableFilesTotal(result.total);
      setAvailableFilesPage(nextPage);
    } catch {
      if (!requestController.signal.aborted && availableFilesRequestVersionRef.current === requestVersion) {
        toast.error(t("filesLoadFailed"));
      }
    } finally {
      if (availableFilesRequestControllerRef.current === requestController) {
        availableFilesRequestControllerRef.current = null;
      }
      if (availableFilesRequestVersionRef.current === requestVersion) setAvailableFilesLoadingMore(false);
    }
  }, [availableFilesLoadingMore, availableFilesPage, availableFilesTotal, fileQuery, mode, selected, t]);

  const saveDraft = React.useCallback(async () => {
    const name = draft?.name.trim() ?? "";
    if (!draft || !name || saving) return;
    const creating = !draft.publicID;
    setSaving(true);
    try {
      const token = await requireAccessToken();
      const payload = { name, description: draft.description.trim() };
      const result = draft.publicID
        ? mode === "admin"
          ? await updateAdminKnowledgeBase(token, draft.publicID, payload)
          : await updateMyKnowledgeBase(token, draft.publicID, payload)
        : mode === "admin"
          ? await createAdminKnowledgeBase(token, { ...payload, enabled: true })
          : await createMyKnowledgeBase(token, payload);
      setDraft(null);
      await loadItems(result.knowledgeBase.publicID);
      dispatchKnowledgeBaseInvalidated(result.knowledgeBase.publicID);
      if (creating) setMobileView("detail");
      toast.success(t("saved"));
    } catch (error) {
      toast.error(t("saveFailed"), { description: resolveErrorMessage(error) });
    } finally {
      setSaving(false);
    }
  }, [draft, loadItems, mode, resolveErrorMessage, saving, t]);

  const confirmAddFiles = React.useCallback(async () => {
    if (!selected || selectedFileIDs.length === 0 || addingFiles) return;
    if (selectedFileIDs.length > FILE_ACTION_LIMIT) {
      toast.error(t("tooManyFiles", { max: FILE_ACTION_LIMIT }));
      return;
    }
    setAddingFiles(true);
    try {
      const token = await requireAccessToken();
      if (mode === "admin") {
        await addAdminKnowledgeBaseFiles(token, selected.publicID, { fileIDs: selectedFileIDs });
      } else {
        await addMyKnowledgeBaseFiles(token, selected.publicID, { fileIDs: selectedFileIDs });
      }
      setAddFilesOpen(false);
      setSelectedFileIDs([]);
      await replaceFileList(token, selected.publicID);
      await loadItems(undefined, true);
      dispatchKnowledgeBaseInvalidated(selected.publicID);
      toast.success(t("added"));
    } catch {
      toast.error(t("addFailed"));
    } finally {
      setAddingFiles(false);
    }
  }, [addingFiles, loadItems, mode, replaceFileList, selected, selectedFileIDs, t]);

  const uploadAndAddFiles = React.useCallback(async (nextFiles: File[]) => {
    if (!selected || nextFiles.length === 0 || uploadingFiles || addingFiles) return;
    if (nextFiles.length > FILE_ACTION_LIMIT) {
      toast.error(t("tooManyFiles", { max: FILE_ACTION_LIMIT }));
      return;
    }
    setUploadingFiles(true);
    uploadRequestControllerRef.current?.abort();
    const requestController = new AbortController();
    uploadRequestControllerRef.current = requestController;
    let token = "";
    let fileIDs: string[] = [];
    let failedCount = 0;
    try {
      token = await requireAccessToken();
      if (requestController.signal.aborted) return;
      const upload: (
        accessToken: string,
        file: File,
        signal: AbortSignal,
      ) => Promise<KnowledgeFileUploadResult> = mode === "admin"
        ? (accessToken: string, file: File, signal: AbortSignal) =>
            uploadAdminKnowledgeBaseFile(accessToken, file, signal)
        : (accessToken: string, file: File, signal: AbortSignal) =>
            uploadFile(accessToken, file, { signal });
      const uploadResults = await runSettledItemsWithConcurrency({
        items: nextFiles,
        signal: requestController.signal,
        runItem: (file) => upload(token, file, requestController.signal),
      });
      if (requestController.signal.aborted) return;
      const uploaded = uploadResults.flatMap((result) =>
        result.status === "fulfilled" ? [result.value] : []);
      const failed = uploadResults.length - uploaded.length;
      fileIDs = Array.from(new Set(uploaded.map((result) => result.file.fileID)));
      failedCount = failed;
      if (fileIDs.length === 0) {
        toast.error(t("uploadFailed"));
        return;
      }
    } catch (error) {
      if (requestController.signal.aborted) return;
      toast.error(t("uploadFailed"), { description: resolveErrorMessage(error) });
      return;
    } finally {
      if (
        fileIDs.length === 0 &&
        uploadRequestControllerRef.current === requestController
      ) {
        uploadRequestControllerRef.current = null;
        if (!requestController.signal.aborted) setUploadingFiles(false);
      }
    }

    try {
      if (mode === "admin") {
        await addAdminKnowledgeBaseFiles(token, selected.publicID, { fileIDs });
      } else {
        await addMyKnowledgeBaseFiles(token, selected.publicID, { fileIDs });
      }
      await replaceFileList(token, selected.publicID);
      setAddFilesOpen(false);
      setSelectedFileIDs([]);
      setFileQuery("");
      await loadItems(undefined, true);
      dispatchKnowledgeBaseInvalidated(selected.publicID);
      toast.success(t("uploadedAndAdded", { count: fileIDs.length }));
      if (failedCount > 0) {
        toast.error(t("partialUploadFailed"), {
          description: t("partialUploadDescription", { success: fileIDs.length, failed: failedCount }),
        });
      }
    } catch (error) {
      toast.error(t("uploadSucceededAddFailed"), { description: resolveErrorMessage(error) });
    } finally {
      if (uploadRequestControllerRef.current === requestController) {
        uploadRequestControllerRef.current = null;
        if (!requestController.signal.aborted) setUploadingFiles(false);
      }
    }
  }, [addingFiles, loadItems, mode, replaceFileList, resolveErrorMessage, selected, t, uploadingFiles]);

  const deletePlatformFile = React.useCallback(async (fileID: string): Promise<boolean> => {
    if (mode !== "admin" || deletingPlatformFileID || addingFiles || uploadingFiles) return false;
    setDeletingPlatformFileID(fileID);
    try {
      const token = await requireAccessToken();
      await deleteAdminKnowledgeBaseFile(token, fileID);
      setAvailableFiles((current) => current.filter((file) => file.fileID !== fileID));
      setAvailableFilesTotal((current) => Math.max(0, current - 1));
      setSelectedFileIDs((current) => current.filter((id) => id !== fileID));
      dispatchKnowledgeBaseInvalidated();
      toast.success(t("platformFileDeleted"));
      return true;
    } catch (error) {
      toast.error(t("platformFileDeleteFailed"), { description: resolveErrorMessage(error) });
      return false;
    } finally {
      setDeletingPlatformFileID("");
    }
  }, [addingFiles, deletingPlatformFileID, mode, resolveErrorMessage, t, uploadingFiles]);

  const removeFile = React.useCallback(async (fileID: string) => {
    if (!selected || removingFileID || (mode === "user" && selected.scope !== "user")) return;
    const knowledgeBaseID = selected.publicID;
    filesRequestVersionRef.current += 1;
    setFilesLoading(false);
    setFilesLoadingMore(false);
    setRemovingFileID(fileID);
    try {
      const token = await requireAccessToken();
      if (mode === "admin") {
        await removeAdminKnowledgeBaseFile(token, knowledgeBaseID, fileID);
      } else {
        await removeMyKnowledgeBaseFile(token, knowledgeBaseID, fileID);
      }
      if (selectedIDRef.current === knowledgeBaseID) {
        setFiles((current) => current.filter((file) => file.fileID !== fileID));
        setFilesTotal((current) => Math.max(0, current - 1));
      }
      await loadItems(undefined, true);
      dispatchKnowledgeBaseInvalidated(knowledgeBaseID);
      toast.success(t("removed"));
    } catch {
      toast.error(t("removeFailed"));
    } finally {
      setRemovingFileID("");
    }
  }, [loadItems, mode, removingFileID, selected, t]);

  const loadMoreFiles = React.useCallback(async () => {
    if (!selected || filesLoadingMore || files.length >= filesTotal) return;
    const knowledgeBaseID = selected.publicID;
    const requestVersion = filesRequestVersionRef.current;
    filesRequestControllerRef.current?.abort();
    const requestController = new AbortController();
    filesRequestControllerRef.current = requestController;
    setFilesLoadingMore(true);
    try {
      const token = await requireAccessToken();
      const nextPage = filesPage + 1;
      const page = await listFilePage(token, knowledgeBaseID, nextPage, requestController.signal);
      if (
        selectedIDRef.current !== knowledgeBaseID ||
        filesRequestVersionRef.current !== requestVersion
      ) return;
      setFiles((current) => {
        const existing = new Set(current.map((file) => file.fileID));
        return [...current, ...page.results.filter((file) => !existing.has(file.fileID))];
      });
      setFilesTotal(page.total);
      setFilesPage(nextPage);
    } catch {
      if (!requestController.signal.aborted && filesRequestVersionRef.current === requestVersion) {
        toast.error(t("filesLoadFailed"));
      }
    } finally {
      if (filesRequestControllerRef.current === requestController) {
        filesRequestControllerRef.current = null;
      }
      if (
        selectedIDRef.current === knowledgeBaseID &&
        filesRequestVersionRef.current === requestVersion
      ) setFilesLoadingMore(false);
    }
  }, [files.length, filesLoadingMore, filesPage, filesTotal, listFilePage, selected, t]);

  const confirmDelete = React.useCallback(async () => {
    if (!deleteTarget || deleting) return;
    setDeleting(true);
    try {
      const token = await requireAccessToken();
      if (mode === "admin") {
        await deleteAdminKnowledgeBase(token, deleteTarget.publicID, { deleteFiles });
      } else {
        await deleteMyKnowledgeBase(token, deleteTarget.publicID, { deleteFiles });
      }
      setDeleteTarget(null);
      setDeleteFiles(false);
      await loadItems();
      dispatchKnowledgeBaseInvalidated(deleteTarget.publicID);
      toast.success(t("deleted"));
    } catch {
      toast.error(t("deleteFailed"));
    } finally {
      setDeleting(false);
    }
  }, [deleteFiles, deleteTarget, deleting, loadItems, mode, t]);

  const toggleKnowledgeBaseSelection = React.useCallback((publicID: string, checked: boolean) => {
    setSelectedKnowledgeBaseIDs((current) => {
      const next = new Set(current);
      if (checked) next.add(publicID);
      else next.delete(publicID);
      return Array.from(next);
    });
  }, []);

  const confirmBulkDelete = React.useCallback(async () => {
    if (bulkDeleting || selectedKnowledgeBaseIDs.length === 0) return;
    const selectedIDs = new Set(selectedKnowledgeBaseIDs);
    const targets = selectableItems.filter((item) => selectedIDs.has(item.publicID));
    if (targets.length === 0) {
      setBulkDeleteOpen(false);
      setSelectedKnowledgeBaseIDs([]);
      return;
    }

    setBulkDeleting(true);
    try {
      const token = await requireAccessToken();
      const results = await runSettledBulkItems({
        chunkSize: 10,
        items: targets,
        title: t("bulkDeleting"),
        runItem: (item) => mode === "admin"
          ? deleteAdminKnowledgeBase(token, item.publicID, { deleteFiles: bulkDeleteFiles })
          : deleteMyKnowledgeBase(token, item.publicID, { deleteFiles: bulkDeleteFiles }),
      });
      const successCount = results.filter((result) => result.status === "fulfilled").length;
      const failedCount = results.length - successCount;
      setSelectedKnowledgeBaseIDs([]);
      setBulkDeleteOpen(false);
      setBulkDeleteFiles(false);
      await loadItems();
      if (successCount > 0) {
        dispatchKnowledgeBaseInvalidated();
      }
      if (failedCount > 0) {
        toast.error(t("bulkDeletePartialFailed"), {
          description: t("bulkDeletePartialDescription", { success: successCount, failed: failedCount }),
        });
      } else {
        toast.success(t("bulkDeleted", { count: successCount }));
      }
    } catch (error) {
      toast.error(t("deleteFailed"), { description: resolveErrorMessage(error) });
    } finally {
      setBulkDeleting(false);
    }
  }, [
    bulkDeleteFiles,
    bulkDeleting,
    loadItems,
    mode,
    resolveErrorMessage,
    selectableItems,
    selectedKnowledgeBaseIDs,
    t,
  ]);

  const toggleBuiltinEnabled = React.useCallback(async (enabled: boolean) => {
    if (mode !== "admin" || !selected || toggling) return;
    setToggling(true);
    try {
      const token = await requireAccessToken();
      await updateAdminKnowledgeBase(token, selected.publicID, { enabled });
      await loadItems(undefined, true);
      dispatchKnowledgeBaseInvalidated(selected.publicID);
      toast.success(t(enabled ? "enabledToast" : "disabledToast"));
    } catch {
      toast.error(t("toggleFailed"));
    } finally {
      setToggling(false);
    }
  }, [loadItems, mode, selected, t, toggling]);

  const onAddFilesOpenChange = React.useCallback((open: boolean) => {
    if (!open && (addingFiles || uploadingFiles)) return;
    setAddFilesOpen(open);
    if (open) return;
    availableFilesRequestVersionRef.current += 1;
    setAvailableFilesLoadingMore(false);
    setSelectedFileIDs([]);
    setFileQuery("");
  }, [addingFiles, uploadingFiles]);

  return {
    list: {
      items,
      loading,
      loadingMore: itemsLoadingMore,
      hasMore: items.length < itemsTotal,
      mobileView,
      selectedID,
      selectedIDs: selectedKnowledgeBaseIDs,
      sortKey,
      query,
      searchOpen,
      sidebarCollapsed,
      bulkDeleting,
      refresh: () => {
        void loadItems();
      },
      loadMore: loadMoreItems,
      create: () => setDraft({ name: "", description: "" }),
      select: (publicID: string) => {
        setSelectedID(publicID);
        setMobileView("detail");
      },
      edit: (item: KnowledgeBaseDTO) => {
        setSelectedID(item.publicID);
        setDraft({ publicID: item.publicID, name: item.name, description: item.description });
      },
      requestDelete: (item: KnowledgeBaseDTO) => {
        setDeleteFiles(false);
        setDeleteTarget(item);
      },
      toggleSelection: toggleKnowledgeBaseSelection,
      selectAll: () => setSelectedKnowledgeBaseIDs(selectableItems.map((item) => item.publicID)),
      clearSelection: () => setSelectedKnowledgeBaseIDs([]),
      changeSort: setSortKey,
      changeQuery: setQuery,
      toggleSearch: () => {
        setSearchOpen((current) => {
          if (current) setQuery("");
          else setSidebarCollapsed(false);
          return !current;
        });
      },
      toggleSidebarCollapsed: () => setSidebarCollapsed((current) => !current),
      requestBulkDelete: () => {
        if (selectedKnowledgeBaseIDs.length > 0) setBulkDeleteOpen(true);
      },
    },
    detail: {
      selected, files, filesTotal, filesLoading, filesLoadingMore, removingFileID, toggling,
      back: () => setMobileView("list"),
      addFiles: () => setAddFilesOpen(true),
      loadMoreFiles,
      removeFile,
      toggleBuiltinEnabled,
      previewFile: (file: KnowledgeBaseFileDTO) => {
        if (!selected) return;
        setPreviewTarget({ knowledgeBaseID: selected.publicID, admin: mode === "admin", file });
      },
    },
    editor: {
      draft, saving,
      change: setDraft,
      close: () => setDraft(null),
      save: saveDraft,
    },
    addFilesDialog: {
      open: addFilesOpen,
      files: availableFiles,
      loading: availableFilesLoading,
      loadingMore: availableFilesLoadingMore,
      hasMore: availableFilesPage * AVAILABLE_FILE_PAGE_SIZE < availableFilesTotal,
      query: fileQuery,
      selectedFileIDs,
      adding: addingFiles,
      uploading: uploadingFiles,
      deletingPlatformFileID,
      selectionLimit: FILE_ACTION_LIMIT,
      changeOpen: onAddFilesOpenChange,
      changeQuery: setFileQuery,
      changeSelection: setSelectedFileIDs,
      loadMore: loadMoreAvailableFiles,
      upload: uploadAndAddFiles,
      deletePlatformFile,
      confirm: confirmAddFiles,
    },
    deleteDialog: {
      target: deleteTarget,
      deleting,
      deleteFiles,
      close: () => {
        setDeleteTarget(null);
        setDeleteFiles(false);
      },
      changeDeleteFiles: setDeleteFiles,
      confirm: confirmDelete,
    },
    bulkDeleteDialog: {
      open: bulkDeleteOpen,
      count: selectedKnowledgeBaseIDs.length,
      hasFiles: selectableItems.some(
        (item) => selectedKnowledgeBaseIDs.includes(item.publicID) && item.fileCount > 0,
      ),
      deleting: bulkDeleting,
      deleteFiles: bulkDeleteFiles,
      close: () => {
        if (bulkDeleting) return;
        setBulkDeleteOpen(false);
        setBulkDeleteFiles(false);
      },
      changeDeleteFiles: setBulkDeleteFiles,
      confirm: confirmBulkDelete,
    },
    preview: {
      snapshot: previewSnapshot,
      open: previewTarget !== null,
      close: () => setPreviewTarget(null),
      loadContent: loadPreviewContent,
    },
  };
}

async function requireAccessToken(): Promise<string> {
  const token = await resolveAccessToken();
  if (!token) throw new Error("missing access token");
  return token;
}
