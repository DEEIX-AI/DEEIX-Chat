"use client";

import { useTranslations } from "next-intl";
import * as React from "react";
import { sanitizeConversationOptions } from "@/features/chat/model/conversation-options";
import type {
  ChatModelOption,
  ModelOptionControl,
  ModelOptionControlType,
} from "@/features/chat/types/chat-runtime";
import { parseSendShortcut, type SendShortcut } from "@/features/settings";
import { getBillingConfig } from "@/shared/api/billing";
import { listConversationRuns } from "@/shared/api/conversation";
import type { ConversationOptions } from "@/shared/api/conversation.types";
import { listPublicModels } from "@/shared/api/model";
import type { PublicModelDTO } from "@/shared/api/model.types";
import { getMCPPolicy, getModelOptionPolicy } from "@/shared/api/settings";
import { resolveAccessToken } from "@/shared/auth/resolve-access-token";
import {
  type BillingDisplayCurrency,
  normalizeBillingDisplayCurrency,
} from "@/shared/lib/billing-display";
import type { ModelNativeToolConfig, ModelOptionPolicy } from "@/shared/lib/model-option-policy";
import { parseProtocolsJSON } from "@/shared/lib/model-protocols";
import { nativeToolDefinitionVariantsFromConfig, nativeToolPayloadSignature } from "@/shared/lib/native-tool-payload";
import {
  type ChatContentWidth,
  parseChatContentWidth,
} from "@/shared/model/chat-content-width";
import { resolveConversationDefaultModel } from "@/shared/model/conversation-default-model";
import { parseKindsJSON } from "@/shared/model/llm-schema";
import { useUserSettings } from "@/shared/model/user-settings-store";

type ModelCatalogRefreshResult = {
  models: PublicModelDTO[];
  modelOptionPolicy: ModelOptionPolicy | null;
};

function parseJSONObject(raw: string): Record<string, unknown> | null {
  const normalized = raw.trim();
  if (!normalized) {
    return null;
  }
  try {
    const parsed = JSON.parse(normalized) as unknown;
    if (parsed === null || Array.isArray(parsed) || typeof parsed !== "object") {
      return null;
    }
    return parsed as Record<string, unknown>;
  } catch {
    return null;
  }
}

function resolveChatContentWidth(settings: Record<string, string>): ChatContentWidth {
  return parseChatContentWidth(settings["chat.content_width"]);
}

function normalizeNativeToolPayload(value: unknown): Record<string, unknown> {
  if (value === null || Array.isArray(value) || typeof value !== "object") {
    return {};
  }
  return value as Record<string, unknown>;
}

function normalizeNativeToolString(value: unknown): string {
  return typeof value === "string" ? value.trim() : "";
}

function normalizeNativeToolStrings(value: unknown): string[] {
  if (!Array.isArray(value)) {
    return [];
  }
  return Array.from(
    new Set(
      value
        .map((item) => normalizeNativeToolString(item))
        .filter(Boolean),
    ),
  );
}

function nativeToolID({
  key,
  protocols,
  type,
  index,
}: {
  key: string;
  protocols: string[];
  type: string;
  index: number;
}): string {
  return [key, ...protocols, type].map((item) => item.trim()).filter(Boolean).join(":") || `native-tool-${index}`;
}

