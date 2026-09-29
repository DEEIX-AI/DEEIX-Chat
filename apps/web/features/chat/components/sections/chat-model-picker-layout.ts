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
