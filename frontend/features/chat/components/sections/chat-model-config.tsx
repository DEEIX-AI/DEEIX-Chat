"use client";

import { CircleHelp } from "lucide-react";
import { useMessages, useTranslations } from "next-intl";
import * as React from "react";
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
import type { ModelOptionControl } from "@/features/chat/types/chat-runtime";
import { cn } from "@/lib/utils";
import type { ConversationOptions } from "@/shared/api/conversation.types";
import { JsonCodeEditor } from "@/shared/components/json-code-editor";
import type { ModelNativeToolConfig, ModelOptionPolicy, NativeToolDefinition } from "@/shared/lib/model-option-policy";
import { isModelOptionPathFiltered, resolveModelOptionPolicyProtocol } from "@/shared/lib/model-option-policy";
import { localizedNativeToolText } from "@/shared/lib/native-tool-i18n";
import { nativeToolDefinitionVariantsFromConfig, nativeToolPayloadSignature } from "@/shared/lib/native-tool-payload";

type EditableOptionValue = string | number | boolean | null;
type VisualOptionKind = "boolean" | "number" | "select" | "text";

type VisualOption = {
  key: string;
  path: string[];
  value: unknown;
  label?: string;
  description?: string;
  kind?: VisualOptionKind;
  selectValues?: string[];
  placeholder?: string;
  active: boolean;
  editable: boolean;
  locked?: boolean;
  forcedFilterStatus?: ModelOptionFilterStatus;
};

type ModelOptionFilterStatus = "inactive" | "passed" | "filtered" | "route-dependent" | "unknown";

type OptionValueEntry = {
  key: string;
  path: string[];
  value: unknown;
};

type NativeToolVisualOption = {
  primary: NativeToolDefinition;
  variants: NativeToolDefinition[];
  protocols: string[];
};

type ChatModelConfigProps = {
  disabled: boolean;
  options: ConversationOptions;
  defaultOptions: ConversationOptions;
  optionControls: ModelOptionControl[];
  lockedOptionPaths: string[];
  nativeToolKeys: string[];
  nativeTools: ModelNativeToolConfig[];
  modelOptionPolicy: ModelOptionPolicy | null;
  selectedProtocols: string[];
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
  "duration",
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
  "generation_config.max_output_tokens",
  "generation_config.thinking_level",
  "generation_config.thinking_summaries",
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
  "duration",
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

const NUMBER_OPTION_PLACEHOLDERS: Record<string, string> = {
  "generation_config.max_output_tokens": "4096",
};

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
  gemini_interactions: "Interactions",
  openai_chat_completions: "Chat Completions",
  openrouter_chat_completions: "OpenRouter Chat Completions",
  openai_image_edits: "Images Edits",
  openai_image_generations: "Images Generations",
  openai_responses: "Responses",
  openrouter_responses: "OpenRouter Responses",
  openai_video_generations: "Video Generations",
  replicate_predictions: "Predictions",
  stability_ai_generate: "Image Generation",
  xai_image: "Images Generations",
  xai_image_edits: "Images Edits",
  xai_video: "Video Generations",
  xai_video_extensions: "Video Extensions",
  xai_responses: "xAI Responses",
};

function stringifyOptions(value: ConversationOptions): string {
  if (Object.keys(value).length === 0) {
    return "";
  }
  return JSON.stringify(value, null, 2);
}

function parseOptionsDraft(value: string): {
  options: ConversationOptions | null;
  rawOptions: ConversationOptions | null;
  error: string;
} {
  try {
    const parsed = JSON.parse(value.trim() || "{}") as unknown;
    if (parsed === null || Array.isArray(parsed) || typeof parsed !== "object") {
      return { options: null, rawOptions: null, error: "JSON must be an object" };
    }
    const rawOptions = parsed as ConversationOptions;
    return {
      options: sanitizeConversationOptions(rawOptions),
      rawOptions,
      error: "",
    };
  } catch {
    return { options: null, rawOptions: null, error: "" };
  }
}

