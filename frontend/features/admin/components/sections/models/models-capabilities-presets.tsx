"use client";

import { useState } from "react";
import { Check, Copy, Search } from "lucide-react";
import { toast } from "sonner";

import { Badge } from "@/components/ui/badge";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from "@/components/ui/dialog";
import { Tabs, TabsContent, TabsList, TabsTrigger } from "@/components/ui/tabs";
import { cn } from "@/lib/utils";
import type { AdminLLMAdapter, AdminLLMModelDTO } from "@/features/admin/api/llm.types";
import { parseProtocolsJSON } from "@/shared/lib/model-protocols";
import { MODEL_OPTION_POLICY_PROTOCOL_LABELS, resolveModelOptionPolicyProtocol } from "@/shared/lib/model-option-policy";

type CapabilityPreset = {
  id: string;
  protocol: AdminLLMAdapter;
  payload: Record<string, unknown>;
};

const MODEL_CAPABILITY_PRESETS: CapabilityPreset[] = [
  {
    id: "openai_responses",
    protocol: "openai_responses",
    payload: {
      defaultOptions: {
        reasoning: {
          effort: "high",
          summary: "auto",
        },
        text: {
          verbosity: "medium",
        },
        store: false,
      },
      optionControls: [
        {
          path: "reasoning.effort",
          type: "select",
          label: "Reasoning Effort",
          options: ["minimal", "low", "medium", "high", "xhigh"],
        },
        {
          path: "reasoning.summary",
          type: "select",
          label: "Reasoning Summary",
          options: ["auto", "concise", "detailed"],
        },
        {
          path: "text.verbosity",
          type: "select",
          label: "Text Verbosity",
          options: ["low", "medium", "high"],
        },
        {
          path: "store",
          type: "boolean",
          label: "Store",
        },
      ],
      nativeTools: [
        {
          key: "openai.code_interpreter",
          protocols: ["openai_responses"],
          label: "Code Interpreter",
          enabled: true,
          defaultEnabled: true,
          payload: {
            container: {
              type: "auto",
            },
            type: "code_interpreter",
          },
          provider: "OpenAI",
          type: "code_interpreter",
          description: "OpenAI hosted code interpreter with an automatic container.",
        },
        {
          key: "openai.web_search",
          protocols: ["openai_chat_completions", "openai_responses"],
          label: "Web Search",
          enabled: true,
          defaultEnabled: true,
          payload: {
            type: "web_search",
          },
          provider: "OpenAI",
          type: "web_search",
          description: "OpenAI hosted web search.",
        },
      ],
    },
  },
  {
    id: "anthropic_messages",
    protocol: "anthropic_messages",
    payload: {
      defaultOptions: {
        max_tokens: 64000,
        thinking: {
          type: "adaptive",
          display: "summarized",
        },
        output_config: {
          effort: "high",
        },
        cache_control: {
          type: "ephemeral",
          ttl: "5m",
        },
      },
      optionControls: [
        {
          path: "max_tokens",
          type: "number",
          label: "Max Tokens",
        },
        {
          path: "thinking.type",
          type: "select",
          label: "Thinking Type",
          options: ["adaptive"],
        },
        {
          path: "thinking.display",
          type: "select",
          label: "Thinking Display",
          options: ["summarized", "omitted"],
        },
        {
          path: "output_config.effort",
          type: "select",
          label: "Output Config Effort",
          options: ["low", "medium", "high", "xhigh", "max"],
        },
        {
          path: "cache_control.type",
          type: "select",
          label: "Cache Control Type",
          options: ["ephemeral"],
        },
        {
          path: "cache_control.ttl",
          type: "select",
          label: "Cache Control TTL",
          options: ["5m", "1h"],
        },
      ],
      nativeTools: [
        {
          key: "anthropic.web_fetch_20260209",
          protocols: ["anthropic_messages"],
          label: "Web Fetch",
          enabled: true,
          defaultEnabled: true,
          payload: {
            allowed_callers: ["direct"],
            name: "web_fetch",
            type: "web_fetch_20260209",
          },
          provider: "Anthropic",
          type: "web_fetch_20260209",
          description: "Anthropic hosted web fetch tool.",
        },
        {
          key: "anthropic.web_search_20260209",
          protocols: ["anthropic_messages"],
          label: "Web Search",
          enabled: true,
          defaultEnabled: true,
          payload: {
            allowed_callers: ["direct"],
            name: "web_search",
            type: "web_search_20260209",
          },
          provider: "Anthropic",
          type: "web_search_20260209",
          description: "Anthropic hosted web search tool.",
        },
      ],
    },
  },
  {
    id: "google_generate_content",
    protocol: "google_generate_content",
    payload: {
      defaultOptions: {
        generationConfig: {
          thinkingConfig: {
            includeThoughts: true,
            thinkingLevel: "high",
          },
        },
      },
      optionControls: [
        {
          path: "generationConfig.thinkingConfig.includeThoughts",
          type: "boolean",
        },
        {
          path: "generationConfig.thinkingConfig.thinkingLevel",
          type: "select",
          label: "Thinking Level",
          options: ["low", "medium", "high"],
        },
      ],
      nativeTools: [
        {
          key: "google.code_execution",
          protocols: ["google_generate_content", "gemini_generate_content"],
          label: "Code Execution",
          enabled: true,
          defaultEnabled: true,
          payload: {
            code_execution: {},
          },
          type: "code_execution",
        },
        {
          key: "google.google_search",
          protocols: ["google_generate_content", "gemini_generate_content"],
          label: "Google Search",
          enabled: true,
          defaultEnabled: true,
          payload: {
            google_search: {},
          },
          provider: "Google",
          type: "google_search",
          description: "Google hosted search grounding tool.",
        },
        {
          key: "google.url_context",
          protocols: ["google_generate_content", "gemini_generate_content"],
          label: "URL Context",
          enabled: true,
          defaultEnabled: true,
          payload: {
            url_context: {},
          },
          type: "url_context",
        },
      ],
    },
  },
  {
    id: "xai_responses",
    protocol: "xai_responses",
    payload: {
      defaultOptions: {
        store: false,
      },
      optionControls: [
        {
          path: "store",
          type: "boolean",
          label: "Store",
        },
      ],
      nativeTools: [
        {
          key: "xai.code_interpreter",
          protocols: ["xai_responses"],
          label: "Code Interpreter",
          enabled: true,
          defaultEnabled: true,
          payload: {
            type: "code_interpreter",
          },
          provider: "xAI",
          type: "code_interpreter",
          description: "xAI hosted code interpreter.",
        },
        {
          key: "xai.web_search",
          protocols: ["xai_responses"],
          label: "Web Search",
          enabled: true,
          defaultEnabled: true,
          payload: {
            type: "web_search",
            enable_image_understanding: true,
          },
          provider: "xAI",
          type: "web_search",
          description: "xAI hosted web search.",
        },
        {
          key: "xai.x_search",
          protocols: ["xai_responses"],
          label: "X Search",
          enabled: true,
          defaultEnabled: true,
          payload: {
            type: "x_search",
            enable_image_understanding: true,
          },
          provider: "xAI",
          type: "x_search",
          description: "xAI hosted X search.",
        },
      ],
    },
  },
];
const MODEL_CAPABILITY_PRESET_ORDER = new Map(MODEL_CAPABILITY_PRESETS.map((preset, index) => [preset.id, index]));

