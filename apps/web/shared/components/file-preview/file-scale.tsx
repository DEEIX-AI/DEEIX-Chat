"use client";

import * as React from "react";

export type FileScaleConfig = {
  min: number;
  max: number;
  step: number;
  initial: number;
};

type ScrollCenterAnchor = {
  x: number;
  y: number;
};

type UseFileScaleOptions = {
  scrollRef?: React.RefObject<HTMLElement | null>;
};

export function clampFileScale(config: FileScaleConfig, value: number): number {
  return Math.min(config.max, Math.max(config.min, value));
}

function readScrollCenter(node: HTMLElement): ScrollCenterAnchor {
  return {
    x: (node.scrollLeft + node.clientWidth / 2) / Math.max(node.scrollWidth, node.clientWidth, 1),
    y: (node.scrollTop + node.clientHeight / 2) / Math.max(node.scrollHeight, node.clientHeight, 1),
  };
}

function restoreScrollCenter(node: HTMLElement, anchor: ScrollCenterAnchor) {
  node.scrollLeft = anchor.x * Math.max(node.scrollWidth, node.clientWidth, 1) - node.clientWidth / 2;
  node.scrollTop = anchor.y * Math.max(node.scrollHeight, node.clientHeight, 1) - node.clientHeight / 2;
}

export function useFileScale(config: FileScaleConfig, options: UseFileScaleOptions = {}) {
  const { min, max, step, initial } = config;
  const scrollRefStoreRef = React.useRef(options.scrollRef);
  const pendingAnchorRef = React.useRef<ScrollCenterAnchor | null>(null);
  const [scale, setScaleState] = React.useState(initial);

  React.useLayoutEffect(() => {
    scrollRefStoreRef.current = options.scrollRef;
  }, [options.scrollRef]);

  const setScale = React.useCallback(
    (nextScale: number | ((current: number) => number)) => {
      const scrollNode = scrollRefStoreRef.current?.current ?? null;
      if (scrollNode) {
        pendingAnchorRef.current = readScrollCenter(scrollNode);
      }

      setScaleState((current) => {
        const resolvedScale = typeof nextScale === "function" ? nextScale(current) : nextScale;
        return Math.min(max, Math.max(min, resolvedScale));
      });
    },
    [max, min],
  );

  React.useLayoutEffect(() => {
    const anchor = pendingAnchorRef.current;
    const scrollNode = scrollRefStoreRef.current?.current ?? null;
    if (!anchor || !scrollNode) {
      return;
    }

    pendingAnchorRef.current = null;
    restoreScrollCenter(scrollNode, anchor);
  }, [scale]);

  const zoomOut = React.useCallback(() => {
    setScale((current) => current - step);
  }, [setScale, step]);

  const zoomIn = React.useCallback(() => {
    setScale((current) => current + step);
  }, [setScale, step]);

  return {
    scale,
    setScale,
    zoomOut,
    zoomIn,
    canZoomOut: scale > min,
    canZoomIn: scale < max,
  };
}
