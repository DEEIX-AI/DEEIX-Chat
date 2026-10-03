import type { PublicModelReasoningDTO } from "@/shared/api/model-types";
import { isOneOf, isRecord } from "@/shared/lib/type-guards";

// Canonical reasoning effort levels, ordered from lowest to highest. The backend maps the
// selected level to each upstream protocol's native form; the frontend only uses this set.
// The level union comes from the generated API contract; this runtime list must enumerate it in order.
export type ReasoningEffortLevel = PublicModelReasoningDTO["default"];

const REASONING_EFFORT_LEVELS = [
  "none",
  "minimal",
  "low",
  "medium",
  "high",
  "xhigh",
  "max",
] as const satisfies readonly ReasoningEffortLevel[];

export const isReasoningEffortLevel = isOneOf(REASONING_EFFORT_LEVELS);

// Normalized reasoning capability served with the public model list; `locked` means the
// administrator pinned the level to `default`. The composer uses the reasoning model control;
// this capability only drives "Regenerate with…".
export type ModelReasoningCapability = PublicModelReasoningDTO;

const isModelReasoningCapabilitySource = isOneOf(
  ["explicit", "inferred", "catalog"] as const satisfies readonly ModelReasoningCapability["source"][],
);

function normalizeOptionPath(value: unknown): string | null {
  if (typeof value !== "string") {
    return null;
  }
  const path = value
    .split(".")
    .map((segment) => segment.trim())
    .filter(Boolean)
    .join(".");
  return path || null;
}

// Parses the normalized reasoning capability of a public model DTO. Unknown levels are dropped;
// an empty level set or a malformed payload yields null (no reasoning support).
export function parseModelReasoningCapability(value: unknown): ModelReasoningCapability | null {
  if (!isRecord(value) || !Array.isArray(value.levels)) {
    return null;
  }
  const declared = new Set(value.levels.map((item) => (typeof item === "string" ? item.trim() : "")));
  const levels = REASONING_EFFORT_LEVELS.filter((level) => declared.has(level));
  if (levels.length === 0) {
    return null;
  }
  const rawDefault = typeof value.default === "string" ? value.default.trim() : "";
  const defaultLevel = isReasoningEffortLevel(rawDefault) && levels.includes(rawDefault) ? rawDefault : levels[0];
  return {
    levels,
    default: defaultLevel,
    controlPath: normalizeOptionPath(value.controlPath),
    // Older backends omit the source; their capabilities are always administrator-configured.
    source: isModelReasoningCapabilitySource(value.source) ? value.source : "explicit",
    locked: value.locked === true,
  };
}
