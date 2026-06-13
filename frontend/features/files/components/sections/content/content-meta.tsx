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
import { resolveEmbedStatusLabel, resolveExtractStatusLabel, resolveFileProcessingBadge } from "@/shared/lib/file-processing";
import type { FileObjectDTO } from "@/shared/api/file.types";
import { useAppLocale } from "@/i18n/app-i18n-provider";

function MetaRow({ label, value, mono = false }: { label: string; value: string; mono?: boolean }) {
  return (
    <div className="grid gap-1 py-2.5 sm:grid-cols-[96px_minmax(0,1fr)] sm:gap-3">
      <dt className="text-[11px] text-muted-foreground">{label}</dt>
      <dd className={mono ? "break-all font-mono text-[12px] text-foreground" : "text-[13px] text-foreground"}>{value}</dd>
    </div>
  );
}

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

  return (
    <Drawer shouldScaleBackground={false} container={container}>
      <div className="absolute inset-x-0 bottom-0 z-20 px-4 pb-3">
        <div className="flex justify-center">
          <div className="rounded-full bg-background/88 shadow-[0_16px_36px_-28px_color-mix(in_oklch,var(--foreground)_28%,transparent)] backdrop-blur-md">
            <DrawerTrigger asChild>
              <button
                type="button"
                className="inline-flex h-9 items-center justify-center gap-1.5 px-3 text-xs font-medium text-muted-foreground hover:text-foreground"
              >
                <span>{t("moreInfo")}</span>
              </button>
            </DrawerTrigger>
          </div>
        </div>
      </div>


      <DrawerContent className="mx-auto w-full max-w-[720px] bg-background">
        <DrawerHeader>
          <DrawerTitle>{t("moreInfo")}</DrawerTitle>
          <DrawerDescription className="text-xs">{t("description")}</DrawerDescription>
        </DrawerHeader>

        <div className="max-h-[min(72vh,640px)] overflow-y-auto px-8 py-6">
          <dl>
            <MetaRow label={t("id")} value={file.fileID} mono />
            <MetaRow label={t("category")} value={file.fileCategory || t("unknown")} />
            <MetaRow label={t("detectedMime")} value={file.detectedMIME || file.mimeType || t("unknown")} />
            <MetaRow label={t("processingStatus")} value={processingBadge.label} />
            <MetaRow label={t("extractStatus")} value={resolveExtractStatusLabel(file.extractStatus, (key, values) => tStatus(key, values))} />
            <MetaRow label={t("embedStatus")} value={resolveEmbedStatusLabel(file.embedStatus, (key, values) => tStatus(key, values))} />
            {file.chunkCount > 0 ? <MetaRow label={t("chunks")} value={`${file.chunkCount}`} /> : null}
            <MetaRow label={t("size")} value={formatBytes(file.sizeBytes)} />
            <MetaRow label={t("purpose")} value={file.purpose || t("unset")} />
            <MetaRow label={t("createdAt")} value={formatDateTime(file.createdAt, locale)} />
            <MetaRow label={t("updatedAt")} value={formatDateTime(file.updatedAt, locale)} />
            <MetaRow label={t("expiresAt")} value={formatDateTime(file.expiresAt, locale)} />
            <MetaRow label={t("sha256")} value={file.sha256} mono />
            {file.processingErrorMessage ? <MetaRow label={t("failureReason")} value={file.processingErrorMessage} /> : null}
            {file.embedError ? <MetaRow label={t("embedError")} value={file.embedError} /> : null}
          </dl>
        </div>
      </DrawerContent>
    </Drawer>
  );
}
