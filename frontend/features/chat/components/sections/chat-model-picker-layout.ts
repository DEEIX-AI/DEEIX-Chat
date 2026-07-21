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

  const hasTrigger =
    typeof input.triggerTop === "number" &&
    Number.isFinite(input.triggerTop) &&
    typeof input.triggerBottom === "number" &&
    Number.isFinite(input.triggerBottom);

  const spaceBelowTrigger = hasTrigger
    ? input.viewportBottom - (input.triggerBottom as number) - input.sideOffset
    : viewportHeight;
  const spaceAboveTrigger = hasTrigger
    ? (input.triggerTop as number) - input.viewportTop - input.sideOffset
    : viewportHeight;
  const preferredSideSpace = Math.max(spaceBelowTrigger, spaceAboveTrigger, 0);
  const maxPanelHeight = Math.min(viewportHeight, preferredSideSpace || viewportHeight);

  return resolveDesktopMenuListMaxHeight(maxPanelHeight, input.verticalChrome);
}