function collectOptionValueEntries(value: unknown, path: string[]): OptionValueEntry[] {
  if (path.length === 0) {
    return [];
  }
  if (!isPlainOptionObject(value)) {
    return [{ key: optionPathKey(path), path, value }];
  }
  const entries = Object.entries(value).flatMap(([key, nestedValue]) => collectOptionValueEntries(nestedValue, [...path, key]));
  if (entries.length === 0) {
    return [{ key: optionPathKey(path), path, value }];
  }
  return entries;
}

function optionValueEntriesFromOptions(options: ConversationOptions): OptionValueEntry[] {
  return Object.entries(options).flatMap(([key, value]) => {
    if (key === "tools") {
      return [{ key, path: [key], value }];
    }
    return collectOptionValueEntries(value, [key]);
  });
}

function formatVisualOptionValue(value: unknown): string {
  if (typeof value === "string") {
    return value;
  }
  if (typeof value === "number" || typeof value === "boolean" || value === null) {
    return String(value);
  }
  try {
    return JSON.stringify(value);
  } catch {
    return String(value);
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
  const toolType = typeof tool.type === "string" ? tool.type.trim() : "";
  if (toolType) {
    return toolType === definition.type;
  }
  return Object.keys(definition.payload ?? {}).some((key) => key !== "type" && Object.hasOwn(tool, key));
}

function nativeToolDefinitionsFromKeys(
  toolKeys: string[],
  catalog: NativeToolDefinition[],
): NativeToolDefinition[] {
  const allowedKeys = new Set(toolKeys.map((key) => key.trim()).filter(Boolean));
  return catalog.filter((tool) => allowedKeys.has(tool.toolKey.trim()));
}

function nativeToolVisualOptionsFromConfigs(
  configs: ModelNativeToolConfig[],
  fallbackToolKeys: string[],
  catalog: NativeToolDefinition[],
  modelProtocols: string[],
): NativeToolVisualOption[] {
  const sourceConfigs = configs.length > 0
    ? configs
    : nativeToolDefinitionsFromKeys(fallbackToolKeys, catalog).map((tool): ModelNativeToolConfig => ({
      id: `${tool.protocol}:${tool.toolKey}:${tool.type}`,
      key: tool.toolKey,
      protocol: tool.protocol,
      protocols: [tool.protocol],
      provider: tool.provider,
      type: tool.type,
      label: tool.label,
      description: tool.description,
      enabled: true,
      defaultEnabled: false,
      payload: tool.payload,
    }));
  const visualOptions = new Map<string, NativeToolVisualOption>();
  sourceConfigs.forEach((config) => {
    if (!config.enabled) {
      return;
    }
    const definitions = nativeToolDefinitionVariantsFromConfig(config, catalog, modelProtocols);
    if (definitions.length === 0) {
      return;
    }
    definitions.forEach((definition) => {
      const visualKey = definition.type.trim()
        || definition.toolKey.trim()
        || definition.provider.trim()
        || `payload:${nativeToolPayloadSignature(definition.payload)}`;
      const existing = visualOptions.get(visualKey);
      if (!existing) {
        visualOptions.set(visualKey, {
          primary: definition,
          variants: [definition],
          protocols: [definition.protocol].filter(Boolean),
        });
        return;
      }
      const variantSignature = `${resolveModelOptionPolicyProtocol(definition.protocol)}:${nativeToolPayloadSignature(definition.payload)}`;
      const hasVariant = existing.variants.some((candidate) =>
        `${resolveModelOptionPolicyProtocol(candidate.protocol)}:${nativeToolPayloadSignature(candidate.payload)}` === variantSignature
      );
      if (!hasVariant) {
        existing.variants.push(definition);
      }
      existing.protocols = Array.from(new Set([...existing.protocols, definition.protocol].filter(Boolean)));
    });
  });
  return Array.from(visualOptions.values());
}

function providerToolMatchesAnyDefinition(
  value: unknown,
  definitions: NativeToolDefinition[],
): boolean {
  if (!isPlainOptionObject(value)) {
    return false;
  }
  return definitions.some((definition) => providerToolMatchesDefinition(value, definition));
}

function ignoredProviderToolValues(
  value: unknown,
  definitions: NativeToolDefinition[],
): unknown[] {
  if (value === undefined) {
    return [];
  }
  if (!Array.isArray(value)) {
    return [value];
  }
  return value.filter((item) => !providerToolMatchesAnyDefinition(item, definitions));
}

function hasProviderTool(options: ConversationOptions, definitions: NativeToolDefinition[]): boolean {
  return providerToolObjectsFromOptions(options).some((tool) =>
    definitions.some((definition) => providerToolMatchesDefinition(tool, definition))
  );
}

function setProviderToolEnabled(
  options: ConversationOptions,
  definitions: NativeToolDefinition[],
  enabled: boolean,
): ConversationOptions {
  const tools = providerToolObjectsFromOptions(options);
  const matchesTool = (tool: Record<string, unknown>) =>
    definitions.some((definition) => providerToolMatchesDefinition(tool, definition));
  const nextTools = tools.filter((tool) => !matchesTool(tool));
  if (enabled) {
    const seenPayloads = new Set<string>();
    for (const definition of definitions) {
      const payload = Object.keys(definition.payload).length > 0
        ? definition.payload
        : { type: definition.type };
      const signature = nativeToolPayloadSignature(payload);
      if (seenPayloads.has(signature)) {
        continue;
      }
      seenPayloads.add(signature);
      nextTools.push({ ...payload });
    }
  }

  if (nextTools.length === 0) {
    const { tools: _tools, ...rest } = options;
    return rest;
  }

  return { ...options, tools: nextTools };
}

function optionPathKey(path: string[]): string {
  return path.join(".");
}

function optionFilterStatusForPath(
  policy: ModelOptionPolicy | null,
  protocols: string[],
  key: string,
  path: string[],
): ModelOptionFilterStatus {
  if (isReservedConversationOptionKey(path[0] ?? "")) {
    return "filtered";
  }
  return resolveModelOptionFilterStatus(policy, protocols, key);
}

function policyLimitedVisualOption(
  entry: OptionValueEntry,
  value: unknown,
  status: "filtered" | "route-dependent",
): VisualOption {
  return {
    key: entry.key,
    path: entry.path,
    value,
    active: true,
    editable: false,
    forcedFilterStatus: status,
  };
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
    if (!isPlainOptionObject(current) || !Object.hasOwn(current, segment)) {
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

function applyLockedDefaultOptions(
  options: ConversationOptions,
  defaults: ConversationOptions,
  lockedPaths: string[],
): ConversationOptions {
  if (lockedPaths.length === 0) {
    return options;
  }
  return lockedPaths.reduce((nextOptions, key) => {
    const path = optionPathFromControl(key);
    if (path.length === 0) {
      return nextOptions;
    }
    const defaultValue = getOptionAtPath(defaults, path);
    return defaultValue === undefined ? nextOptions : setOptionAtPath(nextOptions, path, defaultValue);
  }, options);
}

function visualOptionsFromOptions(
  options: ConversationOptions,
  policy: ModelOptionPolicy | null,
  protocols: string[],
  nativeToolDefinitions: NativeToolDefinition[],
): VisualOption[] {
  const nestedOptions = NESTED_VISUAL_OPTION_PATHS.flatMap((path): VisualOption[] => {
    if (isReservedConversationOptionKey(path[0] ?? "")) {
      return [];
    }
    const value = getOptionAtPath(options, path);
    if (!isEditableOptionValue(value)) {
      return [];
    }
    return [{ key: optionPathKey(path), path, value, active: true, editable: true }];
  });
  const topLevelOptions = Object.entries(options).flatMap(([key, value]): VisualOption[] => {
    if (isReservedConversationOptionKey(key)) {
      return [];
    }
    if (isEditableOptionValue(value)) {
      return [{ key, path: [key], value, active: true, editable: true }];
    }
    return [];
  });
  const editableOptions = [...nestedOptions, ...topLevelOptions]
    .filter((item, index, items) => items.findIndex((candidate) => candidate.key === item.key) === index);
  const visibleKeys = new Set(editableOptions.map((item) => item.key));
  const ignoredOptions = optionValueEntriesFromOptions(options).flatMap((entry): VisualOption[] => {
    if (visibleKeys.has(entry.key)) {
      return [];
    }
    if (entry.key === "tools") {
      const ignoredTools = ignoredProviderToolValues(entry.value, nativeToolDefinitions);
      return ignoredTools.length > 0 ? [policyLimitedVisualOption(entry, ignoredTools, "filtered")] : [];
    }
    const status = optionFilterStatusForPath(policy, protocols, entry.key, entry.path);
    if (status !== "filtered" && status !== "route-dependent") {
      return [];
    }
    return [policyLimitedVisualOption(entry, entry.value, status)];
  });
  return [...editableOptions, ...ignoredOptions]
    .filter((item, index, items) => items.findIndex((candidate) => candidate.key === item.key) === index)
    .sort((left, right) => compareOptionKeys(left.key, right.key));
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
  defaultOptions: ConversationOptions = {},
): VisualOption[] {
  return controls.flatMap((control): VisualOption[] => {
    const path = optionPathFromControl(control.path);
    if (path.length === 0 || isReservedConversationOptionKey(path[0] ?? "")) {
      return [];
    }
    const key = optionPathKey(path);
    const hasLockedDefault = Boolean(control.locked && hasOptionAtPath(defaultOptions, path));
    const value = hasLockedDefault
      ? resolveControlEditableValue(defaultOptions, path)
      : resolveControlEditableValue(options, path);
    const active = hasOptionAtPath(options, path) || hasLockedDefault;
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
      editable: !control.locked,
      locked: control.locked,
    }];
  });
}

function hasVisualConfigurationContent({
  nativeToolDefinitions,
  optionControls,
  options,
  policy,
  protocols,
}: {
  nativeToolDefinitions: NativeToolDefinition[];
  optionControls: ModelOptionControl[];
  options: ConversationOptions;
  policy: ModelOptionPolicy | null;
  protocols: string[];
}): boolean {
  if (nativeToolDefinitions.length > 0) {
    return true;
  }
  const configuredOptions = visualOptionsFromControls(optionControls, options);
  if (configuredOptions.length > 0) {
    return true;
  }
  const configuredKeys = new Set(configuredOptions.map((item) => item.key));
  return visualOptionsFromOptions(options, policy, protocols, nativeToolDefinitions)
    .some((item) => !configuredKeys.has(item.key));
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

function resolveSelectOptionValue(key: string, value: string, selectValues: string[]): string | number {
  if (NUMBER_OPTION_KEYS.has(key) && selectValues.every((item) => typeof parseVisualNumberInput(item) === "number")) {
    const parsed = parseVisualNumberInput(value);
    if (typeof parsed === "number") {
      return parsed;
    }
  }
  return value;
}

function resolveModelOptionFilterStatus(
  policy: ModelOptionPolicy | null,
  protocols: string[],
  path: string,
): ModelOptionFilterStatus {
  if (!policy) {
    return "unknown";
  }
  const policyProtocols = Array.from(new Set(protocols.map(resolveModelOptionPolicyProtocol).filter(Boolean)));
  if (policyProtocols.length === 0) {
    return isModelOptionPathFiltered({ policy, protocol: "", path }) ? "filtered" : "passed";
  }
  const filteredCount = policyProtocols.filter((protocol) =>
    isModelOptionPathFiltered({ policy, protocol, path })
  ).length;
  if (filteredCount === 0) {
    return "passed";
  }
  return filteredCount === policyProtocols.length ? "filtered" : "route-dependent";
}

function resolveNativeToolRouteStatus(modelProtocols: string[], toolProtocols: string[]): ModelOptionFilterStatus {
  const modelProtocolSet = new Set(modelProtocols.map(resolveModelOptionPolicyProtocol).filter(Boolean));
  const toolProtocolSet = new Set(toolProtocols.map(resolveModelOptionPolicyProtocol).filter(Boolean));
  if (modelProtocolSet.size === 0 || toolProtocolSet.size === 0) {
    return "passed";
  }
  let matchedCount = 0;
  for (const protocol of modelProtocolSet) {
    if (toolProtocolSet.has(protocol)) {
      matchedCount++;
    }
  }
  if (matchedCount === 0) {
    return "filtered";
  }
  return matchedCount === modelProtocolSet.size ? "passed" : "route-dependent";
}

function ModelOptionFilterBadge({
  status,
  inactiveLabel,
  ignoredLabel,
  passedLabel,
  routeDependentLabel,
}: {
  status: ModelOptionFilterStatus;
  inactiveLabel: string;
  ignoredLabel: string;
  passedLabel: string;
  routeDependentLabel: string;
}) {
  if (status === "unknown") {
    return null;
  }
  return (
    <span
      data-filtered={status === "filtered"}
      data-inactive={status === "inactive"}
      data-route-dependent={status === "route-dependent"}
      className="shrink-0 rounded-md bg-emerald-500/10 px-1.5 py-0.5 text-[10px] leading-none text-emerald-700 data-[filtered=true]:bg-muted data-[filtered=true]:text-muted-foreground data-[inactive=true]:bg-muted data-[inactive=true]:text-muted-foreground data-[route-dependent=true]:bg-amber-500/10 data-[route-dependent=true]:text-amber-700"
    >
      {status === "inactive"
        ? inactiveLabel
        : status === "filtered"
          ? ignoredLabel
          : status === "route-dependent"
            ? routeDependentLabel
            : passedLabel}
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

function resolveNativeToolLabel(tool: NativeToolDefinition, messages: unknown): string {
  return localizedNativeToolText(messages, "nativeToolLabels", tool.toolKey) || tool.label || tool.type || tool.toolKey;
}

function resolveNativeToolDescription(tool: NativeToolDefinition, messages: unknown): string {
  return localizedNativeToolText(messages, "nativeToolDescriptions", tool.toolKey) || tool.description || tool.type || tool.toolKey;
}

export function ChatModelConfig({
  disabled,
  options,
  defaultOptions,
  optionControls,
  lockedOptionPaths,
  nativeToolKeys,
  nativeTools,
  modelOptionPolicy,
  selectedProtocols,
  selectedModelName,
  onOptionsChange,
  onOptionsReset,
  onDefaultOptionsRestore,
}: ChatModelConfigProps) {
  const tCommon = useTranslations("common.actions");
  const tComposer = useTranslations("chat.composer");
  const tOptionLabels = useTranslations("chat.optionLabels");
  const tOptionDescriptions = useTranslations("chat.optionDescriptions");
  const messages = useMessages();
  const [hovered, setHovered] = React.useState(false);
  const [dialogOpen, setDialogOpen] = React.useState(false);
  const [optionsDraft, setOptionsDraft] = React.useState("");
  const [optionsObject, setOptionsObject] = React.useState<ConversationOptions>({});
  const [mobileView, setMobileView] = React.useState<"json" | "visual">("visual");
  const [defaultRestorePending, setDefaultRestorePending] = React.useState(false);
  const [restoredDefaultOptions, setRestoredDefaultOptions] = React.useState<ConversationOptions | null>(null);
  const optionsObjectRef = React.useRef<ConversationOptions>({});
  const effectiveDefaultOptions = restoredDefaultOptions ?? defaultOptions;
  const modelProtocolLabels = selectedProtocols.map(resolveProtocolLabel).join(" / ");
  const nativeToolVisualOptions = React.useMemo(
    () => nativeToolVisualOptionsFromConfigs(nativeTools, nativeToolKeys, modelOptionPolicy?.nativeTools ?? [], selectedProtocols),
    [modelOptionPolicy?.nativeTools, nativeToolKeys, nativeTools, selectedProtocols],
  );
  const nativeToolDefinitions = React.useMemo(
    () => nativeToolVisualOptions.flatMap((item) => item.variants),
    [nativeToolVisualOptions],
  );
  const configuredOptions = React.useMemo(
    () => visualOptionsFromControls(optionControls, optionsObject, effectiveDefaultOptions),
    [effectiveDefaultOptions, optionControls, optionsObject],
  );
  const configuredOptionKeys = React.useMemo(
    () => new Set(configuredOptions.map((item) => item.key)),
    [configuredOptions],
  );
  const editableOptions = React.useMemo(
    () => visualOptionsFromOptions(optionsObject, modelOptionPolicy, selectedProtocols, nativeToolDefinitions)
      .filter((item) => !configuredOptionKeys.has(item.key)),
    [configuredOptionKeys, modelOptionPolicy, nativeToolDefinitions, optionsObject, selectedProtocols],
  );
  const nativeToolGroup = React.useMemo(() => {
    if (nativeToolVisualOptions.length === 0) {
      return null;
    }
    const providers = Array.from(new Set(
      nativeToolVisualOptions.flatMap((item) => item.variants.map((variant) => variant.provider).filter(Boolean)),
    ));
    const provider = providers.length === 1 ? providers[0] : "";
    return {
      title: provider ? resolveNativeToolGroupTitle(provider, provider, tComposer) : tComposer("nativeTools.official"),
      options: nativeToolVisualOptions,
    };
  }, [nativeToolVisualOptions, tComposer]);
  const visibleOptions = React.useMemo(
    () => [...configuredOptions, ...editableOptions],
    [configuredOptions, editableOptions],
  );
  const lockedOptionPathSet = React.useMemo(() => new Set(lockedOptionPaths), [lockedOptionPaths]);
  const hasRecognizedOptions = Boolean(nativeToolGroup) || visibleOptions.length > 0;

  React.useEffect(() => {
    optionsObjectRef.current = optionsObject;
  }, [optionsObject]);

  const openOptionsDialog = React.useCallback(() => {
    const sanitized = applyLockedDefaultOptions(
      sanitizeConversationOptions(options),
      effectiveDefaultOptions,
      lockedOptionPaths,
    );
    const hasVisualContent = hasVisualConfigurationContent({
      nativeToolDefinitions,
      optionControls,
      options: sanitized,
      policy: modelOptionPolicy,
      protocols: selectedProtocols,
    });
    optionsObjectRef.current = sanitized;
    setOptionsObject(sanitized);
    setOptionsDraft(stringifyOptions(sanitized));
    setMobileView(hasVisualContent ? "visual" : "json");
    setRestoredDefaultOptions(null);
    setDialogOpen(true);
  }, [effectiveDefaultOptions, lockedOptionPaths, modelOptionPolicy, nativeToolDefinitions, optionControls, options, selectedProtocols]);

  const replaceOptionsDraft = React.useCallback((next: ConversationOptions) => {
    const sanitized = applyLockedDefaultOptions(
      sanitizeConversationOptions(next),
      effectiveDefaultOptions,
      lockedOptionPaths,
    );
    optionsObjectRef.current = sanitized;
    setOptionsObject(sanitized);
    setOptionsDraft(stringifyOptions(sanitized));
  }, [effectiveDefaultOptions, lockedOptionPaths]);

  const replaceRawOptionsDraft = React.useCallback((next: ConversationOptions) => {
    const parsed = parseOptionsDraft(stringifyOptions(next));
    const nextOptions = applyLockedDefaultOptions(
      parsed.rawOptions ?? next,
      effectiveDefaultOptions,
      lockedOptionPaths,
    );
    optionsObjectRef.current = nextOptions;
    setOptionsObject(nextOptions);
    setOptionsDraft(stringifyOptions(nextOptions));
  }, [effectiveDefaultOptions, lockedOptionPaths]);

  const updateOptionValue = React.useCallback(
    (path: string[], value: unknown) => {
      replaceRawOptionsDraft(setOptionAtPath(optionsObjectRef.current, path, value));
    },
    [replaceRawOptionsDraft],
  );

  const updateProviderTool = React.useCallback(
    (definitions: NativeToolDefinition[], enabled: boolean) => {
      replaceRawOptionsDraft(setProviderToolEnabled(optionsObjectRef.current, definitions, enabled));
    },
    [replaceRawOptionsDraft],
  );

  const handleOptionsJSONChange = React.useCallback((value: string) => {
    setOptionsDraft(value);

    const parsed = parseOptionsDraft(value);
    if (parsed.rawOptions && parsed.options) {
      const nextOptions = applyLockedDefaultOptions(parsed.rawOptions, effectiveDefaultOptions, lockedOptionPaths);
      optionsObjectRef.current = nextOptions;
      setOptionsObject(nextOptions);
    }
  }, [effectiveDefaultOptions, lockedOptionPaths]);

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
    const nextOptions = applyLockedDefaultOptions(
      parsed.options,
      effectiveDefaultOptions,
      lockedOptionPaths,
    );
    if (JSON.stringify(nextOptions) !== JSON.stringify(parsed.options)) {
      setOptionsDraft(stringifyOptions(nextOptions));
      setOptionsObject(nextOptions);
      optionsObjectRef.current = nextOptions;
    }
    if (JSON.stringify(nextOptions) === JSON.stringify(effectiveDefaultOptions)) {
      onOptionsReset(effectiveDefaultOptions);
      setDialogOpen(false);
      return;
    }
    onOptionsChange(nextOptions);
    setDialogOpen(false);
  }, [effectiveDefaultOptions, lockedOptionPaths, onOptionsChange, onOptionsReset, optionsDraft, tComposer]);

  const renderOptionsViewToggle = () => (
    <Tabs
      value={mobileView}
      onValueChange={(value) => setMobileView(value as "json" | "visual")}
      className="w-fit gap-0"
    >
      <TabsList className="h-7">
        <TabsTrigger value="visual">{tComposer("visual")}</TabsTrigger>
        <TabsTrigger value="json">JSON</TabsTrigger>
      </TabsList>
    </Tabs>
  );

  const renderOptionsEditor = () => (
    <div className="flex h-full min-h-0 flex-col space-y-1.5">
      <div className="flex min-w-0 items-center justify-between gap-2">
        <div className="flex min-w-0 items-center gap-2">
          <p className="shrink-0 text-xs text-muted-foreground">{tComposer("jsonConfig")}</p>
        </div>
        {modelProtocolLabels ? (
          <span
            className="max-w-[70%] shrink-0 truncate rounded-md bg-muted px-1.5 py-0.5 font-mono text-[11px] leading-none text-muted-foreground"
            title={selectedProtocols.join(", ")}
          >
            {modelProtocolLabels}
          </span>
        ) : null}
      </div>
      <div className="min-h-0 flex-1 p-0.5">
        <JsonCodeEditor
          key={mobileView === "json" ? "json-visible" : "json-hidden"}
          value={optionsDraft}
          onChange={handleOptionsJSONChange}
          autoFocus={mobileView === "json"}
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
            <div className="space-y-1.5 text-xs">
              <p>{tComposer("notEnabledHelp")}</p>
              <p>{tComposer("ignoredHelp")}</p>
              <p>{tComposer("routeDependentHelp")}</p>
              <p>{tComposer("lockedHelp")}</p>
            </div>
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
                    const tool = toolOption.primary;
                    const checked = hasProviderTool(optionsObject, toolOption.variants);
                    const label = resolveNativeToolLabel(tool, messages);
                    const description = resolveNativeToolDescription(tool, messages);
                    const typeLabel = tool.type.trim();
                    const protocolLabels = toolOption.protocols.map(resolveProtocolLabel).join(" / ");
                    const status = checked
                      ? resolveNativeToolRouteStatus(selectedProtocols, toolOption.protocols)
                      : "inactive";
                    return (
                      <label
                        key={tool.toolKey || `${tool.provider}:${tool.type}`}
                        className="flex min-w-0 cursor-pointer items-center gap-2 rounded-md px-2 py-1.5 hover:bg-muted/50"
                      >
                        <Checkbox
                          checked={checked}
                          onCheckedChange={(nextChecked) => updateProviderTool(toolOption.variants, nextChecked === true)}
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
                          {typeLabel ? (
                            <span className="min-w-0">
                              <span
                                className="inline-flex max-w-full truncate rounded-md bg-muted px-1.5 py-0.5 font-mono text-[10px] leading-none text-muted-foreground"
                                title={typeLabel}
                              >
                                {typeLabel}
                              </span>
                            </span>
                          ) : null}
                        </span>
                        <Tooltip>
                          <TooltipTrigger asChild>
                            <span className="inline-flex shrink-0 flex-col items-end gap-1">
                              <ModelOptionFilterBadge
                                status={status}
                                inactiveLabel={tComposer("notEnabled")}
                                ignoredLabel={tComposer("ignored")}
                                passedLabel={tComposer("willPass")}
                                routeDependentLabel={tComposer("routeDependent")}
                              />
                            </span>
                          </TooltipTrigger>
                          <TooltipContent side="left" align="end" className="max-w-72 text-xs">
                            <p>{description}</p>
                            <p className="mt-1 text-muted-foreground">
                              {tComposer("modelProtocols")}：{modelProtocolLabels || "-"}
                            </p>
                            <p className="text-muted-foreground">
                              {tComposer("toolProtocols")}：{protocolLabels || "-"}
                            </p>
                            {status === "route-dependent" ? (
                              <p className="mt-1 text-amber-700">{tComposer("routeDependentHelp")}</p>
                            ) : status === "filtered" ? (
                              <p className="mt-1 text-muted-foreground">{tComposer("ignoredHelp")}</p>
                            ) : null}
                          </TooltipContent>
                        </Tooltip>
                      </label>
                    );
                  })}
                </div>
              </div>
            ) : null}
            {visibleOptions.map(({ key, path, value, active, editable, locked, forcedFilterStatus, label, description, kind: configuredKind, selectValues: configuredSelectValues, placeholder }) => {
              const editableValue = isEditableOptionValue(value) ? value : null;
              const selectValues = resolveSelectValues(key, configuredSelectValues);
              const kind = configuredKind === "select" && selectValues.length === 0
                ? "text"
                : (configuredKind ?? resolveOptionKind(key, editableValue));
              const title = resolveOptionTitle(key, label, tOptionLabels);
              const optionDescription = resolveOptionDescription(key, description, tOptionDescriptions);
              const detailText = optionDescription || key;
              const filterStatus = forcedFilterStatus ?? (active ? resolveModelOptionFilterStatus(modelOptionPolicy, selectedProtocols, key) : "inactive");
              const ignored = filterStatus === "filtered";
              const lockedByPath = locked || lockedOptionPathSet.has(key);
              const editableInput = editable && !lockedByPath;
              const valueText = editableInput ? "" : formatVisualOptionValue(value);

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
                        status={lockedByPath && filterStatus === "inactive" ? "passed" : filterStatus}
                        inactiveLabel={tComposer("notEnabled")}
                        ignoredLabel={tComposer("ignored")}
                        passedLabel={lockedByPath ? tComposer("locked") : tComposer("willPass")}
                        routeDependentLabel={tComposer("routeDependent")}
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
                  {!editableInput ? (
                    <code
                      className={cn(
                        "block max-w-full truncate justify-self-start rounded-md bg-muted/60 px-2 py-1 font-mono text-[11px] leading-none text-muted-foreground sm:justify-self-end",
                        ignored && "line-through",
                      )}
                      title={valueText}
                    >
                      {valueText}
                    </code>
                  ) : kind === "boolean" ? (
                    <Select
                      value={editableValue === true ? "true" : editableValue === false ? "false" : undefined}
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
                      value={(typeof editableValue === "string" || typeof editableValue === "number") && String(editableValue).trim()
                        ? String(editableValue)
                        : undefined}
                      onValueChange={(nextValue) => updateOptionValue(path, resolveSelectOptionValue(key, nextValue, selectValues))}
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
                      value={editableValue === null ? "" : String(editableValue)}
                      inputMode={kind === "number" ? "decimal" : undefined}
                      placeholder={placeholder ?? (kind === "number" ? (NUMBER_OPTION_PLACEHOLDERS[key] ?? "0.7") : key)}
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
      <Tooltip>
        <TooltipTrigger asChild>
          <InputGroupButton
            type="button"
            variant="ghost"
            size="icon-sm"
            className="size-7 rounded-md text-muted-foreground hover:text-foreground sm:size-8"
            disabled={disabled}
            onClick={openOptionsDialog}
            aria-label={tComposer("modelOptions")}
            onMouseEnter={() => setHovered(true)}
            onMouseLeave={() => setHovered(false)}
          >
            <Cog
              size={20}
              strokeWidth={1.4}
              animate={hovered ? "default" : false}
            />
          </InputGroupButton>
        </TooltipTrigger>
        <TooltipContent side="top" className="text-xs">
          {tComposer("modelOptions")}
        </TooltipContent>
      </Tooltip>

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
