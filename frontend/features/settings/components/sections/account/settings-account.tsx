"use client";

import * as React from "react";
import { useTranslations } from "next-intl";

import {
  AlertDialog,
  AlertDialogAction,
  AlertDialogCancel,
  AlertDialogContent,
  AlertDialogDescription,
  AlertDialogFooter,
  AlertDialogHeader,
  AlertDialogTitle,
} from "@/components/ui/alert-dialog";
import { SpinnerLabel } from "@/components/ui/spinner";
import {
  CurrentEmailVerificationDialog,
  EmailSecurityDialog,
} from "@/features/settings/components/sections/account/account-email-dialogs";
import { ChangePasswordDialog } from "@/features/settings/components/sections/account/account-password-dialog";
import { TwoFactorDialog } from "@/features/settings/components/sections/account/account-two-factor-dialog";
import { SecurityVerificationDialog } from "@/features/settings/components/sections/account/account-verification-dialog";
import { useSettingsAccount } from "@/features/settings/hooks/use-settings-account";
import { shouldUseEmailBootstrap } from "@/features/settings/model/account-page";
import type { SecurityVerificationMethod } from "@/shared/api/auth.types";
import {
  SettingsPage,
  SettingsSectionSeparator,
} from "@/shared/components/settings-layout";
import { AccountActiveSessionsSection } from "./account-active-sessions";
import { AccountIdentitiesSection } from "./account-identities";
import { AccountOverviewSection } from "./account-overview";

