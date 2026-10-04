"use client";

import type * as React from "react";
import { Check, ChevronDownIcon, CircleHelp } from "lucide-react";
import { useTranslations } from "next-intl";

import {
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuItem,
  DropdownMenuTrigger,
} from "@/components/ui/dropdown-menu";
import { Tooltip, TooltipContent, TooltipTrigger } from "@/components/ui/tooltip";
import { cn } from "@/lib/utils";

type ModelCapabilityRowProps = {
  label: string;
  description: string;
  /** Accessible name of the dropdown trigger. */
  editLabel: string;
  /** Whether the value is a custom override (自定义) rather than automatic detection (自动识别). */
  custom: boolean;
  disabled?: boolean;
  /** Right-aligned value; clipped at the row end when it does not fit. */
  children: React.ReactNode;
  /**
   * Set when the value is itself interactive (e.g. an input). It is then rendered outside the dropdown
   * trigger, which only covers the mode and the chevron; otherwise the whole value opens the menu.
   */
  interactiveValue?: boolean;
  menu: React.ReactNode;
  menuClassName?: string;
  onMenuCloseAutoFocus?: (event: Event) => void;
};

/**
 * Capability row of the model sheet: label on the left; value, mode (auto / custom) and the dropdown on
 * the right. There is no fixed two-column split, so the value can use all the remaining width.
 */
export function ModelCapabilityRow({
  label,
  description,
  editLabel,
  custom,
  disabled = false,
  children,
  interactiveValue = false,
  menu,
  menuClassName,
  onMenuCloseAutoFocus,
}: ModelCapabilityRowProps) {
  // ml-auto right-aligns the value; when it no longer fits, the auto margin collapses and the row clips at its end.
  const value = (
    <span className="flex h-full min-w-0 flex-1 overflow-hidden pr-2">
      <span className={cn("ml-auto flex min-w-0 items-center gap-1", interactiveValue && "flex-1")}>{children}</span>
    </span>
  );
  const t = useTranslations("adminModels.sheet.capabilityMode");
  // Both labels share one grid cell so the column is as wide as the longer one and the dividers line up across rows.
  const modeAndChevron = (
    <>
      <span className="grid h-full shrink-0 items-center border-l border-border/50 px-2 text-xs text-muted-foreground">
        <span className={cn("col-start-1 row-start-1", custom && "invisible")} aria-hidden={custom}>{t("auto")}</span>
        <span className={cn("col-start-1 row-start-1", !custom && "invisible")} aria-hidden={!custom}>{t("custom")}</span>
      </span>
      <span className="inline-flex h-full w-8 shrink-0 items-center justify-center border-l border-border/50 text-muted-foreground">
        <ChevronDownIcon className="size-3.5" aria-hidden="true" />
      </span>
    </>
  );
  return (
    <div className="flex h-8 min-w-0 items-center overflow-hidden rounded-md bg-secondary text-secondary-foreground">
      <div className="flex shrink-0 items-center gap-1 px-2">
        <span className="whitespace-nowrap text-xs leading-4">{label}</span>
        <Tooltip>
          <TooltipTrigger asChild>
            <button
              type="button"
              className="inline-flex size-4 shrink-0 items-center justify-center text-muted-foreground transition-colors hover:text-secondary-foreground"
              aria-label={description}
            >
              <CircleHelp className="size-3" aria-hidden="true" />
            </button>
          </TooltipTrigger>
          <TooltipContent className="max-w-72">{description}</TooltipContent>
        </Tooltip>
      </div>
      <div className="flex h-full min-w-0 flex-1 items-center pl-2 transition-colors hover:bg-background/30 focus-within:bg-background/50">
        {interactiveValue ? value : null}
        <DropdownMenu>
          <DropdownMenuTrigger asChild>
            <button
              type="button"
              disabled={disabled}
              className={cn(
                "flex h-full items-center text-left focus-visible:outline-none disabled:pointer-events-none disabled:opacity-50",
                interactiveValue ? "shrink-0" : "min-w-0 flex-1",
              )}
              aria-label={editLabel}
            >
              {interactiveValue ? null : value}
              {modeAndChevron}
            </button>
          </DropdownMenuTrigger>
          <DropdownMenuContent align="end" className={cn("w-56", menuClassName)} onCloseAutoFocus={onMenuCloseAutoFocus}>
            {menu}
          </DropdownMenuContent>
        </DropdownMenu>
      </div>
    </div>
  );
}

type ModelCapabilityMenuItemProps = {
  checked: boolean;
  disabled?: boolean;
  /** Keep the menu open after selecting, for multi-select lists. */
  keepOpen?: boolean;
  onSelect?: () => void;
  role?: "menuitemcheckbox" | "menuitemradio";
  children: React.ReactNode;
};

/** Menu item styled like the context window menu: content first, check mark at the end. */
export function ModelCapabilityMenuItem({
  checked,
  disabled = false,
  keepOpen = false,
  onSelect,
  role = "menuitemcheckbox",
  children,
}: ModelCapabilityMenuItemProps) {
  return (
    <DropdownMenuItem
      role={role}
      aria-checked={checked}
      disabled={disabled}
      className="data-[disabled]:opacity-100"
      onSelect={(event) => {
        if (keepOpen) event.preventDefault();
        onSelect?.();
      }}
    >
      {children}
      <Check className={cn("ml-auto size-3.5", checked ? "opacity-100" : "opacity-0")} />
    </DropdownMenuItem>
  );
}
