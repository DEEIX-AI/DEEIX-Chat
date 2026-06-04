"use client";

import * as React from "react";
import { Copy, MoreHorizontal, Unlink } from "lucide-react";
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
import { Button } from "@/components/ui/button";
import {
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuItem,
  DropdownMenuTrigger,
} from "@/components/ui/dropdown-menu";
import { SpinnerLabel } from "@/components/ui/spinner";
import { Table, TableBody, TableCell, TableEmptyRow, TableHead, TableHeader, TableRow, TableSkeletonRows } from "@/components/ui/table";
import { ChangePasswordDialog, CurrentEmailVerificationDialog, EmailSecurityDialog, SecurityVerificationDialog, TwoFactorDialog } from "@/features/settings/components/sections/account-security-dialogs";
import { useSettingsAccount } from "@/features/settings/hooks/use-settings-account";
import {
  formatDateTime,
  resolveEmailTitle,
  resolveEmailValue,
  resolveSessionIP,
  resolveSessionLocation,
  resolveSessionTitle,
  shouldUseEmailBootstrap,
} from "@/features/settings/model/account-page";
import type { ActiveSessionDTO, SecurityVerificationMethod } from "@/shared/api/auth.types";
import { IdentityProviderIcon } from "@/shared/components/identity-provider-icon";
import { useAppLocale } from "@/i18n/app-i18n-provider";
import {
  SettingsPage,
  SettingsSection,
  SettingsSectionSeparator,
} from "@/shared/components/settings-layout";

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

