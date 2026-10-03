"use client";

import { RefreshCw } from "lucide-react";
import { useLocale, useTranslations } from "next-intl";

import { Button } from "@/components/ui/button";
import { Tooltip, TooltipContent, TooltipTrigger } from "@/components/ui/tooltip";
import { useAdminModelsReasoningSync } from "@/features/admin/hooks/use-admin-models-reasoning-sync";
import { cn } from "@/lib/utils";

function formatSyncedAt(raw: string | null, locale: string): string | null {
  if (!raw) return null;
  const date = new Date(raw);
  if (Number.isNaN(date.getTime())) return raw;
  return new Intl.DateTimeFormat(locale, { dateStyle: "medium", timeStyle: "short" }).format(date);
}

/**
 * Syncs the reasoning capability data used for models without their own reasoning parameters.
 * The data is only a runtime fallback and is never written into model configs; the tooltip shows
 * the last sync. Mounting (opening the capabilities dialog) also refreshes stale data on the server.
 */
export function ModelsReasoningSyncButton() {
  const t = useTranslations("adminModels.reasoningSync");
  const locale = useLocale();
  const { status, syncing, sync } = useAdminModelsReasoningSync();
  const syncedAt = formatSyncedAt(status?.fetchedAt ?? null, locale);

  return (
    <Tooltip>
      <TooltipTrigger asChild>
        <Button
          type="button"
          variant="secondary"
          size="sm"
          className="h-7 shrink-0 gap-1 px-2 text-xs font-normal shadow-none"
          disabled={syncing}
          onClick={() => void sync()}
          // The dialog auto-focuses this first button on open; only keyboard focus should show the tooltip.
          onFocus={(event) => {
            if (!event.currentTarget.matches(":focus-visible")) event.preventDefault();
          }}
        >
          <RefreshCw className={cn("size-3.5", syncing && "animate-spin")} />
          {syncing ? t("syncing") : t("button")}
        </Button>
      </TooltipTrigger>
      <TooltipContent className="max-w-72">
        <div className="space-y-1">
          <p>{t("description")}</p>
          {syncedAt && status ? (
            <p className="text-background/70">{t("lastSynced", { time: syncedAt, count: status.modelCount })}</p>
          ) : null}
          {status?.lastError ? (
            <p className="line-clamp-2 break-all text-background/70">{t("lastError", { error: status.lastError })}</p>
          ) : null}
        </div>
      </TooltipContent>
    </Tooltip>
  );
}
