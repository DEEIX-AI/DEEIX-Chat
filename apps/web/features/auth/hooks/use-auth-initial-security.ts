import { useTranslations } from "next-intl";
import * as React from "react";
import { toast } from "sonner";

import { readLocalAppearancePreferences, serializeAppearancePreferences } from "@/features/settings";
import { useAppLocale } from "@/i18n/app-i18n-provider";
import type { AppLocale } from "@/i18n/config";
import { useLocalizedErrorMessage } from "@/i18n/use-localized-error";
import {
  cancelCurrentTwoFactorSetup,
  completeOnboarding,
  confirmCurrentTwoFactorSetup,
  isPasswordReuseNotAllowedError,
  patchMe,
  patchUsername,
  startCurrentTwoFactorSetup,
} from "@/shared/api/auth";
import type { TwoFactorSetupStartData, UserDTO } from "@/shared/api/auth-types";
import { isDisplayNameLengthValid, isPasswordPolicyValid, isUsernamePolicyValid } from "@/shared/auth/account-policy";
import { useAuthSession } from "@/shared/auth/auth-session-context";
import { clearSessionAndRedirectToLogin } from "@/shared/auth/session";
import { dispatchUserProfileUpdated } from "@/shared/auth/user-profile-events";
import { useTheme } from "@/shared/components/theme-provider";
import { detectCurrentTimeZone } from "@/shared/lib/time-zone";

/**
 * Drives the first-login onboarding wizard (welcome → account → 2FA → theme
 * preset → personalization → finish). The step is advanced only after the
 * step's save succeeds, so step, form drafts and request state live together.
 * 2FA setup is started once when the 2FA step is reached (guarded by a ref so
 * re-renders do not issue a second setup) and can be retried after failure.
 */