function resolveNativeTools(raw: string): ModelNativeToolConfig[] {
  const parsed = parseJSONObject(raw);
  if (!parsed) {
    return [];
  }
  const rawTools = parsed.nativeTools;
  if (Array.isArray(rawTools)) {
    return rawTools.flatMap((item, index): ModelNativeToolConfig[] => {
      if (item === null || Array.isArray(item) || typeof item !== "object") {
        return [];
      }
      const source = item as Record<string, unknown>;
      const key = normalizeNativeToolString(source.key ?? source.toolKey);
      const payload = normalizeNativeToolPayload(source.payload);
      const type = normalizeNativeToolString(source.type) || normalizeNativeToolString(payload.type);
      const protocol = normalizeNativeToolString(source.protocol);
      const protocols = normalizeNativeToolStrings(source.protocols);
      const effectiveProtocols = protocols.length > 0 ? protocols : (protocol ? [protocol] : []);
      if (!key && !type && Object.keys(payload).length === 0) {
        return [];
      }
      return [{
        id: normalizeNativeToolString(source.id) || nativeToolID({ key, protocols: effectiveProtocols, type, index }),
        key,
        protocol,
        protocols: effectiveProtocols,
        provider: normalizeNativeToolString(source.provider) || undefined,
        type,
        label: normalizeNativeToolString(source.label) || type || key,
        description: normalizeNativeToolString(source.description) || undefined,
        enabled: source.enabled !== false,
        defaultEnabled: source.defaultEnabled === true,
        payload,
      }];
    }).filter((item) => item.enabled);
  }
  return resolveNativeToolKeys(raw).map((key, index) => ({
    id: nativeToolID({ key, protocols: [], type: "", index }),
    key,
    protocol: "",
    protocols: [] as string[],
    type: "",
    label: key,
    enabled: true,
    defaultEnabled: false,
    payload: {},
  }));
}

function mergeDefaultNativeTools(
  defaultOptions: ConversationOptions,
  nativeTools: ModelNativeToolConfig[],
  catalog: ModelOptionPolicy["nativeTools"],
  modelProtocols: string[],
): ConversationOptions {
  const currentTools = Array.isArray(defaultOptions.tools)
    ? defaultOptions.tools.filter((item) => item !== null && typeof item === "object" && !Array.isArray(item))
    : [];
  const seenPayloads = new Set(currentTools.map(nativeToolPayloadSignature));
  const defaultToolPayloads = nativeTools
    .filter((tool) =>
      tool.enabled
      && tool.defaultEnabled
    )
    .flatMap((tool) => nativeToolDefinitionVariantsFromConfig(tool, catalog, modelProtocols))
    .flatMap((tool) => {
      if (Object.keys(tool.payload).length === 0) {
        return [];
      }
      const signature = nativeToolPayloadSignature(tool.payload);
      if (seenPayloads.has(signature)) {
        return [];
      }
      seenPayloads.add(signature);
      return [{ ...tool.payload }];
    });
  if (defaultToolPayloads.length === 0) {
    return defaultOptions;
  }
  return sanitizeConversationOptions({
    ...defaultOptions,
    tools: [...currentTools, ...defaultToolPayloads],
  });
}

function resolveDefaultOptions(
  raw: string,
  nativeTools: ModelNativeToolConfig[],
  catalog: ModelOptionPolicy["nativeTools"],
  modelProtocols: string[],
): ConversationOptions {
  const parsed = parseJSONObject(raw);
  if (!parsed) {
    return {};
  }
  const defaults = parsed.defaultOptions;
  const defaultOptions = defaults === null || Array.isArray(defaults) || typeof defaults !== "object"
    ? {}
    : sanitizeConversationOptions(defaults as ConversationOptions);
  return mergeDefaultNativeTools(defaultOptions, nativeTools, catalog, modelProtocols);
}

const MODEL_OPTION_CONTROL_TYPES = new Set<ModelOptionControlType>(["boolean", "number", "select", "text"]);

function normalizeOptionControlPath(value: unknown): string {
  if (typeof value !== "string") {
    return "";
  }
  return value
    .split(".")
    .map((segment) => segment.trim())
    .filter(Boolean)
    .join(".");
}

function normalizeOptionControlType(value: unknown): ModelOptionControlType | undefined {
  if (typeof value !== "string") {
    return undefined;
  }
  const normalized = value.trim();
  if (!MODEL_OPTION_CONTROL_TYPES.has(normalized as ModelOptionControlType)) {
    return undefined;
  }
  return normalized as ModelOptionControlType;
}

