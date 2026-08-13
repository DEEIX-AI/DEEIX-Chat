"use client";

import * as React from "react";
import { CircleHelp } from "lucide-react";
import { useTranslations } from "next-intl";

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
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from "@/components/ui/select";
import { Switch } from "@/components/ui/switch";
import { Textarea } from "@/components/ui/textarea";
import { Tooltip, TooltipContent, TooltipTrigger } from "@/components/ui/tooltip";
import type { MCPToolSchemaStringArgument } from "@/features/admin/components/sections/tools/mcp-tool-schema";
import { useDialogSnapshot } from "@/shared/hooks/use-dialog-snapshot";

export type MCPToolEditFormState = {
  id: number;
  displayName: string;
  description: string;
  attachmentInputMode: "none" | "image";
  attachmentArgument: string;
  attachmentEncoding: "base64" | "data_url";
  attachmentPromptArgument: string;
  passUserPrompt: boolean;
  schemaStringArguments: MCPToolSchemaStringArgument[];
  schemaRequiredArguments: string[];
};

type MCPToolEditDialogProps = {
  form: MCPToolEditFormState | null;
  saving: boolean;
  onFormChange: React.Dispatch<React.SetStateAction<MCPToolEditFormState | null>>;
  onClose: () => void;
  onSave: () => void;
};

function HelpTooltip({ label, children }: { label: string; children: React.ReactNode }) {
  return (
    <Tooltip>
      <TooltipTrigger asChild>
        <Button
          type="button"
          variant="ghost"
          size="icon-xs"
          className="text-muted-foreground hover:bg-transparent hover:text-foreground"
          aria-label={label}
        >
          <CircleHelp className="size-3" />
        </Button>
      </TooltipTrigger>
      <TooltipContent side="top" className="max-w-xs text-xs leading-5">
        {children}
      </TooltipContent>
    </Tooltip>
  );
}

function attachmentFormValid(form: MCPToolEditFormState | null): boolean {
  if (!form || form.attachmentInputMode === "none") {
    return true;
  }
  const argumentNames = new Set(form.schemaStringArguments.map((argument) => argument.name));
  if (!argumentNames.has(form.attachmentArgument)) {
    return false;
  }
  if (form.passUserPrompt && (
    !argumentNames.has(form.attachmentPromptArgument) ||
    form.attachmentPromptArgument === form.attachmentArgument
  )) {
    return false;
  }
  const mappedArguments = new Set([
    form.attachmentArgument,
    ...(form.passUserPrompt ? [form.attachmentPromptArgument] : []),
  ]);
  return form.schemaRequiredArguments.every((argument) => mappedArguments.has(argument));
}

