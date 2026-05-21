"use client";

import * as React from "react";

import {
  applyChatFontPreference,
  applyChatFontWeightPreference,
  useChatFontPreference,
  useChatFontWeightPreference,
} from "@/features/settings/utils/chat-font";

export function ChatFontProvider({
  children,
}: {
  children: React.ReactNode;
}) {
  const chatFont = useChatFontPreference();
  const chatFontWeight = useChatFontWeightPreference();

  React.useEffect(() => {
    applyChatFontPreference(chatFont);
  }, [chatFont]);

  React.useEffect(() => {
    applyChatFontWeightPreference(chatFontWeight);
  }, [chatFontWeight]);

  return children;
}
