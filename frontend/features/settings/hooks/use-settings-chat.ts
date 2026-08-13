"use client";

import * as React from "react";
import { useTranslations } from "next-intl";
import { toast } from "sonner";

import type { ChatSettings } from "@/features/settings/types/settings";
import {
  DEFAULT_CHAT_SETTINGS,
  groupModelsForPresentation,
  parseChatSettings,
} from "@/features/settings/utils/chat-settings";
import { dispatchUserSettingsUpdated } from "@/features/settings/events/user-settings-events";
import { useAuthSession } from "@/shared/auth/auth-session-context";
import { listPublicModels } from "@/shared/api/model";
import { getBillingConfig } from "@/shared/api/billing";
import { getChatContextPolicy } from "@/shared/api/settings";
import { getUserSettings, patchUserSettings } from "@/shared/api/user-settings";
import type { PublicModelDTO } from "@/shared/api/model.types";
import type { BillingMode } from "@/shared/api/billing.types";
import { useLocalizedErrorMessage } from "@/i18n/use-localized-error";

type UseSettingsChatResult = {
  settings: ChatSettings;
  loading: boolean;
  billingMode: BillingMode;
  contextCompressionEnabled: boolean;
  modelGroups: ReturnType<typeof groupModelsForPresentation>;
  handleBool: (key: string, field: keyof ChatSettings) => (checked: boolean) => void;
  handleEnum: (key: string, field: keyof ChatSettings) => (value: string) => void;
  handleDefaultModel: (value: string) => void;
};

export function useSettingsChat(): UseSettingsChatResult {
  const t = useTranslations("settings.chatPage.toasts");
  const translateError = useLocalizedErrorMessage();
  const { accessToken } = useAuthSession();
  const [settings, setSettings] = React.useState<ChatSettings>(DEFAULT_CHAT_SETTINGS);
  const [models, setModels] = React.useState<PublicModelDTO[]>([]);
  const [loading, setLoading] = React.useState(true);
  const [billingMode, setBillingMode] = React.useState<BillingMode>("self");
  const [contextCompressionEnabled, setContextCompressionEnabled] = React.useState(false);
  const settingRequestSeqRef = React.useRef<Record<string, number>>({});

  React.useEffect(() => {
    let cancelled = false;

    void (async () => {
      try {
        const [map, modelList, billingConfig, contextPolicy] = await Promise.all([
          getUserSettings(accessToken),
          listPublicModels(accessToken).catch((): PublicModelDTO[] => []),
          getBillingConfig(accessToken).catch(() => null),
          getChatContextPolicy(accessToken).catch(() => ({ contextCompactEnabled: false })),
        ]);

        if (cancelled) {
          return;
        }

        setSettings(parseChatSettings(map));
        setModels(modelList);
        setBillingMode(billingConfig?.config.mode ?? "self");
        setContextCompressionEnabled(contextPolicy.contextCompactEnabled);
      } finally {
        if (!cancelled) {
          setLoading(false);
        }
      }
    })();

    return () => {
      cancelled = true;
    };
  }, [accessToken]);

  const modelGroups = React.useMemo(() => groupModelsForPresentation(models), [models]);

  const persistSetting = React.useCallback(
    <K extends keyof ChatSettings>(key: string, field: K, value: string, previousValue: ChatSettings[K]) => {
      const requestSeq = (settingRequestSeqRef.current[key] ?? 0) + 1;
      settingRequestSeqRef.current[key] = requestSeq;
      void patchUserSettings(accessToken, { [key]: value })
        .then((map) => {
          if (settingRequestSeqRef.current[key] !== requestSeq) {
            return;
          }
          const saved = parseChatSettings(map);
          setSettings((current) => ({ ...current, [field]: saved[field] }));
          dispatchUserSettingsUpdated(map);
        })
        .catch((error) => {
          if (settingRequestSeqRef.current[key] !== requestSeq) {
            return;
          }
          setSettings((current) => ({ ...current, [field]: previousValue }));
          toast.error(t("saveFailed"), { description: translateError(error, t("retryLater")) });
        });
    },
    [accessToken, t, translateError],
  );

  const handleBool = React.useCallback(
    (key: string, field: keyof ChatSettings) => (checked: boolean) => {
      setSettings((previous) => {
        persistSetting(key, field, checked ? "true" : "false", previous[field]);
        return { ...previous, [field]: checked };
      });
    },
    [persistSetting],
  );

  const handleEnum = React.useCallback(
    (key: string, field: keyof ChatSettings) => (value: string) => {
      setSettings((previous) => {
        persistSetting(key, field, value, previous[field]);
        return { ...previous, [field]: value };
      });
    },
    [persistSetting],
  );

  const handleDefaultModel = React.useCallback(
    (value: string) => {
      const code = value === "none" ? "" : value;
      setSettings((previous) => {
        persistSetting("chat.default_model", "defaultModel", code, previous.defaultModel);
        return { ...previous, defaultModel: code };
      });
    },
    [persistSetting],
  );

  return {
    settings,
    loading,
    billingMode,
    contextCompressionEnabled,
    modelGroups,
    handleBool,
    handleEnum,
    handleDefaultModel,
  };
}
