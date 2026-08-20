"use client";

import { useTranslations } from "next-intl";
import * as React from "react";

import { HoverCard, HoverCardContent, HoverCardTrigger } from "@/components/ui/hover-card";
import { useMessageScrollerVisibility } from "@/components/ui/message-scroller";
import { cn } from "@/lib/utils";

const MIN_OUTLINE_HEADINGS = 2;
const OUTLINE_GUIDE_RATIO = 0.28;
const OUTLINE_SCROLL_MARGIN_PX = 24;
const OUTLINE_MIN_RESPONSE_HEIGHT_PX = 360;
const OUTLINE_MAX_RESPONSE_HEIGHT_THRESHOLD_PX = 560;
const OUTLINE_MARKER_WIDTH_REM = [1, 0.75, 0.5] as const;
const OUTLINE_CLOSE_DELAY_MS = 180;
const OUTLINE_VISIBILITY_MARGIN_PX = 4;
const ASSISTANT_MESSAGE_SELECTOR = '[data-chat-message-role="assistant"]';
const ASSISTANT_HEADING_SELECTOR =
  "[data-chat-assistant-content] [data-chat-markdown-scope] :is(h1, h2, h3)";

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

function keepItemVisible(viewport: HTMLElement, item: HTMLElement) {
  const viewportRect = viewport.getBoundingClientRect();
  const itemRect = item.getBoundingClientRect();
  const offsetBefore = itemRect.top - viewportRect.top - OUTLINE_VISIBILITY_MARGIN_PX;
  const offsetAfter = itemRect.bottom - viewportRect.bottom + OUTLINE_VISIBILITY_MARGIN_PX;
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
  currentAnchorID: string,
  visibleMessageIDs: Iterable<string>,
): HTMLElement | null {
  const candidateIDs = Array.from(
    new Set([currentAnchorID, ...visibleMessageIDs].filter(Boolean)),
  );
  if (candidateIDs.length === 0) {
    return null;
  }

  const viewportRect = viewport.getBoundingClientRect();
  const guideY = viewportRect.top + viewportRect.height * OUTLINE_GUIDE_RATIO;
  const selector = candidateIDs
    .map(
      (messageID) =>
        `${ASSISTANT_MESSAGE_SELECTOR}[data-chat-message-id="${CSS.escape(messageID)}"]`,
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

  const anchorCandidate = candidates.find(
    (candidate) => candidate.element.dataset.chatMessageId === currentAnchorID,
  );
  if (anchorCandidate) {
    return anchorCandidate.element;
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
  const { currentAnchorId, visibleMessageIds } = useMessageScrollerVisibility();
  const [headings, setHeadings] = React.useState<ResponseOutlineHeading[]>([]);
  const [activeHeadingIndex, setActiveHeadingIndex] = React.useState(0);
  const [hoveredHeadingIndex, setHoveredHeadingIndex] = React.useState<number | null>(null);
  const [outlineOpen, setOutlineOpen] = React.useState(false);
  const [railOverflowing, setRailOverflowing] = React.useState(false);
  const headingElementsRef = React.useRef<HTMLElement[]>([]);
  const headingSignatureRef = React.useRef("");
  const scanFrameRef = React.useRef<number | null>(null);
  const railViewportRef = React.useRef<HTMLDivElement | null>(null);
  const railContentRef = React.useRef<HTMLDivElement | null>(null);
  const railItemRefs = React.useRef(new Map<number, HTMLButtonElement>());
  const menuViewportRef = React.useRef<HTMLDivElement | null>(null);
  const menuItemRefs = React.useRef(new Map<number, HTMLButtonElement>());
  const visibilityRef = React.useRef({ currentAnchorId, visibleMessageIds });

  React.useLayoutEffect(() => {
    visibilityRef.current = { currentAnchorId, visibleMessageIds };
  }, [currentAnchorId, visibleMessageIds]);

  const clearOutline = React.useCallback(() => {
    headingElementsRef.current = [];
    setOutlineOpen(false);
    setHoveredHeadingIndex(null);
    if (headingSignatureRef.current) {
      headingSignatureRef.current = "";
      setHeadings([]);
    }
    setActiveHeadingIndex(0);
  }, []);

  const updateActiveHeading = React.useCallback(() => {
    const viewport = boundaryRef.current;
    const elements = headingElementsRef.current;
    if (!viewport || elements.length === 0) {
      setActiveHeadingIndex(0);
      return;
    }

    const viewportRect = viewport.getBoundingClientRect();
    const guideY = viewportRect.top + viewportRect.height * OUTLINE_GUIDE_RATIO;
    let nextIndex = 0;
    for (let index = 0; index < elements.length; index += 1) {
      if (elements[index]?.getBoundingClientRect().top <= guideY) {
        nextIndex = index;
        continue;
      }
      break;
    }
    setActiveHeadingIndex((current) => (current === nextIndex ? current : nextIndex));
  }, [boundaryRef]);

  const scanOutline = React.useCallback(() => {
    scanFrameRef.current = null;
    const viewport = boundaryRef.current;
    if (!viewport || disabled) {
      clearOutline();
      return;
    }

    const visibility = visibilityRef.current;
    const message = resolveActiveAssistantMessage(
      viewport,
      visibility.currentAnchorId,
      visibility.visibleMessageIds,
    );
    if (!message) {
      clearOutline();
      return;
    }

    const viewportRect = viewport.getBoundingClientRect();
    const messageRect = message.getBoundingClientRect();
    const minimumResponseHeight = Math.max(
      OUTLINE_MIN_RESPONSE_HEIGHT_PX,
      Math.min(OUTLINE_MAX_RESPONSE_HEIGHT_THRESHOLD_PX, viewportRect.height * 0.72),
    );
    const elements = Array.from(
      message.querySelectorAll<HTMLElement>(ASSISTANT_HEADING_SELECTOR),
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
      nextHeadings.length >= MIN_OUTLINE_HEADINGS && messageRect.height >= minimumResponseHeight;
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

  React.useLayoutEffect(() => {
    scheduleOutlineScan();
  });

  React.useEffect(() => {
    const viewport = boundaryRef.current;
    if (!viewport) {
      return;
    }

    viewport.addEventListener("scroll", scheduleOutlineScan, { passive: true });
    const resizeObserver =
      typeof ResizeObserver === "undefined" ? null : new ResizeObserver(scheduleOutlineScan);
    resizeObserver?.observe(viewport);
    return () => {
      viewport.removeEventListener("scroll", scheduleOutlineScan);
      resizeObserver?.disconnect();
      if (scanFrameRef.current !== null) {
        window.cancelAnimationFrame(scanFrameRef.current);
        scanFrameRef.current = null;
      }
    };
  }, [boundaryRef, scheduleOutlineScan]);

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
      const viewport = boundaryRef.current;
      const heading = headingElementsRef.current[index];
      if (!viewport || !heading) {
        return;
      }
      const viewportRect = viewport.getBoundingClientRect();
      const headingRect = heading.getBoundingClientRect();
      const top =
        viewport.scrollTop + headingRect.top - viewportRect.top - OUTLINE_SCROLL_MARGIN_PX;
      const reducedMotion = window.matchMedia("(prefers-reduced-motion: reduce)").matches;
      viewport.scrollTo({
        top: Math.max(0, top),
        behavior: reducedMotion ? "auto" : "smooth",
      });
      setActiveHeadingIndex(index);
    },
    [boundaryRef],
  );

  if (disabled || headings.length < MIN_OUTLINE_HEADINGS) {
    return null;
  }

  const minimumLevel = Math.min(...headings.map((item) => item.level));

  return (
    <HoverCard
      open={outlineOpen}
      openDelay={0}
      closeDelay={OUTLINE_CLOSE_DELAY_MS}
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
                    style={{ width: `${OUTLINE_MARKER_WIDTH_REM[depth] ?? OUTLINE_MARKER_WIDTH_REM[2]}rem` }}
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
        className="z-[9999] flex max-h-[min(68vh,34rem)] w-72 flex-col overflow-hidden rounded-lg border-0 bg-sidebar-accent p-1 text-foreground shadow-none"
        role="navigation"
        aria-label={t("responseOutline")}
        data-screenshot-exclude="true"
      >
        <div className="shrink-0 px-2 pb-1 pt-1 text-[11px] font-medium text-muted-foreground">
          {t("responseOutline")}
        </div>
        <div
          ref={menuViewportRef}
          className="scroll-fade-y scroll-fade-8 flex min-h-0 flex-1 flex-col gap-0.5 overflow-y-auto overscroll-contain [scrollbar-width:none] [&::-webkit-scrollbar]:hidden"
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
                  "block w-full shrink-0 truncate rounded-md py-1 pr-2 text-left text-xs leading-5 text-muted-foreground transition-colors hover:bg-foreground/[0.04] hover:text-foreground focus-visible:bg-foreground/[0.04] focus-visible:text-foreground focus-visible:outline-none",
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
