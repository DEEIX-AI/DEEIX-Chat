"use client";

import * as React from "react";

type AutoExpandDisclosureOptions = {
  active: boolean;
  autoExpand: boolean;
  collapseReady?: boolean;
};

/**
 * Keeps an active disclosure aligned with the user's auto-expand preference
 * while preserving manual changes for the current activity cycle.
 */
export function useAutoExpandDisclosure({
  active,
  autoExpand,
  collapseReady = !active,
}: AutoExpandDisclosureOptions) {
  const [open, setOpen] = React.useState(() => active && autoExpand);
  const manuallyChangedRef = React.useRef(false);
  const wasActiveRef = React.useRef(active);

  React.useEffect(() => {
    if (active) {
      if (!manuallyChangedRef.current) {
        setOpen(autoExpand);
      }
      wasActiveRef.current = true;
      return;
    }

    if (wasActiveRef.current && collapseReady) {
      setOpen(false);
      manuallyChangedRef.current = false;
      wasActiveRef.current = false;
    }
  }, [active, autoExpand, collapseReady]);

  const onOpenChange = React.useCallback((nextOpen: boolean) => {
    if (active) {
      manuallyChangedRef.current = true;
    }
    setOpen(nextOpen);
  }, [active]);

  return { open, onOpenChange };
}
