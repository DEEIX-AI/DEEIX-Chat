"use client";

import * as React from "react";
import { useTranslations } from "next-intl";
import { toast } from "sonner";

import { useSettingsAppearancePersistence } from "@/features/settings/hooks/use-settings-appearance-persistence";
import { useSettingsGeneralProfile } from "@/features/settings/hooks/use-settings-general-profile";
import {
  type FontSizeOption,
  useFontSizePreference,
  writeFontSizePreference,
} from "@/features/settings/utils/font-size";
import { type ToastPosition, useToastPosition, writeToastPosition } from "@/shared/lib/toast-position";
import type { ThemeMode } from "@/features/settings/types/settings";
import {
  disableResponseCompletionNotifications,
  enableResponseCompletionNotifications,
  getBrowserNotificationPermission,
  isBrowserNotificationSupported,
  readResponseCompletionNotificationsEnabled,
} from "@/shared/lib/browser-notifications";
import {
  SettingsPage,
  SettingsSectionSeparator,
} from "@/shared/components/settings-layout";
import { useTheme } from "@/shared/components/theme-provider";
import { GeneralAppearanceSection } from "./general-appearance";
import { GeneralNotificationsSection } from "./general-notifications";
import { GeneralProfileSection } from "./general-profile";

export function SettingsGeneral() {
  const t = useTranslations("settings");
  const { preset, resolvedTheme, setPreset, setTheme, theme } = useTheme();
  const {
    viewer,
    draft,
    setDraft,
    loading,
    saving,
    hasEdits,
    canEditUsername,
    usernameDraft,
    setUsernameDraft,
    viewerInitial,
    draftAvatarSrc,
    avatarDialogOpen,
    setAvatarDialogOpen,
    avatarDialogValue,
    setAvatarDialogValue,
    avatarUploading,
    avatarDialogPreviewSrc,
    handleSave,
    handleDiscard,
    handleOpenAvatarDialog,
    handleSaveAvatarDialog,
    handleCycleGeneratedAvatar,
    handleUploadAvatarFile,
  } = useSettingsGeneralProfile();
  const [themeRuntimeReady, setThemeRuntimeReady] = React.useState(false);
  const fontSize = useFontSizePreference();
  const toastPosition = useToastPosition();
  const [notificationRuntimeReady, setNotificationRuntimeReady] = React.useState(false);
  const [notificationSupported, setNotificationSupported] = React.useState(false);
  const [responseCompletionNotificationsEnabled, setResponseCompletionNotificationsEnabled] = React.useState(false);
  const [notificationPermission, setNotificationPermission] = React.useState<NotificationPermission | "unsupported">("unsupported");
  const persistAppearancePreferences = useSettingsAppearancePersistence();

  React.useEffect(() => {
    setThemeRuntimeReady(true);
    setNotificationRuntimeReady(true);
    setNotificationSupported(isBrowserNotificationSupported());
    setResponseCompletionNotificationsEnabled(readResponseCompletionNotificationsEnabled());
    setNotificationPermission(getBrowserNotificationPermission());
  }, []);

  const activeThemeMode: ThemeMode = themeRuntimeReady ? theme : "system";
  const activeThemePreset = themeRuntimeReady ? preset : "default";

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
    },
    [setTheme],
  );

  const handleThemePresetChange = React.useCallback(
    (nextPreset: typeof preset) => {
      setPreset(nextPreset);
      persistAppearancePreferences({ preset: nextPreset });
    },
    [persistAppearancePreferences, setPreset],
  );

  const handleFontSizeChange = React.useCallback((value: FontSizeOption) => {
    writeFontSizePreference(value);
    persistAppearancePreferences({ fontSize: value });
  }, [persistAppearancePreferences]);
  // Device-local like the color mode: screen size and keyboard behavior, not
  // the account, decide where a toast is out of the way.
  const handleToastPositionChange = React.useCallback((value: ToastPosition) => {
    writeToastPosition(value);
    // Show the result where it now lives, so the choice is immediately legible.
    toast.success(t("generalPage.appearance.toastPositionPreview"), { id: "toast-position-preview" });
  }, [t]);

  return (
    <SettingsPage>
      <GeneralProfileSection
        viewer={viewer}
        draft={draft}
        loading={loading}
        saving={saving}
        hasEdits={hasEdits}
        canEditUsername={canEditUsername}
        usernameDraft={usernameDraft}
        viewerInitial={viewerInitial}
        draftAvatarSrc={draftAvatarSrc}
        avatarDialogOpen={avatarDialogOpen}
        avatarDialogValue={avatarDialogValue}
        avatarUploading={avatarUploading}
        avatarDialogPreviewSrc={avatarDialogPreviewSrc}
        onDraftChange={setDraft}
        onUsernameDraftChange={setUsernameDraft}
        onReset={handleDiscard}
        onSave={() => void handleSave()}
        onOpenAvatarDialog={handleOpenAvatarDialog}
        onAvatarDialogOpenChange={setAvatarDialogOpen}
        onAvatarDialogValueChange={setAvatarDialogValue}
        onCycleGeneratedAvatar={handleCycleGeneratedAvatar}
        onUploadAvatarFile={(file) => void handleUploadAvatarFile(file)}
        onSaveAvatarDialog={handleSaveAvatarDialog}
      />

      <SettingsSectionSeparator />

      <GeneralNotificationsSection
        notificationRuntimeReady={notificationRuntimeReady}
        notificationSupported={notificationSupported}
        responseCompletionNotificationsEnabled={responseCompletionNotificationsEnabled}
        notificationHelpText={notificationHelpText}
        onResponseCompletionNotificationsChange={handleResponseCompletionNotificationsChange}
      />

      <SettingsSectionSeparator />

      <GeneralAppearanceSection
        resolvedTheme={resolvedTheme}
        activeThemeMode={activeThemeMode}
        activeThemePreset={activeThemePreset}
        fontSize={fontSize}
        toastPosition={toastPosition}
        onThemeModeChange={handleThemeModeChange}
        onThemePresetChange={handleThemePresetChange}
        onFontSizeChange={handleFontSizeChange}
        onToastPositionChange={handleToastPositionChange}
      />
    </SettingsPage>
  );
}
