"use client";

import { useTranslations } from "next-intl";

import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { SpinnerLabel } from "@/components/ui/spinner";
import { PASSWORD_MIN_LENGTH } from "@/shared/auth/account-policy";
import { useLoginPage } from "@/features/auth/hooks/use-auth-login-page";
import { AppLogo } from "@/shared/components/app-logo";
import { IdentityProviderIcon } from "@/shared/components/identity-provider-icon";
import { PoweredByDeeix } from "@/shared/components/powered-by-deeix";
import { brandAssets } from "@/shared/lib/branding";
import { TurnstileWidget } from "@/features/auth/components/turnstile-widget";
import { cn } from "@/lib/utils";

type LoginPageProps = {
  nextPath: string;
};

function LoginBrandMark() {
  return (
    <AppLogo
      width={32}
      height={32}
      priority
      className="mx-auto h-9 w-auto"
    />
  );
}

export function LoginPage({ nextPath }: LoginPageProps) {
  const t = useTranslations("login");
  const loginPage = useLoginPage({ nextPath });
  const {
    cancelTwoFactorChallenge,
    canShowRegisterSwitch,
    codeSent,
    configReady,
    emailRegistrationEnabled,
    emailVerificationEnabled,
    handleProviderLogin,
    loginProviders,
    mode,
    onLoginSubmit,
    onRegisterSubmit,
    options,
    password,
    passwordLoginEnabled,
    passwordResetEnabled,
    registerCode,
    registerCodeCooldownSeconds,
    registerDebugCode,
    registerEmail,
    registerPassword,
    registerTurnstileRequired,
    registerTurnstileResetSignal,
    registerTurnstileSiteKey,
    registerTurnstileToken,
    requestRegisterCode,
    requestPasswordResetCode,
    requestTwoFactorEmailCode,
    resetCode,
    resetCodeCooldownSeconds,
    resetCodeSent,
    resetEmail,
    resetPassword,
    sendingCode,
    setPassword,
    setRegisterCode,
    setRegisterPassword,
    setRegisterTurnstileToken,
    setResetCode,
    setResetPassword,
    setTwoFactorCode,
    switchTwoFactorVerificationMethod,
    setUsername,
    submitting,
    toggleLoginMode,
    twoFactorChallengeToken,
    twoFactorCode,
    twoFactorEmailCodeCooldownSeconds,
    twoFactorEmailDebugCode,
    twoFactorVerificationMethod,
    twoFactorVerificationMethods,
    updateResetEmail,
    updateRegisterEmail,
    username,
    onPasswordResetSubmit,
  } = loginPage;

  const accountLabel = options.emailEnabled && options.usernameEnabled
    ? t("account")
    : options.emailEnabled
      ? t("email")
      : t("username");
  const accountPlaceholder = options.emailEnabled && options.usernameEnabled
    ? t("emailOrUsername")
    : options.emailEnabled
      ? t("email")
      : t("username");
  const alternativeTwoFactorMethod = twoFactorVerificationMethods.find((method) => method !== twoFactorVerificationMethod && method !== "none");
  const twoFactorUsesEmail = twoFactorVerificationMethod === "email";

  return (
    <main className="flex min-h-screen items-center justify-center px-4 py-8 text-foreground" aria-busy={!configReady}>
      <div className="w-full max-w-[360px]">
        <LoginBrandMark />

        <div
          aria-hidden={!configReady}
          className={cn(
            "grid transition-[grid-template-rows,opacity] duration-200 ease-out",
            configReady ? "grid-rows-[1fr] opacity-100" : "pointer-events-none grid-rows-[0fr] opacity-0",
          )}
        >
          {configReady ? (
            <div className="min-h-0 overflow-hidden px-2">
            {mode === "login" && twoFactorChallengeToken ? (
              <>
                <form className="mt-7 space-y-4" onSubmit={onLoginSubmit}>
                  <div className="space-y-2">
                    <label className="text-sm font-medium leading-none text-foreground" htmlFor="otp">
                      {twoFactorUsesEmail ? t("verificationCode") : t("twoFactorCode")}
                    </label>
                    <div className="flex gap-2">
                      <Input
                        id="otp"
                        name="otp"
                        type="text"
                        inputMode={twoFactorUsesEmail ? "numeric" : "text"}
                        autoComplete="one-time-code"
                        pattern={twoFactorUsesEmail ? "[0-9]*" : undefined}
                        className="h-9 min-w-0 border-input/50"
                        placeholder={twoFactorUsesEmail ? t("verificationCodePlaceholder") : t("twoFactorPlaceholder")}
                        value={twoFactorCode}
                        onChange={(event) => setTwoFactorCode(event.target.value)}
                        required
                      />
                      {twoFactorUsesEmail ? (
                        <Button
                          type="button"
                          variant="secondary"
                          className="h-9 min-w-[4.5rem] shrink-0 rounded-md border-0 bg-muted px-3 text-sm font-semibold text-foreground shadow-none hover:bg-muted/80"
                          disabled={sendingCode || twoFactorEmailCodeCooldownSeconds > 0}
                          onClick={() => {
                            void requestTwoFactorEmailCode();
                          }}
                        >
                          {sendingCode ? <SpinnerLabel>{t("sending")}</SpinnerLabel> : twoFactorEmailCodeCooldownSeconds > 0 ? t("resendIn", { seconds: twoFactorEmailCodeCooldownSeconds }) : t("send")}
                        </Button>
                      ) : null}
                    </div>
                    {twoFactorEmailDebugCode ? <p className="text-xs font-medium text-muted-foreground">{t("debugCode", { code: twoFactorEmailDebugCode })}</p> : null}
                  </div>
                  <Button
                    className="mt-1 h-9 w-full rounded-md bg-foreground text-sm font-semibold text-background shadow-none hover:bg-foreground/90"
                    type="submit"
                    disabled={submitting}
                  >
                    {submitting ? <SpinnerLabel>{t("signingIn")}</SpinnerLabel> : t("verifyAndSignIn")}
                  </Button>
                </form>
                {alternativeTwoFactorMethod ? (
                  <Button
                    type="button"
                    variant="ghost"
                    className="mt-2 h-9 w-full text-xs text-muted-foreground shadow-none"
                    onClick={() => switchTwoFactorVerificationMethod(alternativeTwoFactorMethod)}
                  >
                    {alternativeTwoFactorMethod === "email" ? t("useEmailVerification") : t("useTwoFactorVerification")}
                  </Button>
                ) : null}
                <Button
                  type="button"
                  variant="ghost"
                  className="mt-2 h-9 w-full text-xs text-muted-foreground shadow-none"
                  onClick={cancelTwoFactorChallenge}
                >
                  {passwordLoginEnabled ? t("backToPasswordLogin") : t("backToLoginMethods")}
                </Button>
              </>
            ) : null}

            {mode === "login" && !twoFactorChallengeToken && passwordLoginEnabled ? (
              <form className="mt-7 space-y-4" onSubmit={onLoginSubmit}>
                <div className="space-y-2">
                  <label className="text-sm font-medium leading-none text-foreground" htmlFor="username">
                    {accountLabel}
                  </label>
                  <Input
                    id="username"
                    name="username"
                    autoComplete="username"
                    className="h-9 border-input/50"
                    placeholder={accountPlaceholder}
                    value={username}
                    onChange={(event) => setUsername(event.target.value)}
                    required
                  />
                </div>
                <div className="space-y-2">
                  <div className="flex items-center justify-between gap-3">
                    <label className="text-sm font-medium leading-none text-foreground" htmlFor="password">
                      {t("password")}
                    </label>
                    {passwordResetEnabled ? (
                      <button
                        type="button"
                        className="text-xs font-medium text-muted-foreground underline-offset-4 transition-colors hover:text-foreground hover:underline focus-visible:text-foreground focus-visible:underline focus-visible:outline-none"
                        onClick={() => {
                          updateResetEmail(username.includes("@") ? username : "");
                          loginPage.setMode("reset-password");
                        }}
                      >
                        {t("forgotPassword")}
                      </button>
                    ) : null}
                  </div>
                  <Input
                    id="password"
                    name="password"
                    type="password"
                    autoComplete="current-password"
                    className="h-9 border-input/50"
                    placeholder={t("password")}
                    value={password}
                    onChange={(event) => setPassword(event.target.value)}
                    required
                  />
                </div>
                <Button
                  className="mt-2 h-9 w-full rounded-md bg-foreground text-sm font-semibold text-background shadow-none hover:bg-foreground/90"
                  type="submit"
                  disabled={submitting}
                >
                  {submitting ? <SpinnerLabel>{t("signingIn")}</SpinnerLabel> : t("signIn")}
                </Button>
              </form>
            ) : null}

            {mode === "reset-password" && passwordResetEnabled ? (
              <>
                <form className="mt-7 space-y-4" onSubmit={onPasswordResetSubmit}>
                  <div className="space-y-2">
                    <label className="text-sm font-medium leading-none text-foreground" htmlFor="reset-email">
                      {t("email")}
                    </label>
                    <Input
                      id="reset-email"
                      type="email"
                      autoComplete="email"
                      className="h-9 border-input/50"
                      placeholder={t("email")}
                      value={resetEmail}
                      onChange={(event) => updateResetEmail(event.target.value)}
                      required
                    />
                  </div>
                  <div className="space-y-2">
                    <label className="text-sm font-medium leading-none text-foreground" htmlFor="reset-password">
                      {t("newPassword")}
                    </label>
                    <Input
                      id="reset-password"
                      type="password"
                      autoComplete="new-password"
                      className="h-9 border-input/50"
                      placeholder={t("newPasswordPlaceholder")}
                      value={resetPassword}
                      onChange={(event) => setResetPassword(event.target.value)}
                      minLength={PASSWORD_MIN_LENGTH}
                      required
                    />
                  </div>
                  <div className="space-y-2">
                    <label className="text-sm font-medium leading-none text-foreground" htmlFor="reset-code">
                      {t("verificationCode")}
                    </label>
                    <div className="flex gap-2">
                      <Input
                        id="reset-code"
                        inputMode="numeric"
                        autoComplete="one-time-code"
                        className="h-9 border-input/50"
                        placeholder={t("verificationCodePlaceholder")}
                        value={resetCode}
                        onChange={(event) => setResetCode(event.target.value)}
                        required
                      />
                      <Button
                        type="button"
                        variant="secondary"
                        className="h-9 min-w-[4.5rem] shrink-0 rounded-md border-0 bg-muted px-3 text-sm font-semibold text-foreground shadow-none hover:bg-muted/80"
                        disabled={sendingCode || resetCodeCooldownSeconds > 0 || !resetEmail.trim()}
                        onClick={() => {
                          void requestPasswordResetCode();
                        }}
                      >
                        {sendingCode ? <SpinnerLabel>{t("sending")}</SpinnerLabel> : resetCodeCooldownSeconds > 0 ? t("resendIn", { seconds: resetCodeCooldownSeconds }) : resetCodeSent ? t("resend") : t("send")}
                      </Button>
                    </div>
                  </div>
                  <Button
                    className="mt-1 h-9 w-full rounded-md bg-foreground text-sm font-semibold text-background shadow-none hover:bg-foreground/90"
                    type="submit"
                    disabled={submitting || resetCode.length !== 6}
                  >
                    {submitting ? <SpinnerLabel>{t("resettingPassword")}</SpinnerLabel> : t("resetPassword")}
                  </Button>
                </form>
                <div className="mt-6 text-center text-sm font-normal leading-5 text-muted-foreground">
                  {t("rememberPassword")}{" "}
                  <button
                    type="button"
                    className="font-semibold text-foreground underline-offset-4 hover:underline focus-visible:underline focus-visible:outline-none"
                    onClick={() => loginPage.setMode("login")}
                  >
                    {t("back")}
                  </button>
                </div>
              </>
            ) : null}

            {mode === "register" && emailRegistrationEnabled ? (
              <form className="mt-7 space-y-4" onSubmit={onRegisterSubmit}>
                <div className="space-y-2">
                  <label className="text-sm font-medium leading-none text-foreground" htmlFor="register-email">
                    {t("email")}
                  </label>
                  <Input
                    id="register-email"
                    type="email"
                    autoComplete="email"
                    className="h-9 border-input/50"
                    placeholder={t("email")}
                    value={registerEmail}
                    onChange={(event) => updateRegisterEmail(event.target.value)}
                    required
                  />
                </div>
                <div className="space-y-2">
                  <label className="text-sm font-medium leading-none text-foreground" htmlFor="register-password">
                    {t("password")}
                  </label>
                  <Input
                    id="register-password"
                    type="password"
                    autoComplete="new-password"
                    className="h-9 border-input/50"
                    placeholder={t("newPasswordPlaceholder")}
                    value={registerPassword}
                    onChange={(event) => setRegisterPassword(event.target.value)}
                    minLength={PASSWORD_MIN_LENGTH}
                    required
                  />
                </div>
                {registerTurnstileRequired ? (
                  <TurnstileWidget
                    siteKey={registerTurnstileSiteKey}
                    resetSignal={registerTurnstileResetSignal}
                    onTokenChange={setRegisterTurnstileToken}
                  />
                ) : null}
                {emailVerificationEnabled ? (
                  <div className="space-y-2">
                    <label className="text-sm font-medium leading-none text-foreground" htmlFor="register-code">
                      {t("verificationCode")}
                    </label>
                    <div className="flex gap-2">
                      <Input
                        id="register-code"
                        inputMode="numeric"
                        autoComplete="one-time-code"
                        className="h-9 border-input/50"
                        placeholder={t("verificationCodePlaceholder")}
                        value={registerCode}
                        onChange={(event) => setRegisterCode(event.target.value)}
                        required
                      />
                      <Button
                        type="button"
                        variant="secondary"
                        className="h-9 min-w-[4.5rem] shrink-0 rounded-md border-0 bg-muted px-3 text-sm font-semibold text-foreground shadow-none hover:bg-muted/80"
                        disabled={sendingCode || registerCodeCooldownSeconds > 0 || !registerEmail.trim() || (registerTurnstileRequired && !registerTurnstileToken)}
                        onClick={() => {
                          void requestRegisterCode();
                        }}
                      >
                        {sendingCode ? <SpinnerLabel>{t("sending")}</SpinnerLabel> : registerCodeCooldownSeconds > 0 ? t("resendIn", { seconds: registerCodeCooldownSeconds }) : codeSent ? t("resend") : t("send")}
                      </Button>
                    </div>
                  </div>
                ) : null}
                {registerDebugCode ? <p className="text-xs font-medium text-muted-foreground">{t("debugCode", { code: registerDebugCode })}</p> : null}
                <Button
                  className="mt-1 h-9 w-full rounded-md bg-foreground text-sm font-semibold text-background shadow-none hover:bg-foreground/90"
                  type="submit"
                  disabled={submitting || (emailVerificationEnabled && registerCode.length !== 6) || (registerTurnstileRequired && !registerTurnstileToken)}
                >
                  {submitting ? <SpinnerLabel>{t("registering")}</SpinnerLabel> : t("register")}
                </Button>
              </form>
            ) : null}

            {mode === "login" && !twoFactorChallengeToken && loginProviders.length > 0 ? (
              <div className={cn("space-y-2.5", passwordLoginEnabled ? "mt-5" : "mt-7")}>
                {loginProviders.map((provider) => (
                  <Button
                    key={provider.publicID}
                    type="button"
                    variant="secondary"
                    className="h-9 w-full rounded-md border-0 bg-muted text-sm font-semibold text-foreground shadow-none hover:bg-muted/80"
                    onClick={() => {
                      void handleProviderLogin(provider.slug);
                    }}
                  >
                    <span className="inline-flex min-w-0 items-center justify-center gap-2">
                      <IdentityProviderIcon
                        name={provider.name}
                        slug={provider.slug}
                        logoURL={provider.logoURL}
                        className="size-5"
                        iconClassName="size-5"
                        fallbackClassName="text-sm font-semibold uppercase text-foreground"
                      />
                      <span className="truncate">{t("providerLogin", { provider: provider.name })}</span>
                    </span>
                  </Button>
                ))}
              </div>
            ) : null}

            {canShowRegisterSwitch && mode !== "reset-password" ? (
              <div className="mt-6 text-center text-sm font-normal leading-5 text-muted-foreground">
                {mode === "register" ? t("alreadyHaveAccount") : t("noAccount")}{" "}
                <button
                  type="button"
                  className="font-semibold text-foreground underline-offset-4 hover:underline focus-visible:underline focus-visible:outline-none"
                  onClick={toggleLoginMode}
                >
                  {mode === "register" ? t("signIn") : t("register")}
                </button>
              </div>
            ) : null}
            </div>
          ) : null}
        </div>

      </div>

      {brandAssets.logo ? (
        <div className="fixed bottom-4 right-4">
          <PoweredByDeeix />
        </div>
      ) : null}
    </main>
  );
}
