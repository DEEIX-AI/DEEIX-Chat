"use client";

import * as React from "react";

type UseLoadMoreSentinelOptions = {
  enabled: boolean;
  rootMargin?: string;
  targetRef: React.RefObject<Element | null>;
  onLoadMore: () => void | Promise<void>;
};

export function useLoadMoreSentinel({
  enabled,
  rootMargin = "120px",
  targetRef,
  onLoadMore,
}: UseLoadMoreSentinelOptions) {
  const onLoadMoreRef = React.useRef(onLoadMore);

  React.useEffect(() => {
    onLoadMoreRef.current = onLoadMore;
  }, [onLoadMore]);

  React.useEffect(() => {
    if (!enabled) {
      return;
    }

    const target = targetRef.current;
    if (!target) {
      return;
    }

    const root = target.parentElement?.closest<HTMLElement>("[data-sidebar-scroll-root='true']") ?? null;
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
    return () => observer.disconnect();
  }, [enabled, rootMargin, targetRef]);
}
