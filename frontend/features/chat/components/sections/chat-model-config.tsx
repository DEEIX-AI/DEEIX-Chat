"use client";

import * as React from "react";
import { CircleHelp } from "lucide-react";
import { useTranslations } from "next-intl";
import { toast } from "sonner";

import { Cog } from "@/components/animate-ui/icons/cog";
import { Button } from "@/components/ui/button";
import { Checkbox } from "@/components/ui/checkbox";
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from "@/components/ui/dialog";
import { Input } from "@/components/ui/input";
import { InputGroupButton } from "@/components/ui/input-group";
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from "@/components/ui/select";
import { Tabs, TabsList, TabsTrigger } from "@/components/ui/tabs";
import { Tooltip, TooltipContent, TooltipTrigger } from "@/components/ui/tooltip";
import {
  isReservedConversationOptionKey,
  sanitizeConversationOptions,
} from "@/features/chat/model/conversation-options";
import { cn } from "@/lib/utils";
import type { ModelOptionControl } from "@/features/chat/types/chat-runtime";
import type { ConversationOptions } from "@/shared/api/conversation.types";
import { JsonCodeEditor } from "@/shared/components/json-code-editor";
import type { ModelOptionPolicy, NativeToolDefinition } from "@/shared/lib/model-option-policy";
import { isModelOptionPathFiltered, resolveModelOptionPolicyProtocol } from "@/shared/lib/model-option-policy";

type EditableOptionValue = string | number | boolean | null;
type VisualOptionKind = "boolean" | "number" | "select" | "text";

type VisualOption = {
  key: string;
  path: string[];
  value: EditableOptionValue;
  label?: string;
  description?: string;
  kind?: VisualOptionKind;
  selectValues?: string[];
  placeholder?: string;
  active: boolean;
};

type FilteredOption = {
  key: string;
  value: unknown;
};

type ModelOptionFilterStatus = "inactive" | "passed" | "filtered" | "unknown";

type NativeToolVisualOption = {
  definition: NativeToolDefinition;
  protocols: string[];
  protocolMatched: boolean;
};

type ChatModelConfigProps = {
  disabled: boolean;
  options: ConversationOptions;
  defaultOptions: ConversationOptions;
  optionControls: ModelOptionControl[];
  nativeToolKeys: string[];
  modelOptionPolicy: ModelOptionPolicy | null;
  selectedProtocol: string;
  selectedModelName: string;
  onOptionsChange: React.Dispatch<React.SetStateAction<ConversationOptions>>;
  onOptionsReset: (defaults?: ConversationOptions) => void;
  onDefaultOptionsRestore: () => Promise<ConversationOptions | null>;
};

type OptionTranslationResolver = ((key: string) => string) & {
  has?: (key: string) => boolean;
};

const OPTION_LABEL_KEYS = new Set<string>([
  "budget_tokens",
  "cache_timeout",
  "candidate_count",
  "effort",
  "enable_cache",
  "enable_thinking",
  "frequency_penalty",
  "generationConfig.candidateCount",
  "generationConfig.frequencyPenalty",
  "generationConfig.imageConfig.aspectRatio",
  "generationConfig.imageConfig.imageSize",
  "generationConfig.logprobs",
  "generationConfig.maxOutputTokens",
  "generationConfig.mediaResolution",
  "generationConfig.presencePenalty",
  "generationConfig.responseModalities",
  "generationConfig.responseLogprobs",
  "generationConfig.responseMimeType",
  "generationConfig.seed",
  "generationConfig.thinkingConfig.includeThoughts",
  "generationConfig.thinkingConfig.thinkingBudget",
  "generationConfig.thinkingConfig.thinkingLevel",
  "generationConfig.topK",
  "logprobs",
  "max_completion_tokens",
  "max_output_tokens",
  "max_tokens",
  "background",
  "input_fidelity",
  "moderation",
  "n",
  "output_compression",
  "output_format",
  "output_config.effort",
  "output_config.format.type",
  "partial_images",
  "quality",
  "resolution",
  "size",
  "presence_penalty",
  "reasoning.summary",
  "reasoning.effort",
  "reasoning_effort",
  "reasoning_summary",
  "response_format",
  "response_format.type",
  "response_logprobs",
  "seed",
  "service_tier",
  "speed",
  "temperature",
  "think",
  "thinking",
  "thinking_display",
  "thinking.budget_tokens",
  "thinking.display",
  "thinking.includeThoughts",
  "thinking.include_thoughts",
  "thinking.thinkingBudget",
  "thinking.thinkingLevel",
  "thinking.thinking_budget",
  "thinking.thinking_level",
  "thinking.type",
  "thinkingConfig.includeThoughts",
  "thinkingConfig.thinkingBudget",
  "thinkingConfig.thinkingLevel",
  "tool_config.functionCallingConfig.mode",
  "toolConfig.functionCallingConfig.mode",
  "tool_choice.type",
  "tool_choice",
  "top_k",
  "top_p",
  "verbosity",
  "web_search",
  "aspect_ratio",
  "aspectRatio",
  "image_size",
  "imageSize",
  "imageConfig.aspectRatio",
  "imageConfig.imageSize",
] as const);

