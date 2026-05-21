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
  THEME_PRESET_STORAGE_KEY,
  THEME_STORAGE_KEY,
  type Theme,
  type ThemePreset,
} from "@/shared/components/theme-provider";

export type AppearancePreferences = {
  theme: Theme;
  preset: ThemePreset;
  chatFont: ChatFontOption;
  chatFontWeight: ChatFontWeightOption;
};

export type AppearancePreferencePatch = Partial<AppearancePreferences>;

function isPlainObject(value: unknown): value is Record<string, unknown> {
  return typeof value === "object" && value !== null && !Array.isArray(value);
}

function isTheme(value: unknown): value is Theme {
  return value === "light" || value === "dark" || value === "system";
}

function isThemePreset(value: unknown): value is ThemePreset {
  return value === "default" || value === "azure" || value === "cobalt" || value === "graphite" || value === "lagoon" || value === "ink" || value === "ochre" || value === "sepia";
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
    if (isTheme(parsed.theme)) {
      result.theme = parsed.theme;
    }
    if (isThemePreset(parsed.preset)) {
      result.preset = parsed.preset;
    }
    if (isChatFontOption(parsed.chatFont)) {
      result.chatFont = parsed.chatFont;
    }
    if (isChatFontWeightOption(parsed.chatFontWeight)) {
      result.chatFontWeight = parsed.chatFontWeight;
    }
    return result;
  } catch {
    return {};
  }
}

export function readLocalAppearancePreferences(): AppearancePreferences {
  if (typeof window === "undefined") {
    return {
      theme: "system",
      preset: "default",
      chatFont: "default",
      chatFontWeight: "regular",
    };
  }

  const storedTheme = window.localStorage.getItem(THEME_STORAGE_KEY);
  const storedPreset = window.localStorage.getItem(THEME_PRESET_STORAGE_KEY);
  const storedChatFont = window.localStorage.getItem(CHAT_FONT_STORAGE_KEY);
  const storedChatFontWeight = window.localStorage.getItem(CHAT_FONT_WEIGHT_STORAGE_KEY);
  return {
    theme: isTheme(storedTheme) ? storedTheme : "system",
    preset: isThemePreset(storedPreset) ? storedPreset : "default",
    chatFont: isChatFontOption(storedChatFont) ? storedChatFont : "default",
    chatFontWeight: isChatFontWeightOption(storedChatFontWeight) ? storedChatFontWeight : "regular",
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
    theme: preferences.theme,
    preset: preferences.preset,
    chatFont: preferences.chatFont,
    chatFontWeight: preferences.chatFontWeight,
  });
}
