export const THEMES = ["light", "dark", "system"] as const;
export const THEME_PRESETS = ["default", "azure", "cobalt", "graphite", "lagoon", "ink", "ochre", "sepia"] as const;

export type Theme = (typeof THEMES)[number];
export type ThemePreset = (typeof THEME_PRESETS)[number];

export const THEME_STORAGE_KEY = "theme";
export const THEME_PRESET_STORAGE_KEY = "theme-preset";
