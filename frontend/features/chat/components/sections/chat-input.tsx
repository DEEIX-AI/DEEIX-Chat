"use client";

import * as React from "react";
import dynamic from "next/dynamic";
import { Check, CircleHelp, Info } from "lucide-react";
import { useTranslations } from "next-intl";
import { toast } from "sonner";

import { AudioLines } from "@/components/animate-ui/icons/audio-lines";
import { Cog } from "@/components/animate-ui/icons/cog";
import { Pause } from "@/components/animate-ui/icons/pause";
import { Plus } from "@/components/animate-ui/icons/plus";
import { Link as LinkIcon } from "@/components/animate-ui/icons/link";
import { Crop } from "@/components/animate-ui/icons/crop";
import { Unplug } from "@/components/animate-ui/icons/unplug";
import { X as XIcon } from "@/components/animate-ui/icons/x";
import type {
  ChatModelOption,
  PendingAttachment,
  UploadingAttachment,
} from "@/features/chat/types/chat-runtime";
import { useSpeechInput } from "@/features/chat/hooks/use-speech-input";
import { ChatModelPicker } from "@/features/chat/components/sections/chat-model-picker";
import { formatBytes, resolveFileIcon } from "@/features/files/utils/file-display";
import {
  isReservedConversationOptionKey,
  sanitizeConversationOptions,
} from "@/features/chat/model/conversation-options";
import { isMediaSubmitTask, resolveChatSubmitTask } from "@/features/chat/model/chat-task";
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
import {
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuItem,
  DropdownMenuTrigger,
} from "@/components/ui/dropdown-menu";
import {
  InputGroup,
  InputGroupAddon,
  InputGroupButton,
  InputGroupTextarea,
} from "@/components/ui/input-group";
import { Input } from "@/components/ui/input";
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from "@/components/ui/select";
import { Popover, PopoverContent, PopoverTrigger } from "@/components/ui/popover";
import { Skeleton } from "@/components/ui/skeleton";
import { Tabs, TabsList, TabsTrigger } from "@/components/ui/tabs";
import { Tooltip, TooltipContent, TooltipTrigger } from "@/components/ui/tooltip";
import { resolveFileProcessingBadge, resolveFileProcessingToneClass } from "@/shared/lib/file-processing";
import { JsonCodeEditor } from "@/shared/components/json-code-editor";
import { cn } from "@/lib/utils";
import type { ConversationOptions } from "@/shared/api/conversation.types";
import type { MCPToolDTO } from "@/shared/api/mcp.types";
import type { ModelOptionPolicy } from "@/shared/lib/model-option-policy";
import { isModelOptionPathFiltered } from "@/shared/lib/model-option-policy";
import type { SendShortcut } from "@/features/settings/types/settings";
import { isSendShortcutEvent } from "@/shared/lib/platform-shortcuts";

const FilePreviewDialog = dynamic(
  () => import("@/features/files/components/preview/file-preview-dialog").then((module) => module.FilePreviewDialog),
  { ssr: false },
);

type ChatInputProps = {
  draft: string;
  loading: boolean;
  sending: boolean;
  uploading: boolean;
  isConversationMode: boolean;
  maxFilesPerMessage: number;
  fileMode?: "auto" | "full_context" | "rag";
  sendShortcut?: SendShortcut;
  inputHeight?: "compact" | "standard" | "loose";
  attachments: PendingAttachment[];
  uploadingAttachments: UploadingAttachment[];
  modelOptions: ChatModelOption[];
  selectedPlatformModelName: string;
  availableTools: MCPToolDTO[];
  selectedToolIDs: number[];
  toolsLoading: boolean;
  options: ConversationOptions;
  defaultOptions: ConversationOptions;
  modelOptionPolicy: ModelOptionPolicy | null;
  modelLoading: boolean;
  modelDisabled?: boolean;
  onDraftChange: (value: string) => void;
  onModelChange: (platformModelName: string) => void;
  onSelectedToolsChange: (toolIDs: number[]) => void;
  onOptionsChange: React.Dispatch<React.SetStateAction<ConversationOptions>>;
  onOptionsReset: () => void;
  onUploadFiles: (files: File[]) => void | Promise<void>;
  onCaptureScreenshot: () => void | Promise<void>;
  onRemoveAttachment: (fileID: string) => void;
  onSendMessage: () => void | Promise<void>;
  onStopMessage: () => void;
};

type EditableOptionValue = string | number | boolean | null;

type VisualOption = {
  key: string;
  path: string[];
  value: EditableOptionValue;
};

type FilteredOption = {
  key: string;
  value: unknown;
};

type NativeToolOption = {
  type: string;
  labelKey: string;
  descriptionKey: string;
  payload?: Record<string, unknown>;
};

type ModelOptionFilterStatus = "passed" | "filtered" | "unknown";

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
  "generationConfig.logprobs",
  "generationConfig.maxOutputTokens",
  "generationConfig.mediaResolution",
  "generationConfig.presencePenalty",
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
  "output_config.effort",
  "output_config.format.type",
  "presence_penalty",
  "reasoning.summary",
  "reasoning.effort",
  "reasoning_effort",
  "reasoning_summary",
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
] as const);

const XAI_NATIVE_TOOL_OPTIONS: NativeToolOption[] = [
  {
    type: "web_search",
    labelKey: "webSearch",
    descriptionKey: "grokWebSearch",
  },
  {
    type: "x_search",
    labelKey: "xSearch",
    descriptionKey: "grokXSearch",
  },
  {
    type: "code_interpreter",
    labelKey: "codeInterpreter",
    descriptionKey: "grokCodeInterpreter",
  },
];

