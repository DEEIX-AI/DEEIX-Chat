"use client";

import * as React from "react";
import { Monitor, Moon, Sun } from "lucide-react";
import { useTranslations } from "next-intl";
import { toast } from "sonner";

import { Avatar, AvatarFallback, AvatarImage } from "@/components/ui/avatar";
import { Button } from "@/components/ui/button";
import { SpinnerLabel } from "@/components/ui/spinner";
import { Switch } from "@/components/ui/switch";
import { dispatchUserProfileUpdated } from "@/features/settings/events/user-profile-events";
import { useTheme } from "@/shared/components/theme-provider";
import { Dialog, DialogContent, DialogDescription, DialogFooter, DialogHeader, DialogTitle } from "@/components/ui/dialog";
import { Field, FieldGroup, FieldLabel } from "@/components/ui/field";
import { Input } from "@/components/ui/input";
import { Textarea } from "@/components/ui/textarea";
import {
  readLocalAppearancePreferences,
  serializeAppearancePreferences,
  type AppearancePreferencePatch,
} from "@/features/settings/utils/appearance-preferences";
import {
  type ChatFontOption,
  type ChatFontWeightOption,
  useChatFontPreference,
  useChatFontWeightPreference,
  writeChatFontPreference,
  writeChatFontWeightPreference,
} from "@/features/settings/utils/chat-font";
import type {
  ChatFontPreview,
  ChatFontWeightPreview,
  ProfileDraft,
  ThemeMode,
  ThemePresetPreview,
  ThemePreviewPalette,
} from "@/features/settings/types/settings";
import {
  createDraftFromUser,
  isProfileDraftEqual,
  resolveSettingsErrorMessage,
} from "@/features/settings/utils/profile-settings";
import { createGeneratedGithubAvatarRef, generateAvatarVariant, resolveAvatarImageSrc } from "@/shared/lib/avatar";
import { useAuthSession } from "@/shared/auth/auth-session-context";
import {
  disableResponseCompletionNotifications,
  enableResponseCompletionNotifications,
  getBrowserNotificationPermission,
  isBrowserNotificationSupported,
  readResponseCompletionNotificationsEnabled,
} from "@/shared/lib/browser-notifications";
import { patchMe, patchUsername } from "@/shared/api/auth";
import { ApiError } from "@/shared/api/http-client";
import {
  DISPLAY_NAME_MAX_LENGTH,
  USERNAME_MAX_LENGTH,
  isDisplayNameLengthValid,
  isUsernamePolicyValid,
} from "@/shared/auth/account-policy";
import { cn } from "@/lib/utils";
import type { UserDTO } from "@/shared/api/auth.types";
import {
  SettingsFieldRow,
  SettingsPage,
  SettingsSection,
  SettingsSectionSeparator,
} from "@/shared/components/settings-layout";
import { TimeZoneSelect } from "@/shared/components/time-zone-select";

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

const CHAT_FONT_OPTIONS: ChatFontPreview[] = [
  { label: "Default", value: "default", fontFamily: "var(--font-sans)", sampleText: "Aa" },
  { label: "Serif", value: "songti", fontFamily: "var(--font-songti)", sampleText: "Aa" },
  { label: "Sans", value: "heiti", fontFamily: "var(--font-heiti)", sampleText: "Aa" },
  { label: "Mono", value: "mono", fontFamily: "var(--font-mono)", sampleText: "Aa" },
];

const CHAT_FONT_WEIGHT_OPTIONS: ChatFontWeightPreview[] = [
  { label: "Regular", value: "regular", fontWeight: 400, sampleText: "Aa" },
  { label: "Medium", value: "medium", fontWeight: 500, sampleText: "Aa" },
  { label: "Semibold", value: "semibold", fontWeight: 600, sampleText: "Aa" },
  { label: "Bold", value: "bold", fontWeight: 700, sampleText: "Aa" },
];

