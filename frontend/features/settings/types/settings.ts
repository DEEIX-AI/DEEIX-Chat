import type { ChatFontOption, ChatFontWeightOption } from "@/features/settings/utils/chat-font";
import type { ThemePreset } from "@/shared/components/theme-provider";
import type { PublicModelDTO } from "@/shared/api/model.types";

export type SendShortcut = "enter" | "ctrl_enter" | "meta_enter";
export type FileMode = "auto" | "full_context" | "rag";
export type ChatInputHeight = "compact" | "standard" | "loose";

export type ChatSettings = {
  defaultModel: string;
  sendShortcut: SendShortcut;
  showTokenUsage: boolean;
  showModelInfo: boolean;
  showLatency: boolean;
  showBillingCost: boolean;
  markdownRender: boolean;
  contextCompactAuto: boolean;
  restoreDraftOnFailure: boolean;
  preserveConversationDrafts: boolean;
  inputHeight: ChatInputHeight;
  fileMode: FileMode;
};

export type ModelVendorGroup = [vendor: string, items: PublicModelDTO[]];

export type ProfileDraft = {
  avatarUrl: string;
  displayName: string;
  timezone: string;
  locale: string;
  profilePreferences: string;
};

export type ThemeMode = "light" | "system" | "dark";

export type ThemePresetPreview = {
  label: string;
  tone: "cool" | "neutral" | "warm";
  value: ThemePreset;
  light: ThemePreviewPalette;
  dark: ThemePreviewPalette;
};

export type ThemePreviewPalette = {
  background: string;
  sidebar: string;
  sidebarBorder: string;
  surface: string;
  surfaceBorder: string;
  textStrong: string;
  textSoft: string;
  accent: string;
};

export type ChatFontPreview = {
  label: string;
  value: ChatFontOption;
  fontFamily: string;
  sampleText: string;
};

export type ChatFontWeightPreview = {
  label: string;
  value: ChatFontWeightOption;
  fontWeight: number;
  sampleText: string;
};
