"use client";

import * as React from "react";

const POINTER_INTERACTION_QUERIES = [
  "(hover: none)",
  "(hover: hover)",
  "(pointer: coarse)",
  "(any-hover: none)",
  "(any-hover: hover)",
  "(any-pointer: coarse)",
] as const;

export type PointerInteraction = {
  hasTouchInput: boolean;
  hasHoverInput: boolean;
  hasCoarsePointer: boolean;
};

const DEFAULT_POINTER_INTERACTION: PointerInteraction = {
  hasTouchInput: false,
  hasHoverInput: false,
  hasCoarsePointer: false,
};

function matchesMedia(query: string): boolean {
  if (typeof window === "undefined") {
    return false;
  }
  return window.matchMedia(query).matches;
}

function readPointerInteraction(): PointerInteraction {
  if (typeof window === "undefined") {
    return DEFAULT_POINTER_INTERACTION;
  }

  const hasCoarsePointer = matchesMedia("(pointer: coarse)") || matchesMedia("(any-pointer: coarse)");
  const hasHoverInput = matchesMedia("(hover: hover)") || matchesMedia("(any-hover: hover)");
  const hasNoHoverInput = matchesMedia("(hover: none)") || matchesMedia("(any-hover: none)");
  const maxTouchPoints = navigator.maxTouchPoints || 0;

  return {
    hasTouchInput: maxTouchPoints > 0 || hasCoarsePointer || hasNoHoverInput,
    hasHoverInput,
    hasCoarsePointer,
  };
}

function samePointerInteraction(left: PointerInteraction, right: PointerInteraction): boolean {
  return (
    left.hasTouchInput === right.hasTouchInput &&
    left.hasHoverInput === right.hasHoverInput &&
    left.hasCoarsePointer === right.hasCoarsePointer
  );
}

// usePointerInteraction 暴露设备交互能力，用于区分 hover-first 与 touch-first UI 行为。
export function usePointerInteraction(): PointerInteraction {
  const [interaction, setInteraction] = React.useState(DEFAULT_POINTER_INTERACTION);

  React.useEffect(() => {
    const queries = POINTER_INTERACTION_QUERIES.map((query) => window.matchMedia(query));
    const update = () => {
      const next = readPointerInteraction();
      setInteraction((current) => (samePointerInteraction(current, next) ? current : next));
    };

    update();
    for (const query of queries) {
      query.addEventListener("change", update);
    }
    return () => {
      for (const query of queries) {
        query.removeEventListener("change", update);
      }
    };
  }, []);

  return interaction;
}
