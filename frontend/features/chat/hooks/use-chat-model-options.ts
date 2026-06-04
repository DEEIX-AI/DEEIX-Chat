"use client";

import * as React from "react";
import { useTranslations } from "next-intl";

import type {
  ChatModelOption,
  ModelOptionControl,
  ModelOptionControlType,
} from "@/features/chat/types/chat-runtime";
import { resolveAccessToken } from "@/shared/auth/resolve-access-token";
import { parseProtocolsJSON } from "@/features/chat/model/chat-adapter-options";
import { sanitizeConversationOptions } from "@/features/chat/model/conversation-options";
import { listConversationRuns } from "@/shared/api/conversation";
import { listPublicModels } from "@/shared/api/model";
import { getBillingConfig } from "@/shared/api/billing";
import { getMCPPolicy, getModelOptionPolicy } from "@/shared/api/settings";
import { getUserSettings } from "@/shared/api/user-settings";
import type { PublicModelDTO } from "@/shared/api/model.types";
import type { ModelNativeToolConfig, ModelOptionPolicy } from "@/shared/lib/model-option-policy";
import { parseKindsJSON } from "@/shared/model/llm-schema";
import type { ConversationOptions } from "@/shared/api/conversation.types";
import type { SendShortcut } from "@/features/settings/types/settings";
import { parseSendShortcut } from "@/features/settings/utils/chat-settings";

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
  protocol,
  type,
  index,
}: {
  key: string;
  protocol: string;
  type: string;
  index: number;
}): string {
  return [key, protocol, type].map((item) => item.trim()).filter(Boolean).join(":") || `native-tool-${index}`;
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
      if (!key && !type && Object.keys(payload).length === 0) {
        return [];
      }
      return [{
        id: normalizeNativeToolString(source.id) || nativeToolID({ key, protocol, type, index }),
        key,
        protocol,
        protocols: protocols.length > 0 ? protocols : (protocol ? [protocol] : []),
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
    id: nativeToolID({ key, protocol: "", type: "", index }),
    key,
    protocol: "",
    protocols: [],
    type: "",
    label: key,
    enabled: true,
    defaultEnabled: false,
    payload: {},
  }));
}

function mergeDefaultNativeTools(defaultOptions: ConversationOptions, nativeTools: ModelNativeToolConfig[]): ConversationOptions {
  const defaultToolPayloads = nativeTools
    .filter((tool) => tool.enabled && tool.defaultEnabled && Object.keys(tool.payload).length > 0)
    .map((tool) => ({ ...tool.payload }));
  if (defaultToolPayloads.length === 0) {
    return defaultOptions;
  }
  const currentTools = Array.isArray(defaultOptions.tools)
    ? defaultOptions.tools.filter((item) => item !== null && typeof item === "object" && !Array.isArray(item))
    : [];
  return sanitizeConversationOptions({
    ...defaultOptions,
    tools: [...currentTools, ...defaultToolPayloads],
  });
}

