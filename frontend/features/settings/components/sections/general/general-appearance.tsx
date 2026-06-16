"use client";

import * as React from "react";
import { Monitor, Moon, Sun } from "lucide-react";
import { useTranslations } from "next-intl";

import { Field, FieldGroup, FieldLabel } from "@/components/ui/field";
import type { FontSizeOption } from "@/features/settings/utils/font-size";
import type {
  FontSizePreview,
  ThemeMode,
  ThemePresetPreview,
  ThemePreviewPalette,
} from "@/features/settings/types/settings";
import { cn } from "@/lib/utils";
import type { ThemePreset } from "@/shared/components/theme-provider";
import { SettingsSection } from "@/shared/components/settings-layout";

const THEME_PREVIEW_PALETTES: Record<"light" | "dark", ThemePreviewPalette> = {
  light: {
    background: "oklch(0.9818 0.0054 95.1)",
    sidebar: "oklch(0.9791 0.0041 91.45)",
    sidebarBorder: "oklch(0.93 0 0)",
    surface: "oklch(1 0 0)",
    surfaceBorder: "oklch(0.9 0 0)",
    textStrong: "oklch(0.25 0.0269 95.7226)",
    textSoft: "oklch(0.6059 0.0075 97.4233)",
    accent: "oklch(0.667 0.1081 42.03)",
  },
  dark: {
    background: "oklch(0.2637 0.0037 106.65)",
    sidebar: "oklch(0.2357 0.0024 67.7077)",
    sidebarBorder: "oklch(0.3085 0.0035 106.6039)",
    surface: "oklch(0.3085 0.0035 106.6039)",
    surfaceBorder: "oklch(0.4336 0.0113 100.2195)",
    textStrong: "oklch(0.9211 0.004 106.4781)",
    textSoft: "oklch(0.65 0.0142 93.0137)",
    accent: "oklch(0.6724 0.1308 38.7559)",
  },
};