export function SettingsAccount() {
  const t = useTranslations("settings.accountPage");
  const { locale } = useAppLocale();
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
    handleCopyPublicID,
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

  return (
    <SettingsPage>
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
                  onClick={() => setCurrentEmailVerificationDialogOpen(true)}
                >
                  {t("actions.verify")}
                </Button>
              ) : null}
              <Button
                type="button"
                variant="outline"
                disabled={loading || changingPassword}
                onClick={() => setEmailDialogOpen(true)}
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
              onClick={() => setPasswordDialogOpen(true)}
            >
              {viewer?.passwordEnabled ? t("actions.update") : t("actions.set")}
            </Button>
          }
        />

        {twoFactorStatus?.available ? (
          <ActionRow
            title={t("twoFactor")}
            action={
              <Button
                type="button"
                variant="outline"
                disabled={loading || twoFactorOpening}
                onClick={() => {
                  if (twoFactorEnabled) {
                    setTwoFactorDialogOpen(true);
                    return;
                  }
                  setTwoFactorOpening(true);
                  void handleStartTwoFactorSetup()
                    .then((started) => {
                      if (started) setTwoFactorDialogOpen(true);
                    })
                    .finally(() => setTwoFactorOpening(false));
                }}
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
              onClick={() => void handleLogoutAll()}
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
              onClick={() => setDeleteDialogOpen(true)}
            >
              {deletingAccount ? <SpinnerLabel>{t("actions.deleting")}</SpinnerLabel> : t("actions.deleteAccount")}
            </Button>
          }
        />

        <ValueRow
          title={t("publicID")}
          value={viewer?.publicID || "-"}
          action={
            <Button
              type="button"
              variant="ghost"
              size="icon"
              onClick={() => void handleCopyPublicID()}
              disabled={!viewer?.publicID}
              aria-label={t("copyPublicID")}
              className="size-4 p-3"
            >
              <Copy className="size-3.5" />
            </Button>
          }
        />
      </SettingsSection>

      {identityProviders.length > 0 ? (
        <>
          <SettingsSectionSeparator />

          <SettingsSection
            title={t("identity.title")}
            actions={
              availableBindProviders.length > 0 ? (
                <DropdownMenu modal={false}>
                  <DropdownMenuTrigger asChild>
                    <Button type="button" variant="outline" disabled={loading}>
                      {t("actions.bind")}
                    </Button>
                  </DropdownMenuTrigger>
                  <DropdownMenuContent align="end">
                    {availableBindProviders.map((provider) => (
                      <DropdownMenuItem key={provider.publicID} onClick={() => void handleBindIdentity(provider)}>
                        <IdentityProviderIcon name={provider.name} slug={provider.slug} logoURL={provider.logoURL} />
                        {provider.name}
                      </DropdownMenuItem>
                    ))}
                  </DropdownMenuContent>
                </DropdownMenu>
              ) : null
            }
          >
            <Table className="table-fixed" style={{ minWidth: 800 }}>
              <colgroup>
                <col style={{ width: 180 }} />
                <col style={{ width: 160 }} />
                <col style={{ width: 240 }} />
                <col style={{ width: 164 }} />
                <col style={{ width: 56 }} />
              </colgroup>
              <TableHeader>
                <TableRow>
                  <TableHead>{t("identity.provider")}</TableHead>
                  <TableHead>{t("identity.username")}</TableHead>
                  <TableHead>{t("identity.email")}</TableHead>
                  <TableHead>{t("identity.linkedAt")}</TableHead>
                  <TableHead className="w-[56px]" stickyEnd />
                </TableRow>
              </TableHeader>
              <TableBody>
                {loading && identities.length === 0 ? <TableSkeletonRows colSpan={5} rowCount={4} /> : null}
                {!loading && identities.length === 0 ? (
                  <TableEmptyRow colSpan={5}>{t("identity.empty")}</TableEmptyRow>
                ) : null}
                {identities.map((identity) => (
                  <TableRow key={identity.id}>
                    <TableCell className="max-w-0 font-medium">
                      <div className="flex min-w-0 items-center gap-2">
                        <IdentityProviderIcon
                          name={identity.providerName || identity.providerSlug || identity.providerType}
                          slug={identity.providerSlug}
                          logoURL={providerLogoBySlug.get(identity.providerSlug) || identity.providerLogoURL}
                        />
                        <div className="min-w-0">
                          <div className="truncate">{identity.providerName || identity.providerSlug || identity.providerType}</div>
                        </div>
                      </div>
                    </TableCell>
                    <TableCell className="max-w-0 text-muted-foreground">
                      <span className="block truncate" title={identity.providerDisplayName || undefined}>
                        {identity.providerDisplayName || "-"}
                      </span>
                    </TableCell>
                    <TableCell className="max-w-0 text-muted-foreground">
                      <span className="block truncate" title={identity.email || undefined}>
                        {identity.email || "-"}
                      </span>
                    </TableCell>
                    <TableCell className="max-w-0 text-muted-foreground">
                      <span className="block truncate" title={formatDateTime(identity.linkedAt, locale)}>
                        {formatDateTime(identity.linkedAt, locale)}
                      </span>
                    </TableCell>
                    <TableCell className="w-[56px] whitespace-nowrap" stickyEnd>
                      <div className="flex justify-end">
                        <Button
                          type="button"
                          variant="ghost"
                          size="icon"
                          className="size-8 text-muted-foreground shadow-none"
                          onClick={() => void handleDeleteIdentity(identity)}
                          disabled={identityUnlinkDisabled}
                          aria-label={t("identity.unlink")}
                          title={identityUnlinkDisabled ? t("identity.unlinkDisabled") : t("identity.unlink")}
                        >
                          <Unlink className="size-3.5" />
                        </Button>
                      </div>
                    </TableCell>
                  </TableRow>
                ))}
              </TableBody>
            </Table>
          </SettingsSection>

          <SettingsSectionSeparator />
        </>
      ) : (
        <SettingsSectionSeparator />
      )}

      <SettingsSection title={t("session.title")}>
        <Table className="table-fixed" style={{ minWidth: 840 }}>
          <colgroup>
            <col style={{ width: 260 }} />
            <col style={{ width: 220 }} />
            <col style={{ width: 152 }} />
            <col style={{ width: 152 }} />
            <col style={{ width: 56 }} />
          </colgroup>
          <TableHeader>
            <TableRow>
              <TableHead>{t("session.device")}</TableHead>
              <TableHead>{t("session.location")}</TableHead>
              <TableHead>{t("session.createdAt")}</TableHead>
              <TableHead>{t("session.updatedAt")}</TableHead>
              <TableHead className="w-[56px]" stickyEnd />
            </TableRow>
          </TableHeader>
          <TableBody>
            {!loading && sessions.length === 0 ? (
              <TableEmptyRow colSpan={5}>{t("session.empty")}</TableEmptyRow>
            ) : null}

            {(loading ? Array.from({ length: 2 }) : sessions).map((item, index) => {
              if (loading) {
                return (
                  <TableRow key={`session-skeleton-${index}`}>
                    <TableCell>
                      <div className="my-2 h-4 w-full max-w-[10rem] animate-pulse rounded-full bg-muted/60" />
                    </TableCell>
                    <TableCell>
                      <div className="my-2 h-4 w-full max-w-[12rem] animate-pulse rounded-full bg-muted/50" />
                    </TableCell>
                    <TableCell>
                      <div className="my-2 h-4 w-full max-w-[8rem] animate-pulse rounded-full bg-muted/50" />
                    </TableCell>
                    <TableCell>
                      <div className="my-2 h-4 w-full max-w-[8rem] animate-pulse rounded-full bg-muted/50" />
                    </TableCell>
                    <TableCell>
                      <div className="ml-auto my-2 h-4 w-4 animate-pulse rounded-full bg-muted/50" />
                    </TableCell>
                  </TableRow>
                );
              }

              const session = item as ActiveSessionDTO;

              return (
                <TableRow key={session.sessionID}>
                  <TableCell className="max-w-0">
                    <div className="flex min-w-0 items-center gap-2">
                      <span className="min-w-0 truncate font-medium" title={resolveSessionTitle(session, t)}>
                        {resolveSessionTitle(session, t)}
                      </span>
                      {session.current ? (
                        <span className="inline-flex shrink-0 items-center rounded-md bg-muted px-1.5 py-0.5 text-xs">
                          {t("session.current")}
                        </span>
                      ) : null}
                    </div>
                  </TableCell>

                  <TableCell className="max-w-0 text-muted-foreground">
                    <div className="flex min-w-0 flex-col gap-1">
                      <span className="truncate" title={resolveSessionLocation(session, t)}>{resolveSessionLocation(session, t)}</span>
                      <span className="truncate text-xs" title={resolveSessionIP(session, t)}>{resolveSessionIP(session, t)}</span>
                    </div>
                  </TableCell>
                  <TableCell className="max-w-0 text-muted-foreground">
                    <span className="block truncate" title={formatDateTime(session.createdAt, locale)}>
                      {formatDateTime(session.createdAt, locale)}
                    </span>
                  </TableCell>
                  <TableCell className="max-w-0 text-muted-foreground">
                    <span className="block truncate" title={formatDateTime(session.updatedAt, locale)}>
                      {formatDateTime(session.updatedAt, locale)}
                    </span>
                  </TableCell>
                  <TableCell className="w-[56px] whitespace-nowrap" stickyEnd>
                    <div className="flex justify-end">
                      <DropdownMenu modal={false}>
                        <DropdownMenuTrigger asChild>
                          <Button
                            type="button"
                            variant="ghost"
                            size="icon"
                            className="size-8"
                            disabled={revokingSessionID === session.sessionID}
                            aria-label={t("session.actions")}
                          >
                            <MoreHorizontal className="size-3.5 stroke-1" />
                          </Button>
                        </DropdownMenuTrigger>
                        <DropdownMenuContent align="end">
                          <DropdownMenuItem
                            disabled={revokingSessionID === session.sessionID}
                            onClick={() => void handleLogoutSession(session)}
                          >
                            {t("session.logoutThisSession")}
                          </DropdownMenuItem>
                        </DropdownMenuContent>
                      </DropdownMenu>
                    </div>
                  </TableCell>
                </TableRow>
              );
            })}
          </TableBody>
        </Table>
      </SettingsSection>

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
