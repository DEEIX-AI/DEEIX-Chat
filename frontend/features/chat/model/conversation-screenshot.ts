"use client";

import { domToBlob } from "modern-screenshot";

function safeFileNamePart(value: string) {
  const normalized = value
    .trim()
    .replace(/[\\/:*?"<>|]+/g, "-")
    .replace(/\s+/g, "-")
    .replace(/-+/g, "-")
    .replace(/^-|-$/g, "");
  return normalized || "conversation";
}

function formatScreenshotTimestamp(date = new Date()) {
  const pad = (part: number) => String(part).padStart(2, "0");
  return [
    date.getFullYear(),
    pad(date.getMonth() + 1),
    pad(date.getDate()),
    "-",
    pad(date.getHours()),
    pad(date.getMinutes()),
    pad(date.getSeconds()),
  ].join("");
}

export function resolveConversationScreenshotFileName(title: string) {
  return `conversation-${safeFileNamePart(title)}-${formatScreenshotTimestamp()}.png`;
}

function resolveCaptureBackgroundColor(element: HTMLElement) {
  const ownerWindow = element.ownerDocument?.defaultView ?? window;
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

const SCREENSHOT_SCALE = 2;
const SCREENSHOT_PADDING = 24;
const MAX_CANVAS_DIMENSION = 16384;
const MIN_SCREENSHOT_SCALE = 1;

export class ConversationScreenshotTooLargeError extends Error {
  constructor() {
    super("conversation_screenshot_too_large");
    this.name = "ConversationScreenshotTooLargeError";
  }
}

function resolveSafeScale(element: HTMLElement, requestedScale: number, padding: number) {
  const width = element.scrollWidth + padding * 2;
  const height = element.scrollHeight + padding * 2;
  const largestSide = Math.max(width, height);
  if (largestSide <= 0) {
    return requestedScale;
  }
  const maxScale = MAX_CANVAS_DIMENSION / largestSide;
  if (maxScale < MIN_SCREENSHOT_SCALE) {
    throw new ConversationScreenshotTooLargeError();
  }
  return Math.min(requestedScale, maxScale);
}

const SCREENSHOT_OMIT_SELECTOR = [
  "[data-screenshot-exclude='true']",
  ".chat-message-meta",
  ".chat-screenshot-omit",
  "[data-streamdown='code-block-actions']",
  "[data-streamdown='mermaid-block-actions']",
].join(",");

export async function captureElementToPngBlob(element: HTMLElement): Promise<Blob> {
  const padding = SCREENSHOT_PADDING;
  const safeScale = resolveSafeScale(element, SCREENSHOT_SCALE, padding);
  const width = element.scrollWidth + padding * 2;
  const height = element.scrollHeight + padding * 2;
  const blob = await domToBlob(element, {
    scale: safeScale,
    width,
    height,
    backgroundColor: resolveCaptureBackgroundColor(element),
    maximumCanvasSize: MAX_CANVAS_DIMENSION,
    features: {
      copyScrollbar: false,
    },
    style: {
      boxSizing: "content-box",
      padding: `${padding}px`,
      margin: "0",
    },
    filter: (node) => {
      if (!(node instanceof HTMLElement)) {
        return true;
      }
      return !node.matches(SCREENSHOT_OMIT_SELECTOR);
    },
  });
  if (!blob) {
    throw new Error("Failed to generate screenshot");
  }
  return blob;
}

export function downloadPngBlob(blob: Blob, fileName: string) {
  const url = URL.createObjectURL(blob);
  const link = document.createElement("a");
  link.href = url;
  link.download = fileName;
  link.rel = "noopener";
  document.body.appendChild(link);
  link.click();
  link.remove();
  URL.revokeObjectURL(url);
}

export function isClipboardImageWriteSupported() {
  return (
    typeof navigator !== "undefined" &&
    typeof navigator.clipboard?.write === "function" &&
    typeof ClipboardItem !== "undefined"
  );
}

export async function copyPngBlobToClipboard(blob: Blob) {
  if (!isClipboardImageWriteSupported()) {
    throw new Error("Clipboard image write is not supported");
  }
  const pngBlob = blob.type === "image/png" ? blob : new Blob([blob], { type: "image/png" });
  await navigator.clipboard.write([new ClipboardItem({ "image/png": pngBlob })]);
}
