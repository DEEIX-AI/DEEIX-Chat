"use client";

import * as React from "react";

import { LAYOUT_STORAGE_PREFIX, parseMessagingLayout } from "../model/launcher-layout";

type Point = { x: number; y: number };
type WindowBounds = Point & { width: number; height: number };
type DragSnapshot = {
  pointerID: number;
  startX: number;
  startY: number;
  origin: Point;
  moved: boolean;
};
type ResizeDirection = "n" | "ne" | "e" | "se" | "s" | "sw" | "w" | "nw";
type ResizeSnapshot = {
  pointerID: number;
  startX: number;
  startY: number;
  direction: ResizeDirection;
  bounds: WindowBounds;
};
const VIEWPORT_MARGIN = 8;
const MIN_WINDOW_WIDTH = 320;
const MIN_WINDOW_HEIGHT = 360;
export const RESIZE_HANDLES: Array<{ direction: ResizeDirection; className: string }> = [
  { direction: "n", className: "-top-1 left-3 right-3 h-2 cursor-n-resize" },
  { direction: "ne", className: "-right-1 -top-1 size-4 cursor-ne-resize" },
  { direction: "e", className: "-right-1 bottom-3 top-3 w-2 cursor-e-resize" },
  { direction: "se", className: "-bottom-1 -right-1 size-5 cursor-se-resize" },
  { direction: "s", className: "-bottom-1 left-3 right-3 h-2 cursor-s-resize" },
  { direction: "sw", className: "-bottom-1 -left-1 size-4 cursor-sw-resize" },
  { direction: "w", className: "-left-1 bottom-3 top-3 w-2 cursor-w-resize" },
  { direction: "nw", className: "-left-1 -top-1 size-4 cursor-nw-resize" },
];

function clamp(value: number, minimum: number, maximum: number) {
  return Math.min(Math.max(value, minimum), Math.max(minimum, maximum));
}

function initialWindowBounds(): WindowBounds {
  if (typeof window === "undefined") {
    return { x: 24, y: 24, width: 400, height: 600 };
  }
  const width = Math.min(400, Math.max(240, window.innerWidth - VIEWPORT_MARGIN * 2));
  const height = Math.min(640, Math.max(320, window.innerHeight - VIEWPORT_MARGIN * 2));
  return {
    x: Math.max(VIEWPORT_MARGIN, window.innerWidth - width - 20),
    y: Math.max(VIEWPORT_MARGIN, window.innerHeight - height - 20),
    width,
    height,
  };
}

function clampWindowBounds(bounds: WindowBounds): WindowBounds {
  const maximumWidth = Math.max(1, window.innerWidth - VIEWPORT_MARGIN * 2);
  const maximumHeight = Math.max(1, window.innerHeight - VIEWPORT_MARGIN * 2);
  const minimumWidth = Math.min(MIN_WINDOW_WIDTH, maximumWidth);
  const minimumHeight = Math.min(MIN_WINDOW_HEIGHT, maximumHeight);
  const width = clamp(bounds.width, minimumWidth, maximumWidth);
  const height = clamp(bounds.height, minimumHeight, maximumHeight);
  return {
    x: clamp(bounds.x, VIEWPORT_MARGIN, window.innerWidth - width - VIEWPORT_MARGIN),
    y: clamp(bounds.y, VIEWPORT_MARGIN, window.innerHeight - height - VIEWPORT_MARGIN),
    width,
    height,
  };
}

function isPoint(value: unknown): value is Point {
  if (!value || typeof value !== "object") return false;
  const candidate = value as Partial<Point>;
  return Number.isFinite(candidate.x) && Number.isFinite(candidate.y);
}

function isWindowBounds(value: unknown): value is WindowBounds {
  if (!isPoint(value)) return false;
  const candidate = value as Partial<WindowBounds>;
  return Number.isFinite(candidate.width) && Number.isFinite(candidate.height);
}

function useMobileMessagingLayout() {
  const [mobile, setMobile] = React.useState(false);

  React.useEffect(() => {
    const media = window.matchMedia("(max-width: 640px)");
    const update = () => setMobile(media.matches);
    update();
    media.addEventListener("change", update);
    return () => media.removeEventListener("change", update);
  }, []);

  return mobile;
}

