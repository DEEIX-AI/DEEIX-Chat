"use client";

import * as React from "react";

const CHAT_SCROLL_STORAGE_KEY = "deeix-chat:chat-scroll:v1";
const BOTTOM_THRESHOLD_PX = 96;
const TOP_LOAD_THRESHOLD_PX = 48;
const SCROLL_POSITION_PERSIST_DELAY_MS = 180;
const RESTORE_RETRY_FRAMES = 3;

type PersistedScrollEntry = {
  mode: "bottom" | "offset";
  scrollTop: number;
  updatedAt: string;
};

type PersistedScrollStore = Record<string, PersistedScrollEntry>;
type RestoreState = "idle" | "pending";
type FollowMode = "follow" | "manual";

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
  latestMessageKey,
  showPendingAssistant,
  streamingText,
  streamingTraceText,
  hasOlderMessages = false,
  loadingOlderMessages = false,
  onLoadOlderMessages,
}: {
  conversationID: string | null;
  loading: boolean;
  isConversationMode: boolean;
  visibleMessageCount: number;
  latestMessageKey: string;
  showPendingAssistant: boolean;
  streamingText: string;
  streamingTraceText: string;
  hasOlderMessages?: boolean;
  loadingOlderMessages?: boolean;
  onLoadOlderMessages?: () => Promise<boolean> | boolean;
}) {
  const messageViewportRef = React.useRef<HTMLDivElement | null>(null);
  const messageContentRef = React.useRef<HTMLDivElement | null>(null);
  const messageEndRef = React.useRef<HTMLDivElement | null>(null);
  const followModeRef = React.useRef<FollowMode>("follow");
  const layoutFrameRef = React.useRef<number | null>(null);
  const persistTimerRef = React.useRef<number | null>(null);
  const restoreFrameRef = React.useRef<number | null>(null);
  const restoreStateRef = React.useRef<RestoreState>("idle");
  const currentConversationIDRef = React.useRef<string | null>(conversationID);
  const handledConversationIDRef = React.useRef<string | null | undefined>(undefined);
  const wasStreamingRef = React.useRef(false);
  const hasOlderMessagesRef = React.useRef(hasOlderMessages);
  const loadingOlderMessagesRef = React.useRef(loadingOlderMessages);
  const onLoadOlderMessagesRef = React.useRef(onLoadOlderMessages);
  const loadingOlderInFlightRef = React.useRef(false);
  const olderScrollRestoreRef = React.useRef<{ scrollHeight: number; scrollTop: number } | null>(null);
  const [showScrollToLatestButton, setShowScrollToLatestButton] = React.useState(false);

  const hasLiveStreamingContent = showPendingAssistant || streamingText.length > 0 || streamingTraceText.length > 0;
  const contentVersion = `${visibleMessageCount}:${latestMessageKey}:${streamingText.length}:${streamingTraceText.length}:${showPendingAssistant ? "1" : "0"}`;

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

  const setFollowMode = React.useCallback((mode: FollowMode) => {
    followModeRef.current = mode;
  }, []);

  React.useEffect(() => {
    hasOlderMessagesRef.current = hasOlderMessages;
  }, [hasOlderMessages]);

  React.useEffect(() => {
    loadingOlderMessagesRef.current = loadingOlderMessages;
  }, [loadingOlderMessages]);

  React.useEffect(() => {
    onLoadOlderMessagesRef.current = onLoadOlderMessages;
  }, [onLoadOlderMessages]);

  const scrollViewportToBottom = React.useCallback(
    (viewport: HTMLDivElement | null = messageViewportRef.current) => {
      if (!viewport) {
        setShowScrollToLatestButton(false);
        return;
      }

      const maxScrollTop = Math.max(0, viewport.scrollHeight - viewport.clientHeight);
      if (Math.abs(viewport.scrollTop - maxScrollTop) > 1) {
        viewport.scrollTop = maxScrollTop;
      }
      updateScrollAffordance(viewport);
    },
    [updateScrollAffordance],
  );

  const applyScrollPolicy = React.useCallback(() => {
    const viewport = messageViewportRef.current;
    if (!viewport || restoreStateRef.current === "pending") {
      return;
    }

    if (followModeRef.current === "follow") {
      scrollViewportToBottom(viewport);
      return;
    }

    updateScrollAffordance(viewport);
  }, [scrollViewportToBottom, updateScrollAffordance]);

  const scheduleApplyScrollPolicy = React.useCallback(() => {
    if (layoutFrameRef.current !== null) {
      return;
    }
    layoutFrameRef.current = window.requestAnimationFrame(() => {
      layoutFrameRef.current = null;
      applyScrollPolicy();
    });
  }, [applyScrollPolicy]);

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

  const triggerLoadOlderMessages = React.useCallback(() => {
    const viewport = messageViewportRef.current;
    const loadOlder = onLoadOlderMessagesRef.current;
    if (
      !viewport ||
      !loadOlder ||
      !hasOlderMessagesRef.current ||
      loadingOlderMessagesRef.current ||
      loadingOlderInFlightRef.current
    ) {
      return;
    }

    setFollowMode("manual");
    loadingOlderInFlightRef.current = true;
    olderScrollRestoreRef.current = {
      scrollHeight: viewport.scrollHeight,
      scrollTop: viewport.scrollTop,
    };

    Promise.resolve(loadOlder())
      .then((loaded) => {
        if (!loaded) {
          olderScrollRestoreRef.current = null;
        }
      })
      .catch(() => {
        olderScrollRestoreRef.current = null;
      })
      .finally(() => {
        loadingOlderInFlightRef.current = false;
      });
  }, [setFollowMode]);

  const restoreViewportPosition = React.useCallback(() => {
    const viewport = messageViewportRef.current;
    if (!viewport) {
      return false;
    }

    const entry = readScrollEntry(currentConversationIDRef.current);
    if (!entry || entry.mode === "bottom") {
      setFollowMode("follow");
      scrollViewportToBottom(viewport);
      return true;
    }

    const maxScrollTop = Math.max(0, viewport.scrollHeight - viewport.clientHeight);
    setFollowMode("manual");
    viewport.scrollTop = Math.min(entry.scrollTop, maxScrollTop);
    updateScrollAffordance(viewport);
    return true;
  }, [scrollViewportToBottom, setFollowMode, updateScrollAffordance]);

  const onScroll = React.useCallback(() => {
    const viewport = messageViewportRef.current;
    if (!viewport || restoreStateRef.current === "pending") {
      return;
    }

    setFollowMode(updateScrollAffordance(viewport) ? "follow" : "manual");
    if (viewport.scrollTop <= TOP_LOAD_THRESHOLD_PX) {
      triggerLoadOlderMessages();
    }
    schedulePersistViewportPosition(currentConversationIDRef.current);
  }, [schedulePersistViewportPosition, setFollowMode, triggerLoadOlderMessages, updateScrollAffordance]);

  const onScrollToLatest = React.useCallback(() => {
    const viewport = messageViewportRef.current;
    setFollowMode("follow");
    scrollViewportToBottom(viewport);
    persistViewportPosition(currentConversationIDRef.current, viewport);
  }, [persistViewportPosition, scrollViewportToBottom, setFollowMode]);

  React.useLayoutEffect(() => {
    if (handledConversationIDRef.current === conversationID) {
      return;
    }

    const previousConversationID = currentConversationIDRef.current;
    const viewport = messageViewportRef.current;
    handledConversationIDRef.current = conversationID;
    currentConversationIDRef.current = conversationID;
    restoreStateRef.current = conversationID ? "pending" : "idle";
    wasStreamingRef.current = false;
    olderScrollRestoreRef.current = null;
    loadingOlderInFlightRef.current = false;
    setFollowMode("follow");
    setShowScrollToLatestButton(false);

    return () => {
      if (layoutFrameRef.current !== null) {
        window.cancelAnimationFrame(layoutFrameRef.current);
        layoutFrameRef.current = null;
      }
      if (persistTimerRef.current !== null) {
        window.clearTimeout(persistTimerRef.current);
        persistTimerRef.current = null;
      }
      persistViewportPosition(previousConversationID, viewport);
    };
  }, [conversationID, persistViewportPosition, setFollowMode]);

  React.useLayoutEffect(() => {
    if (restoreStateRef.current !== "pending" || loading) {
      return;
    }

    if (restoreFrameRef.current !== null) {
      window.cancelAnimationFrame(restoreFrameRef.current);
    }

    const restoreWhenReady = (attempt: number) => {
      restoreFrameRef.current = null;
      const restored = restoreViewportPosition();
      if (restored || attempt >= RESTORE_RETRY_FRAMES) {
        restoreStateRef.current = "idle";
        return;
      }
      restoreFrameRef.current = window.requestAnimationFrame(() => restoreWhenReady(attempt + 1));
    };

    restoreFrameRef.current = window.requestAnimationFrame(() => restoreWhenReady(0));
    return () => {
      if (restoreFrameRef.current !== null) {
        window.cancelAnimationFrame(restoreFrameRef.current);
        restoreFrameRef.current = null;
      }
    };
  }, [loading, restoreViewportPosition, visibleMessageCount]);

  React.useLayoutEffect(() => {
    if (loadingOlderMessages) {
      return;
    }

    const restore = olderScrollRestoreRef.current;
    const viewport = messageViewportRef.current;
    if (!restore || !viewport) {
      return;
    }

    const heightDelta = viewport.scrollHeight - restore.scrollHeight;
    setFollowMode("manual");
    viewport.scrollTop = restore.scrollTop + Math.max(0, heightDelta);
    olderScrollRestoreRef.current = null;
    updateScrollAffordance(viewport);
  }, [loadingOlderMessages, setFollowMode, updateScrollAffordance, visibleMessageCount]);

  React.useLayoutEffect(() => {
    if (restoreStateRef.current === "pending") {
      return;
    }

    if (!isConversationMode && visibleMessageCount === 0) {
      updateScrollAffordance(messageViewportRef.current);
      return;
    }

    const wasStreaming = wasStreamingRef.current;
    wasStreamingRef.current = hasLiveStreamingContent;
    if (hasLiveStreamingContent && !wasStreaming) {
      setFollowMode("follow");
    }

    applyScrollPolicy();
  }, [
    applyScrollPolicy,
    contentVersion,
    hasLiveStreamingContent,
    isConversationMode,
    setFollowMode,
    updateScrollAffordance,
    visibleMessageCount,
  ]);

  React.useEffect(() => {
    const viewport = messageViewportRef.current;
    if (!viewport || restoreStateRef.current === "pending" || loading || loadingOlderMessages) {
      return;
    }
    if (viewport.scrollTop <= TOP_LOAD_THRESHOLD_PX) {
      triggerLoadOlderMessages();
    }
  }, [hasOlderMessages, loading, loadingOlderMessages, triggerLoadOlderMessages, visibleMessageCount]);

  React.useEffect(() => {
    const content = messageContentRef.current;
    const viewport = messageViewportRef.current;
    if ((!content && !viewport) || typeof ResizeObserver === "undefined") {
      return;
    }

    const observer = new ResizeObserver(() => {
      scheduleApplyScrollPolicy();
    });
    if (content) {
      observer.observe(content);
    }
    if (viewport) {
      observer.observe(viewport);
    }
    return () => observer.disconnect();
  }, [scheduleApplyScrollPolicy]);

  React.useEffect(() => {
    const viewport = messageViewportRef.current;

    return () => {
      if (layoutFrameRef.current !== null) {
        window.cancelAnimationFrame(layoutFrameRef.current);
        layoutFrameRef.current = null;
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
    messageEndRef,
    onScroll,
    onScrollToLatest,
    showScrollToLatestButton,
  };
}
