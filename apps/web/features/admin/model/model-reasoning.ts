import type {
  AdminLLMModelReasoningFormat,
  AdminLLMModelReasoningLevel,
} from "@/features/admin/api/llm-types";
import { parseCapabilitiesObject, stringifyCapabilitiesObject } from "@/features/admin/model/capabilities-json";
import { isRecord } from "@/shared/lib/type-guards";

// Canonical order; keep in sync with the backend (domain/channel/model_reasoning.go).
export const REASONING_LEVELS = ["none", "minimal", "low", "medium", "high", "xhigh", "max"] as const satisfies readonly AdminLLMModelReasoningLevel[];

const REASONING_FORMATS = [
  "openai",
  "anthropic_effort",
  "anthropic_budget",
  "gemini_level",
  "gemini_budget",
  "qwen",
  "toggle",
] as const satisfies readonly AdminLLMModelReasoningFormat[];

const REASONING_KEY = "reasoning";

/** Explicit `reasoning` declaration in the capabilities JSON (the same shape the backend validates). */
export type ModelReasoningDeclaration = {
  format: AdminLLMModelReasoningFormat;
  levels: AdminLLMModelReasoningLevel[];
  default: AdminLLMModelReasoningLevel;
  budgets?: Record<string, number>;
};

export function isReasoningLevel(value: unknown): value is AdminLLMModelReasoningLevel {
  return REASONING_LEVELS.some((level) => level === value);
}

function isReasoningFormat(value: unknown): value is AdminLLMModelReasoningFormat {
  return REASONING_FORMATS.some((format) => format === value);
}

export function sortReasoningLevels(levels: readonly AdminLLMModelReasoningLevel[]): AdminLLMModelReasoningLevel[] {
  return REASONING_LEVELS.filter((level) => levels.includes(level));
}

/**
 * Default level when the administrator has not picked one, mirroring the backend catalog rule:
 * medium, else the highest non-none level below medium, else the lowest non-none level.
 */
export function fallbackReasoningDefault(levels: readonly AdminLLMModelReasoningLevel[]): AdminLLMModelReasoningLevel | null {
  const sorted = sortReasoningLevels(levels);
  const mediumRank = REASONING_LEVELS.indexOf("medium");
  const belowMedium = sorted.filter((level) => level !== "none" && REASONING_LEVELS.indexOf(level) <= mediumRank);
  return belowMedium.at(-1) ?? sorted.find((level) => level !== "none") ?? sorted[0] ?? null;
}

/**
 * Upstream parameter value a level is translated to, mirroring the backend translation
 * (application/conversation/model_option_reasoning.go): level formats send the level name, budget formats
 * send the token budget (`{ budget }`), and switches send enabled / disabled.
 */
export function reasoningNativeValue(
  format: AdminLLMModelReasoningFormat,
  level: AdminLLMModelReasoningLevel,
  budgets: Readonly<Record<string, number>> | null | undefined,
): string | { budget: number } {
  switch (format) {
    case "openai":
    case "gemini_level":
      return level;
    case "anthropic_effort":
      return level === "none" ? "disabled" : level;
    case "gemini_budget":
      return { budget: level === "none" ? 0 : (budgets?.[level] ?? 0) };
    case "anthropic_budget":
      return level === "none" ? "disabled" : { budget: budgets?.[level] ?? 0 };
    case "qwen": {
      const budget = budgets?.[level];
      return level === "none" ? "disabled" : typeof budget === "number" ? { budget } : "enabled";
    }
    case "toggle":
      return level === "none" ? "disabled" : "enabled";
  }
}

/**
 * Explicit reasoning declaration, or null when the model follows automatic detection.
 * A declaration that is present but malformed is reported as `invalid` (the backend then disables reasoning).
 */
export function modelReasoningOverride(
  value: string | null | undefined,
): ModelReasoningDeclaration | "invalid" | null {
  const raw = parseCapabilitiesObject(value)?.[REASONING_KEY];
  if (raw === undefined || raw === null) {
    return null;
  }
  if (!isRecord(raw) || !isReasoningFormat(raw.format) || !Array.isArray(raw.levels)) {
    return "invalid";
  }
  const format = raw.format;
  const levels = sortReasoningLevels(raw.levels.filter(isReasoningLevel));
  const defaultLevel = isReasoningLevel(raw.default) && levels.includes(raw.default) ? raw.default : null;
  if (levels.length === 0 || !defaultLevel) {
    return "invalid";
  }
  const budgets = isRecord(raw.budgets)
    ? Object.fromEntries(Object.entries(raw.budgets).filter((entry): entry is [string, number] => typeof entry[1] === "number"))
    : undefined;
  return {
    format,
    levels,
    default: defaultLevel,
    ...(budgets ? { budgets } : {}),
  };
}

/**
 * Writes or removes the explicit reasoning declaration; null restores automatic detection.
 * Returns null when the capabilities JSON is not an object (the caller reports it).
 */
export function setModelReasoningInCapabilities(
  value: string | null | undefined,
  declaration: ModelReasoningDeclaration | null,
): string | null {
  const payload = parseCapabilitiesObject(value);
  if (!payload) {
    return null;
  }
  if (declaration === null) {
    delete payload[REASONING_KEY];
  } else {
    const levels = sortReasoningLevels(declaration.levels);
    const source = declaration.budgets ?? {};
    const budgets = Object.fromEntries(levels.filter((level) => level in source).map((level) => [level, source[level]]));
    payload[REASONING_KEY] = {
      format: declaration.format,
      levels,
      default: declaration.default,
      ...(Object.keys(budgets).length > 0 ? { budgets } : {}),
    };
  }
  return stringifyCapabilitiesObject(payload);
}
