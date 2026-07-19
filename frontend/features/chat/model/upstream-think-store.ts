"use client";

import * as React from "react";

import type { ChatMessageProcessTrace, ChatTraceBlock } from "@/features/chat/types/messages";
import { toPendingProcessTrace } from "@/features/chat/model/message-submit";
import type { StreamMessageEvent } from "@/shared/api/conversation.types";

type UpstreamThinkDeltaEvent = Extract<StreamMessageEvent, { type: "upstream_think_delta" }>;
type Listener = () => void;

const traces = new Map<string, ChatMessageProcessTrace>();
const listeners = new Map<string, Set<Listener>>();

function nowISO() {
  return new Date().toISOString();
}

function normalizeRunID(runID: string | null | undefined) {
  return runID?.trim() || "";
}

function mergeContent(previous: string, event: UpstreamThinkDeltaEvent) {
  if (typeof event.contentMarkdown === "string") {
    return event.contentMarkdown;
  }
  if (typeof event.delta === "string" && event.delta.length > 0) {
    return `${previous}${event.delta}`;
  }
  return previous;
}

function mergeUpstreamThinkBlock(current: ChatTraceBlock | undefined, event: UpstreamThinkDeltaEvent): ChatTraceBlock {
  const contentMarkdown = mergeContent(current?.contentMarkdown ?? "", event);
  return {
    title: event.title?.trim() || current?.title || "",
    summary: event.summary?.trim() || current?.summary || "",
    contentMarkdown,
    status: event.status || current?.status || "streaming",
    stage: event.stage || current?.stage || "think",
    roundID: event.roundID || current?.roundID,
    parentEventID: current?.parentEventID,
    updatedAt: nowISO(),
    payloadJson: current?.payloadJson,
  };
}

function mergeUpstreamThinkDeltaTrace(
  current: ChatMessageProcessTrace | undefined,
  event: UpstreamThinkDeltaEvent,
): ChatMessageProcessTrace | undefined {
  if (event.trace?.enabled) {
    return toPendingProcessTrace(event.trace);
  }
  const upstreamThink = mergeUpstreamThinkBlock(current?.upstreamThink, event);
  return {
    enabled: true,
    status: event.status || current?.status || "streaming",
    process: current?.process,
    tools: current?.tools,
    upstreamThink,
    promptTrace: current?.promptTrace,
    events: current?.events,
  };
}

function notify(runID: string) {
  listeners.get(runID)?.forEach((listener) => {
    listener();
  });
}

function subscribe(runID: string, listener: Listener) {
  if (!runID) {
    return () => {};
  }
  let listenersForRun = listeners.get(runID);
  if (!listenersForRun) {
    listenersForRun = new Set();
    listeners.set(runID, listenersForRun);
  }
  listenersForRun.add(listener);
  return () => {
    listenersForRun.delete(listener);
    if (listenersForRun.size === 0) {
      listeners.delete(runID);
    }
  };
}

export function readLiveUpstreamThinkTrace(runID: string | null | undefined) {
  const key = normalizeRunID(runID);
  return key ? traces.get(key) : undefined;
}

export function upsertLiveUpstreamThinkTrace(runID: string | null | undefined, event: UpstreamThinkDeltaEvent) {
  const key = normalizeRunID(runID);
  if (!key) {
    return undefined;
  }
  const next = mergeUpstreamThinkDeltaTrace(traces.get(key), event);
  if (!next) {
    return traces.get(key);
  }
  traces.set(key, next);
  notify(key);
  return next;
}

export function clearLiveUpstreamThinkTrace(runID: string | null | undefined) {
  const key = normalizeRunID(runID);
  if (!key || !traces.delete(key)) {
    return;
  }
  notify(key);
}

export function mergeLiveUpstreamThinkTrace(
  base: ChatMessageProcessTrace | undefined,
  live: ChatMessageProcessTrace | undefined,
) {
  if (!live?.upstreamThink) {
    return base;
  }
  return {
    enabled: true,
    status: live.status || base?.status || "streaming",
    process: base?.process,
    tools: base?.tools,
    upstreamThink: live.upstreamThink,
    promptTrace: base?.promptTrace,
    events: base?.events,
  };
}

export function useLiveUpstreamThinkTrace(runID: string | null | undefined) {
  const key = normalizeRunID(runID);
  return React.useSyncExternalStore(
    React.useCallback((listener) => subscribe(key, listener), [key]),
    React.useCallback(() => readLiveUpstreamThinkTrace(key), [key]),
    () => undefined,
  );
}
