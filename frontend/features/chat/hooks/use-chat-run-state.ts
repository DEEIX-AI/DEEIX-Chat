"use client";

import * as React from "react";

import { ConversationRunStore } from "@/features/chat/model/conversation-run-store";
import { streamActiveConversationRuns } from "@/shared/api/conversation";
import type { ActiveConversationRunEvent } from "@/shared/api/conversation.types";
import { useOptionalAuthSession } from "@/shared/auth/auth-session-context";

const RUN_STREAM_RECONNECT_MIN_MS = 1_000;
const RUN_STREAM_RECONNECT_MAX_MS = 30_000;
const RUN_STREAM_RECONNECT_JITTER_RATIO = 0.2;
const RUN_STATE_CHANNEL = "deeix-conversation-runs";

export function useChatRunState() {
  const authSession = useOptionalAuthSession();
  const accessToken = authSession?.accessToken ?? "";
  const userPublicID = authSession?.user?.publicID.trim() ?? "";
  const storeRef = React.useRef<ConversationRunStore | null>(null);
  const storeUserPublicIDRef = React.useRef(userPublicID);
  const runStateChannelRef = React.useRef<BroadcastChannel | null>(null);
  if (!storeRef.current) {
    storeRef.current = new ConversationRunStore();
  }
  const store = storeRef.current;

  const registerConversationRun = React.useCallback(
    (runID: string, conversationPublicID: string) => {
      const normalizedRunID = runID.trim();
      const normalizedConversationID = conversationPublicID.trim();
      if (!normalizedRunID || !normalizedConversationID) {
        return;
      }
      runStateChannelRef.current?.postMessage({
        type: "started",
        runID: normalizedRunID,
        conversationPublicID: normalizedConversationID,
      });
      store.register(normalizedRunID, normalizedConversationID);
    },
    [store],
  );

  const detachConversationRun = React.useCallback(
    (runID: string) => store.detach(runID),
    [store],
  );

  const finishConversationRun = React.useCallback(
    (runID: string) => {
      const normalizedRunID = runID.trim();
      if (!normalizedRunID) {
        return;
      }
      runStateChannelRef.current?.postMessage({ type: "finished", runID: normalizedRunID });
      store.settle(normalizedRunID);
    },
    [store],
  );

  React.useEffect(() => {
    if (storeUserPublicIDRef.current === userPublicID) {
      return;
    }
    storeUserPublicIDRef.current = userPublicID;
    store.clear();
  }, [store, userPublicID]);

  React.useEffect(() => {
    if (typeof BroadcastChannel === "undefined" || !userPublicID) {
      return;
    }
    const channel = new BroadcastChannel(`${RUN_STATE_CHANNEL}:${userPublicID}`);
    runStateChannelRef.current = channel;
    channel.onmessage = (event: MessageEvent<unknown>) => {
      if (!event.data || typeof event.data !== "object") {
        return;
      }
      const message = event.data as {
        type?: unknown;
        runID?: unknown;
        conversationPublicID?: unknown;
      };
      const runID = typeof message.runID === "string" ? message.runID.trim() : "";
      if (!runID) {
        return;
      }
      if (message.type === "finished") {
        store.applyFinished(runID, false);
        return;
      }
      const conversationPublicID =
        typeof message.conversationPublicID === "string"
          ? message.conversationPublicID.trim()
          : "";
      if (message.type === "started" && conversationPublicID) {
        store.applyStarted(runID, conversationPublicID);
      }
    };
    return () => {
      runStateChannelRef.current = null;
      channel.close();
    };
  }, [store, userPublicID]);

  React.useEffect(() => {
    if (!accessToken) {
      store.clear();
      return;
    }
    let disposed = false;
    let reconnectAttempt = 0;
    let reconnectTimer: number | null = null;
    let streamController: AbortController | null = null;

    function clearReconnectTimer() {
      if (reconnectTimer !== null) {
        window.clearTimeout(reconnectTimer);
        reconnectTimer = null;
      }
    }

    function scheduleReconnect() {
      if (disposed || document.visibilityState !== "visible" || navigator.onLine === false) {
        return;
      }
      clearReconnectTimer();
      const baseDelay = Math.min(
        RUN_STREAM_RECONNECT_MIN_MS * 2 ** reconnectAttempt,
        RUN_STREAM_RECONNECT_MAX_MS,
      );
      reconnectAttempt += 1;
      const jitter =
        baseDelay * RUN_STREAM_RECONNECT_JITTER_RATIO * (Math.random() * 2 - 1);
      reconnectTimer = window.setTimeout(connect, Math.max(0, Math.round(baseDelay + jitter)));
    }

    function handleEvent(event: ActiveConversationRunEvent) {
      reconnectAttempt = 0;
      if (event.type === "snapshot") {
        store.synchronize(event.runs);
      } else if (event.type === "started") {
        store.applyStarted(event.runID, event.conversationPublicID ?? "");
      } else {
        store.applyFinished(event.runID, true);
      }
    }

    function connect() {
      if (
        disposed ||
        streamController ||
        document.visibilityState !== "visible" ||
        navigator.onLine === false
      ) {
        return;
      }
      clearReconnectTimer();
      const controller = new AbortController();
      streamController = controller;
      void streamActiveConversationRuns(accessToken, {
        signal: controller.signal,
        onEvent: handleEvent,
      })
        .catch(() => {
          // Reconnect below with bounded exponential backoff.
        })
        .finally(() => {
          if (streamController === controller) {
            streamController = null;
          }
          if (
            !disposed &&
            document.visibilityState === "visible" &&
            navigator.onLine !== false
          ) {
            if (controller.signal.aborted) {
              connect();
            } else {
              scheduleReconnect();
            }
          }
        });
    }

    const handleVisibilityChange = () => {
      if (document.visibilityState === "visible") {
        connect();
        return;
      }
      clearReconnectTimer();
      streamController?.abort();
    };
    const handleOnline = () => connect();
    const handleOffline = () => {
      clearReconnectTimer();
      streamController?.abort();
    };

    connect();
    window.addEventListener("focus", handleOnline);
    window.addEventListener("online", handleOnline);
    window.addEventListener("offline", handleOffline);
    document.addEventListener("visibilitychange", handleVisibilityChange);
    return () => {
      disposed = true;
      clearReconnectTimer();
      streamController?.abort();
      window.removeEventListener("focus", handleOnline);
      window.removeEventListener("online", handleOnline);
      window.removeEventListener("offline", handleOffline);
      document.removeEventListener("visibilitychange", handleVisibilityChange);
    };
  }, [accessToken, store]);

  return {
    detachConversationRun,
    finishConversationRun,
    registerConversationRun,
    store,
  };
}
