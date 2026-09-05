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
import { Spinner, SpinnerLabel } from "@/components/ui/spinner";
import { Textarea } from "@/components/ui/textarea";
import { cn } from "@/lib/utils";
import { DeleteFilesOption } from "@/shared/components/delete-files-option";
import type { KnowledgeBaseDTO, KnowledgeBaseFileDTO } from "@/shared/api/knowledge-bases.types";
import type { KnowledgeBaseDraft } from "@/features/knowledge-bases/types/knowledge-bases";
import { useDialogSnapshot } from "@/shared/hooks/use-dialog-snapshot";
import { formatBytes, resolveFileIcon } from "@/shared/lib/file-display";
import { resolveFileRetrievalBadge } from "@/shared/lib/file-processing";

export function KnowledgeBaseEditorDialog({
  draft,
  saving,
  onDraftChange,
  onClose,
  onSave,
}: {
  draft: KnowledgeBaseDraft | null;
  saving: boolean;
  onDraftChange: (draft: KnowledgeBaseDraft) => void;
  onClose: () => void;
  onSave: () => void;
}) {
  const t = useTranslations("knowledgeBases");
  const stableDraft = useDialogSnapshot(draft);

  return (
    <Dialog open={Boolean(draft)} onOpenChange={(open) => !open && !saving && onClose()}>
      <DialogContent className="flex max-h-[min(86vh,760px)] w-[calc(100vw-2rem)] flex-col gap-0 overflow-hidden p-0 sm:max-w-[560px]">
        <DialogHeader className="shrink-0 px-4 py-4">
          <DialogTitle>{stableDraft?.publicID ? t("editTitle") : t("createTitle")}</DialogTitle>
          <DialogDescription>{t("editorDescription")}</DialogDescription>
        </DialogHeader>
        <form
          className="flex min-h-0 flex-1 flex-col"
          onSubmit={(event) => {
            event.preventDefault();
            onSave();
          }}
        >
          <div className="min-h-0 flex-1 space-y-4 overflow-y-auto px-4 py-2">
            <div className="space-y-1">
              <label className="text-xs font-normal text-muted-foreground" htmlFor="knowledge-base-name">
                {t("name")}
              </label>
              <Input
                id="knowledge-base-name"
                autoFocus
                maxLength={80}
                value={stableDraft?.name ?? ""}
                placeholder={t("namePlaceholder")}
                disabled={saving}
                required
                onChange={(event) => draft && onDraftChange({ ...draft, name: event.target.value })}
              />
            </div>
            <div className="space-y-1">
              <label className="text-xs font-normal text-muted-foreground" htmlFor="knowledge-base-description">
                {t("descriptionLabel")}
              </label>
              <Textarea
                id="knowledge-base-description"
                maxLength={255}
                className="min-h-20 resize-none"
                value={stableDraft?.description ?? ""}
                placeholder={t("descriptionPlaceholder")}
                disabled={saving}
                onChange={(event) => draft && onDraftChange({ ...draft, description: event.target.value })}
              />
            </div>
          </div>
          <DialogFooter className="shrink-0 px-4 py-3">
            <Button type="button" variant="ghost" disabled={saving} onClick={onClose}>{t("cancel")}</Button>
            <Button type="submit" disabled={!stableDraft?.name.trim() || saving}>
              {saving ? <SpinnerLabel>{t("save")}</SpinnerLabel> : t("save")}
            </Button>
          </DialogFooter>
        </form>
      </DialogContent>
    </Dialog>
  );
}

