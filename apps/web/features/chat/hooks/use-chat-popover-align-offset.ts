import * as React from "react";

/** Gap kept between a composer popover and the viewport edge; matches their `collisionPadding`. */
const VIEWPORT_PADDING = 8;

/**
 * Composer popovers keep their side fixed (`avoidCollisions={false}`), which also switches off
 * Radix's horizontal shift: on a phone a panel aligned to its trigger can start outside the
 * viewport. This returns the `alignOffset` that slides the panel back in without letting it flip.
 */
export function useChatPopoverAlignOffset(align: "start" | "end") {
  const triggerRef = React.useRef<HTMLButtonElement>(null);
  const [content, setContent] = React.useState<HTMLDivElement | null>(null);
  const [alignOffset, setAlignOffset] = React.useState(0);

  React.useLayoutEffect(() => {
    if (!content) return;
    const update = () => {
      const trigger = triggerRef.current;
      if (!trigger) return;
      const anchor = trigger.getBoundingClientRect();
      // offsetWidth ignores the open animation's scale.
      const width = content.offsetWidth;
      const left = align === "end" ? anchor.right - width : anchor.left;
      const maxLeft = Math.max(document.documentElement.clientWidth - VIEWPORT_PADDING - width, VIEWPORT_PADDING);
      const shift = Math.round(Math.min(Math.max(left, VIEWPORT_PADDING), maxLeft) - left);
      // The alignment axis points away from the aligned edge, so "end" counts leftwards.
      setAlignOffset(align === "end" ? -shift : shift);
    };
    update();
    window.addEventListener("resize", update);
    return () => window.removeEventListener("resize", update);
  }, [content, align]);

  return { triggerRef, contentRef: setContent, alignOffset };
}