function normalizeOptionControlString(value: unknown): string | undefined {
  if (typeof value !== "string") {
    return undefined;
  }
  const normalized = value.trim();
  return normalized || undefined;
}

function normalizeOptionControlOptions(value: unknown): string[] | undefined {
  if (!Array.isArray(value)) {
    return undefined;
  }
  const options = Array.from(
    new Set(
      value
        .map((item) => (typeof item === "string" ? item.trim() : ""))
        .filter(Boolean),
    ),
  );
  return options.length > 0 ? options : undefined;
}

function resolveLockedOptionPaths(raw: string): string[] {
  const parsed = parseJSONObject(raw);
  const rawPaths = parsed?.lockedOptionPaths;
  if (!Array.isArray(rawPaths)) {
    return [];
  }
  return Array.from(
    new Set(
      rawPaths
        .map((item) => normalizeOptionControlPath(item))
        .filter(Boolean),
    ),
  );
}

function resolveOptionControls(raw: string): ModelOptionControl[] {
  const parsed = parseJSONObject(raw);
  const rawControls = parsed?.optionControls;
  if (!Array.isArray(rawControls)) {
    return [];
  }
  const lockedPaths = new Set(resolveLockedOptionPaths(raw));

  const controls = rawControls.flatMap((item): ModelOptionControl[] => {
    if (item === null || Array.isArray(item) || typeof item !== "object") {
      return [];
    }
    const source = item as Record<string, unknown>;
    const path = normalizeOptionControlPath(source.path);
    if (!path) {
      return [];
    }
    const control: ModelOptionControl = { path };
    if (lockedPaths.has(path)) {
      control.locked = true;
    }
    const type = normalizeOptionControlType(source.type);
    const label = normalizeOptionControlString(source.label);
    const description = normalizeOptionControlString(source.description);
    const placeholder = normalizeOptionControlString(source.placeholder);
    const options = normalizeOptionControlOptions(source.options);
    if (type) {
      control.type = type;
    }
    if (label) {
      control.label = label;
    }
    if (description) {
      control.description = description;
    }
    if (placeholder) {
      control.placeholder = placeholder;
    }
    if (options) {
      control.options = options;
    }
    return [control];
  });

  return controls.filter((item, index) => controls.findIndex((candidate) => candidate.path === item.path) === index);
}

function resolveVideoExtensionConfig(raw: string, protocols: string[]): ChatModelOption["videoExtension"] {
  const parsed = parseJSONObject(raw);
  const mediaTasks = parsed?.mediaTasks;
  const taskSource = mediaTasks && typeof mediaTasks === "object" && !Array.isArray(mediaTasks)
    ? (mediaTasks as Record<string, unknown>).video_extension
    : undefined;
  const task = taskSource && typeof taskSource === "object" && !Array.isArray(taskSource)
    ? (taskSource as Record<string, unknown>)
    : null;
  const protocolSupported = protocols.includes("xai_video_extensions");
  if (!protocolSupported || task?.enabled === false) {
    return null;
  }
  const defaultOptions = task?.defaultOptions && typeof task.defaultOptions === "object" && !Array.isArray(task.defaultOptions)
    ? sanitizeConversationOptions(task.defaultOptions as ConversationOptions)
    : { duration: 6 };
  const rawControls = Array.isArray(task?.optionControls) ? task.optionControls : [{ path: "duration", type: "select", label: "Duration", description: "2–10 seconds", options: ["2", "3", "4", "5", "6", "7", "8", "9", "10"] }];
  const controls = rawControls.flatMap((item): ModelOptionControl[] => {
    if (!item || typeof item !== "object" || Array.isArray(item)) return [];
    const source = item as Record<string, unknown>;
    const path = normalizeOptionControlPath(source.path);
    if (path !== "duration") return [];
    return [{
      path,
      type: normalizeOptionControlType(source.type) ?? "select",
      label: normalizeOptionControlString(source.label),
      description: normalizeOptionControlString(source.description),
      options: normalizeOptionControlOptions(source.options) ?? ["2", "3", "4", "5", "6", "7", "8", "9", "10"],
    }];
  });
  return { enabled: true, defaultOptions, optionControls: controls };
}

