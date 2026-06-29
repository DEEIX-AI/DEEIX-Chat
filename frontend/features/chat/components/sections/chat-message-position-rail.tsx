"use client";

import * as React from "react";
import { createPortal } from "react-dom";

import {
  useMessageScroller,
  useMessageScrollerVisibility,
} from "@/components/ui/message-scroller";
import type { ChatAreaMessage } from "@/features/chat/types/messages";
import { cn } from "@/lib/utils";

const QUESTION_PREVIEW_MAX_LENGTH = 240;
const ANSWER_PREVIEW_MAX_LENGTH = 420;
const PREVIEW_EDGE_MARGIN_PX = 12;
const PREVIEW_ESTIMATED_HEIGHT_PX = 96;
const PREVIEW_OFFSET_X_PX = 8;

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

function ChatMessagePositionPreview({
  item,
  position,
  previewRef,
  top,
}: {
  item: TurnPreviewItem;
  position: PreviewPosition;
  previewRef: React.RefObject<HTMLDivElement | null>;
  top: number;
}) {
  return createPortal(
    <div
      ref={previewRef}
      className="pointer-events-none fixed z-[9999] w-[min(22rem,calc(100vw-5rem))] -translate-y-1/2"
      style={{ left: position.left, maxHeight: position.maxHeight, top }}
      data-screenshot-exclude="true"
    >
      <div className="max-h-full scroll-fade-y scroll-fade-12 overflow-y-auto rounded-lg bg-sidebar-accent px-3 py-2 text-left text-foreground [scrollbar-width:none] [&::-webkit-scrollbar]:hidden">
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
    </div>,
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
  const { currentAnchorId, visibleMessageIds } = useMessageScrollerVisibility();
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
      item.messageIDs.some((messageID) => messageID === currentAnchorId || visibleIDs.has(messageID)),
    [currentAnchorId, visibleIDs],
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

  const currentIndex = items.findIndex(turnIsActive);
  const hoveredIndex = hoveredID ? items.findIndex((item) => item.id === hoveredID) : -1;
  const activeIndex = hoveredIndex >= 0 ? hoveredIndex : currentIndex >= 0 ? currentIndex : items.length - 1;
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
    previewItem && previewPosition && previewTop !== null && typeof document !== "undefined" ? (
      <ChatMessagePositionPreview
        item={previewItem}
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
          const focused = distance === 0;
          const lineClassName = cn(
            "h-0.5 rounded-full bg-current opacity-35 transition-[opacity,width]",
            focused && "w-6 text-foreground opacity-100",
            distance === 1 && "w-4 opacity-70",
            distance === 2 && "w-3.5 opacity-50",
            distance > 2 && (index === 0 || index === items.length - 1 ? "w-2.5" : "w-3"),
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
                className="flex h-1.5 w-6 items-center justify-center rounded-sm"
                onMouseEnter={(event) => activatePreview(item.id, event.currentTarget)}
                onFocus={(event) => activatePreview(item.id, event.currentTarget)}
                onClick={() => scrollToMessage(item.id, { align: "start", behavior: "smooth", scrollMargin: 16 })}
                aria-label={item.question}
                tabIndex={-1}
              >
                <span className={lineClassName} />
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
