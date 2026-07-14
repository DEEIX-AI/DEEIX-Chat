"use client";

import * as React from "react";
import { ChevronLeft, Download, ExternalLink, LoaderCircle, Trash2 } from "lucide-react";
import { useTranslations } from "next-intl";

import { formatBytes, formatDateTime, resolveFileExtension, resolveFileIcon } from "@/shared/lib/file-display";
import type { FilePreviewState } from "@/features/files/hooks/use-file-preview";
import { Button } from "@/components/ui/button";
import { resolveFileProcessingBadge, resolveFileProcessingToneClass } from "@/shared/lib/file-processing";
import type { FileObjectDTO } from "@/shared/api/file.types";
import { Avatar, AvatarImage, AvatarFallback } from "@/components/ui/avatar";
import { useAppLocale } from "@/i18n/app-i18n-provider";
import { cn } from "@/lib/utils";

type ContentHeaderProps = {
  file: FileObjectDTO | null;
  preview: FilePreviewState;
  deleting: boolean;
  onBack?: () => void;
  onOpen: () => void;
  onDownload: () => void;
  onDeleteRequest: (file: FileObjectDTO) => void;
  onToggleRagOptOut: (fileID: string, current: boolean) => Promise<void>;
};

function resolveRawFileTypeLabel(file: FileObjectDTO): string {
  const mimeType = file.mimeType.trim().toLowerCase();
  if (mimeType && mimeType !== "application/octet-stream") {
    return mimeType;
  }

  const extension = resolveFileExtension(file.fileName);
  if (extension) {
    return extension;
  }

  return "unknown";
}

export function ContentHeader({
  file,
  preview,
  deleting,
  onBack,
  onOpen,
  onDownload,
  onDeleteRequest,
  onToggleRagOptOut,
}: ContentHeaderProps) {
  const tCommon = useTranslations("common.actions");
  const t = useTranslations("files");
  const tStatus = useTranslations("files.status");
  const { locale } = useAppLocale();

  if (!file) {
    return null;
  }

  const fileIcon = resolveFileIcon(file);
  const fileTypeLabel = resolveRawFileTypeLabel(file);
  const isReady = preview.status === "ready";
  const thumbnail = isReady && preview.isImage ? preview.objectURL : null;
  const avatarFallback = file.fileName.slice(0, 2).toUpperCase() || "?";
  const processingBadge = resolveFileProcessingBadge({
    fileCategory: file.fileCategory,
    processingStatus: file.processingStatus,
    processingReady: file.processingReady,
    processingErrorCode: file.processingErrorCode,
    processingErrorMessage: file.processingErrorMessage,
    extractStatus: file.extractStatus,
    embedStatus: file.embedStatus,
    embedError: file.embedError,
  }, (key, values) => tStatus(key, values));

  return (
    <div className="flex min-w-0 shrink-0 items-center justify-between gap-4 border-b border-border/40 px-3 py-4 md:px-6">
      <div className="flex min-w-0 flex-1 items-center gap-3">
        {onBack ? (
          <Button
            type="button"
            variant="ghost"
            size="icon"
            className="-ml-1 size-8 shrink-0 rounded-md text-muted-foreground hover:bg-muted hover:text-foreground md:hidden"
            aria-label={tCommon("back")}
            onClick={onBack}
          >
            <ChevronLeft className="size-4" />
          </Button>
        ) : null}

        <div className={cn("flex size-7 shrink-0 items-center justify-center overflow-hidden rounded-md bg-background/70", onBack && "hidden md:flex")}>
          {thumbnail ? (
            <Avatar className="h-7 w-7 rounded-md">
              <AvatarImage src={thumbnail} alt={file.fileName} />
              <AvatarFallback className="rounded-md bg-background text-base font-medium text-foreground">{avatarFallback}</AvatarFallback>
            </Avatar>
          ) : (
            React.createElement(fileIcon, { className: "size-5 text-muted-foreground" })
          )}
        </div>

        <div className="min-w-0 flex-1">
          <p className="truncate text-sm font-medium text-foreground">{file.fileName}</p>
          <div className="flex min-w-0 flex-wrap items-center gap-1.5 pt-0.5">
            <p className="text-[11px] text-muted-foreground">
              {formatDateTime(file.createdAt, locale)}
              <span className="px-1.5 text-border">|</span>
              {fileTypeLabel}
              <span className="px-1.5 text-border">|</span>
              {formatBytes(file.sizeBytes)}
            </p>
            <span
              className={cn(
                "inline-flex rounded-md border px-1.5 py-0.5 text-[10px] font-medium",
                resolveFileProcessingToneClass(processingBadge.tone),
              )}
              title={processingBadge.detail}
            >
              {processingBadge.label}
            </span>
            {file.embedStatus === "ready" ? (
              <Button
                type="button"
                variant="ghost"
                size="sm"
                onClick={() => onToggleRagOptOut(file.fileID, file.ragOptOut)}
                title={file.ragOptOut ? t("rag.disabledTitle") : t("rag.enabledTitle")}
                className={cn(
                  "h-auto gap-0.5 rounded-md border px-1.5 py-0.5 text-[10px] font-medium shadow-none",
                  file.ragOptOut
                    ? "border-border/50 text-muted-foreground/60 hover:border-border hover:text-muted-foreground"
                    : "border-primary/25 bg-primary/10 text-primary hover:bg-primary/15",
                )}
              >
                <span>⚡</span>
                <span>{file.ragOptOut ? t("rag.disabled") : t("rag.enabled")}</span>
              </Button>
            ) : null}
          </div>
        </div>
      </div>

      <div className="flex shrink-0 items-center gap-1">
        <Button
          type="button"
          variant="ghost"
          size="sm"
          className="size-6"
          onClick={onOpen}
          disabled={!isReady}
          aria-label={t("actions.open")}
          title={t("actions.open")}
        >
          <ExternalLink className="size-3.5" strokeWidth={1.6} />
        </Button>
        <Button
          type="button"
          variant="ghost"
          size="sm"
          className="size-6"
          onClick={onDownload}
          disabled={!isReady}
          aria-label={t("actions.download")}
          title={t("actions.download")}
        >
          <Download className="size-3.5" strokeWidth={1.6} />
        </Button>
        <Button
          type="button"
          variant="ghost"
          size="sm"
          className="size-6"
          onClick={() => onDeleteRequest(file)}
          disabled={deleting}
          aria-label={t("actions.delete")}
          title={t("actions.delete")}
        >
          {deleting ? <LoaderCircle className="size-3.5 animate-spin" strokeWidth={1.6} /> : <Trash2 className="size-3.5" strokeWidth={1.6} />}
        </Button>
      </div>
    </div>
  );
}
