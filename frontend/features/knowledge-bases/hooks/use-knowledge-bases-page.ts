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
  listAdminKnowledgeBaseFiles,
  listAllAdminKnowledgeBases,
  listAllVisibleKnowledgeBases,
  listAvailableAdminKnowledgeBaseFiles,
  listAvailableMyKnowledgeBaseFiles,
  listKnowledgeBaseFiles,
  removeAdminKnowledgeBaseFile,
  removeMyKnowledgeBaseFile,
  updateAdminKnowledgeBase,
  updateMyKnowledgeBase,
  uploadAdminKnowledgeBaseFile,
} from "@/shared/api/knowledge-bases";
import type { KnowledgeBaseDTO, KnowledgeBaseFileDTO } from "@/shared/api/knowledge-bases.types";
import { resolveAccessToken } from "@/shared/auth/resolve-access-token";
import type { PreviewDialogFile } from "@/shared/components/file-preview/preview-dialog";
import { useDialogSnapshot } from "@/shared/hooks/use-dialog-snapshot";
import { runSettledBulkItems } from "@/shared/lib/bulk-action";

const FILE_ACTION_LIMIT = 100;
const FILE_PAGE_SIZE = 100;
const AVAILABLE_FILE_PAGE_SIZE = 50;
const UPLOAD_CONCURRENCY = 4;
const PROCESSING_REFRESH_INTERVAL_MS = 2500;
const AVAILABLE_FILE_SEARCH_DEBOUNCE_MS = 200;