const OPTION_DESCRIPTION_KEYS = new Set<string>([
  ...OPTION_LABEL_KEYS,
  "background",
  "input_fidelity",
  "moderation",
  "output_compression",
  "output_format",
  "partial_images",
  "quality",
  "size",
] as const);

const OPTION_ORDER = [
  "temperature",
  "top_p",
  "top_k",
  "generationConfig.topK",
  "candidate_count",
  "generationConfig.candidateCount",
  "seed",
  "generationConfig.seed",
  "presence_penalty",
  "generationConfig.presencePenalty",
  "frequency_penalty",
  "generationConfig.frequencyPenalty",
  "generationConfig.imageConfig.aspectRatio",
  "generationConfig.imageConfig.imageSize",
  "size",
  "quality",
  "background",
  "moderation",
  "output_format",
  "output_compression",
  "partial_images",
  "input_fidelity",
  "response_logprobs",
  "generationConfig.responseLogprobs",
  "logprobs",
  "generationConfig.logprobs",
  "generationConfig.responseModalities",
  "generationConfig.responseMimeType",
  "generationConfig.mediaResolution",
  "service_tier",
  "max_tokens",
  "speed",
  "enable_thinking",
  "thinking_display",
  "effort",
  "enable_cache",
  "cache_timeout",
  "thinking.type",
  "thinking.include_thoughts",
  "thinking.includeThoughts",
  "reasoning_effort",
  "reasoning.effort",
  "reasoning.summary",
  "reasoning_summary",
  "output_config.effort",
  "output_config.format.type",
  "response_format",
  "response_format.type",
  "resolution",
  "budget_tokens",
  "thinking.budget_tokens",
  "thinking.thinking_budget",
  "thinking.thinkingBudget",
  "thinking.thinking_level",
  "thinking.thinkingLevel",
  "thinking.display",
  "thinkingConfig.includeThoughts",
  "thinkingConfig.thinkingBudget",
  "thinkingConfig.thinkingLevel",
  "generationConfig.thinkingConfig.includeThoughts",
  "generationConfig.thinkingConfig.thinkingBudget",
  "generationConfig.thinkingConfig.thinkingLevel",
  "max_output_tokens",
  "max_completion_tokens",
  "generationConfig.maxOutputTokens",
  "verbosity",
  "tool_config.functionCallingConfig.mode",
  "toolConfig.functionCallingConfig.mode",
  "tool_choice.type",
  "tool_choice",
  "thinking",
  "think",
  "web_search",
  "aspect_ratio",
  "aspectRatio",
  "image_size",
  "imageSize",
  "imageConfig.aspectRatio",
  "imageConfig.imageSize",
];

