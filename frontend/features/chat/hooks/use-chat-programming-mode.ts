"use client";

import * as React from "react";

const PROGRAMMING_MODE_STORAGE_KEY = "deeix-chat:programming-mode:v1";
const useIsomorphicLayoutEffect = typeof window === "undefined" ? React.useEffect : React.useLayoutEffect;

function readProgrammingModeEnabled(): boolean {
  if (typeof window === "undefined") {
    return false;
  }
  try {
    return window.localStorage.getItem(PROGRAMMING_MODE_STORAGE_KEY) === "true";
  } catch {
    return false;
  }
}

function writeProgrammingModeEnabled(enabled: boolean): void {
  if (typeof window === "undefined") {
    return;
  }
  try {
    window.localStorage.setItem(PROGRAMMING_MODE_STORAGE_KEY, String(enabled));
  } catch {
    // localStorage may be unavailable in private browsing or strict environments.
  }
}

export function useChatProgrammingMode() {
  const [enabled, setEnabledState] = React.useState(false);

  useIsomorphicLayoutEffect(() => {
    setEnabledState(readProgrammingModeEnabled());
  }, []);

  React.useEffect(() => {
    if (typeof window === "undefined") {
      return;
    }

    function onStorage(event: StorageEvent) {
      if (event.key === PROGRAMMING_MODE_STORAGE_KEY) {
        setEnabledState(event.newValue === "true");
      }
    }

    window.addEventListener("storage", onStorage);
    return () => window.removeEventListener("storage", onStorage);
  }, []);

  const setEnabled = React.useCallback((next: React.SetStateAction<boolean>) => {
    setEnabledState((previous) => {
      const resolved = typeof next === "function" ? next(previous) : next;
      writeProgrammingModeEnabled(resolved);
      return resolved;
    });
  }, []);

  return {
    enabled,
    setEnabled,
  };
}
