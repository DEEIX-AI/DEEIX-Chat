"use client";

import { motion, type HTMLMotionProps } from "motion/react";
import * as React from "react";

export const MORPH_DURATION_S = 0.28;
const MORPH_EASE = "cubic-bezier(0.22, 1, 0.36, 1)";

function morphTransition(property: string): string {
  return `${property} ${MORPH_DURATION_S}s ${MORPH_EASE}`;
}

// False until the element has painted where it first appeared, so a card that opens is placed directly
// instead of gliding in from its first, unmeasured position.
function useSettled(): boolean {
  const [settled, setSettled] = React.useState(false);
  React.useEffect(() => {
    let second = 0;
    const first = window.requestAnimationFrame(() => {
      second = window.requestAnimationFrame(() => setSettled(true));
    });
    return () => {
      window.cancelAnimationFrame(first);
      window.cancelAnimationFrame(second);
    };
  }, []);
  return settled;
}

/**
 * 1 when `index` moved down since its last value, -1 when it moved up: the side the next content
 * slides in from. Negative indexes (nothing active) are ignored, so the direction survives gaps.
 */
export function useSwitchDirection(index: number): 1 | -1 {
  const [state, setState] = React.useState<{ index: number; direction: 1 | -1 }>({ index, direction: 1 });
  if (index >= 0 && index !== state.index) {
    const next = { index, direction: state.index < 0 || index > state.index ? 1 : -1 } as const;
    setState(next);
    return next.direction;
  }
  return state.direction;
}

type MorphingCardProps = Omit<HTMLMotionProps<"div">, "style"> & {
  ref?: React.Ref<HTMLDivElement>;
  /** The clipping box whose height animates; it also measures the card's content. */
  bodyRef?: React.Ref<HTMLDivElement>;
  /** Offset within the positioned parent. Changes glide once the card has settled. */
  top: number;
  /** Explicit content height so it can animate; null keeps the natural height (not measured yet). */
  bodyHeight: number | null;
  style?: React.CSSProperties;
  children: React.ReactNode;
};

// An absolutely positioned card that glides between positions and heights. Pair the content with a
// SlideSwitch so a swap and a glide read as one motion.
export function MorphingCard({ ref, bodyRef, top, bodyHeight, style, children, ...props }: MorphingCardProps) {
  const settled = useSettled();
  return (
    <motion.div ref={ref} {...props} style={{ ...style, top, transition: settled ? morphTransition("top") : "none" }}>
      <div
        ref={bodyRef}
        className="relative min-h-0 overflow-hidden"
        style={{
          height: bodyHeight ?? undefined,
          transition: settled ? morphTransition("height") : "none",
        }}
      >
        {children}
      </div>
    </motion.div>
  );
}
