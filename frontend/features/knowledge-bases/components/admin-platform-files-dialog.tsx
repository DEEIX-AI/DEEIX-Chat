"use client";

import dynamic from "next/dynamic";
import { Search, Trash2, Upload } from "lucide-react";
import { useTranslations } from "next-intl";
import * as React from "react";
import { toast } from "sonner";

import {
  AlertDialog,
  AlertDialogAction,
  AlertDialogCancel,
  AlertDialogContent,
  AlertDialogDescription,
  AlertDialogFooter,
  AlertDialogHeader,
  AlertDialogTitle,
} from "@/components/ui/alert-dialog";
import { Button } from "@/components/ui/button";
import { Checkbox } from "@/components/ui/checkbox";
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from "@/components/ui/dialog";
import { Input } from "@/components/ui/input";
import { Spinner } from "@/components/ui/spinner";
import { DialogHeightTransition } from "@/features/knowledge-bases/components/knowledge-base-dialogs";
import { useLocalizedErrorMessage } from "@/i18n/use-localized-error";
import {
  deleteAdminKnowledgeBaseFile,
  fetchAdminPlatformFileContent,
  listAdminPlatformFiles,
  uploadAdminKnowledgeBaseFile,
} from "@/shared/api/knowledge-bases";
import type { KnowledgeBaseFileDTO } from "@/shared/api/knowledge-bases.types";
import { resolveAccessToken } from "@/shared/auth/resolve-access-token";
import type { PreviewDialogFile } from "@/shared/components/file-preview/preview-dialog";
import { useDialogSnapshot } from "@/shared/hooks/use-dialog-snapshot";
import { formatBytes, resolveFileIcon } from "@/shared/lib/file-display";
import { resolveFileRetrievalBadge } from "@/shared/lib/file-processing";
import { runSettledBulkItems, runSettledItemsWithConcurrency } from "@/shared/lib/bulk-action";

const PAGE_SIZE = 50;
const UPLOAD_LIMIT = 100;
const SEARCH_DEBOUNCE_MS = 200;

const FilePreviewDialog = dynamic(
  () => import("@/shared/components/file-preview/preview-dialog").then((mod) => mod.FilePreviewDialog),
  { ssr: false },
);

