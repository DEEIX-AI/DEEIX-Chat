"use client";

import { RotateCw, Search } from "lucide-react";
import { useLocale, useTranslations } from "next-intl";

import * as React from "react";

import { Badge } from "@/components/ui/badge";
import { Button } from "@/components/ui/button";
import { Checkbox } from "@/components/ui/checkbox";
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogHeightTransition,
  DialogTitle,
} from "@/components/ui/dialog";
import { Input } from "@/components/ui/input";
import { Spinner } from "@/components/ui/spinner";
import { Tabs, TabsList, TabsTrigger } from "@/components/ui/tabs";
import { Tooltip, TooltipContent, TooltipTrigger } from "@/components/ui/tooltip";
import { formatBytes, resolveFileIcon } from "@/entities/file";
import type { AdminEmbeddingTask, AdminEmbeddingTaskBucket } from "@/features/admin/api";
import { useAdminFilesEmbeddingTasks } from "@/features/admin/hooks/use-admin-files-embedding-tasks";
import { ADMIN_EMBEDDING_TASK_BUCKETS, isAdminEmbeddingTaskBucket } from "@/features/admin/model/admin-unions";
import { formatDateTime } from "@/features/admin/utils/user-display";
import { cn } from "@/lib/utils";
import { useLoadMoreSentinel } from "@/shared/hooks/use-load-more-sentinel";
import { isOneOf } from "@/shared/lib/type-guards";

const EMBED_STATUSES = ["none", "queued", "processing", "ready", "failed", "stale", "empty"] as const;
const isEmbedStatus = isOneOf(EMBED_STATUSES);
const RETRY_BLOCKED_REASONS = ["processing", "unsupported", "not_ready", "already_ready", "not_found"] as const;
const isRetryBlockedReason = isOneOf(RETRY_BLOCKED_REASONS);

const STATUS_BADGE_CLASS_NAMES: Record<(typeof EMBED_STATUSES)[number], string> = {
  none: "bg-muted text-muted-foreground",
  ready: "bg-green-500/10 text-green-700 dark:text-green-300",
  queued: "bg-sky-500/10 text-sky-700 dark:text-sky-300",
  processing: "bg-sky-500/10 text-sky-700 dark:text-sky-300",
  failed: "bg-destructive/10 text-destructive",
  stale: "bg-amber-500/10 text-amber-700 dark:text-amber-300",
  empty: "bg-muted text-muted-foreground",
};

export type EmbeddingTaskBucketCounts = Record<AdminEmbeddingTaskBucket, number>;

/**
 * Platform-wide embedding task browser opened from the index status panel.
 * Admins pick a bucket, search, and retry single files or a selection;
 * ready files are never touched.
 */
