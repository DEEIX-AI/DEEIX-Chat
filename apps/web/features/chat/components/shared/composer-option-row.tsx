"use client";

import { Check, type LucideIcon } from "lucide-react";
import type * as React from "react";

import { cn } from "@/lib/utils";

// Hover and keyboard focus share one tint across the composer's option popovers (knowledge bases,
// interactive components, MCP tools), so the pointer reads the same in every list.
export const composerOptionRowHoverClassName =
  "enabled:hover:bg-accent/70 enabled:hover:text-foreground focus-visible:bg-accent/70 focus-visible:text-foreground";

type ComposerOptionRowProps = Omit<React.ComponentProps<"button">, "children" | "type"> & {
  icon: LucideIcon;
  label: React.ReactNode;
  /** Quiet trailing text, e.g. a file count or a scope tag. */
  meta?: React.ReactNode;
  selected: boolean;
};

// One row of a multi-select composer popover. Selection is carried by text contrast and a trailing
// check, not by a filled background: with several rows selected the fills merged into one grey block
// and hover could no longer be told apart from selection.
export function ComposerOptionRow({ icon: Icon, label, meta, selected, className, ...props }: ComposerOptionRowProps) {
  return (
    <button
      {...props}
      type="button"
      data-selected={selected}
      aria-pressed={selected}
      className={cn(
        "flex h-7 w-full items-center gap-2 rounded-md px-2 text-left text-xs font-medium text-muted-foreground outline-none transition-colors data-[selected=true]:text-foreground disabled:cursor-not-allowed disabled:opacity-45",
        composerOptionRowHoverClassName,
        className,
      )}
    >
      <Icon className="size-3.5 shrink-0" strokeWidth={1.6} aria-hidden="true" />
      <span className="min-w-0 flex-1 truncate">{label}</span>
      {meta ? (
        <span className="shrink-0 text-[10px] font-normal leading-none tabular-nums text-muted-foreground">{meta}</span>
      ) : null}
      <Check
        className={cn("size-3.5 shrink-0 transition-opacity", selected ? "opacity-100" : "opacity-0")}
        strokeWidth={2}
        aria-hidden="true"
      />
    </button>
  );
}