const THEME_PRESET_PREVIEWS: ThemePresetPreview[] = [
  {
    label: "Default",
    tone: "warm",
    value: "default",
    light: THEME_PREVIEW_PALETTES.light,
    dark: THEME_PREVIEW_PALETTES.dark,
  },
  {
    label: "Azure",
    tone: "cool",
    value: "azure",
    light: {
      background: "oklch(1 0 0)",
      sidebar: "oklch(0.9784 0.0011 197.1387)",
      sidebarBorder: "oklch(0.9271 0.0101 238.5177)",
      surface: "oklch(0.9784 0.0011 197.1387)",
      surfaceBorder: "oklch(0.9317 0.0118 231.6594)",
      textStrong: "oklch(0.1884 0.0128 248.5103)",
      textSoft: "oklch(0.5637 0.0078 247.9662)",
      accent: "oklch(0.6723 0.1606 244.9955)",
    },
    dark: {
      background: "oklch(0.1580 0.0060 255)",
      sidebar: "oklch(0.1760 0.0070 255)",
      sidebarBorder: "oklch(0.2650 0.0080 250)",
      surface: "oklch(0.1880 0.0080 255)",
      surfaceBorder: "oklch(0.2850 0.0080 250)",
      textStrong: "oklch(0.9328 0.0025 228.7857)",
      textSoft: "oklch(0.7000 0.0078 247.9662)",
      accent: "oklch(0.6692 0.1607 245.0110)",
    },
  },
  {
    label: "Cobalt",
    tone: "cool",
    value: "cobalt",
    light: {
      background: "oklch(1.0000 0 0)",
      sidebar: "oklch(0.9848 0 0)",
      sidebarBorder: "oklch(0.9234 0 0)",
      surface: "oklch(1.0000 0 0)",
      surfaceBorder: "oklch(0.8610 0 0)",
      textStrong: "oklch(0.2697 0 0)",
      textSoft: "oklch(0.5547 0 0)",
      accent: "oklch(0.5296 0.1863 258.1459)",
    },
    dark: {
      background: "oklch(0.2376 0 0)",
      sidebar: "oklch(0.2156 0 0)",
      sidebarBorder: "oklch(0.3411 0 0)",
      surface: "oklch(0.2697 0 0)",
      surfaceBorder: "oklch(0.3705 0 0)",
      textStrong: "oklch(0.9389 0 0)",
      textSoft: "oklch(0.7244 0 0)",
      accent: "oklch(0.7782 0.1158 252.5545)",
    },
  },
  {
    label: "Graphite",
    tone: "neutral",
    value: "graphite",
    light: {
      background: "oklch(0.9900 0 0)",
      sidebar: "oklch(0.9900 0 0)",
      sidebarBorder: "oklch(0.9400 0 0)",
      surface: "oklch(0.9900 0 0)",
      surfaceBorder: "oklch(0.9200 0 0)",
      textStrong: "oklch(0 0 0)",
      textSoft: "oklch(0.4400 0 0)",
      accent: "oklch(0 0 0)",
    },
    dark: {
      background: "oklch(0 0 0)",
      sidebar: "oklch(0 0 0)",
      sidebarBorder: "oklch(0.3200 0 0)",
      surface: "oklch(0 0 0)",
      surfaceBorder: "oklch(0.2600 0 0)",
      textStrong: "oklch(1 0 0)",
      textSoft: "oklch(0.7200 0 0)",
      accent: "oklch(0.6480 0.2000 131.6840)",
    },
  },
  {
    label: "Lagoon",
    tone: "cool",
    value: "lagoon",
    light: {
      background: "oklch(0.9876 0.0044 185.3322)",
      sidebar: "oklch(0.9804 0.0050 185.3165)",
      sidebarBorder: "oklch(0.9326 0.0138 186.2170)",
      surface: "oklch(1.0000 0 0)",
      surfaceBorder: "oklch(0.9214 0.0198 186.0428)",
      textStrong: "oklch(0.3079 0.0364 195.4276)",
      textSoft: "oklch(0.5873 0.0387 196.1462)",
      accent: "oklch(0.6840 0.1270 176.8287)",
    },
    dark: {
      background: "oklch(0.1592 0.0163 195.6380)",
      sidebar: "oklch(0.1426 0.0142 195.6773)",
      sidebarBorder: "oklch(0.2601 0.0242 195.7669)",
      surface: "oklch(0.2032 0.0218 195.5721)",
      surfaceBorder: "oklch(0.3331 0.0335 195.6612)",
      textStrong: "oklch(0.9724 0.0044 185.3299)",
      textSoft: "oklch(0.7768 0.0173 184.8604)",
      accent: "oklch(0.8607 0.1582 177.3031)",
    },
  },
  {
    label: "Ink",
    tone: "cool",
    value: "ink",
    light: {
      background: "oklch(0.9800 0.0300 260)",
      sidebar: "oklch(0.9800 0.0300 260)",
      sidebarBorder: "oklch(0.9400 0 0)",
      surface: "oklch(0.9900 0.0200 260)",
      surfaceBorder: "oklch(0.9200 0 0)",
      textStrong: "oklch(0.0600 0.0100 270)",
      textSoft: "oklch(0.4400 0 0)",
      accent: "oklch(0.0600 0.0100 270)",
    },
    dark: {
      background: "oklch(0.0400 0.0050 270)",
      sidebar: "oklch(0.1800 0.0050 270)",
      sidebarBorder: "oklch(0.3200 0 0)",
      surface: "oklch(0.1400 0.0050 270)",
      surfaceBorder: "oklch(0.2600 0 0)",
      textStrong: "oklch(0.8800 0.0400 260)",
      textSoft: "oklch(0.7200 0 0)",
      accent: "oklch(0.8800 0.0400 260)",
    },
  },
  {
    label: "Ochre",
    tone: "warm",
    value: "ochre",
    light: {
      background: "oklch(0.9808 0.0079 73.7452)",
      sidebar: "oklch(0.9550 0.0179 64.9320)",
      sidebarBorder: "oklch(0.8607 0.0222 69.7490)",
      surface: "oklch(0.9724 0.0096 72.6627)",
      surfaceBorder: "oklch(0.8900 0.0196 72.5571)",
      textStrong: "oklch(0.1804 0.0154 57.0973)",
      textSoft: "oklch(0.4806 0.0254 51.1528)",
      accent: "oklch(0.7006 0.1891 46.5400)",
    },
    dark: {
      background: "oklch(0.1488 0.0098 61.6463)",
      sidebar: "oklch(0.1488 0.0098 61.6463)",
      sidebarBorder: "oklch(0.2311 0.0097 52.9899)",
      surface: "oklch(0.2606 0.0040 84.5838)",
      surfaceBorder: "oklch(0.2701 0.0106 48.3077)",
      textStrong: "oklch(0.9206 0.0042 56.3709)",
      textSoft: "oklch(0.5802 0.0145 48.4582)",
      accent: "oklch(0.7006 0.1891 46.5400)",
    },
  },
  {
    label: "Sepia",
    tone: "warm",
    value: "sepia",
    light: {
      background: "oklch(0.9821 0 0)",
      sidebar: "oklch(0.9881 0 0)",
      sidebarBorder: "oklch(0.9401 0 0)",
      surface: "oklch(0.9911 0 0)",
      surfaceBorder: "oklch(0.8822 0 0)",
      textStrong: "oklch(0.2435 0 0)",
      textSoft: "oklch(0.5032 0 0)",
      accent: "oklch(0.4341 0.0392 41.9938)",
    },
    dark: {
      background: "oklch(0.1776 0 0)",
      sidebar: "oklch(0.2103 0.0059 285.8852)",
      sidebarBorder: "oklch(0.2739 0.0055 286.0326)",
      surface: "oklch(0.2134 0 0)",
      surfaceBorder: "oklch(0.2351 0.0115 91.7467)",
      textStrong: "oklch(0.9491 0 0)",
      textSoft: "oklch(0.7699 0 0)",
      accent: "oklch(0.9247 0.0524 66.1732)",
    },
  },
];