function flattenDefaultOptions(value: unknown, prefix: string[] = []): { path: string }[] {
  if (value && typeof value === "object" && !Array.isArray(value)) {
    return Object.entries(value).flatMap(([key, child]) => flattenDefaultOptions(child, [...prefix, key]));
  }
  return prefix.length > 0 ? [{ path: prefix.join(".") }] : [];
}

function capabilityPresetPayload(preset: CapabilityPreset): Record<string, unknown> {
  return preset.payload;
}

function formatCapabilitiesJSON(value: Record<string, unknown>): string {
  return Object.keys(value).length > 0 ? JSON.stringify(value, null, 2) : "";
}

function capabilityPresetMatched(preset: CapabilityPreset, routeProtocolSet: Set<string>): boolean {
  return routeProtocolSet.size === 0 || routeProtocolSet.has(resolveModelOptionPolicyProtocol(preset.protocol));
}

function hasModelCapabilities(value: string | null | undefined): boolean {
  const normalized = value?.trim() ?? "";
  return Boolean(normalized && normalized !== "{}");
}

function capabilityPresetSummary(preset: CapabilityPreset): { defaults: number; controls: number; tools: number } {
  const defaultOptions = preset.payload.defaultOptions;
  const optionControls = preset.payload.optionControls;
  const nativeTools = preset.payload.nativeTools;
  return {
    defaults: flattenDefaultOptions(defaultOptions).length,
    controls: Array.isArray(optionControls) ? optionControls.length : 0,
    tools: Array.isArray(nativeTools) ? nativeTools.length : 0,
  };
}