function resolveNativeToolKeys(raw: string): string[] {
  const parsed = parseJSONObject(raw);
  const rawKeys = parsed?.nativeToolKeys;
  if (!Array.isArray(rawKeys)) {
    return [];
  }
  return Array.from(
    new Set(
      rawKeys
        .map((item) => (typeof item === "string" ? item.trim() : ""))
        .filter(Boolean),
    ),
  );
}

function resolveMCPMaxSelectedTools(value: unknown): number {
  const numeric = typeof value === "number" ? value : Number(value);
  if (!Number.isFinite(numeric) || numeric <= 0) {
    return 32;
  }
  return Math.min(Math.floor(numeric), 128);
}

function toChatModelOption(
  item: PublicModelDTO,
  nativeToolCatalog: ModelOptionPolicy["nativeTools"] = [],
): ChatModelOption {
  const protocols = parseProtocolsJSON(item.protocolsJSON);
  const nativeTools = resolveNativeTools(item.capabilitiesJSON);
  return {
    platformModelName: item.platformModelName,
    icon: item.icon,
    vendor: item.vendor,
    vendorName: item.vendorName,
    vendorIcon: item.vendorIcon,
    displayGroupID: item.displayGroupID,
    displayGroupName: item.displayGroupName,
    displayGroupIcon: item.displayGroupIcon,
    kinds: parseKindsJSON(item.kindsJSON),
    protocols,
    defaultOptions: resolveDefaultOptions(item.capabilitiesJSON, nativeTools, nativeToolCatalog, protocols),
    optionControls: resolveOptionControls(item.capabilitiesJSON),
    lockedOptionPaths: resolveLockedOptionPaths(item.capabilitiesJSON),
    nativeToolKeys: resolveNativeToolKeys(item.capabilitiesJSON),
    nativeTools,
    pricing: item.pricing,
    videoExtension: resolveVideoExtensionConfig(item.capabilitiesJSON, protocols),
  };
}

