"use client";

import { motion } from "motion/react";
import type { HTMLAttributes } from "react";
import { forwardRef, useImperativeHandle, useState } from "react";

import { cn } from "@/lib/utils";

export interface PlusIconHandle {
  startAnimation: () => void;
  stopAnimation: () => void;
}

interface PlusIconProps extends HTMLAttributes<HTMLDivElement> {
  animate?: "default" | false;
  size?: number;
  strokeWidth?: number;
}

const PlusIcon = forwardRef<PlusIconHandle, PlusIconProps>(
  ({ animate, className, size = 28, strokeWidth = 2, ...props }, ref) => {
    const [imperativeActive, setImperativeActive] = useState(false);
    const active = Boolean(animate) || imperativeActive;

    useImperativeHandle(ref, () => {
      return {
        startAnimation: () => setImperativeActive(true),
        stopAnimation: () => setImperativeActive(false),
      };
    });

    return (
      <div
        className={cn(className)}
        {...props}
      >
        <motion.svg
          initial={{ rotate: 0 }}
          animate={{ rotate: active ? 180 : 0 }}
          fill="none"
          height={size}
          stroke="currentColor"
          strokeLinecap="round"
          strokeLinejoin="round"
          strokeWidth={strokeWidth}
          transition={active ? { duration: 0.16, ease: "easeOut" } : { duration: 0 }}
          viewBox="0 0 24 24"
          width={size}
          xmlns="http://www.w3.org/2000/svg"
        >
          <path d="M5 12h14" />
          <path d="M12 5v14" />
        </motion.svg>
      </div>
    );
  }
);

PlusIcon.displayName = "PlusIcon";

export { PlusIcon };
