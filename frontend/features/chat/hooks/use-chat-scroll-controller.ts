"use client";

import * as React from "react";

const CHAT_SCROLL_STORAGE_KEY = "deeix-chat:chat-scroll:v1";
const BOTTOM_THRESHOLD_PX = 96;
const SCROLL_POSITION_PERSIST_DELAY_MS = 180;

type PersistedScrollEntry = {
  mode: "bottom" | "offset";
  scrollTop: number;
  updatedAt: string;
};

type PersistedScrollStore = Record<string, PersistedScrollEntry>;

function readScrollStore(): PersistedScrollStore {
  if (typeof window === "undefined") {
    return {};
  }

  try {
    const raw = window.localStorage.getItem(CHAT_SCROLL_STORAGE_KEY);
    if (!raw) {
      return {};
    }
    const parsed = JSON.parse(raw) as unknown;
    if (!parsed || typeof parsed !== "object") {
      return {};
    }
    return parsed as PersistedScrollStore;
  } catch {
    return {};
  }
}

function writeScrollStore(store: PersistedScrollStore) {
  if (typeof window === "undefined") {
    return;
  }

  try {
    if (Object.keys(store).length === 0) {
      window.localStorage.removeItem(CHAT_SCROLL_STORAGE_KEY);
      return;
    }
    window.localStorage.setItem(CHAT_SCROLL_STORAGE_KEY, JSON.stringify(store));
  } catch {
    // Ignore storage write failures and keep scrolling usable.
  }
}

function readScrollEntry(conversationID: string | null): PersistedScrollEntry | null {
  const key = conversationID?.trim();
  if (!key) {
    return null;
  }
  return readScrollStore()[key] ?? null;
}

function writeScrollEntry(conversationID: string | null, entry: PersistedScrollEntry | null) {
  const key = conversationID?.trim();
  if (!key) {
    return;
  }

  const store = readScrollStore();
  if (!entry) {
    delete store[key];
  } else {
    store[key] = entry;
  }
  writeScrollStore(store);
}

