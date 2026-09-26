"use client";

import { AnimatePresence, motion, useReducedMotion } from "motion/react";
import * as React from "react";

import { cn } from "@/lib/utils";

type SlideSwitchProps = {
  // Changing this key slides the old content out and the new content in.
  itemKey: string;
  // 1 slides in from the right (moving forward), -1 from the left.
  direction: 1 | -1;
  children: React.ReactNode;
  className?: string;
};

const SLIDE_DISTANCE = 24;
const SLIDE_TRANSITION = { duration: 0.2, ease: [0.22, 1, 0.36, 1] } as const;

// Variants read the distance from `custom` so the leaving panel follows the
// direction of the *new* switch, not the one it entered with.
const SLIDE_VARIANTS = {
  enter: (distance: number) => ({ opacity: 0, x: distance }),
  center: { opacity: 1, x: 0 },
  exit: (distance: number) => ({ opacity: 0, x: -distance }),
};

// Direction-aware crossfade between sibling panels (tabs, filters, wizard
// steps). The leaving panel is popped out of flow so the parent can animate
// height independently, e.g. via HeightTransition.
export function SlideSwitch({ itemKey, direction, children, className }: SlideSwitchProps) {
  const reduceMotion = useReducedMotion();
  const distance = reduceMotion ? 0 : SLIDE_DISTANCE * direction;
  return (
    <AnimatePresence initial={false} mode="popLayout" custom={distance}>
      <motion.div
        key={itemKey}
        className={cn("min-w-0", className)}
        custom={distance}
        variants={SLIDE_VARIANTS}
        initial="enter"
        animate="center"
        exit="exit"
        transition={SLIDE_TRANSITION}
      >
        {children}
      </motion.div>
    </AnimatePresence>
  );
}
