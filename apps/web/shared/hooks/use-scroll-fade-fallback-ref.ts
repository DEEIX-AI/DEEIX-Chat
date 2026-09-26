"use client";

import * as React from "react";

const overflowVariables = [
  "--scroll-area-overflow-x-start",
  "--scroll-area-overflow-x-end",
  "--scroll-area-overflow-y-start",
  "--scroll-area-overflow-y-end",
] as const;

function normalizeScrollOffset(value: number, maximum: number) {
  if (maximum <= 0) {
    return 0;
  }

  const clamped = Math.min(Math.max(value, 0), maximum);
  if (clamped <= 1) {
    return 0;
  }
  if (maximum - clamped <= 1) {
    return maximum;
  }
  return clamped;
}

function assignRef<T>(ref: React.Ref<T> | undefined, value: T | null) {
  if (typeof ref === "function") {
    ref(value);
    return;
  }
  if (ref) {
    ref.current = value;
  }
}

function supportsScrollDrivenAnimations() {
  return typeof CSS !== "undefined" && CSS.supports("animation-timeline: scroll()");
}

/**
 * Publishes Base UI-compatible overflow metrics for native scroll containers while
 * shadcn-ui/ui#11291 remains unresolved. Remove with the compatibility CSS once these
 * containers use an upstream-supported fallback or all target browsers support scroll timelines.
 */
export function useScrollFadeFallbackRef<T extends HTMLElement>(forwardedRef?: React.Ref<T>) {
  const cleanupRef = React.useRef<(() => void) | null>(null);

  return React.useCallback(
    (node: T | null) => {
      cleanupRef.current?.();
      cleanupRef.current = null;
      assignRef(forwardedRef, node);

      if (!node || supportsScrollDrivenAnimations()) {
        return;
      }

      let active = true;
      let frameID = 0;
      let previousMetrics = [Number.NaN, Number.NaN, Number.NaN, Number.NaN];
      const direction = window.getComputedStyle(node).direction;

      const update = () => {
        frameID = 0;
        if (!active) {
          return;
        }

        const maximumX = Math.max(0, node.scrollWidth - node.clientWidth);
        const maximumY = Math.max(0, node.scrollHeight - node.clientHeight);
        const xStart = normalizeScrollOffset(
          direction === "rtl" ? -node.scrollLeft : node.scrollLeft,
          maximumX,
        );
        const yStart = normalizeScrollOffset(node.scrollTop, maximumY);
        const nextMetrics = [xStart, maximumX - xStart, yStart, maximumY - yStart];

        for (let index = 0; index < overflowVariables.length; index += 1) {
          const value = nextMetrics[index];
          if (previousMetrics[index] !== value) {
            node.style.setProperty(overflowVariables[index], `${value}px`);
          }
        }
        previousMetrics = nextMetrics;
      };

      const scheduleUpdate = () => {
        if (frameID === 0) {
          frameID = window.requestAnimationFrame(update);
        }
      };

      const observedChildren = new Set<Element>();
      const resizeObserver =
        typeof ResizeObserver === "undefined" ? null : new ResizeObserver(scheduleUpdate);
      const syncObservedChildren = () => {
        if (!resizeObserver) {
          return;
        }
        const currentChildren = new Set(node.children);
        for (const child of observedChildren) {
          if (!currentChildren.has(child)) {
            resizeObserver.unobserve(child);
            observedChildren.delete(child);
          }
        }
        for (const child of currentChildren) {
          if (!observedChildren.has(child)) {
            resizeObserver.observe(child);
            observedChildren.add(child);
          }
        }
      };

      resizeObserver?.observe(node);
      syncObservedChildren();
      const mutationObserver =
        typeof MutationObserver === "undefined"
          ? null
          : new MutationObserver(() => {
              syncObservedChildren();
              scheduleUpdate();
            });
      mutationObserver?.observe(node, { childList: true });
      node.addEventListener("scroll", scheduleUpdate, { passive: true });
      update();

      cleanupRef.current = () => {
        active = false;
        node.removeEventListener("scroll", scheduleUpdate);
        resizeObserver?.disconnect();
        mutationObserver?.disconnect();
        if (frameID !== 0) {
          window.cancelAnimationFrame(frameID);
        }
        for (const variable of overflowVariables) {
          node.style.removeProperty(variable);
        }
      };
    },
    [forwardedRef],
  );
}