export function useKnowledgeBasesPage(mode: KnowledgeBaseMode) {
  const t = useTranslations("knowledgeBases");
  const resolveErrorMessage = useLocalizedErrorMessage();
  const [items, setItems] = React.useState<KnowledgeBaseDTO[]>([]);
  const [selectedID, setSelectedID] = React.useState("");
  const [selectedKnowledgeBaseIDs, setSelectedKnowledgeBaseIDs] = React.useState<string[]>([]);
  const [sortKey, setSortKey] = React.useState<KnowledgeBaseSortKey>("default");
  const [query, setQuery] = React.useState("");
  const [searchOpen, setSearchOpen] = React.useState(false);
  const [sidebarCollapsed, setSidebarCollapsed] = React.useState(false);
  const [mobileView, setMobileView] = React.useState<KnowledgeBaseMobileView>("list");
  const [files, setFiles] = React.useState<KnowledgeBaseFileDTO[]>([]);
  const [filesTotal, setFilesTotal] = React.useState(0);
  const [filesPage, setFilesPage] = React.useState(1);
  const [loading, setLoading] = React.useState(true);
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
  const itemsRequestVersionRef = React.useRef(0);
  const filesRequestVersionRef = React.useRef(0);
  const availableFilesRequestVersionRef = React.useRef(0);
  const selectedIDRef = React.useRef(selectedID);
  selectedIDRef.current = selectedID;
  const previewSnapshot = useDialogSnapshot(previewTarget);

  const selected = React.useMemo(
    () => items.find((item) => item.publicID === selectedID) ?? null,
    [items, selectedID],
  );

  const visibleItems = React.useMemo(() => {
    const normalizedQuery = query.trim().toLocaleLowerCase();
    const filteredItems = normalizedQuery
      ? items.filter((item) =>
          `${item.name}\n${item.description}`.toLocaleLowerCase().includes(normalizedQuery))
      : items;
    return sortKnowledgeBases(filteredItems, sortKey);
  }, [items, query, sortKey]);
  const selectableItems = React.useMemo(
    () => visibleItems.filter((item) => mode === "admin" || item.scope === "user"),
    [mode, visibleItems],
  );

  React.useEffect(() => {
    const selectableIDs = new Set(selectableItems.map((item) => item.publicID));
    setSelectedKnowledgeBaseIDs((current) => current.filter((id) => selectableIDs.has(id)));
  }, [selectableItems]);

  const listFilePage = React.useCallback((
    accessToken: string,
    knowledgeBaseID: string,
    page: number,
  ) => mode === "admin"
    ? listAdminKnowledgeBaseFiles(accessToken, knowledgeBaseID, page, FILE_PAGE_SIZE)
    : listKnowledgeBaseFiles(accessToken, knowledgeBaseID, page, FILE_PAGE_SIZE), [mode]);

  const replaceFileList = React.useCallback(async (
    accessToken: string,
    knowledgeBaseID: string,
  ) => {
    const requestVersion = ++filesRequestVersionRef.current;
    setFilesLoadingMore(false);
    try {
      const page = await listFilePage(accessToken, knowledgeBaseID, 1);
      if (
        selectedIDRef.current !== knowledgeBaseID ||
        filesRequestVersionRef.current !== requestVersion
      ) return;
      setFiles(page.results);
      setFilesTotal(page.total);
      setFilesPage(1);
    } finally {
      if (
        selectedIDRef.current === knowledgeBaseID &&
        filesRequestVersionRef.current === requestVersion
      ) setFilesLoading(false);
    }
  }, [listFilePage]);

  const loadPreviewContent = React.useCallback(async (file: PreviewDialogFile) => {
    if (!previewSnapshot) throw new Error("missing knowledge base preview target");
    const token = await requireAccessToken();
    return fetchKnowledgeBaseFileContent(
      token,
      previewSnapshot.knowledgeBaseID,
      file.fileID,
      previewSnapshot.admin,
    );
  }, [previewSnapshot]);

  React.useEffect(() => {
    if (!loading && !selected) setMobileView("list");
  }, [loading, selected]);

  const loadItems = React.useCallback(async (preferredID?: string, silent = false) => {
    const requestVersion = ++itemsRequestVersionRef.current;
    try {
      const token = await requireAccessToken();
      const results = mode === "admin"
        ? await listAllAdminKnowledgeBases(token)
        : await listAllVisibleKnowledgeBases(token);
      if (itemsRequestVersionRef.current !== requestVersion) return;
      setItems(results);
      setSelectedID((current) => {
        const next = preferredID || current;
        return results.some((item) => item.publicID === next) ? next : (results[0]?.publicID ?? "");
      });
    } catch {
      if (!silent && itemsRequestVersionRef.current === requestVersion) toast.error(t("loadFailed"));
    } finally {
      if (itemsRequestVersionRef.current === requestVersion) setLoading(false);
    }
  }, [mode, t]);

  React.useEffect(() => {
    void loadItems();
  }, [loadItems]);

  React.useEffect(() => {
    if (!selectedID) {
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
    setFilesLoading(true);
    setFilesLoadingMore(false);
    void (async () => {
      try {
        const token = await requireAccessToken();
        const page = await listFilePage(token, selectedID, 1);
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
          !cancelled &&
          selectedIDRef.current === selectedID &&
          filesRequestVersionRef.current === requestVersion
        ) toast.error(t("filesLoadFailed"));
      } finally {
        if (
          !cancelled &&
          selectedIDRef.current === selectedID &&
          filesRequestVersionRef.current === requestVersion
        ) setFilesLoading(false);
      }
    })();
    return () => {
      cancelled = true;
    };
  }, [listFilePage, selectedID, t]);

  const processingFilePages = React.useMemo(
    () => Array.from(new Set(files.flatMap((file, index) => {
      const processing =
        file.processingStatus === "uploaded" ||
        file.processingStatus === "queued" ||
        file.processingStatus === "extracting" ||
        file.processingStatus === "embedding" ||
        file.embedStatus === "processing";
      return processing ? [Math.floor(index / FILE_PAGE_SIZE) + 1] : [];
    }))),
    [files],
  );

  React.useEffect(() => {
    if (!selectedID || processingFilePages.length === 0 || filesLoading || filesLoadingMore) return;
    let cancelled = false;
    let refreshing = false;
    const refresh = async () => {
      if (refreshing) return;
      refreshing = true;
      try {
        const token = await requireAccessToken();
        const pages = await Promise.all(
          processingFilePages.map((page) => listFilePage(token, selectedID, page)),
        );
        if (cancelled || selectedIDRef.current !== selectedID) return;
        const updates = new Map(pages.flatMap((page) => page.results).map((file) => [file.fileID, file]));
        setFiles((current) => current.map((file) => updates.get(file.fileID) ?? file));
        setFilesTotal(pages[0]?.total ?? 0);
        await loadItems(undefined, true);
      } catch {
        // The explicit load path owns user-visible errors; polling remains best-effort.
      } finally {
        refreshing = false;
      }
    };
    const timer = window.setInterval(() => void refresh(), PROCESSING_REFRESH_INTERVAL_MS);
    return () => {
      cancelled = true;
      window.clearInterval(timer);
    };
  }, [filesLoading, filesLoadingMore, listFilePage, loadItems, processingFilePages, selectedID]);

  React.useEffect(() => {
    if (!addFilesOpen || !selected) return;
    let cancelled = false;
    const requestVersion = ++availableFilesRequestVersionRef.current;
    setAvailableFilesLoading(true);
    setAvailableFilesLoadingMore(false);
    const timer = window.setTimeout(() => void (async () => {
      try {
        const token = await requireAccessToken();
        const result = mode === "admin"
          ? await listAvailableAdminKnowledgeBaseFiles(token, selected.publicID, {
              page: 1, pageSize: AVAILABLE_FILE_PAGE_SIZE, query: fileQuery,
            })
          : await listAvailableMyKnowledgeBaseFiles(token, selected.publicID, {
              page: 1, pageSize: AVAILABLE_FILE_PAGE_SIZE, query: fileQuery,
            });
        if (!cancelled && availableFilesRequestVersionRef.current === requestVersion) {
          setAvailableFiles(result.results);
          setAvailableFilesTotal(result.total);
          setAvailableFilesPage(1);
        }
      } catch {
        if (!cancelled && availableFilesRequestVersionRef.current === requestVersion) toast.error(t("filesLoadFailed"));
      } finally {
        if (!cancelled && availableFilesRequestVersionRef.current === requestVersion) setAvailableFilesLoading(false);
      }
    })(), AVAILABLE_FILE_SEARCH_DEBOUNCE_MS);
    return () => {
      cancelled = true;
      window.clearTimeout(timer);
      if (availableFilesRequestVersionRef.current === requestVersion) {
        availableFilesRequestVersionRef.current += 1;
      }
    };
  }, [addFilesOpen, fileQuery, mode, selected, t]);

  const loadMoreAvailableFiles = React.useCallback(async () => {
    if (!selected || availableFilesLoadingMore || availableFilesPage * AVAILABLE_FILE_PAGE_SIZE >= availableFilesTotal) return;
    setAvailableFilesLoadingMore(true);
    const requestVersion = availableFilesRequestVersionRef.current;
    try {
      const token = await requireAccessToken();
      const nextPage = availableFilesPage + 1;
      const result = mode === "admin"
        ? await listAvailableAdminKnowledgeBaseFiles(token, selected.publicID, {
            page: nextPage, pageSize: AVAILABLE_FILE_PAGE_SIZE, query: fileQuery,
          })
        : await listAvailableMyKnowledgeBaseFiles(token, selected.publicID, {
            page: nextPage, pageSize: AVAILABLE_FILE_PAGE_SIZE, query: fileQuery,
          });
      if (availableFilesRequestVersionRef.current !== requestVersion) return;
      setAvailableFiles((current) => {
        const seen = new Set(current.map((file) => file.fileID));
        return [...current, ...result.results.filter((file) => !seen.has(file.fileID))];
      });
      setAvailableFilesTotal(result.total);
      setAvailableFilesPage(nextPage);
    } catch {
      if (availableFilesRequestVersionRef.current === requestVersion) toast.error(t("filesLoadFailed"));
    } finally {
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
    let token = "";
    let fileIDs: string[] = [];
    let failedCount = 0;
    try {
      token = await requireAccessToken();
      const upload = mode === "admin" ? uploadAdminKnowledgeBaseFile : uploadFile;
      const { uploaded, failed } = await uploadFiles(token, nextFiles, upload);
      fileIDs = Array.from(new Set(uploaded.map((result) => result.file.fileID)));
      failedCount = failed;
      if (fileIDs.length === 0) {
        toast.error(t("uploadFailed"));
        return;
      }
    } catch (error) {
      toast.error(t("uploadFailed"), { description: resolveErrorMessage(error) });
      return;
    } finally {
      if (fileIDs.length === 0) setUploadingFiles(false);
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
      toast.success(t("uploadedAndAdded", { count: fileIDs.length }));
      if (failedCount > 0) {
        toast.error(t("partialUploadFailed"), {
          description: t("partialUploadDescription", { success: fileIDs.length, failed: failedCount }),
        });
      }
    } catch (error) {
      toast.error(t("uploadSucceededAddFailed"), { description: resolveErrorMessage(error) });
    } finally {
      setUploadingFiles(false);
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
    setFilesLoadingMore(true);
    try {
      const token = await requireAccessToken();
      const nextPage = filesPage + 1;
      const page = await listFilePage(token, knowledgeBaseID, nextPage);
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
      if (filesRequestVersionRef.current === requestVersion) toast.error(t("filesLoadFailed"));
    } finally {
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
      items: visibleItems,
      loading,
      mobileView,
      selectedID,
      selectedIDs: selectedKnowledgeBaseIDs,
      sortKey,
      query,
      searchOpen,
      sidebarCollapsed,
      bulkDeleting,
      refresh: () => {
        setLoading(true);
        void loadItems(undefined, true);
      },
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

function sortKnowledgeBases(
  items: KnowledgeBaseDTO[],
  sortKey: KnowledgeBaseSortKey,
): KnowledgeBaseDTO[] {
  if (sortKey === "default") return items;
  const collator = new Intl.Collator(undefined, { numeric: true, sensitivity: "base" });
  return items
    .map((item, index) => ({ item, index }))
    .sort((left, right) => {
      let result = 0;
      if (sortKey === "name") result = collator.compare(left.item.name, right.item.name);
      else if (sortKey === "files") result = right.item.fileCount - left.item.fileCount;
      else {
        const field = sortKey === "created" ? "createdAt" : "updatedAt";
        result = Date.parse(right.item[field]) - Date.parse(left.item[field]);
      }
      return result || left.index - right.index;
    })
    .map(({ item }) => item);
}

type KnowledgeFileUploadResult = { file: { fileID: string } };

async function uploadFiles(
  accessToken: string,
  files: File[],
  upload: (accessToken: string, file: File) => Promise<KnowledgeFileUploadResult>,
) {
  const uploaded: KnowledgeFileUploadResult[] = [];
  let failed = 0;
  let nextIndex = 0;
  const workerCount = Math.min(UPLOAD_CONCURRENCY, files.length);
  await Promise.all(Array.from({ length: workerCount }, async () => {
    while (nextIndex < files.length) {
      const index = nextIndex;
      nextIndex += 1;
      try {
        uploaded.push(await upload(accessToken, files[index]));
      } catch {
        failed += 1;
      }
    }
  }));
  return { uploaded, failed };
}
