"use client";

import type { ButtonHTMLAttributes, HTMLAttributes, ReactNode } from "react";

import { Tooltip, TooltipContent, TooltipTrigger } from "@/components/ui/tooltip";
import { cn } from "@/lib/utils";

export function MediaActionBar({
  children,
  className,
  ...props
}: HTMLAttributes<HTMLSpanElement>) {
  return (
    <span
      className={cn(
        "inline-flex items-center gap-0.5 rounded-full border border-white/15 bg-neutral-950/55 p-0.5 text-neutral-100 shadow-md shadow-black/15 backdrop-blur-md",
        className,
      )}
      {...props}
    >
      {children}
    </span>
  );
}

export function MediaActionButton({
  children,
  className,
  label,
  type = "button",
  ...props
}: Omit<ButtonHTMLAttributes<HTMLButtonElement>, "aria-label"> & {
  children: ReactNode;
  label: string;
}) {
  return (
    <Tooltip>
      <TooltipTrigger asChild>
        <button
          aria-label={label}
          className={cn(
            "inline-flex size-7 items-center justify-center rounded-full transition-colors hover:bg-white/15 hover:text-white focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-white/40",
            className,
          )}
          type={type}
          {...props}
        >
          {children}
        </button>
      </TooltipTrigger>
      <TooltipContent>{label}</TooltipContent>
    </Tooltip>
  );
}
