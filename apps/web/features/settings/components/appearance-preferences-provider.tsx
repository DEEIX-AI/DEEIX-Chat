"use client";

import * as React from "react";

import {
  applyChatFontPreference,
  applyChatFontWeightPreference,
  useChatFontPreference,
  useChatFontWeightPreference,
} from "@/features/settings/utils/chat-font";
import {
  applyFontSizePreference,
  useFontSizePreference,
} from "@/features/settings/utils/font-size";

export function AppearancePreferencesProvider({ children }: { children: React.ReactNode }) {
  const chatFont = useChatFontPreference();
  const chatFontWeight = useChatFontWeightPreference();
  const fontSize = useFontSizePreference();

  React.useEffect(() => {
    applyChatFontPreference(chatFont);
    applyChatFontWeightPreference(chatFontWeight);
  }, [chatFont, chatFontWeight]);

  React.useEffect(() => {
    applyFontSizePreference(fontSize);
  }, [fontSize]);

  return children;
}
