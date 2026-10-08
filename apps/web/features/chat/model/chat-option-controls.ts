import {
  MODEL_CONTROL_TOGGLE_OFF,
  MODEL_CONTROL_TOGGLE_ON,
  type ModelControl,
  type ModelControlValue,
} from "@/entities/model";
import type { ChatModelControlSelections } from "@/features/chat/model/chat-model-controls";
import {
  getOptionAtPath,
  isReservedConversationOptionKey,
  optionPathSegments,
  removeOptionAtPath,
  setOptionAtPath,
} from "@/features/chat/model/conversation-options";
import type { ModelOptionControl } from "@/features/chat/types/chat-runtime";
import type { ConversationOptions } from "@/shared/api/conversation-types";

// Media tasks (image, video) have no server-side model controls: their parameters are the
// administrator's optionControls, written straight into the request options. These helpers present
// them as composer controls, so media models get the same parameters popover and pinning as chat.

/** Parameters whose values are numbers even when they are picked from a list of options. */
export const NUMBER_OPTION_KEYS = new Set([
  "budget_tokens",
  "candidate_count",
  "duration",
  "frequency_penalty",
  "generationConfig.candidateCount",
  "generationConfig.frequencyPenalty",
  "generationConfig.logprobs",
  "generationConfig.maxOutputTokens",
  "generationConfig.presencePenalty",
  "generationConfig.seed",
  "generationConfig.thinkingConfig.thinkingBudget",
  "generationConfig.topK",
  "logprobs",
  "max_completion_tokens",
  "max_output_tokens",
  "max_tokens",
  "n",
  "output_compression",
  "partial_images",
  "presence_penalty",
  "seed",
  "temperature",
  "thinking.budget_tokens",
  "thinking.thinking_budget",
  "thinking.thinkingBudget",
  "thinkingConfig.thinkingBudget",
  "top_k",
  "top_p",
]);

/** Built-in choices of well-known parameters, used when the administrator lists none. */
export const OPTION_SELECT_VALUES: Record<string, string[]> = {
  cache_timeout: ["5m", "1h"],
  effort: ["low", "medium", "high", "xhigh", "max"],
  service_tier: ["default", "priority", "flex"],
  speed: ["fast"],
  "generationConfig.mediaResolution": ["MEDIA_RESOLUTION_UNSPECIFIED", "MEDIA_RESOLUTION_LOW", "MEDIA_RESOLUTION_MEDIUM", "MEDIA_RESOLUTION_HIGH"],
  "generationConfig.responseModalities": ["TEXT", "IMAGE"],
  "generationConfig.responseMimeType": ["text/plain", "application/json", "text/x.enum"],
  "generationConfig.imageConfig.aspectRatio": ["1:1", "2:3", "3:2", "3:4", "4:3", "4:5", "5:4", "9:16", "16:9", "21:9"],
  "generationConfig.imageConfig.imageSize": ["1K", "2K", "4K"],
  background: ["auto", "opaque", "transparent"],
  "generationConfig.thinkingConfig.thinkingLevel": ["low", "medium", "high"],
  "imageConfig.aspectRatio": ["1:1", "2:3", "3:2", "3:4", "4:3", "4:5", "5:4", "9:16", "16:9", "21:9"],
  "imageConfig.imageSize": ["1K", "2K", "4K"],
  input_fidelity: ["low", "high"],
  moderation: ["auto", "low"],
  "output_config.effort": ["low", "medium", "high"],
  "output_config.format.type": ["json_object", "json_schema", "text"],
  output_format: ["png", "jpeg", "webp"],
  quality: ["auto", "low", "medium", "high", "standard", "hd"],
  "reasoning.effort": ["low", "medium", "high"],
  "reasoning.summary": ["auto", "concise", "detailed"],
  reasoning_effort: ["minimal", "low", "medium", "high", "xhigh"],
  reasoning_summary: ["auto", "concise", "detailed"],
  response_format: ["url", "b64_json"],
  "response_format.type": ["json_object", "json_schema", "text"],
  resolution: ["1k", "2k"],
  size: ["auto", "1024x1024", "1024x1536", "1536x1024", "2048x2048", "2048x1152", "3840x2160", "2160x3840"],
  aspect_ratio: ["1:1", "2:3", "3:2", "3:4", "4:3", "4:5", "5:4", "9:16", "16:9", "21:9"],
  aspectRatio: ["1:1", "2:3", "3:2", "3:4", "4:3", "4:5", "5:4", "9:16", "16:9", "21:9"],
  image_size: ["1K", "2K", "4K"],
  imageSize: ["1K", "2K", "4K"],
  "thinking.display": ["summarized", "omitted"],
  "thinking.thinking_level": ["low", "medium", "high"],
  "thinking.thinkingLevel": ["low", "medium", "high"],
  "thinking.type": ["enabled", "adaptive", "disabled"],
  "thinkingConfig.thinkingLevel": ["low", "medium", "high"],
  "tool_config.functionCallingConfig.mode": ["AUTO", "ANY", "NONE"],
  "toolConfig.functionCallingConfig.mode": ["AUTO", "ANY", "NONE"],
  "tool_choice.type": ["auto", "any", "none"],
  verbosity: ["low", "medium", "high"],
};