export function FilesEmbeddingTasksDialog({
  open,
  onOpenChange,
  bucket,
  onBucketChange,
  counts,
  onTasksChanged,
}: {
  open: boolean;
  onOpenChange: (open: boolean) => void;
  bucket: AdminEmbeddingTaskBucket;
  onBucketChange: (bucket: AdminEmbeddingTaskBucket) => void;
  counts: EmbeddingTaskBucketCounts | null;
  onTasksChanged: () => void;
}) {
  const t = useTranslations("adminFiles.embeddingTasks");
  const {
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
  } = useAdminFilesEmbeddingTasks({ open, bucket, onTasksChanged });
  const listRef = React.useRef<HTMLDivElement>(null);
  const retryableCount = tasks.filter((task) => task.retryable).length;
  // Auto-load when the list is scrolled near its end; the hook re-observes after every page, so a
  // short first page keeps filling until the list scrolls or every task is loaded.
  const loadMoreRef = useLoadMoreSentinel<HTMLDivElement>({
    enabled: open && hasMore && !loading,
    rootRef: listRef,
    onLoadMore: loadMore,
  });

  return (
    <Dialog open={open} onOpenChange={onOpenChange}>
      <DialogContent className="w-[calc(100vw-2rem)] gap-0 overflow-hidden p-0 sm:max-w-[680px]">
        <DialogHeightTransition contentClassName="max-h-[min(86vh,600px)]">
          <DialogHeader className="shrink-0 px-5 pb-3 pt-5">
            <DialogTitle>{t("title")}</DialogTitle>
            <DialogDescription>{t("description")}</DialogDescription>
          </DialogHeader>
          <div className="shrink-0 space-y-2.5 px-5 py-2">
            <Tabs
              value={bucket}
              onValueChange={(value) => {
                if (isAdminEmbeddingTaskBucket(value)) onBucketChange(value);
              }}
            >
              <TabsList variant="line" className="max-w-full justify-start overflow-x-auto">
                {ADMIN_EMBEDDING_TASK_BUCKETS.map((item) => (
                  <TabsTrigger key={item} value={item} disabled={retrying}>
                    {t(`buckets.${item}`)}
                    {counts ? (
                      <span className="ml-1 tabular-nums text-muted-foreground">{counts[item]}</span>
                    ) : null}
                  </TabsTrigger>
                ))}
              </TabsList>
            </Tabs>
            <p className="text-[11px] leading-relaxed text-muted-foreground">{t(`bucketHints.${bucket}`)}</p>
            <div className="relative">
              <Search
                className="pointer-events-none absolute left-3 top-1/2 size-4 -translate-y-1/2 text-muted-foreground"
                strokeWidth={1.6}
              />
              <Input
                value={query}
                className="pl-9"
                placeholder={t("searchPlaceholder")}
                aria-label={t("searchPlaceholder")}
                onChange={(event) => setQuery(event.target.value)}
              />
            </div>
          </div>
          <div className="flex min-h-0 flex-1 flex-col px-5 pb-2">
            <div className="flex min-h-0 flex-1 flex-col overflow-hidden rounded-md bg-muted/20">
              {loading ? (
                <div className="flex flex-1 items-center justify-center py-14">
                  <Spinner className="size-4" />
                </div>
              ) : tasks.length > 0 ? (
                <>
                  <div className="flex h-8 shrink-0 items-center gap-3 px-3 text-[11px] text-muted-foreground">
                    <label className="flex cursor-pointer items-center gap-2">
                      <Checkbox
                        checked={allRetryableSelected ? true : someRetryableSelected ? "indeterminate" : false}
                        disabled={retrying || retryableCount === 0}
                        aria-label={t("selectAll")}
                        onCheckedChange={(checked) => toggleAllRetryable(checked === true)}
                      />
                      <span>{t("selectAll")}</span>
                    </label>
                    {selectedFileIDs.length > 0 ? (
                      <>
                        <span>{t("selectedCount", { count: selectedFileIDs.length })}</span>
                        <Button
                          type="button"
                          variant="ghost"
                          size="xs"
                          className="ml-auto shadow-none"
                          disabled={retrying}
                          onClick={() => void retry(selectedFileIDs)}
                        >
                          {retrying ? <Spinner className="size-3" /> : <RotateCw className="size-3" strokeWidth={1.8} />}
                          {t("retrySelected")}
                        </Button>
                      </>
                    ) : null}
                  </div>
                  <div ref={listRef} className="min-h-0 space-y-px overflow-y-auto p-1 pt-0">
                    {tasks.map((task) => (
                      <EmbeddingTaskRow
                        key={task.fileID}
                        task={task}
                        selected={selectedFileIDSet.has(task.fileID)}
                        retrying={retryingFileIDSet.has(task.fileID)}
                        disabled={retrying}
                        onSelectedChange={(checked) => toggleSelection(task.fileID, checked)}
                        onRetry={() => void retry([task.fileID])}
                      />
                    ))}
                    {hasMore ? (
                      <div ref={loadMoreRef} className="flex h-8 items-center justify-center">
                        {loadingMore ? (
                          <Spinner className="size-3.5 text-muted-foreground" label={t("loadingMore")} />
                        ) : null}
                      </div>
                    ) : null}
                  </div>
                </>
              ) : (
                <div className="flex min-h-36 flex-1 items-center justify-center px-3 py-6 text-center text-xs text-muted-foreground">
                  {query.trim() ? t("emptySearch") : t("empty")}
                </div>
              )}
            </div>
          </div>
          <DialogFooter className="shrink-0 px-5 py-3">
            <span className="mr-auto self-center text-xs text-muted-foreground">{t("total", { count: total })}</span>
            <Button type="button" variant="ghost" onClick={() => onOpenChange(false)}>
              {t("close")}
            </Button>
          </DialogFooter>
        </DialogHeightTransition>
      </DialogContent>
    </Dialog>
  );
}