const XAI_NATIVE_TOOL_TYPES = new Set(XAI_NATIVE_TOOL_OPTIONS.map((item) => item.type));

const OPENAI_NATIVE_TOOL_OPTIONS: NativeToolOption[] = [
  {
    type: "web_search",
    labelKey: "webSearch",
    descriptionKey: "openaiWebSearch",
    payload: { type: "web_search" },
  },
  {
    type: "shell",
    labelKey: "shell",
    descriptionKey: "openaiShell",
    payload: {
      type: "shell",
      environment: { type: "container_auto" },
    },
  },
  {
    type: "image_generation",
    labelKey: "imageGeneration",
    descriptionKey: "openaiImageGeneration",
    payload: { type: "image_generation" },
  },
  {
    type: "code_interpreter",
    labelKey: "codeInterpreter",
    descriptionKey: "openaiCodeInterpreter",
    payload: {
      type: "code_interpreter",
      container: { type: "auto" },
    },
  },
];

const ANTHROPIC_NATIVE_TOOL_OPTIONS: NativeToolOption[] = [
  {
    type: "web_search_20260209",
    labelKey: "webSearch",
    descriptionKey: "claudeWebSearch",
    payload: { type: "web_search_20260209", name: "web_search", allowed_callers: ["direct"] },
  },
  {
    type: "web_fetch_20260209",
    labelKey: "webFetch",
    descriptionKey: "claudeWebFetch",
    payload: { type: "web_fetch_20260209", name: "web_fetch", allowed_callers: ["direct"] },
  },
  {
    type: "code_execution_20260120",
    labelKey: "codeExecution",
    descriptionKey: "claudeCodeExecution",
    payload: { type: "code_execution_20260120", name: "code_execution" },
  },
  {
    type: "advisor_20260301",
    labelKey: "advisor",
    descriptionKey: "claudeAdvisor",
    payload: { type: "advisor_20260301", name: "advisor", model: "claude-opus-4-7" },
  },
  {
    type: "tool_search_tool_regex_20251119",
    labelKey: "toolSearchRegex",
    descriptionKey: "claudeToolSearchRegex",
    payload: { type: "tool_search_tool_regex_20251119", name: "tool_search_tool_regex" },
  },
  {
    type: "tool_search_tool_bm25_20251119",
    labelKey: "toolSearchBm25",
    descriptionKey: "claudeToolSearchBm25",
    payload: { type: "tool_search_tool_bm25_20251119", name: "tool_search_tool_bm25" },
  },
];

const NATIVE_TOOL_OPTIONS = [...XAI_NATIVE_TOOL_OPTIONS, ...OPENAI_NATIVE_TOOL_OPTIONS, ...ANTHROPIC_NATIVE_TOOL_OPTIONS];
const NATIVE_TOOL_TYPES = new Set(NATIVE_TOOL_OPTIONS.map((item) => item.type));

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
  "response_logprobs",
  "generationConfig.responseLogprobs",
  "logprobs",
  "generationConfig.logprobs",
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
  "response_format.type",
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
  "generationConfig.responseMimeType": ["text/plain", "application/json", "text/x.enum"],
  "generationConfig.thinkingConfig.thinkingLevel": ["low", "medium", "high"],
  "output_config.effort": ["low", "medium", "high"],
  "output_config.format.type": ["json_object", "json_schema", "text"],
  "reasoning.effort": ["low", "medium", "high"],
  "reasoning.summary": ["auto", "concise", "detailed"],
  reasoning_effort: ["minimal", "low", "medium", "high", "xhigh"],
  reasoning_summary: ["auto", "concise", "detailed"],
  "response_format.type": ["json_object", "json_schema", "text"],
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
  ["generationConfig", "responseMimeType"],
  ["generationConfig", "mediaResolution"],
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
  google_imagen: "Imagen",
  openai_chat_completions: "Chat Completions",
  openai_image_edits: "Image Edits",
  openai_image_generations: "Image Generations",
  openai_responses: "Responses",
  openai_video_generations: "Video Generations",
  replicate_predictions: "Predictions",
  stability_ai_generate: "Image Generation",
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

function hasProviderTool(options: ConversationOptions, type: string): boolean {
  return providerToolObjectsFromOptions(options).some((tool) => tool.type === type);
}

function setProviderToolEnabled(
  options: ConversationOptions,
  toolOption: NativeToolOption,
  enabled: boolean,
): ConversationOptions {
  const type = toolOption.type;
  if (!NATIVE_TOOL_TYPES.has(type)) {
    return options;
  }
  const tools = providerToolObjectsFromOptions(options);
  const hasTool = tools.some((tool) => tool.type === type);
  const nextTools = enabled
    ? hasTool
      ? tools
      : [...tools, { ...(toolOption.payload ?? { type }) }]
    : tools.filter((tool) => tool.type !== type);

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
    return [{ key: optionPathKey(path), path, value }];
  });
  const topLevelOptions = Object.entries(options).flatMap(([key, value]): VisualOption[] => {
    if (isReservedConversationOptionKey(key)) {
      return [];
    }
    if (isEditableOptionValue(value)) {
      return [{ key, path: [key], value }];
    }
    return [];
  });
  const merged = [...nestedOptions, ...topLevelOptions];
  const deduped = merged.filter((item, index) => merged.findIndex((candidate) => candidate.key === item.key) === index);
  return deduped.sort((left, right) => compareOptionKeys(left.key, right.key));
}