/** Nested parameters shown as fields of their own; other nested values stay in the JSON. */
export const NESTED_VISUAL_OPTION_PATHS = [
  ["thinking", "type"],
  ["thinking", "budget_tokens"],
  ["thinking", "include_thoughts"],
  ["thinking", "thinking_budget"],
  ["thinking", "thinking_level"],
  ["thinking", "includeThoughts"],
  ["thinking", "thinkingBudget"],
  ["thinking", "thinkingLevel"],
  ["thinking", "display"],
  ["thinkingConfig", "includeThoughts"],
  ["thinkingConfig", "thinkingBudget"],
  ["thinkingConfig", "thinkingLevel"],
  ["reasoning", "effort"],
  ["reasoning", "summary"],
  ["output_config", "effort"],
  ["output_config", "format", "type"],
  ["response_format", "type"],
  ["tool_choice", "type"],
  ["generationConfig", "maxOutputTokens"],
  ["generationConfig", "temperature"],
  ["generationConfig", "topP"],
  ["generationConfig", "topK"],
  ["generationConfig", "candidateCount"],
  ["generationConfig", "seed"],
  ["generationConfig", "presencePenalty"],
  ["generationConfig", "frequencyPenalty"],
  ["generationConfig", "responseLogprobs"],
  ["generationConfig", "logprobs"],
  ["generationConfig", "responseModalities"],
  ["generationConfig", "responseMimeType"],
  ["generationConfig", "mediaResolution"],
  ["generationConfig", "imageConfig", "aspectRatio"],
  ["generationConfig", "imageConfig", "imageSize"],
  ["imageConfig", "aspectRatio"],
  ["imageConfig", "imageSize"],
  ["generationConfig", "thinkingConfig", "includeThoughts"],
  ["generationConfig", "thinkingConfig", "thinkingBudget"],
  ["generationConfig", "thinkingConfig", "thinkingLevel"],
  ["tool_config", "functionCallingConfig", "mode"],
  ["toolConfig", "functionCallingConfig", "mode"],
];

function isNumericText(value: string): boolean {
  return value.trim() !== "" && Number.isFinite(Number(value));
}

/**
 * Composer controls for the administrator's option controls, by the rule the server applies to chat
 * models (compileLegacyModelControls): select with choices, boolean and number. Free-text controls
 * and selects without choices have no composer control and stay in the parameter dialog.
 */
export function modelControlsFromOptionControls(
  optionControls: readonly ModelOptionControl[],
  defaultOptions: ConversationOptions,
): ModelControl[] {
  const seen = new Set<string>();
  return optionControls.flatMap((source): ModelControl[] => {
    const path = optionPathSegments(source.path);
    const id = path.join(".");
    if (!id || seen.has(id) || isReservedConversationOptionKey(path[0] ?? "")) {
      return [];
    }
    const values = Array.from(new Set((source.options ?? []).map((item) => item.trim()).filter(Boolean)));
    const type = source.type === "boolean"
      ? "toggle"
      : source.type === "number"
        ? "number"
        : source.type !== "text" && values.length > 0 ? "select" : null;
    if (!type) {
      return [];
    }
    seen.add(id);
    const defaultValue = getOptionAtPath(defaultOptions, path);
    const defaultText = typeof defaultValue === "boolean"
      ? (defaultValue ? MODEL_CONTROL_TOGGLE_ON : MODEL_CONTROL_TOGGLE_OFF)
      : typeof defaultValue === "string" || typeof defaultValue === "number" ? String(defaultValue).trim() : "";
    const options = type === "toggle" ? [MODEL_CONTROL_TOGGLE_ON, MODEL_CONTROL_TOGGLE_OFF] : type === "select" ? values : [];
    return [{
      id,
      kind: "generic",
      type,
      label: source.label?.trim() ?? "",
      description: source.description?.trim() ?? "",
      icon: "",
      placement: "menu",
      default: (type === "number" ? isNumericText(defaultText) : options.includes(defaultText)) ? defaultText : null,
      locked: source.locked === true,
      options: options.map((value) => ({ value, label: "", description: "" })),
      min: null,
      max: null,
      step: null,
      integer: false,
      protocols: [],
    }];
  });
}

