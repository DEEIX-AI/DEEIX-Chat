"use client";

import { AnimatePresence, motion } from "motion/react";
import * as React from "react";
import { createPortal } from "react-dom";

import {
  useMessageScroller,
  useMessageScrollerScrollable,
  useMessageScrollerVisibility,
} from "@/components/ui/message-scroller";
import type { ChatAreaMessage } from "@/features/chat/types/messages";
import { cn } from "@/lib/utils";
import { useScrollFadeFallbackRef } from "@/shared/hooks/use-scroll-fade-fallback-ref";

const QUESTION_PREVIEW_MAX_LENGTH = 240;
const ANSWER_PREVIEW_MAX_LENGTH = 420;
const PREVIEW_EDGE_MARGIN_PX = 12;
const PREVIEW_ESTIMATED_HEIGHT_PX = 96;
const PREVIEW_OFFSET_X_PX = 8;
const RAIL_LINE_BASE_WIDTH_REM = 0.6;
const RAIL_LINE_ACTIVE_WIDTH_MULTIPLIER = 2;
const RAIL_LINE_ADJACENT_WIDTH_MULTIPLIER = 1.5;

type TurnPreviewItem = {
  answer: string;
  id: string;
  messageIDs: string[];
  question: string;
};

type PreviewPosition = {
  boundaryBottom: number;
  boundaryTop: number;
  left: number;
  maxHeight: number;
  top: number;
};

function clamp(value: number, min: number, max: number) {
  return Math.min(Math.max(value, min), max);
}

export function chatMessageScrollerID(item: ChatAreaMessage) {
  return item.key;
}

