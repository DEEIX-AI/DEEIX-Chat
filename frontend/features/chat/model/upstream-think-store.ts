"use client";

import * as React from "react";

import type { ChatMessageProcessTrace, ChatTraceBlock, ChatTraceEvent } from "@/features/chat/types/messages";
import { toPendingProcessTrace } from "@/features/chat/model/message-submit";
import type { StreamMessageEvent } from "@/shared/api/conversation.types";

type UpstreamThinkDeltaEvent = Extract<StreamMessageEvent, { type: "upstream_think_delta" }>;
type Listener = () => void;

// 实时快照不再携带思考正文，已结束轮次的思考块以 events 形式留在 live trace 里，用于回填快照事件的正文与终态。
type LiveThinkEntry = {
  trace: ChatMessageProcessTrace;
  currentEventID: string;
};

const traces = new Map<string, LiveThinkEntry>();
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
  const roundID = event.roundID || current?.roundID;
  const roundChanged = Boolean(roundID && current?.roundID && roundID !== current.roundID);
  const contentMarkdown = mergeContent(roundChanged ? "" : (current?.contentMarkdown ?? ""), event);
  const eventStartedAt = typeof event.startedAt === "string"
    ? event.startedAt.trim() || undefined
    : undefined;
  const eventEndedAt = typeof event.endedAt === "string"
    ? event.endedAt.trim() || undefined
    : undefined;
  return {
    title: event.title?.trim() || current?.title || "",
    summary: event.summary?.trim() || current?.summary || "",
    contentMarkdown,
    status: event.status || current?.status || "streaming",
    stage: event.stage || current?.stage || "think",
    roundID,
    parentEventID: current?.parentEventID,
    startedAt: eventStartedAt ?? (roundChanged ? undefined : current?.startedAt) ?? nowISO(),
    endedAt: eventEndedAt ?? (roundChanged ? undefined : current?.endedAt),
    updatedAt: nowISO(),
    payloadJson: current?.payloadJson,
  };
}

function isTerminalThinkStatus(status: string | undefined) {
  const normalized = status?.trim().toLowerCase();
  return normalized === "completed" || normalized === "error";
}

function thinkBlockToEvent(block: ChatTraceBlock, eventID: string): ChatTraceEvent {
  return {
    eventID,
    eventType: "think",
    phase: "upstream_think",
    stage: block.stage,
    roundID: block.roundID,
    parentEventID: block.parentEventID,
    title: block.title,
    summary: block.summary,
    contentMarkdown: block.contentMarkdown,
    status: block.status,
    seq: 0,
    startedAt: block.startedAt,
    endedAt: block.endedAt,
    updatedAt: block.updatedAt,
    payloadJson: block.payloadJson,
  };
}

function mergeUpstreamThinkDeltaTrace(
  current: LiveThinkEntry | undefined,
  event: UpstreamThinkDeltaEvent,
): LiveThinkEntry | undefined {
  const eventID = event.eventID?.trim() || current?.currentEventID || "";
  const fullTrace = toPendingProcessTrace(event.trace);
  if (fullTrace) {
    return { trace: fullTrace, currentEventID: eventID };
  }
  const currentTrace = current?.trace;
  const previousBlock = currentTrace?.upstreamThink;
  const roundChanged = Boolean(event.roundID && previousBlock?.roundID && event.roundID !== previousBlock.roundID);
  const events = currentTrace?.events ? [...currentTrace.events] : [];
  if (roundChanged && previousBlock && (previousBlock.contentMarkdown || previousBlock.summary)) {
    events.push(thinkBlockToEvent(previousBlock, current?.currentEventID || ""));
  }
  const upstreamThink = mergeUpstreamThinkBlock(previousBlock, event);
  return {
    trace: {
      enabled: true,
      status: event.status || currentTrace?.status || "streaming",
      process: currentTrace?.process,
      tools: currentTrace?.tools,
      upstreamThink,
      promptTrace: currentTrace?.promptTrace,
      events: events.length > 0 ? events : undefined,
    },
    currentEventID: eventID,
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
  return key ? traces.get(key)?.trace : undefined;
}

export function upsertLiveUpstreamThinkTrace(runID: string | null | undefined, event: UpstreamThinkDeltaEvent) {
  const key = normalizeRunID(runID);
  if (!key) {
    return undefined;
  }
  const next = mergeUpstreamThinkDeltaTrace(traces.get(key), event);
  if (!next) {
    return traces.get(key)?.trace;
  }
  traces.set(key, next);
  notify(key);
  return next.trace;
}

export function clearLiveUpstreamThinkTrace(runID: string | null | undefined) {
  const key = normalizeRunID(runID);
  if (!key || !traces.delete(key)) {
    return;
  }
  notify(key);
}

function liveThinkBlockFor(event: ChatTraceEvent, live: ChatMessageProcessTrace): ChatTraceBlock | undefined {
  const eventID = event.eventID?.trim() || "";
  const roundID = event.roundID?.trim() || "";
  const byEvent = live.events?.find((item) => eventID && item.eventID === eventID);
  if (byEvent) {
    return byEvent;
  }
  const byRound = live.events?.find((item) => roundID && item.roundID === roundID);
  if (byRound) {
    return byRound;
  }
  if (live.upstreamThink && roundID && live.upstreamThink.roundID === roundID) {
    return live.upstreamThink;
  }
  return undefined;
}

// 快照里的思考事件只带结构与摘要，正文与终态由实时思考块回填。
function enrichThinkEvents(events: ChatTraceEvent[] | undefined, live: ChatMessageProcessTrace): ChatTraceEvent[] | undefined {
  if (!events?.length) {
    return events;
  }
  return events.map((event) => {
    if (event.phase !== "upstream_think" && event.eventType !== "think") {
      return event;
    }
    const block = liveThinkBlockFor(event, live);
    if (!block) {
      return event;
    }
    const terminal = isTerminalThinkStatus(block.status);
    return {
      ...event,
      summary: event.summary || block.summary,
      contentMarkdown: block.contentMarkdown.length > event.contentMarkdown.length ? block.contentMarkdown : event.contentMarkdown,
      status: terminal ? block.status : event.status,
      endedAt: event.endedAt ?? block.endedAt,
    };
  });
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
    events: enrichThinkEvents(base?.events, live),
  };
}

export function preserveRicherLiveUpstreamThinkTrace(
  base: ChatMessageProcessTrace | undefined,
  live: ChatMessageProcessTrace | undefined,
) {
  const baseContent = base?.upstreamThink?.contentMarkdown ?? "";
  const liveContent = live?.upstreamThink?.contentMarkdown ?? "";
  if (!live?.upstreamThink || liveContent.length <= baseContent.length) {
    return base ?? live;
  }
  return mergeLiveUpstreamThinkTrace(base, live);
}

export function useLiveUpstreamThinkTrace(runID: string | null | undefined) {
  const key = normalizeRunID(runID);
  return React.useSyncExternalStore(
    React.useCallback((listener) => subscribe(key, listener), [key]),
    React.useCallback(() => readLiveUpstreamThinkTrace(key), [key]),
    (): undefined => undefined,
  );
}
