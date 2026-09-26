"use client";

import * as React from "react";
import { useTranslations } from "next-intl";
import { Sparkles, Upload } from "lucide-react";

import { Avatar, AvatarFallback, AvatarImage } from "@/components/ui/avatar";
import { Button } from "@/components/ui/button";
import { Dialog, DialogContent, DialogDescription, DialogFooter, DialogHeader, DialogTitle } from "@/components/ui/dialog";
import { Field, FieldGroup, FieldLabel } from "@/components/ui/field";
import { Input } from "@/components/ui/input";
import { SpinnerLabel } from "@/components/ui/spinner";
import { Textarea } from "@/components/ui/textarea";
import type { ProfileDraft } from "@/features/settings/types/settings";
import { DISPLAY_NAME_MAX_LENGTH, USERNAME_MAX_LENGTH } from "@/shared/auth/account-policy";
import type { UserDTO } from "@/shared/api/auth.types";
import { SettingsSection } from "@/shared/components/settings-layout";
import { TimeZoneSelect } from "@/shared/components/time-zone-select";

export function GeneralProfileSection({
  viewer,
  draft,
  loading,
  saving,
  hasEdits,
  canEditUsername,
  usernameDraft,
  viewerInitial,
  draftAvatarSrc,
  avatarDialogOpen,
  avatarDialogValue,
  avatarUploading,
  avatarDialogPreviewSrc,
  onDraftChange,
  onUsernameDraftChange,
  onReset,
  onSave,
  onOpenAvatarDialog,
  onAvatarDialogOpenChange,
  onAvatarDialogValueChange,
  onCycleGeneratedAvatar,
  onUploadAvatarFile,
  onSaveAvatarDialog,
}: {
  viewer: UserDTO | null;
  draft: ProfileDraft;
  loading: boolean;
  saving: boolean;
  hasEdits: boolean;
  canEditUsername: boolean;
  usernameDraft: string;
  viewerInitial: string;
  draftAvatarSrc: string;
  avatarDialogOpen: boolean;
  avatarDialogValue: string;
  avatarUploading: boolean;
  avatarDialogPreviewSrc: string;
  onDraftChange: React.Dispatch<React.SetStateAction<ProfileDraft>>;
  onUsernameDraftChange: (value: string) => void;
  onReset: () => void;
  onSave: () => void;
  onOpenAvatarDialog: () => void;
  onAvatarDialogOpenChange: (open: boolean) => void;
  onAvatarDialogValueChange: (value: string) => void;
  onCycleGeneratedAvatar: () => void;
  onUploadAvatarFile: (file: File) => void;
  onSaveAvatarDialog: () => void;
}) {
  const t = useTranslations("settings");
  const common = useTranslations("common");
  const avatarFileInputRef = React.useRef<HTMLInputElement | null>(null);

  return (
    <>
      <SettingsSection
        title={t("profile")}
        actions={
          hasEdits ? (
            <>
              <Button type="button" variant="ghost" size="sm" disabled={saving} onClick={onReset}>
                {common("actions.reset")}
              </Button>
              <Button type="button" size="sm" disabled={saving} onClick={onSave}>
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
                  onClick={onOpenAvatarDialog}
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
                  onChange={(event) => onUsernameDraftChange(event.target.value.toLowerCase())}
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
                onChange={(event) => onDraftChange((current) => ({ ...current, displayName: event.target.value }))}
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
              onChange={(value) => onDraftChange((current) => ({ ...current, timezone: value }))}
            />
          </Field>

          <Field>
            <FieldLabel>{t("generalPage.profile.conversationPreferences")}</FieldLabel>
            <Textarea
              maxLength={1024}
              value={draft.profilePreferences}
              onChange={(event) =>
                onDraftChange((current) => ({ ...current, profilePreferences: event.target.value }))
              }
              placeholder={t("generalPage.profile.conversationPreferencesPlaceholder")}
              className="h-24 resize-none overflow-y-auto [field-sizing:fixed]"
              disabled={loading || saving}
            />
          </Field>
        </FieldGroup>
      </SettingsSection>

      <Dialog open={avatarDialogOpen} onOpenChange={onAvatarDialogOpenChange}>
        <DialogContent>
          <DialogHeader>
            <DialogTitle>{t("generalPage.avatarDialog.title")}</DialogTitle>
            <DialogDescription>{t("generalPage.avatarDialog.description")}</DialogDescription>
          </DialogHeader>

          <div className="space-y-4">
            <div className="flex justify-center">
              <Avatar className="size-16 bg-pure">
                <AvatarImage
                  src={avatarDialogPreviewSrc || undefined}
                  alt={draft.displayName || viewer?.username || t("generalPage.profile.avatarAlt")}
                />
                <AvatarFallback className="bg-foreground text-3xl font-medium text-background">
                  {viewerInitial}
                </AvatarFallback>
              </Avatar>
            </div>

            <div className="flex justify-center gap-1.5">
              <input
                ref={avatarFileInputRef}
                type="file"
                accept="image/*"
                className="hidden"
                onChange={(event) => {
                  const file = event.target.files?.[0];
                  event.currentTarget.value = "";
                  if (file) {
                    onUploadAvatarFile(file);
                  }
                }}
              />
              <Button
                type="button"
                variant="ghost"
                size="sm"
                className="h-8 gap-1.5 rounded-md bg-muted/60 px-3 text-xs font-medium text-foreground shadow-none hover:bg-muted"
                disabled={saving || avatarUploading}
                onClick={() => avatarFileInputRef.current?.click()}
              >
                {avatarUploading ? (
                  <SpinnerLabel>{t("generalPage.avatarDialog.uploading")}</SpinnerLabel>
                ) : (
                  <>
                    <Upload className="size-3.5 stroke-1" />
                    {t("generalPage.avatarDialog.upload")}
                  </>
                )}
              </Button>
              <Button
                type="button"
                variant="ghost"
                size="sm"
                className="h-8 gap-1.5 rounded-md bg-muted/60 px-3 text-xs font-medium text-foreground shadow-none hover:bg-muted"
                disabled={saving || avatarUploading}
                onClick={onCycleGeneratedAvatar}
              >
                <Sparkles className="size-3.5 stroke-1" />
                {t("generalPage.avatarDialog.generate")}
              </Button>
            </div>

            <Field>
              <FieldLabel>{t("generalPage.avatarDialog.avatarURL")}</FieldLabel>
              <Input
                value={avatarDialogValue}
                onChange={(event) => onAvatarDialogValueChange(event.target.value)}
                placeholder="https://example.com/avatar.png"
                disabled={saving || avatarUploading}
              />
            </Field>
          </div>

          <DialogFooter>
            <Button
              type="button"
              variant="ghost"
              disabled={saving || avatarUploading}
              onClick={() => onAvatarDialogOpenChange(false)}
            >
              {common("actions.cancel")}
            </Button>
            <Button
              type="button"
              disabled={saving || avatarUploading}
              onClick={onSaveAvatarDialog}
            >
              {saving ? (
                <SpinnerLabel>{common("actions.saving")}</SpinnerLabel>
              ) : (
                t("generalPage.avatarDialog.apply")
              )}
            </Button>
          </DialogFooter>
        </DialogContent>
      </Dialog>
    </>
  );
}
