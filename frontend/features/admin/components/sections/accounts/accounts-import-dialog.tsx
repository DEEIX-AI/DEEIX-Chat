"use client";

import * as React from "react";
import { Database } from "lucide-react";
import { motion } from "motion/react";
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
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import { SpinnerLabel } from "@/components/ui/spinner";
import type {
  ImportOpenWebUIUsersData,
  ImportOpenWebUIUsersRequest,
} from "@/features/admin/api/admin.types";

type AccountOpenWebUIImportDialogProps = {
  open: boolean;
  pending: boolean;
  result: ImportOpenWebUIUsersData | null;
  onOpenChange: (open: boolean) => void;
  onPreviewReset: () => void;
  onSubmit: (payload: ImportOpenWebUIUsersRequest) => Promise<void>;
};

const DIALOG_LAYOUT_TRANSITION = {
  layout: {
    duration: 0.22,
    ease: [0.16, 1, 0.3, 1] as const,
  },
};

export function AccountOpenWebUIImportDialog({
  open,
  pending,
  result,
  onOpenChange,
  onPreviewReset,
  onSubmit,
}: AccountOpenWebUIImportDialogProps) {
  const t = useTranslations("adminUsers.importOpenWebUI");
  const [dsn, setDsn] = React.useState("");
  const [creditMultiplier, setCreditMultiplier] = React.useState("1");
  const previewReady = result !== null;

  React.useEffect(() => {
    if (open) {
      return;
    }
    setDsn("");
    setCreditMultiplier("1");
  }, [open]);

  function handleDSNChange(value: string) {
    setDsn(value);
    if (result) {
      onPreviewReset();
    }
  }

  function handleCreditMultiplierChange(value: string) {
    setCreditMultiplier(value);
    if (result) {
      onPreviewReset();
    }
  }

  async function handleSubmit(event: React.FormEvent<HTMLFormElement>) {
    event.preventDefault();
    const parsedMultiplier = Number(creditMultiplier);
    if (pending || !dsn.trim() || !Number.isFinite(parsedMultiplier) || parsedMultiplier < 0) {
      return;
    }
    await onSubmit({
      dsn: dsn.trim(),
      creditMultiplier: parsedMultiplier,
      dryRun: !previewReady,
    });
  }

  return (
    <Dialog open={open} onOpenChange={onOpenChange}>
      <DialogContent className="flex max-h-[min(86vh,760px)] w-[calc(100vw-2rem)] flex-col gap-0 overflow-hidden p-0 sm:max-w-[560px]">
        <DialogHeader className="shrink-0 px-4 py-4">
          <DialogTitle>{t("title")}</DialogTitle>
          <DialogDescription>{t("description")}</DialogDescription>
        </DialogHeader>

        <motion.form layout transition={DIALOG_LAYOUT_TRANSITION} className="flex min-h-0 flex-1 flex-col" onSubmit={handleSubmit}>
          <div className="min-h-0 flex-1 space-y-4 overflow-y-auto px-4 py-2">
            <div className="flex items-center gap-3 rounded-md bg-muted/45 px-3 py-2">
              <Database className="size-3.5 shrink-0 text-foreground" />
              <div className="min-w-0 space-y-0.5">
                <div className="truncate text-xs font-medium text-foreground">{t("dedupeTitle")}</div>
                <div className="truncate text-xs text-muted-foreground">{t("dedupeDescription")}</div>
              </div>
            </div>

            <div className="space-y-1">
              <Label htmlFor="openwebui-dsn" className="text-xs font-normal text-muted-foreground">
                {t("dsnLabel")}
              </Label>
              <Input
                id="openwebui-dsn"
                value={dsn}
                onChange={(event) => handleDSNChange(event.target.value)}
                placeholder={t("dsnPlaceholder")}
                disabled={pending}
                autoComplete="off"
              />
            </div>

            <div className="space-y-1">
              <Label htmlFor="openwebui-credit-multiplier" className="text-xs font-normal text-muted-foreground">
                {t("creditMultiplierLabel")}
              </Label>
              <Input
                id="openwebui-credit-multiplier"
                type="number"
                min="0"
                step="0.000001"
                value={creditMultiplier}
                onChange={(event) => handleCreditMultiplierChange(event.target.value)}
                placeholder="1"
                disabled={pending}
              />
            </div>

            {result ? (
              <div className="grid grid-cols-2 gap-2 rounded-md bg-muted/45 p-3 text-xs text-muted-foreground">
                <span>{t("summary.scanned", { count: result.scanned })}</span>
                <span>{t("summary.imported", { count: result.imported })}</span>
                <span>{t("summary.skippedExistingEmail", { count: result.skippedExistingEmail })}</span>
                <span>{t("summary.skippedDuplicateSourceEmail", { count: result.skippedDuplicateSourceEmail })}</span>
                <span>{t("summary.skippedInvalidEmail", { count: result.skippedInvalidEmail })}</span>
                <span>{t("summary.skippedInvalidRow", { count: result.skippedInvalidRow })}</span>
              </div>
            ) : null}
          </div>

          <DialogFooter className="shrink-0 px-4 py-3">
            <Button type="button" variant="ghost" disabled={pending} onClick={() => onOpenChange(false)}>
              {t("cancel")}
            </Button>
            <Button type="submit" disabled={pending || !dsn.trim()}>
              {pending ? <SpinnerLabel>{t(previewReady ? "importing" : "previewing")}</SpinnerLabel> : t(previewReady ? "submit" : "preview")}
            </Button>
          </DialogFooter>
        </motion.form>
      </DialogContent>
    </Dialog>
  );
}
