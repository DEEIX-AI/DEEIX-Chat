"use client";

import type { Context, Options } from "modern-screenshot";

import { SCREENSHOT_STYLE_PROPERTIES } from "@/features/chat/model/conversation-screenshot-style-properties";
import { screenshotWorkerPath } from "@/shared/generated/screenshot-worker";

export const MAX_SCREENSHOT_MESSAGES = 100;

const SCREENSHOT_SCALE = 2;
const SCREENSHOT_PADDING = 24;
const MAX_CANVAS_DIMENSION = 16384;
const MAX_CANVAS_PIXEL_AREA = 24 * 1024 * 1024;
const MIN_SCREENSHOT_SCALE = 1;
const SCREENSHOT_CLONE_YIELD_INTERVAL = 128;
const SCREENSHOT_CLONE_FRAME_BUDGET_MS = 16;
const SCREENSHOT_RESOURCE_WORKER_COUNT = 2;

type ScreenshotRenderer = typeof import("modern-screenshot");

let screenshotRendererPromise: Promise<ScreenshotRenderer> | null = null;

const SCREENSHOT_OMIT_SELECTOR = [
  "[data-screenshot-exclude='true']",
  ".chat-message-meta",
  ".chat-screenshot-omit",
  "[data-streamdown='code-block-actions']",
  "[data-streamdown='mermaid-block-actions']",
].join(",");

export class ConversationScreenshotTooLargeError extends Error {
  constructor() {
    super("conversation_screenshot_too_large");
    this.name = "ConversationScreenshotTooLargeError";
  }
}

type CaptureElementOptions = {
  signal?: AbortSignal;
};

type YieldingScheduler = {
  yield?: () => Promise<void>;
};

function resolveCaptureBackgroundColor(element: HTMLElement) {
  const ownerWindow = element.ownerDocument.defaultView ?? window;
  const bodyBackground = ownerWindow.getComputedStyle(ownerWindow.document.body).backgroundColor;
  if (bodyBackground && bodyBackground !== "rgba(0, 0, 0, 0)" && bodyBackground !== "transparent") {
    return bodyBackground;
  }
  const rootBackground = ownerWindow.getComputedStyle(ownerWindow.document.documentElement).backgroundColor;
  if (rootBackground && rootBackground !== "rgba(0, 0, 0, 0)" && rootBackground !== "transparent") {
    return rootBackground;
  }
  return "#ffffff";
}

function resolveSafeScale(element: HTMLElement, requestedScale: number, padding: number) {
  const width = element.scrollWidth + padding * 2;
  const height = element.scrollHeight + padding * 2;
  const largestSide = Math.max(width, height);
  const area = width * height;
  if (largestSide <= 0 || area <= 0) {
    return requestedScale;
  }

  const maxScale = Math.min(
    MAX_CANVAS_DIMENSION / largestSide,
    Math.sqrt(MAX_CANVAS_PIXEL_AREA / area),
  );
  if (maxScale < MIN_SCREENSHOT_SCALE) {
    throw new ConversationScreenshotTooLargeError();
  }
  return Math.min(requestedScale, maxScale);
}

function captureAbortError(signal: AbortSignal) {
  return signal.reason instanceof Error ? signal.reason : new DOMException("Screenshot capture aborted", "AbortError");
}

function throwIfCaptureAborted(signal?: AbortSignal) {
  if (signal?.aborted) {
    throw captureAbortError(signal);
  }
}

export function isScreenshotCaptureAbort(error: unknown) {
  return error instanceof DOMException && error.name === "AbortError";
}

function yieldToMainThread() {
  const scheduler = (globalThis as typeof globalThis & { scheduler?: YieldingScheduler }).scheduler;
  if (scheduler?.yield) {
    return scheduler.yield();
  }
  return new Promise<void>((resolve) => {
    window.setTimeout(resolve, 0);
  });
}

function stopCaptureContext(context: Context<HTMLElement>, error?: Error) {
  if (error) {
    context.requests.forEach((request) => {
      request.reject?.(error);
    });
  }
  context.workers.forEach((worker) => {
    worker.terminate();
  });
}

function screenshotWorkerURL(element: HTMLElement) {
  return new URL(screenshotWorkerPath, element.ownerDocument.baseURI).href;
}

export function loadConversationScreenshotRenderer(): Promise<ScreenshotRenderer> {
  screenshotRendererPromise ??= import("modern-screenshot").catch((error) => {
    screenshotRendererPromise = null;
    throw error;
  });
  return screenshotRendererPromise;
}

export async function captureElementToPngBlob(
  element: HTMLElement,
  { signal }: CaptureElementOptions = {},
): Promise<Blob> {
  throwIfCaptureAborted(signal);
  const { createContext, destroyContext, domToBlob } = await loadConversationScreenshotRenderer();
  throwIfCaptureAborted(signal);

  const padding = SCREENSHOT_PADDING;
  const safeScale = resolveSafeScale(element, SCREENSHOT_SCALE, padding);
  const width = element.scrollWidth + padding * 2;
  const height = element.scrollHeight + padding * 2;
  const scheduling = {
    clonedNodeCount: 0,
    lastMainThreadYieldAt: performance.now(),
  };
  const options = {
    scale: safeScale,
    width,
    height,
    backgroundColor: resolveCaptureBackgroundColor(element),
    maximumCanvasSize: MAX_CANVAS_DIMENSION,
    features: {
      copyScrollbar: false,
      removeAbnormalAttributes: false,
    },
    font: {
      preferredFormat: "woff2",
    },
    includeStyleProperties: SCREENSHOT_STYLE_PROPERTIES,
    onCloneEachNode: () => {
      throwIfCaptureAborted(signal);
      scheduling.clonedNodeCount += 1;
      if (scheduling.clonedNodeCount % SCREENSHOT_CLONE_YIELD_INTERVAL !== 0) {
        return;
      }
      const now = performance.now();
      if (now - scheduling.lastMainThreadYieldAt < SCREENSHOT_CLONE_FRAME_BUDGET_MS) {
        return;
      }
      scheduling.lastMainThreadYieldAt = now;
      return yieldToMainThread();
    },
    workerUrl: screenshotWorkerURL(element),
    workerNumber: SCREENSHOT_RESOURCE_WORKER_COUNT,
    style: {
      boxSizing: "content-box",
      padding: `${padding}px`,
      margin: "0",
    },
    filter: (node) => {
      throwIfCaptureAborted(signal);
      return !(node instanceof HTMLElement) || !node.matches(SCREENSHOT_OMIT_SELECTOR);
    },
  } satisfies Options;

  const context = await createContext(element, options);
  let rejectOnAbort: ((error: Error) => void) | null = null;
  const aborted = new Promise<never>((_resolve, reject) => {
    rejectOnAbort = reject;
  });
  const abortCapture = () => {
    const error = captureAbortError(signal as AbortSignal);
    stopCaptureContext(context, error);
    rejectOnAbort?.(error);
  };
  signal?.addEventListener("abort", abortCapture, { once: true });

  try {
    throwIfCaptureAborted(signal);
    const capture = domToBlob(context);
    const blob = signal ? await Promise.race([capture, aborted]) : await capture;
    if (!blob) {
      throw new Error("Failed to generate screenshot");
    }
    return blob;
  } finally {
    signal?.removeEventListener("abort", abortCapture);
    rejectOnAbort = null;
    stopCaptureContext(context);
    destroyContext(context);
  }
}
