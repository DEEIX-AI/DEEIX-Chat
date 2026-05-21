"use client";

import * as React from "react";

import type { PendingExchange } from "@/features/chat/types/chat-runtime";

const STREAM_TEXT_FLUSH_INTERVAL_MS = 50;

export function useChatStreamBuffer({
  setPendingExchange,
}: {
  setPendingExchange: React.Dispatch<React.SetStateAction<PendingExchange | null>>;
}) {
  const streamingMessageKeyRef = React.useRef<string | null>(null);
  const pendingStreamTextRef = React.useRef("");
  const streamFlushFrameRef = React.useRef<number | null>(null);
  const streamFlushTimeoutRef = React.useRef<number | null>(null);
  const lastStreamFlushAtRef = React.useRef(0);

  const flushStreamText = React.useCallback(function flushStreamText() {
    streamFlushFrameRef.current = null;
    lastStreamFlushAtRef.current = performance.now();
    const pendingText = pendingStreamTextRef.current;
    const exchangeKey = streamingMessageKeyRef.current;
    if (!exchangeKey || !pendingText) {
      return;
    }
    pendingStreamTextRef.current = "";

    setPendingExchange((prev) => {
      if (!prev || prev.key !== exchangeKey) {
        return prev;
      }
      return {
        ...prev,
        assistantPending: false,
        assistantStreaming: true,
        assistantText: prev.assistantText + pendingText,
      };
    });

  }, [setPendingExchange]);

  const scheduleStreamFlush = React.useCallback(() => {
    if (streamFlushFrameRef.current !== null || streamFlushTimeoutRef.current !== null) {
      return;
    }

    const elapsed = performance.now() - lastStreamFlushAtRef.current;
    if (elapsed >= STREAM_TEXT_FLUSH_INTERVAL_MS) {
      streamFlushFrameRef.current = window.requestAnimationFrame(flushStreamText);
      return;
    }

    streamFlushTimeoutRef.current = window.setTimeout(() => {
      streamFlushTimeoutRef.current = null;
      streamFlushFrameRef.current = window.requestAnimationFrame(flushStreamText);
    }, STREAM_TEXT_FLUSH_INTERVAL_MS - elapsed);
  }, [flushStreamText]);

  const enqueueStreamText = React.useCallback(
    (delta: string) => {
      if (!delta) {
        return;
      }
      pendingStreamTextRef.current += delta;
      scheduleStreamFlush();
    },
    [scheduleStreamFlush],
  );

  const startStream = React.useCallback((exchangeKey: string) => {
    pendingStreamTextRef.current = "";
    streamingMessageKeyRef.current = exchangeKey;
    lastStreamFlushAtRef.current = 0;
  }, []);

  const flushStreamTextNow = React.useCallback(() => {
    if (streamFlushFrameRef.current !== null) {
      window.cancelAnimationFrame(streamFlushFrameRef.current);
      streamFlushFrameRef.current = null;
    }
    if (streamFlushTimeoutRef.current !== null) {
      window.clearTimeout(streamFlushTimeoutRef.current);
      streamFlushTimeoutRef.current = null;
    }
    flushStreamText();
  }, [flushStreamText]);

  const resetStreamBuffer = React.useCallback(() => {
    if (streamFlushFrameRef.current !== null) {
      window.cancelAnimationFrame(streamFlushFrameRef.current);
      streamFlushFrameRef.current = null;
    }
    if (streamFlushTimeoutRef.current !== null) {
      window.clearTimeout(streamFlushTimeoutRef.current);
      streamFlushTimeoutRef.current = null;
    }
    pendingStreamTextRef.current = "";
    streamingMessageKeyRef.current = null;
  }, []);

  React.useEffect(() => {
    return () => {
      resetStreamBuffer();
    };
  }, [resetStreamBuffer]);

  return {
    enqueueStreamText,
    flushStreamTextNow,
    resetStreamBuffer,
    startStream,
  };
}
