"use client";

import * as React from "react";
import { Camera, X } from "lucide-react";
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
    <div className="flex w-full flex-wrap items-center justify-between gap-2 rounded-lg border border-border/60 bg-muted/40 px-3 py-2">
      <span className="text-xs font-medium text-muted-foreground">
        {t("selectedCount", { count: selectedCount })}
      </span>
      <div className="flex items-center gap-1.5">
        <Button
          type="button"
          variant="ghost"
          size="sm"
          onClick={allSelected ? onClearSelection : onSelectAll}
        >
          {allSelected ? t("clearSelection") : t("selectAll")}
        </Button>
        <Button
          type="button"
          size="sm"
          disabled={capturing || selectedCount === 0}
          onClick={onCapture}
        >
          {capturing ? (
            <SpinnerLabel>{t("generating")}</SpinnerLabel>
          ) : (
            <>
              <Camera className="size-3.5" />
              {t("captureSelected")}
            </>
          )}
        </Button>
        <Button
          type="button"
          variant="ghost"
          size="icon-sm"
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
