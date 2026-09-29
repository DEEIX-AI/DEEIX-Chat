import { useTranslations } from "next-intl";
import * as React from "react";
import { toast } from "sonner";

import type { ProfileDraft } from "@/features/settings/types/settings";
import { createDraftFromUser, isProfileDraftEqual } from "@/features/settings/utils/profile-settings";
import { resolveLocalizedErrorMessage } from "@/i18n/resolve-error-message";
import { patchMe, patchUsername } from "@/shared/api/auth";
import type { UserDTO } from "@/shared/api/auth-types";
import { uploadFile } from "@/shared/api/file";
import { ApiError } from "@/shared/api/http-client";
import { isDisplayNameLengthValid, isUsernamePolicyValid } from "@/shared/auth/account-policy";
import { useAuthSession } from "@/shared/auth/auth-session-context";
import { dispatchUserProfileUpdated } from "@/shared/auth/user-profile-events";
import {
  createFileAvatarRef,
  createGeneratedGithubAvatarRef,
  generateAvatarVariant,
  parseFileAvatarID,
  resolveAvatarImageSrc,
} from "@/shared/lib/avatar";

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

/**
 * Profile editing on the general settings page: the draft vs. the last saved
 * state, the one-time username change, and the avatar dialog. A freshly
 * uploaded avatar is previewed from a local object URL until a saved profile
 * points at that file.
 */
export function useSettingsGeneralProfile() {
  const t = useTranslations("settings");
  const { accessToken, user, userStatus } = useAuthSession();
  const [viewer, setViewer] = React.useState<UserDTO | null>(null);
  const [draft, setDraft] = React.useState<ProfileDraft>(() => createDraftFromUser());
  const [initialDraft, setInitialDraft] = React.useState<ProfileDraft>(() => createDraftFromUser());
  const [avatarDialogOpen, setAvatarDialogOpen] = React.useState(false);
  const [avatarDialogValue, setAvatarDialogValue] = React.useState("");
  const [avatarUploading, setAvatarUploading] = React.useState(false);
  const [avatarUploadPreview, setAvatarUploadPreview] = React.useState<AvatarUploadPreview | null>(null);
  const [loading, setLoading] = React.useState(true);
  const [saving, setSaving] = React.useState(false);
  const [usernameDraft, setUsernameDraft] = React.useState("");
  const initialUsernameToastShownRef = React.useRef(false);

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

  const handleSaveAvatarDialog = React.useCallback(async () => {
    if (saving || avatarUploading) {
      return;
    }

    const nextAvatarURL = avatarDialogValue.trim();
    if (nextAvatarURL === initialDraft.avatarUrl) {
      setAvatarDialogOpen(false);
      return;
    }

    try {
      setSaving(true);
      const nextViewer = await patchMe(accessToken, { avatarURL: nextAvatarURL });
      const nextInitialDraft = createDraftFromUser(nextViewer);
      setViewer(nextViewer);
      setDraft((current) => ({ ...current, avatarUrl: nextInitialDraft.avatarUrl }));
      setInitialDraft(nextInitialDraft);
      setAvatarUploadPreview((current) => {
        const savedFileID = parseFileAvatarID(nextInitialDraft.avatarUrl);
        if (current && current.fileID === savedFileID) {
          return null;
        }
        return current;
      });
      dispatchUserProfileUpdated(nextViewer);
      setAvatarDialogOpen(false);
      toast.success(t("generalPage.toast.profileUpdated"));
    } catch (error) {
      toast.error(t("generalPage.toast.saveProfileFailed"), {
        description: resolveLocalizedErrorMessage(error),
      });
    } finally {
      setSaving(false);
    }
  }, [accessToken, avatarDialogValue, avatarUploading, initialDraft.avatarUrl, saving, t]);

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

  return {
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
  };
}
