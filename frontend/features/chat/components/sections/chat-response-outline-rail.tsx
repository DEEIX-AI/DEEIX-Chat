"use client";

import { useTranslations } from "next-intl";
import * as React from "react";

import { HoverCard, HoverCardContent, HoverCardTrigger } from "@/components/ui/hover-card";
import {
  useMessageScroller,
  useMessageScrollerVisibility,
} from "@/components/ui/message-scroller";
import { cn } from "@/lib/utils";

type ResponseOutlineHeading = {
  label: string;
  level: number;
};

function rectIntersectsViewport(rect: DOMRect, viewportRect: DOMRect) {
  return rect.bottom > viewportRect.top && rect.top < viewportRect.bottom;
}

function distanceToGuide(rect: DOMRect, guideY: number) {
  if (rect.top <= guideY && rect.bottom >= guideY) {
    return 0;
  }
  return Math.min(Math.abs(rect.top - guideY), Math.abs(rect.bottom - guideY));
}

function headingLevel(element: HTMLElement) {
  const parsed = Number.parseInt(element.tagName.slice(1), 10);
  return Number.isFinite(parsed) ? parsed : 1;
}

function normalizedHeadingLabel(element: HTMLElement) {
  return (element.textContent ?? "").replace(/\s+/g, " ").trim();
}

function outlineSignature(messageID: string, headings: ResponseOutlineHeading[]) {
  return `${messageID}\u0000${headings.map((item) => `${item.level}:${item.label}`).join("\u0001")}`;
}

function mutationAffectsOutline(mutation: MutationRecord) {
  const targetElement =
    mutation.target instanceof Element ? mutation.target : mutation.target.parentElement;
  if (
    targetElement?.closest(
      "[data-chat-assistant-content] [data-chat-markdown-scope] :is(h1, h2, h3)",
    )
  ) {
    return true;
  }

  return [...mutation.addedNodes, ...mutation.removedNodes].some(
    (node) =>
      node instanceof Element &&
      (node.matches(
        "[data-chat-assistant-content] [data-chat-markdown-scope] :is(h1, h2, h3)",
      ) ||
        node.querySelector(
          "[data-chat-assistant-content] [data-chat-markdown-scope] :is(h1, h2, h3)",
        )),
  );
}

function keepItemVisible(viewport: HTMLElement, item: HTMLElement) {
  const viewportRect = viewport.getBoundingClientRect();
  const itemRect = item.getBoundingClientRect();
  const offsetBefore = itemRect.top - viewportRect.top - 4;
  const offsetAfter = itemRect.bottom - viewportRect.bottom + 4;
  const offset = offsetBefore < 0 ? offsetBefore : offsetAfter > 0 ? offsetAfter : 0;
  if (offset === 0) {
    return;
  }

  const reducedMotion = window.matchMedia("(prefers-reduced-motion: reduce)").matches;
  viewport.scrollTo({
    top: Math.max(0, viewport.scrollTop + offset),
    behavior: reducedMotion ? "auto" : "smooth",
  });
}

function resolveActiveAssistantMessage(
  viewport: HTMLElement,
  visibleMessageIDs: Iterable<string>,
): HTMLElement | null {
  const candidateIDs = Array.from(new Set(visibleMessageIDs));
  if (candidateIDs.length === 0) {
    return null;
  }

  const viewportRect = viewport.getBoundingClientRect();
  const guideY = viewportRect.top + viewportRect.height * 0.28;
  const selector = candidateIDs
    .map(
      (messageID) =>
        `[data-chat-message-role="assistant"][data-chat-message-id="${CSS.escape(messageID)}"]`,
    )
    .join(", ");
  const candidates = Array.from(
    viewport.querySelectorAll<HTMLElement>(selector),
    (element) => ({ element, rect: element.getBoundingClientRect() }),
  ).filter((candidate) => rectIntersectsViewport(candidate.rect, viewportRect));

  if (candidates.length === 0) {
    return null;
  }

  const guideCandidate = candidates.find(
    (candidate) => distanceToGuide(candidate.rect, guideY) === 0,
  );
  if (guideCandidate) {
    return guideCandidate.element;
  }

  return candidates.reduce((closest, candidate) =>
    distanceToGuide(candidate.rect, guideY) < distanceToGuide(closest.rect, guideY)
      ? candidate
      : closest,
  ).element;
}

