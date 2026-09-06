"use client";

import { useTranslations } from "next-intl";
import * as React from "react";

import { Binary } from "@/components/animate-ui/icons/binary";
import { InputGroupButton } from "@/components/ui/input-group";
import { Tooltip, TooltipContent, TooltipTrigger } from "@/components/ui/tooltip";
import { cn } from "@/lib/utils";

type ChatProgrammingModeProps = {
  enabled: boolean;
  disabled?: boolean;
  shellEnabled?: boolean;
  onChange: (enabled: boolean) => void;
};

export function ChatProgrammingMode({
  enabled,
  disabled = false,
  shellEnabled = false,
  onChange,
}: ChatProgrammingModeProps) {
  const t = useTranslations("chat.programming");
  const [hovered, setHovered] = React.useState(false);

  return (
    <Tooltip>
      <TooltipTrigger asChild>
        <InputGroupButton
          type="button"
          variant="ghost"
          size="icon-sm"
          className={cn(
            "size-7 rounded-md text-muted-foreground hover:text-foreground sm:size-8",
            enabled && "bg-primary/10 text-primary hover:bg-primary/10 hover:text-primary",
          )}
          disabled={disabled}
          aria-label={t("toggle")}
          aria-pressed={enabled}
          onMouseEnter={() => setHovered(true)}
          onMouseLeave={() => setHovered(false)}
          onFocus={() => setHovered(true)}
          onBlur={() => setHovered(false)}
          onClick={() => onChange(!enabled)}
        >
          <Binary size={16} strokeWidth={1.7} animate={hovered || enabled ? "default" : undefined} />
        </InputGroupButton>
      </TooltipTrigger>
      <TooltipContent side="top" align="center" sideOffset={6} className="max-w-64">
        <p className="font-medium">{enabled ? t("active") : t("title")}</p>
        <p className="mt-1 text-xs text-background/80">
          {shellEnabled ? t("descriptionWithShell") : t("description")}
        </p>
      </TooltipContent>
    </Tooltip>
  );
}
