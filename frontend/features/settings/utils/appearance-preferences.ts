"use client";

import {
  CHAT_FONT_STORAGE_KEY,
  CHAT_FONT_WEIGHT_STORAGE_KEY,
  isChatFontOption,
  isChatFontWeightOption,
  type ChatFontOption,
  type ChatFontWeightOption,
} from "@/features/settings/utils/chat-font";
import {
  FONT_SIZE_STORAGE_KEY,
  isFontSizeOption,
  type FontSizeOption,
} from "@/features/settings/utils/font-size";
import {
  THEME_PRESET_STORAGE_KEY,
  THEME_PRESETS,
  type ThemePreset,
} from "@/shared/model/theme";

export type AppearancePreferences = {
  preset: ThemePreset;
  chatFont: ChatFontOption;
  chatFontWeight: ChatFontWeightOption;
  fontSize: FontSizeOption;
};

export type AppearancePreferencePatch = Partial<AppearancePreferences>;

function isPlainObject(value: unknown): value is Record<string, unknown> {
  return typeof value === "object" && value !== null && !Array.isArray(value);
}

function isThemePreset(value: unknown): value is ThemePreset {
  return typeof value === "string" && (THEME_PRESETS as readonly string[]).includes(value);
}

export function parseAppearancePreferences(raw: string | null | undefined): AppearancePreferencePatch {
  if (!raw) {
    return {};
  }

  try {
    const parsed: unknown = JSON.parse(raw);
    if (!isPlainObject(parsed)) {
      return {};
    }

    const result: AppearancePreferencePatch = {};
    if (isThemePreset(parsed.preset)) {
      result.preset = parsed.preset;
    }
    if (isChatFontOption(parsed.chatFont)) {
      result.chatFont = parsed.chatFont;
    }
    if (isChatFontWeightOption(parsed.chatFontWeight)) {
      result.chatFontWeight = parsed.chatFontWeight;
    }
    if (isFontSizeOption(parsed.fontSize)) {
      result.fontSize = parsed.fontSize;
    }
    return result;
  } catch {
    return {};
  }
}

export function readLocalAppearancePreferences(): AppearancePreferences {
  if (typeof window === "undefined") {
    return {
      preset: "default",
      chatFont: "default",
      chatFontWeight: "regular",
      fontSize: "standard",
    };
  }

  const storedPreset = window.localStorage.getItem(THEME_PRESET_STORAGE_KEY);
  const storedChatFont = window.localStorage.getItem(CHAT_FONT_STORAGE_KEY);
  const storedChatFontWeight = window.localStorage.getItem(CHAT_FONT_WEIGHT_STORAGE_KEY);
  const storedFontSize = window.localStorage.getItem(FONT_SIZE_STORAGE_KEY);
  return {
    preset: isThemePreset(storedPreset) ? storedPreset : "default",
    chatFont: isChatFontOption(storedChatFont) ? storedChatFont : "default",
    chatFontWeight: isChatFontWeightOption(storedChatFontWeight) ? storedChatFontWeight : "regular",
    fontSize: isFontSizeOption(storedFontSize) ? storedFontSize : "standard",
  };
}

export function resolveAppearancePreferences(
  accountPreferences: string | null | undefined,
): AppearancePreferences {
  return {
    ...readLocalAppearancePreferences(),
    ...parseAppearancePreferences(accountPreferences),
  };
}

export function serializeAppearancePreferences(preferences: AppearancePreferences): string {
  return JSON.stringify({
    preset: preferences.preset,
    chatFont: preferences.chatFont,
    chatFontWeight: preferences.chatFontWeight,
    fontSize: preferences.fontSize,
  });
}