export function useAuthInitialSecurity() {
  const t = useTranslations("guide");
  const tCommonErrors = useTranslations("common.errors");
  const resolveErrorMessage = useLocalizedErrorMessage();
  const { locale } = useAppLocale();
  const { preset } = useTheme();
  const { accessToken, user, refreshUser } = useAuthSession();
  const [viewer, setViewer] = React.useState<UserDTO | null>(null);
  const [step, setStep] = React.useState(1);
  const [guideActive, setGuideActive] = React.useState(false);
  const [username, setUsername] = React.useState("");
  const [displayName, setDisplayName] = React.useState("");
  const [timezone, setTimezone] = React.useState(detectCurrentTimeZone);
  const [password, setPassword] = React.useState("");
  const [otp, setOtp] = React.useState("");
  const [savingAccount, setSavingAccount] = React.useState(false);
  const [savingTwoFactor, setSavingTwoFactor] = React.useState(false);
  const [savingLocale, setSavingLocale] = React.useState<AppLocale | null>(null);
  const [savingThemePreset, setSavingThemePreset] = React.useState(false);
  const [savingPersonalization, setSavingPersonalization] = React.useState(false);
  const [finishing, setFinishing] = React.useState(false);
  const [twoFactorSetup, setTwoFactorSetup] = React.useState<TwoFactorSetupStartData | null>(null);
  const [recoveryCodes, setRecoveryCodes] = React.useState<string[]>([]);
  const [twoFactorSkipped, setTwoFactorSkipped] = React.useState(false);
  const setupStartedRef = React.useRef(false);
  const initializedTimeZoneUserRef = React.useRef<string | null>(null);
  const currentTimeZone = React.useMemo(() => detectCurrentTimeZone(), []);

  React.useEffect(() => {
    setViewer(user);
    setUsername(user?.username ?? "");
    setDisplayName(user?.displayName ?? "");
    if (!user) {
      setGuideActive(false);
      setRecoveryCodes([]);
      setTwoFactorSetup(null);
      setTwoFactorSkipped(false);
      setupStartedRef.current = false;
      initializedTimeZoneUserRef.current = null;
      setStep(1);
      return;
    }
    if (user.initialSecurityRequired) {
      if (initializedTimeZoneUserRef.current !== user.publicID) {
        initializedTimeZoneUserRef.current = user.publicID;
        setTimezone(detectCurrentTimeZone());
      }
    } else {
      initializedTimeZoneUserRef.current = null;
      setTimezone(user.timezone.trim() || detectCurrentTimeZone());
    }
    setGuideActive(Boolean(user.initialSecurityRequired));
  }, [user]);

  React.useEffect(() => {
    if (step !== 3 || !viewer?.twoFactorAvailable || viewer.twoFactorEnabled || setupStartedRef.current) {
      return;
    }
    setupStartedRef.current = true;
    setTwoFactorSkipped(false);
    setSavingTwoFactor(true);
    void startCurrentTwoFactorSetup(accessToken)
      .then((result) => setTwoFactorSetup(result))
      .catch((error) => {
        setupStartedRef.current = false;
        toast.error(t("toasts.startTwoFactorFailed"), {
          description: resolveErrorMessage(error, tCommonErrors("unknown")),
        });
      })
      .finally(() => setSavingTwoFactor(false));
  }, [accessToken, resolveErrorMessage, step, t, tCommonErrors, viewer?.twoFactorAvailable, viewer?.twoFactorEnabled]);

  React.useEffect(() => {
    setOtp("");
  }, [twoFactorSetup?.secret]);

  const submitAccountStep = React.useCallback(async () => {
    if (!viewer?.initialSecurityRequired || savingAccount) return;
    const nextUsername = username.trim().toLowerCase();
    const nextDisplayName = displayName.trim();
    const nextPassword = password.trim();
    if (viewer.initialUsernameRequired && nextUsername === viewer.username.trim().toLowerCase()) {
      toast.error(t("toasts.changeInitialUsername"));
      return;
    }
    if (viewer.initialUsernameRequired && !isUsernamePolicyValid(nextUsername)) {
      toast.error(t("toasts.usernameTooShort"));
      return;
    }
    if (!viewer.mustResetPassword && !isDisplayNameLengthValid(nextDisplayName)) {
      toast.error(t("toasts.displayNameRequired"));
      return;
    }
    if (viewer.mustResetPassword && !isPasswordPolicyValid(nextPassword)) {
      toast.error(t("toasts.passwordTooShort"));
      return;
    }

    setSavingAccount(true);
    try {
      let nextViewer = viewer;
      if (viewer.initialUsernameRequired) {
        nextViewer = await patchUsername(accessToken, { username: nextUsername });
      }
      const profilePayload: Parameters<typeof patchMe>[1] = {};
      if (!viewer.mustResetPassword && nextDisplayName !== viewer.displayName.trim()) {
        profilePayload.displayName = nextDisplayName;
      }
      if (Object.keys(profilePayload).length > 0) {
        nextViewer = await patchMe(accessToken, profilePayload);
      }
      setViewer(nextViewer);
      dispatchUserProfileUpdated(nextViewer);
      setStep(3);
    } catch (error) {
      toast.error(t("toasts.saveAccountFailed"), {
        description: resolveErrorMessage(error, tCommonErrors("unknown")),
      });
    } finally {
      setSavingAccount(false);
    }
  }, [accessToken, displayName, password, resolveErrorMessage, savingAccount, t, tCommonErrors, username, viewer]);

  const confirmTwoFactor = React.useCallback(async () => {
    if (savingTwoFactor) return;
    const code = otp.replace(/\D/g, "").slice(0, 6);
    if (code.length !== 6) {
      toast.error(t("toasts.otpRequired"));
      return;
    }

    setSavingTwoFactor(true);
    try {
      const result = await confirmCurrentTwoFactorSetup(accessToken, code);
      const status = result.status;
      if (!status?.totpEnabled) {
        throw new Error(t("toasts.twoFactorNotEnabled"));
      }
      const nextViewer = await refreshUser();
      if (nextViewer && !nextViewer.twoFactorEnabled) {
        throw new Error(t("toasts.twoFactorNotSynced"));
      }
      setRecoveryCodes(result.recoveryCodes);
      setTwoFactorSkipped(false);
      setViewer((current) => nextViewer ?? (current ? {
        ...current,
        twoFactorAvailable: status.available,
        twoFactorEnabled: status.totpEnabled,
        twoFactorRequired: status.required,
        twoFactorRecoveryCount: status.recoveryCount,
      } : current));
      setTwoFactorSetup(null);
      setupStartedRef.current = false;
      setStep(4);
      if (nextViewer) {
        dispatchUserProfileUpdated(nextViewer);
      }
    } catch (error) {
      toast.error(t("toasts.enableTwoFactorFailed"), {
        description: resolveErrorMessage(error, tCommonErrors("unknown")),
      });
    } finally {
      setSavingTwoFactor(false);
    }
  }, [accessToken, otp, refreshUser, resolveErrorMessage, savingTwoFactor, t, tCommonErrors]);

  const skipTwoFactor = React.useCallback(async () => {
    if (savingTwoFactor) return;
    setSavingTwoFactor(true);
    try {
      if (twoFactorSetup) {
        await cancelCurrentTwoFactorSetup(accessToken);
      }
      setTwoFactorSkipped(true);
      setTwoFactorSetup(null);
      setupStartedRef.current = false;
      setStep(4);
    } catch (error) {
      toast.error(t("toasts.skipTwoFactorFailed"), {
        description: resolveErrorMessage(error, tCommonErrors("unknown")),
      });
    } finally {
      setSavingTwoFactor(false);
    }
  }, [accessToken, resolveErrorMessage, savingTwoFactor, t, tCommonErrors, twoFactorSetup]);

  const saveWelcomeStep = React.useCallback(async () => {
    if (!viewer || savingLocale) {
      return;
    }
    const nextLocale = locale;

    if (nextLocale === viewer.locale.trim()) {
      setStep(2);
      return;
    }

    setSavingLocale(nextLocale);
    try {
      const nextViewer = await patchMe(accessToken, { locale: nextLocale });
      setViewer(nextViewer);
      dispatchUserProfileUpdated(nextViewer);
      setStep(2);
    } catch (error) {
      toast.error(t("toasts.saveLanguageFailed"), {
        description: resolveErrorMessage(error, tCommonErrors("unknown")),
      });
    } finally {
      setSavingLocale((current) => (current === nextLocale ? null : current));
    }
  }, [accessToken, locale, resolveErrorMessage, savingLocale, t, tCommonErrors, viewer]);

  const currentAppearancePreferences = React.useCallback(
    () => serializeAppearancePreferences({
      ...readLocalAppearancePreferences(),
      preset,
    }),
    [preset],
  );

  const saveThemePresetStep = React.useCallback(async () => {
    if (!viewer || savingThemePreset) return;
    const appearancePreferences = currentAppearancePreferences();

    if (appearancePreferences === (viewer.appearancePreferences?.trim() ?? "")) {
      setStep(5);
      return;
    }

    setSavingThemePreset(true);
    try {
      const nextViewer = await patchMe(accessToken, { appearancePreferences });
      setViewer(nextViewer);
      dispatchUserProfileUpdated(nextViewer);
      setStep(5);
    } catch (error) {
      toast.error(t("toasts.savePersonalizationFailed"), {
        description: resolveErrorMessage(error, tCommonErrors("unknown")),
      });
    } finally {
      setSavingThemePreset(false);
    }
  }, [accessToken, currentAppearancePreferences, resolveErrorMessage, savingThemePreset, t, tCommonErrors, viewer]);

  const savePersonalizationStep = React.useCallback(async () => {
    if (!viewer || savingPersonalization) return;
    const nextTimezone = timezone.trim() || currentTimeZone;
    const profilePayload: Parameters<typeof patchMe>[1] = {};
    const appearancePreferences = currentAppearancePreferences();

    if (nextTimezone !== (viewer.timezone.trim() || "Etc/UTC")) {
      profilePayload.timezone = nextTimezone;
    }
    if (appearancePreferences !== (viewer.appearancePreferences?.trim() ?? "")) {
      profilePayload.appearancePreferences = appearancePreferences;
    }

    if (Object.keys(profilePayload).length === 0) {
      setStep(6);
      return;
    }

    setSavingPersonalization(true);
    try {
      const nextViewer = await patchMe(accessToken, profilePayload);
      setViewer(nextViewer);
      dispatchUserProfileUpdated(nextViewer);
      setStep(6);
    } catch (error) {
      toast.error(t("toasts.savePersonalizationFailed"), {
        description: resolveErrorMessage(error, tCommonErrors("unknown")),
      });
    } finally {
      setSavingPersonalization(false);
    }
  }, [accessToken, currentAppearancePreferences, currentTimeZone, resolveErrorMessage, savingPersonalization, t, tCommonErrors, timezone, viewer]);

  const finishInitialSecurity = React.useCallback(async () => {
    if (!viewer || finishing) return;
    if (viewer.mustResetPassword && !isPasswordPolicyValid(password)) {
      toast.error(t("toasts.passwordTooShort"));
      setStep(2);
      return;
    }
    setFinishing(true);
    try {
      const refreshedViewer = await refreshUser();
      let nextViewer = refreshedViewer;
      if (!twoFactorSkipped && nextViewer?.twoFactorAvailable && !nextViewer.twoFactorEnabled) {
        throw new Error(t("toasts.twoFactorNotSynced"));
      }
      nextViewer = await completeOnboarding(
        accessToken,
        viewer.mustResetPassword ? { newPassword: password.trim() } : undefined,
      );
      if (nextViewer) {
        setViewer(nextViewer);
        dispatchUserProfileUpdated(nextViewer);
      }
      setRecoveryCodes([]);
      setGuideActive(false);
      if (viewer.mustResetPassword) {
        toast.success(t("toasts.initializedRelogin"));
        clearSessionAndRedirectToLogin();
        return;
      }
      toast.success(t("toasts.complete"));
    } catch (error) {
      if (isPasswordReuseNotAllowedError(error)) {
        setStep(2);
      }
      toast.error(t("toasts.completeFailed"), {
        description: resolveErrorMessage(error, tCommonErrors("unknown")),
      });
    } finally {
      setFinishing(false);
    }
  }, [accessToken, finishing, password, refreshUser, resolveErrorMessage, t, tCommonErrors, twoFactorSkipped, viewer]);

  return {
    viewer,
    step,
    setStep,
    guideActive,
    currentTimeZone,
    username,
    setUsername,
    displayName,
    setDisplayName,
    timezone,
    setTimezone,
    password,
    setPassword,
    otp,
    setOtp,
    twoFactorSetup,
    recoveryCodes,
    savingAccount,
    savingTwoFactor,
    savingLocale,
    savingThemePreset,
    savingPersonalization,
    finishing,
    saveWelcomeStep,
    submitAccountStep,
    confirmTwoFactor,
    skipTwoFactor,
    saveThemePresetStep,
    savePersonalizationStep,
    finishInitialSecurity,
  };
}
