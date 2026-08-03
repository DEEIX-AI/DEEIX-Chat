"use client";

import { Tags } from "lucide-react";
import { useTranslations } from "next-intl";

import {
  DropdownMenuItem,
  DropdownMenuItemIcon,
} from "@/components/ui/dropdown-menu";

type ConversationLabelsMenuItemProps = {
  labels: readonly string[];
  disabled?: boolean;
  onSelect: () => void;
};

export function ConversationLabelsMenuItem({
  labels,
  disabled = false,
  onSelect,
}: ConversationLabelsMenuItemProps) {
  const t = useTranslations("conversation.labelsDialog");

  return (
    <DropdownMenuItem
      disabled={disabled}
      onSelect={(event) => {
        event.preventDefault();
        if (!disabled) {
          onSelect();
        }
      }}
    >
      <DropdownMenuItemIcon icon={Tags} className="text-current" />
      {t("labels")}
      {labels.length > 0 ? (
        <span className="ml-auto flex size-3.5 items-center justify-center text-[11px] tabular-nums text-muted-foreground">
          {labels.length}
        </span>
      ) : null}
    </DropdownMenuItem>
  );
}
