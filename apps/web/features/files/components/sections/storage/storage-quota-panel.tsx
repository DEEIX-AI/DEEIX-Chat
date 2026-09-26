"use client";

import { useTranslations } from "next-intl";

import { formatBytes } from "@/shared/lib/file-display";
import type { UserStorageQuotaDTO } from "@/shared/api/file.types";

type StorageQuotaPanelProps = {
  quota: UserStorageQuotaDTO | null;
};

function quotaUsagePercent(quota: UserStorageQuotaDTO): number {
  if (quota.quotaBytes <= 0) {
    return 0;
  }
  return Math.min(100, Math.max(0, (Math.max(0, quota.usedBytes) / quota.quotaBytes) * 100));
}

export function StorageQuotaPanel({ quota }: StorageQuotaPanelProps) {
  const t = useTranslations("files");
  if (!quota) {
    return null;
  }

  const unlimited = quota.quotaBytes <= 0;
  const percent = quotaUsagePercent(quota);
  const remainingBytes = unlimited ? 0 : Math.max(0, quota.quotaBytes - Math.max(0, quota.usedBytes));

  return (
    <div className="shrink-0 px-1 pb-2 pt-1">
      <div className="rounded-md bg-muted/35 px-2 py-2">
        <div className="flex min-w-0 items-center justify-between gap-2">
          <span className="truncate text-xs font-medium text-foreground">{t("storage.title")}</span>
          <span className="min-w-0 truncate text-right text-[11px] text-muted-foreground">
            {unlimited
              ? t("storage.usedUnlimited", { used: formatBytes(quota.usedBytes) })
              : t("storage.usedTotal", { used: formatBytes(quota.usedBytes), total: formatBytes(quota.quotaBytes) })}
          </span>
        </div>
        {!unlimited ? (
          <div className="mt-2 h-1 overflow-hidden rounded-full bg-muted">
            <div
              className="h-full rounded-full bg-foreground/70 transition-[width] duration-300"
              style={{ width: `${percent}%` }}
            />
          </div>
        ) : null}
        <div className="mt-1.5 flex min-w-0 items-center justify-between gap-2 text-[11px] text-muted-foreground">
          <span className="truncate">
            {unlimited ? t("storage.unlimited") : t("storage.usedPercent", { percent: Math.round(percent) })}
          </span>
          <span className="min-w-0 truncate text-right">
            {unlimited ? t("storage.noLimit") : t("storage.remaining", { remaining: formatBytes(remainingBytes) })}
          </span>
        </div>
      </div>
    </div>
  );
}
