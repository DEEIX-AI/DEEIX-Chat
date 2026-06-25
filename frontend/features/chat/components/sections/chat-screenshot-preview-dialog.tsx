"use client";

import * as React from "react";
import { Copy, Download } from "lucide-react";
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
      <DialogContent className="sm:max-w-[680px]">
        <DialogHeader>
          <DialogTitle>{t("previewTitle")}</DialogTitle>
          <DialogDescription>{t("previewDescription")}</DialogDescription>
        </DialogHeader>

        <div className="max-h-[60vh] overflow-auto rounded-lg border border-border/60 bg-muted/30 p-3">
          {previewURL ? (
            // eslint-disable-next-line @next/next/no-img-element
            <img src={previewURL} alt={t("previewTitle")} className="mx-auto block h-auto w-full rounded-md" />
          ) : null}
        </div>

        <DialogFooter>
          <Button type="button" variant="ghost" onClick={() => onOpenChange(false)}>
            {t("close")}
          </Button>
          {clipboardSupported ? (
            <Button type="button" variant="ghost" onClick={() => void onCopy()}>
              <Copy className="size-4" />
              {t("copyImage")}
            </Button>
          ) : null}
          <Button type="button" onClick={onDownload}>
            <Download className="size-4" />
            {t("downloadImage")}
          </Button>
        </DialogFooter>
      </DialogContent>
    </Dialog>
  );
}
