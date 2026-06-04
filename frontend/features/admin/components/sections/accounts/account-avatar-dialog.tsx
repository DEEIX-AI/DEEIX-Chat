"use client";

import { useTranslations } from "next-intl";

import { Avatar, AvatarFallback, AvatarImage } from "@/components/ui/avatar";
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
import { SpinnerLabel } from "@/components/ui/spinner";

export function AccountAvatarEditorDialog({
  open,
  onOpenChange,
  title,
  description,
  previewSrc,
  alt,
  fallback,
  value,
  onValueChange,
  onRandomize,
  onApply,
  applyLabel,
  pending,
}: {
  open: boolean;
  onOpenChange: (open: boolean) => void;
  title: string;
  description: string;
  previewSrc?: string;
  alt: string;
  fallback: string;
  value: string;
  onValueChange: (value: string) => void;
  onRandomize: () => void;
  onApply: () => void;
  applyLabel: string;
  pending?: boolean;
}) {
  const t = useTranslations("adminUsers.avatar");
  return (
    <Dialog open={open} onOpenChange={onOpenChange}>
      <DialogContent className="flex max-h-[min(86vh,760px)] w-[calc(100vw-2rem)] flex-col gap-0 overflow-hidden p-0 sm:max-w-[420px]">
        <DialogHeader className="shrink-0 px-4 py-4">
          <DialogTitle>{title}</DialogTitle>
          <DialogDescription>{description}</DialogDescription>
        </DialogHeader>

        <div className="min-h-0 flex-1 space-y-4 overflow-y-auto px-4 py-2">
          <div className="flex justify-center">
            <button
              type="button"
              className="rounded-2xl transition-transform hover:scale-[1.03] focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring"
              onClick={onRandomize}
              disabled={pending}
            >
              <Avatar className="size-16 bg-pure">
                <AvatarImage src={previewSrc || undefined} alt={alt} />
                <AvatarFallback className="bg-foreground text-3xl font-medium text-background">
                  {fallback}
                </AvatarFallback>
              </Avatar>
            </button>
          </div>

          <div className="space-y-1">
            <p className="text-xs text-muted-foreground">{t("urlLabel")}</p>
            <Input
              value={value}
              onChange={(event) => onValueChange(event.target.value)}
              placeholder="https://example.com/avatar.png"
              disabled={pending}
            />
          </div>
        </div>

        <DialogFooter className="shrink-0 px-4 py-3">
          <Button type="button" variant="ghost" onClick={() => onOpenChange(false)} disabled={pending}>
            {t("cancel")}
          </Button>
          <Button type="button" onClick={onApply} disabled={pending}>
            {pending ? <SpinnerLabel>{t("saving")}</SpinnerLabel> : applyLabel}
          </Button>
        </DialogFooter>
      </DialogContent>
    </Dialog>
  );
}
