"use client";

import * as React from "react";
import { useTranslations } from "next-intl";

import type { ChatModelOption } from "@/features/chat/types/chat-runtime";
import { resolveAccessToken } from "@/shared/auth/resolve-access-token";
import { parseProtocolsJSON } from "@/features/chat/model/chat-adapter-options";
import { sanitizeConversationOptions } from "@/features/chat/model/conversation-options";
import { listConversationRuns } from "@/shared/api/conversation";
import { listPublicModels } from "@/shared/api/model";
import { getBillingConfig } from "@/shared/api/billing";
import { getModelOptionPolicy } from "@/shared/api/settings";
import { getUserSettings } from "@/shared/api/user-settings";
import type { PublicModelDTO } from "@/shared/api/model.types";
import type { ModelOptionPolicy } from "@/shared/lib/model-option-policy";
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

function resolveDefaultOptions(raw: string): ConversationOptions {
  const parsed = parseJSONObject(raw);
  if (!parsed) {
    return {};
  }
  const defaults = parsed.defaultOptions;
  if (defaults === null || Array.isArray(defaults) || typeof defaults !== "object") {
    return {};
  }
  return sanitizeConversationOptions(defaults as ConversationOptions);
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
  const activeConversationRef = React.useRef<string | null>(null);
  const userSelectedModelRef = React.useRef(false);
  const runModelRequestRef = React.useRef(0);

  const selectPlatformModelName = React.useCallback((platformModelName: string) => {
    userSelectedModelRef.current = true;
    setSelectedPlatformModelName(platformModelName);
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
        const [nextModels, settings, billingConfig, nextModelOptionPolicy] = await Promise.all([
          listPublicModels(token),
          getUserSettings(token).catch(() => ({} as Record<string, string>)),
          getBillingConfig(token).catch(() => null),
          getModelOptionPolicy(token).catch(() => null),
        ]);
        if (cancelled) {
          return;
        }
        setAvailableModels(nextModels);
        setModelOptionPolicy(nextModelOptionPolicy);
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
      availableModels.map((item) => {
        return {
          platformModelName: item.platformModelName,
          icon: item.icon,
          vendor: item.vendor,
          kinds: parseKindsJSON(item.kindsJSON),
          protocols: parseProtocolsJSON(item.protocolsJSON),
          defaultOptions: resolveDefaultOptions(item.capabilitiesJSON),
          pricing: item.pricing,
        };
      }),
    [availableModels],
  );

  return {
    modelOptions,
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
    selectedPlatformModelName,
    setSelectedPlatformModelName: selectPlatformModelName,
  };
}
