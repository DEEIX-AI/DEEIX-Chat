"use client";

import * as React from "react";

import { cn } from "@/lib/utils";
import { useScrollFadeFallbackRef } from "@/shared/hooks/use-scroll-fade-fallback-ref";

type AnimatedTextProps = {
  text: string;
  className?: string;
  textClassName?: string;
  durationMs?: number;
  scrollOverflow?: boolean;
};

type TextTransition = {
  previous: string;
  next: string;
  active: boolean;
};

function OverflowScrollingText({
  text,
  className,
  textClassName,
}: Pick<AnimatedTextProps, "text" | "className" | "textClassName">) {
  const viewportRef = React.useRef<HTMLSpanElement | null>(null);
  const scrollFadeRef = useScrollFadeFallbackRef(viewportRef);
  const [hovered, setHovered] = React.useState(false);

  React.useEffect(() => {
    const viewport = viewportRef.current;
    if (!viewport) {
      return;
    }

    const hoverTarget = viewport.closest<HTMLElement>("[data-animated-text-scroll-trigger]") ?? viewport;
    const handleMouseEnter = () => setHovered(true);
    const handleMouseLeave = () => setHovered(false);

    hoverTarget.addEventListener("mouseenter", handleMouseEnter);
    hoverTarget.addEventListener("mouseleave", handleMouseLeave);
    setHovered(hoverTarget.matches(":hover"));

    return () => {
      hoverTarget.removeEventListener("mouseenter", handleMouseEnter);
      hoverTarget.removeEventListener("mouseleave", handleMouseLeave);
    };
  }, []);

  React.useEffect(() => {
    const viewport = viewportRef.current;
    if (!viewport) {
      return;
    }
    const reducedMotion = window.matchMedia("(prefers-reduced-motion: reduce)").matches;
    const overflowing = viewport.scrollWidth - viewport.clientWidth > 1;
    if (!overflowing || !hovered || reducedMotion) {
      viewport.scrollTo({ left: 0, behavior: !hovered && !reducedMotion ? "smooth" : "auto" });
      return;
    }
    const pauseMs = 800;
    const pixelsPerSecond = 28;
    const direction = window.getComputedStyle(viewport).direction === "rtl" ? -1 : 1;
    let frameID = 0;
    let startedAt = 0;
    const animate = (timestamp: number) => {
      if (!startedAt) {
        startedAt = timestamp;
      }
      const distance = Math.max(0, viewport.scrollWidth - viewport.clientWidth);
      const travelMs = Math.max(1, (distance / pixelsPerSecond) * 1_000);
      const cycleMs = pauseMs * 2 + travelMs * 2;
      const elapsed = (timestamp - startedAt) % cycleMs;
      const forwardEnd = pauseMs + travelMs;
      const backwardStart = forwardEnd + pauseMs;
      let offset = 0;
      if (elapsed > pauseMs && elapsed <= forwardEnd) {
        offset = ((elapsed - pauseMs) / travelMs) * distance;
      } else if (elapsed > backwardStart) {
        offset = distance * (1 - (elapsed - backwardStart) / travelMs);
      } else if (elapsed > forwardEnd) {
        offset = distance;
      }
      viewport.scrollLeft = offset * direction;
      frameID = requestAnimationFrame(animate);
    };
    frameID = requestAnimationFrame(animate);
    return () => {
      cancelAnimationFrame(frameID);
    };
  }, [hovered, text]);

  return (
    <span
      ref={scrollFadeRef}
      className={cn(
        "no-scrollbar scroll-fade-x scroll-fade-8 block min-w-0 overflow-x-hidden whitespace-nowrap",
        className,
      )}
      aria-label={text}
    >
      <span
        aria-hidden="true"
        className={cn("block w-max whitespace-nowrap", textClassName)}
      >
        {text}
      </span>
    </span>
  );
}

export function AnimatedText({
  text,
  className,
  textClassName,
  durationMs = 180,
  scrollOverflow = false,
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
    if (scrollOverflow) {
      return (
        <OverflowScrollingText
          text={text}
          className={className}
          textClassName={textClassName}
        />
      );
    }
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
