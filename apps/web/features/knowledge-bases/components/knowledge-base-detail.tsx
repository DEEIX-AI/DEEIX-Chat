"use client";

import { ArrowLeft, BookOpen, DatabaseZap, Link2Off, Plus } from "lucide-react";
import { useTranslations } from "next-intl";

import { Badge } from "@/components/ui/badge";
import { Button } from "@/components/ui/button";
import { Checkbox } from "@/components/ui/checkbox";
import { CenteredEmptyState } from "@/components/ui/empty-state";
import { Spinner } from "@/components/ui/spinner";
import { Switch } from "@/components/ui/switch";
import type {
  KnowledgeBaseMobileView,
  KnowledgeBaseMode,
} from "@/features/knowledge-bases/types/knowledge-bases";
import { cn } from "@/lib/utils";
import type { KnowledgeBaseDTO, KnowledgeBaseFileDTO } from "@/shared/api/knowledge-bases.types";
import { formatBytes, resolveFileIcon } from "@/shared/lib/file-display";
import { canManuallyVectorizeFile, isVectorIndexOutdated, resolveFileRetrievalBadge } from "@/shared/lib/file-processing";

type KnowledgeBaseDetailProps = {
  mode: KnowledgeBaseMode;
  mobileView: KnowledgeBaseMobileView;
  selected: KnowledgeBaseDTO | null;
  files: KnowledgeBaseFileDTO[];
  filesTotal: number;
  loading: boolean;
  loadingMore: boolean;
  removingFileID: string;
  toggling: boolean;
  selectedFileIDs: string[];
  vectorizingFileIDs: string[];
  onBack: () => void;
  onAddFiles: () => void;
  onLoadMore: () => Promise<void>;
  onRemoveFile: (fileID: string) => Promise<void>;
  onToggleEnabled: (enabled: boolean) => Promise<void>;
  onPreviewFile: (file: KnowledgeBaseFileDTO) => void;
  onToggleFileSelection: (fileID: string, checked: boolean) => void;
  onSelectVectorizableFiles: () => void;
  onClearFileSelection: () => void;
  onVectorizeFile: (fileID: string) => Promise<void>;
  onVectorizeSelectedFiles: () => Promise<void>;
};

export function KnowledgeBaseDetail({
  mode,
  mobileView,
  selected,
  files,
  filesTotal,
  loading,
  loadingMore,
  removingFileID,
  toggling,
  selectedFileIDs,
  vectorizingFileIDs,
  onBack,
  onAddFiles,
  onLoadMore,
  onRemoveFile,
  onToggleEnabled,
  onPreviewFile,
  onToggleFileSelection,
  onSelectVectorizableFiles,
  onClearFileSelection,
  onVectorizeFile,
  onVectorizeSelectedFiles,
}: KnowledgeBaseDetailProps) {
  const t = useTranslations("knowledgeBases");

  return (
    <section
      className={cn(
        "relative min-h-0 min-w-0 flex-1 flex-col overflow-hidden md:flex",
        mobileView === "detail" ? "flex" : "hidden",
      )}
    >
      {selected ? (
        <>
          <KnowledgeBaseDetailHeader
            mode={mode}
            selected={selected}
            toggling={toggling}
            onBack={onBack}
            onAddFiles={onAddFiles}
            onToggleEnabled={onToggleEnabled}
          />
          {loading || files.length > 0 ? (
            <KnowledgeBaseFileList
              mode={mode}
              selected={selected}
              files={files}
              filesTotal={filesTotal}
              loading={loading}
              loadingMore={loadingMore}
              removingFileID={removingFileID}
              selectedFileIDs={selectedFileIDs}
              vectorizingFileIDs={vectorizingFileIDs}
              onLoadMore={onLoadMore}
              onRemoveFile={onRemoveFile}
              onPreviewFile={onPreviewFile}
              onToggleFileSelection={onToggleFileSelection}
              onSelectVectorizableFiles={onSelectVectorizableFiles}
              onClearFileSelection={onClearFileSelection}
              onVectorizeFile={onVectorizeFile}
              onVectorizeSelectedFiles={onVectorizeSelectedFiles}
            />
          ) : (
            <div className="pointer-events-none absolute inset-0">
              <CenteredEmptyState
                title={t("noFiles")}
                description={
                  mode === "admin" || selected.scope === "user"
                    ? t("noFilesDescription")
                    : undefined
                }
              />
            </div>
          )}
        </>
      ) : (
        <CenteredEmptyState title={t("selectHint")} description={t("selectHintDescription")} />
      )}
    </section>
  );
}

