"use client";

import { useTranslations } from "next-intl";

import { formatBytes, formatDateTime } from "@/shared/lib/file-display";
import {
  Drawer,
  DrawerContent,
  DrawerDescription,
  DrawerHeader,
  DrawerTitle,
  DrawerTrigger,
} from "@/components/ui/drawer";
import { Button } from "@/components/ui/button";
import { resolveEmbedStatusLabel, resolveExtractStatusLabel, resolveFileProcessingBadge } from "@/shared/lib/file-processing";
import type { FileObjectDTO } from "@/shared/api/file.types";
import { useAppLocale } from "@/i18n/app-i18n-provider";

type ContentMetaProps = {
  file: FileObjectDTO;
  container: HTMLElement | null;
};

export function ContentMeta({ file, container }: ContentMetaProps) {
  const t = useTranslations("files.meta");
  const tStatus = useTranslations("files.status");
  const { locale } = useAppLocale();
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
  const metaRows = [
    { label: t("id"), value: file.fileID, mono: true },
    { label: t("category"), value: file.fileCategory || t("unknown") },
    { label: t("detectedMime"), value: file.detectedMIME || file.mimeType || t("unknown") },
    { label: t("processingStatus"), value: processingBadge.label },
    { label: t("extractStatus"), value: resolveExtractStatusLabel(file.extractStatus, (key, values) => tStatus(key, values)) },
    { label: t("embedStatus"), value: resolveEmbedStatusLabel(file.embedStatus, (key, values) => tStatus(key, values)) },
    ...(file.chunkCount > 0 ? [{ label: t("chunks"), value: `${file.chunkCount}` }] : []),
    { label: t("size"), value: formatBytes(file.sizeBytes) },
    { label: t("purpose"), value: file.purpose || t("unset") },
    { label: t("createdAt"), value: formatDateTime(file.createdAt, locale) },
    { label: t("updatedAt"), value: formatDateTime(file.updatedAt, locale) },
    { label: t("expiresAt"), value: formatDateTime(file.expiresAt, locale) },
    { label: t("sha256"), value: file.sha256, mono: true },
    ...(file.processingErrorMessage ? [{ label: t("failureReason"), value: file.processingErrorMessage }] : []),
    ...(file.embedError ? [{ label: t("embedError"), value: file.embedError }] : []),
  ];

  return (
    <Drawer shouldScaleBackground={false} container={container}>
      <div className="absolute inset-x-0 bottom-0 z-20 px-4 pb-3">
        <div className="flex justify-center">
          <div className="rounded-full bg-background/90">
            <DrawerTrigger asChild>
              <Button
                type="button"
                variant="ghost"
                size="sm"
                className="h-9 rounded-full px-3 text-xs text-muted-foreground shadow-none hover:bg-transparent hover:text-foreground"
              >
                <span>{t("moreInfo")}</span>
              </Button>
            </DrawerTrigger>
          </div>
        </div>
      </div>

      <DrawerContent className="mx-auto w-full max-w-[720px] bg-background">
        <DrawerHeader>
          <DrawerTitle className="text-sm">{t("moreInfo")}</DrawerTitle>
          <DrawerDescription className="text-xs">{t("description")}</DrawerDescription>
        </DrawerHeader>

        <div className="max-h-[min(72vh,640px)] overflow-y-auto px-8 py-6">
          <dl>
            {metaRows.map((row) => (
              <div key={row.label} className="grid gap-1 py-2.5 text-[11px] sm:grid-cols-[96px_minmax(0,1fr)] sm:gap-3">
                <dt className="text-muted-foreground">{row.label}</dt>
                <dd className={row.mono ? "break-all font-mono text-foreground" : "text-foreground"}>{row.value}</dd>
              </div>
            ))}
          </dl>
        </div>
      </DrawerContent>
    </Drawer>
  );
}
