"use client";

import * as React from "react";
import {
  MessageScroller as MessageScrollerPrimitive,
  useMessageScroller,
  useMessageScrollerScrollable,
  useMessageScrollerVisibility,
} from "@shadcn/react/message-scroller";

import { Button } from "@/components/ui/button";
import { cn } from "@/lib/utils";
import { ArrowDownIcon } from "lucide-react";

const MESSAGE_SCROLLER_EDGE_THRESHOLD_PX = 48;

// The primitive follows the bottom from a ResizeObserver, which fires after
// paint: every burst of streamed text paints one frame with the new content
// below the fold before the follow pulls it back, and the message footer
// visibly twitches. Content mutations are observed here as well so the
// viewport is pinned in the same frame, before that paint. The primitive's
// own follow stays as the fallback for changes that are not DOM mutations.
const FollowContext = React.createContext<{ autoScroll: boolean; threshold: number }>({
  autoScroll: false,
  threshold: MESSAGE_SCROLLER_EDGE_THRESHOLD_PX,
});

function MessageScrollerProvider({
  autoScroll = true,
  scrollEdgeThreshold = MESSAGE_SCROLLER_EDGE_THRESHOLD_PX,
  ...props
}: React.ComponentProps<typeof MessageScrollerPrimitive.Provider>) {
  const follow = React.useMemo(() => ({ autoScroll, threshold: scrollEdgeThreshold }), [autoScroll, scrollEdgeThreshold]);
  return (
    <FollowContext.Provider value={follow}>
      <MessageScrollerPrimitive.Provider
        autoScroll={autoScroll}
        scrollEdgeThreshold={scrollEdgeThreshold}
        {...props}
      />
    </FollowContext.Provider>
  );
}

function useSameFramePinToBottom(content: HTMLElement | null) {
  const { autoScroll, threshold } = React.useContext(FollowContext);
  React.useEffect(() => {
    const viewport = content?.closest<HTMLElement>('[data-slot="message-scroller-viewport"]');
    if (!autoScroll || !content || !viewport || typeof MutationObserver === "undefined") {
      return;
    }
    const gap = () => viewport.scrollHeight - viewport.scrollTop - viewport.clientHeight;
    // Same "at the bottom" rule as the primitive, so both agree on when to follow.
    let atBottom = gap() <= threshold;
    const onScroll = () => {
      atBottom = gap() <= threshold;
    };
    const observer = new MutationObserver(() => {
      if (!atBottom) {
        return;
      }
      const target = viewport.scrollHeight - viewport.clientHeight;
      if (viewport.scrollTop < target) {
        viewport.scrollTop = target;
      }
    });
    observer.observe(content, { childList: true, characterData: true, subtree: true });
    viewport.addEventListener("scroll", onScroll, { passive: true });
    return () => {
      observer.disconnect();
      viewport.removeEventListener("scroll", onScroll);
    };
  }, [autoScroll, content, threshold]);
}

function MessageScroller({
  className,
  children,
  ...props
}: React.ComponentProps<typeof MessageScrollerPrimitive.Root>) {
  const scrollable = useMessageScrollerScrollable();
  return (
    <MessageScrollerPrimitive.Root
      data-slot="message-scroller"
      className={cn(
        "group/message-scroller relative flex size-full min-h-0 flex-col overflow-hidden",
        className,
      )}
      {...props}
    >
      {children}
      <div
        aria-hidden="true"
        className={cn(
          "pointer-events-none absolute inset-x-0 top-0 z-10 h-12 bg-gradient-to-b from-background from-0% to-transparent to-100% opacity-0 transition-opacity duration-150",
          scrollable.start && "opacity-100",
        )}
      />
      <div
        aria-hidden="true"
        className={cn(
          "pointer-events-none absolute inset-x-0 bottom-0 z-10 h-12 bg-gradient-to-t from-background from-0% to-transparent to-100% opacity-0 transition-opacity duration-150",
          scrollable.end && "opacity-100",
        )}
      />
    </MessageScrollerPrimitive.Root>
  );
}

