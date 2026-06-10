"use client";

import * as React from "react";
import { useTranslations } from "next-intl";
import { toast } from "sonner";

import { dispatchUserProfileUpdated } from "@/features/settings/events/user-profile-events";
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
import {
  type FontSizeOption,
  useFontSizePreference,
  writeFontSizePreference,
} from "@/features/settings/utils/font-size";
import type { ProfileDraft, ThemeMode } from "@/features/settings/types/settings";
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
  return resolveSettingsErrorMessage(error);
}

export function SettingsGeneral() {
  const t = useTranslations("settings");
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
  const fontSize = useFontSizePreference();
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
    (nextPreset: typeof preset) => {
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
        avatarDialogPreviewSrc={avatarDialogPreviewSrc}
        onDraftChange={setDraft}
        onUsernameDraftChange={setUsernameDraft}
        onReset={handleDiscard}
        onSave={() => void handleSave()}
        onOpenAvatarDialog={handleOpenAvatarDialog}
        onAvatarDialogOpenChange={setAvatarDialogOpen}
        onAvatarDialogValueChange={setAvatarDialogValue}
        onCycleGeneratedAvatar={handleCycleGeneratedAvatar}
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
        chatFont={chatFont}
        chatFontWeight={chatFontWeight}
        fontSize={fontSize}
        onThemeModeChange={handleThemeModeChange}
        onThemePresetChange={handleThemePresetChange}
        onChatFontChange={handleChatFontChange}
        onChatFontWeightChange={handleChatFontWeightChange}
        onFontSizeChange={handleFontSizeChange}
      />
    </SettingsPage>
  );
}
