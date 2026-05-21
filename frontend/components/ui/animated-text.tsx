"use client";

import * as React from "react";

import { cn } from "@/lib/utils";

type AnimatedTextProps = {
  text: string;
  className?: string;
  textClassName?: string;
  durationMs?: number;
};

type TextTransition = {
  previous: string;
  next: string;
  active: boolean;
};

export function AnimatedText({
  text,
  className,
  textClassName,
  durationMs = 180,
}: AnimatedTextProps) {
  const currentTextRef = React.useRef(text);
  const frameRef = React.useRef<number | null>(null);
  const timerRef = React.useRef<number | null>(null);
  const [transition, setTransition] = React.useState<TextTransition | null>(null);

  React.useEffect(() => {
    if (text === currentTextRef.current) {
      return;
    }

    const previous = currentTextRef.current;
    currentTextRef.current = text;
    setTransition({ previous, next: text, active: false });

    if (frameRef.current !== null) {
      cancelAnimationFrame(frameRef.current);
    }
    if (timerRef.current !== null) {
      window.clearTimeout(timerRef.current);
    }

    frameRef.current = requestAnimationFrame(() => {
      setTransition((current) => current ? { ...current, active: true } : current);
    });
    timerRef.current = window.setTimeout(() => {
      setTransition(null);
      frameRef.current = null;
      timerRef.current = null;
    }, durationMs);

    return () => {
      if (frameRef.current !== null) {
        cancelAnimationFrame(frameRef.current);
        frameRef.current = null;
      }
      if (timerRef.current !== null) {
        window.clearTimeout(timerRef.current);
        timerRef.current = null;
      }
    };
  }, [durationMs, text]);

  if (!transition) {
    return (
      <span className={cn("block min-w-0 truncate", className)}>
        <span className={cn("block truncate", textClassName)}>{text}</span>
      </span>
    );
  }

  return (
    <span className={cn("relative block min-w-0 overflow-hidden", className)} aria-label={transition.next}>
      <span className={cn("invisible block truncate", textClassName)}>{transition.next}</span>
      <span
        aria-hidden="true"
        className={cn(
          "absolute inset-0 block truncate transition-[opacity,transform] ease-out motion-reduce:transition-none",
          transition.active ? "-translate-y-1 opacity-0" : "translate-y-0 opacity-100",
          textClassName,
        )}
        style={{ transitionDuration: `${durationMs}ms` }}
      >
        {transition.previous}
      </span>
      <span
        aria-hidden="true"
        className={cn(
          "absolute inset-0 block truncate transition-[opacity,transform] ease-out motion-reduce:transition-none",
          transition.active ? "translate-y-0 opacity-100" : "translate-y-1 opacity-0",
          textClassName,
        )}
        style={{ transitionDuration: `${durationMs}ms` }}
      >
        {transition.next}
      </span>
    </span>
  );
}