export function AddKnowledgeBaseFilesDialog({
  open,
  platformFiles = false,
  knowledgeBaseName,
  files,
  loading,
  loadingMore,
  hasMore,
  query,
  selectedFileIDs,
  adding,
  uploading,
  deletingPlatformFileID,
  onOpenChange,
  onQueryChange,
  onSelectedFileIDsChange,
  selectionLimit,
  onLoadMore,
  onUploadFiles,
  onDeletePlatformFile,
  onConfirm,
}: {
  open: boolean;
  platformFiles?: boolean;
  knowledgeBaseName: string;
  files: KnowledgeBaseFileDTO[];
  loading: boolean;
  loadingMore: boolean;
  hasMore: boolean;
  query: string;
  selectedFileIDs: string[];
  adding: boolean;
  uploading: boolean;
  deletingPlatformFileID: string;
  onOpenChange: (open: boolean) => void;
  onQueryChange: (query: string) => void;
  onSelectedFileIDsChange: (ids: string[]) => void;
  selectionLimit: number;
  onLoadMore: () => void;
  onUploadFiles: (files: File[]) => void;
  onDeletePlatformFile: (fileID: string) => Promise<boolean>;
  onConfirm: () => void;
}) {
  const t = useTranslations("knowledgeBases");
  const tStatus = useTranslations("files.status");
  const fileInputRef = React.useRef<HTMLInputElement>(null);
  const [deleteTarget, setDeleteTarget] = React.useState<KnowledgeBaseFileDTO | null>(null);
  const [deleteDialogOpen, setDeleteDialogOpen] = React.useState(false);
  const busy = adding || uploading || Boolean(deletingPlatformFileID);
  const normalizedQuery = query.trim().toLowerCase();
  const filteredFiles = normalizedQuery
    ? files.filter((file) => file.fileName.toLowerCase().includes(normalizedQuery))
    : files;

  return (
    <Dialog open={open} onOpenChange={(nextOpen) => {
      if (!nextOpen && busy) return;
      onOpenChange(nextOpen);
    }}>
      <DialogContent className="w-[calc(100vw-2rem)] gap-0 overflow-hidden p-0 sm:max-w-[560px]">
        <DialogHeightTransition>
          <DialogHeader className="shrink-0 px-5 pb-3 pt-5">
            <DialogTitle>{t("addFilesTitle")}</DialogTitle>
            <DialogDescription>
              {t(platformFiles ? "addPlatformFilesDescription" : "addFilesDescription", { name: knowledgeBaseName })}
            </DialogDescription>
          </DialogHeader>
          <input
            ref={fileInputRef}
            type="file"
            multiple
            className="hidden"
            onChange={(event) => {
              const nextFiles = Array.from(event.target.files ?? []);
              event.target.value = "";
              if (nextFiles.length > 0) onUploadFiles(nextFiles);
            }}
          />
          <div className="min-h-0 px-5 py-2">
            <div className="flex shrink-0 gap-2 pb-2.5">
              <div className="relative min-w-0 flex-1">
                <Search className="pointer-events-none absolute left-3 top-1/2 size-4 -translate-y-1/2 text-muted-foreground" strokeWidth={1.6} />
                <Input
                  value={query}
                  className="pl-9"
                  placeholder={t(platformFiles ? "searchPlatformFiles" : "searchFiles")}
                  disabled={busy}
                  onChange={(event) => onQueryChange(event.target.value)}
                />
              </div>
              <Button variant="outline" className="shrink-0 shadow-none" disabled={busy} onClick={() => fileInputRef.current?.click()}>
                {uploading ? <Spinner className="size-3.5" /> : <Upload className="size-3.5" strokeWidth={1.6} />}
                {t("uploadFiles")}
              </Button>
            </div>

            <div className="max-h-[min(50vh,320px)] overflow-y-auto rounded-md bg-muted/20 p-1">
              {loading ? (
                <div className="flex justify-center py-14"><Spinner className="size-4" /></div>
              ) : filteredFiles.length > 0 ? (
                <div className="space-y-px">
                  {filteredFiles.map((file) => {
                    const checked = selectedFileIDs.includes(file.fileID);
                    const selectionDisabled = busy || (!checked && selectedFileIDs.length >= selectionLimit);
                    const toggleSelection = () => {
                      if (selectionDisabled) return;
                      onSelectedFileIDsChange(
                        checked
                          ? selectedFileIDs.filter((id) => id !== file.fileID)
                          : [...selectedFileIDs, file.fileID],
                      );
                    };
                    const FileIcon = resolveFileIcon(file);
                    const statusLabel = resolveFileRetrievalBadge(
                      file,
                      (key, values) => tStatus(key, values),
                    ).label;
                    return (
                      <div
                        key={file.fileID}
                        className={cn(
                          "flex h-8 items-center rounded-md px-1.5 transition-colors hover:bg-muted/55 has-[:disabled]:opacity-55",
                          selectionDisabled ? "cursor-not-allowed" : "cursor-pointer",
                          checked && "bg-accent/70 hover:bg-accent/80",
                        )}
                        role="checkbox"
                        aria-checked={checked}
                        aria-disabled={selectionDisabled}
                        tabIndex={selectionDisabled ? -1 : 0}
                        onClick={toggleSelection}
                        onKeyDown={(event) => {
                          if (event.currentTarget !== event.target || (event.key !== "Enter" && event.key !== " ")) return;
                          event.preventDefault();
                          toggleSelection();
                        }}
                      >
                        <div className="flex min-w-0 flex-1 items-center gap-2">
                          <Checkbox
                            className="mr-1 shrink-0"
                            checked={checked}
                            disabled={selectionDisabled}
                            onClick={(event) => event.stopPropagation()}
                            onCheckedChange={(next) => onSelectedFileIDsChange(
                              next === true
                                ? [...selectedFileIDs, file.fileID]
                                : selectedFileIDs.filter((id) => id !== file.fileID),
                            )}
                          />
                          <span className="flex size-4 shrink-0 items-center justify-center text-muted-foreground">
                            <FileIcon className="size-3" strokeWidth={1.5} />
                          </span>
                          <span className="min-w-0 flex-1 truncate text-xs font-normal text-foreground" title={file.fileName}>
                            {file.fileName}
                          </span>
                          <span className="w-14 shrink-0 text-right text-[10px] text-muted-foreground">
                            {formatBytes(file.sizeBytes)}
                          </span>
                          <span className="w-16 shrink-0 truncate text-right text-[10px] text-muted-foreground" title={statusLabel}>
                            {statusLabel}
                          </span>
                        </div>
                        {platformFiles ? (
                          <Button
                            type="button"
                            variant="ghost"
                            size="icon-sm"
                            className="ml-1.5 size-5 shrink-0 text-muted-foreground hover:text-destructive"
                            aria-label={t("deletePlatformFile")}
                            disabled={busy}
                            onClick={(event) => {
                              event.stopPropagation();
                              setDeleteTarget(file);
                              setDeleteDialogOpen(true);
                            }}
                          >
                            {deletingPlatformFileID === file.fileID
                              ? <Spinner className="size-3" />
                              : <Trash2 className="size-3" strokeWidth={1.6} />}
                          </Button>
                        ) : null}
                      </div>
                    );
                  })}
                </div>
              ) : (
                <div className="flex min-h-40 items-center justify-center px-3 py-6 text-center text-xs text-muted-foreground">
                  {t(platformFiles ? "noAvailablePlatformFiles" : "noAvailableFiles")}
                </div>
              )}
              {hasMore ? (
                <div className="flex justify-center py-2">
                  <Button variant="ghost" size="sm" disabled={busy || loadingMore} onClick={onLoadMore}>
                    {loadingMore ? <Spinner className="size-3.5" /> : null}
                    {t("loadMore")}
                  </Button>
                </div>
              ) : null}
            </div>
          </div>
          <DialogFooter className="shrink-0 px-5 py-3">
            <span className="mr-auto self-center text-xs text-muted-foreground">
              {t("selectedWithLimit", { count: selectedFileIDs.length, max: selectionLimit })}
            </span>
            <Button variant="ghost" disabled={busy} onClick={() => onOpenChange(false)}>{t("cancel")}</Button>
            <Button disabled={selectedFileIDs.length === 0 || busy} onClick={onConfirm}>
              {adding ? <Spinner className="size-3.5" /> : null}
              {t("confirmAdd")}
            </Button>
          </DialogFooter>
        </DialogHeightTransition>
      </DialogContent>
      <AlertDialog open={deleteDialogOpen} onOpenChange={(nextOpen) => {
        if (!nextOpen && deletingPlatformFileID) return;
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
            <AlertDialogCancel disabled={Boolean(deletingPlatformFileID)}>{t("cancel")}</AlertDialogCancel>
            <AlertDialogAction
              variant="destructive"
              disabled={!deleteTarget || Boolean(deletingPlatformFileID)}
              onClick={(event) => {
                event.preventDefault();
                if (!deleteTarget) return;
                void onDeletePlatformFile(deleteTarget.fileID).then((deleted) => {
                  if (deleted) setDeleteDialogOpen(false);
                });
              }}
            >
              {deletingPlatformFileID ? <Spinner className="size-3.5" /> : null}
              {t("deletePlatformFile")}
            </AlertDialogAction>
          </AlertDialogFooter>
        </AlertDialogContent>
      </AlertDialog>
    </Dialog>
  );
}

export function DeleteKnowledgeBaseDialog({
  target,
  deleting,
  deleteFiles,
  onClose,
  onDeleteFilesChange,
  onConfirm,
}: {
  target: KnowledgeBaseDTO | null;
  deleting: boolean;
  deleteFiles: boolean;
  onClose: () => void;
  onDeleteFilesChange: (checked: boolean) => void;
  onConfirm: () => void;
}) {
  const t = useTranslations("knowledgeBases");
  const tCommon = useTranslations("common.actions");
  const deleteFilesID = React.useId();
  const stableTarget = useDialogSnapshot(target);
  const stableDeleteFiles = useDialogSnapshot(target ? deleteFiles : null) ?? false;

  return (
    <AlertDialog open={Boolean(target)} onOpenChange={(open) => !open && !deleting && onClose()}>
      <AlertDialogContent size="compact">
        <AlertDialogHeader>
          <AlertDialogTitle>{t("deleteTitle")}</AlertDialogTitle>
          <AlertDialogDescription>{t("deleteDescription", { name: stableTarget?.name ?? "" })}</AlertDialogDescription>
        </AlertDialogHeader>
        {(stableTarget?.fileCount ?? 0) > 0 ? (
          <DeleteFilesOption
            id={deleteFilesID}
            checked={stableDeleteFiles}
            disabled={deleting}
            label={t("deleteFilesLabel")}
            description={t("deleteFilesDescription")}
            onCheckedChange={onDeleteFilesChange}
          />
        ) : null}
        <AlertDialogFooter>
          <AlertDialogCancel disabled={deleting}>{t("cancel")}</AlertDialogCancel>
          <AlertDialogAction
            variant="destructive"
            disabled={deleting}
            onClick={(event) => {
              event.preventDefault();
              onConfirm();
            }}
          >
            {deleting ? <Spinner className="size-3.5" /> : null}
            {tCommon("delete")}
          </AlertDialogAction>
        </AlertDialogFooter>
      </AlertDialogContent>
    </AlertDialog>
  );
}

export function BulkDeleteKnowledgeBasesDialog({
  open,
  count,
  hasFiles,
  deleting,
  deleteFiles,
  onClose,
  onDeleteFilesChange,
  onConfirm,
}: {
  open: boolean;
  count: number;
  hasFiles: boolean;
  deleting: boolean;
  deleteFiles: boolean;
  onClose: () => void;
  onDeleteFilesChange: (checked: boolean) => void;
  onConfirm: () => void;
}) {
  const t = useTranslations("knowledgeBases");
  const tCommon = useTranslations("common.actions");
  const deleteFilesID = React.useId();
  const stableCount = useDialogSnapshot(open ? count : null) ?? 0;
  const stableHasFiles = useDialogSnapshot(open ? hasFiles : null) ?? false;
  const stableDeleteFiles = useDialogSnapshot(open ? deleteFiles : null) ?? false;

  return (
    <AlertDialog open={open} onOpenChange={(nextOpen) => !nextOpen && !deleting && onClose()}>
      <AlertDialogContent size="compact">
        <AlertDialogHeader>
          <AlertDialogTitle>{t("bulkDeleteTitle")}</AlertDialogTitle>
          <AlertDialogDescription>{t("bulkDeleteDescription", { count: stableCount })}</AlertDialogDescription>
        </AlertDialogHeader>
        {stableHasFiles ? (
          <DeleteFilesOption
            id={deleteFilesID}
            checked={stableDeleteFiles}
            disabled={deleting}
            label={t("deleteFilesLabel")}
            description={t("deleteFilesDescription")}
            onCheckedChange={onDeleteFilesChange}
          />
        ) : null}
        <AlertDialogFooter>
          <AlertDialogCancel disabled={deleting}>{t("cancel")}</AlertDialogCancel>
          <AlertDialogAction
            variant="destructive"
            disabled={deleting}
            onClick={(event) => {
              event.preventDefault();
              onConfirm();
            }}
          >
            {deleting ? <Spinner className="size-3.5" /> : null}
            {tCommon("delete")}
          </AlertDialogAction>
        </AlertDialogFooter>
      </AlertDialogContent>
    </AlertDialog>
  );
}
