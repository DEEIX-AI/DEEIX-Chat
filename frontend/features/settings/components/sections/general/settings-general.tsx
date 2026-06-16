"use client";

import * as React from "react";
import { useTranslations } from "next-intl";
import { toast } from "sonner";

import { dispatchUserProfileUpdated } from "@/features/settings/events/user-profile-events";
import { useAppearancePreferencesPersistence } from "@/features/settings/hooks/use-appearance-preferences-persistence";
import {
  type FontSizeOption,
  useFontSizePreference,
  writeFontSizePreference,
} from "@/features/settings/utils/font-size";
import type { ProfileDraft, ThemeMode } from "@/features/settings/types/settings";
import {
  createDraftFromUser,
  isProfileDraftEqual,
} from "@/features/settings/utils/profile-settings";
import { resolveLocalizedErrorMessage } from "@/i18n/resolve-error-message";
import {
  createFileAvatarRef,
  createGeneratedGithubAvatarRef,
  generateAvatarVariant,
  parseFileAvatarID,
  resolveAvatarImageSrc,
} from "@/shared/lib/avatar";
import { useAuthSession } from "@/shared/auth/auth-session-context";
import {
  disableResponseCompletionNotifications,
  enableResponseCompletionNotifications,
  getBrowserNotificationPermission,
  isBrowserNotificationSupported,
  readResponseCompletionNotificationsEnabled,
} from "@/shared/lib/browser-notifications";
import { patchMe, patchUsername } from "@/shared/api/auth";
import { uploadFile } from "@/shared/api/file";
import { ApiError } from "@/shared/api/http-client";
import {
  isDisplayNameLengthValid,
  isUsernamePolicyValid,
} from "@/shared/auth/account-policy";
import type { UserDTO } from "@/shared/api/auth.types";
import {
  SettingsPage,
  SettingsSectionSeparator,
} from "@/shared/components/settings-layout";
import { useTheme } from "@/shared/components/theme-provider";
import { GeneralAppearanceSection } from "./general-appearance";
import { GeneralNotificationsSection } from "./general-notifications";
import { GeneralProfileSection } from "./general-profile";

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
  return resolveLocalizedErrorMessage(error);
}

type AvatarUploadPreview = {
  fileID: string;
  url: string;
};

export function SettingsGeneral() {
  const t = useTranslations("settings");
  const { accessToken, user, userStatus } = useAuthSession();
  const { preset, resolvedTheme, setPreset, setTheme, theme } = useTheme();
  const [viewer, setViewer] = React.useState<UserDTO | null>(null);
  const [draft, setDraft] = React.useState<ProfileDraft>(() => createDraftFromUser());
  const [initialDraft, setInitialDraft] = React.useState<ProfileDraft>(() => createDraftFromUser());
  const [avatarDialogOpen, setAvatarDialogOpen] = React.useState(false);
  const [avatarDialogValue, setAvatarDialogValue] = React.useState("");
  const [avatarUploading, setAvatarUploading] = React.useState(false);
  const [avatarUploadPreview, setAvatarUploadPreview] = React.useState<AvatarUploadPreview | null>(null);
  const [themeRuntimeReady, setThemeRuntimeReady] = React.useState(false);
  const fontSize = useFontSizePreference();
  const [notificationRuntimeReady, setNotificationRuntimeReady] = React.useState(false);
  const [notificationSupported, setNotificationSupported] = React.useState(false);
  const [responseCompletionNotificationsEnabled, setResponseCompletionNotificationsEnabled] = React.useState(false);
  const [notificationPermission, setNotificationPermission] = React.useState<NotificationPermission | "unsupported">("unsupported");
  const [loading, setLoading] = React.useState(true);
  const [saving, setSaving] = React.useState(false);
  const [usernameDraft, setUsernameDraft] = React.useState("");
  const initialUsernameToastShownRef = React.useRef(false);
  const persistAppearancePreferences = useAppearancePreferencesPersistence();

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
      if (avatarUploadPreview) {
        URL.revokeObjectURL(avatarUploadPreview.url);
      }
    };
  }, [avatarUploadPreview]);

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
  const resolveAvatarPreviewSrc = React.useCallback(
    (value: string) => {
      const fileID = parseFileAvatarID(value.trim());
      if (fileID && avatarUploadPreview?.fileID === fileID) {
        return avatarUploadPreview.url;
      }
      return resolveAvatarImageSrc(value, avatarSource);
    },
    [avatarSource, avatarUploadPreview],
  );
  const draftAvatarSrc = React.useMemo(
    () => resolveAvatarPreviewSrc(draft.avatarUrl),
    [draft.avatarUrl, resolveAvatarPreviewSrc],
  );
  const avatarDialogPreviewSrc = React.useMemo(
    () => resolveAvatarPreviewSrc(avatarDialogValue),
    [avatarDialogValue, resolveAvatarPreviewSrc],
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
      setAvatarUploadPreview((current) => {
        const savedFileID = parseFileAvatarID(nextDraft.avatarUrl);
        if (current && current.fileID === savedFileID) {
          return null;
        }
        return current;
      });
      dispatchUserProfileUpdated(nextViewer);
      toast.success(
        hasUsernameEdit && !hasProfileEdits
          ? t("generalPage.toast.usernameUpdated")
          : t("generalPage.toast.profileUpdated"),
      );
    } catch (error) {
      toast.error(t("generalPage.toast.saveProfileFailed"), {
        description: resolveLocalizedErrorMessage(error),
      });
    } finally {
      setSaving(false);
    }
  }, [accessToken, draft, hasEdits, hasProfileEdits, hasUsernameEdit, normalizedUsernameDraft, saving, t, viewer]);

  const handleDiscard = React.useCallback(() => {
    setDraft(initialDraft);
    setUsernameDraft(viewer?.username ?? "");
    setAvatarUploadPreview((current) => {
      const initialFileID = parseFileAvatarID(initialDraft.avatarUrl);
      if (current && current.fileID !== initialFileID) {
        return null;
      }
      return current;
    });
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

  const handleUploadAvatarFile = React.useCallback(async (file: File) => {
    if (avatarUploading) {
      return;
    }
    if (!file.type.toLowerCase().startsWith("image/")) {
      toast.error(t("generalPage.avatarDialog.uploadInvalid"));
      return;
    }

    let previewURL: string | null = null;
    try {
      setAvatarUploading(true);
      previewURL = URL.createObjectURL(file);
      const result = await uploadFile(accessToken, file, { purpose: "avatar" });
      const nextPreviewURL = previewURL;
      previewURL = null;
      setAvatarUploadPreview({
        fileID: result.file.fileID,
        url: nextPreviewURL,
      });
      setAvatarDialogValue(createFileAvatarRef(result.file.fileID));
      toast.success(t("generalPage.avatarDialog.uploaded"));
    } catch (error) {
      if (previewURL) {
        URL.revokeObjectURL(previewURL);
      }
      toast.error(t("generalPage.avatarDialog.uploadFailed"), {
        description: resolveLocalizedErrorMessage(error),
      });
    } finally {
      setAvatarUploading(false);
    }
  }, [accessToken, avatarUploading, t]);

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
      persistAppearancePreferences({ theme: mode });
    },
    [persistAppearancePreferences, setTheme],
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
        onThemeModeChange={handleThemeModeChange}
        onThemePresetChange={handleThemePresetChange}
        onFontSizeChange={handleFontSizeChange}
      />
    </SettingsPage>
  );
}
