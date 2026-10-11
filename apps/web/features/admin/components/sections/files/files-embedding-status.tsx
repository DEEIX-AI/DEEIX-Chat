"use client";

import { useTranslations } from "next-intl";
import * as React from "react";

import { Button } from "@/components/ui/button";
import type { AdminEmbeddingIndexStatus, AdminEmbeddingTaskBucket } from "@/features/admin/api";
import { cn } from "@/lib/utils";
import { SettingsFieldInset } from "@/shared/components/settings-layout";

import { type EmbeddingTaskBucketCounts, FilesEmbeddingTasksDialog } from "./files-embedding-tasks-dialog";

type StatusCell = {
  // Every cell maps to a task bucket, so any count can open its file list.
  key: AdminEmbeddingTaskBucket;
  label: string;
  value: number;
  valueClassName: string;
  detail?: string;
};

function resolveBucketCounts(status: AdminEmbeddingIndexStatus | null): EmbeddingTaskBucketCounts | null {
  if (!status) return null;
  return {
    ready: status.readyCount,
    pending: status.pendingCount,
    failed: status.failedCount,
    stale: status.staleCount,
    empty: status.emptyCount,
    unsupported: status.unsupportedCount,
  };
}

/**
 * Index health for the embedding settings group. Every non-ready count opens
 * the platform-wide task dialog filtered to that bucket, where files can be
 * retried individually instead of re-running the whole index.
 */
export function FilesEmbeddingStatusPanel({
  status,
  loading,
  reindexing,
  disabled,
  onReindex,
  onStatusRefresh,
}: {
  status: AdminEmbeddingIndexStatus | null;
  loading: boolean;
  reindexing: boolean;
  disabled: boolean;
  onReindex: (includeEmpty: boolean) => void;
  onStatusRefresh: () => void;
}) {
  const t = useTranslations("adminFiles.embeddingStatus");
  const [tasksOpen, setTasksOpen] = React.useState(false);
  const [tasksBucket, setTasksBucket] = React.useState<AdminEmbeddingTaskBucket>("pending");
  const counts = resolveBucketCounts(status);
  const reindexRunning = status?.reindexRunning ?? false;
  const actionDisabled = reindexing || reindexRunning || loading || disabled;

  const openTasks = (bucket: AdminEmbeddingTaskBucket) => {
    setTasksBucket(bucket);
    setTasksOpen(true);
  };

  const cells: StatusCell[] = status
    ? [
        { key: "ready", label: t("ready"), value: status.readyCount, valueClassName: "text-green-600 dark:text-green-400" },
        {
          key: "pending",
          label: t("pending"),
          value: status.pendingCount,
          valueClassName: "text-muted-foreground",
          detail: status.stalledCount > 0 ? t("stalled", { count: status.stalledCount }) : undefined,
        },
        {
          key: "failed",
          label: t("failed"),
          value: status.failedCount,
          valueClassName: status.failedCount > 0 ? "text-destructive" : "text-muted-foreground",
        },
        {
          key: "stale",
          label: t("stale"),
          value: status.staleCount,
          valueClassName: status.staleCount > 0 ? "text-amber-600 dark:text-amber-400" : "text-muted-foreground",
        },
        { key: "empty", label: t("noText"), value: status.emptyCount, valueClassName: "text-muted-foreground" },
        { key: "unsupported", label: t("unsupported"), value: status.unsupportedCount, valueClassName: "text-muted-foreground" },
      ]
    : [];

  return (
    <SettingsFieldInset className="min-w-0 space-y-3">
      <div className="flex min-w-0 items-center justify-between gap-3">
        <div className="min-w-0 space-y-0.5">
          <p className="text-xs font-medium">{t("title")}</p>
          {status?.modelSignature ? (
            <p className="min-w-0 truncate font-mono text-[10px] text-muted-foreground" title={status.modelSignature}>
              {status.modelSignature}
            </p>
          ) : (
            <p className="text-[10px] text-muted-foreground">{t("noSignature")}</p>
          )}
        </div>
        <div className="flex shrink-0 items-center gap-1.5">
          {status && status.emptyCount > 0 ? (
            <Button
              type="button"
              size="sm"
              variant="ghost"
              className="h-7 px-2 text-xs shadow-none"
              disabled={actionDisabled}
              onClick={() => onReindex(true)}
            >
              {t("reindexIncludingNoText")}
            </Button>
          ) : null}
          <Button
            type="button"
            size="sm"
            variant="outline"
            className="h-7 px-2 text-xs shadow-none"
            disabled={actionDisabled}
            onClick={() => onReindex(false)}
          >
            {reindexing ? t("reindexing") : reindexRunning ? t("reindexRunning") : t("reindex")}
          </Button>
        </div>
      </div>
      {status ? (
        <div className="grid min-w-0 grid-cols-3 overflow-hidden rounded-md bg-muted/30 text-center sm:grid-cols-6">
          {cells.map((cell) => {
            const content = (
              <>
                <p className={cn("text-sm font-semibold tabular-nums", cell.valueClassName)}>{cell.value}</p>
                <p className="mt-0.5 text-[10px] text-muted-foreground">{cell.label}</p>
                {cell.detail ? (
                  <p className="mt-0.5 text-[10px] text-amber-600 dark:text-amber-400">{cell.detail}</p>
                ) : null}
              </>
            );
            return (
              <button
                key={cell.key}
                type="button"
                className="px-3 py-2.5 transition-colors hover:bg-muted/60 focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-inset focus-visible:ring-ring"
                aria-label={t("viewBucket", { label: cell.label, count: cell.value })}
                onClick={() => openTasks(cell.key)}
              >
                {content}
              </button>
            );
          })}
        </div>
      ) : loading ? (
        <div className="grid min-w-0 grid-cols-3 overflow-hidden rounded-md bg-muted/30 sm:grid-cols-6" aria-hidden="true">
          {Array.from({ length: 6 }).map((_, index) => (
            <div key={`embedding-status-skeleton-${index}`} className="px-3 py-2.5">
              <div className="mx-auto h-4 w-8 animate-pulse rounded-sm bg-muted/70" />
              <div className="mx-auto mt-1.5 h-2.5 w-10 animate-pulse rounded-sm bg-muted/60" />
            </div>
          ))}
        </div>
      ) : (
        <p className="text-[11px] text-muted-foreground">{t("empty")}</p>
      )}
      {status ? (
        <p className="text-[11px] leading-relaxed text-muted-foreground">
          {reindexRunning ? t("runningHint") : t("hint")}
        </p>
      ) : null}
      {status?.needsReindex ? (
        <p className="text-[11px] text-amber-600 dark:text-amber-400">{t("needsReindex")}</p>
      ) : null}
      <FilesEmbeddingTasksDialog
        open={tasksOpen}
        onOpenChange={setTasksOpen}
        bucket={tasksBucket}
        onBucketChange={setTasksBucket}
        counts={counts}
        onTasksChanged={onStatusRefresh}
      />
    </SettingsFieldInset>
  );
}
