"use client";

import * as React from "react";
import { AnimatePresence, motion } from "motion/react";

import { cn } from "@/lib/utils";

type SettingsCollapsibleContentProps = {
  open: boolean;
  children: React.ReactNode;
  className?: string;
  contentClassName?: string;
};

const SETTINGS_COLLAPSE_TRANSITION = {
  duration: 0.2,
  ease: [0.22, 1, 0.36, 1],
} as const;

export function SettingsCollapsibleContent({
  open,
  children,
  className,
  contentClassName,
}: SettingsCollapsibleContentProps) {
  return (
    <AnimatePresence initial={false}>
      {open ? (
        <motion.div
          className={cn("min-w-0", className)}
          initial={{ opacity: 0, gridTemplateRows: "0fr" }}
          animate={{ opacity: 1, gridTemplateRows: "1fr" }}
          exit={{ opacity: 0, gridTemplateRows: "0fr" }}
          transition={SETTINGS_COLLAPSE_TRANSITION}
          style={{ display: "grid" }}
        >
          <div className={cn("min-w-0 overflow-hidden", contentClassName)}>
            {children}
          </div>
        </motion.div>
      ) : null}
    </AnimatePresence>
  );
}