function resolveDefaultOptions(raw: string): ConversationOptions {
  const parsed = parseJSONObject(raw);
  if (!parsed) {
    return {};
  }
  const defaults = parsed.defaultOptions;
  const defaultOptions = defaults === null || Array.isArray(defaults) || typeof defaults !== "object"
    ? {}
    : sanitizeConversationOptions(defaults as ConversationOptions);
  return mergeDefaultNativeTools(defaultOptions, resolveNativeTools(raw));
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

function resolveOptionControls(raw: string): ModelOptionControl[] {
  const parsed = parseJSONObject(raw);
  const rawControls = parsed?.optionControls;
  if (!Array.isArray(rawControls)) {
    return [];
  }

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

function toChatModelOption(item: PublicModelDTO): ChatModelOption {
  return {
    platformModelName: item.platformModelName,
    icon: item.icon,
    vendor: item.vendor,
    kinds: parseKindsJSON(item.kindsJSON),
    protocols: parseProtocolsJSON(item.protocolsJSON),
    defaultOptions: resolveDefaultOptions(item.capabilitiesJSON),
    optionControls: resolveOptionControls(item.capabilitiesJSON),
    nativeToolKeys: resolveNativeToolKeys(item.capabilitiesJSON),
    nativeTools: resolveNativeTools(item.capabilitiesJSON),
    pricing: item.pricing,
  };
}

export function useChatModelOptions({
  conversationPublicID,
  conversationModel,
}: {
  conversationPublicID: string | null;
  conversationModel?: string | null;
}) {
  const t = useTranslations("chat.models");
  const [availableModels, setAvailableModels] = React.useState<PublicModelDTO[]>([]);
  const [modelsLoading, setModelsLoading] = React.useState(true);
  const [modelsErrorMsg, setModelsErrorMsg] = React.useState("");
  const [selectedPlatformModelName, setSelectedPlatformModelName] = React.useState("");
  const [userDefaultModel, setUserDefaultModel] = React.useState("");
  const [sendShortcut, setSendShortcut] = React.useState<SendShortcut>("enter");
  const [restoreDraftOnFailure, setRestoreDraftOnFailure] = React.useState(true);
  const [preserveConversationDrafts, setPreserveConversationDrafts] = React.useState(true);
  const [inputHeight, setInputHeight] = React.useState<"compact" | "standard" | "loose">("standard");
  const [markdownRender, setMarkdownRender] = React.useState(true);
  const [showModelInfo, setShowModelInfo] = React.useState(true);
  const [showLatency, setShowLatency] = React.useState(true);
  const [showTokenUsage, setShowTokenUsage] = React.useState(true);
  const [showBillingCost, setShowBillingCost] = React.useState(false);
  const [modelOptionPolicy, setModelOptionPolicy] = React.useState<ModelOptionPolicy | null>(null);
  const [mcpMaxSelectedTools, setMCPMaxSelectedTools] = React.useState(32);
  const activeConversationRef = React.useRef<string | null>(null);
  const userSelectedModelRef = React.useRef(false);
  const runModelRequestRef = React.useRef(0);

  const selectPlatformModelName = React.useCallback((platformModelName: string) => {
    userSelectedModelRef.current = true;
    setSelectedPlatformModelName(platformModelName);
  }, []);

  const refreshModelOption = React.useCallback(async (platformModelName: string): Promise<ChatModelOption | null> => {
    const normalizedName = platformModelName.trim();
    if (!normalizedName) {
      return null;
    }

    const token = await resolveAccessToken();
    if (!token) {
      throw new Error("missing access token");
    }

    const nextModels = await listPublicModels(token);
    setAvailableModels(nextModels);
    const nextModel = nextModels.find((item) => item.platformModelName === normalizedName);
    return nextModel ? toChatModelOption(nextModel) : null;
  }, []);

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
        const [nextModels, settings, billingConfig, nextModelOptionPolicy, nextMCPPolicy] = await Promise.all([
          listPublicModels(token),
          getUserSettings(token).catch(() => ({} as Record<string, string>)),
          getBillingConfig(token).catch(() => null),
          getModelOptionPolicy(token).catch(() => null),
          getMCPPolicy(token).catch(() => null),
        ]);
        if (cancelled) {
          return;
        }
        setAvailableModels(nextModels);
        setModelOptionPolicy(nextModelOptionPolicy);
        setMCPMaxSelectedTools(resolveMCPMaxSelectedTools(nextMCPPolicy?.maxSelectedToolsPerMessage));
        setUserDefaultModel(settings["chat.default_model"]?.trim() ?? "");
        setSendShortcut(parseSendShortcut(settings["chat.send_on_enter"]));
        setRestoreDraftOnFailure(settings["chat.restore_draft_on_failure"] !== "false");
        setPreserveConversationDrafts(settings["chat.preserve_conversation_drafts"] !== "false");
        setMarkdownRender(settings["chat.markdown_render"] !== "false");
        setShowModelInfo(settings["chat.show_model_info"] !== "false");
        setShowLatency(settings["chat.show_latency"] !== "false");
        setShowTokenUsage(settings["chat.show_token_usage"] !== "false");
        setShowBillingCost((billingConfig?.config.mode ?? "self") !== "self" && settings["chat.show_billing_cost"] !== "false");
        setInputHeight(
          settings["chat.input_height"] === "compact" || settings["chat.input_height"] === "loose"
            ? settings["chat.input_height"]
            : "standard",
        );
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
  }, [t]);

  React.useEffect(() => {
    const normalizedConversationID = conversationPublicID?.trim() || null;
    if (!normalizedConversationID) {
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

    async function loadLatestRunModel() {
      const token = await resolveAccessToken();
      if (!token) {
        return;
      }

      const runs = await listConversationRuns(token, normalizedConversationID, { page: 1, pageSize: 1 });
      if (cancelled || requestID !== runModelRequestRef.current || userSelectedModelRef.current) {
        return;
      }

      const latestRunModel = runs.results[0]?.platformModelName?.trim() || "";
      setSelectedPlatformModelName(latestRunModel || fallbackModel);
    }

    void loadLatestRunModel().catch(() => undefined);

    return () => {
      cancelled = true;
    };
  }, [conversationModel, conversationPublicID]);

  React.useEffect(() => {
    if (availableModels.length === 0) {
      return;
    }
    if (conversationPublicID?.trim()) {
      return;
    }

    setSelectedPlatformModelName((current) => {
      const normalizedCurrent = current.trim();
      if (normalizedCurrent && availableModels.some((item) => item.platformModelName === normalizedCurrent)) {
        return normalizedCurrent;
      }

      // User default model for new conversations.
      if (userDefaultModel && availableModels.some((item) => item.platformModelName === userDefaultModel)) {
        return userDefaultModel;
      }

      return availableModels[0].platformModelName;
    });
  }, [availableModels, conversationPublicID, userDefaultModel]);

  const modelOptions = React.useMemo<ChatModelOption[]>(
    () =>
      availableModels.map(toChatModelOption),
    [availableModels],
  );

  return {
    modelOptions,
    refreshModelOption,
    modelsLoading,
    modelsErrorMsg,
    sendShortcut,
    restoreDraftOnFailure,
    preserveConversationDrafts,
    inputHeight,
    markdownRender,
    showModelInfo,
    showLatency,
    showTokenUsage,
    showBillingCost,
    modelOptionPolicy,
    mcpMaxSelectedTools,
    selectedPlatformModelName,
    setSelectedPlatformModelName: selectPlatformModelName,
  };
}
