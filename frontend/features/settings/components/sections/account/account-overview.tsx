"use client";

import * as React from "react";
import { useTranslations } from "next-intl";

import { Button } from "@/components/ui/button";
import { SpinnerLabel } from "@/components/ui/spinner";
import { resolveEmailTitle, resolveEmailValue } from "@/features/settings/model/account-page";
import type { UserDTO } from "@/shared/api/auth.types";
import { CopyActionButton } from "@/shared/components/copy-action";
import { SettingsSection } from "@/shared/components/settings-layout";

function ActionRow({
  title,
  value,
  action,
}: {
  title: string;
  value?: string;
  action: React.ReactNode;
}) {
  return (
    <div className="flex items-center justify-between gap-4">
      <div className="flex min-w-0 flex-1 items-baseline gap-2">
        <p className="shrink-0 text-xs font-medium">{title}</p>
        {value ? <p className="max-w-[min(60vw,24rem)] truncate text-xs text-muted-foreground">{value}</p> : null}
      </div>
      <div className="flex shrink-0 justify-end">{action}</div>
    </div>
  );
}

function ValueRow({
  title,
  value,
  action,
}: {
  title: string;
  value: string;
  action?: React.ReactNode;
}) {
  return (
    <div className="flex items-center justify-between gap-4">
      <p className="min-w-0 flex-1 text-xs font-medium">{title}</p>
      <div className="flex min-w-0 max-w-[min(60vw,26rem)] shrink items-center gap-2 rounded-lg bg-muted/35 px-2 py-1 text-xs text-muted-foreground">
        <span className="max-w-[min(75vw,26rem)] truncate">{value}</span>
        {action}
      </div>
    </div>
  );
}

export function AccountOverviewSection({
  viewer,
  loading,
  loggingOut,
  deletingAccount,
  changingPassword,
  twoFactorAvailable,
  twoFactorEnabled,
  twoFactorOpening,
  emailVerificationEnabled,
  canVerifyCurrentEmail,
  emailActionLabel,
  onOpenCurrentEmailVerification,
  onOpenEmailDialog,
  onOpenPasswordDialog,
  onOpenTwoFactorDialog,
  onStartTwoFactorSetup,
  onLogoutAll,
  onOpenDeleteDialog,
}: {
  viewer: UserDTO | null;
  loading: boolean;
  loggingOut: boolean;
  deletingAccount: boolean;
  changingPassword: boolean;
  twoFactorAvailable: boolean;
  twoFactorEnabled: boolean;
  twoFactorOpening: boolean;
  emailVerificationEnabled: boolean;
  canVerifyCurrentEmail: boolean;
  emailActionLabel: string;
  onOpenCurrentEmailVerification: () => void;
  onOpenEmailDialog: () => void;
  onOpenPasswordDialog: () => void;
  onOpenTwoFactorDialog: () => void;
  onStartTwoFactorSetup: () => void;
  onLogoutAll: () => void;
  onOpenDeleteDialog: () => void;
}) {
  const t = useTranslations("settings.accountPage");

  return (
    <SettingsSection title={t("title")}>
      <ActionRow
        title={resolveEmailTitle(viewer, t)}
        value={resolveEmailValue(viewer, emailVerificationEnabled, t)}
        action={
          <div className="flex items-center gap-2">
            {canVerifyCurrentEmail ? (
              <Button
                type="button"
                variant="outline"
                disabled={loading || changingPassword}
                onClick={onOpenCurrentEmailVerification}
              >
                {t("actions.verify")}
              </Button>
            ) : null}
            <Button
              type="button"
              variant="outline"
              disabled={loading || changingPassword}
              onClick={onOpenEmailDialog}
            >
              {emailActionLabel}
            </Button>
          </div>
        }
      />

      <ActionRow
        title={t("password")}
        action={
          <Button
            type="button"
            variant="outline"
            disabled={loading || changingPassword}
            onClick={onOpenPasswordDialog}
          >
            {viewer?.passwordEnabled ? t("actions.update") : t("actions.set")}
          </Button>
        }
      />

      {twoFactorAvailable ? (
        <ActionRow
          title={t("twoFactor")}
          action={
            <Button
              type="button"
              variant="outline"
              disabled={loading || twoFactorOpening}
              onClick={twoFactorEnabled ? onOpenTwoFactorDialog : onStartTwoFactorSetup}
            >
              {twoFactorOpening ? <SpinnerLabel>{t("actions.generating")}</SpinnerLabel> : twoFactorEnabled ? t("actions.manage") : t("actions.set")}
            </Button>
          }
        />
      ) : null}

      <ActionRow
        title={t("logoutAllDevices")}
        action={
          <Button
            type="button"
            variant="outline"
            disabled={loading || loggingOut}
            onClick={onLogoutAll}
          >
            {loggingOut ? <SpinnerLabel>{t("actions.loggingOut")}</SpinnerLabel> : t("actions.logOut")}
          </Button>
        }
      />

      <ActionRow
        title={t("deleteAccount")}
        action={
          <Button
            type="button"
            variant="destructive"
            disabled={loading || deletingAccount}
            onClick={onOpenDeleteDialog}
          >
            {deletingAccount ? <SpinnerLabel>{t("actions.deleting")}</SpinnerLabel> : t("actions.deleteAccount")}
          </Button>
        }
      />

      <ValueRow
        title={t("publicID")}
        value={viewer?.publicID || "-"}
        action={
          <CopyActionButton
            type="button"
            variant="ghost"
            size="icon"
            value={viewer?.publicID || ""}
            messages={{
              copied: t("toasts.publicIDCopied"),
              failed: t("toasts.copyFailed"),
              failedDescription: t("toasts.retryLater"),
            }}
            disabled={!viewer?.publicID}
            aria-label={t("copyPublicID")}
            className="size-4 p-3"
          />
        }
      />
    </SettingsSection>
  );
}