function EmbeddingTaskRow({
  task,
  selected,
  retrying,
  disabled,
  onSelectedChange,
  onRetry,
}: {
  task: AdminEmbeddingTask;
  selected: boolean;
  retrying: boolean;
  disabled: boolean;
  onSelectedChange: (checked: boolean) => void;
  onRetry: () => void;
}) {
  const t = useTranslations("adminFiles.embeddingTasks");
  const locale = useLocale();
  const FileIcon = resolveFileIcon(task);
  const status = isEmbedStatus(task.embedStatus) ? task.embedStatus : null;
  const blockedReason = !task.retryable && isRetryBlockedReason(task.retryBlockedReason)
    ? t(`blockedReasons.${task.retryBlockedReason}`)
    : "";
  const owner = task.userLabel.trim() || (task.userID > 0 ? String(task.userID) : "-");
  const updatedLabel = task.updatedAt ? t("updatedAt", { time: formatDateTime(task.updatedAt, locale) }) : "";
  const retryLabel = blockedReason || t("retry");

  return (
    <div className="flex items-start gap-2.5 rounded-md px-2 py-2 transition-colors hover:bg-muted/50">
      <span className="flex h-4 w-4 shrink-0 items-center justify-center pt-0.5">
        <Checkbox
          checked={selected}
          disabled={disabled || !task.retryable}
          aria-label={t("selectFile", { name: task.fileName })}
          onCheckedChange={(checked) => onSelectedChange(checked === true)}
        />
      </span>
      <FileIcon className="mt-0.5 size-3.5 shrink-0 text-muted-foreground" strokeWidth={1.5} />
      <div className="min-w-0 flex-1">
        <div className="flex min-w-0 items-center gap-1.5">
          <span className="min-w-0 truncate text-xs text-foreground" title={task.fileName}>
            {task.fileName}
          </span>
          {status ? (
            <Badge variant="secondary" className={cn("shrink-0", STATUS_BADGE_CLASS_NAMES[status])}>
              {t(`status.${status}`)}
            </Badge>
          ) : null}
          {task.stalled ? (
            <Badge variant="secondary" className="shrink-0 bg-amber-500/10 text-amber-700 dark:text-amber-300">
              {t("stalled")}
            </Badge>
          ) : null}
        </div>
        <p className="mt-0.5 truncate text-[10px] text-muted-foreground" title={task.fileID}>
          {/* The MIME type tells admins why a file is unsupported, e.g. video/mp4 or image/bmp. */}
          {[owner, task.mimeType, formatBytes(task.sizeBytes), updatedLabel].filter(Boolean).join(" · ")}
        </p>
        {task.embedError ? (
          <p
            className={cn(
              "mt-1 line-clamp-2 break-words text-[11px] leading-snug",
              status === "failed" ? "text-destructive/90" : "text-muted-foreground",
            )}
            title={task.embedError}
          >
            {task.embedError}
          </p>
        ) : null}
      </div>
      {/* The span wrapper keeps the tooltip reachable while the button is disabled. */}
      <Tooltip>
        <TooltipTrigger asChild>
          <span className="shrink-0">
            <Button
              type="button"
              variant="ghost"
              size="icon-sm"
              className="text-muted-foreground hover:text-foreground"
              aria-label={retryLabel}
              disabled={disabled || !task.retryable}
              onClick={onRetry}
            >
              {retrying ? <Spinner className="size-3" /> : <RotateCw className="size-3.5" strokeWidth={1.8} />}
            </Button>
          </span>
        </TooltipTrigger>
        <TooltipContent side="left">{retryLabel}</TooltipContent>
      </Tooltip>
    </div>
  );
}
