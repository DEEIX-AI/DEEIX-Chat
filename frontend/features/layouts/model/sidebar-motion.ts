import type { Transition } from "motion/react";

export const SIDEBAR_TRANSFER_TRANSITION: Transition = {
  layout: {
    duration: 0.34,
    ease: [0.16, 1, 0.3, 1],
  },
};

export const SIDEBAR_OVERFLOW_ROW_TRANSITION: Transition = {
  duration: 0.24,
  ease: [0.22, 1, 0.36, 1],
};
