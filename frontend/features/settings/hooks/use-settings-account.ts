"use client";

import * as React from "react";
import { useTranslations } from "next-intl";
import { toast } from "sonner";

import { resolveApiBaseURL } from "@/shared/api/http-client";
import { resolveAccessToken } from "@/shared/auth/resolve-access-token";
import {
  cancelCurrentTwoFactorSetup,
  changePassword,
  confirmCurrentTwoFactorSetup,
  completeEmailBootstrap,
  completeEmailChange,
  completeCurrentEmailVerification,
  deleteCurrentUserIdentity,
  deleteMe,
  disableCurrentTwoFactor,
  getCurrentActiveSessions,
  getCurrentTwoFactorStatus,
  getLoginOptions,
  getMe,
  listCurrentUserIdentities,
  logoutAll,
  logoutSession,
  regenerateCurrentTwoFactorRecoveryCodes,
  startCurrentTwoFactorSetup,
  startCurrentEmailChange,
  startCurrentEmailVerification,
  startAccountDeleteVerification,
  startEmailBootstrap,
  startNewEmailChange,
  startPasswordChangeVerification,
} from "@/shared/api/auth";
import { clearSessionAndRedirectToLogin } from "@/shared/auth/session";
import { useLocalizedErrorMessage } from "@/i18n/use-localized-error";
import type { ActiveSessionDTO, IdentityProviderDTO, SecurityVerificationMethod, TwoFactorSetupStartData, TwoFactorStatusData, UserDTO, UserIdentityDTO } from "@/shared/api/auth.types";

type UseSettingsAccountResult = {
  viewer: UserDTO | null;
  sessions: ActiveSessionDTO[];
  identities: UserIdentityDTO[];
  identityProviders: IdentityProviderDTO[];
  twoFactorStatus: TwoFactorStatusData | null;
  twoFactorSetup: TwoFactorSetupStartData | null;
  twoFactorRecoveryCodes: string[];
  loading: boolean;
  loggingOut: boolean;
  deletingAccount: boolean;
  changingPassword: boolean;
  sendingPasswordCode: boolean;
  passwordDialogOpen: boolean;
  emailDialogOpen: boolean;
  currentEmailVerificationDialogOpen: boolean;
  revokingSessionID: string;
  deleteDialogOpen: boolean;
  deleteCodeDebug: string;
  deleteCodeCooldownSeconds: number;
  sendingDeleteCode: boolean;
  emailVerificationEnabled: boolean;
  passwordCodeDebug: string;
  emailCodeDebug: string;
  currentEmailCodeDebug: string;
  passwordCodeCooldownSeconds: number;
  emailCodeCooldownSeconds: number;
  currentEmailCodeCooldownSeconds: number;
  sendingEmailCode: boolean;
  setPasswordDialogOpen: React.Dispatch<React.SetStateAction<boolean>>;
  setEmailDialogOpen: React.Dispatch<React.SetStateAction<boolean>>;
  setCurrentEmailVerificationDialogOpen: React.Dispatch<React.SetStateAction<boolean>>;
  setDeleteDialogOpen: React.Dispatch<React.SetStateAction<boolean>>;
  handleSendPasswordCode: (method: SecurityVerificationMethod) => Promise<void>;
  handleChangePassword: (payload: { currentPassword: string; newPassword: string; verificationMethod: SecurityVerificationMethod; code: string }) => Promise<void>;
  handleSendEmailBootstrapCode: (email: string) => Promise<void>;
  handleCompleteEmailBootstrap: (payload: { email: string; code: string }) => Promise<void>;
  handleSendCurrentEmailVerificationCode: () => Promise<void>;
  handleCompleteCurrentEmailVerification: (code: string) => Promise<void>;
  handleSendCurrentEmailCode: (method: SecurityVerificationMethod) => Promise<void>;
  handleSendNewEmailCode: (email: string) => Promise<void>;
  handleCompleteEmailChange: (payload: { email: string; currentVerificationMethod: SecurityVerificationMethod; currentCode: string; newCode: string }) => Promise<void>;
  handleSendDeleteAccountCode: (method: SecurityVerificationMethod) => Promise<void>;
  handleStartTwoFactorSetup: () => Promise<boolean>;
  handleConfirmTwoFactorSetup: (code: string) => Promise<void>;
  handleDisableTwoFactor: (code: string) => Promise<boolean>;
  handleRegenerateTwoFactorRecoveryCodes: (code: string) => Promise<void>;
  handleCancelTwoFactorSetup: () => Promise<void>;
  clearTwoFactorRecoveryCodes: () => void;
  handleLogoutAll: () => Promise<void>;
  handleDeleteAccount: (payload: { verificationMethod: SecurityVerificationMethod; code: string }) => Promise<void>;
  handleLogoutSession: (session: ActiveSessionDTO) => Promise<void>;
  handleBindIdentity: (provider: IdentityProviderDTO) => Promise<void>;
  handleDeleteIdentity: (identity: UserIdentityDTO) => Promise<void>;
};

