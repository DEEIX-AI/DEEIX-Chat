"use client";

import * as React from "react";

import {
  THEME_PRESET_STORAGE_KEY,
  THEME_PRESETS,
  THEME_STORAGE_KEY,
  THEMES,
  type Theme,
  type ThemePreset,
} from "@/shared/model/theme";

type ThemeContextValue = {
  theme: Theme;
  preset: ThemePreset;
  setTheme: (theme: Theme) => void;
  setPreset: (preset: ThemePreset) => void;
  resolvedTheme: "light" | "dark";
  systemTheme: "light" | "dark";
  themes: Theme[];
  presets: ThemePreset[];
};

const ThemeContext = React.createContext<ThemeContextValue | null>(null);

function resolveSystemTheme(): "light" | "dark" {
  if (typeof window === "undefined") return "light";
  return window.matchMedia("(prefers-color-scheme: dark)").matches ? "dark" : "light";
}

export function normalizeTheme(value: string | null | undefined): Theme {
  return (THEMES as readonly string[]).includes(value ?? "") ? (value as Theme) : "system";
}

export function normalizeThemePreset(value: string | null | undefined): ThemePreset {
  return (THEME_PRESETS as readonly string[]).includes(value ?? "") ? (value as ThemePreset) : "default";
}

function applyTheme(theme: Theme, systemTheme: "light" | "dark", preset: ThemePreset) {
  const resolvedTheme = theme === "system" ? systemTheme : theme;
  const root = document.documentElement;
  root.classList.remove("light", "dark");
  root.classList.add(resolvedTheme);
  root.dataset.theme = preset;
  root.style.colorScheme = resolvedTheme;
}

export function ThemeProvider({
  children,
}: {
  children: React.ReactNode;
}) {
  const [theme, setThemeState] = React.useState<Theme>("system");
  const [preset, setPresetState] = React.useState<ThemePreset>("default");
  const [systemTheme, setSystemTheme] = React.useState<"light" | "dark">("light");
  const themeRef = React.useRef<Theme>("system");
  const presetRef = React.useRef<ThemePreset>("default");

  React.useEffect(() => {
    const initialSystemTheme = resolveSystemTheme();
    const storedTheme = window.localStorage.getItem(THEME_STORAGE_KEY);
    const storedPreset = window.localStorage.getItem(THEME_PRESET_STORAGE_KEY);
    const initialTheme = normalizeTheme(storedTheme);
    const initialPreset = normalizeThemePreset(storedPreset);
    themeRef.current = initialTheme;
    presetRef.current = initialPreset;
    setThemeState(initialTheme);
    setPresetState(initialPreset);
    setSystemTheme(initialSystemTheme);
    applyTheme(initialTheme, initialSystemTheme, initialPreset);

    const mediaQuery = window.matchMedia("(prefers-color-scheme: dark)");
    const handleSystemThemeChange = () => {
      const nextSystemTheme = resolveSystemTheme();
      setSystemTheme(nextSystemTheme);
      applyTheme(themeRef.current, nextSystemTheme, presetRef.current);
    };
    mediaQuery.addEventListener("change", handleSystemThemeChange);
    return () => mediaQuery.removeEventListener("change", handleSystemThemeChange);
  }, []);

  const setTheme = React.useCallback(
    (nextTheme: Theme) => {
      themeRef.current = nextTheme;
      setThemeState(nextTheme);
      window.localStorage.setItem(THEME_STORAGE_KEY, nextTheme);
      applyTheme(nextTheme, systemTheme, presetRef.current);
    },
    [systemTheme],
  );

  const setPreset = React.useCallback(
    (nextPreset: ThemePreset) => {
      presetRef.current = nextPreset;
      setPresetState(nextPreset);
      window.localStorage.setItem(THEME_PRESET_STORAGE_KEY, nextPreset);
      applyTheme(themeRef.current, systemTheme, nextPreset);
    },
    [systemTheme],
  );

  const value = React.useMemo<ThemeContextValue>(
    () => ({
      theme,
      preset,
      setTheme,
      setPreset,
      resolvedTheme: theme === "system" ? systemTheme : theme,
      systemTheme,
      themes: [...THEMES],
      presets: [...THEME_PRESETS],
    }),
    [preset, setPreset, setTheme, systemTheme, theme],
  );

  return <ThemeContext.Provider value={value}>{children}</ThemeContext.Provider>;
}

export function useTheme(): ThemeContextValue {
  const context = React.useContext(ThemeContext);
  if (!context) {
    return {
      theme: "system" as Theme,
      preset: "default" as ThemePreset,
      setTheme: () => undefined,
      setPreset: () => undefined,
      resolvedTheme: "light" as const,
      systemTheme: "light" as const,
      themes: [...THEMES],
      presets: [...THEME_PRESETS],
    };
  }
  return context;
}
