"use client";

import { ChevronLeft, ChevronRight, MessageCircle } from "lucide-react";
import { useTranslations } from "next-intl";
import * as React from "react";

import { Button } from "@/components/ui/button";
import { cn } from "@/lib/utils";

import {
  clampLauncherPoint,
  dockLauncherPoint,
  LAUNCHER_MARGIN,
  LAUNCHER_SIZE,
  LAUNCHER_TAB_WIDTH,
  LAYOUT_STORAGE_PREFIX,
  type LauncherPoint,
  type LauncherPosition,
  parseMessagingLayout,
  restoreLauncherPosition,
} from "../model/launcher-layout";

type DragSnapshot = {
  pointerID: number;
  startX: number;
  startY: number;
  origin: LauncherPoint;
  point: LauncherPoint;
  moved: boolean;
};

const IDLE_DELAY = 3_000;

export function InternalMessagingLauncherButton({
  accountID,
  unreadCount,
  label,
  onOpen,
}: {
  accountID: string;
  unreadCount: number;
  label: string;
  onOpen: () => void;
}) {
  const t = useTranslations("internalMessaging");
  const [position, setPosition] = React.useState<LauncherPosition | null>(null);
  const [collapsed, setCollapsed] = React.useState(false);
  const [hovered, setHovered] = React.useState(false);
  const [keyboardFocused, setKeyboardFocused] = React.useState(false);
  const [dragging, setDragging] = React.useState(false);
  const [highlighted, setHighlighted] = React.useState(false);
  const dragRef = React.useRef<DragSnapshot | null>(null);
  const suppressClickRef = React.useRef(false);
  const pointerTypeRef = React.useRef("");
  const previousUnreadRef = React.useRef(unreadCount);

  React.useEffect(() => {
    let stored: unknown;
    try {
      const raw = window.localStorage.getItem(`${LAYOUT_STORAGE_PREFIX}:${accountID}`);
      stored = parseMessagingLayout(raw);
    } catch {
      // Use the viewport corner when storage is unavailable or invalid.
    }
    setPosition(restoreLauncherPosition(stored, window.innerWidth, window.innerHeight));
    const resize = () => {
      setPosition((current) => current
        ? dockLauncherPoint(current.point, window.innerWidth, window.innerHeight, current.edge)
        : current);
    };
    resize();
    window.addEventListener("resize", resize);
    return () => window.removeEventListener("resize", resize);
  }, [accountID]);

  React.useEffect(() => {
    if (!position || collapsed || hovered || keyboardFocused || dragging) return;
    const timer = window.setTimeout(() => setCollapsed(true), IDLE_DELAY);
    return () => window.clearTimeout(timer);
  }, [position, collapsed, hovered, keyboardFocused, dragging]);

  React.useEffect(() => {
    const increased = unreadCount > previousUnreadRef.current;
    previousUnreadRef.current = unreadCount;
    setHighlighted(increased);
    if (!increased) return;
    const timer = window.setTimeout(() => setHighlighted(false), 2_400);
    return () => window.clearTimeout(timer);
  }, [unreadCount]);

  const startDrag = (event: React.PointerEvent<HTMLButtonElement>) => {
    if (event.button !== 0 || !event.isPrimary || dragRef.current) return;
    pointerTypeRef.current = event.pointerType;
    suppressClickRef.current = false;
    setKeyboardFocused(false);
    if (collapsed) return;
    const rect = event.currentTarget.getBoundingClientRect();
    const origin = { x: rect.left, y: rect.top };
    dragRef.current = {
      pointerID: event.pointerId,
      startX: event.clientX,
      startY: event.clientY,
      origin,
      point: origin,
      moved: false,
    };
    // Disable position transitions as soon as the pointer is pressed.
    setPosition((current) => current ? { ...current, point: origin } : current);
    setDragging(true);
    event.currentTarget.setPointerCapture(event.pointerId);
  };

  const move = (event: React.PointerEvent<HTMLButtonElement>) => {
    const drag = dragRef.current;
    if (!drag || drag.pointerID !== event.pointerId) return;
    const deltaX = event.clientX - drag.startX;
    const deltaY = event.clientY - drag.startY;
    if (!drag.moved && Math.abs(deltaX) + Math.abs(deltaY) <= 3) return;
    drag.moved = true;
    drag.point = clampLauncherPoint(
      { x: drag.origin.x + deltaX, y: drag.origin.y + deltaY },
      window.innerWidth,
      window.innerHeight,
    );
    setPosition((current) => current ? { ...current, point: drag.point } : current);
  };

  const stopDrag = (event: React.PointerEvent<HTMLButtonElement>) => {
    const drag = dragRef.current;
    if (!drag || drag.pointerID !== event.pointerId) return;
    dragRef.current = null;
    suppressClickRef.current = drag.moved || event.type !== "pointerup";
    setDragging(false);
    if (drag.moved) {
      const docked = dockLauncherPoint(drag.point, window.innerWidth, window.innerHeight);
      setPosition(docked);
      setHovered(false);
      try {
        // Save before opening the lazy window; preserve its separate bounds.
        const key = `${LAYOUT_STORAGE_PREFIX}:${accountID}`;
        const raw = window.localStorage.getItem(key);
        const stored = parseMessagingLayout(raw);
        window.localStorage.setItem(key, JSON.stringify({
          ...stored,
          button: docked.point,
          buttonEdge: docked.edge,
        }));
      } catch {
        // Dragging remains available without local storage.
      }
    }
    if (event.currentTarget.hasPointerCapture(event.pointerId)) {
      event.currentTarget.releasePointerCapture(event.pointerId);
    }
  };

  const edge = position?.edge ?? "right";
  const size = collapsed ? LAUNCHER_TAB_WIDTH : LAUNCHER_SIZE;
  const inset = `max(${collapsed ? 0 : LAUNCHER_MARGIN}px, env(safe-area-inset-${edge}))`;
  const left = dragging && position
    ? position.point.x
    : edge === "left" ? inset : `calc(100% - ${size}px - ${inset})`;
  const top = position
    ? `clamp(max(8px, env(safe-area-inset-top)), ${position.point.y}px, calc(100% - 44px - max(8px, env(safe-area-inset-bottom))))`
    : "calc(100% - 44px - max(20px, env(safe-area-inset-bottom)))";
  const unreadLabel = unreadCount > 0 ? t("aria.unread", { count: unreadCount }) : "";

  return (
    <Button
      aria-label={unreadLabel ? `${label}, ${unreadLabel}` : label}
      title={collapsed ? t("aria.expandLauncher") : label}
      data-messaging-launcher=""
      data-collapsed={collapsed}
      data-edge={edge}
      data-highlighted={highlighted}
      className={cn(
        "fixed z-[69] h-11 select-none p-0 shadow-lg transition-[left,top,width,border-radius,background-color,box-shadow] duration-200 motion-reduce:transition-none focus-visible:outline-2 focus-visible:outline-offset-2 focus-visible:outline-ring",
        "touch-none cursor-grab active:cursor-grabbing",
        collapsed
          ? edge === "left" ? "rounded-l-none rounded-r-xl" : "rounded-l-xl rounded-r-none"
          : "rounded-full",
        dragging && "transition-none",
        highlighted && "ring-2 ring-primary/60 ring-offset-2 ring-offset-background",
      )}
      style={{ left, top, width: size }}
      size="icon"
      onPointerDown={startDrag}
      onPointerMove={move}
      onPointerUp={stopDrag}
      onPointerCancel={stopDrag}
      onLostPointerCapture={stopDrag}
      onPointerEnter={(event) => {
        if (event.pointerType === "touch" || dragRef.current) return;
        setHovered(true);
        setCollapsed(false);
      }}
      onPointerLeave={() => setHovered(false)}
      onFocus={(event) => {
        if (!event.currentTarget.matches(":focus-visible")) return;
        setKeyboardFocused(true);
        setCollapsed(false);
      }}
      onBlur={() => setKeyboardFocused(false)}
      onClick={(event) => {
        if (suppressClickRef.current && event.detail !== 0) {
          suppressClickRef.current = false;
          return;
        }
        if (collapsed && pointerTypeRef.current === "touch" && event.detail !== 0) {
          setCollapsed(false);
          return;
        }
        onOpen();
      }}
    >
      {collapsed ? (
        unreadCount > 0 ? (
          <span aria-hidden="true" className="rounded-full bg-destructive px-0.5 text-[10px] leading-4 text-destructive-foreground">
            {unreadCount > 99 ? "99+" : unreadCount}
          </span>
        ) : edge === "left" ? <ChevronRight className="size-4" /> : <ChevronLeft className="size-4" />
      ) : (
        <>
          <MessageCircle className="size-5" />
          {unreadCount > 0 ? (
            <span aria-hidden="true" className={cn(
              "absolute -top-1 flex min-w-4 items-center justify-center rounded-full bg-destructive px-1 text-[10px] text-destructive-foreground",
              edge === "left" ? "-right-1" : "-left-1",
            )}>
              {unreadCount > 99 ? "99+" : unreadCount}
            </span>
          ) : null}
        </>
      )}
    </Button>
  );
}
