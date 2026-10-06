"use client";

import { RefreshCw } from "lucide-react";
import { useLocale, useTranslations } from "next-intl";

import { Button } from "@/components/ui/button";
import { Tooltip, TooltipContent, TooltipTrigger } from "@/components/ui/tooltip";
import { cn } from "@/lib/utils";
import type { AdminLLMModelCatalogStatus } from "@/features/admin/api/llm-types";

function formatSyncedAt(raw: string | null | undefined, locale: string): string | null {
  if (!raw) return null;
  const date = new Date(raw);
  if (Number.isNaN(date.getTime())) return raw;
  return new Intl.DateTimeFormat(locale, { dateStyle: "medium", timeStyle: "short" }).format(date);
}

type ModelsCatalogSyncButtonProps = {
  modelsDevStatus: AdminLLMModelCatalogStatus | null;
  openRouterFetchedAt?: string;
  syncing: boolean;
  disabled?: boolean;
  onSync: () => void;
};

/** Syncs every catalog behind the capability rows (models.dev and OpenRouter); the tooltip shows the last sync of each. */
export function ModelsCatalogSyncButton({
  modelsDevStatus,
  openRouterFetchedAt,
  syncing,
  disabled = false,
  onSync,
}: ModelsCatalogSyncButtonProps) {
  const t = useTranslations("adminModels.sheet.catalogSync");
  const locale = useLocale();
  const modelsDevSyncedAt = formatSyncedAt(modelsDevStatus?.fetchedAt, locale);
  const openRouterSyncedAt = formatSyncedAt(openRouterFetchedAt, locale);

  return (
    <Tooltip>
      <TooltipTrigger asChild>
        <Button
          type="button"
          variant="ghost"
          size="sm"
          className="h-6 shrink-0 gap-1 px-1.5 text-[11px] font-normal text-muted-foreground hover:text-foreground"
          disabled={disabled || syncing}
          onClick={onSync}
        >
          <RefreshCw className={cn("size-3", syncing && "animate-spin")} aria-hidden="true" />
          {syncing ? t("syncing") : t("button")}
        </Button>
      </TooltipTrigger>
      <TooltipContent className="max-w-72">
        <div className="space-y-1">
          <p>{t("description")}</p>
          <p className="text-background/70">
            {t("lastSynced", { source: "models.dev", time: modelsDevSyncedAt ?? t("never") })}
          </p>
          <p className="text-background/70">
            {t("lastSynced", { source: "OpenRouter", time: openRouterSyncedAt ?? t("never") })}
          </p>
          {modelsDevStatus?.lastError ? (
            <p className="line-clamp-2 break-all text-background/70">{t("lastError", { error: modelsDevStatus.lastError })}</p>
          ) : null}
        </div>
      </TooltipContent>
    </Tooltip>
  );
}
