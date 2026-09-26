"use client";

import * as React from "react";
import { AnimatePresence, motion, type HTMLMotionProps } from "motion/react";

import { cn } from "@/lib/utils";

type CollapsibleMotionContentProps = Omit<HTMLMotionProps<"div">, "animate" | "children" | "exit" | "initial" | "transition"> & {
  open: boolean;
  children: React.ReactNode;
  contentClassName?: string;
};

const COLLAPSIBLE_MOTION_TRANSITION = {
  duration: 0.2,
  ease: [0.22, 1, 0.36, 1],
} as const;

export function CollapsibleMotionContent({
  open,
  children,
  className,
  contentClassName,
  style,
  ...props
}: CollapsibleMotionContentProps) {
  return (
    <AnimatePresence initial={false}>
      {open ? (
        <motion.div
          {...props}
          className={cn("min-w-0", className)}
          initial={{ opacity: 0, gridTemplateRows: "0fr" }}
          animate={{ opacity: 1, gridTemplateRows: "1fr" }}
          exit={{ opacity: 0, gridTemplateRows: "0fr" }}
          transition={COLLAPSIBLE_MOTION_TRANSITION}
          style={{ ...style, display: "grid" }}
        >
          <div className={cn("min-w-0 overflow-hidden", contentClassName)}>
            {children}
          </div>
        </motion.div>
      ) : null}
    </AnimatePresence>
  );
}
