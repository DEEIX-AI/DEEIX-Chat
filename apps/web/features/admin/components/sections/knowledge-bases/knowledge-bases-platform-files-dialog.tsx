"use client";

import { Search, Trash2, Upload } from "lucide-react";
import { useTranslations } from "next-intl";
import * as React from "react";

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
  DialogHeightTransition,
  DialogHeader,
  DialogTitle,
} from "@/components/ui/dialog";
import { Input } from "@/components/ui/input";
import { Spinner } from "@/components/ui/spinner";
import { useAdminKnowledgeBasesPlatformFiles } from "@/features/admin/hooks/use-admin-knowledge-bases-platform-files";
import type { KnowledgeBaseFileDTO } from "@/shared/api/knowledge-bases-types";
import { useDialogSnapshot } from "@/shared/hooks/use-dialog-snapshot";
import { LazyFilePreviewDialog, formatBytes, resolveFileIcon, resolveFileRetrievalBadge } from "@/entities/file";

export function AdminPlatformFilesDialog({
  open,
  onOpenChange,
}: {
  open: boolean;
  onOpenChange: (open: boolean) => void;
}) {
  const t = useTranslations("knowledgeBases");
  const tStatus = useTranslations("files.status");
  const fileInputRef = React.useRef<HTMLInputElement>(null);
  const [previewTarget, setPreviewTarget] = React.useState<KnowledgeBaseFileDTO | null>(null);
  const previewSnapshot = useDialogSnapshot(previewTarget);
  const {
    query,
    setQuery,
    files,
    loading,
    loadingMore,
    uploading,
    deletingFileID,
    selectedFileIDSet,
    allLoadedSelected,
    someLoadedSelected,
    selectedFileIDs,
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
    total,
    hasMore,
    loadMore,
    uploadFiles,
    deleteFile,
    toggleFileSelection,
    toggleAllLoadedFiles,
    requestBulkDelete,
    bulkDeleteFiles,
    loadPreviewContent,
  } = useAdminKnowledgeBasesPlatformFiles({ open });

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
                      {hasMore ? (
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
                  <div className="flex min-h-36 items-center justify-center px-3 py-6 text-center text-xs text-muted-foreground">
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
        <LazyFilePreviewDialog
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