export function useChatScrollController({
  conversationID,
  loading,
  isConversationMode,
  visibleMessageCount,
  showPendingAssistant,
  streamingText,
  streamingTraceText,
}: {
  conversationID: string | null;
  loading: boolean;
  isConversationMode: boolean;
  visibleMessageCount: number;
  showPendingAssistant: boolean;
  streamingText: string;
  streamingTraceText: string;
}) {
  const messageViewportRef = React.useRef<HTMLDivElement | null>(null);
  const messageContentRef = React.useRef<HTMLDivElement | null>(null);
  const autoFollowRef = React.useRef(true);
  const autoFollowFrameRef = React.useRef<number | null>(null);
  const persistTimerRef = React.useRef<number | null>(null);
  const restoreFrameRef = React.useRef<number | null>(null);
  const programmaticScrollRef = React.useRef(false);
  const pendingConversationRestoreRef = React.useRef(false);
  const currentConversationIDRef = React.useRef<string | null>(conversationID);
  const wasStreamingRef = React.useRef(false);
  const [showScrollToLatestButton, setShowScrollToLatestButton] = React.useState(false);

  const isNearBottom = React.useCallback((viewport: HTMLDivElement) => {
    const distanceFromBottom = viewport.scrollHeight - viewport.scrollTop - viewport.clientHeight;
    return distanceFromBottom <= BOTTOM_THRESHOLD_PX;
  }, []);

  const updateScrollAffordance = React.useCallback(
    (viewport: HTMLDivElement | null) => {
      if (!viewport) {
        setShowScrollToLatestButton(false);
        return true;
      }
      const scrollable = viewport.scrollHeight - viewport.clientHeight > BOTTOM_THRESHOLD_PX;
      const nearBottom = isNearBottom(viewport);
      setShowScrollToLatestButton(scrollable && !nearBottom);
      return nearBottom;
    },
    [isNearBottom],
  );

  const persistViewportPosition = React.useCallback(
    (targetConversationID: string | null, viewport: HTMLDivElement | null) => {
      if (!targetConversationID || !viewport) {
        return;
      }

      writeScrollEntry(targetConversationID, {
        mode: isNearBottom(viewport) ? "bottom" : "offset",
        scrollTop: viewport.scrollTop,
        updatedAt: new Date().toISOString(),
      });
    },
    [isNearBottom],
  );

  const schedulePersistViewportPosition = React.useCallback(
    (targetConversationID: string | null) => {
      if (!targetConversationID || persistTimerRef.current !== null) {
        return;
      }

      persistTimerRef.current = window.setTimeout(() => {
        persistTimerRef.current = null;
        persistViewportPosition(targetConversationID, messageViewportRef.current);
      }, SCROLL_POSITION_PERSIST_DELAY_MS);
    },
    [persistViewportPosition],
  );

  const scrollToLatest = React.useCallback((behavior: ScrollBehavior = "auto") => {
    const viewport = messageViewportRef.current;
    if (!viewport) {
      return;
    }
    programmaticScrollRef.current = true;
    autoFollowRef.current = true;
    viewport.scrollTo({
      top: viewport.scrollHeight,
      behavior,
    });
    setShowScrollToLatestButton(false);
    window.requestAnimationFrame(() => {
      window.requestAnimationFrame(() => {
        programmaticScrollRef.current = false;
        updateScrollAffordance(viewport);
      });
    });
  }, [updateScrollAffordance]);

  const scrollToLatestSmoothly = React.useCallback(() => {
    scrollToLatest("smooth");
  }, [scrollToLatest]);

  const restoreViewportPosition = React.useCallback(() => {
    const viewport = messageViewportRef.current;
    if (!viewport) {
      return false;
    }

    const entry = readScrollEntry(currentConversationIDRef.current);
    if (!entry || entry.mode === "bottom") {
      scrollToLatest();
      return true;
    }

    const maxScrollTop = Math.max(0, viewport.scrollHeight - viewport.clientHeight);
    programmaticScrollRef.current = true;
    autoFollowRef.current = false;
    viewport.scrollTop = Math.min(entry.scrollTop, maxScrollTop);
    window.requestAnimationFrame(() => {
      programmaticScrollRef.current = false;
      updateScrollAffordance(viewport);
    });
    return true;
  }, [scrollToLatest, updateScrollAffordance]);

  const scheduleScrollToLatest = React.useCallback(() => {
    if (!autoFollowRef.current || autoFollowFrameRef.current !== null) {
      return;
    }
    autoFollowFrameRef.current = window.requestAnimationFrame(() => {
      autoFollowFrameRef.current = null;
      if (!autoFollowRef.current) {
        return;
      }
      scrollToLatest();
    });
  }, [scrollToLatest]);

  const onScroll = React.useCallback(() => {
    const viewport = messageViewportRef.current;
    if (!viewport || programmaticScrollRef.current) {
      return;
    }
    autoFollowRef.current = updateScrollAffordance(viewport);
    schedulePersistViewportPosition(currentConversationIDRef.current);
  }, [schedulePersistViewportPosition, updateScrollAffordance]);

  React.useEffect(() => {
    if (!isConversationMode && visibleMessageCount === 0) {
      return;
    }
    if (pendingConversationRestoreRef.current) {
      return;
    }
    scheduleScrollToLatest();
  }, [isConversationMode, scheduleScrollToLatest, visibleMessageCount]);

  const hasLiveStreamingContent = showPendingAssistant || streamingText.length > 0 || streamingTraceText.length > 0;
  const liveContentTick = `${visibleMessageCount}:${streamingText.length}:${streamingTraceText.length}:${showPendingAssistant ? "1" : "0"}`;

  React.useLayoutEffect(() => {
    if (!hasLiveStreamingContent) {
      if (wasStreamingRef.current) {
        wasStreamingRef.current = false;
        scheduleScrollToLatest();
        return;
      }
      wasStreamingRef.current = false;
      return;
    }

    if (!wasStreamingRef.current) {
      wasStreamingRef.current = true;
      autoFollowRef.current = true;
      scrollToLatest();
      return;
    }

    scheduleScrollToLatest();
  }, [hasLiveStreamingContent, scheduleScrollToLatest, scrollToLatest]);

  React.useLayoutEffect(() => {
    if (pendingConversationRestoreRef.current) {
      return;
    }
    const viewport = messageViewportRef.current;
    if (!viewport) {
      setShowScrollToLatestButton(false);
      return;
    }
    if (!hasLiveStreamingContent) {
      updateScrollAffordance(viewport);
      return;
    }
    if (autoFollowRef.current) {
      scrollToLatest();
      return;
    }
    updateScrollAffordance(viewport);
  }, [hasLiveStreamingContent, liveContentTick, scrollToLatest, updateScrollAffordance]);

  React.useEffect(() => {
    const content = messageContentRef.current;
    if (!content || typeof ResizeObserver === "undefined") {
      return;
    }

    const observer = new ResizeObserver(() => {
      const viewport = messageViewportRef.current;
      if (!viewport) {
        updateScrollAffordance(null);
        return;
      }
      if (!autoFollowRef.current) {
        updateScrollAffordance(viewport);
        return;
      }
      scheduleScrollToLatest();
    });
    observer.observe(content);
    return () => observer.disconnect();
  }, [scheduleScrollToLatest, updateScrollAffordance]);

  React.useLayoutEffect(() => {
    currentConversationIDRef.current = conversationID;
    autoFollowRef.current = true;
    pendingConversationRestoreRef.current = Boolean(conversationID);
    const viewport = messageViewportRef.current;

    return () => {
      if (persistTimerRef.current !== null) {
        window.clearTimeout(persistTimerRef.current);
        persistTimerRef.current = null;
      }
      persistViewportPosition(conversationID, viewport);
    };
  }, [conversationID, persistViewportPosition]);

  React.useLayoutEffect(() => {
    if (!pendingConversationRestoreRef.current || loading) {
      return;
    }

    if (restoreFrameRef.current !== null) {
      window.cancelAnimationFrame(restoreFrameRef.current);
    }

    restoreFrameRef.current = window.requestAnimationFrame(() => {
      restoreFrameRef.current = null;
      pendingConversationRestoreRef.current = false;
      restoreViewportPosition();
    });

    return () => {
      if (restoreFrameRef.current !== null) {
        window.cancelAnimationFrame(restoreFrameRef.current);
        restoreFrameRef.current = null;
      }
    };
  }, [loading, restoreViewportPosition, visibleMessageCount]);

  React.useEffect(() => {
    const viewport = messageViewportRef.current;

    return () => {
      if (autoFollowFrameRef.current !== null) {
        window.cancelAnimationFrame(autoFollowFrameRef.current);
        autoFollowFrameRef.current = null;
      }
      if (persistTimerRef.current !== null) {
        window.clearTimeout(persistTimerRef.current);
        persistTimerRef.current = null;
      }
      if (restoreFrameRef.current !== null) {
        window.cancelAnimationFrame(restoreFrameRef.current);
        restoreFrameRef.current = null;
      }
      persistViewportPosition(currentConversationIDRef.current, viewport);
    };
  }, [persistViewportPosition]);

  return {
    messageViewportRef,
    messageContentRef,
    onScroll,
    onScrollToLatest: scrollToLatestSmoothly,
    scheduleScrollToLatest,
    showScrollToLatestButton,
  };
}