type KnowledgeBaseDetailHeaderProps = Pick<
  KnowledgeBaseDetailProps,
  "mode" | "selected" | "toggling" | "onBack" | "onAddFiles" | "onToggleEnabled"
> & {
  selected: KnowledgeBaseDTO;
};

function KnowledgeBaseDetailHeader({
  mode,
  selected,
  toggling,
  onBack,
  onAddFiles,
  onToggleEnabled,
}: KnowledgeBaseDetailHeaderProps) {
  const t = useTranslations("knowledgeBases");

  return (
    <header className="flex h-15 min-w-0 shrink-0 items-center justify-between gap-3 border-b border-border/40 px-3 md:px-5">
      <div className="flex min-w-0 flex-1 items-center gap-3">
        <Button
          type="button"
          variant="ghost"
          size="icon"
          className="-ml-1 size-8 shrink-0 rounded-md text-muted-foreground hover:bg-muted hover:text-foreground md:hidden"
          onClick={onBack}
          aria-label={t("back")}
        >
          <ArrowLeft className="size-4" strokeWidth={1.6} />
        </Button>
        <div className="hidden size-7 shrink-0 items-center justify-center md:flex">
          <BookOpen className="size-5 text-muted-foreground" strokeWidth={1.5} />
        </div>
        <div className="min-w-0 flex-1">
          <div className="flex min-w-0 items-center gap-1.5">
            <h2 className="truncate text-[13px] font-medium text-foreground">{selected.name}</h2>
            <Badge variant="secondary" className="h-4 rounded-md px-1.5 text-[9px] font-normal">
              {selected.scope === "builtin" ? t("builtin") : t("personal")}
            </Badge>
          </div>
          <p className="mt-0.5 truncate text-[11px] text-muted-foreground">
            {selected.description || t("files", { count: selected.fileCount })}
          </p>
        </div>
      </div>
      {mode === "admin" || selected.scope === "user" ? (
        <div className="flex shrink-0 items-center gap-1">
          {mode === "admin" ? (
            <label className="mr-1 flex items-center gap-1.5 text-[11px] text-muted-foreground">
              <span className="hidden lg:inline">
                {selected.enabled ? t("enabled") : t("disabled")}
              </span>
              <Switch
                size="sm"
                checked={selected.enabled}
                disabled={toggling}
                onCheckedChange={(checked) => void onToggleEnabled(checked)}
                aria-label={t("availability")}
              />
            </label>
          ) : null}
          <Button
            type="button"
            size="sm"
            className="ml-0 h-7 px-2 sm:ml-1"
            onClick={onAddFiles}
            aria-label={t("addFiles")}
          >
            <Plus className="size-3.5" strokeWidth={1.6} />
            <span className="hidden sm:inline">{t("confirmAdd")}</span>
          </Button>
        </div>
      ) : null}
    </header>
  );
}

type KnowledgeBaseFileListProps = Pick<
  KnowledgeBaseDetailProps,
  | "mode"
  | "selected"
  | "files"
  | "filesTotal"
  | "loading"
  | "loadingMore"
  | "removingFileID"
  | "selectedFileIDs"
  | "vectorizingFileIDs"
  | "onLoadMore"
  | "onRemoveFile"
  | "onPreviewFile"
  | "onToggleFileSelection"
  | "onSelectVectorizableFiles"
  | "onClearFileSelection"
  | "onVectorizeFile"
  | "onVectorizeSelectedFiles"
