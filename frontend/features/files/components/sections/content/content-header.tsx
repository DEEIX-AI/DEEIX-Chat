"use client";

import { ChevronLeft, DatabaseZap, Download, ExternalLink, LoaderCircle, Trash2 } from "lucide-react";
import { useTranslations } from "next-intl";
import * as React from "react";
import { AnimatedText } from "@/components/ui/animated-text";
import { Avatar, AvatarFallback, AvatarImage } from "@/components/ui/avatar";
import { Button } from "@/components/ui/button";
import type { FilePreviewState } from "@/features/files/hooks/use-file-preview";
import { useAppLocale } from "@/i18n/app-i18n-provider";
import { cn } from "@/lib/utils";
import type { FileObjectDTO } from "@/shared/api/file.types";
import { formatBytes, formatDateTime, resolveFileExtension, resolveFileIcon } from "@/shared/lib/file-display";
import { canManuallyVectorizeFile, isVectorIndexOutdated, resolveFileProcessingBadge, resolveFileProcessingToneClass } from "@/shared/lib/file-processing";

type ContentHeaderProps = {
  file: FileObjectDTO | null;
  preview: FilePreviewState;
  deleting: boolean;
  vectorizing: boolean;
  onBack?: () => void;
  onOpen: () => void;
  onDownload: () => void;
  onDeleteRequest: (file: FileObjectDTO) => void;
  onToggleRagOptOut: (fileID: string, current: boolean) => Promise<void>;
  onVectorize: (fileID: string) => Promise<void>;
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
  vectorizing,
  onBack,
  onOpen,
  onDownload,
  onDeleteRequest,
  onToggleRagOptOut,
  onVectorize,
}: ContentHeaderProps) {
  const tCommon = useTranslations("common.actions");
  const t = useTranslations("files");
  const tStatus = useTranslations("files.status");
  const { locale } = useAppLocale();

  if (!file) {
    return <div aria-hidden="true" className="h-15 min-w-0 shrink-0 border-b border-border/40" />;
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
    <div
      className="flex h-15 min-w-0 shrink-0 items-center justify-between gap-3 border-b border-border/40 px-3 md:px-5"
      data-animated-text-scroll-trigger
    >
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
          <AnimatedText
            text={file.fileName}
            className="text-[13px] font-medium text-foreground"
            textClassName="text-current"
            scrollOverflow
          />
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
                "inline-flex rounded-md px-1.5 py-0.5 text-[10px] font-medium",
                resolveFileProcessingToneClass(processingBadge.tone),
                "border-0",
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
                  "h-auto rounded-md border-0 px-1.5 py-0.5 text-[10px] font-medium shadow-none",
                  file.ragOptOut
                    ? "bg-muted text-muted-foreground/70 hover:bg-muted/80 hover:text-muted-foreground"
                    : "bg-primary/10 text-primary hover:bg-primary/15 hover:text-primary",
                )}
              >
                <span>{file.ragOptOut ? t("rag.disabled") : t("rag.enabled")}</span>
              </Button>
            ) : null}
          </div>
        </div>
      </div>

      <div className="flex shrink-0 items-center gap-1">
        {canManuallyVectorizeFile(file) ? (
          <Button
            type="button"
            variant="ghost"
            size="sm"
            className="size-6"
            onClick={() => onVectorize(file.fileID)}
            disabled={vectorizing}
            aria-label={t(isVectorIndexOutdated(file) ? "actions.updateIndex" : "actions.vectorize")}
            title={t(isVectorIndexOutdated(file) ? "actions.updateIndex" : "actions.vectorize")}
          >
            {vectorizing ? <LoaderCircle className="size-3.5 animate-spin" strokeWidth={1.6} /> : <DatabaseZap className="size-3.5" strokeWidth={1.6} />}
          </Button>
        ) : null}
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