function resolveOptionTitle(key: string, translate: (key: string) => string): string {
  if (OPTION_LABEL_KEYS.has(key)) {
    try {
      return translate(key.replaceAll(".", "__"));
    } catch {
      return key;
    }
  }
  return key;
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

function resolveSelectValues(key: string): string[] {
  return Array.from(new Set((OPTION_SELECT_VALUES[key] ?? []).map((item) => item.trim()).filter(Boolean)));
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
  ignoredLabel,
  passedLabel,
}: {
  status: ModelOptionFilterStatus;
  ignoredLabel: string;
  passedLabel: string;
}) {
  if (status === "unknown") {
    return null;
  }
  return (
    <span
      data-filtered={status === "filtered"}
      className="shrink-0 rounded-md bg-emerald-500/10 px-1.5 py-0.5 text-[10px] leading-none text-emerald-700 data-[filtered=true]:bg-muted data-[filtered=true]:text-muted-foreground"
    >
      {status === "filtered" ? ignoredLabel : passedLabel}
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

function resolveMCPToolLabel(tool: MCPToolDTO, fallback: string): string {
  return tool.displayName.trim() || tool.name.trim() || fallback;
}

function resolveMCPToolServerName(tool: MCPToolDTO): string {
  return tool.serverName?.trim() ?? "";
}

function matchesMCPToolSearch(tool: MCPToolDTO, query: string): boolean {
  const normalizedQuery = query.trim().toLocaleLowerCase();
  if (!normalizedQuery) {
    return true;
  }
  return [
    resolveMCPToolLabel(tool, String(tool.id)),
    resolveMCPToolServerName(tool),
    tool.name,
    tool.description,
  ]
    .join(" ")
    .toLocaleLowerCase()
    .includes(normalizedQuery);
}

function clipboardFilesFromPaste(event: React.ClipboardEvent<HTMLTextAreaElement>): File[] {
  const itemFiles = Array.from(event.clipboardData.items ?? [])
    .filter((item) => item.kind === "file")
    .map((item) => item.getAsFile())
    .filter((file): file is File => file !== null);
  const sourceFiles = itemFiles.length > 0 ? itemFiles : Array.from(event.clipboardData.files ?? []);
  const pastedAt = Date.now();

  return sourceFiles.map((file, index) => {
    if (file.name.trim()) {
      return file;
    }
    const extension = file.type.startsWith("image/") ? ".png" : "";
    const prefix = file.type.startsWith("image/") ? "pasted-image" : "pasted-file";
    return new File([file], `${prefix}-${pastedAt}-${index + 1}${extension}`, {
      type: file.type,
      lastModified: file.lastModified,
    });
  });
}

function ChatInputComponent({
  draft,
  loading,
  sending,
  uploading,
  fileMode,
  sendShortcut = "enter",
  inputHeight = "standard",
  attachments,
  uploadingAttachments,
  modelOptions,
  selectedPlatformModelName,
  availableTools,
  selectedToolIDs,
  toolsLoading,
  options,
  defaultOptions,
  modelOptionPolicy,
  modelLoading,
  modelDisabled = false,
  onDraftChange,
  onModelChange,
  onSelectedToolsChange,
  onOptionsChange,
  onOptionsReset,
  onUploadFiles,
  onCaptureScreenshot,
  onRemoveAttachment,
  onSendMessage,
  onStopMessage,
}: ChatInputProps) {
  const tCommon = useTranslations("common.actions");
  const tComposer = useTranslations("chat.composer");
  const tFileStatus = useTranslations("files.status");
  const tOptionLabels = useTranslations("chat.optionLabels");
  const tNativeToolLabels = useTranslations("chat.nativeToolLabels");
  const tNativeToolDescriptions = useTranslations("chat.nativeToolDescriptions");
  const [isPlusHovered, setIsPlusHovered] = React.useState(false);
  const [isModelConfigHovered, setIsModelConfigHovered] = React.useState(false);
  const [isMCPToolsHovered, setIsMCPToolsHovered] = React.useState(false);
  const [isVoiceHovered, setIsVoiceHovered] = React.useState(false);
  const speechInput = useSpeechInput({ draft, onDraftChange });
  const [hoveredTool, setHoveredTool] = React.useState<"upload" | "screenshot" | null>(null);
  const [mcpToolsOpen, setMCPToolsOpen] = React.useState(false);
  const [mcpToolSearch, setMCPToolSearch] = React.useState("");
  const [ragWarnDismissed, setRagWarnDismissed] = React.useState(false);
  const [optionsDialogOpen, setOptionsDialogOpen] = React.useState(false);
  const [optionsDraft, setOptionsDraft] = React.useState("");
  const [optionsObject, setOptionsObject] = React.useState<ConversationOptions>({});
  const [filteredOptions, setFilteredOptions] = React.useState<FilteredOption[]>([]);
  const [optionsMobileView, setOptionsMobileView] = React.useState<"json" | "visual">("json");
  const [previewAttachment, setPreviewAttachment] = React.useState<PendingAttachment | null>(null);
  const fileInputRef = React.useRef<HTMLInputElement | null>(null);
  const optionsObjectRef = React.useRef<ConversationOptions>({});
  const composingRef = React.useRef(false);
  const canSend = (draft.trim().length > 0 || attachments.length > 0) && !sending && !loading && !uploading;
  const inputHeightClassName =
    inputHeight === "compact" ? "max-h-32" : inputHeight === "loose" ? "max-h-64" : "max-h-44";

  // Only relevant in RAG mode: all document attachments opted out of RAG.
  const docAttachments = attachments.filter((a) => a.fileCategory !== "image");
  const allRagOptOut =
    fileMode === "rag" &&
    docAttachments.length > 0 &&
    docAttachments.every((a) => a.ragOptOut === true);
  const showRagWarn = allRagOptOut && !ragWarnDismissed;

  const closePreviewDialog = React.useCallback((open: boolean) => {
    if (!open) {
      setPreviewAttachment(null);
    }
  }, []);

  const selectedModel = React.useMemo(
    () => modelOptions.find((item) => item.platformModelName === selectedPlatformModelName) ?? null,
    [modelOptions, selectedPlatformModelName],
  );
  const selectedProtocol = selectedModel?.protocols[0]?.trim() ?? "";
  const selectedProtocolLabel = selectedProtocol ? resolveProtocolLabel(selectedProtocol) : "";
  const submitTask = resolveChatSubmitTask(selectedModel, attachments);
  const isMediaMode = isMediaSubmitTask(submitTask);
  const modelOptionPolicyDisabled = modelOptionPolicy?.mode?.trim() === "disabled";
  const selectedToolIDSet = React.useMemo(() => new Set(selectedToolIDs), [selectedToolIDs]);
  const selectedToolCount = selectedToolIDs.length;
  const filteredMCPTools = React.useMemo(
    () => availableTools.filter((tool) => matchesMCPToolSearch(tool, mcpToolSearch)),
    [availableTools, mcpToolSearch],
  );
  const showMCPToolsButton = availableTools.length > 0 && !isMediaMode;
  const editableOptions = React.useMemo(() => visualOptionsFromOptions(optionsObject), [optionsObject]);
  const nativeToolGroup = React.useMemo(() => {
    if (isMediaMode) {
      return null;
    }
    if (selectedProtocol === "xai_responses") {
      return {
        title: tComposer("nativeTools.grok"),
        options: XAI_NATIVE_TOOL_OPTIONS,
      };
    }
    if (selectedProtocol === "openai_responses") {
      return {
        title: tComposer("nativeTools.openai"),
        options: OPENAI_NATIVE_TOOL_OPTIONS,
      };
    }
    if (selectedProtocol === "anthropic_messages") {
      return {
        title: tComposer("nativeTools.claude"),
        options: ANTHROPIC_NATIVE_TOOL_OPTIONS,
      };
    }
    return null;
  }, [isMediaMode, selectedProtocol, tComposer]);
  const hasRecognizedOptions = Boolean(nativeToolGroup) || editableOptions.length > 0 || filteredOptions.length > 0;
  React.useEffect(() => {
    optionsObjectRef.current = optionsObject;
  }, [optionsObject]);
  React.useEffect(() => {
    if (modelOptionPolicyDisabled) {
      setOptionsDialogOpen(false);
    }
  }, [modelOptionPolicyDisabled]);
  const openOptionsDialog = React.useCallback(() => {
    const sanitized = sanitizeConversationOptions(options);
    optionsObjectRef.current = sanitized;
    setOptionsObject(sanitized);
    setFilteredOptions([]);
    setOptionsDraft(stringifyOptions(sanitized));
    setOptionsMobileView("json");
    setOptionsDialogOpen(true);
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
    (tool: NativeToolOption, enabled: boolean) => {
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
  const saveOptionsDraft = React.useCallback(() => {
    const parsed = parseOptionsDraft(optionsDraft);
    if (!parsed.options) {
      setOptionsMobileView("json");
      toast.error(tComposer("saveFailed"));
      return;
    }
    if (JSON.stringify(parsed.options) === JSON.stringify(defaultOptions)) {
      onOptionsReset();
      setOptionsDialogOpen(false);
      return;
    }
    onOptionsChange(parsed.options);
    setOptionsDialogOpen(false);
  }, [defaultOptions, onOptionsChange, onOptionsReset, optionsDraft, tComposer]);
  const onSelectUploadTool = React.useCallback(() => {
    fileInputRef.current?.click();
  }, []);

  const onSelectScreenshotTool = React.useCallback(() => {
    void onCaptureScreenshot();
  }, [onCaptureScreenshot]);
  const toggleMCPTool = React.useCallback(
    (toolID: number, checked: boolean) => {
      onSelectedToolsChange(
        checked
          ? [...selectedToolIDs, toolID]
          : selectedToolIDs.filter((item) => item !== toolID),
      );
    },
    [onSelectedToolsChange, selectedToolIDs],
  );

  const renderOptionsViewToggle = () => (
    <Tabs
      value={optionsMobileView}
      onValueChange={(value) => setOptionsMobileView(value as "json" | "visual")}
      className="w-fit gap-0"
    >
      <TabsList className="h-7">
        <TabsTrigger value="json">JSON</TabsTrigger>
        <TabsTrigger value="visual">{tComposer("visual")}</TabsTrigger>
      </TabsList>
    </Tabs>
  );

  const renderOptionsEditor = () => (
    <div className="space-y-1.5">
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
      <div className="p-0.5">
        <JsonCodeEditor
          value={optionsDraft}
          onChange={handleOptionsJSONChange}
          height="min(52dvh, 420px)"
          className="min-h-56"
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
                onClick={() => replaceOptionsDraft(defaultOptions)}
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
    <div className="flex min-h-0 flex-col space-y-1.5 md:border-l md:pl-5">
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
        <div className="min-h-0 flex-1 overflow-y-auto pr-1 md:h-[min(52dvh,420px)] md:max-h-[min(52dvh,420px)]">
          <div className="space-y-2 md:space-y-2.5">
            {nativeToolGroup ? (
              <div className="space-y-1.5 px-2 py-1.5">
                <div className="min-w-0">
                  <p className="truncate text-xs text-foreground/80">{nativeToolGroup.title}</p>
                </div>
                <div className="space-y-1">
                  {nativeToolGroup.options.map((tool) => {
                    const checked = hasProviderTool(optionsObject, tool.type);
                    const filterStatus = resolveModelOptionFilterStatus(modelOptionPolicy, selectedProtocol, "tools");
                    const ignored = filterStatus === "filtered";
                    return (
                      <label
                        key={tool.type}
                        className={cn(
                          "flex min-w-0 cursor-pointer items-center gap-2 rounded-md px-2 py-1.5 hover:bg-muted/50",
                          ignored && "text-muted-foreground",
                        )}
                      >
                        <Checkbox
                          checked={checked}
                          onCheckedChange={(nextChecked) => updateProviderTool(tool, nextChecked === true)}
                        />
                        <span className="min-w-0 flex flex-1 items-center gap-2 text-xs">
                          <span
                            className={cn(
                              "shrink-0 text-foreground/80",
                              ignored && "text-muted-foreground line-through",
                            )}
                          >
                            {tNativeToolLabels(tool.labelKey)}
                          </span>
                          <span className={cn("min-w-0 truncate text-[11px] text-muted-foreground", ignored && "line-through")}>
                            {tNativeToolDescriptions(tool.descriptionKey)}
                          </span>
                        </span>
                        <ModelOptionFilterBadge
                          status={filterStatus}
                          ignoredLabel={tComposer("ignored")}
                          passedLabel={tComposer("willPass")}
                        />
                      </label>
                    );
                  })}
                </div>
              </div>
            ) : null}
            {editableOptions.map(({ key, path, value }) => {
              const kind = resolveOptionKind(key, value);
              const selectValues = resolveSelectValues(key);
              const title = resolveOptionTitle(key, tOptionLabels);
              const filterStatus = resolveModelOptionFilterStatus(modelOptionPolicy, selectedProtocol, key);
              const ignored = filterStatus === "filtered";

              return (
                <div
                  key={key}
                  className={cn(
                    "grid grid-cols-[minmax(0,1fr)_116px] items-center gap-2 rounded-md px-2 py-1.5 sm:grid-cols-[minmax(0,1fr)_132px] sm:gap-3 md:grid-cols-[minmax(0,1fr)_148px]",
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
                        ignoredLabel={tComposer("ignored")}
                        passedLabel={tComposer("willPass")}
                      />
                    </div>
                    {title !== key ? (
                      <p
                        className={cn("truncate text-[11px] text-muted-foreground", ignored && "line-through")}
                      >
                        {key}
                      </p>
                    ) : null}
                  </div>
                  {kind === "boolean" ? (
                    <Select
                      value={value === true ? "true" : "false"}
                      onValueChange={(nextValue) => updateOptionValue(path, nextValue === "true")}
                    >
                      <SelectTrigger size="sm">
                        <SelectValue />
                      </SelectTrigger>
                      <SelectContent>
                        <SelectItem value="true">{tComposer("booleanOn")}</SelectItem>
                        <SelectItem value="false">{tComposer("booleanOff")}</SelectItem>
                      </SelectContent>
                    </Select>
                  ) : kind === "select" ? (
                    <Select value={String(value)} onValueChange={(nextValue) => updateOptionValue(path, nextValue)}>
                      <SelectTrigger size="sm">
                        <SelectValue />
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
                      placeholder={kind === "number" ? "0.7" : key}
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
              <div className={cn("space-y-2", editableOptions.length > 0 ? "pt-1" : "")}>
                {filteredOptions.map((item) => (
                  <div
                    key={item.key}
                    className="grid grid-cols-[minmax(0,1fr)_116px] items-center gap-2 rounded-md px-2 py-1.5 text-muted-foreground sm:grid-cols-[minmax(0,1fr)_132px] sm:gap-3 md:grid-cols-[minmax(0,1fr)_148px]"
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
        <div className="flex h-40 min-h-0 flex-1 items-center justify-center text-xs text-muted-foreground md:h-[min(52dvh,420px)]">
          {tComposer("noVisualFields")}
        </div>
      )}
    </div>
  );

  return (
    <div className="w-full">
      <input
        ref={fileInputRef}
        type="file"
        multiple
        className="sr-only "
        onChange={(event) => {
          const files = Array.from(event.target.files ?? []);
          if (files.length > 0) {
            void onUploadFiles(files);
          }
          event.currentTarget.value = "";
        }}
      />

      <InputGroup
        className={cn(
          "bg-pure rounded-3xl border-[0.5px] border-border/70 shadow-xs has-[[data-slot=input-group-control]:focus-visible]:ring-0 has-[[data-slot=input-group-control]:focus-visible]:border-border",
        )}
      >
        {attachments.length > 0 || uploadingAttachments.length > 0 ? (
          <div className="w-full space-y-2 px-2.5 pt-2">
            {showRagWarn ? (
              <div className="flex items-center gap-2 rounded-lg border border-amber-200/70 bg-amber-50/70 px-3 py-2 text-[11px] text-amber-700 dark:border-amber-700/40 dark:bg-amber-950/30 dark:text-amber-400">
                <span className="shrink-0">⚠</span>
                <span className="flex-1">{tComposer("ragAllDisabled")}</span>
                <button
                  type="button"
                  className="shrink-0 text-amber-500 hover:text-amber-700 dark:text-amber-500 dark:hover:text-amber-300"
                  onClick={() => setRagWarnDismissed(true)}
                  aria-label={tComposer("closeHint")}
                >
                  ✕
                </button>
              </div>
            ) : null}
            <div className="w-full overflow-x-auto">
              <div className="flex w-max gap-2 px-1.5 pb-1 pt-2">
                {attachments.map((item) => (
                  <div
                    key={item.fileID}
                    role="button"
                    tabIndex={0}
                    className="bg-pure group relative flex h-14 w-[212px] shrink-0 items-center gap-2.5 rounded-lg border border-border/50 bg-background/95 px-2.5 text-left shadow-[0_1px_2px_rgba(0,0,0,0.025)] transition-colors hover:border-border hover:bg-accent/30 sm:w-[228px]"
                    onClick={() => setPreviewAttachment(item)}
                    onKeyDown={(event) => {
                      if (event.key === "Enter" || event.key === " ") {
                        event.preventDefault();
                        setPreviewAttachment(item);
                      }
                    }}
                  >
                    <button
                      type="button"
                      className="bg-pure absolute -right-1.5 -top-1.5 z-20 inline-flex size-5 items-center justify-center rounded-full border border-border text-muted-foreground opacity-0 shadow-sm transition-opacity duration-150 group-hover:opacity-100 focus:opacity-100 hover:bg-accent hover:text-foreground"
                      onClick={(event) => {
                        event.stopPropagation();
                        onRemoveAttachment(item.fileID);
                      }}
                      aria-label={tComposer("removeAttachment", { name: item.fileName })}
                    >
                      <XIcon size={14} strokeWidth={1.8} animateOnHover="default" />
                    </button>
                    {(() => {
                      const badge = resolveFileProcessingBadge(item, (key, values) => tFileStatus(key, values));
                      const FileIcon = resolveFileIcon(item);
                      return (
                        <>
                          <div className="flex size-6 shrink-0 items-center justify-center">
                            <FileIcon className="size-5 text-muted-foreground" strokeWidth={1.6} />
                          </div>
                          <div className="flex min-w-0 flex-1 flex-col justify-center">
                            <p className="truncate text-[12px] font-medium leading-4 text-foreground/90" title={item.fileName}>
                              {item.fileName}
                            </p>
                            <div className="mt-1 flex min-w-0 items-center gap-1.5">
                              <span className="min-w-0 shrink truncate text-[10px] leading-none text-muted-foreground">
                                {formatBytes(item.sizeBytes)}
                              </span>
                              <span
                                className={cn(
                                  "inline-flex max-w-[82px] shrink-0 items-center rounded-md px-1.5 py-0.5 text-[10px] font-medium leading-none",
                                  resolveFileProcessingToneClass(badge.tone),
                                )}
                                title={badge.detail}
                              >
                                <span className="truncate">{badge.label}</span>
                              </span>
                              {item.ragOptOut && item.fileCategory !== "image" ? (
                                <span
                                  className="shrink-0 rounded-md bg-muted/60 px-1.5 py-0.5 text-[10px] font-medium leading-none text-muted-foreground/65"
                                  title={tComposer("ragDisabledTitle")}
                                >
                                  {tComposer("ragOff")}
                                </span>
                              ) : null}
                            </div>
                          </div>
                        </>
                      );
                    })()}
                  </div>
                ))}
                {uploadingAttachments.map((item) => (
                  <div
                    key={item.tempID}
                    className="bg-pure relative flex h-14 w-[212px] shrink-0 items-center gap-2.5 rounded-lg border border-border/50 bg-background/95 px-2.5 sm:w-[228px]"
                    aria-label={tComposer("uploadingAttachment", { name: item.fileName })}
                  >
                    <Skeleton className="size-5 shrink-0 rounded-sm" />
                    <div className="min-w-0 flex-1 space-y-2">
                      <Skeleton className="h-3 w-[78%]" />
                      <div className="flex items-center gap-1.5">
                        <Skeleton className="h-2.5 w-10" />
                        <Skeleton className="h-4 w-12 rounded-md" />
                      </div>
                    </div>
                  </div>
                ))}
              </div>
            </div>
            {previewAttachment ? (
              <FilePreviewDialog
                file={previewAttachment}
                open={previewAttachment !== null}
                onOpenChange={closePreviewDialog}
              />
            ) : null}
          </div>
        ) : null}

        <InputGroupTextarea
          value={draft}
          disabled={sending || loading || uploading}
          readOnly={speechInput.active}
          placeholder={speechInput.placeholder}
          rows={1}
          style={{ fontFamily: "var(--font-chat)", fontWeight: "var(--font-chat-weight)" }}
          className={cn(
            "rounded-3xl min-h-12 overflow-y-auto px-5 pt-4 text-[15px] placeholder:text-base placeholder:font-medium",
            inputHeightClassName,
            speechInput.active ? "placeholder:font-normal placeholder:text-muted-foreground" : "",
          )}
          onChange={(event) => onDraftChange(event.target.value)}
          onPaste={(event) => {
            const files = clipboardFilesFromPaste(event);
            if (files.length === 0) {
              return;
            }
            if (!event.clipboardData.getData("text/plain")) {
              event.preventDefault();
            }
            void onUploadFiles(files);
          }}
          onCompositionStart={() => {
            composingRef.current = true;
          }}
          onCompositionEnd={() => {
            composingRef.current = false;
          }}
          onKeyDown={(event) => {
            if (event.nativeEvent.isComposing || composingRef.current || event.key === "Process" || event.keyCode === 229) {
              return;
            }
            const shouldSend = isSendShortcutEvent(sendShortcut, event);

            if (shouldSend) {
              event.preventDefault();
              if (canSend) {
                void onSendMessage();
              }
            }
          }}
        />

        <InputGroupAddon align="block-end" className="items-center justify-between pt-2">
          <div className="flex items-center gap-1">
            <DropdownMenu modal={false}>
              <DropdownMenuTrigger asChild>
                <InputGroupButton
                  id="chat-tools-menu-trigger"
                  type="button"
                  variant="ghost"
                  size="icon-sm"
                  className="rounded-md text-muted-foreground hover:text-foreground"
                  disabled={sending || loading || uploading}
                  aria-label={tComposer("openTools")}
                  onMouseEnter={() => setIsPlusHovered(true)}
                  onMouseLeave={() => setIsPlusHovered(false)}
                >
                  <Plus
                    size={20}
                    strokeWidth={1.4}
                    animate={isPlusHovered ? "default" : undefined}
                  />
                </InputGroupButton>
              </DropdownMenuTrigger>
              <DropdownMenuContent side="bottom" align="start" sideOffset={8} className="w-36">
                <DropdownMenuItem
                  onMouseEnter={() => setHoveredTool("upload")}
                  onMouseLeave={() => setHoveredTool((prev) => (prev === "upload" ? null : prev))}
                  onSelect={(event) => {
                    event.preventDefault();
                    onSelectUploadTool();
                  }}
                >
                  <LinkIcon size={12} strokeWidth={1.5} animate={hoveredTool === "upload" ? "default" : undefined} />
                  {tComposer("uploadFile")}
                </DropdownMenuItem>
                <DropdownMenuItem
                  onMouseEnter={() => setHoveredTool("screenshot")}
                  onMouseLeave={() => setHoveredTool((prev) => (prev === "screenshot" ? null : prev))}
                  onSelect={(event) => {
                    event.preventDefault();
                    onSelectScreenshotTool();
                  }}
                >
                  <Crop size={12} strokeWidth={1.5} animate={hoveredTool === "screenshot" ? "default" : undefined} />
                  {tComposer("screenshot")}
                </DropdownMenuItem>
              </DropdownMenuContent>
            </DropdownMenu>

            {!modelOptionPolicyDisabled ? (
              <InputGroupButton
                type="button"
                variant="ghost"
                size="icon-sm"
                className="rounded-md text-muted-foreground hover:text-foreground"
                disabled={sending || loading || uploading || modelLoading}
                onClick={openOptionsDialog}
                aria-label={tComposer("modelOptions")}
                title={tComposer("modelOptions")}
                onMouseEnter={() => setIsModelConfigHovered(true)}
                onMouseLeave={() => setIsModelConfigHovered(false)}
              >
                <Cog
                  size={20}
                  strokeWidth={1.4}
                  animate={isModelConfigHovered ? "default" : undefined}
                />
              </InputGroupButton>
            ) : null}

            {showMCPToolsButton ? (
              <Popover open={mcpToolsOpen} onOpenChange={setMCPToolsOpen}>
                <PopoverTrigger asChild>
                  <InputGroupButton
                    type="button"
                    variant="ghost"
                    size="icon-sm"
                    className="relative rounded-md text-muted-foreground hover:text-foreground"
                    disabled={sending || loading || uploading || toolsLoading}
                    aria-label={tComposer("mcpTools")}
                    title={selectedToolCount > 0 ? tComposer("mcpToolsSelected", { count: selectedToolCount }) : tComposer("mcpTools")}
                    onMouseEnter={() => setIsMCPToolsHovered(true)}
                    onMouseLeave={() => setIsMCPToolsHovered(false)}
                  >
                    <Unplug
                      size={20}
                      strokeWidth={1.4}
                      animate={isMCPToolsHovered ? "default" : undefined}
                    />
                    {selectedToolCount > 0 ? (
                      <span className="absolute -right-0.5 -top-0.5 flex h-3.5 min-w-3.5 items-center justify-center rounded-full bg-primary px-1 text-[9px] font-medium leading-none text-primary-foreground">
                        {selectedToolCount}
                      </span>
                    ) : null}
                  </InputGroupButton>
                </PopoverTrigger>
                <PopoverContent
                  side="bottom"
                  align="start"
                  sideOffset={8}
                  data-mcp-tools-popover-content
                  className="w-80 p-1.5"
                  onPointerDownOutside={(event) => {
                    const target = event.target as HTMLElement | null;
                    if (target?.closest("[data-mcp-tools-popover-content]")) {
                      event.preventDefault();
                    }
                  }}
                  onFocusOutside={(event) => {
                    const target = event.target as HTMLElement | null;
                    if (target?.closest("[data-mcp-tools-popover-content]")) {
                      event.preventDefault();
                    }
                  }}
                >
                  <div className="flex items-center justify-between gap-3 px-2 pb-1.5 text-[11px] font-medium">
                    <span>{tComposer("mcpTools")}</span>
                    {selectedToolCount > 0 ? (
                      <button
                        type="button"
                        className="text-[11px] text-muted-foreground transition-colors hover:text-foreground"
                        onClick={() => onSelectedToolsChange([])}
                      >
                        {tComposer("clear")}
                      </button>
                    ) : null}
                  </div>
                  <div
                    className="px-0.5 py-1"
                    onPointerDown={(event) => event.stopPropagation()}
                    onMouseDown={(event) => event.stopPropagation()}
                    onClick={(event) => event.stopPropagation()}
                  >
                    <Input
                      value={mcpToolSearch}
                      onChange={(event) => setMCPToolSearch(event.target.value)}
                      onKeyDown={(event) => event.stopPropagation()}
                      className="bg-background"
                      placeholder={tComposer("searchToolsPlaceholder")}
                    />
                  </div>
                  <div className="max-h-72 overflow-y-auto px-0.5 pt-1">
                    {filteredMCPTools.map((tool) => {
                      const checked = selectedToolIDSet.has(tool.id);
                      const label = resolveMCPToolLabel(tool, tComposer("tool", { id: tool.id }));
                      const serverName = resolveMCPToolServerName(tool);
                      const description = (tool.description ?? "").trim() || tComposer("noToolDescription");
                      return (
                        <div
                          key={tool.id}
                          data-selected={checked}
                          className="group mb-1 flex h-7 items-center justify-between rounded-md text-[11px] font-medium text-muted-foreground data-[selected=true]:bg-accent data-[selected=true]:text-accent-foreground"
                        >
                          <button
                            type="button"
                            className="flex h-full min-w-0 flex-1 items-center gap-1.5 rounded-md px-1.5 text-left outline-none transition-colors hover:bg-accent hover:text-accent-foreground focus-visible:bg-accent focus-visible:text-accent-foreground"
                            onClick={() => toggleMCPTool(tool.id, !checked)}
                          >
                            <span className="min-w-0 truncate text-xs text-current">{label}</span>
                            {serverName ? (
                              <span className="shrink-0 rounded bg-muted px-1 py-0.5 text-[10px] leading-none text-muted-foreground transition-colors group-data-[selected=true]:text-current">
                                {serverName}
                              </span>
                            ) : null}
                          </button>
                          <span className="flex size-3 shrink-0 items-center justify-center text-current">
                            {checked ? <Check className="size-3 text-current" strokeWidth={1.7} /> : null}
                          </span>
                          <Tooltip>
                            <TooltipTrigger asChild>
                              <button
                                type="button"
                                aria-label={tComposer("viewToolDescription")}
                                className="ml-1 flex size-6 shrink-0 items-center justify-center rounded-md text-current outline-none transition-colors hover:bg-accent hover:text-accent-foreground focus-visible:bg-accent focus-visible:text-accent-foreground"
                              >
                                <Info className="size-3.5" strokeWidth={1.8} />
                              </button>
                            </TooltipTrigger>
                            <TooltipContent
                              side="right"
                              align="center"
                              sideOffset={8}
                              className="max-w-72 whitespace-normal text-left text-xs leading-5 [text-wrap:auto]"
                            >
                              {description}
                            </TooltipContent>
                          </Tooltip>
                        </div>
                      );
                    })}
                    {filteredMCPTools.length === 0 ? (
                      <div className="px-2 py-6 text-center text-xs text-muted-foreground">
                        {tComposer("noMatchingTools")}
                      </div>
                    ) : null}
                  </div>
                </PopoverContent>
              </Popover>
            ) : null}
          </div>

          <div className="flex min-w-0 flex-1 items-center justify-end gap-1.5">
            <ChatModelPicker
              modelOptions={modelOptions}
              selectedPlatformModelName={selectedPlatformModelName}
              loading={modelLoading}
              disabled={modelDisabled}
              onModelChange={onModelChange}
            />

            <InputGroupButton
              type="button"
              variant="ghost"
              size="icon-sm"
              className="rounded-md text-muted-foreground hover:text-foreground"
              disabled={loading || uploading || (!sending && !speechInput.supported)}
              onClick={sending ? onStopMessage : speechInput.toggle}
              onMouseEnter={() => setIsVoiceHovered(true)}
              onMouseLeave={() => setIsVoiceHovered(false)}
              aria-label={sending ? tComposer("pauseGeneration") : speechInput.active ? tComposer("cancelVoiceInput") : tComposer("voiceInput")}
              title={sending ? tComposer("pauseGeneration") : speechInput.supported ? (speechInput.active ? tComposer("cancelVoiceInput") : tComposer("voiceInput")) : tComposer("voiceUnsupported")}
            >
              {sending ? (
                <Pause
                  size={20}
                  strokeWidth={1.4}
                  animate="default-loop"
                />
              ) : speechInput.active ? (
                <AudioLines
                  size={20}
                  strokeWidth={1.4}
                  animate="default"
                />
              ) : (
                <AudioLines
                  size={20}
                  strokeWidth={1.4}
                  animate={isVoiceHovered ? "default" : undefined}
                />
              )}
            </InputGroupButton>
          </div>
        </InputGroupAddon>
      </InputGroup>
      {!modelOptionPolicyDisabled ? (
        <Dialog open={optionsDialogOpen} onOpenChange={setOptionsDialogOpen}>
          <DialogContent
            className="flex max-h-[calc(100dvh-1rem)] w-[calc(100vw-1rem)] flex-col overflow-hidden p-4 sm:max-h-[min(92vh,760px)] sm:w-full sm:max-w-[800px] sm:p-6 md:max-w-[900px]"
          >
            <DialogHeader className="shrink-0">
              <div className="flex min-w-0 items-center justify-between gap-3">
                <DialogTitle className="shrink-0">{tComposer("modelOptions")}</DialogTitle>
                <div className="shrink-0 md:hidden">{renderOptionsViewToggle()}</div>
              </div>
              <DialogDescription className="hidden md:block">
                {tComposer("dialogDescription", { model: selectedModel?.platformModelName || selectedPlatformModelName || tComposer("currentModel") })}
              </DialogDescription>
            </DialogHeader>

            <form
              className="flex min-h-0 flex-1 flex-col gap-4"
              onSubmit={(event) => {
                event.preventDefault();
                saveOptionsDraft();
              }}
            >
              <div className="min-h-0 flex-1 overflow-y-auto sm:pr-1">
                <div className="space-y-3 md:hidden">
                  {optionsMobileView === "json" ? renderOptionsEditor() : renderOptionsVisualFields()}
                </div>

                <div className="hidden min-w-0 gap-4 md:grid md:grid-cols-[minmax(0,430px)_minmax(300px,1fr)] md:gap-5">
                  {renderOptionsEditor()}
                  {renderOptionsVisualFields()}
                </div>
              </div>
              <DialogFooter className="shrink-0">
                <Button type="button" variant="ghost" onClick={() => setOptionsDialogOpen(false)}>
                  {tCommon("cancel")}
                </Button>
                <Button type="submit">
                  {tCommon("save")}
                </Button>
              </DialogFooter>
            </form>
          </DialogContent>
        </Dialog>
      ) : null}
    </div>
  );
}

export const ChatInput = React.memo(ChatInputComponent);
ChatInput.displayName = "ChatInput";
