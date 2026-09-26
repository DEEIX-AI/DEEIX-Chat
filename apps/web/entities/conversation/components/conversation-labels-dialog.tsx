"use client";

import * as React from "react";
import { Plus, X } from "lucide-react";
import { useTranslations } from "next-intl";
import { toast } from "sonner";

import { Badge } from "@/components/ui/badge";
import { Button } from "@/components/ui/button";
import {
  Dialog,
  DialogCollapsible,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from "@/components/ui/dialog";
import { Input } from "@/components/ui/input";
import { SpinnerLabel } from "@/components/ui/spinner";
import {
  MAX_CONVERSATION_LABEL_LENGTH,
  MAX_CONVERSATION_LABELS,
  normalizeConversationLabel,
  normalizeConversationLabels,
} from "@/shared/lib/conversation-labels";

type ConversationLabelsDialogProps = {
  open: boolean;
  labels: string[];
  onOpenChange: (open: boolean) => void;
  onSave: (labels: string[]) => void | Promise<void>;
};

export function ConversationLabelsDialog({
  open,
  labels,
  onOpenChange,
  onSave,
}: ConversationLabelsDialogProps) {
  const t = useTranslations("conversation.labelsDialog");
  const common = useTranslations("common.actions");
  const [draftLabels, setDraftLabels] = React.useState<string[]>([]);
  const [inputValue, setInputValue] = React.useState("");
  const [inputOpen, setInputOpen] = React.useState(false);
  const [saving, setSaving] = React.useState(false);
  const inputRef = React.useRef<HTMLInputElement>(null);
  const addButtonRef = React.useRef<HTMLButtonElement>(null);
  const saveButtonRef = React.useRef<HTMLButtonElement>(null);

  React.useEffect(() => {
    if (!open) {
      return;
    }
    setDraftLabels(normalizeConversationLabels(labels));
    setInputValue("");
    setInputOpen(false);
  }, [labels, open]);

  const normalizedInput = normalizeConversationLabel(inputValue);
  const inputValid = Boolean(
    normalizedInput &&
    Array.from(normalizedInput).length <= MAX_CONVERSATION_LABEL_LENGTH &&
    !draftLabels.some((item) => item.toLowerCase() === normalizedInput.toLowerCase()),
  );
  const addInputLabel = React.useCallback(() => {
    const nextLabel = normalizeConversationLabel(inputValue);
    if (
      !nextLabel ||
      Array.from(nextLabel).length > MAX_CONVERSATION_LABEL_LENGTH ||
      draftLabels.length >= MAX_CONVERSATION_LABELS ||
      draftLabels.some((item) => item.toLowerCase() === nextLabel.toLowerCase())
    ) {
      return;
    }
    setDraftLabels((current) => [...current, nextLabel]);
    setInputValue("");
    setInputOpen(false);
    requestAnimationFrame(() => {
      const nextTarget = draftLabels.length + 1 >= MAX_CONVERSATION_LABELS ? saveButtonRef : addButtonRef;
      nextTarget.current?.focus();
    });
  }, [draftLabels, inputValue]);

  const commitLabels = React.useCallback(async () => {
    if (saving) {
      return;
    }
    const nextLabels = normalizeConversationLabels([
      ...draftLabels,
      ...(inputValid ? [normalizedInput] : []),
    ]);
    setSaving(true);
    try {
      await onSave(nextLabels);
      onOpenChange(false);
    } catch {
      toast.error(t("labelsSaveFailed"));
    } finally {
      setSaving(false);
    }
  }, [draftLabels, inputValid, normalizedInput, onOpenChange, onSave, saving, t]);

  return (
    <Dialog open={open} onOpenChange={(nextOpen) => !saving && onOpenChange(nextOpen)}>
      <DialogContent className="sm:max-w-md">
        <DialogHeader>
          <DialogTitle>{t("labels")}</DialogTitle>
          <DialogDescription>{t("labelsDescription", { count: MAX_CONVERSATION_LABELS })}</DialogDescription>
        </DialogHeader>
        <form
          className="space-y-4"
          onSubmit={(event) => {
            event.preventDefault();
            void commitLabels();
          }}
        >
          <div>
            <div className="flex min-h-7 flex-wrap items-center gap-1.5">
              {draftLabels.length > 0 ? (
                draftLabels.map((label) => (
                  <Badge key={label.toLowerCase()} variant="secondary" className="h-6 gap-1 py-0 pr-1 pl-2">
                    <span className="max-w-56 truncate">{label}</span>
                    <Button
                      type="button"
                      variant="ghost"
                      size="icon-xs"
                      aria-label={t("removeLabel", { label })}
                      disabled={saving}
                      onClick={() => setDraftLabels((current) => current.filter((item) => item !== label))}
                    >
                      <X />
                    </Button>
                  </Badge>
                ))
              ) : (
                <p className="text-xs text-muted-foreground">{t("labelsEmpty")}</p>
              )}
              <Button
                ref={addButtonRef}
                type="button"
                variant="ghost"
                size="icon-sm"
                className="rounded-full border border-dashed border-muted-foreground/45 text-muted-foreground hover:border-foreground/45 hover:text-foreground"
                aria-label={t("addLabel")}
                disabled={saving || draftLabels.length >= MAX_CONVERSATION_LABELS}
                onClick={() => {
                  setInputOpen(true);
                  requestAnimationFrame(() => inputRef.current?.focus());
                }}
              >
                <Plus />
              </Button>
            </div>
            <DialogCollapsible open={inputOpen} className="-mx-1">
              <div className="px-1 pb-1 pt-4">
                <Input
                  ref={inputRef}
                  value={inputValue}
                  placeholder={t("labelPlaceholder")}
                  disabled={saving || draftLabels.length >= MAX_CONVERSATION_LABELS}
                  onChange={(event) => {
                    const nextValue = event.target.value;
                    if (Array.from(normalizeConversationLabel(nextValue)).length <= MAX_CONVERSATION_LABEL_LENGTH) {
                      setInputValue(nextValue);
                    }
                  }}
                  onBlur={() => {
                    if (!inputValue.trim()) {
                      setInputOpen(false);
                    }
                  }}
                  onKeyDown={(event) => {
                    if (event.key === "Enter") {
                      event.preventDefault();
                      addInputLabel();
                    } else if (event.key === "Escape") {
                      event.preventDefault();
                      setInputValue("");
                      setInputOpen(false);
                      requestAnimationFrame(() => addButtonRef.current?.focus());
                    }
                  }}
                />
              </div>
            </DialogCollapsible>
          </div>
          <DialogFooter>
            <Button type="button" variant="ghost" disabled={saving} onClick={() => onOpenChange(false)}>
              {common("cancel")}
            </Button>
            <Button ref={saveButtonRef} type="submit" disabled={saving}>
              {saving ? <SpinnerLabel>{common("saving")}</SpinnerLabel> : common("save")}
            </Button>
          </DialogFooter>
        </form>
      </DialogContent>
    </Dialog>
  );
}
