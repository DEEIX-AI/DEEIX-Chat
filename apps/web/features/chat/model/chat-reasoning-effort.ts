import { isReasoningEffortLevel, type ReasoningEffortLevel } from "@/entities/model";

/** Normalizes the effective level recorded on an assistant message; null when absent or unknown. */
export function normalizeMessageReasoningEffort(value: unknown): ReasoningEffortLevel | null {
  const normalized = typeof value === "string" ? value.trim() : "";
  return isReasoningEffortLevel(normalized) ? normalized : null;
}
