import type { ChatAreaMessage } from "@/features/chat/types/messages";

type ScrollAnchorMessage = Pick<ChatAreaMessage, "key" | "role" | "isPending" | "isStreaming">;

export function resolveLiveAnchorMessageKey(messages: ScrollAnchorMessage[]) {
  const liveMessageIndex = messages.findIndex((item) => item.isPending || item.isStreaming);
  if (liveMessageIndex < 0) {
    return "";
  }

  for (let index = liveMessageIndex; index >= 0; index -= 1) {
    const item = messages[index];
    if (item?.role === "user") {
      return item.key;
    }
  }

  return "";
}