function ThemePreviewCanvas({ palette }: { palette: ThemePreviewPalette }) {
  return (
    <div className="absolute inset-0 overflow-hidden rounded-[9px]" style={{ backgroundColor: palette.background }}>
      <div
        className="absolute inset-y-0 left-0 w-[26%] border-r"
        style={{ backgroundColor: palette.sidebar, borderColor: palette.sidebarBorder }}
      >
        <div className="space-y-1 px-1.5 pt-2">
          <div className="h-px w-4 rounded-full" style={{ backgroundColor: palette.textStrong, opacity: 0.42 }} />
          <div className="h-px w-3 rounded-full" style={{ backgroundColor: palette.textSoft, opacity: 0.7 }} />
        </div>

        <div className="space-y-1.5 px-1.5 pt-4">
          <div className="h-px w-3 rounded-full" style={{ backgroundColor: palette.textSoft, opacity: 0.55 }} />
          <div className="h-px w-2.5 rounded-full" style={{ backgroundColor: palette.textSoft, opacity: 0.4 }} />
          <div className="h-px w-3.5 rounded-full" style={{ backgroundColor: palette.textSoft, opacity: 0.3 }} />
        </div>
      </div>

      <div className="absolute inset-y-0 right-0 left-[26%]">
        <div className="flex items-center justify-between px-2.5 pt-2">
          <div className="space-y-1">
            <div className="h-px w-5 rounded-full" style={{ backgroundColor: palette.textStrong, opacity: 0.45 }} />
            <div className="h-px w-7 rounded-full" style={{ backgroundColor: palette.textSoft, opacity: 0.45 }} />
          </div>
          <div
            className="h-2.5 w-6 rounded-full"
            style={{ backgroundColor: palette.surface, border: `1px solid ${palette.surfaceBorder}` }}
          />
        </div>

        <div
          className="absolute inset-x-2.5 top-8 rounded-[8px] border"
          style={{ height: "24px", backgroundColor: palette.surface, borderColor: palette.surfaceBorder }}
        >
          <div className="space-y-1 px-2 pt-2">
            <div className="h-px w-6 rounded-full" style={{ backgroundColor: palette.textStrong, opacity: 0.4 }} />
            <div className="h-px w-8 rounded-full" style={{ backgroundColor: palette.textSoft, opacity: 0.32 }} />
          </div>
        </div>

        <div
          className="absolute inset-x-2.5 bottom-2.5 rounded-[8px] border"
          style={{ height: "20px", backgroundColor: palette.surface, borderColor: palette.surfaceBorder }}
        >
          <div className="absolute bottom-1.5 right-1.5 h-2 w-2 rounded-[3px]" style={{ backgroundColor: palette.accent }} />
        </div>
      </div>
    </div>
  );
}

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
function ChatFontPreviewCard({
  item,
  active,
  onSelect,
}: {
  item: ChatFontPreview;
  active: boolean;
  onSelect: (value: ChatFontOption) => void;
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
          className="truncate text-center text-[clamp(0.9rem,2.8vw,1.125rem)] leading-none text-foreground/90"
          style={{ fontFamily: item.fontFamily }}
        >
          {item.label} {item.sampleText}
        </span>
      </div>
    </button>
  );
}

function ChatFontWeightPreviewCard({
  item,
  active,
  onSelect,
}: {
  item: ChatFontWeightPreview;
  active: boolean;
  onSelect: (value: ChatFontWeightOption) => void;
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
          className="truncate text-center text-[clamp(0.9rem,2.8vw,1.125rem)] leading-none text-foreground/90"
          style={{ fontFamily: "var(--font-chat)", fontWeight: item.fontWeight }}
        >
          {item.label} {item.sampleText}
        </span>
      </div>
    </button>
  );
}

function resolveUsernameErrorMessage(
  error: unknown,
  labels: { invalid: string; alreadyChanged: string; taken: string },
): string {
  if (error instanceof ApiError) {
    if (error.status === 400) {
      return labels.invalid;
    }
    if (error.status === 409) {
      return error.message.includes("already used") ? labels.alreadyChanged : labels.taken;
    }
  }
  return resolveSettingsErrorMessage(error);
}

