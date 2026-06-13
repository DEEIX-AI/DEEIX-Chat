"use client";

import * as React from "react";

const HTML_VISUAL_PROMPT_STORAGE_KEY = "deeix-chat:html-visual-prompt:v1";
const useIsomorphicLayoutEffect = typeof window === "undefined" ? React.useEffect : React.useLayoutEffect;

function readHTMLVisualPromptEnabled(): boolean {
  if (typeof window === "undefined") {
    return false;
  }
  try {
    return window.localStorage.getItem(HTML_VISUAL_PROMPT_STORAGE_KEY) === "true";
  } catch {
    return false;
  }
}

function writeHTMLVisualPromptEnabled(enabled: boolean): void {
  if (typeof window === "undefined") {
    return;
  }
  try {
    window.localStorage.setItem(HTML_VISUAL_PROMPT_STORAGE_KEY, String(enabled));
  } catch {
    // localStorage may be unavailable in private browsing or strict environments.
  }
}

export function useChatVisualPrompt() {
  const [enabled, setEnabledState] = React.useState(false);

  useIsomorphicLayoutEffect(() => {
    setEnabledState(readHTMLVisualPromptEnabled());
  }, []);

  React.useEffect(() => {
    if (typeof window === "undefined") {
      return;
    }

    function onStorage(event: StorageEvent) {
      if (event.key === HTML_VISUAL_PROMPT_STORAGE_KEY) {
        setEnabledState(event.newValue === "true");
      }
    }

    window.addEventListener("storage", onStorage);
    return () => window.removeEventListener("storage", onStorage);
  }, []);

  const setEnabled = React.useCallback((next: React.SetStateAction<boolean>) => {
    setEnabledState((previous) => {
      const resolved = typeof next === "function" ? next(previous) : next;
      writeHTMLVisualPromptEnabled(resolved);
      return resolved;
    });
  }, []);

  return {
    enabled,
    setEnabled,
  };
}
