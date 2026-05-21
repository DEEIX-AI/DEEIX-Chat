"use client";

import * as React from "react";

type ProgressiveRowsOptions = {
  initialCount?: number;
  step?: number;
  disabled?: boolean;
};

export function useProgressiveRows<T>(
  rows: readonly T[],
  {
    initialCount = 12,
    step = 16,
    disabled = false,
  }: ProgressiveRowsOptions = {},
) {
  const normalizedInitialCount = Math.max(1, initialCount);
  const normalizedStep = Math.max(1, step);
  const [visibleCount, setVisibleCount] = React.useState(() =>
    disabled ? rows.length : Math.min(rows.length, normalizedInitialCount),
  );
  const previousLengthRef = React.useRef(rows.length);

  React.useEffect(() => {
    const previousLength = previousLengthRef.current;
    previousLengthRef.current = rows.length;
    if (disabled || rows.length <= normalizedInitialCount) {
      setVisibleCount(rows.length);
      return;
    }
    if (rows.length <= previousLength) {
      setVisibleCount(rows.length);
      return;
    }

    let cancelled = false;
    let frameID = 0;
    setVisibleCount((current) => Math.min(rows.length, Math.max(current, normalizedInitialCount)));

    const scheduleNext = () => {
      frameID = window.requestAnimationFrame(() => {
        if (cancelled) {
          return;
        }
        React.startTransition(() => {
          setVisibleCount((current) => {
            const next = Math.min(rows.length, current + normalizedStep);
            if (next < rows.length) {
              scheduleNext();
            }
            return next;
          });
        });
      });
    };

    scheduleNext();

    return () => {
      cancelled = true;
      window.cancelAnimationFrame(frameID);
    };
  }, [disabled, normalizedInitialCount, normalizedStep, rows]);

  const safeVisibleCount = Math.min(visibleCount, rows.length);
  const visibleRows = React.useMemo(
    () => rows.slice(0, safeVisibleCount),
    [rows, safeVisibleCount],
  );

  return {
    visibleRows,
    visibleCount: safeVisibleCount,
    isRendering: safeVisibleCount < rows.length,
  };
}