export function MCPToolEditDialog({
  form,
  saving,
  onFormChange,
  onClose,
  onSave,
}: MCPToolEditDialogProps) {
  const t = useTranslations("adminTools.toolDialog");
  const tActions = useTranslations("common.actions");
  const stableForm = useDialogSnapshot(form);
  const selectedImageArgument = stableForm?.schemaStringArguments.find(
    (argument) => argument.name === stableForm.attachmentArgument,
  ) ?? null;
  const selectedPromptArgument = stableForm?.schemaStringArguments.find(
    (argument) => argument.name === stableForm.attachmentPromptArgument,
  ) ?? null;
  const mappedArguments = new Set([
    stableForm?.attachmentArgument ?? "",
    ...(stableForm?.passUserPrompt ? [stableForm.attachmentPromptArgument] : []),
  ]);
  const unmappedRequiredArguments = stableForm?.schemaRequiredArguments.filter(
    (argument) => !mappedArguments.has(argument),
  ) ?? [];

  return (
    <Dialog open={Boolean(form)} onOpenChange={(open) => !open && onClose()}>
      <DialogContent className="overflow-hidden sm:max-w-[520px]">
        <form
          className="contents"
          onSubmit={(event) => {
            event.preventDefault();
            onSave();
          }}
        >
          <DialogHeader>
            <DialogTitle>{t("title")}</DialogTitle>
            <DialogDescription className="sr-only">{t("description")}</DialogDescription>
          </DialogHeader>

          <div className="min-h-0 space-y-4 overflow-y-auto px-0.5">
            <div className="space-y-1">
              <p className="text-xs text-muted-foreground">{t("displayName")}</p>
              <Input
                value={stableForm?.displayName ?? ""}
                placeholder={t("displayNamePlaceholder")}
                maxLength={160}
                onChange={(event) => onFormChange((prev) => (
                  prev ? { ...prev, displayName: event.target.value } : prev
                ))}
              />
            </div>

            <div className="space-y-1">
              <p className="text-xs text-muted-foreground">{t("toolDescription")}</p>
              <Textarea
                value={stableForm?.description ?? ""}
                className="h-24 resize-none text-xs leading-5"
                placeholder={t("toolDescriptionPlaceholder")}
                maxLength={4096}
                onChange={(event) => onFormChange((prev) => (
                  prev ? { ...prev, description: event.target.value } : prev
                ))}
              />
            </div>

            <div className="border-t border-border/60 pt-4">
              <div className="flex items-start justify-between gap-4">
                <div className="flex min-w-0 flex-1 items-center gap-1">
                  <p className="text-xs font-medium text-foreground">{t("imageRoutingTitle")}</p>
                  <HelpTooltip label={t("imageRoutingTitle")}>
                    {t("imageRoutingTooltip")}
                  </HelpTooltip>
                </div>
                <Switch
                  size="sm"
                  checked={stableForm?.attachmentInputMode === "image"}
                  aria-label={t("imageRoutingTitle")}
                  onCheckedChange={(checked) => onFormChange((prev) => (
                    prev ? { ...prev, attachmentInputMode: checked ? "image" : "none" } : prev
                  ))}
                />
              </div>

              {stableForm ? (
                <DialogCollapsible open={stableForm.attachmentInputMode === "image"}>
                  <div className="space-y-4 pt-4">
                    {stableForm.schemaStringArguments.length === 0 ? (
                      <p className="text-[11px] leading-5 text-destructive">
                        {t("noStringArguments")}
                      </p>
                    ) : (
                      <div className="grid gap-3 sm:grid-cols-2">
                        <div className="space-y-1.5">
                          <div className="flex items-center gap-1">
                            <p className="text-xs text-muted-foreground">{t("imageArgument")}</p>
                            <HelpTooltip label={t("imageArgument")}>
                              {selectedImageArgument?.description || t("imageArgumentTooltip")}
                            </HelpTooltip>
                          </div>
                          <Select
                            value={selectedImageArgument?.name}
                            onValueChange={(value) => onFormChange((prev) => (
                              prev ? {
                                ...prev,
                                attachmentArgument: value,
                                attachmentPromptArgument: prev.attachmentPromptArgument === value
                                  ? ""
                                  : prev.attachmentPromptArgument,
                              } : prev
                            ))}
                          >
                            <SelectTrigger>
                              <SelectValue placeholder={t("selectImageArgument")} />
                            </SelectTrigger>
                            <SelectContent>
                              {stableForm.schemaStringArguments.map((argument) => (
                                <SelectItem key={argument.name} value={argument.name}>
                                  {argument.required
                                    ? `${argument.name} · ${t("schemaArgumentRequired")}`
                                    : argument.name}
                                </SelectItem>
                              ))}
                            </SelectContent>
                          </Select>
                        </div>

                        <div className="space-y-1.5">
                          <div className="flex items-center gap-1">
                            <p className="text-xs text-muted-foreground">{t("imageEncoding")}</p>
                            <HelpTooltip label={t("imageEncoding")}>
                              {t("imageEncodingTooltip")}
                            </HelpTooltip>
                          </div>
                          <Select
                            value={stableForm.attachmentEncoding}
                            onValueChange={(value: "base64" | "data_url") => onFormChange((prev) => (
                              prev ? { ...prev, attachmentEncoding: value } : prev
                            ))}
                          >
                            <SelectTrigger>
                              <SelectValue />
                            </SelectTrigger>
                            <SelectContent>
                              <SelectItem value="data_url">{t("imageEncodingDataURL")}</SelectItem>
                              <SelectItem value="base64">{t("imageEncodingBase64")}</SelectItem>
                            </SelectContent>
                          </Select>
                        </div>
                      </div>
                    )}

                    <div className="border-t border-border/60 pt-4">
                      <div className="flex items-start gap-3">
                        <div className="flex min-w-0 flex-1 items-center gap-1">
                          <p className="text-xs font-medium text-foreground">{t("passUserPrompt")}</p>
                          <HelpTooltip label={t("passUserPrompt")}>
                            {t("passUserPromptTooltip")}
                          </HelpTooltip>
                        </div>
                        <Switch
                          size="sm"
                          checked={stableForm.passUserPrompt}
                          aria-label={t("passUserPrompt")}
                          onCheckedChange={(checked) => onFormChange((prev) => (
                            prev ? {
                              ...prev,
                              passUserPrompt: checked,
                              attachmentPromptArgument: checked ? prev.attachmentPromptArgument : "",
                            } : prev
                          ))}
                        />
                      </div>

                      <DialogCollapsible open={stableForm.passUserPrompt}>
                        <div className="space-y-1.5 pt-3">
                          <p className="text-xs text-muted-foreground">{t("promptArgument")}</p>
                          <Select
                            value={selectedPromptArgument?.name}
                            onValueChange={(value) => onFormChange((prev) => (
                              prev ? { ...prev, attachmentPromptArgument: value } : prev
                            ))}
                          >
                            <SelectTrigger>
                              <SelectValue placeholder={t("selectPromptArgument")} />
                            </SelectTrigger>
                            <SelectContent>
                              {stableForm.schemaStringArguments
                                .filter((argument) => argument.name !== stableForm.attachmentArgument)
                                .map((argument) => (
                                  <SelectItem key={argument.name} value={argument.name}>
                                    {argument.required
                                      ? `${argument.name} · ${t("schemaArgumentRequired")}`
                                      : argument.name}
                                  </SelectItem>
                                ))}
                            </SelectContent>
                          </Select>
                          {selectedPromptArgument?.description ? (
                            <p className="text-xs leading-5 text-muted-foreground">
                              {selectedPromptArgument.description}
                            </p>
                          ) : null}
                        </div>
                      </DialogCollapsible>
                    </div>

                    {unmappedRequiredArguments.length > 0 ? (
                      <p className="text-[11px] leading-5 text-destructive">
                        {t("unmappedRequiredArguments", { names: unmappedRequiredArguments.join(", ") })}
                      </p>
                    ) : null}
                  </div>
                </DialogCollapsible>
              ) : null}
            </div>
          </div>

          <DialogFooter>
            <Button type="button" variant="ghost" onClick={onClose} disabled={saving}>
              {tActions("cancel")}
            </Button>
            <Button type="submit" disabled={saving || !attachmentFormValid(stableForm)}>
              {tActions("save")}
            </Button>
          </DialogFooter>
        </form>
      </DialogContent>
    </Dialog>
  );
}
