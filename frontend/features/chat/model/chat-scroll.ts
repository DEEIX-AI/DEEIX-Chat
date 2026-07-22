import type { ChatAreaMessage } from "@/features/chat/types/messages";

type ScrollAnchorMessage = Pick<ChatAreaMessage, "key" | "role" | "isPending" | "isStreaming">;

export const PENDING_USER_SCROLL_OPTIONS = { behavior: "smooth" } as const;

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