const VERIFICATION_CODE_RESEND_COOLDOWN_MS = 60_000;

function providerPKCEStorageKey(slug: string): string {
  return `deeix-chat:oauth:${slug}:pkce_verifier`;
}

function base64URL(bytes: Uint8Array): string {
  let binary = "";
  bytes.forEach((byte) => {
    binary += String.fromCharCode(byte);
  });
  return btoa(binary).replaceAll("+", "-").replaceAll("/", "_").replaceAll("=", "");
}

async function createProviderPKCE() {
  const verifierBytes = new Uint8Array(48);
  window.crypto.getRandomValues(verifierBytes);
  const verifier = base64URL(verifierBytes);
  const digest = await window.crypto.subtle.digest("SHA-256", new TextEncoder().encode(verifier));
  return {
    verifier,
    challenge: base64URL(new Uint8Array(digest)),
  };
}

export function useSettingsAccount(): UseSettingsAccountResult {
  const t = useTranslations("settings.accountPage.toasts");
  const translateError = useLocalizedErrorMessage();
  const [viewer, setViewer] = React.useState<UserDTO | null>(null);
  const [sessions, setSessions] = React.useState<ActiveSessionDTO[]>([]);
  const [identities, setIdentities] = React.useState<UserIdentityDTO[]>([]);
  const [identityProviders, setIdentityProviders] = React.useState<IdentityProviderDTO[]>([]);
  const [twoFactorStatus, setTwoFactorStatus] = React.useState<TwoFactorStatusData | null>(null);
  const [twoFactorSetup, setTwoFactorSetup] = React.useState<TwoFactorSetupStartData | null>(null);
  const [twoFactorRecoveryCodes, setTwoFactorRecoveryCodes] = React.useState<string[]>([]);
  const [loading, setLoading] = React.useState(true);
  const [loggingOut, setLoggingOut] = React.useState(false);
  const [deletingAccount, setDeletingAccount] = React.useState(false);
  const [changingPassword, setChangingPassword] = React.useState(false);
  const [sendingPasswordCode, setSendingPasswordCode] = React.useState(false);
  const [revokingSessionID, setRevokingSessionID] = React.useState("");
  const [passwordDialogOpen, setPasswordDialogOpen] = React.useState(false);
  const [emailDialogOpen, setEmailDialogOpen] = React.useState(false);
  const [currentEmailVerificationDialogOpen, setCurrentEmailVerificationDialogOpen] = React.useState(false);
  const [deleteDialogOpen, setDeleteDialogOpen] = React.useState(false);
  const [emailVerificationEnabled, setEmailVerificationEnabled] = React.useState(false);
  const [passwordCodeDebug, setPasswordCodeDebug] = React.useState("");
  const [emailCodeDebug, setEmailCodeDebug] = React.useState("");
  const [currentEmailCodeDebug, setCurrentEmailCodeDebug] = React.useState("");
  const [deleteCodeDebug, setDeleteCodeDebug] = React.useState("");
  const [passwordCodeResendAt, setPasswordCodeResendAt] = React.useState(0);
  const [emailCodeResendAt, setEmailCodeResendAt] = React.useState(0);
  const [currentEmailCodeResendAt, setCurrentEmailCodeResendAt] = React.useState(0);
  const [deleteCodeResendAt, setDeleteCodeResendAt] = React.useState(0);
  const [cooldownNow, setCooldownNow] = React.useState(() => Date.now());
  const [sendingEmailCode, setSendingEmailCode] = React.useState(false);
  const [sendingDeleteCode, setSendingDeleteCode] = React.useState(false);
  const passwordCodeCooldownSeconds = Math.max(0, Math.ceil((passwordCodeResendAt - cooldownNow) / 1000));
  const emailCodeCooldownSeconds = Math.max(0, Math.ceil((emailCodeResendAt - cooldownNow) / 1000));
  const currentEmailCodeCooldownSeconds = Math.max(0, Math.ceil((currentEmailCodeResendAt - cooldownNow) / 1000));
  const deleteCodeCooldownSeconds = Math.max(0, Math.ceil((deleteCodeResendAt - cooldownNow) / 1000));

  React.useEffect(() => {
    if (passwordCodeCooldownSeconds === 0 && emailCodeCooldownSeconds === 0 && currentEmailCodeCooldownSeconds === 0 && deleteCodeCooldownSeconds === 0) {
      return undefined;
    }
    const timer = window.setInterval(() => setCooldownNow(Date.now()), 1000);
    return () => window.clearInterval(timer);
  }, [currentEmailCodeCooldownSeconds, deleteCodeCooldownSeconds, emailCodeCooldownSeconds, passwordCodeCooldownSeconds]);

  const startPasswordCodeCooldown = React.useCallback(() => {
    const now = Date.now();
    setCooldownNow(now);
    setPasswordCodeResendAt(now + VERIFICATION_CODE_RESEND_COOLDOWN_MS);
  }, []);

  const startEmailCodeCooldown = React.useCallback(() => {
    const now = Date.now();
    setCooldownNow(now);
    setEmailCodeResendAt(now + VERIFICATION_CODE_RESEND_COOLDOWN_MS);
  }, []);

  const startCurrentEmailCodeCooldown = React.useCallback(() => {
    const now = Date.now();
    setCooldownNow(now);
    setCurrentEmailCodeResendAt(now + VERIFICATION_CODE_RESEND_COOLDOWN_MS);
  }, []);

  const startDeleteCodeCooldown = React.useCallback(() => {
    const now = Date.now();
    setCooldownNow(now);
    setDeleteCodeResendAt(now + VERIFICATION_CODE_RESEND_COOLDOWN_MS);
  }, []);

  const loadAccountData = React.useCallback(async () => {
    setLoading(true);

    try {
      const token = await resolveAccessToken();
      if (!token) {
        setViewer(null);
        setSessions([]);
        return;
      }

      const [nextViewer, sessionData, loginOptions, identityData, twoFactorData] = await Promise.all([getMe(token), getCurrentActiveSessions(token), getLoginOptions(), listCurrentUserIdentities(token), getCurrentTwoFactorStatus(token)]);
      setViewer(nextViewer);
      setSessions(sessionData.results);
      setIdentities(identityData.results);
      setIdentityProviders(loginOptions.providers.filter((provider) => provider.loginEnabled));
      setTwoFactorStatus(twoFactorData);
      setEmailVerificationEnabled(loginOptions.emailVerificationEnabled);
    } catch (error) {
      toast.error(t("loadFailed"), { description: translateError(error, t("retryLater")) });
    } finally {
      setLoading(false);
    }
  }, [t, translateError]);

  React.useEffect(() => {
    void loadAccountData();
  }, [loadAccountData]);

  const handleLogoutAll = React.useCallback(async () => {
    if (loggingOut) {
      return;
    }

    setLoggingOut(true);
    try {
      const token = await resolveAccessToken();
      if (token) {
        await logoutAll(token);
      }
    } catch {
      // Ignore API errors and clear local session to ensure exit.
    } finally {
      clearSessionAndRedirectToLogin();
      setLoggingOut(false);
    }
  }, [loggingOut]);

  const handleSendPasswordCode = React.useCallback(async (method: SecurityVerificationMethod) => {
    if (method !== "email" || sendingPasswordCode || passwordCodeCooldownSeconds > 0) {
      return;
    }
    setSendingPasswordCode(true);
    setPasswordCodeDebug("");
    try {
      const token = await resolveAccessToken();
      if (!token) {
        throw new Error(t("sessionMissing"));
      }
      const result = await startPasswordChangeVerification(token, method);
      setPasswordCodeDebug(result.debugCode ?? "");
      if (result.sent) {
        startPasswordCodeCooldown();
        toast.success(t("codeSent"));
      }
    } catch (error) {
      toast.error(t("sendCodeFailed"), { description: translateError(error, t("retryLater")) });
    } finally {
      setSendingPasswordCode(false);
    }
  }, [passwordCodeCooldownSeconds, sendingPasswordCode, startPasswordCodeCooldown, t, translateError]);

  const handleChangePassword = React.useCallback(async (payload: { currentPassword: string; newPassword: string; verificationMethod: SecurityVerificationMethod; code: string }) => {
    if (changingPassword) {
      return;
    }
    setChangingPassword(true);
    try {
      const token = await resolveAccessToken();
      if (!token) {
        throw new Error(t("sessionMissing"));
      }
      await changePassword(token, {
        currentPassword: payload.currentPassword || undefined,
        newPassword: payload.newPassword,
        verificationMethod: payload.verificationMethod,
        code: payload.code,
      });
      toast.success(t("passwordChanged"), { description: t("passwordChangedDescription") });
      setPasswordDialogOpen(false);
      clearSessionAndRedirectToLogin();
    } catch (error) {
      toast.error(t("changePasswordFailed"), { description: translateError(error, t("retryLater")) });
    } finally {
      setChangingPassword(false);
    }
  }, [changingPassword, t, translateError]);

  const handleSendEmailBootstrapCode = React.useCallback(async (email: string) => {
    if (sendingEmailCode || emailCodeCooldownSeconds > 0) return;
    setSendingEmailCode(true);
    setEmailCodeDebug("");
    try {
      const token = await resolveAccessToken();
      if (!token) throw new Error(t("sessionMissing"));
      const result = await startEmailBootstrap(token, email);
      setEmailCodeDebug(result.debugCode ?? "");
      if (result.sent) {
        startEmailCodeCooldown();
        toast.success(t("codeSent"));
      }
    } catch (error) {
      toast.error(t("sendCodeFailed"), { description: translateError(error, t("retryLater")) });
    } finally {
      setSendingEmailCode(false);
    }
  }, [emailCodeCooldownSeconds, sendingEmailCode, startEmailCodeCooldown, t, translateError]);

  const handleCompleteEmailBootstrap = React.useCallback(async (payload: { email: string; code: string }) => {
    if (changingPassword) return;
    setChangingPassword(true);
    try {
      const token = await resolveAccessToken();
      if (!token) throw new Error(t("sessionMissing"));
      const nextViewer = await completeEmailBootstrap(token, payload);
      setViewer(nextViewer);
      setEmailDialogOpen(false);
      toast.success(t("emailSet"));
    } catch (error) {
      toast.error(t("setEmailFailed"), { description: translateError(error, t("retryLater")) });
    } finally {
      setChangingPassword(false);
    }
  }, [changingPassword, t, translateError]);

  const handleSendCurrentEmailVerificationCode = React.useCallback(async () => {
    if (sendingEmailCode || currentEmailCodeCooldownSeconds > 0) return;
    setSendingEmailCode(true);
    setCurrentEmailCodeDebug("");
    try {
      const token = await resolveAccessToken();
      if (!token) throw new Error(t("sessionMissing"));
      const result = await startCurrentEmailVerification(token);
      setCurrentEmailCodeDebug(result.debugCode ?? "");
      if (result.sent) {
        startCurrentEmailCodeCooldown();
        toast.success(t("currentEmailCodeSent"));
      }
    } catch (error) {
      toast.error(t("sendCodeFailed"), { description: translateError(error, t("retryLater")) });
    } finally {
      setSendingEmailCode(false);
    }
  }, [currentEmailCodeCooldownSeconds, sendingEmailCode, startCurrentEmailCodeCooldown, t, translateError]);

  const handleCompleteCurrentEmailVerification = React.useCallback(async (code: string) => {
    if (changingPassword) return;
    setChangingPassword(true);
    try {
      const token = await resolveAccessToken();
      if (!token) throw new Error(t("sessionMissing"));
      const nextViewer = await completeCurrentEmailVerification(token, code);
      setViewer(nextViewer);
      setCurrentEmailVerificationDialogOpen(false);
      toast.success(t("emailVerified"));
    } catch (error) {
      toast.error(t("verifyEmailFailed"), { description: translateError(error, t("retryLater")) });
    } finally {
      setChangingPassword(false);
    }
  }, [changingPassword, t, translateError]);

  const handleSendCurrentEmailCode = React.useCallback(async (method: SecurityVerificationMethod) => {
    if (method !== "email" || sendingEmailCode || currentEmailCodeCooldownSeconds > 0) return;
    setSendingEmailCode(true);
    setCurrentEmailCodeDebug("");
    try {
      const token = await resolveAccessToken();
      if (!token) throw new Error(t("sessionMissing"));
      const result = await startCurrentEmailChange(token, method);
      setCurrentEmailCodeDebug(result.debugCode ?? "");
      if (result.sent) {
        startCurrentEmailCodeCooldown();
        toast.success(t("currentEmailCodeSent"));
      }
    } catch (error) {
      toast.error(t("sendCodeFailed"), { description: translateError(error, t("retryLater")) });
    } finally {
      setSendingEmailCode(false);
    }
  }, [currentEmailCodeCooldownSeconds, sendingEmailCode, startCurrentEmailCodeCooldown, t, translateError]);

  const handleSendNewEmailCode = React.useCallback(async (email: string) => {
    if (sendingEmailCode || emailCodeCooldownSeconds > 0) return;
    setSendingEmailCode(true);
    setEmailCodeDebug("");
    try {
      const token = await resolveAccessToken();
      if (!token) throw new Error(t("sessionMissing"));
      const result = await startNewEmailChange(token, email);
      setEmailCodeDebug(result.debugCode ?? "");
      if (result.sent) {
        startEmailCodeCooldown();
        toast.success(t("newEmailCodeSent"));
      }
    } catch (error) {
      toast.error(t("sendCodeFailed"), { description: translateError(error, t("retryLater")) });
    } finally {
      setSendingEmailCode(false);
    }
  }, [emailCodeCooldownSeconds, sendingEmailCode, startEmailCodeCooldown, t, translateError]);

  const handleCompleteEmailChange = React.useCallback(async (payload: { email: string; currentVerificationMethod: SecurityVerificationMethod; currentCode: string; newCode: string }) => {
    if (changingPassword) return;
    setChangingPassword(true);
    try {
      const token = await resolveAccessToken();
      if (!token) throw new Error(t("sessionMissing"));
      const nextViewer = await completeEmailChange(token, {
        email: payload.email,
        currentVerificationMethod: payload.currentVerificationMethod,
        currentCode: payload.currentCode,
        newCode: payload.newCode,
      });
      setViewer(nextViewer);
      setEmailDialogOpen(false);
      toast.success(t("emailChanged"));
    } catch (error) {
      toast.error(t("changeEmailFailed"), { description: translateError(error, t("retryLater")) });
    } finally {
      setChangingPassword(false);
    }
  }, [changingPassword, t, translateError]);

  const handleSendDeleteAccountCode = React.useCallback(async (method: SecurityVerificationMethod) => {
    if (method !== "email" || sendingDeleteCode || deleteCodeCooldownSeconds > 0) return;
    setSendingDeleteCode(true);
    setDeleteCodeDebug("");
    try {
      const token = await resolveAccessToken();
      if (!token) throw new Error(t("sessionMissing"));
      const result = await startAccountDeleteVerification(token, method);
      setDeleteCodeDebug(result.debugCode ?? "");
      if (result.sent) {
        startDeleteCodeCooldown();
        toast.success(t("deleteAccountCodeSent"));
      }
    } catch (error) {
      toast.error(t("sendCodeFailed"), { description: translateError(error, t("retryLater")) });
    } finally {
      setSendingDeleteCode(false);
    }
  }, [deleteCodeCooldownSeconds, sendingDeleteCode, startDeleteCodeCooldown, t, translateError]);

  const handleDeleteAccount = React.useCallback(async (payload: { verificationMethod: SecurityVerificationMethod; code: string }) => {
    if (deletingAccount) {
      return;
    }

    setDeletingAccount(true);
    try {
      const token = await resolveAccessToken();
      if (!token) {
        throw new Error(t("sessionMissing"));
      }

      await deleteMe(token, payload);
      setDeleteDialogOpen(false);
      clearSessionAndRedirectToLogin();
    } catch (error) {
      toast.error(t("deleteAccountFailed"), { description: translateError(error, t("retryLater")) });
    } finally {
      setDeletingAccount(false);
    }
  }, [deletingAccount, t, translateError]);

  const handleLogoutSession = React.useCallback(
    async (session: ActiveSessionDTO) => {
      const targetSessionID = session.sessionID.trim();
      if (!targetSessionID || revokingSessionID) {
        return;
      }

      setRevokingSessionID(targetSessionID);
      try {
        const token = await resolveAccessToken();
        if (!token) {
          throw new Error(t("sessionMissing"));
        }

        await logoutSession(token, targetSessionID);
        if (session.current) {
          clearSessionAndRedirectToLogin();
          return;
        }

        setSessions((currentSessions) => currentSessions.filter((item) => item.sessionID !== targetSessionID));
        toast.success(t("sessionLoggedOut"));
      } catch (error) {
        toast.error(t("logoutSessionFailed"), { description: translateError(error, t("retryLater")) });
      } finally {
        setRevokingSessionID("");
      }
    },
    [revokingSessionID, t, translateError],
  );

  const handleDeleteIdentity = React.useCallback(async (identity: UserIdentityDTO) => {
    try {
      const token = await resolveAccessToken();
      if (!token) throw new Error(t("sessionMissing"));
      await deleteCurrentUserIdentity(token, identity.id);
      const [nextViewer, identityData] = await Promise.all([getMe(token), listCurrentUserIdentities(token)]);
      setViewer(nextViewer);
      setIdentities(identityData.results);
      toast.success(t("identityUnlinked"));
    } catch (error) {
      toast.error(t("unlinkIdentityFailed"), { description: translateError(error, t("retryLater")) });
    }
  }, [t, translateError]);

  const handleStartTwoFactorSetup = React.useCallback(async () => {
    try {
      const token = await resolveAccessToken();
      if (!token) throw new Error(t("sessionMissing"));
      const result = await startCurrentTwoFactorSetup(token);
      setTwoFactorSetup(result);
      setTwoFactorRecoveryCodes([]);
      return true;
    } catch (error) {
      toast.error(t("startTwoFactorFailed"), { description: translateError(error, t("retryLater")) });
      return false;
    }
  }, [t, translateError]);

  const handleConfirmTwoFactorSetup = React.useCallback(async (code: string) => {
    try {
      const token = await resolveAccessToken();
      if (!token) throw new Error(t("sessionMissing"));
      const result = await confirmCurrentTwoFactorSetup(token, code);
      const status = await getCurrentTwoFactorStatus(token);
      setTwoFactorStatus(status);
      setTwoFactorSetup(null);
      setTwoFactorRecoveryCodes(result.recoveryCodes);
      toast.success(t("twoFactorEnabled"));
    } catch (error) {
      toast.error(t("enableTwoFactorFailed"), { description: translateError(error, t("retryLater")) });
    }
  }, [t, translateError]);

  const handleDisableTwoFactor = React.useCallback(async (code: string) => {
    try {
      const token = await resolveAccessToken();
      if (!token) throw new Error(t("sessionMissing"));
      await disableCurrentTwoFactor(token, code);
      const status = await getCurrentTwoFactorStatus(token);
      setTwoFactorStatus(status);
      setTwoFactorSetup(null);
      setTwoFactorRecoveryCodes([]);
      toast.success(t("twoFactorDisabled"));
      return true;
    } catch (error) {
      toast.error(t("disableTwoFactorFailed"), { description: translateError(error, t("retryLater")) });
      return false;
    }
  }, [t, translateError]);

  const handleRegenerateTwoFactorRecoveryCodes = React.useCallback(async (code: string) => {
    try {
      const token = await resolveAccessToken();
      if (!token) throw new Error(t("sessionMissing"));
      const result = await regenerateCurrentTwoFactorRecoveryCodes(token, code);
      const status = await getCurrentTwoFactorStatus(token);
      setTwoFactorStatus(status);
      setTwoFactorRecoveryCodes(result.recoveryCodes);
      toast.success(t("recoveryCodesRegenerated"));
    } catch (error) {
      toast.error(t("regenerateRecoveryCodesFailed"), { description: translateError(error, t("retryLater")) });
    }
  }, [t, translateError]);

  const handleCancelTwoFactorSetup = React.useCallback(async () => {
    if (!twoFactorSetup) {
      setTwoFactorRecoveryCodes([]);
      return;
    }
    try {
      const token = await resolveAccessToken();
      if (token) {
        await cancelCurrentTwoFactorSetup(token);
      }
    } catch {
      // Closing the setup dialog should not block the user; the next setup rotates the secret.
    } finally {
      setTwoFactorSetup(null);
      setTwoFactorRecoveryCodes([]);
    }
  }, [twoFactorSetup]);

  const clearTwoFactorRecoveryCodes = React.useCallback(() => {
    setTwoFactorRecoveryCodes([]);
  }, []);

  const handleBindIdentity = React.useCallback(async (provider: IdentityProviderDTO) => {
    try {
      const token = await resolveAccessToken();
      if (!token) {
        throw new Error(t("sessionMissing"));
      }
      if (!provider.loginEnabled) {
        throw new Error(t("providerUnavailable"));
      }
      const redirectURI = `${window.location.origin}/auth/callback?provider=${encodeURIComponent(provider.slug)}`;
      const pkce = await createProviderPKCE();
      window.sessionStorage.setItem(providerPKCEStorageKey(provider.slug), pkce.verifier);
      const params = new URLSearchParams();
      params.set("redirect_uri", redirectURI);
      params.set("next", "/setting/account");
      params.set("code_challenge", pkce.challenge);
      params.set("intent", "bind");
      window.location.href = `${resolveApiBaseURL()}/api/v1/auth/providers/${encodeURIComponent(provider.slug)}/start?${params.toString()}`;
    } catch (error) {
      toast.error(t("bindIdentityFailed"), { description: translateError(error, t("retryLater")) });
    }
  }, [t, translateError]);

  return {
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
    sendingEmailCode,
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
  };
}