export function useMessagingWindowLayout(accountID: string) {
  const mobileLayout = useMobileMessagingLayout();
  const [expanded, setExpanded] = React.useState(false);
  const [windowBounds, setWindowBounds] = React.useState<WindowBounds>(initialWindowBounds);
  const windowDragRef = React.useRef<DragSnapshot | null>(null);
  const resizeRef = React.useRef<ResizeSnapshot | null>(null);
  const layoutHydratedRef = React.useRef(false);

  React.useEffect(() => {
    setWindowBounds((current) => clampWindowBounds(current));

    const handleResize = () => {
      setWindowBounds((current) => clampWindowBounds(current));
    };
    window.addEventListener("resize", handleResize);
    return () => window.removeEventListener("resize", handleResize);
  }, []);

  React.useEffect(() => {
    if (!accountID) return;
    layoutHydratedRef.current = false;
    try {
      const raw = window.localStorage.getItem(`${LAYOUT_STORAGE_PREFIX}:${accountID}`);
      if (raw) {
        const stored = parseMessagingLayout(raw);
        if (isWindowBounds(stored.window)) setWindowBounds(clampWindowBounds(stored.window));
      }

    } catch {
      // Invalid local UI state falls back to the safe viewport defaults.
    }
    layoutHydratedRef.current = true;
  }, [accountID]);

  React.useEffect(() => {
    if (!accountID || !layoutHydratedRef.current) return;
    const timer = window.setTimeout(() => {
      try {
        const key = `${LAYOUT_STORAGE_PREFIX}:${accountID}`;
        const raw = window.localStorage.getItem(key);
        const stored = parseMessagingLayout(raw);
        window.localStorage.setItem(key, JSON.stringify({ ...stored, window: windowBounds }));
      } catch { /* Storage may be unavailable in a private browser session. */ }
    }, 200);
    return () => window.clearTimeout(timer);
  }, [accountID, windowBounds]);

  const startWindowDrag = (event: React.PointerEvent<HTMLElement>) => {
    if (
      mobileLayout ||
      event.button !== 0 ||
      (event.target as HTMLElement).closest("button")
    ) {
      return;
    }
    windowDragRef.current = {
      pointerID: event.pointerId,
      startX: event.clientX,
      startY: event.clientY,
      origin: { x: windowBounds.x, y: windowBounds.y },
      moved: false,
    };
    event.currentTarget.setPointerCapture(event.pointerId);
  };

  const moveWindow = (event: React.PointerEvent<HTMLElement>) => {
    const drag = windowDragRef.current;
    if (!drag || drag.pointerID !== event.pointerId) return;
    setWindowBounds((current) =>
      clampWindowBounds({
        ...current,
        x: drag.origin.x + event.clientX - drag.startX,
        y: drag.origin.y + event.clientY - drag.startY,
      }),
    );
  };

  const stopWindowDrag = (event: React.PointerEvent<HTMLElement>) => {
    const drag = windowDragRef.current;
    if (!drag || drag.pointerID !== event.pointerId) return;
    windowDragRef.current = null;
    if (event.currentTarget.hasPointerCapture(event.pointerId)) {
      event.currentTarget.releasePointerCapture(event.pointerId);
    }
  };

  const startResize = (event: React.PointerEvent<HTMLButtonElement>) => {
    if (event.button !== 0) return;
    const direction = event.currentTarget.dataset.direction as ResizeDirection;
    resizeRef.current = {
      pointerID: event.pointerId,
      startX: event.clientX,
      startY: event.clientY,
      direction,
      bounds: windowBounds,
    };
    event.currentTarget.setPointerCapture(event.pointerId);
    event.preventDefault();
  };

  const resizeWindow = (event: React.PointerEvent<HTMLButtonElement>) => {
    const resize = resizeRef.current;
    if (!resize || resize.pointerID !== event.pointerId) return;
    const deltaX = event.clientX - resize.startX;
    const deltaY = event.clientY - resize.startY;
    const minimumWidth = Math.min(
      MIN_WINDOW_WIDTH,
      window.innerWidth - VIEWPORT_MARGIN * 2,
    );
    const minimumHeight = Math.min(
      MIN_WINDOW_HEIGHT,
      window.innerHeight - VIEWPORT_MARGIN * 2,
    );
    let left = resize.bounds.x;
    let right = resize.bounds.x + resize.bounds.width;
    let top = resize.bounds.y;
    let bottom = resize.bounds.y + resize.bounds.height;

    if (resize.direction.includes("e")) {
      right = clamp(
        right + deltaX,
        left + minimumWidth,
        window.innerWidth - VIEWPORT_MARGIN,
      );
    }
    if (resize.direction.includes("w")) {
      left = clamp(left + deltaX, VIEWPORT_MARGIN, right - minimumWidth);
    }
    if (resize.direction.includes("s")) {
      bottom = clamp(
        bottom + deltaY,
        top + minimumHeight,
        window.innerHeight - VIEWPORT_MARGIN,
      );
    }
    if (resize.direction.includes("n")) {
      top = clamp(top + deltaY, VIEWPORT_MARGIN, bottom - minimumHeight);
    }

    setWindowBounds({ x: left, y: top, width: right - left, height: bottom - top });
  };

  const stopResize = (event: React.PointerEvent<HTMLButtonElement>) => {
    const resize = resizeRef.current;
    if (!resize || resize.pointerID !== event.pointerId) return;
    resizeRef.current = null;
    if (event.currentTarget.hasPointerCapture(event.pointerId)) {
      event.currentTarget.releasePointerCapture(event.pointerId);
    }
  };

  return { mobileLayout, expanded, setExpanded, windowBounds, startWindowDrag, moveWindow, stopWindowDrag, startResize, resizeWindow, stopResize };
}
