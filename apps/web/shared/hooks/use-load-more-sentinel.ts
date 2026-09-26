"use client";

import * as React from "react";

type UseLoadMoreSentinelOptions = {
  enabled: boolean;
  rootMargin?: string;
  rootRef?: React.RefObject<HTMLElement | null>;
  onLoadMore: () => void | Promise<void>;
};

function rootMarginToPixels(rootMargin: string): number {
  const values = rootMargin.trim().split(/\s+/).filter(Boolean);
  const value = values.length >= 3 ? values[2] : values[0] ?? "";
  const match = value.match(/^(-?\d+(?:\.\d+)?)px$/);
  if (!match) {
    return 120;
  }
  return Number(match[1]) || 0;
}

export function useLoadMoreSentinel<T extends Element = HTMLElement>({
  enabled,
  rootMargin = "120px",
  rootRef,
  onLoadMore,
}: UseLoadMoreSentinelOptions): React.RefCallback<T> {
  const [target, setTarget] = React.useState<T | null>(null);
  const onLoadMoreRef = React.useRef(onLoadMore);
  const targetRef = React.useCallback((node: T | null) => {
    setTarget(node);
  }, []);

  React.useEffect(() => {
    onLoadMoreRef.current = onLoadMore;
  }, [onLoadMore]);

  React.useEffect(() => {
    if (!enabled) {
      return;
    }

    if (!target) {
      return;
    }

    const root = rootRef?.current ?? target.parentElement?.closest<HTMLElement>("[data-sidebar-scroll-root='true']") ?? null;
    const marginPx = rootMarginToPixels(rootMargin);
    let animationFrame: number | null = null;
    let disposed = false;

    const isNearLoadMorePoint = () => {
      if (root) {
        return root.scrollHeight - root.scrollTop - root.clientHeight <= marginPx;
      }
      return target.getBoundingClientRect().top <= window.innerHeight + marginPx;
    };

    const check = () => {
      animationFrame = null;
      if (disposed || !isNearLoadMorePoint()) {
        return;
      }
      void onLoadMoreRef.current();
    };

    const scheduleCheck = () => {
      if (animationFrame !== null) {
        return;
      }
      animationFrame = window.requestAnimationFrame(check);
    };

    const observer = new IntersectionObserver(
      (entries) => {
        const entry = entries[0];
        if (entry?.isIntersecting) {
          void onLoadMoreRef.current();
        }
      },
      { root, rootMargin },
    );

    observer.observe(target);
    const scrollTarget: HTMLElement | Window = root ?? window;
    scrollTarget.addEventListener("scroll", scheduleCheck, { passive: true });
    window.addEventListener("resize", scheduleCheck);

    const resizeObserver = typeof ResizeObserver === "undefined" ? null : new ResizeObserver(scheduleCheck);
    resizeObserver?.observe(target);
    if (root) {
      resizeObserver?.observe(root);
    }

    scheduleCheck();

    return () => {
      disposed = true;
      observer.disconnect();
      scrollTarget.removeEventListener("scroll", scheduleCheck);
      window.removeEventListener("resize", scheduleCheck);
      resizeObserver?.disconnect();
      if (animationFrame !== null) {
        window.cancelAnimationFrame(animationFrame);
      }
    };
  }, [enabled, rootMargin, rootRef, target]);

  return targetRef;
}
