"use client";

import { X } from "lucide-react";
import { useTranslations } from "next-intl";

import { Button } from "@/components/ui/button";
import { SpinnerLabel } from "@/components/ui/spinner";
type ChatScreenshotSelectionBarProps = {
  selectedCount: number;
  totalCount: number;
  capturing: boolean;
  onSelectAll: () => void;
  onClearSelection: () => void;
  onCapture: () => void;
  onExit: () => void;
};

export function ChatScreenshotSelectionBar({
  selectedCount,
  totalCount,
  capturing,
  onSelectAll,
  onClearSelection,
  onCapture,
  onExit,
}: ChatScreenshotSelectionBarProps) {
  const t = useTranslations("chat.screenshot");
  const allSelected = totalCount > 0 && selectedCount >= totalCount;

  return (
    <div className="flex w-full flex-col gap-2 rounded-lg bg-muted/25 px-2 py-1.5 sm:flex-row sm:items-center sm:justify-between">
      <div className="flex min-w-0 items-center gap-2">
        <span className="min-w-0 truncate text-xs font-medium text-foreground">
          {t("captureSelect")}
        </span>
        <span className="shrink-0 text-[11px] text-muted-foreground tabular-nums">
          {selectedCount}/{totalCount}
        </span>
      </div>
      <div className="flex min-w-0 items-center justify-end gap-1">
        <Button
          type="button"
          variant="ghost"
          size="sm"
          className="h-7 shrink-0 rounded-md px-2 text-xs text-muted-foreground shadow-none hover:bg-muted hover:text-foreground"
          onClick={allSelected ? onClearSelection : onSelectAll}
        >
          {allSelected ? t("clearSelection") : t("selectAll")}
        </Button>
        <Button
          type="button"
          size="sm"
          className="h-7 shrink-0 rounded-md px-2.5 text-xs shadow-none"
          disabled={capturing || selectedCount === 0}
          onClick={onCapture}
        >
          {capturing ? (
            <SpinnerLabel>{t("generating")}</SpinnerLabel>
          ) : t("captureSelected")}
        </Button>
        <Button
          type="button"
          variant="ghost"
          size="icon-sm"
          className="size-7 shrink-0 rounded-md text-muted-foreground shadow-none hover:bg-muted hover:text-foreground"
          aria-label={t("exitSelection")}
          title={t("exitSelection")}
          onClick={onExit}
        >
          <X className="size-4" />
        </Button>
      </div>
    </div>
  );
}
