"use client";

import * as React from "react";
import { Copy, MapPinned, Monitor, Moon, ShieldCheck, Sun } from "lucide-react";
import { motion } from "motion/react";
import { useTranslations } from "next-intl";
import { toast } from "sonner";

import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { LogoCarousel, type LogoCarouselLogo } from "@/components/ui/logo-carousel";
import { Onboarding } from "@/components/ui/onboarding";
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from "@/components/ui/select";
import { SpinnerLabel } from "@/components/ui/spinner";
import { dispatchUserProfileUpdated } from "@/features/settings/events/user-profile-events";
import {
  readLocalAppearancePreferences,
  serializeAppearancePreferences,
} from "@/features/settings/utils/appearance-preferences";
import { cancelCurrentTwoFactorSetup, completeOnboarding, confirmCurrentTwoFactorSetup, patchMe, patchUsername, startCurrentTwoFactorSetup } from "@/shared/api/auth";
import type { TwoFactorSetupStartData, UserDTO } from "@/shared/api/auth.types";
import {
  DISPLAY_NAME_MAX_LENGTH,
  PASSWORD_MIN_LENGTH,
  USERNAME_MAX_LENGTH,
  isDisplayNameLengthValid,
  isPasswordPolicyValid,
  isUsernamePolicyValid,
} from "@/shared/auth/account-policy";
import { useAuthSession } from "@/shared/auth/auth-session-context";
import { clearSessionAndRedirectToLogin } from "@/shared/auth/session";
import { useAppLocale } from "@/i18n/app-i18n-provider";
import { APP_LOCALE_LABELS, APP_LOCALES, type AppLocale } from "@/i18n/config";
import { useLocalizedErrorMessage } from "@/i18n/use-localized-error";
import { AppLogo } from "@/shared/components/app-logo";
import { TimeZoneSelect } from "@/shared/components/time-zone-select";
import { useTheme, type ThemePreset } from "@/shared/components/theme-provider";
import { lobehubIconManifest } from "@/shared/generated/lobehub-icon-manifest";
import { writeClipboardText } from "@/shared/lib/clipboard";
import { createQRCodeSVG } from "@/shared/lib/qr-code";
import { detectCurrentTimeZone } from "@/shared/lib/time-zone";
import { cn } from "@/lib/utils";

type OnboardingTip = {
  key: string;
};

const adminOnboardingTips: OnboardingTip[] = [
  { key: "adminTips.upstreams" },
  { key: "adminTips.mcp" },
  { key: "adminTips.files" },
  { key: "adminTips.context" },
  { key: "adminTips.trace" },
  { key: "adminTips.billing" },
  { key: "adminTips.admin" },
  { key: "adminTips.ops" },
];

const userOnboardingTips: OnboardingTip[] = [
  { key: "userTips.profile" },
  { key: "userTips.models" },
  { key: "userTips.files" },
  { key: "userTips.conversation" },
  { key: "userTips.twoFactor" },
];

function buildLogoCarouselItems(): LogoCarouselLogo[] {
  const supportedIconSlugs = [
    "openai",
    "codex",
    "anthropic",
    "claude",
    "google",
    "gemini",
    "gemma",
    "xai",
    "grok",
    "moonshot",
    "kimi",
    "alibaba",
    "alibabacloud",
    "qwen",
    "deepseek",
    "xiaomimimo",
    "zhipu",
    "chatglm",
    "minimax",
    "doubao",
    "mistral",
    "hunyuan",
    "longcat",
    "openrouter",
    "copilot",
    "replicate",
    "fal",
    "stability",
    "runway",
    "luma",
    "ideogram",
    "midjourney",
    "suno",
    "elevenlabs",
  ];
  const iconByID = new Map<string, (typeof lobehubIconManifest)[number]>(
    lobehubIconManifest.map((item) => [item.id, item]),
  );
  return supportedIconSlugs.flatMap((slug, index) => {
    const item = iconByID.get(slug);
    if (!item) return [];
    return {
      id: `${item.id}-${index}`,
      name: item.name,
      src: item.src,
    };
  });
}