export function SettingsGeneral() {
  const t = useTranslations("settings");
  const common = useTranslations("common");
  const { accessToken, user, userStatus } = useAuthSession();
  const { preset, resolvedTheme, setPreset, setTheme, theme } = useTheme();
  const [viewer, setViewer] = React.useState<UserDTO | null>(null);
  const [draft, setDraft] = React.useState<ProfileDraft>(() => createDraftFromUser());
  const [initialDraft, setInitialDraft] = React.useState<ProfileDraft>(() => createDraftFromUser());
  const [avatarDialogOpen, setAvatarDialogOpen] = React.useState(false);
  const [avatarDialogValue, setAvatarDialogValue] = React.useState("");
  const [themeRuntimeReady, setThemeRuntimeReady] = React.useState(false);
  const chatFont = useChatFontPreference();
  const chatFontWeight = useChatFontWeightPreference();
  const [notificationRuntimeReady, setNotificationRuntimeReady] = React.useState(false);
  const [notificationSupported, setNotificationSupported] = React.useState(false);
  const [responseCompletionNotificationsEnabled, setResponseCompletionNotificationsEnabled] = React.useState(false);
  const [notificationPermission, setNotificationPermission] = React.useState<NotificationPermission | "unsupported">("unsupported");
  const [loading, setLoading] = React.useState(true);
  const [saving, setSaving] = React.useState(false);
  const [usernameDraft, setUsernameDraft] = React.useState("");
  const initialUsernameToastShownRef = React.useRef(false);
  const appearanceSaveTimerRef = React.useRef<number | null>(null);
  const pendingAppearancePatchRef = React.useRef<AppearancePreferencePatch>({});

  React.useEffect(() => {
    if (userStatus === "loading") {
      setLoading(true);
      return;
    }

    if (!user) {
      setViewer(null);
      setLoading(false);
      return;
    }

    const nextDraft = createDraftFromUser(user);
    setViewer(user);
    setDraft(nextDraft);
    setInitialDraft(nextDraft);
    setUsernameDraft(user.username);
    setLoading(false);
  }, [user, userStatus]);

  React.useEffect(() => {
    setThemeRuntimeReady(true);
    setNotificationRuntimeReady(true);
    setNotificationSupported(isBrowserNotificationSupported());
    setResponseCompletionNotificationsEnabled(readResponseCompletionNotificationsEnabled());
    setNotificationPermission(getBrowserNotificationPermission());
  }, []);

  React.useEffect(() => {
    return () => {
      if (appearanceSaveTimerRef.current !== null) {
        window.clearTimeout(appearanceSaveTimerRef.current);
      }
    };
  }, []);

  const viewerInitial = React.useMemo(() => {
    const source = draft.displayName || viewer?.username || "?";
    return source.trim().charAt(0).toUpperCase() || "?";
  }, [draft.displayName, viewer?.username]);

  const avatarSource = React.useMemo(
    () => ({
      publicID: viewer?.publicID,
      username: viewer?.username,
      displayName: draft.displayName || viewer?.displayName,
    }),
    [draft.displayName, viewer?.displayName, viewer?.publicID, viewer?.username],
  );
  const draftAvatarSrc = React.useMemo(
    () => resolveAvatarImageSrc(draft.avatarUrl, avatarSource),
    [avatarSource, draft.avatarUrl],
  );
  const avatarDialogPreviewSrc = React.useMemo(
    () => resolveAvatarImageSrc(avatarDialogValue, avatarSource),
    [avatarDialogValue, avatarSource],
  );
  const hasProfileEdits = !isProfileDraftEqual(draft, initialDraft);
  const canEditUsername = Boolean(viewer && !viewer.usernameChangedAt);
  const normalizedUsernameDraft = usernameDraft.trim().toLowerCase();
  const hasUsernameEdit = canEditUsername && normalizedUsernameDraft !== "" && normalizedUsernameDraft !== viewer?.username;
  const hasEdits = hasProfileEdits || hasUsernameEdit;
  const activeThemeMode = themeRuntimeReady
    ? ((theme as ThemeMode | undefined) ?? "system")
    : "system";
  const activeThemePreset = themeRuntimeReady ? preset : "default";
  const activeThemePresetPreview = React.useMemo(
    () => THEME_PRESET_PREVIEWS.find((item) => item.value === activeThemePreset) ?? THEME_PRESET_PREVIEWS[0],
    [activeThemePreset],
  );

  React.useEffect(() => {
    if (viewer?.initialUsernameRequired && !initialUsernameToastShownRef.current) {
      initialUsernameToastShownRef.current = true;
      toast.info(t("generalPage.toast.initialUsernameRequired"));
    }
  }, [t, viewer?.initialUsernameRequired]);

  const handleSave = React.useCallback(async () => {
    if (saving || !hasEdits) {
      return;
    }

    try {
      if (hasUsernameEdit && !isUsernamePolicyValid(normalizedUsernameDraft)) {
        toast.error(t("generalPage.toast.setUsernameFailed"), {
          description: t("generalPage.username.invalid"),
        });
        return;
      }
      if (hasProfileEdits && !isDisplayNameLengthValid(draft.displayName)) {
        toast.error(t("generalPage.toast.saveProfileFailed"), {
          description: t("generalPage.profile.displayNameInvalid"),
        });
        return;
      }

      setSaving(true);

      let nextViewer = viewer;
      if (hasUsernameEdit) {
        try {
          nextViewer = await patchUsername(accessToken, { username: normalizedUsernameDraft });
        } catch (error) {
          toast.error(t("generalPage.toast.setUsernameFailed"), {
            description: resolveUsernameErrorMessage(error, {
              invalid: t("generalPage.username.invalid"),
              alreadyChanged: t("generalPage.username.alreadyChanged"),
              taken: t("generalPage.username.taken"),
            }),
          });
          return;
        }
      }

      if (hasProfileEdits) {
        nextViewer = await patchMe(accessToken, {
          avatarURL: draft.avatarUrl,
          displayName: draft.displayName,
          timezone: draft.timezone,
          locale: draft.locale,
          profilePreferences: draft.profilePreferences,
        });
      }

      if (!nextViewer) {
        return;
      }

      const nextDraft = createDraftFromUser(nextViewer);
      setViewer(nextViewer);
      setDraft(nextDraft);
      setInitialDraft(nextDraft);
      setUsernameDraft(nextViewer.username);
      dispatchUserProfileUpdated(nextViewer);
      toast.success(
        hasUsernameEdit && !hasProfileEdits
          ? t("generalPage.toast.usernameUpdated")
          : t("generalPage.toast.profileUpdated"),
      );
    } catch (error) {
      toast.error(t("generalPage.toast.saveProfileFailed"), {
        description: resolveSettingsErrorMessage(error),
      });
    } finally {
      setSaving(false);
    }
  }, [accessToken, draft, hasEdits, hasProfileEdits, hasUsernameEdit, normalizedUsernameDraft, saving, t, viewer]);

  const handleDiscard = React.useCallback(() => {
    setDraft(initialDraft);
    setUsernameDraft(viewer?.username ?? "");
  }, [initialDraft, viewer?.username]);

  const handleOpenAvatarDialog = React.useCallback(() => {
    setAvatarDialogValue(draft.avatarUrl.trim());
    setAvatarDialogOpen(true);
  }, [draft.avatarUrl]);

  const handleSaveAvatarDialog = React.useCallback(() => {
    setDraft((current) => ({ ...current, avatarUrl: avatarDialogValue.trim() }));
    setAvatarDialogOpen(false);
  }, [avatarDialogValue]);

  const handleCycleGeneratedAvatar = React.useCallback(() => {
    setAvatarDialogValue(createGeneratedGithubAvatarRef(generateAvatarVariant()));
  }, []);

  const handleResponseCompletionNotificationsChange = React.useCallback((checked: boolean) => {
    if (!notificationSupported) {
      return;
    }

    if (!checked) {
      disableResponseCompletionNotifications();
      setResponseCompletionNotificationsEnabled(false);
      setNotificationPermission(getBrowserNotificationPermission());
      return;
    }

    void (async () => {
      const result = await enableResponseCompletionNotifications();
      setResponseCompletionNotificationsEnabled(result.enabled);
      setNotificationPermission(result.permission);

      if (result.permission === "unsupported") {
        toast.error(t("generalPage.notifications.unsupportedTitle"), {
          description: t("generalPage.notifications.unsupportedDescription"),
        });
        return;
      }

      if (result.permission === "denied") {
        toast.error(t("generalPage.notifications.deniedTitle"), {
          description: t("generalPage.notifications.deniedDescription"),
        });
        return;
      }

      if (result.enabled) {
        toast.success(t("generalPage.notifications.enabledTitle"), {
          description: t("generalPage.notifications.enabledDescription"),
        });
      }
    })();
  }, [notificationSupported, t]);

  const persistAppearancePreferences = React.useCallback(
    (patch: AppearancePreferencePatch) => {
      if (!accessToken) {
        return;
      }

      pendingAppearancePatchRef.current = {
        ...pendingAppearancePatchRef.current,
        ...patch,
      };
      if (appearanceSaveTimerRef.current !== null) {
        window.clearTimeout(appearanceSaveTimerRef.current);
      }

      appearanceSaveTimerRef.current = window.setTimeout(() => {
        void (async () => {
          const pendingPatch = pendingAppearancePatchRef.current;
          pendingAppearancePatchRef.current = {};
          appearanceSaveTimerRef.current = null;
          const appearancePreferences = serializeAppearancePreferences({
            ...readLocalAppearancePreferences(),
            ...pendingPatch,
          });
          try {
            const nextViewer = await patchMe(accessToken, { appearancePreferences });
            setViewer((current) =>
              current ? { ...current, appearancePreferences: nextViewer.appearancePreferences } : nextViewer,
            );
          } catch (error) {
            toast.error(t("generalPage.toast.saveProfileFailed"), {
              description: resolveSettingsErrorMessage(error),
            });
          }
        })();
      }, 300);
    },
    [accessToken, t],
  );

  const notificationHelpText = React.useMemo(() => {
    if (!notificationRuntimeReady) {
      return t("generalPage.notifications.defaultHelp");
    }
    if (!notificationSupported) {
      return t("generalPage.notifications.unsupportedHelp");
    }
    if (notificationPermission === "denied") {
      return t("generalPage.notifications.deniedHelp");
    }
    if (notificationPermission === "granted") {
      return t("generalPage.notifications.grantedHelp");
    }
    return t("generalPage.notifications.defaultHelp");
  }, [notificationPermission, notificationRuntimeReady, notificationSupported, t]);

  const handleThemeModeChange = React.useCallback(
    (mode: ThemeMode) => {
      setTheme(mode);
      persistAppearancePreferences({ theme: mode });
    },
    [persistAppearancePreferences, setTheme],
  );

  const handleThemePresetChange = React.useCallback(
    (nextPreset: ThemePresetPreview["value"]) => {
      setPreset(nextPreset);
      persistAppearancePreferences({ preset: nextPreset });
    },
    [persistAppearancePreferences, setPreset],
  );

  const handleChatFontChange = React.useCallback((value: ChatFontOption) => {
    writeChatFontPreference(value);
    persistAppearancePreferences({ chatFont: value });
  }, [persistAppearancePreferences]);

  const handleChatFontWeightChange = React.useCallback((value: ChatFontWeightOption) => {
    writeChatFontWeightPreference(value);
    persistAppearancePreferences({ chatFontWeight: value });
  }, [persistAppearancePreferences]);

  return (
    <SettingsPage>
      <SettingsSection
        title={t("profile")}
        actions={
          hasEdits ? (
            <>
              <Button type="button" variant="ghost" size="sm" disabled={saving} onClick={handleDiscard}>
                {common("actions.reset")}
              </Button>
              <Button type="button" size="sm" disabled={saving} onClick={() => void handleSave()}>
                {saving ? <SpinnerLabel>{common("actions.saving")}</SpinnerLabel> : common("actions.save")}
              </Button>
            </>
          ) : null
        }
      >
        <FieldGroup className="gap-3 md:gap-4">
          <div className="grid gap-3 md:gap-4 xl:grid-cols-[minmax(0,132px)_minmax(0,1fr)_minmax(0,1fr)]">
            <Field>
              <FieldLabel>{t("generalPage.profile.avatar")}</FieldLabel>
              <div className="flex items-center">
                <button
                  type="button"
                  className="rounded-full transition-opacity hover:opacity-85 focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring"
                  onClick={handleOpenAvatarDialog}
                  disabled={loading || saving}
                >
                  <Avatar className="size-9 bg-muted">
                    <AvatarImage src={draftAvatarSrc || undefined} alt={draft.displayName || viewer?.username || t("generalPage.profile.avatarAlt")} />
                    <AvatarFallback className="bg-foreground text-sm font-medium text-background">
                      {viewerInitial}
                    </AvatarFallback>
                  </Avatar>
                </button>
              </div>
            </Field>

            <Field>
              <FieldLabel>{t("generalPage.profile.username")}</FieldLabel>
              <div className="space-y-1.5">
                <Input
                  value={usernameDraft}
                  onChange={(event) => setUsernameDraft(event.target.value.toLowerCase())}
                  readOnly={!canEditUsername}
                  disabled={loading || saving || !canEditUsername}
                  maxLength={USERNAME_MAX_LENGTH}
                  placeholder={t("generalPage.profile.usernamePlaceholder")}
                />
              </div>
            </Field>

            <Field>
              <FieldLabel>{t("generalPage.profile.displayName")}</FieldLabel>
              <Input
                value={draft.displayName}
                onChange={(event) => setDraft((current) => ({ ...current, displayName: event.target.value }))}
                placeholder={t("generalPage.profile.displayNamePlaceholder")}
                disabled={loading || saving}
                maxLength={DISPLAY_NAME_MAX_LENGTH}
              />
            </Field>
          </div>

          <Field>
            <FieldLabel>{t("generalPage.profile.timezone")}</FieldLabel>
            <TimeZoneSelect
              id="settings-timezone"
              value={draft.timezone}
              disabled={loading || saving}
              onChange={(value) => setDraft((current) => ({ ...current, timezone: value }))}
            />
          </Field>

          <Field>
            <FieldLabel>{t("generalPage.profile.conversationPreferences")}</FieldLabel>
            <Textarea
              maxLength={1024}
              value={draft.profilePreferences}
              onChange={(event) =>
                setDraft((current) => ({ ...current, profilePreferences: event.target.value }))
              }
              placeholder={t("generalPage.profile.conversationPreferencesPlaceholder")}
              className="h-24 resize-none overflow-y-auto [field-sizing:fixed]"
              disabled={loading || saving}
            />
          </Field>
        </FieldGroup>
      </SettingsSection>

      <SettingsSectionSeparator />

      <SettingsSection title={t("notifications")}>
        <SettingsFieldRow
          title={t("generalPage.notifications.responseCompletionTitle")}
          description={
            <>
              {t("generalPage.notifications.responseCompletionDescription")}
              <br />
              {notificationHelpText}
            </>
          }
          controlClassName="sm:w-auto md:w-auto"
        >
          <Switch
            checked={responseCompletionNotificationsEnabled}
            onCheckedChange={handleResponseCompletionNotificationsChange}
            aria-label={t("generalPage.notifications.toggleResponseCompletion")}
            disabled={!notificationRuntimeReady || !notificationSupported}
            className="shrink-0"
          />
        </SettingsFieldRow>
      </SettingsSection>

      <SettingsSectionSeparator />

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
                  onSelect={handleThemePresetChange}
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
                onSelect={handleThemeModeChange}
              />
              <ThemePreviewCard
                label={t("generalPage.appearance.theme.system")}
                mode="system"
                lightPalette={activeThemePresetPreview.light}
                darkPalette={activeThemePresetPreview.dark}
                active={activeThemeMode === "system"}
                onSelect={handleThemeModeChange}
              />
              <ThemePreviewCard
                label={t("generalPage.appearance.theme.dark")}
                mode="dark"
                lightPalette={activeThemePresetPreview.light}
                darkPalette={activeThemePresetPreview.dark}
                active={activeThemeMode === "dark"}
                onSelect={handleThemeModeChange}
              />
            </div>
          </Field>

          <Field>
            <FieldLabel>{t("generalPage.appearance.chatFont")}</FieldLabel>
            <div className="grid grid-cols-2 gap-2 sm:grid-cols-4 md:gap-3 xl:gap-4">
              {CHAT_FONT_OPTIONS.map((item) => (
                <ChatFontPreviewCard
                  key={item.value}
                  item={{ ...item, label: t(`generalPage.appearance.font.${item.value}`) }}
                  active={chatFont === item.value}
                  onSelect={handleChatFontChange}
                />
              ))}
            </div>
          </Field>

          <Field>
            <FieldLabel>{t("generalPage.appearance.chatFontWeight")}</FieldLabel>
            <div className="grid grid-cols-2 gap-2 sm:grid-cols-4 md:gap-3 xl:gap-4">
              {CHAT_FONT_WEIGHT_OPTIONS.map((item) => (
                <ChatFontWeightPreviewCard
                  key={item.value}
                  item={{ ...item, label: t(`generalPage.appearance.fontWeight.${item.value}`) }}
                  active={chatFontWeight === item.value}
                  onSelect={handleChatFontWeightChange}
                />
              ))}
            </div>
          </Field>
        </FieldGroup>
      </SettingsSection>

      <Dialog open={avatarDialogOpen} onOpenChange={setAvatarDialogOpen}>
        <DialogContent>
          <DialogHeader>
            <DialogTitle>{t("generalPage.avatarDialog.title")}</DialogTitle>
            <DialogDescription>{t("generalPage.avatarDialog.description")}</DialogDescription>
          </DialogHeader>

          <div className="space-y-4">
            <div className="flex justify-center">
              <button
                type="button"
                className="rounded-2xl transition-transform hover:scale-[1.03] focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring"
                onClick={handleCycleGeneratedAvatar}
              >
                <Avatar className="size-16 bg-pure">
                  <AvatarImage src={avatarDialogPreviewSrc || undefined} alt={draft.displayName || viewer?.username || t("generalPage.profile.avatarAlt")} />
                  <AvatarFallback className="bg-foreground text-3xl font-medium text-background">
                    {viewerInitial}
                  </AvatarFallback>
                </Avatar>
              </button>
            </div>

            <Field>
              <FieldLabel>{t("generalPage.avatarDialog.avatarURL")}</FieldLabel>
              <Input
                value={avatarDialogValue}
                onChange={(event) => setAvatarDialogValue(event.target.value)}
                placeholder="https://example.com/avatar.png"
                disabled={saving}
              />
            </Field>
          </div>

          <DialogFooter>
            <Button type="button" variant="ghost" onClick={() => setAvatarDialogOpen(false)}>
              {common("actions.cancel")}
            </Button>
            <Button type="button" onClick={handleSaveAvatarDialog}>
              {t("generalPage.avatarDialog.apply")}
            </Button>
          </DialogFooter>
        </DialogContent>
      </Dialog>
    </SettingsPage>
  );
}