function MessageScrollerViewport({
  className,
  // Browser scroll anchoring can move the conversation when asynchronously
  // rendered content changes height. Only compensate for actual history prepends.
  preserveScrollOnPrepend = true,
  ...props
}: React.ComponentProps<typeof MessageScrollerPrimitive.Viewport>) {
  return (
    <MessageScrollerPrimitive.Viewport
      data-slot="message-scroller-viewport"
      className={cn(
        "size-full min-h-0 min-w-0 scrollbar-none overflow-y-auto overscroll-contain outline-none contain-content [overflow-anchor:none]",
        className,
      )}
      preserveScrollOnPrepend={preserveScrollOnPrepend}
      {...props}
    />
  );
}

function MessageScrollerContent({
  className,
  ref,
  ...props
}: React.ComponentProps<typeof MessageScrollerPrimitive.Content>) {
  const [element, setElement] = React.useState<HTMLElement | null>(null);
  useSameFramePinToBottom(element);
  const mergedRef = React.useCallback(
    (node: HTMLDivElement | null) => {
      setElement(node);
      if (typeof ref === "function") {
        ref(node);
      } else if (ref) {
        ref.current = node;
      }
    },
    [ref],
  );
  return (
    <MessageScrollerPrimitive.Content
      ref={mergedRef}
      data-slot="message-scroller-content"
      className={cn("flex h-max min-h-full flex-col gap-6", className)}
      {...props}
    />
  );
}

type MessageScrollerItemProps = Omit<
  React.ComponentProps<typeof MessageScrollerPrimitive.Item>,
  "scrollAnchor"
>;

function MessageScrollerItem({
  className,
  ...props
}: MessageScrollerItemProps) {
  return (
    <MessageScrollerPrimitive.Item
      data-slot="message-scroller-item"
      className={cn("min-w-0 shrink-0", className)}
      {...props}
    />
  );
}

function MessageScrollerButton({
  direction = "end",
  className,
  children,
  render,
  variant = "secondary",
  size = "icon-sm",
  ...props
}: React.ComponentProps<typeof MessageScrollerPrimitive.Button> &
  Pick<React.ComponentProps<typeof Button>, "variant" | "size">) {
  return (
    <MessageScrollerPrimitive.Button
      data-slot="message-scroller-button"
      data-direction={direction}
      data-variant={variant}
      data-size={size}
      direction={direction}
      className={cn(
        "absolute inset-s-1/2 -translate-x-1/2 rounded-full border-border bg-background text-foreground transition-[translate,scale,opacity] duration-200 hover:bg-muted hover:text-foreground data-[active=false]:pointer-events-none data-[active=false]:scale-95 data-[active=false]:opacity-0 data-[active=false]:duration-400 data-[active=false]:ease-[cubic-bezier(0.7,0,0.84,0)] data-[active=true]:translate-y-0 data-[active=true]:scale-100 data-[active=true]:opacity-100 data-[active=true]:ease-[cubic-bezier(0.23,1,0.32,1)] data-[direction=end]:bottom-4 data-[direction=end]:data-[active=false]:translate-y-full data-[direction=start]:top-4 data-[direction=start]:data-[active=false]:-translate-y-full rtl:translate-x-1/2 data-[direction=start]:[&_svg]:rotate-180",
        className,
      )}
      render={render ?? <Button variant={variant} size={size} />}
      {...props}
    >
      {children ?? (
        <>
          <ArrowDownIcon />
          <span className="sr-only">
            {direction === "end" ? "Scroll to end" : "Scroll to start"}
          </span>
        </>
      )}
    </MessageScrollerPrimitive.Button>
  );
}

export {
  MessageScrollerProvider,
  MessageScroller,
  MessageScrollerViewport,
  MessageScrollerContent,
  MessageScrollerItem,
  MessageScrollerButton,
  useMessageScroller,
  useMessageScrollerScrollable,
  useMessageScrollerVisibility,
};