> & {
  selected: KnowledgeBaseDTO;
};

function KnowledgeBaseFileList({
  mode,
  selected,
  files,
  filesTotal,
  loading,
  loadingMore,
  removingFileID,
  selectedFileIDs,
  vectorizingFileIDs,
  onLoadMore,
  onRemoveFile,
  onPreviewFile,
  onToggleFileSelection,
  onSelectVectorizableFiles,
  onClearFileSelection,
  onVectorizeFile,
  onVectorizeSelectedFiles,
}: KnowledgeBaseFileListProps) {
  const t = useTranslations("knowledgeBases");
  const tStatus = useTranslations("files.status");
  const editable = mode === "admin" || selected.scope === "user";
  const selectedFileIDSet = new Set(selectedFileIDs);
  const vectorizingFileIDSet = new Set(vectorizingFileIDs);
  const vectorizingFiles = vectorizingFileIDs.length > 0;
  const vectorizableFiles = files.filter(canManuallyVectorizeFile);
  const allVectorizableSelected = vectorizableFiles.length > 0
    && vectorizableFiles.every((file) => selectedFileIDSet.has(file.fileID));

  return (
    <div className="flex min-h-0 flex-1 flex-col overflow-hidden">
      {editable && selectedFileIDs.length > 0 ? (
        <div className="flex h-10 shrink-0 items-center gap-2 border-b border-border/35 px-3 text-[11px] text-muted-foreground md:px-5">
          <Checkbox
            checked={allVectorizableSelected ? true : selectedFileIDs.length > 0 ? "indeterminate" : false}
            onCheckedChange={(checked) => checked ? onSelectVectorizableFiles() : onClearFileSelection()}
            aria-label={t("selectVectorizableFiles")}
          />
          <span className="min-w-0 flex-1 truncate">{t("selectedVectorizeFiles", { count: selectedFileIDs.length })}</span>
          <Button
            type="button"
            variant="ghost"
            size="sm"
            className="h-7 px-2 text-xs"
            disabled={vectorizingFiles}
            onClick={onClearFileSelection}
          >
            {t("cancel")}
          </Button>
          <Button
            type="button"
            variant="ghost"
            size="sm"
            className="h-7 gap-1 px-2 text-xs"
            disabled={selectedFileIDs.length === 0 || vectorizingFiles}
            onClick={() => void onVectorizeSelectedFiles()}
          >
            {vectorizingFiles ? <Spinner className="size-3.5" /> : <DatabaseZap className="size-3.5" strokeWidth={1.5} />}
            {t("vectorize")}
          </Button>
        </div>
      ) : null}
      <div className="min-h-0 flex-1 overflow-y-auto px-3 py-3 md:px-5">
      {loading ? (
        <div className="flex h-full items-center justify-center text-muted-foreground">
          <Spinner className="size-4" />
        </div>
      ) : (
        <div className="w-full">
          <div className="space-y-0.5">
            {files.map((file) => {
              const FileIcon = resolveFileIcon(file);
              const statusLabel = resolveFileRetrievalBadge(
                file,
                (key, values) => tStatus(key, values),
              ).label;
              const vectorizable = canManuallyVectorizeFile(file);
              const outdatedIndex = isVectorIndexOutdated(file);
              const checked = selectedFileIDSet.has(file.fileID);
              return (
                <div key={file.fileID} className={cn("group relative -mx-2 min-h-10 rounded-md", checked && "bg-muted/35")}>
                  {editable && vectorizable ? (
                    <Checkbox
                      checked={checked}
                      className={cn(
                        "absolute left-3 top-1/2 z-10 -translate-y-1/2 transition-opacity",
                        checked ? "opacity-100" : "opacity-100 md:opacity-0 md:group-hover:opacity-100 md:focus-visible:opacity-100",
                      )}
                      onClick={(event) => event.stopPropagation()}
                      onCheckedChange={(value) => onToggleFileSelection(file.fileID, value === true)}
                      aria-label={t("selectFile", { name: file.fileName })}
                    />
                  ) : null}
                  <button
                    type="button"
                    className={cn(
                      "flex min-h-10 w-full items-center gap-2 rounded-md px-2 py-1 text-left transition-colors hover:bg-muted/45 focus-visible:bg-muted/45 focus-visible:outline-none",
                      editable && "pr-18",
                    )}
                    onClick={() => onPreviewFile(file)}
                  >
                    <span className={cn(
                      "flex size-6 shrink-0 items-center justify-center text-muted-foreground",
                      vectorizable && (checked ? "opacity-0" : "opacity-0 transition-opacity md:opacity-100 md:group-hover:opacity-0"),
                    )}>
                      <FileIcon className="size-4" strokeWidth={1.5} />
                    </span>
                    <span className="min-w-0 flex-1">
                      <span
                        className="block truncate text-xs font-normal text-foreground"
                        title={file.fileName}
                      >
                        {file.fileName}
                      </span>
                      <span className="mt-0.5 block truncate text-[10px] text-muted-foreground sm:hidden">
                        {formatBytes(file.sizeBytes)} · {statusLabel}
                      </span>
                    </span>
                    <span className="hidden w-20 shrink-0 text-right text-[11px] text-muted-foreground sm:block">
                      {formatBytes(file.sizeBytes)}
                    </span>
                    <span
                      className="hidden w-20 shrink-0 truncate text-right text-[11px] text-muted-foreground sm:block"
                      title={statusLabel}
                    >
                      {statusLabel}
                    </span>
                  </button>
                  {editable ? (
                    <div className="absolute right-1 top-1/2 flex -translate-y-1/2 items-center gap-0.5">
                      {vectorizable ? (
                        <Button
                          type="button"
                          variant="ghost"
                          size="icon-sm"
                          className="text-muted-foreground/70 hover:bg-muted hover:text-foreground"
                          disabled={vectorizingFiles}
                          onClick={(event) => {
                            event.stopPropagation();
                            void onVectorizeFile(file.fileID);
                          }}
                          aria-label={t(outdatedIndex ? "updateVectorIndexFile" : "vectorizeFile", { name: file.fileName })}
                          title={t(outdatedIndex ? "updateVectorIndex" : "vectorize")}
                        >
                          {vectorizingFileIDSet.has(file.fileID) ? <Spinner className="size-3.5" /> : <DatabaseZap className="size-3.5" strokeWidth={1.5} />}
                        </Button>
                      ) : null}
                      <Button
                        type="button"
                        variant="ghost"
                        size="icon-sm"
                        className="text-muted-foreground/70 hover:bg-muted hover:text-foreground"
                        disabled={Boolean(removingFileID)}
                        onClick={(event) => {
                          event.stopPropagation();
                          void onRemoveFile(file.fileID);
                        }}
                        aria-label={t("removeFile")}
                        title={t("removeFile")}
                      >
                        {removingFileID === file.fileID ? (
                          <Spinner className="size-3.5" />
                        ) : (
                          <Link2Off className="size-3.5" strokeWidth={1.5} />
                        )}
                      </Button>
                    </div>
                  ) : null}
                </div>
              );
            })}
          </div>
          {files.length < filesTotal ? (
            <div className="flex justify-center pt-3">
              <Button
                variant="ghost"
                size="sm"
                disabled={loadingMore}
                onClick={() => void onLoadMore()}
              >
                {loadingMore ? <Spinner className="size-3.5" /> : null}
                {t("loadMore")}
              </Button>
            </div>
          ) : null}
        </div>
      )}
      </div>
    </div>
  );
}