const FONT_SIZE_OPTIONS: FontSizePreview[] = [
  { label: "Small", value: "small", scale: 0.88 },
  { label: "Standard", value: "standard", scale: 1 },
  { label: "Medium", value: "medium", scale: 1.12 },
  { label: "Large", value: "large", scale: 1.24 },
];

function ThemePresetPreviewCard({
  item,
  resolvedMode,
  active,
  onSelect,
}: {
  item: ThemePresetPreview;
  resolvedMode: "light" | "dark";
  active: boolean;
  onSelect: (preset: ThemePresetPreview["value"]) => void;
}) {
  const palette = resolvedMode === "dark" ? item.dark : item.light;
  const swatches = [
    palette.accent,
    palette.textStrong,
    palette.sidebar,
    palette.surface,
    palette.background,
  ];

  return (
    <button
      type="button"
      onClick={() => onSelect(item.value)}
      className="group min-w-0 text-left"
      aria-pressed={active}
    >
      <div
        className={cn(
          "relative h-24 w-full overflow-hidden rounded-xl border transition-all duration-200 hover:scale-102 hover:border-primary/60",
          active ? "border-primary/60" : "border-border/50",
        )}
        style={{ backgroundColor: palette.background }}
      >
        <div
          className="absolute left-2.5 top-2.5 rounded-full px-2 py-0.5 text-[10px] font-medium leading-none"
          style={{
            backgroundColor: palette.surface,
            color: palette.textStrong,
          }}
        >
          {item.tone}
        </div>

        <div className="absolute right-2.5 top-2.5 flex items-start gap-1.5">
          {swatches.map((color, index) => (
            <span
              key={`${item.value}-${index}`}
              className="h-9 w-2.5 rounded-full border-[0.5px]"
              style={{ backgroundColor: color, borderColor: palette.surfaceBorder }}
            />
          ))}
        </div>

        <div
          className="absolute inset-x-0 bottom-0 h-14"
          style={{
            background: `linear-gradient(to top, ${palette.background} 42%, transparent)`,
          }}
        />
        <span
          className="absolute bottom-3 left-3 right-3 truncate text-lg font-semibold leading-none tracking-normal"
          style={{ color: palette.textStrong }}
        >
          {item.label.toLowerCase()}
        </span>
      </div>
    </button>
  );
}

function ThemePreviewCard({
  label,
  mode,
  lightPalette,
  darkPalette,
  active,
  onSelect,
}: {
  label: string;
  mode: ThemeMode;
  lightPalette: ThemePreviewPalette;
  darkPalette: ThemePreviewPalette;
  active: boolean;
  onSelect: (mode: ThemeMode) => void;
}) {
  const Icon = mode === "light" ? Sun : mode === "dark" ? Moon : Monitor;
  const previewPalette = mode === "dark" ? darkPalette : lightPalette;
  const foregroundPalette = mode === "dark" ? darkPalette : lightPalette;
  const lightClipPath = "polygon(0 0, 72% 0, 28% 100%, 0 100%)";
  const darkClipPath = "polygon(72% 0, 100% 0, 100% 100%, 28% 100%)";
  const content = (
    <span className="flex max-w-[calc(100%-1.5rem)] items-center gap-2">
      <Icon className="size-4 shrink-0" />
      <span className="truncate text-sm font-medium">{label}</span>
    </span>
  );

  return (
    <button
      type="button"
      onClick={() => onSelect(mode)}
      className="group text-left"
      aria-pressed={active}
    >
      <div
        className={cn(
          "relative flex h-24 w-full items-center justify-center overflow-hidden rounded-xl border transition-all duration-200 hover:scale-102 hover:border-primary/60",
          active ? "border-primary/60" : "border-border/50",
        )}
        style={{ backgroundColor: previewPalette.background }}
      >
        {mode === "system" ? (
          <>
            <span
              className="absolute inset-0"
              style={{
                backgroundColor: lightPalette.background,
                clipPath: lightClipPath,
              }}
              aria-hidden="true"
            />
            <span
              className="absolute inset-0"
              style={{
                backgroundColor: darkPalette.background,
                clipPath: darkClipPath,
              }}
              aria-hidden="true"
            />
            <span
              className="pointer-events-none absolute inset-0 flex items-center justify-center"
              style={{ color: lightPalette.textStrong, clipPath: lightClipPath }}
              aria-hidden="true"
            >
              {content}
            </span>
            <span
              className="pointer-events-none absolute inset-0 flex items-center justify-center"
              style={{ color: darkPalette.textStrong, clipPath: darkClipPath }}
            >
              {content}
            </span>
          </>
        ) : (
          <span className="relative flex max-w-[calc(100%-1.5rem)] items-center gap-2" style={{ color: foregroundPalette.textStrong }}>
            <Icon className="size-4 shrink-0" />
            <span className="truncate text-sm font-medium">{label}</span>
          </span>
        )}
      </div>
    </button>
  );
}