export function AdminPlatformFilesDialog({
  open,
  onOpenChange,
}: {
  open: boolean;
  onOpenChange: (open: boolean) => void;
}) {
  const t = useTranslations("knowledgeBases");
  const tStatus = useTranslations("files.status");
  const resolveErrorMessage = useLocalizedErrorMessage();
  const fileInputRef = React.useRef<HTMLInputElement>(null);
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
  const [previewTarget, setPreviewTarget] = React.useState<KnowledgeBaseFileDTO | null>(null);
  const previewSnapshot = useDialogSnapshot(previewTarget);
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

  return (
    <>
      <Dialog open={open} onOpenChange={(nextOpen) => {
        if (!nextOpen && busy) return;
        onOpenChange(nextOpen);
      }}>
        <DialogContent className="w-[calc(100vw-2rem)] gap-0 overflow-hidden p-0 sm:max-w-[620px]">
          <DialogHeightTransition>
            <DialogHeader className="px-5 pb-3 pt-5">
              <DialogTitle>{t("localFilesTitle")}</DialogTitle>
              <DialogDescription>{t("localFilesDescription")}</DialogDescription>
            </DialogHeader>
            <input
              ref={fileInputRef}
              type="file"
              multiple
              className="hidden"
              onChange={(event) => {
                const selectedFiles = Array.from(event.target.files ?? []);
                event.target.value = "";
                void uploadFiles(selectedFiles);
              }}
            />
            <div className="min-h-0 px-5 py-2">
              <div className="flex gap-2 pb-2.5">
                <div className="relative min-w-0 flex-1">
                  <Search className="pointer-events-none absolute left-3 top-1/2 size-4 -translate-y-1/2 text-muted-foreground" strokeWidth={1.6} />
                  <Input
                    value={query}
                    className="pl-9"
                    placeholder={t("searchPlatformFiles")}
                    disabled={busy}
                    onChange={(event) => setQuery(event.target.value)}
                  />
                </div>
                <Button
                  type="button"
                  variant="outline"
                  className="shrink-0 shadow-none"
                  disabled={busy}
                  onClick={() => fileInputRef.current?.click()}
                >
                  {uploading ? <Spinner className="size-3.5" /> : <Upload className="size-3.5" strokeWidth={1.6} />}
                  {t("uploadFiles")}
                </Button>
              </div>
              <div className="flex max-h-[min(52vh,360px)] flex-col overflow-hidden rounded-md bg-muted/20">
                {loading ? (
                  <div className="flex justify-center py-14"><Spinner className="size-4" /></div>
                ) : files.length > 0 ? (
                  <>
                    <div className="flex h-8 shrink-0 items-center px-3 text-[11px] text-muted-foreground">
                      <label className="flex cursor-pointer items-center gap-2">
                        <Checkbox
                          checked={allLoadedSelected ? true : someLoadedSelected ? "indeterminate" : false}
                          disabled={busy}
                          aria-label={t("selectAllPlatformFiles")}
                          onCheckedChange={(checked) => toggleAllLoadedFiles(checked === true)}
                        />
                        <span>{t("selectAll")}</span>
                      </label>
                      {selectedFileIDs.length > 0 ? (
                        <>
                          <span className="ml-3">{t("selectedPlatformFileCount", { count: selectedFileIDs.length })}</span>
                          <span className="ml-auto flex h-6 w-10 shrink-0 items-center justify-center">
                            <Button
                              type="button"
                              variant="ghost"
                              size="icon-sm"
                              className="size-5 text-destructive shadow-none hover:bg-destructive/10 hover:text-destructive"
                              aria-label={t("bulkDeletePlatformFiles")}
                              disabled={busy}
                              onClick={requestBulkDelete}
                            >
                              <Trash2 className="size-3" strokeWidth={1.6} />
                            </Button>
                          </span>
                        </>
                      ) : null}
                    </div>
                    <div className="min-h-0 space-y-px overflow-y-auto p-1 pt-0">
                      {files.map((file) => {
                        const FileIcon = resolveFileIcon(file);
                        const statusLabel = resolveFileRetrievalBadge(
                          file,
                          (key, values) => tStatus(key, values),
                        ).label;
                        return (
                          <div
                            key={file.fileID}
                            className="group flex h-9 w-full items-center rounded-md px-2 text-left transition-colors hover:bg-muted/60 focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring"
                            role="button"
                            tabIndex={0}
                            onClick={() => setPreviewTarget(file)}
                            onKeyDown={(event) => {
                              if (event.currentTarget !== event.target || (event.key !== "Enter" && event.key !== " ")) return;
                              event.preventDefault();
                              setPreviewTarget(file);
                            }}
                          >
                            <span className="mr-2 flex size-4 shrink-0 items-center justify-center">
                              <Checkbox
                                checked={selectedFileIDSet.has(file.fileID)}
                                disabled={busy}
                                aria-label={t("selectPlatformFile", { name: file.fileName })}
                                onClick={(event) => event.stopPropagation()}
                                onCheckedChange={(checked) => toggleFileSelection(file.fileID, checked === true)}
                              />
                            </span>
                            <span className="mr-2.5 flex size-4 shrink-0 items-center justify-center text-muted-foreground">
                              <FileIcon className="size-3.5" strokeWidth={1.5} />
                            </span>
                            <span className="min-w-0 flex-1 truncate text-xs text-foreground" title={file.fileName}>
                              {file.fileName}
                            </span>
                            <span className="flex h-5 w-16 shrink-0 items-center justify-end text-right text-[10px] leading-none text-muted-foreground">
                              {formatBytes(file.sizeBytes)}
                            </span>
                            <span className="ml-3 flex h-5 w-16 shrink-0 items-center justify-end truncate text-right text-[10px] leading-none text-muted-foreground" title={statusLabel}>
                              {statusLabel}
                            </span>
                            <span className="ml-2 flex h-5 w-10 shrink-0 items-center justify-center">
                              <Button
                                type="button"
                                variant="ghost"
                                size="icon-sm"
                                className="size-5 shrink-0 text-muted-foreground hover:text-destructive"
                                aria-label={t("deletePlatformFile")}
                                disabled={busy}
                                onClick={(event) => {
                                  event.stopPropagation();
                                  setDeleteTarget(file);
                                  setDeleteDialogOpen(true);
                                }}
                              >
                                {deletingFileID === file.fileID
                                  ? <Spinner className="size-3" />
                                  : <Trash2 className="size-3" strokeWidth={1.6} />}
                              </Button>
                            </span>
                          </div>
                        );
                      })}
                      {page * PAGE_SIZE < total ? (
                        <div className="flex justify-center py-2">
                          <Button type="button" variant="ghost" size="sm" disabled={busy || loadingMore} onClick={() => void loadMore()}>
                            {loadingMore ? <Spinner className="size-3.5" /> : null}
                            {t("loadMore")}
                          </Button>
                        </div>
                      ) : null}
                    </div>
                  </>
                ) : (
                  <div className="flex min-h-36 items-center justify-center text-xs text-muted-foreground">
                    {t("localFilesEmpty")}
                  </div>
                )}
              </div>
            </div>
            <DialogFooter className="px-5 py-3">
              <span className="mr-auto self-center text-xs text-muted-foreground">
                {t("localFilesCount", { count: total })}
              </span>
              <Button type="button" variant="ghost" disabled={busy} onClick={() => onOpenChange(false)}>
                {t("close")}
              </Button>
            </DialogFooter>
          </DialogHeightTransition>
        </DialogContent>
      </Dialog>

      <AlertDialog open={deleteDialogOpen} onOpenChange={(nextOpen) => {
        if (!nextOpen && deletingFileID) return;
        setDeleteDialogOpen(nextOpen);
      }}>
        <AlertDialogContent
          size="compact"
          onAnimationEnd={(event) => {
            if (event.target === event.currentTarget && event.currentTarget.dataset.state === "closed") {
              setDeleteTarget(null);
            }
          }}
        >
          <AlertDialogHeader>
            <AlertDialogTitle>{t("deletePlatformFileTitle")}</AlertDialogTitle>
            <AlertDialogDescription>
              {t("deletePlatformFileDescription", { name: deleteTarget?.fileName ?? "" })}
            </AlertDialogDescription>
          </AlertDialogHeader>
          <AlertDialogFooter>
            <AlertDialogCancel disabled={Boolean(deletingFileID)}>{t("cancel")}</AlertDialogCancel>
            <AlertDialogAction
              variant="destructive"
              disabled={!deleteTarget || Boolean(deletingFileID)}
              onClick={(event) => {
                event.preventDefault();
                void deleteFile();
              }}
            >
              {deletingFileID ? <Spinner className="size-3.5" /> : null}
              {t("deletePlatformFile")}
            </AlertDialogAction>
          </AlertDialogFooter>
        </AlertDialogContent>
      </AlertDialog>

      <AlertDialog open={bulkDeleteDialogOpen} onOpenChange={(nextOpen) => {
        if (!nextOpen && bulkDeleting) return;
        setBulkDeleteDialogOpen(nextOpen);
      }}>
        <AlertDialogContent
          size="compact"
          onAnimationEnd={(event) => {
            if (event.target === event.currentTarget && event.currentTarget.dataset.state === "closed") {
              setBulkDeleteTargetIDs([]);
            }
          }}
        >
          <AlertDialogHeader>
            <AlertDialogTitle>{t("bulkDeletePlatformFilesTitle")}</AlertDialogTitle>
            <AlertDialogDescription>
              {t("bulkDeletePlatformFilesDescription", { count: bulkDeleteTargetIDs.length })}
            </AlertDialogDescription>
          </AlertDialogHeader>
          <AlertDialogFooter>
            <AlertDialogCancel disabled={bulkDeleting}>{t("cancel")}</AlertDialogCancel>
            <AlertDialogAction
              variant="destructive"
              disabled={bulkDeleteTargetIDs.length === 0 || bulkDeleting}
              onClick={(event) => {
                event.preventDefault();
                void bulkDeleteFiles();
              }}
            >
              {bulkDeleting ? <Spinner className="size-3.5" /> : null}
              {t("deletePlatformFile")}
            </AlertDialogAction>
          </AlertDialogFooter>
        </AlertDialogContent>
      </AlertDialog>

      {previewSnapshot ? (
        <FilePreviewDialog
          file={previewSnapshot}
          open={previewTarget !== null}
          onOpenChange={(nextOpen) => {
            if (!nextOpen) setPreviewTarget(null);
          }}
          loadContent={loadPreviewContent}
        />
      ) : null}
    </>
  );
}

async function requireAccessToken(): Promise<string> {
  const token = await resolveAccessToken();
  if (!token) throw new Error("missing access token");
  return token;
}
