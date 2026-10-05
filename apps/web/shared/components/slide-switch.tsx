"use client";

import { AnimatePresence, motion, useReducedMotion } from "motion/react";
import type * as React from "react";

import { cn } from "@/lib/utils";

type SlideSwitchProps = {
  // Changing this key slides the old content out and the new content in.
  itemKey: string;
  // 1 slides in from the right (moving forward), -1 from the left; on the y axis, from below / above.
  direction: 1 | -1;
  // "y" for vertical lists, where moving down the list brings the new content up from below.
  axis?: "x" | "y";
  // Seconds; match it to whatever animates around the switch so both land together.
  duration?: number;
  children: React.ReactNode;
  className?: string;
};

const SLIDE_DISTANCE = 24;
const SLIDE_EASE = [0.22, 1, 0.36, 1] as const;

type SlideCustom = { axis: "x" | "y"; distance: number };

// Variants read the distance from `custom` so the leaving panel follows the
// direction of the *new* switch, not the one it entered with. The leaving panel
// also stops taking the pointer while it fades out on top of the new one.
const SLIDE_VARIANTS = {
  enter: ({ axis, distance }: SlideCustom) => ({ opacity: 0, [axis]: distance }),
  center: ({ axis }: SlideCustom) => ({ opacity: 1, [axis]: 0, pointerEvents: "auto" as const }),
  exit: ({ axis, distance }: SlideCustom) => ({ opacity: 0, [axis]: -distance, pointerEvents: "none" as const }),
};

// Direction-aware crossfade between sibling panels (tabs, filters, wizard
// steps). The leaving panel is popped out of flow so the parent can animate
// height independently, e.g. via HeightTransition.
export function SlideSwitch({ itemKey, direction, axis = "x", duration = 0.2, children, className }: SlideSwitchProps) {
  const reduceMotion = useReducedMotion();
  const custom: SlideCustom = { axis, distance: reduceMotion ? 0 : SLIDE_DISTANCE * direction };
  return (
    <AnimatePresence initial={false} mode="popLayout" custom={custom}>
      <motion.div
        key={itemKey}
        className={cn("min-w-0", className)}
        custom={custom}
        variants={SLIDE_VARIANTS}
        initial="enter"
        animate="center"
        exit="exit"
        transition={{ duration, ease: SLIDE_EASE }}
      >
        {children}
      </motion.div>
    </AnimatePresence>
  );
}