function FontSizePreviewCard({
  item,
  active,
  onSelect,
}: {
  item: FontSizePreview;
  active: boolean;
  onSelect: (value: FontSizeOption) => void;
}) {
  return (
    <button
      type="button"
      onClick={() => onSelect(item.value)}
      className="group text-left"
      aria-pressed={active}
    >
      <div
        className={cn(
          "flex h-24 w-full items-center justify-center rounded-xl border bg-background px-1 transition-all duration-200 hover:scale-102 hover:border-primary/60",
          active ? "border-primary/60" : "border-border/50",
        )}
      >
        <span
          className="truncate text-center font-medium leading-none text-foreground/90"
          style={{ fontSize: `calc(1rem * ${item.scale})` }}
        >
          {item.label} Aa
        </span>
      </div>
    </button>
  );
}

export function GeneralAppearanceSection({
  resolvedTheme,
  activeThemeMode,
  activeThemePreset,
  fontSize,
  onThemeModeChange,
  onThemePresetChange,
  onFontSizeChange,
}: {
  resolvedTheme: "light" | "dark";
  activeThemeMode: ThemeMode;
  activeThemePreset: ThemePreset;
  fontSize: FontSizeOption;
  onThemeModeChange: (mode: ThemeMode) => void;
  onThemePresetChange: (preset: ThemePreset) => void;
  onFontSizeChange: (value: FontSizeOption) => void;
}) {
  const t = useTranslations("settings");
  const activeThemePresetPreview = React.useMemo(
    () => THEME_PRESET_PREVIEWS.find((item) => item.value === activeThemePreset) ?? THEME_PRESET_PREVIEWS[0],
    [activeThemePreset],
  );

  return (
    <SettingsSection title={t("appearance")}>
      <FieldGroup className="gap-3 md:gap-4">
        <Field>
          <FieldLabel>{t("generalPage.appearance.themePreset")}</FieldLabel>
          <div className="grid grid-cols-2 gap-2 sm:grid-cols-4 md:gap-3 xl:gap-4">
            {THEME_PRESET_PREVIEWS.map((item) => (
              <ThemePresetPreviewCard
                key={item.value}
                item={{ ...item, label: t(`generalPage.appearance.preset.${item.value}`) }}
                resolvedMode={resolvedTheme}
                active={activeThemePreset === item.value}
                onSelect={onThemePresetChange}
              />
            ))}
          </div>
        </Field>

        <Field>
          <FieldLabel>{t("generalPage.appearance.colorMode")}</FieldLabel>
          <div className="grid grid-cols-2 gap-2 sm:grid-cols-4 md:gap-3 xl:gap-4">
            <ThemePreviewCard
              label={t("generalPage.appearance.theme.light")}
              mode="light"
              lightPalette={activeThemePresetPreview.light}
              darkPalette={activeThemePresetPreview.dark}
              active={activeThemeMode === "light"}
              onSelect={onThemeModeChange}
            />
            <ThemePreviewCard
              label={t("generalPage.appearance.theme.system")}
              mode="system"
              lightPalette={activeThemePresetPreview.light}
              darkPalette={activeThemePresetPreview.dark}
              active={activeThemeMode === "system"}
              onSelect={onThemeModeChange}
            />
            <ThemePreviewCard
              label={t("generalPage.appearance.theme.dark")}
              mode="dark"
              lightPalette={activeThemePresetPreview.light}
              darkPalette={activeThemePresetPreview.dark}
              active={activeThemeMode === "dark"}
              onSelect={onThemeModeChange}
            />
          </div>
        </Field>

        <Field>
          <FieldLabel>{t("generalPage.appearance.fontSize")}</FieldLabel>
          <div className="grid grid-cols-2 gap-2 sm:grid-cols-4 md:gap-3 xl:gap-4">
            {FONT_SIZE_OPTIONS.map((item) => (
              <FontSizePreviewCard
                key={item.value}
                item={{ ...item, label: t(`generalPage.appearance.fontSizeOption.${item.value}`) }}
                active={fontSize === item.value}
                onSelect={onFontSizeChange}
              />
            ))}
          </div>
        </Field>

      </FieldGroup>
    </SettingsSection>
  );
}