/** Current values of the controls, read from the request options. */
export function optionControlSelections(
  controls: readonly ModelControl[],
  options: ConversationOptions,
): ChatModelControlSelections {
  const selections: ChatModelControlSelections = {};
  for (const control of controls) {
    const value = getOptionAtPath(options, optionPathSegments(control.id));
    if (typeof value === "boolean") {
      selections[control.id] = value ? MODEL_CONTROL_TOGGLE_ON : MODEL_CONTROL_TOGGLE_OFF;
    } else if (typeof value === "number" && Number.isFinite(value)) {
      selections[control.id] = control.type === "number" ? value : String(value);
    } else if (typeof value === "string" && value.trim()) {
      selections[control.id] = value.trim();
    }
  }
  return selections;
}

/** Options after the user picks a value; null restores the administrator default. */
export function applyOptionControlValue(
  options: ConversationOptions,
  defaultOptions: ConversationOptions,
  control: ModelControl,
  value: ModelControlValue | null,
): ConversationOptions {
  const path = optionPathSegments(control.id);
  const defaultValue = getOptionAtPath(defaultOptions, path);
  if (value === null) {
    return defaultValue === undefined ? removeOptionAtPath(options, path) : setOptionAtPath(options, path, defaultValue);
  }
  if (control.type === "toggle") {
    return setOptionAtPath(options, path, value === MODEL_CONTROL_TOGGLE_ON);
  }
  // Same rule as the parameter dialog: a list of numbers on a numeric parameter is sent as numbers.
  const numeric = control.type === "number" || (
    (NUMBER_OPTION_KEYS.has(control.id) || typeof defaultValue === "number") &&
    control.options.every((option) => isNumericText(option.value))
  );
  return setOptionAtPath(options, path, numeric ? Number(value) : String(value));
}

/**
 * Controls for parameters that are set (in the defaults or the current options) without a declared
 * option control, inferred the way the parameter dialog shows them: booleans, numbers, and strings
 * of parameters with built-in choices. `sources` lists the defaults first.
 */
export function inferOptionControls(
  sources: readonly ConversationOptions[],
  declared: readonly ModelOptionControl[],
): ModelOptionControl[] {
  const declaredPaths = new Set(declared.map((control) => optionPathSegments(control.path).join(".")));
  const found = new Map<string, unknown[]>();
  for (const source of sources) {
    for (const path of [...NESTED_VISUAL_OPTION_PATHS, ...Object.keys(source).map((key) => [key])]) {
      const id = path.join(".");
      const value = getOptionAtPath(source, path);
      if (declaredPaths.has(id) || isReservedConversationOptionKey(path[0] ?? "")) continue;
      if (typeof value === "boolean" || typeof value === "number" || typeof value === "string") {
        found.set(id, [...(found.get(id) ?? []), value]);
      }
    }
  }
  return Array.from(found).flatMap(([path, values]): ModelOptionControl[] => {
    if (typeof values[0] === "boolean") return [{ path, type: "boolean" }];
    if (typeof values[0] === "number" || NUMBER_OPTION_KEYS.has(path)) return [{ path, type: "number" }];
    const choices = OPTION_SELECT_VALUES[path];
    if (!choices) return [];
    // Values in use that the built-in list lacks (such as "auto") stay selectable, ahead of the list.
    const used = values.flatMap((value) => (typeof value === "string" && value.trim() ? [value.trim()] : []));
    return [{ path, type: "select", options: Array.from(new Set([...used.filter((value) => !choices.includes(value)), ...choices])) }];
  });
}
