"use client";

import { useTranslations } from "next-intl";
import * as React from "react";
import { toast } from "sonner";

import type { ChatSettings } from "@/features/settings/types/settings";
import {
  DEFAULT_CHAT_SETTINGS,
  groupModelsForPresentation,
  parseChatSettings,
} from "@/features/settings/utils/chat-settings";
import { useLocalizedErrorMessage } from "@/i18n/use-localized-error";
import { getBillingConfig } from "@/shared/api/billing";
import type { BillingMode } from "@/shared/api/billing.types";
import { listPublicModels } from "@/shared/api/model";
import type { PublicModelDTO } from "@/shared/api/model.types";
import { getChatContextPolicy } from "@/shared/api/settings";
import { useAuthSession } from "@/shared/auth/auth-session-context";
import {
  updateUserSettings,
  useUserSettings,
} from "@/shared/model/user-settings-store";

type UseSettingsChatResult = {
  settings: ChatSettings;
  loading: boolean;
  billingMode: BillingMode;
  contextCompressionEnabled: boolean;
  modelGroups: ReturnType<typeof groupModelsForPresentation>;
  handleBool: (key: string) => (checked: boolean) => void;
  handleEnum: (key: string) => (value: string) => void;
  handleDefaultModel: (value: string) => void;
};

export function useSettingsChat(): UseSettingsChatResult {
  const t = useTranslations("settings.chatPage.toasts");
  const translateError = useLocalizedErrorMessage();
  const { accessToken } = useAuthSession();
  const userSettings = useUserSettings();
  const [models, setModels] = React.useState<PublicModelDTO[]>([]);
  const [metadataLoading, setMetadataLoading] = React.useState(true);
  const [billingMode, setBillingMode] = React.useState<BillingMode>("self");
  const [contextCompressionEnabled, setContextCompressionEnabled] = React.useState(false);

  React.useEffect(() => {
    let cancelled = false;

    void (async () => {
      try {
        const [modelList, billingConfig, contextPolicy] = await Promise.all([
          listPublicModels(accessToken).catch((): PublicModelDTO[] => []),
          getBillingConfig(accessToken).catch((): null => null),
          getChatContextPolicy(accessToken).catch(() => ({ contextCompactEnabled: false })),
        ]);

        if (cancelled) {
          return;
        }

        setModels(modelList);
        setBillingMode(billingConfig?.config.mode ?? "self");
        setContextCompressionEnabled(contextPolicy.contextCompactEnabled);
      } finally {
        if (!cancelled) {
          setMetadataLoading(false);
        }
      }
    })();

    return () => {
      cancelled = true;
    };
  }, [accessToken]);

  const settings = React.useMemo(
    () => userSettings.loaded ? parseChatSettings(userSettings.settings) : DEFAULT_CHAT_SETTINGS,
    [userSettings.loaded, userSettings.settings],
  );
  const modelGroups = React.useMemo(() => groupModelsForPresentation(models), [models]);

  const persistSetting = React.useCallback(
    (key: string, value: string) => {
      void updateUserSettings(accessToken, { [key]: value })
        .catch((error) => {
          toast.error(t("saveFailed"), { description: translateError(error, t("retryLater")) });
        });
    },
    [accessToken, t, translateError],
  );

  const handleBool = React.useCallback(
    (key: string) => (checked: boolean) => {
      persistSetting(key, checked ? "true" : "false");
    },
    [persistSetting],
  );

  const handleEnum = React.useCallback(
    (key: string) => (value: string) => {
      persistSetting(key, value);
    },
    [persistSetting],
  );

  const handleDefaultModel = React.useCallback(
    (value: string) => {
      const code = value === "none" ? "" : value;
      persistSetting("chat.default_model", code);
    },
    [persistSetting],
  );

  return {
    settings,
    loading: metadataLoading || !userSettings.loaded,
    billingMode,
    contextCompressionEnabled,
    modelGroups,
    handleBool,
    handleEnum,
    handleDefaultModel,
  };
}