export function useChatModelOptions({
  conversationPublicID,
  conversationModel,
  resetToken,
}: {
  conversationPublicID: string | null;
  conversationModel?: string | null;
  resetToken?: number;
}) {
  const t = useTranslations("chat.models");
  const { settings: userSettings } = useUserSettings();
  const [availableModels, setAvailableModels] = React.useState<PublicModelDTO[]>([]);
  const [modelsLoading, setModelsLoading] = React.useState(true);
  const [modelsErrorMsg, setModelsErrorMsg] = React.useState("");
  const [selectedPlatformModelName, setSelectedPlatformModelName] = React.useState("");
  const [billingCostAvailable, setBillingCostAvailable] = React.useState(false);
  const [billingDisplayCurrency, setBillingDisplayCurrency] = React.useState<BillingDisplayCurrency>("USD");
  const [billingDisplayUsdToCnyRate, setBillingDisplayUsdToCnyRate] = React.useState<number | null>(null);
  const [modelOptionPolicy, setModelOptionPolicy] = React.useState<ModelOptionPolicy | null>(null);
  const [mcpMaxSelectedTools, setMCPMaxSelectedTools] = React.useState(32);
  const activeConversationRef = React.useRef<string | null>(null);
  const userSelectedModelRef = React.useRef(false);
  const runModelRequestRef = React.useRef(0);
  const modelCatalogRequestRef = React.useRef<Promise<ModelCatalogRefreshResult> | null>(null);
  const userDefaultModel = userSettings["chat.default_model"]?.trim() ?? "";
  const sendShortcut: SendShortcut = parseSendShortcut(userSettings["chat.send_on_enter"]);
  const restoreDraftOnFailure = userSettings["chat.restore_draft_on_failure"] !== "false";
  const preserveConversationDrafts = userSettings["chat.preserve_conversation_drafts"] !== "false";
  const inputHeight: "compact" | "standard" | "loose" =
    userSettings["chat.input_height"] === "compact" || userSettings["chat.input_height"] === "loose"
      ? userSettings["chat.input_height"]
      : "standard";
  const contentWidth: ChatContentWidth = resolveChatContentWidth(userSettings);
  const markdownRender = userSettings["chat.markdown_render"] !== "false";
  const showModelInfo = userSettings["chat.show_model_info"] !== "false";
  const showLatency = userSettings["chat.show_latency"] !== "false";
  const showTokenUsage = userSettings["chat.show_token_usage"] !== "false";
  const showBillingCost = billingCostAvailable && userSettings["chat.show_billing_cost"] !== "false";

  const selectPlatformModelName = React.useCallback((platformModelName: string) => {
    userSelectedModelRef.current = true;
    setSelectedPlatformModelName(platformModelName);
  }, []);

  const loadModelCatalog = React.useCallback((accessToken?: string): Promise<ModelCatalogRefreshResult> => {
    if (modelCatalogRequestRef.current) {
      return modelCatalogRequestRef.current;
    }

    let request: Promise<ModelCatalogRefreshResult>;
    request = (async () => {
      const token = accessToken?.trim() || await resolveAccessToken();
      if (!token) {
        throw new Error("missing access token");
      }

      const [models, modelOptionPolicy] = await Promise.all([
        listPublicModels(token),
        getModelOptionPolicy(token).catch((): null => null),
      ]);
      return { models, modelOptionPolicy };
    })().finally(() => {
      if (modelCatalogRequestRef.current === request) {
        modelCatalogRequestRef.current = null;
      }
    });

    modelCatalogRequestRef.current = request;
    return request;
  }, []);

  const applyModelCatalog = React.useCallback((catalog: ModelCatalogRefreshResult) => {
    setAvailableModels(catalog.models);
    setModelOptionPolicy(catalog.modelOptionPolicy);
  }, []);

  const refreshModelCatalog = React.useCallback(async (): Promise<ModelCatalogRefreshResult> => {
    const catalog = await loadModelCatalog();
    applyModelCatalog(catalog);
    setModelsErrorMsg("");
    return catalog;
  }, [applyModelCatalog, loadModelCatalog]);

  const refreshModelOption = React.useCallback(async (platformModelName: string): Promise<ChatModelOption | null> => {
    const normalizedName = platformModelName.trim();
    if (!normalizedName) {
      return null;
    }

    const catalog = await refreshModelCatalog();
    const nextModel = catalog.models.find((item) => item.platformModelName === normalizedName);
    return nextModel ? toChatModelOption(nextModel, catalog.modelOptionPolicy?.nativeTools ?? []) : null;
  }, [refreshModelCatalog]);

  React.useEffect(() => {
    let cancelled = false;

    async function loadModels() {
      setModelsLoading(true);
      setModelsErrorMsg("");
      try {
        const token = await resolveAccessToken();
        if (!token) {
          setModelsErrorMsg(t("signInRequired"));
          return;
        }
        const [catalog, billingConfig, nextMCPPolicy] = await Promise.all([
          loadModelCatalog(token),
          getBillingConfig(token).catch((): null => null),
          getMCPPolicy(token).catch((): null => null),
        ]);
        if (cancelled) {
          return;
        }
        applyModelCatalog(catalog);
        setMCPMaxSelectedTools(resolveMCPMaxSelectedTools(nextMCPPolicy?.maxSelectedToolsPerMessage));
        setBillingCostAvailable((billingConfig?.config.mode ?? "self") !== "self");
        setBillingDisplayCurrency(normalizeBillingDisplayCurrency(billingConfig?.config.displayCurrency));
        setBillingDisplayUsdToCnyRate(billingConfig?.config.usdToCNYRate ?? null);
      } catch {
        if (!cancelled) {
          setModelsErrorMsg(t("loadFailed"));
        }
      } finally {
        if (!cancelled) {
          setModelsLoading(false);
        }
      }
    }

    void loadModels();
    return () => {
      cancelled = true;
    };
  }, [applyModelCatalog, loadModelCatalog, t]);

  React.useEffect(() => {
    const normalizedConversationID = conversationPublicID?.trim() || null;
    if (!normalizedConversationID) {
      // 无会话状态也可能来自当前页点击“新对话”，要保留用户刚在选择器里切换的模型。
      activeConversationRef.current = null;
      return;
    }

    const conversationChanged = activeConversationRef.current !== normalizedConversationID;
    if (conversationChanged) {
      activeConversationRef.current = normalizedConversationID;
      userSelectedModelRef.current = false;
    }

    const fallbackModel = conversationModel?.trim() || "";
    if (!userSelectedModelRef.current) {
      setSelectedPlatformModelName(fallbackModel);
    }

    let cancelled = false;
    const requestID = runModelRequestRef.current + 1;
    runModelRequestRef.current = requestID;

    // 本次请求绑定的会话 ID（非空）。
    const activeConversationID = normalizedConversationID;

    async function loadLatestRunModel() {
      const token = await resolveAccessToken();
      if (!token) {
        return;
      }

      const runs = await listConversationRuns(token, activeConversationID, { page: 1, pageSize: 1 });
      if (cancelled || requestID !== runModelRequestRef.current || userSelectedModelRef.current) {
        return;
      }

      const latestRunModel = runs.results[0]?.platformModelName?.trim() || "";
      setSelectedPlatformModelName(latestRunModel || fallbackModel);
    }

    void loadLatestRunModel().catch((): undefined => undefined);

    return () => {
      cancelled = true;
    };
  }, [conversationModel, conversationPublicID, resetToken]);

  React.useEffect(() => {
    if (availableModels.length === 0) {
      return;
    }
    if (conversationPublicID?.trim()) {
      return;
    }

    let cancelled = false;
    async function applyDefaultModel() {
      const token = await resolveAccessToken();
      if (!token || cancelled || userSelectedModelRef.current) {
        return;
      }
      const result = await resolveConversationDefaultModel({
        accessToken: token,
        availableModels,
        userDefaultModel,
      });
      if (!cancelled && !userSelectedModelRef.current) {
        setSelectedPlatformModelName(result.platformModelName);
      }
    }

    void applyDefaultModel().catch(() => {
      if (!cancelled && !userSelectedModelRef.current) {
        setSelectedPlatformModelName(availableModels[0]?.platformModelName ?? "");
      }
    });
    return () => {
      cancelled = true;
    };
  }, [availableModels, conversationPublicID, resetToken, userDefaultModel]);

  const modelOptions = React.useMemo<ChatModelOption[]>(
    () =>
      availableModels.map((model) => toChatModelOption(model, modelOptionPolicy?.nativeTools ?? [])),
    [availableModels, modelOptionPolicy?.nativeTools],
  );

  return {
    modelOptions,
    refreshModelCatalog,
    refreshModelOption,
    modelsLoading,
    modelsErrorMsg,
    sendShortcut,
    restoreDraftOnFailure,
    preserveConversationDrafts,
    inputHeight,
    contentWidth,
    markdownRender,
    showModelInfo,
    showLatency,
    showTokenUsage,
    showBillingCost,
    billingDisplayCurrency,
    billingDisplayUsdToCnyRate,
    modelOptionPolicy,
    mcpMaxSelectedTools,
    selectedPlatformModelName,
    setSelectedPlatformModelName: selectPlatformModelName,
  };
}