function messagePreviewText(content: string, maxLength: number) {
  return content
    .slice(0, maxLength)
    .replace(/```[\s\S]*?```/g, "")
    .replace(/[#*_`>\-[\]()]/g, " ")
    .replace(/\s+/g, " ")
    .trim();
}

function resolvePreviewPosition({
  boundary,
  previewHeight,
}: {
  boundary: PreviewPosition;
  previewHeight: number;
}) {
  const halfHeight = previewHeight / 2;
  const minTop = boundary.boundaryTop + halfHeight;
  const maxTop = Math.max(minTop, boundary.boundaryBottom - halfHeight);
  return clamp(boundary.top, minTop, maxTop);
}

function resolveRailLineWidthRem(distance: number, distributed: boolean) {
  if (!distributed || distance >= 2) {
    return RAIL_LINE_BASE_WIDTH_REM;
  }

  return (
    RAIL_LINE_BASE_WIDTH_REM *
    (distance === 0 ? RAIL_LINE_ACTIVE_WIDTH_MULTIPLIER : RAIL_LINE_ADJACENT_WIDTH_MULTIPLIER)
  );
}

function ChatMessagePositionPreview({
  item,
  position,
  previewRef,
  top,
}: {
  item: TurnPreviewItem | null;
  position: PreviewPosition | null;
  previewRef: React.RefObject<HTMLDivElement | null>;
  top: number | null;
}) {
  const scrollFadeRef = useScrollFadeFallbackRef<HTMLDivElement>();

  return createPortal(
    <AnimatePresence initial={false}>
      {item && position && top !== null ? (
        <motion.div
          ref={previewRef}
          key="chat-message-position-preview"
          className="pointer-events-none fixed z-30 w-[min(22rem,calc(100vw-5rem))] -translate-y-1/2"
          style={{ left: position.left, maxHeight: position.maxHeight, top }}
          initial={{ opacity: 0 }}
          animate={{ opacity: 1 }}
          exit={{ opacity: 0 }}
          transition={{ duration: 0.15, ease: "easeOut" }}
          data-screenshot-exclude="true"
        >
          <div
            ref={scrollFadeRef}
            className="max-h-full scroll-fade-y scroll-fade-12 overflow-y-auto rounded-lg bg-sidebar-accent px-3 py-2 text-left text-foreground [scrollbar-width:none] [&::-webkit-scrollbar]:hidden"
          >
            <span
              className="block text-sm font-medium leading-5 text-foreground"
              style={{
                overflow: "hidden",
                textOverflow: "ellipsis",
                whiteSpace: "nowrap",
              }}
            >
              {item.question}
            </span>
            {item.answer ? (
              <span
                className="mt-1 block text-xs leading-5 text-muted-foreground"
                style={{
                  display: "-webkit-box",
                  maxHeight: "3.75rem",
                  overflow: "hidden",
                  WebkitBoxOrient: "vertical",
                  WebkitLineClamp: 3,
                }}
              >
                {item.answer}
              </span>
            ) : null}
          </div>
        </motion.div>
      ) : null}
    </AnimatePresence>,
    document.body,
  );
}

function ChatMessagePositionRailComponent({
  boundaryRef,
  messages,
}: {
  boundaryRef: React.RefObject<HTMLDivElement | null>;
  messages: ChatAreaMessage[];
}) {
  const { scrollToMessage } = useMessageScroller();
  const { end: canScrollToEnd } = useMessageScrollerScrollable();
  const { visibleMessageIds } = useMessageScrollerVisibility();
  const [hoveredID, setHoveredID] = React.useState<string | null>(null);
  const [previewPosition, setPreviewPosition] = React.useState<PreviewPosition | null>(null);
  const [previewHeight, setPreviewHeight] = React.useState(PREVIEW_ESTIMATED_HEIGHT_PX);
  const itemRefs = React.useRef(new Map<string, HTMLButtonElement>());
  const previewRef = React.useRef<HTMLDivElement | null>(null);
  const railViewportRef = React.useRef<HTMLDivElement | null>(null);
  const railContentRef = React.useRef<HTMLDivElement | null>(null);
  const centerFrameRef = React.useRef<number | null>(null);
  const [railOverflowing, setRailOverflowing] = React.useState(false);
  const items = React.useMemo(
    () => {
      const result: TurnPreviewItem[] = [];
      let current: TurnPreviewItem | null = null;

      const flushCurrent = () => {
        if (current?.question) {
          result.push(current);
        }
        current = null;
      };

      for (const item of messages) {
        if (item.isPending) {
          continue;
        }

        const messageID = chatMessageScrollerID(item);
        if (item.role === "user") {
          flushCurrent();
          const question = messagePreviewText(item.content, QUESTION_PREVIEW_MAX_LENGTH);
          if (!question) {
            continue;
          }
          current = {
            id: messageID,
            messageIDs: [messageID],
            question,
            answer: "",
          };
          continue;
        }

        if (!current) {
          continue;
        }

        current.messageIDs.push(messageID);
        if (item.role === "assistant" && !current.answer) {
          current.answer = messagePreviewText(item.content, ANSWER_PREVIEW_MAX_LENGTH);
        }
      }

      flushCurrent();
      return result;
    },
    [messages],
  );
  const visibleIDs = React.useMemo(() => new Set(visibleMessageIds), [visibleMessageIds]);
  const turnIsActive = React.useCallback(
    (item: TurnPreviewItem) =>
      item.messageIDs.some((messageID) => visibleIDs.has(messageID)),
    [visibleIDs],
  );
  const activatePreview = React.useCallback((id: string, target: HTMLElement) => {
    const targetRect = target.getBoundingClientRect();
    const boundaryRect = boundaryRef.current?.getBoundingClientRect();
    const viewportHeight = window.innerHeight || document.documentElement.clientHeight;
    const boundaryTop = (boundaryRect?.top ?? 0) + PREVIEW_EDGE_MARGIN_PX;
    const boundaryBottom = (boundaryRect?.bottom ?? viewportHeight) - PREVIEW_EDGE_MARGIN_PX;
    setHoveredID(id);
    setPreviewPosition({
      boundaryBottom,
      boundaryTop,
      left: targetRect.right + PREVIEW_OFFSET_X_PX,
      maxHeight: Math.max(0, boundaryBottom - boundaryTop),
      top: targetRect.top + targetRect.height / 2,
    });
  }, [boundaryRef]);
  const clearPreview = React.useCallback(() => {
    setHoveredID(null);
    setPreviewPosition(null);
  }, []);

  const visibleIndex = items.findIndex(turnIsActive);
  const currentIndex = !canScrollToEnd && items.length > 0 ? items.length - 1 : visibleIndex;
  const hoveredIndex = hoveredID ? items.findIndex((item) => item.id === hoveredID) : -1;
  const activeIndex = hoveredIndex >= 0 ? hoveredIndex : currentIndex >= 0 ? currentIndex : items.length - 1;
  const railLineDistributionActive = hoveredIndex >= 0;
  const currentItem = items[currentIndex >= 0 ? currentIndex : items.length - 1] ?? null;
  const currentItemID = currentItem?.id ?? "";
  const previewItem = hoveredID ? items.find((item) => item.id === hoveredID) : null;

  const centerCurrentRailItem = React.useCallback(() => {
    const railViewport = railViewportRef.current;
    const activeElement = currentItemID ? itemRefs.current.get(currentItemID) : null;
    if (!railViewport || !activeElement) {
      return;
    }

    const viewportRect = railViewport.getBoundingClientRect();
    const activeRect = activeElement.getBoundingClientRect();
    const offset =
      activeRect.top - viewportRect.top - (viewportRect.height - activeRect.height) / 2;
    const targetTop = railViewport.scrollTop + offset;
    railViewport.scrollTo({ top: Math.max(0, targetTop), behavior: "auto" });
  }, [currentItemID]);

  const scheduleCenterCurrentRailItem = React.useCallback(() => {
    if (centerFrameRef.current !== null) {
      window.cancelAnimationFrame(centerFrameRef.current);
    }
    centerFrameRef.current = window.requestAnimationFrame(() => {
      centerFrameRef.current = null;
      centerCurrentRailItem();
    });
  }, [centerCurrentRailItem]);

  React.useLayoutEffect(() => {
    centerCurrentRailItem();
    scheduleCenterCurrentRailItem();
    return () => {
      if (centerFrameRef.current !== null) {
        window.cancelAnimationFrame(centerFrameRef.current);
        centerFrameRef.current = null;
      }
    };
  }, [centerCurrentRailItem, items.length, scheduleCenterCurrentRailItem]);

  React.useLayoutEffect(() => {
    const railViewport = railViewportRef.current;
    const railContent = railContentRef.current;
    if (!railViewport || !railContent || typeof ResizeObserver === "undefined") {
      return;
    }

    const syncRailLayout = () => {
      setRailOverflowing(railContent.scrollHeight > railViewport.clientHeight + 1);
      scheduleCenterCurrentRailItem();
    };
    syncRailLayout();

    const observer = new ResizeObserver(syncRailLayout);
    observer.observe(railViewport);
    observer.observe(railContent);
    return () => observer.disconnect();
  }, [items.length, scheduleCenterCurrentRailItem]);

  React.useLayoutEffect(() => {
    const height = previewRef.current?.getBoundingClientRect().height;
    if (!height) {
      return;
    }
    setPreviewHeight((previous) => (Math.abs(previous - height) < 0.5 ? previous : height));
  }, [previewItem?.id, previewPosition?.maxHeight]);

  if (items.length <= 1) {
    return null;
  }

  const previewTop = previewPosition ? resolvePreviewPosition({ boundary: previewPosition, previewHeight }) : null;
  const preview =
    typeof document !== "undefined" ? (
      <ChatMessagePositionPreview
        item={previewItem ?? null}
        position={previewPosition}
        previewRef={previewRef}
        top={previewTop}
      />
    ) : null;

  const rail = (
    <div
      ref={railViewportRef}
      className="pointer-events-auto h-full w-6 overflow-y-auto overscroll-contain text-muted-foreground/55 [scrollbar-width:none] [&::-webkit-scrollbar]:hidden"
      role="navigation"
      aria-label="Message position"
      onScroll={clearPreview}
    >
      <div
        ref={railContentRef}
        className={cn(
          "flex min-h-full flex-col items-center gap-1 px-1 py-1",
          !railOverflowing && "justify-center",
        )}
      >
        {items.map((item, index) => {
          const distance = Math.abs(index - activeIndex);
          const lineWidthRem = resolveRailLineWidthRem(distance, railLineDistributionActive);
          const lineClassName = cn(
            "h-0.5 rounded-full bg-current opacity-35 transition-[opacity,width]",
            distance === 0 && "text-foreground opacity-100",
            distance === 1 && "opacity-70",
            distance === 2 && "opacity-50",
          );
          return (
            <div key={item.id} className="relative flex w-6 justify-center">
              <button
                ref={(node) => {
                  if (node) {
                    itemRefs.current.set(item.id, node);
                    return;
                  }
                  itemRefs.current.delete(item.id);
                }}
                type="button"
                className="flex h-1.5 w-6 items-center justify-start rounded-sm"
                onMouseEnter={(event) => activatePreview(item.id, event.currentTarget)}
                onFocus={(event) => activatePreview(item.id, event.currentTarget)}
                onClick={() => scrollToMessage(item.id, { align: "start", behavior: "smooth", scrollMargin: 16 })}
                aria-label={item.question}
                tabIndex={-1}
              >
                <span className={lineClassName} style={{ width: `${lineWidthRem}rem` }} />
              </button>
            </div>
          );
        })}
      </div>
    </div>
  );

  return (
    <div
      className="pointer-events-none absolute bottom-3 left-2 top-3 z-30 hidden w-6 lg:block"
      data-screenshot-exclude="true"
      onMouseLeave={clearPreview}
    >
      {rail}
      {preview}
    </div>
  );
}

export const ChatMessagePositionRail = React.memo(ChatMessagePositionRailComponent, (previous, next) => {
  if (previous.boundaryRef !== next.boundaryRef) {
    return false;
  }
  if (previous.messages.length !== next.messages.length) {
    return false;
  }

  return previous.messages.every((message, index) => {
    const nextMessage = next.messages[index];
    if (!nextMessage) {
      return false;
    }
    const messageIsLive = message.isPending || message.isStreaming || nextMessage.isPending || nextMessage.isStreaming;
    return (
      message.key === nextMessage.key &&
      message.role === nextMessage.role &&
      message.publicID === nextMessage.publicID &&
      message.isPending === nextMessage.isPending &&
      message.isStreaming === nextMessage.isStreaming &&
      (messageIsLive || message.content === nextMessage.content)
    );
  });
});
ChatMessagePositionRail.displayName = "ChatMessagePositionRail";