const NUMBER_OPTION_KEYS = new Set([
  "budget_tokens",
  "candidate_count",
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

const OPTION_SELECT_VALUES: Record<string, string[]> = {
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

const NESTED_VISUAL_OPTION_PATHS = [
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

const PROTOCOL_LABELS: Record<string, string> = {
  anthropic_messages: "Messages",
  fal_queue: "Queue",
  google_generate_content: "Generate Content",
  google_image_generation: "Image Generation",
  openai_chat_completions: "Chat Completions",
  openai_image_edits: "Images Edits",
  openai_image_generations: "Images Generations",
  openai_responses: "Responses",
  openai_video_generations: "Video Generations",
  replicate_predictions: "Predictions",
  stability_ai_generate: "Image Generation",
  xai_image: "Images Generations",
  xai_image_edits: "Images Edits",
  xai_responses: "xAI Responses",
};

function stringifyOptions(value: ConversationOptions): string {
  if (Object.keys(value).length === 0) {
    return "";
  }
  return JSON.stringify(value, null, 2);
}

function parseOptionsDraft(value: string): {
  filteredOptions: FilteredOption[];
  options: ConversationOptions | null;
  rawOptions: ConversationOptions | null;
  error: string;
} {
  try {
    const parsed = JSON.parse(value.trim() || "{}") as unknown;
    if (parsed === null || Array.isArray(parsed) || typeof parsed !== "object") {
      return { filteredOptions: [], options: null, rawOptions: null, error: "JSON must be an object" };
    }
    const rawOptions = parsed as ConversationOptions;
    return {
      filteredOptions: Object.entries(rawOptions)
        .filter(([key]) => isReservedConversationOptionKey(key))
        .map(([key, optionValue]) => ({ key, value: optionValue })),
      options: sanitizeConversationOptions(rawOptions),
      rawOptions,
      error: "",
    };
  } catch {
    return { filteredOptions: [], options: null, rawOptions: null, error: "" };
  }
}

function isEditableOptionValue(value: unknown): value is EditableOptionValue {
  return value === null || ["string", "number", "boolean"].includes(typeof value);
}

function isPlainOptionObject(value: unknown): value is Record<string, unknown> {
  return value !== null && typeof value === "object" && !Array.isArray(value);
}

function providerToolObjectsFromOptions(options: ConversationOptions): Record<string, unknown>[] {
  const rawTools = options.tools;
  if (!Array.isArray(rawTools)) {
    return [];
  }
  return rawTools.filter(isPlainOptionObject);
}

function providerToolMatchesDefinition(tool: Record<string, unknown>, definition: NativeToolDefinition): boolean {
  if (tool.type === definition.type) {
    return true;
  }
  return Object.keys(definition.payload ?? {}).some((key) => key !== "type" && Object.prototype.hasOwnProperty.call(tool, key));
}

function hasProviderTool(options: ConversationOptions, definition: NativeToolDefinition): boolean {
  return providerToolObjectsFromOptions(options).some((tool) => providerToolMatchesDefinition(tool, definition));
}

function setProviderToolEnabled(
  options: ConversationOptions,
  toolOption: NativeToolDefinition,
  enabled: boolean,
): ConversationOptions {
  const type = toolOption.type;
  const tools = providerToolObjectsFromOptions(options);
  const hasTool = tools.some((tool) => providerToolMatchesDefinition(tool, toolOption));
  const nextTools = enabled
    ? hasTool
      ? tools
      : [...tools, { ...(toolOption.payload ?? { type }) }]
    : tools.filter((tool) => !providerToolMatchesDefinition(tool, toolOption));

  if (nextTools.length === 0) {
    const { tools: _tools, ...rest } = options;
    return rest;
  }

  return { ...options, tools: nextTools };
}

function optionPathKey(path: string[]): string {
  return path.join(".");
}

function getOptionAtPath(options: ConversationOptions, path: string[]): unknown {
  let current: unknown = options;
  for (const segment of path) {
    if (!isPlainOptionObject(current)) {
      return undefined;
    }
    current = current[segment];
  }
  return current;
}

function hasOptionAtPath(options: ConversationOptions, path: string[]): boolean {
  let current: unknown = options;
  for (const segment of path) {
    if (!isPlainOptionObject(current) || !Object.prototype.hasOwnProperty.call(current, segment)) {
      return false;
    }
    current = current[segment];
  }
  return true;
}

function setOptionAtPath(options: ConversationOptions, path: string[], value: unknown): ConversationOptions {
  if (path.length === 0) {
    return options;
  }
  const [segment, ...rest] = path;
  if (rest.length === 0) {
    return { ...options, [segment]: value };
  }
  const current = options[segment];
  return {
    ...options,
    [segment]: setOptionAtPath(isPlainOptionObject(current) ? current : {}, rest, value),
  };
}

function visualOptionsFromOptions(options: ConversationOptions): VisualOption[] {
  const nestedOptions = NESTED_VISUAL_OPTION_PATHS.flatMap((path): VisualOption[] => {
    if (isReservedConversationOptionKey(path[0] ?? "")) {
      return [];
    }
    const value = getOptionAtPath(options, path);
    if (!isEditableOptionValue(value)) {
      return [];
    }
    return [{ key: optionPathKey(path), path, value, active: true }];
  });
  const topLevelOptions = Object.entries(options).flatMap(([key, value]): VisualOption[] => {
    if (isReservedConversationOptionKey(key)) {
      return [];
    }
    if (isEditableOptionValue(value)) {
      return [{ key, path: [key], value, active: true }];
    }
    return [];
  });
  const merged = [...nestedOptions, ...topLevelOptions];
  const deduped = merged.filter((item, index) => merged.findIndex((candidate) => candidate.key === item.key) === index);
  return deduped.sort((left, right) => compareOptionKeys(left.key, right.key));
}

function optionPathFromControl(path: string): string[] {
  return path
    .split(".")
    .map((segment) => segment.trim())
    .filter(Boolean);
}

function resolveControlEditableValue(options: ConversationOptions, path: string[]): EditableOptionValue {
  const currentValue = getOptionAtPath(options, path);
  if (isEditableOptionValue(currentValue)) {
    return currentValue;
  }
  return null;
}

function normalizeControlSelectValues(values: string[] | undefined): string[] {
  if (!values) {
    return [];
  }
  return Array.from(new Set(values.map((item) => item.trim()).filter(Boolean)));
}

function resolveControlKind(control: ModelOptionControl): VisualOptionKind | undefined {
  if (control.type) {
    return control.type;
  }
  if (normalizeControlSelectValues(control.options).length > 0) {
    return "select";
  }
  return undefined;
}

function visualOptionsFromControls(
  controls: ModelOptionControl[],
  options: ConversationOptions,
): VisualOption[] {
  return controls.flatMap((control): VisualOption[] => {
    const path = optionPathFromControl(control.path);
    if (path.length === 0 || isReservedConversationOptionKey(path[0] ?? "")) {
      return [];
    }
    const key = optionPathKey(path);
    const value = resolveControlEditableValue(options, path);
    const active = hasOptionAtPath(options, path);
    const selectValues = normalizeControlSelectValues(control.options);
    return [{
      key,
      path,
      value,
      active,
      label: control.label,
      description: control.description,
      kind: resolveControlKind(control),
      selectValues,
      placeholder: control.placeholder,
    }];
  });
}

function resolveOptionTitle(key: string, configuredLabel: string | undefined, translate: OptionTranslationResolver): string {
  const translationKey = key.replaceAll(".", "__");
  if (OPTION_LABEL_KEYS.has(key) && translate.has?.(translationKey)) {
    return translate(translationKey);
  }
  return configuredLabel?.trim() || key;
}

function resolveOptionDescription(key: string, description: string | undefined, translate: OptionTranslationResolver): string {
  const translationKey = key.replaceAll(".", "__");
  if (OPTION_DESCRIPTION_KEYS.has(key) && translate.has?.(translationKey)) {
    return translate(translationKey);
  }
  return description?.trim() || "";
}

function compareOptionKeys(a: string, b: string): number {
  const aIndex = OPTION_ORDER.indexOf(a);
  const bIndex = OPTION_ORDER.indexOf(b);
  if (aIndex >= 0 && bIndex >= 0) {
    return aIndex - bIndex;
  }
  if (aIndex >= 0) {
    return -1;
  }
  if (bIndex >= 0) {
    return 1;
  }
  return a.localeCompare(b);
}

function resolveOptionKind(key: string, value: EditableOptionValue): "boolean" | "number" | "select" | "text" {
  if (typeof value === "boolean") {
    return "boolean";
  }
  const selectValues = OPTION_SELECT_VALUES[key] ?? [];
  if (typeof value === "string" && selectValues.includes(value.trim())) {
    return "select";
  }
  if (typeof value === "number" || (value === null && NUMBER_OPTION_KEYS.has(key))) {
    return "number";
  }
  return "text";
}

function resolveSelectValues(key: string, configuredValues?: string[]): string[] {
  const sourceValues = configuredValues === undefined ? OPTION_SELECT_VALUES[key] : configuredValues;
  return Array.from(new Set((sourceValues ?? []).map((item) => item.trim()).filter(Boolean)));
}

function resolveModelOptionFilterStatus(
  policy: ModelOptionPolicy | null,
  protocol: string,
  path: string,
): ModelOptionFilterStatus {
  if (!policy) {
    return "unknown";
  }
  return isModelOptionPathFiltered({ policy, protocol, path }) ? "filtered" : "passed";
}

function ModelOptionFilterBadge({
  status,
  inactiveLabel,
  ignoredLabel,
  passedLabel,
}: {
  status: ModelOptionFilterStatus;
  inactiveLabel: string;
  ignoredLabel: string;
  passedLabel: string;
}) {
  if (status === "unknown") {
    return null;
  }
  return (
    <span
      data-filtered={status === "filtered"}
      data-inactive={status === "inactive"}
      className="shrink-0 rounded-md bg-emerald-500/10 px-1.5 py-0.5 text-[10px] leading-none text-emerald-700 data-[filtered=true]:bg-muted data-[filtered=true]:text-muted-foreground data-[inactive=true]:bg-muted data-[inactive=true]:text-muted-foreground"
    >
      {status === "inactive" ? inactiveLabel : status === "filtered" ? ignoredLabel : passedLabel}
    </span>
  );
}

function parseVisualNumberInput(value: string): number | string | null {
  const normalized = value.trim();
  if (!normalized) {
    return null;
  }
  if (/^[+-]?(?:\d+|\d*\.\d+)(?:e[+-]?\d+)?$/i.test(normalized)) {
    return Number(normalized);
  }
  return value;
}

function resolveProtocolLabel(protocol: string): string {
  return PROTOCOL_LABELS[protocol] ?? protocol;
}

function resolveNativeToolGroupTitle(provider: string, fallback: string, tComposer: (key: string) => string): string {
  switch (provider.trim().toLowerCase()) {
    case "anthropic":
      return tComposer("nativeTools.claude");
    case "google":
      return tComposer("nativeTools.google");
    case "openai":
      return tComposer("nativeTools.openai");
    case "xai":
      return tComposer("nativeTools.grok");
    default:
      return fallback;
  }
}

function nativeToolMessageKey(toolKey: string): string {
  return toolKey.replaceAll(".", "__");
}

function translateOptional(translate: (key: string) => string, key: string): string {
  try {
    const value = translate(key);
    return value.trim() ? value : "";
  } catch {
    return "";
  }
}

function resolveNativeToolLabel(tool: NativeToolDefinition, translate: (key: string) => string): string {
  return translateOptional(translate, nativeToolMessageKey(tool.toolKey)) || tool.label || tool.type || tool.toolKey;
}

function resolveNativeToolDescription(tool: NativeToolDefinition, translate: (key: string) => string): string {
  return translateOptional(translate, nativeToolMessageKey(tool.toolKey)) || tool.description || tool.type || tool.toolKey;
}

export function ChatModelConfig({
  disabled,
  options,
  defaultOptions,
  optionControls,
  nativeToolKeys,
  modelOptionPolicy,
  selectedProtocol,
  selectedModelName,
  onOptionsChange,
  onOptionsReset,
  onDefaultOptionsRestore,
}: ChatModelConfigProps) {
  const tCommon = useTranslations("common.actions");
  const tComposer = useTranslations("chat.composer");
  const tOptionLabels = useTranslations("chat.optionLabels");
  const tOptionDescriptions = useTranslations("chat.optionDescriptions");
  const tNativeToolLabels = useTranslations("chat.nativeToolLabels");
  const tNativeToolDescriptions = useTranslations("chat.nativeToolDescriptions");
  const [hovered, setHovered] = React.useState(false);
  const [dialogOpen, setDialogOpen] = React.useState(false);
  const [optionsDraft, setOptionsDraft] = React.useState("");
  const [optionsObject, setOptionsObject] = React.useState<ConversationOptions>({});
  const [filteredOptions, setFilteredOptions] = React.useState<FilteredOption[]>([]);
  const [mobileView, setMobileView] = React.useState<"json" | "visual">("json");
  const [defaultRestorePending, setDefaultRestorePending] = React.useState(false);
  const [restoredDefaultOptions, setRestoredDefaultOptions] = React.useState<ConversationOptions | null>(null);
  const optionsObjectRef = React.useRef<ConversationOptions>({});
  const effectiveDefaultOptions = restoredDefaultOptions ?? defaultOptions;
  const selectedProtocolLabel = selectedProtocol ? resolveProtocolLabel(selectedProtocol) : "";
  const configuredOptions = React.useMemo(
    () => visualOptionsFromControls(optionControls, optionsObject),
    [optionControls, optionsObject],
  );
  const configuredOptionKeys = React.useMemo(
    () => new Set(configuredOptions.map((item) => item.key)),
    [configuredOptions],
  );
  const editableOptions = React.useMemo(
    () => visualOptionsFromOptions(optionsObject).filter((item) => !configuredOptionKeys.has(item.key)),
    [configuredOptionKeys, optionsObject],
  );
  const nativeToolGroup = React.useMemo(() => {
    const policyProtocol = resolveModelOptionPolicyProtocol(selectedProtocol);
    const orderedKeys = Array.from(new Set(nativeToolKeys.map((key) => key.trim()).filter(Boolean)));
    if (orderedKeys.length === 0) {
      return null;
    }
    const catalog = modelOptionPolicy?.nativeTools ?? [];
    const options = orderedKeys.flatMap((toolKey): NativeToolVisualOption[] => {
      const definitions = catalog.filter((tool) => tool.toolKey === toolKey);
      if (definitions.length === 0) {
        return [];
      }
      const protocols = Array.from(new Set(definitions.map((tool) => tool.protocol).filter(Boolean)));
      const matchedDefinition = definitions.find((tool) => tool.protocol === policyProtocol);
      return [{
        definition: matchedDefinition ?? definitions[0],
        protocols,
        protocolMatched: Boolean(matchedDefinition),
      }];
    });
    if (options.length === 0) {
      return null;
    }
    const provider = options[0]?.definition.provider ?? selectedProtocol;
    return {
      title: resolveNativeToolGroupTitle(provider, provider, tComposer),
      options,
    };
  }, [modelOptionPolicy?.nativeTools, nativeToolKeys, selectedProtocol, tComposer]);
  const visibleOptions = React.useMemo(
    () => [...configuredOptions, ...editableOptions],
    [configuredOptions, editableOptions],
  );
  const hasRecognizedOptions = Boolean(nativeToolGroup) || visibleOptions.length > 0 || filteredOptions.length > 0;

  React.useEffect(() => {
    optionsObjectRef.current = optionsObject;
  }, [optionsObject]);

  const openOptionsDialog = React.useCallback(() => {
    const sanitized = sanitizeConversationOptions(options);
    optionsObjectRef.current = sanitized;
    setOptionsObject(sanitized);
    setFilteredOptions([]);
    setOptionsDraft(stringifyOptions(sanitized));
    setMobileView("json");
    setRestoredDefaultOptions(null);
    setDialogOpen(true);
  }, [options]);

  const replaceOptionsDraft = React.useCallback((next: ConversationOptions) => {
    const sanitized = sanitizeConversationOptions(next);
    optionsObjectRef.current = sanitized;
    setOptionsObject(sanitized);
    setFilteredOptions([]);
    setOptionsDraft(stringifyOptions(sanitized));
  }, []);

  const replaceRawOptionsDraft = React.useCallback((next: ConversationOptions) => {
    const parsed = parseOptionsDraft(stringifyOptions(next));
    const nextOptions = parsed.rawOptions ?? next;
    optionsObjectRef.current = nextOptions;
    setOptionsObject(nextOptions);
    setFilteredOptions(parsed.filteredOptions);
    setOptionsDraft(stringifyOptions(next));
  }, []);

  const updateOptionValue = React.useCallback(
    (path: string[], value: unknown) => {
      replaceRawOptionsDraft(setOptionAtPath(optionsObjectRef.current, path, value));
    },
    [replaceRawOptionsDraft],
  );

  const updateProviderTool = React.useCallback(
    (tool: NativeToolDefinition, enabled: boolean) => {
      replaceRawOptionsDraft(setProviderToolEnabled(optionsObjectRef.current, tool, enabled));
    },
    [replaceRawOptionsDraft],
  );

  const handleOptionsJSONChange = React.useCallback((value: string) => {
    setOptionsDraft(value);

    const parsed = parseOptionsDraft(value);
    if (parsed.rawOptions && parsed.options) {
      optionsObjectRef.current = parsed.rawOptions;
      setOptionsObject(parsed.rawOptions);
      setFilteredOptions(parsed.filteredOptions);
    }
  }, []);

  const handleRestoreDefaultOptions = React.useCallback(async () => {
    if (defaultRestorePending) {
      return;
    }
    setDefaultRestorePending(true);
    try {
      const latestDefaults = await onDefaultOptionsRestore();
      if (!latestDefaults) {
        toast.error(tComposer("defaultModelUnavailable"));
        return;
      }
      const sanitized = sanitizeConversationOptions(latestDefaults);
      setRestoredDefaultOptions(sanitized);
      replaceOptionsDraft(sanitized);
    } catch {
      toast.error(tComposer("defaultLoadFailed"), { description: tComposer("retryLater") });
    } finally {
      setDefaultRestorePending(false);
    }
  }, [defaultRestorePending, onDefaultOptionsRestore, replaceOptionsDraft, tComposer]);

  const saveOptionsDraft = React.useCallback(() => {
    const parsed = parseOptionsDraft(optionsDraft);
    if (!parsed.options) {
      setMobileView("json");
      toast.error(tComposer("saveFailed"));
      return;
    }
    if (JSON.stringify(parsed.options) === JSON.stringify(effectiveDefaultOptions)) {
      onOptionsReset(effectiveDefaultOptions);
      setDialogOpen(false);
      return;
    }
    onOptionsChange(parsed.options);
    setDialogOpen(false);
  }, [effectiveDefaultOptions, onOptionsChange, onOptionsReset, optionsDraft, tComposer]);

  const renderOptionsViewToggle = () => (
    <Tabs
      value={mobileView}
      onValueChange={(value) => setMobileView(value as "json" | "visual")}
      className="w-fit gap-0"
    >
      <TabsList className="h-7">
        <TabsTrigger value="json">JSON</TabsTrigger>
        <TabsTrigger value="visual">{tComposer("visual")}</TabsTrigger>
      </TabsList>
    </Tabs>
  );

  const renderOptionsEditor = () => (
    <div className="flex h-full min-h-0 flex-col space-y-1.5">
      <div className="flex min-w-0 items-center justify-between gap-2">
        <div className="flex min-w-0 items-center gap-2">
          <p className="shrink-0 text-xs text-muted-foreground">{tComposer("jsonConfig")}</p>
        </div>
        {selectedProtocolLabel ? (
          <span
            className="max-w-[70%] shrink-0 truncate rounded-md bg-muted px-1.5 py-0.5 font-mono text-[11px] leading-none text-muted-foreground"
            title={selectedProtocol}
          >
            {selectedProtocolLabel}
          </span>
        ) : null}
      </div>
      <div className="min-h-0 flex-1 p-0.5">
        <JsonCodeEditor
          value={optionsDraft}
          onChange={handleOptionsJSONChange}
          autoFocus
          height="100%"
          className="h-full min-h-0"
          actions={
            <>
              <Button
                type="button"
                variant="ghost"
                size="sm"
                className="h-6 px-2 text-[11px]"
                onClick={() => replaceOptionsDraft({})}
              >
                {tComposer("clear")}
              </Button>
              <Button
                type="button"
                variant="ghost"
                size="sm"
                className="h-6 px-2 text-[11px]"
                disabled={defaultRestorePending}
                onClick={() => void handleRestoreDefaultOptions()}
              >
                {tComposer("default")}
              </Button>
            </>
          }
          placeholder={`{
  "temperature": 0.7,
  "thinking": {
    "type": "enabled"
  },
  "reasoning": {
    "effort": "high"
  }
}`}
        />
      </div>
    </div>
  );

  const renderOptionsVisualFields = () => (
    <div className="flex h-full min-h-0 flex-col space-y-1.5 md:border-l md:pl-5">
      <div className="flex min-h-6 items-center justify-between gap-2 px-2 py-0.5">
        <p className="shrink-0 text-xs text-muted-foreground">{tComposer("visualConfig")}</p>
        <Tooltip>
          <TooltipTrigger asChild>
            <button
              type="button"
              className="inline-flex size-6 shrink-0 items-center justify-center rounded-md text-muted-foreground hover:bg-muted hover:text-foreground"
              aria-label={tComposer("ignoredHelp")}
            >
              <CircleHelp className="size-3.5" />
            </button>
          </TooltipTrigger>
          <TooltipContent side="left" align="end" className="max-w-64">
            <p>{tComposer("ignoredHelp")}</p>
          </TooltipContent>
        </Tooltip>
      </div>
      {hasRecognizedOptions ? (
        <div className="min-h-0 flex-1 overflow-y-auto pr-1">
          <div className="space-y-2 md:space-y-2.5">
            {nativeToolGroup ? (
              <div className="space-y-1.5 px-2 py-1.5">
                <div className="min-w-0">
                  <p className="truncate text-xs text-foreground/80">{nativeToolGroup.title}</p>
                </div>
                <div className="space-y-1">
                  {nativeToolGroup.options.map((toolOption) => {
                    const tool = toolOption.definition;
                    const checked = hasProviderTool(optionsObject, tool);
                    const label = resolveNativeToolLabel(tool, tNativeToolLabels);
                    const description = resolveNativeToolDescription(tool, tNativeToolDescriptions);
                    const protocolLabels = toolOption.protocols.map(resolveProtocolLabel).join(" / ");
                    const status = checked ? "passed" : "inactive";
                    return (
                      <label
                        key={tool.type}
                        className="flex min-w-0 cursor-pointer items-center gap-2 rounded-md px-2 py-1.5 hover:bg-muted/50"
                      >
                        <Checkbox
                          checked={checked}
                          onCheckedChange={(nextChecked) => updateProviderTool(tool, nextChecked === true)}
                        />
                        <span className="grid min-w-0 flex-1 grid-cols-[minmax(0,1fr)] text-xs">
                          <span className="min-w-0 truncate text-foreground/80">
                            {label}
                          </span>
                          <span
                            className="min-w-0 truncate text-[11px] text-muted-foreground"
                            title={description}
                          >
                            {description}
                          </span>
                        </span>
                        <Tooltip>
                          <TooltipTrigger asChild>
                            <span className="inline-flex shrink-0 items-center gap-1">
                              {!toolOption.protocolMatched ? (
                                <span className="rounded-md bg-amber-500/10 px-1.5 py-0.5 text-[10px] leading-none text-amber-700">
                                  {tComposer("nativeToolMayNotApply")}
                                </span>
                              ) : null}
                              <ModelOptionFilterBadge
                                status={status}
                                inactiveLabel={tComposer("notEnabled")}
                                ignoredLabel={tComposer("ignored")}
                                passedLabel={tComposer("willPass")}
                              />
                            </span>
                          </TooltipTrigger>
                          <TooltipContent side="left" align="end" className="max-w-72 text-xs">
                            <p>{description}</p>
                            <p className="mt-1 text-muted-foreground">
                              {tComposer("currentProtocol")}：{selectedProtocolLabel || selectedProtocol || "-"}
                            </p>
                            <p className="text-muted-foreground">
                              {tComposer("toolProtocols")}：{protocolLabels || "-"}
                            </p>
                            {!toolOption.protocolMatched ? (
                              <p className="mt-1 text-amber-700">{tComposer("nativeToolMayNotApplyHelp")}</p>
                            ) : null}
                          </TooltipContent>
                        </Tooltip>
                      </label>
                    );
                  })}
                </div>
              </div>
            ) : null}
            {visibleOptions.map(({ key, path, value, active, label, description, kind: configuredKind, selectValues: configuredSelectValues, placeholder }) => {
              const selectValues = resolveSelectValues(key, configuredSelectValues);
              const kind = configuredKind === "select" && selectValues.length === 0
                ? "text"
                : (configuredKind ?? resolveOptionKind(key, value));
              const title = resolveOptionTitle(key, label, tOptionLabels);
              const optionDescription = resolveOptionDescription(key, description, tOptionDescriptions);
              const detailText = optionDescription || key;
              const filterStatus = active ? resolveModelOptionFilterStatus(modelOptionPolicy, selectedProtocol, key) : "inactive";
              const ignored = filterStatus === "filtered";

              return (
                <div
                  key={key}
                  className={cn(
                    "grid grid-cols-[minmax(0,2fr)_minmax(0,1fr)] items-center gap-2 rounded-md px-2 py-1.5 sm:gap-3",
                    ignored && "text-muted-foreground",
                  )}
                >
                  <div className="min-w-0">
                    <div className="flex min-w-0 items-center gap-1.5">
                      <p
                        className={cn(
                          "min-w-0 truncate text-xs text-foreground/80",
                          ignored && "text-muted-foreground line-through",
                        )}
                      >
                        {title}
                      </p>
                      <ModelOptionFilterBadge
                        status={filterStatus}
                        inactiveLabel={tComposer("notEnabled")}
                        ignoredLabel={tComposer("ignored")}
                        passedLabel={tComposer("willPass")}
                      />
                    </div>
                    {detailText ? (
                      <p
                        className={cn("truncate text-[11px] leading-4 text-muted-foreground", ignored && "line-through")}
                        title={detailText}
                      >
                        {detailText}
                      </p>
                    ) : null}
                  </div>
                  {kind === "boolean" ? (
                    <Select
                      value={value === true ? "true" : value === false ? "false" : undefined}
                      onValueChange={(nextValue) => updateOptionValue(path, nextValue === "true")}
                    >
                      <SelectTrigger size="sm">
                        <SelectValue placeholder={placeholder ?? key} />
                      </SelectTrigger>
                      <SelectContent>
                        <SelectItem value="true">{tComposer("booleanOn")}</SelectItem>
                        <SelectItem value="false">{tComposer("booleanOff")}</SelectItem>
                      </SelectContent>
                    </Select>
                  ) : kind === "select" ? (
                    <Select
                      value={typeof value === "string" && value.trim() ? value : undefined}
                      onValueChange={(nextValue) => updateOptionValue(path, nextValue)}
                    >
                      <SelectTrigger size="sm">
                        <SelectValue placeholder={placeholder ?? key} />
                      </SelectTrigger>
                      <SelectContent>
                        {selectValues.map((item) => (
                          <SelectItem key={item} value={item}>
                            {item}
                          </SelectItem>
                        ))}
                      </SelectContent>
                    </Select>
                  ) : (
                    <Input
                      value={value === null ? "" : String(value)}
                      inputMode={kind === "number" ? "decimal" : undefined}
                      placeholder={placeholder ?? (kind === "number" ? "0.7" : key)}
                      onChange={(event) => {
                        const nextValue = event.target.value;
                        if (kind === "number") {
                          updateOptionValue(path, parseVisualNumberInput(nextValue));
                          return;
                        }
                        if (NUMBER_OPTION_KEYS.has(key)) {
                          updateOptionValue(path, parseVisualNumberInput(nextValue));
                          return;
                        }
                        updateOptionValue(path, nextValue);
                      }}
                    />
                  )}
                </div>
              );
            })}
            {filteredOptions.length > 0 ? (
              <div className={cn("space-y-2", visibleOptions.length > 0 ? "pt-1" : "")}>
                {filteredOptions.map((item) => (
                  <div
                    key={item.key}
                    className="grid grid-cols-[minmax(0,2fr)_minmax(0,1fr)] items-center gap-2 rounded-md px-2 py-1.5 text-muted-foreground sm:gap-3"
                  >
                    <div className="min-w-0">
                      <p className="truncate text-xs line-through">{item.key}</p>
                    </div>
                    <span className="w-fit text-[11px] sm:justify-self-end">
                      {tComposer("ignored")}
                    </span>
                  </div>
                ))}
              </div>
            ) : null}
          </div>
        </div>
      ) : (
        <div className="flex min-h-0 flex-1 items-center justify-center text-xs text-muted-foreground">
          {tComposer("noVisualFields")}
        </div>
      )}
    </div>
  );

  return (
    <>
      <InputGroupButton
        type="button"
        variant="ghost"
        size="icon-sm"
        className="rounded-md text-muted-foreground hover:text-foreground"
        disabled={disabled}
        onClick={openOptionsDialog}
        aria-label={tComposer("modelOptions")}
        title={tComposer("modelOptions")}
        onMouseEnter={() => setHovered(true)}
        onMouseLeave={() => setHovered(false)}
      >
        <Cog
          size={20}
          strokeWidth={1.4}
          animate={hovered ? "default" : undefined}
        />
      </InputGroupButton>

      <Dialog open={dialogOpen} onOpenChange={setDialogOpen}>
        <DialogContent
          className="flex max-h-[calc(100dvh-1rem)] w-[calc(100vw-1rem)] flex-col overflow-hidden p-4 sm:max-h-[min(92vh,760px)] sm:w-full sm:max-w-[800px] sm:p-6 md:max-w-[900px]"
        >
          <DialogHeader className="shrink-0">
            <div className="flex min-w-0 items-center justify-between gap-3">
              <DialogTitle className="shrink-0">{tComposer("modelOptions")}</DialogTitle>
              <div className="shrink-0 md:hidden">{renderOptionsViewToggle()}</div>
            </div>
            <DialogDescription className="hidden md:block">
              {tComposer("dialogDescription", { model: selectedModelName || tComposer("currentModel") })}
            </DialogDescription>
          </DialogHeader>

          <form
            className="flex min-h-0 flex-1 flex-col gap-4"
            onSubmit={(event) => {
              event.preventDefault();
              saveOptionsDraft();
            }}
          >
            <div className="min-h-0 flex-1 overflow-hidden">
              <div className="grid h-[min(58dvh,520px)] min-h-[320px] min-w-0 gap-4 md:grid-cols-[minmax(0,430px)_minmax(300px,1fr)] md:gap-5">
                <div className={cn(mobileView === "json" ? "block" : "hidden", "h-full min-h-0 md:block")}>
                  {renderOptionsEditor()}
                </div>
                <div className={cn(mobileView === "visual" ? "block" : "hidden", "h-full min-h-0 md:block")}>
                  {renderOptionsVisualFields()}
                </div>
              </div>
            </div>
            <DialogFooter className="shrink-0">
              <Button type="button" variant="ghost" onClick={() => setDialogOpen(false)}>
                {tCommon("cancel")}
              </Button>
              <Button type="submit">
                {tCommon("save")}
              </Button>
            </DialogFooter>
          </form>
        </DialogContent>
      </Dialog>
    </>
  );
}
