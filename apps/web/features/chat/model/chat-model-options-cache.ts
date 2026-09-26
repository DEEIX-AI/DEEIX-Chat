import {
  isConversationOptionsObject,
  sanitizeConversationOptions,
} from "@/features/chat/model/conversation-options";
import type { ConversationOptions } from "@/shared/api/conversation.types";

const MODEL_OPTIONS_STORAGE_PREFIX = "deeix-chat:chat-model-options:";

function modelOptionsStorageKey(platformModelName: string): string {
  return `${MODEL_OPTIONS_STORAGE_PREFIX}${encodeURIComponent(platformModelName)}`;
}

export function readCachedModelOptions(platformModelName: string): ConversationOptions | null {
  if (typeof window === "undefined") {
    return null;
  }
  try {
    const raw = window.localStorage.getItem(modelOptionsStorageKey(platformModelName));
    if (!raw) {
      return null;
    }
    const parsed = JSON.parse(raw) as unknown;
    return isConversationOptionsObject(parsed) ? sanitizeConversationOptions(parsed) : null;
  } catch {
    return null;
  }
}

export function writeCachedModelOptions(
  platformModelName: string,
  options: ConversationOptions,
): void {
  if (typeof window === "undefined") {
    return;
  }
  try {
    window.localStorage.setItem(
      modelOptionsStorageKey(platformModelName),
      JSON.stringify(sanitizeConversationOptions(options)),
    );
  } catch {
    // localStorage may be unavailable in private browsing or strict environments.
  }
}

export function removeCachedModelOptions(platformModelName: string): void {
  if (typeof window === "undefined") {
    return;
  }
  try {
    window.localStorage.removeItem(modelOptionsStorageKey(platformModelName));
  } catch {
    // localStorage may be unavailable in private browsing or strict environments.
  }
}
