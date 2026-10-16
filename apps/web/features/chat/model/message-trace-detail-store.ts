import type { ChatMessageProcessTrace } from "@/features/chat/types/messages";

/** Full trace of a message whose list entry only carries a summary. */
export type MessageTraceDetail =
  | { status: "loading" }
  | { status: "failed" }
  | { status: "loaded"; trace: ChatMessageProcessTrace | undefined };

// Keyed by message public ID. A finished run's trace never changes, so entries outlive list reloads.
const details = new Map<string, MessageTraceDetail>();
const listeners = new Set<() => void>();

export function readMessageTraceDetail(messagePublicID: string): MessageTraceDetail | undefined {
  return details.get(messagePublicID);
}

export function writeMessageTraceDetail(messagePublicID: string, detail: MessageTraceDetail) {
  details.set(messagePublicID, detail);
  for (const listener of listeners) {
    listener();
  }
}

export function subscribeMessageTraceDetails(listener: () => void) {
  listeners.add(listener);
  return () => {
    listeners.delete(listener);
  };
}
