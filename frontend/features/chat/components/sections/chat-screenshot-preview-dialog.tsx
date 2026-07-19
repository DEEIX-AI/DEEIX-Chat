"use client";

import { useTranslations } from "next-intl";

import { Button } from "@/components/ui/button";
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from "@/components/ui/dialog";

type ChatScreenshotPreviewDialogProps = {
  open: boolean;
  onOpenChange: (open: boolean) => void;
  previewURL: string | null;
  clipboardSupported: boolean;
  onDownload: () => void;
  onCopy: () => void | Promise<void>;
};

export function ChatScreenshotPreviewDialog({
  open,
  onOpenChange,
  previewURL,
  clipboardSupported,
  onDownload,
  onCopy,
}: ChatScreenshotPreviewDialogProps) {
  const t = useTranslations("chat.screenshot");

  return (
    <Dialog open={open} onOpenChange={onOpenChange}>
      <DialogContent className="flex max-h-[min(86vh,760px)] w-[calc(100vw-2rem)] flex-col gap-0 overflow-hidden p-0 sm:max-w-[680px]">
        <DialogHeader className="shrink-0 px-4 py-4">
          <DialogTitle>{t("previewTitle")}</DialogTitle>
          <DialogDescription>{t("previewDescription")}</DialogDescription>
        </DialogHeader>

        <div className="min-h-0 flex-1 overflow-y-auto px-4 py-2">
          <div className="overflow-auto rounded-lg border border-border/60 bg-muted/30 p-3">
            {previewURL ? (
              <img src={previewURL} alt={t("previewTitle")} className="mx-auto block h-auto w-full rounded-md" />
            ) : null}
          </div>
        </div>

        <DialogFooter className="shrink-0 px-4 py-3">
          <Button type="button" variant="ghost" onClick={() => onOpenChange(false)}>
            {t("close")}
          </Button>
          {clipboardSupported ? (
            <Button type="button" variant="ghost" onClick={() => void onCopy()}>
              {t("copy")}
            </Button>
          ) : null}
          <Button type="button" onClick={onDownload}>
            {t("download")}
          </Button>
        </DialogFooter>
      </DialogContent>
    </Dialog>
  );
}