function capabilityPresetProtocolLabel(protocol: AdminLLMAdapter): string {
  const resolvedProtocol = resolveModelOptionPolicyProtocol(protocol);
  return MODEL_OPTION_POLICY_PROTOCOL_LABELS[resolvedProtocol] ?? protocol;
}

function capabilityModelProtocolLabel(model: AdminLLMModelDTO): string {
  const protocols = parseProtocolsJSON(model.protocolsJSON);
  return protocols
    .map((protocol) => MODEL_OPTION_POLICY_PROTOCOL_LABELS[protocol as keyof typeof MODEL_OPTION_POLICY_PROTOCOL_LABELS] ?? protocol)
    .join(" / ");
}

export function ModelCapabilitiesPresetDialog({
  open,
  onOpenChange,
  models,
  currentModelID,
  routeProtocols,
  t,
  commonT,
  onApply,
}: {
  open: boolean;
  onOpenChange: (open: boolean) => void;
  models: AdminLLMModelDTO[];
  currentModelID?: number | null;
  routeProtocols: string[];
  t: (key: string, values?: Record<string, string | number>) => string;
  commonT: (key: string) => string;
  onApply: (value: string) => void;
}) {
  const [activeTab, setActiveTab] = useState<"presets" | "models">("presets");
  const routeProtocolSet = new Set(routeProtocols.map((protocol) => resolveModelOptionPolicyProtocol(protocol)).filter(Boolean));
  const sortedPresets = [...MODEL_CAPABILITY_PRESETS].sort((left, right) => {
    const leftMatched = capabilityPresetMatched(left, routeProtocolSet);
    const rightMatched = capabilityPresetMatched(right, routeProtocolSet);
    if (leftMatched !== rightMatched) {
      return leftMatched ? -1 : 1;
    }
    return (MODEL_CAPABILITY_PRESET_ORDER.get(left.id) ?? 0) - (MODEL_CAPABILITY_PRESET_ORDER.get(right.id) ?? 0);
  });
  const reusableModels = models.filter((model) =>
    model.id !== currentModelID
      && hasModelCapabilities(model.capabilitiesJSON),
  );
  const [selectedPresetID, setSelectedPresetID] = useState("");
  const [selectedModelID, setSelectedModelID] = useState("");
  const [modelSearch, setModelSearch] = useState("");
  const selectedPreset = sortedPresets.find((preset) => preset.id === selectedPresetID);
  const selectedModel = reusableModels.find((model) => String(model.id) === selectedModelID);
  const reusableModelItems = reusableModels.map((model) => {
    const protocolLabel = capabilityModelProtocolLabel(model);
    return {
      model,
      protocolLabel,
      searchText: [model.platformModelName, model.vendor, protocolLabel].join(" ").toLowerCase(),
    };
  });
  const normalizedModelSearch = modelSearch.trim().toLowerCase();
  const filteredReusableModelItems = normalizedModelSearch
    ? reusableModelItems.filter((item) => item.searchText.includes(normalizedModelSearch))
    : reusableModelItems;

  function resetSelection() {
    setSelectedPresetID("");
    setSelectedModelID("");
    setModelSearch("");
    setActiveTab("presets");
  }

  function updateModelSearch(value: string) {
    setModelSearch(value);
    const normalized = value.trim().toLowerCase();
    if (!normalized || !selectedModelID) {
      return;
    }
    const selectedItem = reusableModelItems.find((item) => String(item.model.id) === selectedModelID);
    if (selectedItem && !selectedItem.searchText.includes(normalized)) {
      setSelectedModelID("");
    }
  }

  function applyPreset() {
    if (!selectedPreset) {
      return;
    }
    onApply(formatCapabilitiesJSON(capabilityPresetPayload(selectedPreset)));
    toast.success(t("sheet.capabilitiesPreset.applied"));
    onOpenChange(false);
  }

  function applyModel() {
    if (!selectedModel) {
      return;
    }
    onApply(selectedModel.capabilitiesJSON);
    toast.success(t("sheet.capabilitiesPreset.applied"));
    onOpenChange(false);
  }

  return (
    <Dialog
      open={open}
      onOpenChange={(nextOpen) => {
        if (nextOpen) {
          resetSelection();
        }
        onOpenChange(nextOpen);
      }}
    >
      <DialogContent className="flex max-h-[min(82vh,520px)] min-w-0 flex-col gap-0 overflow-hidden p-0 sm:max-w-[520px]">
        <DialogHeader className="shrink-0 px-4 pt-4 pb-2">
          <DialogTitle>{t("sheet.capabilitiesPreset.title")}</DialogTitle>
          <DialogDescription>{t("sheet.capabilitiesPreset.description")}</DialogDescription>
        </DialogHeader>

        <div className="min-h-0 min-w-0 overflow-hidden px-4 py-1">
          <Tabs
            value={activeTab}
            onValueChange={(nextValue) => setActiveTab(nextValue as "presets" | "models")}
            className="min-h-0 min-w-0 overflow-hidden"
          >
            <TabsList className="grid h-8 w-full grid-cols-2">
              <TabsTrigger value="presets" className="min-w-0">
                <span className="min-w-0 truncate">{t("sheet.capabilitiesPreset.presetsTab")}</span>
              </TabsTrigger>
              <TabsTrigger value="models" className="min-w-0">
                <span className="min-w-0 truncate">{t("sheet.capabilitiesPreset.modelsTab")}</span>
              </TabsTrigger>
            </TabsList>

            <TabsContent value="presets" className="mt-2 min-h-0 space-y-2 overflow-hidden">
              <div className="max-h-[min(44vh,260px)] overflow-y-auto rounded-md bg-muted/25 p-1">
                <div className="space-y-0.5">
                  {sortedPresets.map((preset) => {
                    const matched = capabilityPresetMatched(preset, routeProtocolSet);
                    const selected = selectedPreset?.id === preset.id;
                    const summary = capabilityPresetSummary(preset);
                    const protocolLabel = capabilityPresetProtocolLabel(preset.protocol);
                    return (
                      <button
                        key={preset.id}
                        type="button"
                        aria-pressed={selected}
                        onClick={() => setSelectedPresetID(preset.id)}
                        className={cn(
                          "group flex min-h-9 w-full min-w-0 items-center gap-3 rounded-md px-2.5 py-1.5 text-left transition-colors hover:bg-background/60",
                          selected && "bg-background/80",
                        )}
                      >
                        <span className="min-w-0 flex-1 truncate text-xs font-medium leading-5 text-foreground">
                          {protocolLabel}
                        </span>
                        <span className="hidden min-w-0 shrink-0 truncate text-[11px] leading-5 text-muted-foreground sm:block">
                          {t("sheet.capabilitiesPreset.presetSummary", {
                            defaults: summary.defaults,
                            controls: summary.controls,
                            tools: summary.tools,
                          })}
                        </span>
                        <span className="flex h-5 w-[76px] shrink-0 items-center justify-end">
                          {selected ? (
                            <Check className="size-4 text-foreground" strokeWidth={1.8} />
                          ) : matched ? (
                            <span className="sr-only">
                              {protocolLabel}
                            </span>
                          ) : (
                            <Badge variant="secondary" className="h-5 rounded-md px-1.5 text-[10px] font-normal text-muted-foreground shadow-none">
                              {t("sheet.capabilitiesPreset.protocolNotMatched")}
                            </Badge>
                          )}
                        </span>
                        <span className="sr-only">
                          {t("sheet.capabilitiesPreset.presetSummary", {
                            defaults: summary.defaults,
                            controls: summary.controls,
                            tools: summary.tools,
                          })}
                        </span>
                      </button>
                    );
                  })}
                </div>
              </div>
            </TabsContent>

            <TabsContent value="models" className="mt-2 min-h-0 space-y-2 overflow-hidden">
              {reusableModels.length === 0 ? (
                <div className="flex flex-col items-center justify-center rounded-md bg-muted/30 px-3 py-7 text-center">
                  <Copy className="mb-2 size-5 text-muted-foreground" strokeWidth={1.5} />
                  <p className="text-xs text-muted-foreground">{t("sheet.capabilitiesPreset.emptyModels")}</p>
                </div>
              ) : (
                <>
                  <div className="relative">
                    <Search className="pointer-events-none absolute top-1/2 left-2.5 size-3.5 -translate-y-1/2 text-muted-foreground" strokeWidth={1.5} />
                    <Input
                      value={modelSearch}
                      onChange={(event) => updateModelSearch(event.target.value)}
                      placeholder={t("sheet.capabilitiesPreset.modelSearchPlaceholder")}
                      className="h-8 border-transparent bg-muted/25 pr-2 pl-8 text-xs shadow-none focus-visible:border-border/50 focus-visible:ring-0"
                    />
                  </div>
                  <div className="max-h-[min(44vh,260px)] overflow-y-auto rounded-md bg-muted/25 p-1">
                    {filteredReusableModelItems.length === 0 ? (
                      <div className="px-2.5 py-6 text-center text-xs text-muted-foreground">
                        {t("sheet.capabilitiesPreset.emptySearchModels")}
                      </div>
                    ) : (
                      <div className="space-y-0.5">
                        {filteredReusableModelItems.map(({ model, protocolLabel }) => {
                          const selected = selectedModel?.id === model.id;
                          return (
                            <button
                              key={model.id}
                              type="button"
                              aria-pressed={selected}
                              onClick={() => setSelectedModelID(String(model.id))}
                              className={cn(
                                "group flex min-h-9 w-full min-w-0 items-center gap-3 rounded-md px-2.5 py-1.5 text-left transition-colors hover:bg-background/60",
                                selected && "bg-background/80",
                              )}
                            >
                              <span className="min-w-0 flex-1 truncate text-xs font-medium leading-5 text-foreground">
                                {model.platformModelName}
                              </span>
                              <span className="hidden min-w-0 shrink truncate text-[11px] leading-5 text-muted-foreground sm:block">
                                {protocolLabel || t("sheet.capabilitiesPreset.noProtocol")}
                              </span>
                              <Check className={cn("size-4 shrink-0 text-foreground transition-opacity", selected ? "opacity-100" : "opacity-0")} strokeWidth={1.8} />
                              <span className="sr-only">
                                {protocolLabel || t("sheet.capabilitiesPreset.noProtocol")}
                              </span>
                            </button>
                          );
                        })}
                      </div>
                    )}
                  </div>
                </>
              )}
            </TabsContent>
          </Tabs>
        </div>

        <DialogFooter className="shrink-0 px-4 py-3">
          <Button type="button" variant="ghost" onClick={() => onOpenChange(false)}>
            {commonT("actions.cancel")}
          </Button>
          <Button
            type="button"
            onClick={activeTab === "models" ? applyModel : applyPreset}
            disabled={activeTab === "models" ? !selectedModel : !selectedPreset}
          >
            {commonT("actions.confirm")}
          </Button>
        </DialogFooter>
      </DialogContent>
    </Dialog>
  );
}
