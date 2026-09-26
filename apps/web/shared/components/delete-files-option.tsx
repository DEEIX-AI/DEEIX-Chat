"use client";

import { useTranslations } from "next-intl";
import type { ReactNode } from "react";

import { Checkbox } from "@/components/ui/checkbox";
import { cn } from "@/lib/utils";

type DeleteFilesOptionProps = {
  id: string;
  checked: boolean;
  disabled?: boolean;
  label?: ReactNode;
  description?: ReactNode;
  onCheckedChange: (checked: boolean) => void;
};

export function DeleteFilesOption({
  id,
  checked,
  disabled = false,
  label,
  description,
  onCheckedChange,
}: DeleteFilesOptionProps) {
  const t = useTranslations("recent.dialogs");

  return (
    <div className="mt-1 flex items-start gap-2 py-2 text-left">
      <Checkbox
        id={id}
        checked={checked}
        disabled={disabled}
        className="mt-0.5"
        onCheckedChange={(value) => onCheckedChange(value === true)}
      />
      <label
        htmlFor={id}
        className={cn("cursor-pointer space-y-1", disabled && "cursor-not-allowed opacity-60")}
      >
        <span className="block text-xs font-medium text-foreground">{label ?? t("deleteFilesLabel")}</span>
        <span className="block text-xs leading-5 text-muted-foreground">{description ?? t("deleteFilesDescription")}</span>
      </label>
    </div>
  );
}