const onboardingLogoItems = buildLogoCarouselItems();
const onboardingThemePresets: ThemePreset[] = [
  "default",
  "azure",
  "cobalt",
  "graphite",
  "lagoon",
  "ink",
  "ochre",
  "sepia",
];

function OnboardingFeatureCarousel({
  activeIndex,
  logos,
  tips,
}: {
  activeIndex: number;
  logos: LogoCarouselLogo[];
  tips: OnboardingTip[];
}) {
  const t = useTranslations("guide");
  const activeTip = tips[activeIndex] ?? tips[0];
  const progressDurationSeconds = 4.2;

  return (
    <div className="flex h-full flex-col overflow-hidden">
      <div className="flex min-h-[238px] flex-1 items-center justify-center px-5">
        <LogoCarousel
          logos={logos}
          columnCount={3}
          className="w-full justify-center space-x-4"
          columnClassName="h-24 w-24 md:h-24 md:w-24"
          logoClassName="h-12 w-12 md:h-14 md:w-14"
        />
      </div>
      <div className="space-y-3 p-3">
        <div className="flex w-full gap-1.5 overflow-hidden">
          {tips.map((_, index) => (
            <div className="h-1 min-w-0 flex-1 overflow-hidden rounded-full bg-border/80" key={index}>
              {index === activeIndex ? (
                <motion.span
                  key={`onboarding-progress-${activeIndex}`}
                  className="block h-full origin-left rounded-full bg-foreground/75"
                  initial={{ scaleX: 0 }}
                  animate={{ scaleX: 1 }}
                  transition={{ duration: progressDurationSeconds, ease: "linear" }}
                />
              ) : null}
            </div>
          ))}
        </div>
        <div className="flex h-[3.75rem] items-start overflow-hidden">
          <motion.p
            key={activeTip.key}
            className="text-xs font-medium leading-5 tracking-normal text-foreground"
            initial={{ opacity: 0, y: 4 }}
            animate={{ opacity: 1, y: 0 }}
            exit={{ opacity: 0, y: -4 }}
            transition={{ duration: 0.22, ease: "easeOut" }}
          >
            {t(activeTip.key)}
          </motion.p>
        </div>
      </div>
    </div>
  );
}

