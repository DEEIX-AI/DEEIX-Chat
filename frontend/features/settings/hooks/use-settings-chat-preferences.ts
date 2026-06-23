"use client";

import * as React from "react";

import { USER_SETTINGS_UPDATED_EVENT } from "@/features/settings/events/user-settings-events";
import { getUserSettings } from "@/shared/api/user-settings";
import { useAuthSession } from "@/shared/auth/auth-session-context";

type ChatPreferences = {
  autoGenerateTitle: boolean;
  deleteFilesByDefault: boolean;
  reuseModelOptions: boolean;
};

type ChatPreferencesState = ChatPreferences & {
  loaded: boolean;
};

const DEFAULT_CHAT_PREFERENCES: ChatPreferences = {
  autoGenerateTitle: true,
  deleteFilesByDefault: false,
  reuseModelOptions: true,
};

let cachedAccessToken: string | null = null;
let cachedPreferences = DEFAULT_CHAT_PREFERENCES;
let pendingAccessToken: string | null = null;
let pendingPreferences: Promise<ChatPreferences> | null = null;

function resolveChatPreferences(settings: Record<string, string>): ChatPreferences {
  return {
    autoGenerateTitle: settings["chat.auto_generate_title"] !== "false",
    deleteFilesByDefault: settings["chat.delete_conversation_files_by_default"] === "true",
    reuseModelOptions: settings["chat.reuse_model_options"] !== "false",
  };
}

function resetChatPreferencesCache() {
  cachedAccessToken = null;
  cachedPreferences = DEFAULT_CHAT_PREFERENCES;
  pendingAccessToken = null;
  pendingPreferences = null;
}

function loadChatPreferences(accessToken: string): Promise<ChatPreferences> {
  if (cachedAccessToken === accessToken) {
    return Promise.resolve(cachedPreferences);
  }
  if (pendingPreferences && pendingAccessToken === accessToken) {
    return pendingPreferences;
  }

  pendingAccessToken = accessToken;
  pendingPreferences = getUserSettings(accessToken)
    .then(resolveChatPreferences)
    .then((preferences) => {
      if (pendingAccessToken === accessToken) {
        cachedAccessToken = accessToken;
        cachedPreferences = preferences;
      }
      return preferences;
    })
    .catch(() => DEFAULT_CHAT_PREFERENCES)
    .finally(() => {
      if (pendingAccessToken === accessToken) {
        pendingAccessToken = null;
        pendingPreferences = null;
      }
    });

  return pendingPreferences;
}

export function useSettingsChatPreferences(): ChatPreferencesState {
  const { accessToken } = useAuthSession();
  const preferencesVersionRef = React.useRef(0);
  const [preferences, setPreferences] = React.useState<ChatPreferences>(() =>
    accessToken && cachedAccessToken === accessToken ? cachedPreferences : DEFAULT_CHAT_PREFERENCES,
  );
  const [loaded, setLoaded] = React.useState(() => !accessToken || cachedAccessToken === accessToken);

  React.useEffect(() => {
    let cancelled = false;
    const requestVersion = preferencesVersionRef.current;

    void (async () => {
      if (!accessToken) {
        preferencesVersionRef.current += 1;
        resetChatPreferencesCache();
        setPreferences(DEFAULT_CHAT_PREFERENCES);
        setLoaded(true);
        return;
      }
      setLoaded(cachedAccessToken === accessToken);
      const nextPreferences = await loadChatPreferences(accessToken);
      if (!cancelled && requestVersion === preferencesVersionRef.current) {
        setPreferences(nextPreferences);
        setLoaded(true);
      }
    })();

    return () => {
      cancelled = true;
    };
  }, [accessToken]);

  React.useEffect(() => {
    const handleUserSettingsUpdated = (event: Event) => {
      const settings = (event as CustomEvent<Record<string, string>>).detail;
      if (settings && typeof settings === "object") {
        preferencesVersionRef.current += 1;
        const nextPreferences = resolveChatPreferences(settings);
        if (accessToken) {
          if (pendingAccessToken === accessToken) {
            pendingAccessToken = null;
            pendingPreferences = null;
          }
          cachedAccessToken = accessToken;
        }
        cachedPreferences = nextPreferences;
        setPreferences(nextPreferences);
        setLoaded(true);
      }
    };

    window.addEventListener(USER_SETTINGS_UPDATED_EVENT, handleUserSettingsUpdated);
    return () => {
      window.removeEventListener(USER_SETTINGS_UPDATED_EVENT, handleUserSettingsUpdated);
    };
  }, [accessToken]);

  return { ...preferences, loaded };
}
