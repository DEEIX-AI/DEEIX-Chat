"use client";

import * as React from "react";
import { AnimatePresence, motion } from "motion/react";

import { Badge } from "@/components/ui/badge";
import { Tooltip, TooltipContent, TooltipTrigger } from "@/components/ui/tooltip";
import { cn } from "@/lib/utils";

type ChatEmptyStateProps = {
  greetingTitle: string;
  badgeLabel?: string;
  badgeTooltip?: string;
  contentWidthClassName?: string;
  children?: React.ReactNode;
};

const CHAT_EMPTY_TEXT_TRANSITION = {
  duration: 0.22,
  ease: [0.16, 1, 0.3, 1] as const,
};

export function ChatEmptyState({ greetingTitle, badgeLabel, badgeTooltip, contentWidthClassName = "max-w-[1080px]", children }: ChatEmptyStateProps) {
  const badge = badgeLabel ? (
    <span className="absolute left-full top-0 ml-1.5">
      <Badge
        variant="outline"
        className="cursor-default border-border/70 bg-background/60 px-1.5 py-0 text-[9px] font-medium text-muted-foreground"
      >
        {badgeLabel}
      </Badge>
    </span>
  ) : null;
  const titleGroupKey = `${greetingTitle}:${badgeLabel ?? ""}`;

  return (
    <div className="flex h-full min-h-0 flex-col items-center justify-center px-3 py-12 text-center md:px-6 md:py-20">
      <motion.div layout className="relative inline-flex max-w-[calc(100%-4.5rem)] justify-center">
        <AnimatePresence mode="wait" initial={false}>
          <motion.div
            key={titleGroupKey}
            className="relative inline-flex min-w-0 justify-center"
            initial={{ opacity: 0, y: 8 }}
            animate={{ opacity: 1, y: 0 }}
            exit={{ opacity: 0, y: -6 }}
            transition={CHAT_EMPTY_TEXT_TRANSITION}
          >
            <h1 className="min-w-0 text-balance text-[22px] font-medium leading-[1.12] tracking-[-0.005em] text-foreground [font-family:var(--font-economist)] md:text-[32px]">
              {greetingTitle}
            </h1>
            {badge && badgeTooltip ? (
              <Tooltip>
                <TooltipTrigger asChild>{badge}</TooltipTrigger>
                <TooltipContent side="top" className="max-w-72 text-left leading-5">
                  {badgeTooltip}
                </TooltipContent>
              </Tooltip>
            ) : badge}
          </motion.div>
        </AnimatePresence>
      </motion.div>
      {children ? <div className={cn("mt-7 w-full md:mt-8", contentWidthClassName)}>{children}</div> : null}
    </div>
  );
}
