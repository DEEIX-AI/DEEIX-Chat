import { CHAT_CONTROLS_OPTION_KEY } from "@/features/chat/model/chat-model-controls";
import type { ConversationOptions } from "@/shared/api/conversation-types";
import { isReasoningEffortLevel, MODEL_CONTROL_REASONING_ID, type ReasoningEffortLevel } from "@/entities/model";

// Levels offered by the "Regenerate with…" menu (composer model, not locked).
export type ChatRegenerateReasoningOptions = {
  levels: ReasoningEffortLevel[];
  defaultLevel: ReasoningEffortLevel;
};

/** Normalizes the effective level recorded on an assistant message; null when absent or unknown. */
export function normalizeMessageReasoningEffort(value: unknown): ReasoningEffortLevel | null {
  const normalized = typeof value === "string" ? value.trim() : "";
  return isReasoningEffortLevel(normalized) ? normalized : null;
}

/** Single-request override used by "Regenerate with…"; the composer state is left untouched. */
export function withChatReasoningEffortOverride(
  options: ConversationOptions,
  level: ReasoningEffortLevel,
): ConversationOptions {
  return { ...options, [CHAT_CONTROLS_OPTION_KEY]: { [MODEL_CONTROL_REASONING_ID]: level } };
}