export function SettingsAccount() {
  const t = useTranslations("settings.accountPage");
  const {
    viewer,
    sessions,
    identities,
    identityProviders,
    twoFactorStatus,
    twoFactorSetup,
    twoFactorRecoveryCodes,
    loading,
    loggingOut,
    deletingAccount,
    changingPassword,
    sendingPasswordCode,
    sendingEmailCode,
    revokingSessionID,
    passwordDialogOpen,
    emailDialogOpen,
    currentEmailVerificationDialogOpen,
    deleteDialogOpen,
    deleteCodeDebug,
    deleteCodeCooldownSeconds,
    sendingDeleteCode,
    emailVerificationEnabled,
    passwordCodeDebug,
    emailCodeDebug,
    currentEmailCodeDebug,
    passwordCodeCooldownSeconds,
    emailCodeCooldownSeconds,
    currentEmailCodeCooldownSeconds,
    setPasswordDialogOpen,
    setEmailDialogOpen,
    setCurrentEmailVerificationDialogOpen,
    setDeleteDialogOpen,
    handleSendPasswordCode,
    handleChangePassword,
    handleSendEmailBootstrapCode,
    handleCompleteEmailBootstrap,
    handleSendCurrentEmailVerificationCode,
    handleCompleteCurrentEmailVerification,
    handleSendCurrentEmailCode,
    handleSendNewEmailCode,
    handleCompleteEmailChange,
    handleSendDeleteAccountCode,
    handleStartTwoFactorSetup,
    handleConfirmTwoFactorSetup,
    handleDisableTwoFactor,
    handleRegenerateTwoFactorRecoveryCodes,
    handleCancelTwoFactorSetup,
    clearTwoFactorRecoveryCodes,
    handleLogoutAll,
    handleDeleteAccount,
    handleLogoutSession,
    handleBindIdentity,
    handleDeleteIdentity,
  } = useSettingsAccount();
  const [twoFactorDialogOpen, setTwoFactorDialogOpen] = React.useState(false);
  const [twoFactorOpening, setTwoFactorOpening] = React.useState(false);
  const [deleteVerificationDialogOpen, setDeleteVerificationDialogOpen] = React.useState(false);
  const [deleteVerificationMethod, setDeleteVerificationMethod] = React.useState<SecurityVerificationMethod>("none");
  const emailBootstrapMode = shouldUseEmailBootstrap(viewer);
  const twoFactorEnabled = Boolean(twoFactorStatus?.totpEnabled);
  const securityVerificationMethods = React.useMemo<SecurityVerificationMethod[]>(() => {
    const methods: SecurityVerificationMethod[] = [];
    if (twoFactorEnabled) {
      methods.push("two_factor");
    }
    if (emailVerificationEnabled && viewer?.emailVerifiedAt) {
      methods.push("email");
    }
    return methods.length > 0 ? methods : ["none"];
  }, [emailVerificationEnabled, twoFactorEnabled, viewer?.emailVerifiedAt]);
  const deleteVerificationAvailable = securityVerificationMethods.some((method) => method !== "none");
  const beginDeleteAccountVerification = React.useCallback(() => {
    const method = securityVerificationMethods.find((item) => item !== "none") ?? "none";
    if (method === "none") {
      return;
    }
    setDeleteVerificationMethod(method);
    setDeleteDialogOpen(false);
    setDeleteVerificationDialogOpen(true);
  }, [securityVerificationMethods, setDeleteDialogOpen]);
  const canVerifyCurrentEmail = Boolean(emailVerificationEnabled && viewer?.email && !viewer.emailVerifiedAt);
  const emailActionLabel = emailBootstrapMode ? t("actions.set") : t("actions.update");
  const identityUnlinkDisabled = !viewer?.passwordEnabled && identities.length <= 1;
  const availableBindProviders = React.useMemo(
    () => identityProviders.filter((provider) => !identities.some((identity) => identity.providerSlug === provider.slug)),
    [identities, identityProviders],
  );
  const providerLogoBySlug = React.useMemo(() => {
    const result = new Map<string, string>();
    for (const provider of identityProviders) {
      if (provider.logoURL) result.set(provider.slug, provider.logoURL);
    }
    return result;
  }, [identityProviders]);

  const handleOpenTwoFactor = React.useCallback(() => {
    setTwoFactorDialogOpen(true);
  }, []);

  const handleStartTwoFactor = React.useCallback(() => {
    setTwoFactorOpening(true);
    void handleStartTwoFactorSetup()
      .then((started) => {
        if (started) setTwoFactorDialogOpen(true);
      })
      .finally(() => setTwoFactorOpening(false));
  }, [handleStartTwoFactorSetup]);

  return (
    <SettingsPage>
      <AccountOverviewSection
        viewer={viewer}
        loading={loading}
        loggingOut={loggingOut}
        deletingAccount={deletingAccount}
        changingPassword={changingPassword}
        twoFactorAvailable={Boolean(twoFactorStatus?.available)}
        twoFactorEnabled={twoFactorEnabled}
        twoFactorOpening={twoFactorOpening}
        emailVerificationEnabled={emailVerificationEnabled}
        canVerifyCurrentEmail={canVerifyCurrentEmail}
        emailActionLabel={emailActionLabel}
        onOpenCurrentEmailVerification={() => setCurrentEmailVerificationDialogOpen(true)}
        onOpenEmailDialog={() => setEmailDialogOpen(true)}
        onOpenPasswordDialog={() => setPasswordDialogOpen(true)}
        onOpenTwoFactorDialog={handleOpenTwoFactor}
        onStartTwoFactorSetup={handleStartTwoFactor}
        onLogoutAll={() => void handleLogoutAll()}
        onOpenDeleteDialog={() => setDeleteDialogOpen(true)}
      />

      <SettingsSectionSeparator />

      {identityProviders.length > 0 ? (
        <>
          <AccountIdentitiesSection
            loading={loading}
            identities={identities}
            identityProviders={identityProviders}
            availableBindProviders={availableBindProviders}
            providerLogoBySlug={providerLogoBySlug}
            identityUnlinkDisabled={identityUnlinkDisabled}
            onBindIdentity={(provider) => void handleBindIdentity(provider)}
            onDeleteIdentity={(identity) => void handleDeleteIdentity(identity)}
          />
          <SettingsSectionSeparator />
        </>
      ) : null}

      <AccountActiveSessionsSection
        sessions={sessions}
        loading={loading}
        revokingSessionID={revokingSessionID}
        onLogoutSession={(session) => void handleLogoutSession(session)}
      />

      <ChangePasswordDialog
        open={passwordDialogOpen}
        onOpenChange={setPasswordDialogOpen}
        passwordEnabled={Boolean(viewer?.passwordEnabled) && !viewer?.mustResetPassword}
        pending={changingPassword}
        sendingCode={sendingPasswordCode}
        resendCooldownSeconds={passwordCodeCooldownSeconds}
        debugCode={passwordCodeDebug}
        verificationMethods={securityVerificationMethods}
        required={Boolean(viewer?.mustResetPassword)}
        onSendCode={handleSendPasswordCode}
        onSubmit={handleChangePassword}
      />

      <EmailSecurityDialog
        open={emailDialogOpen}
        onOpenChange={setEmailDialogOpen}
        bootstrap={emailBootstrapMode}
        emailVerificationEnabled={emailVerificationEnabled}
        currentVerificationMethods={securityVerificationMethods}
        pending={changingPassword}
        sendingCode={sendingEmailCode}
        currentCodeCooldownSeconds={currentEmailCodeCooldownSeconds}
        newCodeCooldownSeconds={emailCodeCooldownSeconds}
        debugCode={emailCodeDebug}
        currentDebugCode={currentEmailCodeDebug}
        onSendBootstrapCode={handleSendEmailBootstrapCode}
        onCompleteBootstrap={handleCompleteEmailBootstrap}
        onSendCurrentCode={handleSendCurrentEmailCode}
        onSendNewCode={handleSendNewEmailCode}
        onCompleteChange={handleCompleteEmailChange}
      />

      <CurrentEmailVerificationDialog
        open={currentEmailVerificationDialogOpen}
        onOpenChange={setCurrentEmailVerificationDialogOpen}
        email={viewer?.email || ""}
        pending={changingPassword}
        sendingCode={sendingEmailCode}
        resendCooldownSeconds={currentEmailCodeCooldownSeconds}
        debugCode={currentEmailCodeDebug}
        onSendCode={handleSendCurrentEmailVerificationCode}
        onSubmit={handleCompleteCurrentEmailVerification}
      />

      <TwoFactorDialog
        open={twoFactorDialogOpen}
        onOpenChange={setTwoFactorDialogOpen}
        enabled={twoFactorEnabled}
        setupSecret={twoFactorSetup?.secret ?? ""}
        setupURL={twoFactorSetup?.otpauthURL ?? ""}
        setupExpiresAt={twoFactorSetup?.expiresAt ?? ""}
        recoveryCodes={twoFactorRecoveryCodes}
        onStartSetup={handleStartTwoFactorSetup}
        onConfirmSetup={handleConfirmTwoFactorSetup}
        onDisable={handleDisableTwoFactor}
        onRegenerateRecoveryCodes={handleRegenerateTwoFactorRecoveryCodes}
        onCancelSetup={handleCancelTwoFactorSetup}
        onClearRecoveryCodes={clearTwoFactorRecoveryCodes}
      />

      <AlertDialog open={deleteDialogOpen} onOpenChange={setDeleteDialogOpen}>
        <AlertDialogContent>
          <AlertDialogHeader>
            <AlertDialogTitle>{t("deleteDialog.title")}</AlertDialogTitle>
            <AlertDialogDescription>
              {deleteVerificationAvailable ? t("deleteDialog.description") : t("deleteDialog.verificationUnavailable")}
            </AlertDialogDescription>
          </AlertDialogHeader>
          <AlertDialogFooter>
            <AlertDialogCancel disabled={deletingAccount}>{t("actions.cancel")}</AlertDialogCancel>
            <AlertDialogAction variant="destructive" onClick={beginDeleteAccountVerification} disabled={deletingAccount || !deleteVerificationAvailable}>
              {deletingAccount ? <SpinnerLabel>{t("actions.deleting")}</SpinnerLabel> : t("actions.deleteAccount")}
            </AlertDialogAction>
          </AlertDialogFooter>
        </AlertDialogContent>
      </AlertDialog>

      <SecurityVerificationDialog
        open={deleteVerificationDialogOpen}
        onOpenChange={setDeleteVerificationDialogOpen}
        selectedMethod={deleteVerificationMethod}
        availableMethods={securityVerificationMethods}
        onMethodChange={setDeleteVerificationMethod}
        title={t("deleteDialog.verificationTitle")}
        description={deleteVerificationMethod === "two_factor" ? t("deleteDialog.verificationDescription.twoFactor") : t("deleteDialog.verificationDescription.email")}
        debugCode={deleteVerificationMethod === "email" ? deleteCodeDebug : ""}
        pending={deletingAccount}
        sendingCode={sendingDeleteCode}
        resendCooldownSeconds={deleteCodeCooldownSeconds}
        onSendCode={handleSendDeleteAccountCode}
        onSubmit={(code, method) => handleDeleteAccount({ verificationMethod: method, code })}
      />
    </SettingsPage>
  );
}