function ChatResponseOutlineRailComponent({
  boundaryRef,
  disabled = false,
}: {
  boundaryRef: React.RefObject<HTMLDivElement | null>;
  disabled?: boolean;
}) {
  const t = useTranslations("chat.messages");
  const { scrollToMessage } = useMessageScroller();
  const { visibleMessageIds } = useMessageScrollerVisibility();
  const [headings, setHeadings] = React.useState<ResponseOutlineHeading[]>([]);
  const [activeHeadingIndex, setActiveHeadingIndex] = React.useState(0);
  const [hoveredHeadingIndex, setHoveredHeadingIndex] = React.useState<number | null>(null);
  const [outlineOpen, setOutlineOpen] = React.useState(false);
  const [railOverflowing, setRailOverflowing] = React.useState(false);
  const headingElementsRef = React.useRef<HTMLElement[]>([]);
  const headingSignatureRef = React.useRef("");
  const activeMessageRef = React.useRef<HTMLElement | null>(null);
  const outlineRefreshRequestedRef = React.useRef(true);
  const scanFrameRef = React.useRef<number | null>(null);
  const contentScanTimerRef = React.useRef<number | null>(null);
  const navigationTargetRef = React.useRef<HTMLElement | null>(null);
  const navigationSettleTimerRef = React.useRef<number | null>(null);
  const railViewportRef = React.useRef<HTMLDivElement | null>(null);
  const railContentRef = React.useRef<HTMLDivElement | null>(null);
  const railItemRefs = React.useRef(new Map<number, HTMLButtonElement>());
  const menuViewportRef = React.useRef<HTMLDivElement | null>(null);
  const menuItemRefs = React.useRef(new Map<number, HTMLButtonElement>());
  const visibleMessageIDsRef = React.useRef(visibleMessageIds);

  const clearNavigationTarget = React.useCallback(() => {
    navigationTargetRef.current = null;
    if (navigationSettleTimerRef.current !== null) {
      window.clearTimeout(navigationSettleTimerRef.current);
      navigationSettleTimerRef.current = null;
    }
  }, []);

  const clearOutline = React.useCallback(() => {
    clearNavigationTarget();
    activeMessageRef.current = null;
    outlineRefreshRequestedRef.current = true;
    headingElementsRef.current = [];
    setOutlineOpen(false);
    setHoveredHeadingIndex(null);
    if (headingSignatureRef.current) {
      headingSignatureRef.current = "";
      setHeadings([]);
    }
    setActiveHeadingIndex(0);
  }, [clearNavigationTarget]);

  const updateActiveHeading = React.useCallback(() => {
    const viewport = boundaryRef.current;
    const elements = headingElementsRef.current;
    if (!viewport || elements.length === 0) {
      setActiveHeadingIndex(0);
      return;
    }

    const navigationTarget = navigationTargetRef.current;
    if (navigationTarget) {
      const navigationTargetIndex = elements.indexOf(navigationTarget);
      if (navigationTargetIndex >= 0) {
        setActiveHeadingIndex((current) =>
          current === navigationTargetIndex ? current : navigationTargetIndex,
        );
        return;
      }
      clearNavigationTarget();
    }

    const viewportRect = viewport.getBoundingClientRect();
    const guideY = viewportRect.top + viewportRect.height * 0.28;
    let nextIndex = 0;
    for (let index = 0; index < elements.length; index += 1) {
      if (elements[index]?.getBoundingClientRect().top <= guideY) {
        nextIndex = index;
        continue;
      }
      break;
    }
    setActiveHeadingIndex((current) => (current === nextIndex ? current : nextIndex));
  }, [boundaryRef, clearNavigationTarget]);

  const scanOutline = React.useCallback(() => {
    scanFrameRef.current = null;
    const viewport = boundaryRef.current;
    if (!viewport || disabled) {
      clearOutline();
      return;
    }

    const navigationTarget = navigationTargetRef.current;
    const navigationMessage = navigationTarget?.isConnected
      ? navigationTarget.closest<HTMLElement>('[data-chat-message-role="assistant"]')
      : null;
    const message =
      navigationMessage ?? resolveActiveAssistantMessage(viewport, visibleMessageIDsRef.current);
    if (!message) {
      clearOutline();
      return;
    }

    if (activeMessageRef.current === message && !outlineRefreshRequestedRef.current) {
      updateActiveHeading();
      return;
    }
    activeMessageRef.current = message;
    outlineRefreshRequestedRef.current = false;

    const viewportRect = viewport.getBoundingClientRect();
    const messageRect = message.getBoundingClientRect();
    const minimumResponseHeight = Math.max(
      360,
      Math.min(560, viewportRect.height * 0.72),
    );
    const elements = Array.from(
      message.querySelectorAll<HTMLElement>(
        "[data-chat-assistant-content] [data-chat-markdown-scope] :is(h1, h2, h3)",
      ),
    );
    const nextHeadings = elements
      .map((element) => ({
        element,
        heading: {
          label: normalizedHeadingLabel(element),
          level: headingLevel(element),
        },
      }))
      .filter((item) => item.heading.label.length > 0);
    const outlineVisible =
      nextHeadings.length >= 2 && messageRect.height >= minimumResponseHeight;
    const visibleElements = outlineVisible ? nextHeadings.map((item) => item.element) : [];
    const visibleHeadings = outlineVisible ? nextHeadings.map((item) => item.heading) : [];
    const messageID = message.dataset.chatMessageId ?? "";
    const nextSignature = outlineSignature(messageID, visibleHeadings);

    headingElementsRef.current = visibleElements;
    if (headingSignatureRef.current !== nextSignature) {
      headingSignatureRef.current = nextSignature;
      setHoveredHeadingIndex(null);
      setHeadings(visibleHeadings);
    }
    updateActiveHeading();
  }, [boundaryRef, clearOutline, disabled, updateActiveHeading]);

  const scheduleOutlineScan = React.useCallback(() => {
    if (scanFrameRef.current !== null) {
      return;
    }
    scanFrameRef.current = window.requestAnimationFrame(scanOutline);
  }, [scanOutline]);

  const scheduleContentOutlineScan = React.useCallback(() => {
    outlineRefreshRequestedRef.current = true;
    if (contentScanTimerRef.current !== null) {
      return;
    }
    contentScanTimerRef.current = window.setTimeout(() => {
      contentScanTimerRef.current = null;
      if (outlineRefreshRequestedRef.current) {
        scheduleOutlineScan();
      }
    }, 120);
  }, [scheduleOutlineScan]);

  const scheduleImmediateOutlineRefresh = React.useCallback(() => {
    outlineRefreshRequestedRef.current = true;
    scheduleOutlineScan();
  }, [scheduleOutlineScan]);

  const scheduleNavigationSettle = React.useCallback(() => {
    if (navigationSettleTimerRef.current !== null) {
      window.clearTimeout(navigationSettleTimerRef.current);
    }
    navigationSettleTimerRef.current = window.setTimeout(() => {
      navigationSettleTimerRef.current = null;
      navigationTargetRef.current = null;
      scheduleOutlineScan();
    }, 120);
  }, [scheduleOutlineScan]);

  const handleViewportScroll = React.useCallback(() => {
    if (navigationTargetRef.current) {
      scheduleNavigationSettle();
    }
    scheduleOutlineScan();
  }, [scheduleNavigationSettle, scheduleOutlineScan]);

  const cancelNavigationTarget = React.useCallback(() => {
    if (!navigationTargetRef.current) {
      return;
    }
    clearNavigationTarget();
    scheduleOutlineScan();
  }, [clearNavigationTarget, scheduleOutlineScan]);

  React.useLayoutEffect(() => {
    visibleMessageIDsRef.current = visibleMessageIds;
    scheduleOutlineScan();
  }, [scheduleOutlineScan, visibleMessageIds]);

  React.useEffect(() => {
    const viewport = boundaryRef.current;
    if (!viewport) {
      return;
    }

    viewport.addEventListener("scroll", handleViewportScroll, { passive: true });
    viewport.addEventListener("wheel", cancelNavigationTarget, { passive: true });
    viewport.addEventListener("touchstart", cancelNavigationTarget, { passive: true });
    viewport.addEventListener("pointerdown", cancelNavigationTarget, { passive: true });
    const content = viewport.querySelector<HTMLElement>('[data-slot="message-scroller-content"]');
    const viewportResizeObserver =
      typeof ResizeObserver === "undefined"
        ? null
        : new ResizeObserver(scheduleImmediateOutlineRefresh);
    const contentResizeObserver =
      content && typeof ResizeObserver !== "undefined"
        ? new ResizeObserver(scheduleContentOutlineScan)
        : null;
    const contentMutationObserver =
      content && typeof MutationObserver !== "undefined"
        ? new MutationObserver((mutations) => {
            if (mutations.some(mutationAffectsOutline)) {
              scheduleContentOutlineScan();
            }
          })
        : null;

    viewportResizeObserver?.observe(viewport);
    if (content) {
      contentResizeObserver?.observe(content);
      contentMutationObserver?.observe(content, {
        childList: true,
        subtree: true,
        characterData: true,
      });
    }

    return () => {
      viewport.removeEventListener("scroll", handleViewportScroll);
      viewport.removeEventListener("wheel", cancelNavigationTarget);
      viewport.removeEventListener("touchstart", cancelNavigationTarget);
      viewport.removeEventListener("pointerdown", cancelNavigationTarget);
      viewportResizeObserver?.disconnect();
      contentResizeObserver?.disconnect();
      contentMutationObserver?.disconnect();
      clearNavigationTarget();
      if (contentScanTimerRef.current !== null) {
        window.clearTimeout(contentScanTimerRef.current);
        contentScanTimerRef.current = null;
      }
      if (scanFrameRef.current !== null) {
        window.cancelAnimationFrame(scanFrameRef.current);
        scanFrameRef.current = null;
      }
    };
  }, [
    boundaryRef,
    cancelNavigationTarget,
    clearNavigationTarget,
    handleViewportScroll,
    scheduleContentOutlineScan,
    scheduleImmediateOutlineRefresh,
  ]);

  React.useLayoutEffect(() => {
    const railViewport = railViewportRef.current;
    const menuViewport = menuViewportRef.current;
    const targetIndex = hoveredHeadingIndex ?? activeHeadingIndex;
    const railItem = railItemRefs.current.get(targetIndex);
    const menuItem = menuItemRefs.current.get(targetIndex);
    if (railViewport && railItem) {
      keepItemVisible(railViewport, railItem);
    }
    if (menuViewport && menuItem) {
      keepItemVisible(menuViewport, menuItem);
    }
  }, [activeHeadingIndex, headings.length, hoveredHeadingIndex, outlineOpen]);

  React.useLayoutEffect(() => {
    const railViewport = railViewportRef.current;
    const railContent = railContentRef.current;
    if (!railViewport || !railContent) {
      return;
    }

    const updateOverflow = () => {
      const overflowing = railContent.scrollHeight > railViewport.clientHeight + 1;
      setRailOverflowing((current) => (current === overflowing ? current : overflowing));
    };
    updateOverflow();

    if (typeof ResizeObserver === "undefined") {
      return;
    }
    const resizeObserver = new ResizeObserver(updateOverflow);
    resizeObserver.observe(railViewport);
    resizeObserver.observe(railContent);
    return () => resizeObserver.disconnect();
  }, [headings.length]);

  const scrollToHeading = React.useCallback(
    (index: number) => {
      const heading = headingElementsRef.current[index];
      const message = heading?.closest<HTMLElement>('[data-chat-message-role="assistant"]');
      const messageID = message?.dataset.chatMessageId?.trim() ?? "";
      if (!heading || !message || !messageID) {
        return;
      }

      const headingRect = heading.getBoundingClientRect();
      const messageRect = message.getBoundingClientRect();
      const headingOffset = headingRect.top - messageRect.top;
      const reducedMotion = window.matchMedia("(prefers-reduced-motion: reduce)").matches;
      navigationTargetRef.current = heading;
      const scrolled = scrollToMessage(messageID, {
        align: "start",
        behavior: reducedMotion ? "auto" : "smooth",
        scrollMargin: 24 - headingOffset,
      });
      if (scrolled) {
        setActiveHeadingIndex(index);
        scheduleNavigationSettle();
      } else {
        clearNavigationTarget();
      }
    },
    [clearNavigationTarget, scheduleNavigationSettle, scrollToMessage],
  );

  if (disabled || headings.length < 2) {
    return null;
  }

  const minimumLevel = Math.min(...headings.map((item) => item.level));
  return (
    <HoverCard
      open={outlineOpen}
      openDelay={0}
      closeDelay={180}
      onOpenChange={(open) => {
        setOutlineOpen(open);
        if (!open) {
          setHoveredHeadingIndex(null);
        }
      }}
    >
      <HoverCardTrigger asChild>
        <div
          ref={railViewportRef}
          className="pointer-events-auto absolute bottom-3 right-2 top-3 z-30 hidden w-6 overflow-y-auto overscroll-contain text-muted-foreground/55 [scrollbar-width:none] lg:block [&::-webkit-scrollbar]:hidden"
          role="navigation"
          aria-label={t("responseOutline")}
          data-screenshot-exclude="true"
        >
          <div
            ref={railContentRef}
            className={cn(
              "flex min-h-full flex-col items-center gap-1 px-1 py-1",
              !railOverflowing && "justify-center",
            )}
          >
            {headings.map((item, index) => {
              const active = index === activeHeadingIndex;
              const hovered = index === hoveredHeadingIndex;
              const depth = Math.min(2, Math.max(0, item.level - minimumLevel));
              return (
                <button
                  key={`${item.level}:${item.label}:${index}`}
                  ref={(node) => {
                    if (node) {
                      railItemRefs.current.set(index, node);
                      return;
                    }
                    railItemRefs.current.delete(index);
                  }}
                  type="button"
                  className="flex h-1.5 w-6 items-center justify-end rounded-sm focus-visible:outline-none"
                  aria-current={active ? "location" : undefined}
                  aria-label={t("jumpToResponseSection", { title: item.label })}
                  onMouseEnter={() => setHoveredHeadingIndex(index)}
                  onFocus={() => setHoveredHeadingIndex(index)}
                  onClick={() => scrollToHeading(index)}
                >
                  <span
                    className={cn(
                      "h-0.5 rounded-full bg-current opacity-35 transition-[color,opacity,width] duration-150 ease-out",
                      hovered && !active && "text-foreground/55 opacity-100",
                      active && "text-foreground opacity-100",
                    )}
                    style={{ width: depth === 0 ? "1rem" : depth === 1 ? "0.75rem" : "0.5rem" }}
                  />
                </button>
              );
            })}
          </div>
        </div>
      </HoverCardTrigger>
      <HoverCardContent
        side="left"
        align="center"
        sideOffset={8}
        avoidCollisions={false}
        className="z-30 flex max-h-[min(68vh,34rem)] w-72 flex-col overflow-hidden rounded-lg border-0 bg-sidebar-accent p-1 text-foreground shadow-none"
        role="navigation"
        aria-label={t("responseOutline")}
        data-screenshot-exclude="true"
        onPointerDownOutside={(event) => {
          const target = event.detail.originalEvent.target;
          if (target instanceof Node && railViewportRef.current?.contains(target)) {
            event.preventDefault();
          }
        }}
      >
        <div className="shrink-0 px-2 pb-1 pt-1 text-[11px] font-medium text-muted-foreground">
          {t("responseOutline")}
        </div>
        <div
          ref={menuViewportRef}
          className="scroll-fade-y scroll-fade-8 flex min-h-0 flex-1 flex-col gap-0.5 overflow-y-auto overscroll-contain [scrollbar-width:none] [&::-webkit-scrollbar]:hidden"
          onMouseLeave={(event) => {
            if (!event.currentTarget.contains(document.activeElement)) {
              setHoveredHeadingIndex(null);
            }
          }}
          onBlur={(event) => {
            const nextFocusedElement = event.relatedTarget;
            if (
              !(nextFocusedElement instanceof Node) ||
              !event.currentTarget.contains(nextFocusedElement)
            ) {
              setHoveredHeadingIndex(null);
            }
          }}
        >
          {headings.map((item, index) => {
            const active = index === activeHeadingIndex;
            const hovered = index === hoveredHeadingIndex;
            const depth = Math.min(2, Math.max(0, item.level - minimumLevel));
            return (
              <button
                key={`${item.level}:${item.label}:${index}`}
                ref={(node) => {
                  if (node) {
                    menuItemRefs.current.set(index, node);
                    return;
                  }
                  menuItemRefs.current.delete(index);
                }}
                type="button"
                className={cn(
                  "block w-full shrink-0 truncate rounded-md py-1 pr-2 text-left text-xs leading-5 text-muted-foreground transition-colors duration-150 hover:bg-foreground/[0.04] hover:text-foreground focus-visible:bg-foreground/[0.04] focus-visible:text-foreground focus-visible:outline-none",
                  hovered && !active && "bg-foreground/[0.04] text-foreground",
                  active && "bg-foreground/[0.05] font-medium text-foreground",
                )}
                style={{ paddingInlineStart: `${8 + depth * 12}px` }}
                aria-current={active ? "location" : undefined}
                aria-label={t("jumpToResponseSection", { title: item.label })}
                onMouseEnter={() => setHoveredHeadingIndex(index)}
                onFocus={() => setHoveredHeadingIndex(index)}
                onClick={() => scrollToHeading(index)}
              >
                {item.label}
              </button>
            );
          })}
        </div>
      </HoverCardContent>
    </HoverCard>
  );
}

export const ChatResponseOutlineRail = ChatResponseOutlineRailComponent;