export function InitialSecurityGuard() {
  const t = useTranslations("guide");
  const tCommonErrors = useTranslations("common.errors");
  const resolveErrorMessage = useLocalizedErrorMessage();
  const { locale, setLocale } = useAppLocale();
  const { preset, setPreset, theme, setTheme } = useTheme();
  const { accessToken, user, refreshUser } = useAuthSession();
  const [viewer, setViewer] = React.useState<UserDTO | null>(null);
  const [step, setStep] = React.useState(1);
  const [activeTipIndex, setActiveTipIndex] = React.useState(0);
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
  const qrCodeSVG = React.useMemo(
    () => (twoFactorSetup?.otpauthURL ? createQRCodeSVG(twoFactorSetup.otpauthURL, 3, t("aria.twoFactorQRCode")) : ""),
    [t, twoFactorSetup?.otpauthURL],
  );
  const qrCodeUnavailable = Boolean(twoFactorSetup?.otpauthURL && !qrCodeSVG);
  const isAdminGuide = viewer?.role === "admin" || viewer?.role === "superadmin";
  const activeOnboardingTips = isAdminGuide ? adminOnboardingTips : userOnboardingTips;

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
    if (!guideActive) {
      return;
    }

    const timer = window.setInterval(() => {
      setActiveTipIndex((current) => (current + 1) % activeOnboardingTips.length);
    }, 4200);

    return () => window.clearInterval(timer);
  }, [activeOnboardingTips.length, guideActive]);

  React.useEffect(() => {
    setActiveTipIndex(0);
  }, [isAdminGuide]);

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

  const isBootstrapAdminSetup = Boolean(viewer?.mustResetPassword);
  const currentTimeZone = React.useMemo(() => detectCurrentTimeZone(), []);
  const welcomeTitle = isAdminGuide ? t("adminWelcomeTitle") : t("userWelcomeTitle");
  const welcomeDescription = isAdminGuide
    ? t("adminWelcomeDescription")
    : t("userWelcomeDescription");
  const accountTitle = isBootstrapAdminSetup ? t("bootstrapTitle") : isAdminGuide ? t("adminAccountTitle") : t("userAccountTitle");
  const twoFactorTitle = isAdminGuide ? t("adminTwoFactorTitle") : t("userTwoFactorTitle");
  const readyDescription = isBootstrapAdminSetup
    ? t("bootstrapReadyDescription")
    : isAdminGuide
      ? t("adminReadyDescription")
      : t("userReadyDescription");

  const submitAccountStep = React.useCallback(async () => {
    if (!viewer?.initialSecurityRequired || savingAccount) return;
    const nextUsername = username.trim().toLowerCase();
    const nextDisplayName = displayName.trim();
    const nextPassword = password.trim();
    if (viewer.initialUsernameRequired && !isUsernamePolicyValid(nextUsername)) {
      toast.error(t("toasts.usernameTooShort"));
      return;
    }
    if (viewer.initialUsernameRequired && nextUsername === viewer.username.trim().toLowerCase()) {
      toast.error(t("toasts.changeInitialUsername"));
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

  const copyText = React.useCallback(async (value: string, label: string) => {
    try {
      await writeClipboardText(value);
      toast.success(t("toasts.copied", { label }));
    } catch {
      toast.error(t("toasts.copyFailed"), { description: t("toasts.manualCopy") });
    }
  }, [t]);

  const handleLocaleChange = React.useCallback((nextLocale: AppLocale) => {
    if (nextLocale === locale) {
      return;
    }

    void setLocale(nextLocale);
  }, [locale, setLocale]);

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
      theme,
      preset,
    }),
    [preset, theme],
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
      toast.error(t("toasts.completeFailed"), {
        description: resolveErrorMessage(error, tCommonErrors("unknown")),
      });
    } finally {
      setFinishing(false);
    }
  }, [accessToken, finishing, password, refreshUser, resolveErrorMessage, t, tCommonErrors, twoFactorSkipped, viewer]);

  const copyRecoveryCodes = React.useCallback(async () => {
    try {
      await writeClipboardText(recoveryCodes.join("\n"));
      toast.success(t("toasts.recoveryCodesCopied"));
    } catch {
      toast.error(t("toasts.copyFailed"));
    }
  }, [recoveryCodes, t]);

  if (!viewer || (!guideActive && recoveryCodes.length === 0)) {
    return null;
  }

  return (
    <div className="fixed inset-0 z-50 flex min-h-svh items-center justify-center overflow-y-auto bg-background/20 px-3 py-5 backdrop-blur-[2px]">
      <Onboarding
        value={step}
        onValueChange={setStep}
        totalSteps={6}
        role="dialog"
        aria-modal="true"
        aria-label={t("aria.onboarding")}
        className="grid w-full max-w-[820px] animate-in gap-0 overflow-hidden rounded-2xl border border-border/60 bg-background p-0 shadow-xl fade-in-0 zoom-in-95 duration-200 md:h-[430px] md:grid-cols-[0.95fr_1.05fr]"
      >
        <div className="hidden h-full flex-col bg-muted/15 p-4 md:flex">
          <OnboardingFeatureCarousel activeIndex={activeTipIndex} logos={onboardingLogoItems} tips={activeOnboardingTips} />
        </div>

        <div className="flex h-full flex-col p-5">
          <div className="flex items-center justify-between gap-4">
            <AppLogo width={86} height={24} priority className="h-6 w-auto" />
            <Onboarding.StepIndicator variant="dots" dotClassName="bg-muted-foreground/25" />
          </div>

          <div className="flex flex-1">
          <Onboarding.Step step={1} className="flex flex-1 flex-col animate-in fade-in-0 slide-in-from-right-2 duration-200">
            <div className="flex flex-1 items-center">
              <div className="w-full space-y-6">
                <Onboarding.Header className="text-left">
                  <div className="space-y-3">
                    <h2 className="text-2xl font-semibold tracking-normal">{welcomeTitle}</h2>
                    <p className="text-sm leading-6 text-muted-foreground">
                      {welcomeDescription}
                    </p>
                  </div>
                </Onboarding.Header>

                <div className="w-full space-y-2">
                  <label className="text-xs font-medium text-muted-foreground" htmlFor="initial-locale-trigger">
                    {t("labels.language")}
                  </label>
                  <Select
                    value={locale}
                    disabled={Boolean(savingLocale)}
                    onValueChange={(value) => handleLocaleChange(value as AppLocale)}
                  >
                    <SelectTrigger id="initial-locale-trigger" aria-label={t("labels.language")} className="h-8 w-full">
                      <SelectValue />
                    </SelectTrigger>
                    <SelectContent>
                      {APP_LOCALES.map((item) => (
                        <SelectItem key={item} value={item}>
                          {APP_LOCALE_LABELS[item]}
                        </SelectItem>
                      ))}
                    </SelectContent>
                  </Select>
                </div>
              </div>
            </div>
            <Onboarding.Navigation aria-label={t("aria.welcomeNavigation")} className="mt-auto justify-end pt-6">
              <Button type="button" disabled={Boolean(savingLocale)} onClick={() => void saveWelcomeStep()}>
                {savingLocale ? <SpinnerLabel>{t("saving")}</SpinnerLabel> : t("start")}
              </Button>
            </Onboarding.Navigation>
          </Onboarding.Step>

          <Onboarding.Step step={2} className="flex flex-1 animate-in fade-in-0 slide-in-from-right-2 duration-200">
            <form
              className="flex flex-1 flex-col"
              autoComplete="on"
              onSubmit={(event) => {
                event.preventDefault();
                void submitAccountStep();
              }}
            >
              <div className="flex flex-1 items-center">
                <div className="w-full space-y-6">
                  <Onboarding.Header className="text-left">
                    <div className="space-y-2">
                      <h2 className="text-2xl font-semibold tracking-normal">
                        {accountTitle}
                      </h2>
                    </div>
                  </Onboarding.Header>

                  <div className="space-y-4">
                    <label className="block space-y-1.5" htmlFor="initial-username">
                      <span className="flex items-center text-xs font-medium">
                        {t("labels.username")}
                      </span>
                      <Input
                        id="initial-username"
                        name="username"
                        value={username}
                        onChange={(event) => setUsername(event.target.value.toLowerCase())}
                        disabled={savingAccount}
                        readOnly={!viewer.initialUsernameRequired}
                        maxLength={USERNAME_MAX_LENGTH}
                        autoComplete="username"
                        aria-disabled={!viewer.initialUsernameRequired}
                        placeholder={isBootstrapAdminSetup ? t("placeholders.adminUsername") : t("placeholders.username")}
                      />
                    </label>

                    {isBootstrapAdminSetup ? (
                      <label className="block space-y-1.5" htmlFor="initial-admin-password">
                        <span className="flex items-center text-xs font-medium">
                          {t("labels.password")}
                        </span>
                        <Input
                          id="initial-admin-password"
                          name="password"
                          type="password"
                          value={password}
                          onChange={(event) => setPassword(event.target.value)}
                          disabled={savingAccount || !viewer.mustResetPassword}
                          autoComplete="new-password"
                          minLength={PASSWORD_MIN_LENGTH}
                          placeholder={t("placeholders.adminPassword")}
                        />
                      </label>
                    ) : (
                      <label className="block space-y-1.5" htmlFor="initial-display-name">
                        <span className="flex items-center text-xs font-medium">
                          {t("labels.displayName")}
                        </span>
                        <Input
                          id="initial-display-name"
                          name="name"
                          value={displayName}
                          onChange={(event) => setDisplayName(event.target.value)}
                          disabled={savingAccount}
                          maxLength={DISPLAY_NAME_MAX_LENGTH}
                          autoComplete="name"
                          placeholder={t("placeholders.displayName")}
                        />
                      </label>
                    )}

                  </div>
                </div>
              </div>

              <Onboarding.Navigation aria-label={t("aria.accountNavigation")} className="mt-auto justify-end pt-6">
                <Button type="button" variant="ghost" className="shadow-none" disabled={savingAccount} onClick={() => setStep(1)}>
                  {t("back")}
                </Button>
                <Button type="submit" disabled={savingAccount}>
                  {savingAccount ? <SpinnerLabel>{t("saving")}</SpinnerLabel> : t("continue")}
                </Button>
              </Onboarding.Navigation>
            </form>
          </Onboarding.Step>

          <Onboarding.Step step={3} className="flex flex-1 flex-col animate-in fade-in-0 slide-in-from-right-2 duration-200">
            <div className="flex flex-1 items-center">
              <div className="w-full space-y-5">
                <Onboarding.Header className="text-left">
                  <div className="space-y-2">
                    <h2 className="text-2xl font-semibold tracking-normal">{twoFactorTitle}</h2>
                  </div>
                </Onboarding.Header>

                {!viewer.twoFactorAvailable ? (
                  <div className="rounded-lg border border-border/60 bg-muted/25 px-4 py-3 text-xs text-muted-foreground">
                    {t("states.twoFactorUnavailable")}
                  </div>
                ) : viewer.twoFactorEnabled ? (
                  <div className="flex items-center gap-2 rounded-lg border border-border/60 bg-muted/25 px-4 py-3 text-xs font-medium">
                    <ShieldCheck className="size-3.5 text-muted-foreground" />
                    {t("states.twoFactorEnabled")}
                  </div>
                ) : savingTwoFactor && !twoFactorSetup ? (
                  <div className="flex min-h-[7.5rem] items-center justify-center rounded-lg border border-border/60 bg-muted/20 text-xs text-muted-foreground">
                    <SpinnerLabel>{t("generating")}</SpinnerLabel>
                  </div>
                ) : !twoFactorSetup ? (
                  <div className="rounded-lg border border-border/60 bg-muted/25 px-4 py-3 text-xs text-muted-foreground">
                    {t("states.twoFactorPreparing")}
                  </div>
                ) : (
                  <div className="grid items-center gap-5 sm:grid-cols-[7.5rem_minmax(0,1fr)]">
                    <div className="flex min-h-[7.5rem] items-center justify-center">
                      {savingTwoFactor && !qrCodeSVG ? (
                        <div className="flex size-[7.5rem] items-center justify-center rounded-lg border border-border/60 bg-muted/20 text-xs text-muted-foreground">
                          <SpinnerLabel>{t("generating")}</SpinnerLabel>
                        </div>
                      ) : qrCodeSVG ? (
                        <div
                          className="flex size-[7.5rem] items-center justify-center [&_svg]:size-full"
                          dangerouslySetInnerHTML={{ __html: qrCodeSVG }}
                        />
                      ) : (
                        <div className="flex size-[7.5rem] items-center justify-center rounded-lg border border-border/60 bg-muted/20 px-3 text-center text-[11px] leading-4 text-muted-foreground">
                          {qrCodeUnavailable ? t("states.qrUnavailable") : <SpinnerLabel>{t("generating")}</SpinnerLabel>}
                        </div>
                      )}
                    </div>

                    <div className="min-w-0 space-y-2.5">
                      {twoFactorSetup?.secret ? (
                        <div className="space-y-1.5">
                          <span className="text-xs font-medium">{t("labels.manualSecret")}</span>
                          <div className="flex items-center gap-2">
                            <span className="min-w-0 flex-1 break-all font-mono text-[11px] leading-5 text-muted-foreground">
                              {twoFactorSetup.secret}
                            </span>
                            <Button
                              type="button"
                              variant="ghost"
                              size="icon-sm"
                              className="shrink-0 text-muted-foreground shadow-none"
                              onClick={() => void copyText(twoFactorSetup.secret, t("toasts.secret"))}
                              aria-label={t("actions.copySecret")}
                              title={t("actions.copySecretTitle")}
                            >
                              <Copy className="size-3.5" />
                            </Button>
                          </div>
                        </div>
                      ) : null}

                      <label className="block space-y-1.5">
                        <span className="text-xs font-medium">{t("labels.otp")}</span>
                        <Input
                          type="text"
                          inputMode="numeric"
                          autoComplete="one-time-code"
                          pattern="[0-9]*"
                          placeholder={t("placeholders.otp")}
                          value={otp}
                          maxLength={6}
                          className="h-8 text-xs"
                          onInput={(event) => setOtp(event.currentTarget.value.replace(/\D/g, "").slice(0, 6))}
                          onChange={(event) => setOtp(event.target.value.replace(/\D/g, "").slice(0, 6))}
                        />
                      </label>
                    </div>
                  </div>
                )}
                </div>
            </div>

            <Onboarding.Navigation aria-label={t("aria.twoFactorNavigation")} className="mt-auto justify-end pt-6">
              <Button type="button" variant="ghost" className="shadow-none" disabled={savingTwoFactor} onClick={() => void skipTwoFactor()}>
                {t("skip")}
              </Button>
              <Button
                type="button"
                disabled={savingTwoFactor || !viewer.twoFactorAvailable || viewer.twoFactorEnabled || !twoFactorSetup}
                onClick={() => void confirmTwoFactor()}
              >
                {savingTwoFactor ? <SpinnerLabel>{t("processing")}</SpinnerLabel> : t("enable")}
              </Button>
            </Onboarding.Navigation>
          </Onboarding.Step>

          <Onboarding.Step step={4} className="flex flex-1 flex-col animate-in fade-in-0 slide-in-from-right-2 duration-200">
            <div className="flex flex-1 items-center">
              <div className="w-full space-y-5">
                <Onboarding.Header className="text-left">
                  <div className="space-y-2">
                    <h2 className="text-2xl font-semibold tracking-normal">{t("labels.themePreset")}</h2>
                    <p className="text-xs leading-5 text-muted-foreground">
                      {t("themePresetDescription")}
                    </p>
                  </div>
                </Onboarding.Header>

                <div className="grid grid-cols-2 gap-2 sm:grid-cols-4">
                  {onboardingThemePresets.map((item) => (
                    <Button
                      key={item}
                      type="button"
                      variant="outline"
                      className={cn(
                        "h-8 justify-center px-2 text-xs shadow-none",
                        preset === item && "border-primary/45 bg-muted text-foreground hover:bg-muted hover:text-foreground",
                      )}
                      onClick={() => {
                        setPreset(item);
                      }}
                    >
                      {t(`themePreset.${item}`)}
                    </Button>
                  ))}
                </div>
              </div>
            </div>

            <Onboarding.Navigation aria-label={t("aria.themePresetNavigation")} className="mt-auto justify-end pt-6">
              <Button type="button" variant="ghost" className="shadow-none" disabled={savingThemePreset} onClick={() => setStep(3)}>
                {t("back")}
              </Button>
              <Button type="button" disabled={savingThemePreset} onClick={() => void saveThemePresetStep()}>
                {savingThemePreset ? <SpinnerLabel>{t("saving")}</SpinnerLabel> : t("continue")}
              </Button>
            </Onboarding.Navigation>
          </Onboarding.Step>

          <Onboarding.Step step={5} className="flex flex-1 flex-col animate-in fade-in-0 slide-in-from-right-2 duration-200">
            <div className="flex flex-1 items-center">
              <div className="w-full space-y-5">
                <Onboarding.Header className="text-left">
                  <div className="space-y-2">
                    <h2 className="text-2xl font-semibold tracking-normal">{t("personalizationTitle")}</h2>
                    <p className="text-xs leading-5 text-muted-foreground">
                      {t("personalizationDescription")}
                    </p>
                  </div>
                </Onboarding.Header>

                <div className="space-y-4">
                  <label className="block space-y-1.5" htmlFor="initial-timezone">
                    <span className="flex items-center text-xs font-medium">
                      {t("labels.region")}
                    </span>
                    <div className="flex gap-1.5">
                      <TimeZoneSelect
                        id="initial-timezone"
                        value={timezone || currentTimeZone}
                        disabled={savingPersonalization}
                        triggerClassName="h-8 min-w-0 flex-1 text-xs"
                        valueClassName="text-xs"
                        onChange={setTimezone}
                      />
                      <Button
                        type="button"
                        variant="ghost"
                        size="icon"
                        className="h-8 w-8 shrink-0 shadow-none"
                        disabled={savingPersonalization || timezone === currentTimeZone}
                        onClick={() => setTimezone(currentTimeZone)}
                        aria-label={t("actions.syncTimezone")}
                        title={t("actions.syncTimezone")}
                      >
                        <MapPinned className="size-3.5 stroke-1" />
                      </Button>
                    </div>
                  </label>

                  <div className="space-y-1.5">
                    <span className="flex items-center text-xs font-medium">
                      {t("labels.theme")}
                    </span>
                    <div className="grid grid-cols-3 gap-2">
                      {([
                        ["light", Sun],
                        ["system", Monitor],
                        ["dark", Moon],
                      ] as const).map(([mode, Icon]) => (
                        <Button
                          key={mode}
                          type="button"
                          variant="outline"
                          className={cn(
                            "h-8 justify-center gap-1.5 px-2 text-xs shadow-none",
                            theme === mode && "border-foreground/80 bg-muted text-foreground hover:bg-muted hover:text-foreground",
                          )}
                          onClick={() => {
                            setTheme(mode);
                          }}
                        >
                          <Icon className="size-3.5 stroke-1" />
                          {t(`theme.${mode}`)}
                        </Button>
                      ))}
                    </div>
                  </div>
                </div>
              </div>
            </div>

            <Onboarding.Navigation aria-label={t("aria.personalizationNavigation")} className="mt-auto justify-end pt-6">
              <Button type="button" variant="ghost" className="shadow-none" disabled={savingPersonalization} onClick={() => setStep(4)}>
                {t("back")}
              </Button>
              <Button type="button" disabled={savingPersonalization} onClick={() => void savePersonalizationStep()}>
                {savingPersonalization ? <SpinnerLabel>{t("saving")}</SpinnerLabel> : t("continue")}
              </Button>
            </Onboarding.Navigation>
          </Onboarding.Step>

          <Onboarding.Step step={6} className="flex flex-1 flex-col animate-in fade-in-0 slide-in-from-right-2 duration-200">
            <div className="flex flex-1 items-center">
              <div className="w-full space-y-5">
                <Onboarding.Header className="text-left">
                  <div className="space-y-2">
                    <h2 className="text-2xl font-semibold tracking-normal">{t("ready")}</h2>
                    <p className="text-xs text-muted-foreground">
                      {readyDescription}
                    </p>
                  </div>
                </Onboarding.Header>

                {recoveryCodes.length > 0 ? (
                  <div className="space-y-2 rounded-lg border border-border/60 bg-muted/20 p-3">
                    <div className="flex items-center justify-between gap-2">
                      <p className="text-xs font-medium">{t("labels.recoveryCodes")}</p>
                      <Button type="button" variant="ghost" size="icon" className="size-7 shadow-none" onClick={() => void copyRecoveryCodes()} aria-label={t("actions.copyRecoveryCodes")}>
                        <Copy className="size-3.5" />
                      </Button>
                    </div>
                    <div className="grid grid-cols-2 gap-1 font-mono text-[11px] text-muted-foreground">
                      {recoveryCodes.map((code) => (
                        <span key={code}>{code}</span>
                      ))}
                    </div>
                  </div>
                ) : null}
              </div>
            </div>

            <Onboarding.Navigation aria-label={t("aria.finishNavigation")} className="mt-auto justify-end pt-6">
              <Button type="button" variant="ghost" className="shadow-none" disabled={finishing} onClick={() => setStep(5)}>
                {t("back")}
              </Button>
              <Button type="button" disabled={finishing} onClick={() => void finishInitialSecurity()}>
                {finishing ? <SpinnerLabel>{t("finishing")}</SpinnerLabel> : t("finish")}
              </Button>
            </Onboarding.Navigation>
          </Onboarding.Step>
          </div>
        </div>
      </Onboarding>
    </div>
  );
}
