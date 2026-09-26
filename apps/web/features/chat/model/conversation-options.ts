import type { ConversationOptions } from "@/shared/api/conversation.types";

const RESERVED_CONVERSATION_OPTION_KEYS = new Set([
  "contents",
  "instructions",
  "input",
  "messages",
  "model",
  "prompt",
  "stream",
  "system",
  "systemInstruction",
]);

export function isConversationOptionsObject(value: unknown): value is ConversationOptions {
  return value !== null && typeof value === "object" && !Array.isArray(value);
}

export function isReservedConversationOptionKey(key: string): boolean {
  return RESERVED_CONVERSATION_OPTION_KEYS.has(key);
}

export function sanitizeConversationOptions(options: ConversationOptions): ConversationOptions {
  return Object.fromEntries(
    Object.entries(options).filter(([key]) => !isReservedConversationOptionKey(key)),
  );
}

export function cloneConversationOptions(options: ConversationOptions): ConversationOptions {
  try {
    return sanitizeConversationOptions(JSON.parse(JSON.stringify(options)) as ConversationOptions);
  } catch {
    return sanitizeConversationOptions({ ...options });
  }
}
