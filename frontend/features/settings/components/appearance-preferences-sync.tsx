"use client";

import * as React from "react";

import { resolveAppearancePreferences } from "@/features/settings/utils/appearance-preferences";
import {
  readChatFontPreference,
  readChatFontWeightPreference,
  writeChatFontPreference,
  writeChatFontWeightPreference,
} from "@/features/settings/utils/chat-font";
import {
  readFontSizePreference,
  writeFontSizePreference,
} from "@/features/settings/utils/font-size";
import { useAuthSession } from "@/shared/auth/auth-session-context";
import { useTheme } from "@/shared/components/theme-provider";

export function AppearancePreferencesSync(): null {
  const { user, userStatus } = useAuthSession();
  const { setPreset, setTheme } = useTheme();
  const syncedAppearanceRef = React.useRef<string>("");

  React.useEffect(() => {
    if (userStatus !== "ready" || !user) {
      syncedAppearanceRef.current = "";
      return;
    }

    const raw = user?.appearancePreferences?.trim() ?? "";
    const syncKey = `${user.publicID}:${raw}`;
    if (syncKey === syncedAppearanceRef.current) {
      return;
    }
    syncedAppearanceRef.current = syncKey;

    const next = resolveAppearancePreferences(raw);
    setTheme(next.theme);
    setPreset(next.preset);
    if (next.chatFont !== readChatFontPreference()) {
      writeChatFontPreference(next.chatFont);
    }
    if (next.chatFontWeight !== readChatFontWeightPreference()) {
      writeChatFontWeightPreference(next.chatFontWeight);
    }
    if (next.fontSize !== readFontSizePreference()) {
      writeFontSizePreference(next.fontSize);
    }
  }, [setPreset, setTheme, user, userStatus]);

  return null;
}
