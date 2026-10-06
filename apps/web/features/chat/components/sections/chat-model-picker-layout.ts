import { isFiniteNumber } from "@/shared/lib/type-guards";

export function resolveDesktopMenuListMaxHeight(maxPanelHeight: number, verticalChrome: number): number {
  return Math.max(0, maxPanelHeight - verticalChrome);
}

/**
 * Vendor/model list max-height for the desktop picker.
 *
 * Must be derived from the trigger's free space (and a hard viewport cap),
 * never from the floating panel's current top. Using the floating top creates
 * a loop: collision-shift pushes the panel off-screen → maxHeight expands to
 * "full remaining viewport" → the top edge stays clipped while the inner list
 * still scrolls.
 */
export function resolveDesktopModelMenuListMaxHeight(input: {
  viewportTop: number;
  viewportBottom: number;
  triggerTop?: number | null;
  triggerBottom?: number | null;
  sideOffset: number;
  verticalChrome: number;
}): number {
  const viewportHeight = Math.max(0, input.viewportBottom - input.viewportTop);

  const { triggerTop, triggerBottom } = input;
  const hasTrigger = isFiniteNumber(triggerTop) && isFiniteNumber(triggerBottom);

  const spaceBelowTrigger = hasTrigger
    ? input.viewportBottom - triggerBottom - input.sideOffset
    : viewportHeight;
  const spaceAboveTrigger = hasTrigger
    ? triggerTop - input.viewportTop - input.sideOffset
    : viewportHeight;
  const preferredSideSpace = Math.max(spaceBelowTrigger, spaceAboveTrigger, 0);
  const maxPanelHeight = Math.min(viewportHeight, preferredSideSpace || viewportHeight);

  return resolveDesktopMenuListMaxHeight(maxPanelHeight, input.verticalChrome);
}

/**
 * Maps a rect measured inside the popover onto the popover's settled layout box.
 *
 * Radix opens the popover with a scale and a slide, so rects measured while that runs are
 * transformed and positions derived from them jump once the animation ends. The popper wrapper is
 * not animated and sits exactly where the content's untransformed box ends up.
 */
export function createPopoverLayoutRectReader(inside: HTMLElement): (element: Element) => DOMRect {
  const content = inside.closest<HTMLElement>("[data-slot=popover-content]");
  const wrapper = content?.parentElement;
  const contentRect = content?.getBoundingClientRect();
  const scale = content && contentRect && content.offsetWidth > 0 ? contentRect.width / content.offsetWidth : 0;
  if (!wrapper || !contentRect || scale <= 0) {
    return (element) => element.getBoundingClientRect();
  }
  const wrapperRect = wrapper.getBoundingClientRect();
  return (element) => {
    const rect = element.getBoundingClientRect();
    return new DOMRect(
      wrapperRect.left + (rect.left - contentRect.left) / scale,
      wrapperRect.top + (rect.top - contentRect.top) / scale,
      rect.width / scale,
      rect.height / scale,
    );
  };
}

/**
 * Top of `element` inside the positioned `container`, from layout offsets.
 *
 * It ignores transforms (content sliding in, a card gliding) and accounts for scrolled boxes in
 * between, so it is where the element ends up rather than where an animation shows it right now.
 * NaN when `container` is not one of its offset parents.
 */
export function layoutOffsetTop(element: HTMLElement, container: HTMLElement): number {
  let top = 0;
  let node: HTMLElement | null = element;
  while (node && node !== container) {
    const parent: HTMLElement | null = node.offsetParent instanceof HTMLElement ? node.offsetParent : null;
    top += node.offsetTop;
    for (let box: HTMLElement | null = node.parentElement; box && box !== parent; box = box.parentElement) {
      top -= box.scrollTop;
    }
    node = parent;
  }
  return node === container ? top + container.clientTop : Number.NaN;
}
