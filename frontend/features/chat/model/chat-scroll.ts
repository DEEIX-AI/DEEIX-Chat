import type { ChatAreaMessage } from "@/features/chat/types/messages";

type ScrollAnchorMessage = Pick<ChatAreaMessage, "key" | "role" | "isPending" | "isStreaming">;

export const CHAT_SEND_SCROLL_DURATION_MS = 700;

type RequestAnimationFrame = (callback: (timestamp: number) => void) => number;
type CancelAnimationFrame = (frameID: number) => void;
type ChatScrollViewport = Pick<HTMLElement, "scrollTop" | "scrollHeight" | "clientHeight">;

function easeInOutCubic(progress: number) {
  return progress < 0.5
    ? 4 * progress * progress * progress
    : 1 - (-2 * progress + 2) ** 3 / 2;
}

export function animateChatScrollToBottom(
  viewport: ChatScrollViewport,
  requestFrame: RequestAnimationFrame,
  cancelFrame: CancelAnimationFrame,
  durationMS = CHAT_SEND_SCROLL_DURATION_MS,
) {
  const startTop = viewport.scrollTop;
  const initialTargetTop = Math.max(0, viewport.scrollHeight - viewport.clientHeight);
  if (initialTargetTop <= startTop) {
    viewport.scrollTop = initialTargetTop;
    return () => undefined;
  }

  let animationFrameID = 0;
  let startTimestamp: number | null = null;
  let cancelled = false;
  const step = (timestamp: number) => {
    if (cancelled) {
      return;
    }
    if (startTimestamp === null) {
      startTimestamp = timestamp;
    }

    const progress = Math.min(1, Math.max(0, (timestamp - startTimestamp) / durationMS));
    const targetTop = Math.max(0, viewport.scrollHeight - viewport.clientHeight);
    viewport.scrollTop = startTop + (targetTop - startTop) * easeInOutCubic(progress);
    if (progress < 1) {
      animationFrameID = requestFrame(step);
    }
  };

  animationFrameID = requestFrame(step);
  return () => {
    cancelled = true;
    if (animationFrameID > 0) {
      cancelFrame(animationFrameID);
    }
  };
}

export function schedulePendingUserScroll(
  requestFrame: RequestAnimationFrame,
  cancelFrame: CancelAnimationFrame,
  scroll: () => void,
) {
  let secondFrameID: number | null = null;
  const firstFrameID = requestFrame(() => {
    secondFrameID = requestFrame(() => {
      scroll();
    });
  });

  return () => {
    cancelFrame(firstFrameID);
    if (secondFrameID !== null) {
      cancelFrame(secondFrameID);
    }
  };
}

export function resolveLiveAnchorMessageKey(messages: ScrollAnchorMessage[]) {
  const liveMessageIndex = messages.findIndex((item) => item.isPending || item.isStreaming);
  if (liveMessageIndex < 0) {
    return "";
  }

  for (let index = liveMessageIndex - 1; index >= 0; index -= 1) {
    const item = messages[index];
    if (item?.role === "user") {
      return item.key;
    }
  }

  return "";
}

export function resolvePendingUserScrollKey(messages: ScrollAnchorMessage[]) {
  for (let index = messages.length - 1; index >= 0; index -= 1) {
    const item = messages[index];
    if (item?.role === "user" && item.isPending) {
      return item.key;
    }
  }

  return "";
}
