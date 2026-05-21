"use client";

import * as React from "react";
import { Copy } from "lucide-react";
import { useTranslations } from "next-intl";
import { toast } from "sonner";

import { Button } from "@/components/ui/button";
import { Dialog, DialogContent, DialogDescription, DialogFooter, DialogHeader, DialogTitle } from "@/components/ui/dialog";
import { Input } from "@/components/ui/input";
import { SpinnerLabel } from "@/components/ui/spinner";
import { formatDateTime } from "@/features/settings/model/account-page";
import { useAppLocale } from "@/i18n/app-i18n-provider";
import type { SecurityVerificationMethod } from "@/shared/api/auth.types";
import { PASSWORD_MIN_LENGTH, isPasswordPolicyValid } from "@/shared/auth/account-policy";
import { createQRCodeSVG } from "@/shared/lib/qr-code";

function sanitizeVerificationCode(method: SecurityVerificationMethod, value: string) {
  if (method === "two_factor") {
    return value.replace(/[^a-zA-Z0-9-]/g, "").slice(0, 32);
  }
  return value.replace(/\D/g, "").slice(0, 6);
}

function isVerificationCodeReady(method: SecurityVerificationMethod, value: string) {
  if (method === "none") {
    return true;
  }
  if (method === "two_factor") {
    return value.trim().length >= 6;
  }
  return value.trim().length === 6;
}

export function SecurityVerificationDialog({
  open,
  onOpenChange,
  selectedMethod,
  availableMethods,
  onMethodChange,
  title,
  description,
  debugCode,
  pending,
  sendingCode,
  resendCooldownSeconds = 0,
  onSendCode,
  onSubmit,
}: {
  open: boolean;
  onOpenChange: (open: boolean) => void;
  selectedMethod: SecurityVerificationMethod;
  availableMethods: SecurityVerificationMethod[];
  onMethodChange: (method: SecurityVerificationMethod) => void;
  title: string;
  description: string;
  debugCode: string;
  pending: boolean;
  sendingCode: boolean;
  resendCooldownSeconds?: number;
  onSendCode?: (method: SecurityVerificationMethod) => Promise<void>;
  onSubmit: (code: string, method: SecurityVerificationMethod) => Promise<void>;
}) {
  const common = useTranslations("settings.accountPage.securityDialog.common");
  const verification = useTranslations("settings.accountPage.securityDialog.verification");
  const [code, setCode] = React.useState("");
  const disabled = pending || sendingCode;
  const resendDisabled = disabled || resendCooldownSeconds > 0;
  const usesTwoFactor = selectedMethod === "two_factor";
  const alternativeMethod = availableMethods.find((item) => item !== selectedMethod && item !== "none");

  React.useEffect(() => {
    if (!open) {
      setCode("");
    }
  }, [open]);

  React.useEffect(() => {
    setCode("");
  }, [selectedMethod]);

  if (selectedMethod === "none") {
    return null;
  }

  return (
    <Dialog open={open} onOpenChange={onOpenChange}>
      <DialogContent className="sm:max-w-[440px]">
        <DialogHeader>
          <DialogTitle>{title}</DialogTitle>
          <DialogDescription>{description}</DialogDescription>
        </DialogHeader>
        <div className="space-y-1">
          <p className="text-xs text-muted-foreground">
            {usesTwoFactor ? verification("labels.twoFactorCode") : verification("labels.emailCode")}
          </p>
          <div className="flex gap-2">
            <Input
              type="text"
              name={usesTwoFactor ? "otp" : undefined}
              inputMode={usesTwoFactor ? "text" : "numeric"}
              autoComplete="one-time-code"
              pattern={usesTwoFactor ? undefined : "[0-9]*"}
              placeholder={usesTwoFactor ? verification("placeholders.otpOrRecovery") : verification("placeholders.sixDigitCode")}
              value={code}
              onChange={(event) => setCode(sanitizeVerificationCode(selectedMethod, event.target.value))}
              disabled={disabled}
              className="min-w-0"
            />
            {selectedMethod === "email" && onSendCode ? (
              <Button
                type="button"
                variant="secondary"
                className="min-w-[4.5rem] shrink-0 px-3 shadow-none"
                disabled={resendDisabled}
                onClick={() => void onSendCode(selectedMethod)}
              >
                {sendingCode ? <SpinnerLabel>{common("sending")}</SpinnerLabel> : resendCooldownSeconds > 0 ? common("resendIn", { seconds: resendCooldownSeconds }) : common("sendCode")}
              </Button>
            ) : null}
          </div>
          {debugCode ? <p className="text-xs font-medium text-muted-foreground">{common("debugCode", { code: debugCode })}</p> : null}
          {alternativeMethod ? (
            <div className="flex justify-center pt-2">
              <Button
                type="button"
                variant="link"
                className="h-auto px-0 text-xs font-medium text-muted-foreground hover:text-foreground"
                disabled={disabled}
                onClick={() => onMethodChange(alternativeMethod)}
              >
                {alternativeMethod === "email" ? verification("actions.useEmail") : verification("actions.useTwoFactor")}
              </Button>
            </div>
          ) : null}
        </div>
        <DialogFooter>
          <Button variant="ghost" disabled={pending} onClick={() => onOpenChange(false)}>{common("cancel")}</Button>
          <Button
            type="button"
            disabled={disabled || !isVerificationCodeReady(selectedMethod, code)}
            onClick={() => void onSubmit(code, selectedMethod)}
          >
            {pending ? <SpinnerLabel>{common("saving")}</SpinnerLabel> : common("verify")}
          </Button>
        </DialogFooter>
      </DialogContent>
    </Dialog>
  );
}

function EmailChangeVerificationDialog({
  open,
  onOpenChange,
  bootstrap,
  email,
  onEmailChange,
  emailVerificationEnabled,
  currentVerificationMethod,
  currentVerificationMethods,
  onCurrentVerificationMethodChange,
  pending,
  sendingCode,
  currentCodeCooldownSeconds,
  newCodeCooldownSeconds,
  debugCode,
  currentDebugCode,
  onSendCurrentCode,
  onSendNewCode,
  onSubmit,
}: {
  open: boolean;
  onOpenChange: (open: boolean) => void;
  bootstrap: boolean;
  email: string;
  onEmailChange: (email: string) => void;
  emailVerificationEnabled: boolean;
  currentVerificationMethod: SecurityVerificationMethod;
  currentVerificationMethods: SecurityVerificationMethod[];
  onCurrentVerificationMethodChange: (method: SecurityVerificationMethod) => void;
  pending: boolean;
  sendingCode: boolean;
  currentCodeCooldownSeconds: number;
  newCodeCooldownSeconds: number;
  debugCode: string;
  currentDebugCode: string;
  onSendCurrentCode: (method: SecurityVerificationMethod) => Promise<void>;
  onSendNewCode: () => Promise<void>;
  onSubmit: (payload: { currentVerificationMethod: SecurityVerificationMethod; currentCode: string; newCode: string }) => Promise<void>;
}) {
  const common = useTranslations("settings.accountPage.securityDialog.common");
  const verification = useTranslations("settings.accountPage.securityDialog.verification");
  const [currentCode, setCurrentCode] = React.useState("");
  const [newCode, setNewCode] = React.useState("");
  const [step, setStep] = React.useState<"current" | "email">("email");
  const disabled = pending || sendingCode;
  const currentResendDisabled = disabled || currentCodeCooldownSeconds > 0;
  const newResendDisabled = disabled || newCodeCooldownSeconds > 0;
  const needsCurrentVerification = !bootstrap && currentVerificationMethod !== "none";
  const isCurrentStep = needsCurrentVerification && step === "current";
  const usesTwoFactor = currentVerificationMethod === "two_factor";
  const alternativeCurrentMethod = currentVerificationMethods.find((item) => item !== currentVerificationMethod && item !== "none");
  const submitDisabled = disabled
    || !email.trim()
    || (needsCurrentVerification && !isVerificationCodeReady(currentVerificationMethod, currentCode))
    || (emailVerificationEnabled && newCode.length !== 6);

  React.useEffect(() => {
    if (!open) {
      setCurrentCode("");
      setNewCode("");
      setStep("email");
      return;
    }
    setStep(needsCurrentVerification ? "current" : "email");
  }, [needsCurrentVerification, open]);

  React.useEffect(() => {
    setCurrentCode("");
  }, [currentVerificationMethod]);

  return (
    <Dialog open={open} onOpenChange={onOpenChange}>
      <DialogContent className="sm:max-w-[460px]">
        <DialogHeader>
          <DialogTitle>{bootstrap ? verification("title.emailBootstrap") : verification("title.emailChange")}</DialogTitle>
          <DialogDescription>{bootstrap ? verification("description.emailBootstrap") : verification("description.emailChange")}</DialogDescription>
        </DialogHeader>
        <div className="space-y-4">
          {isCurrentStep ? (
            <div className="space-y-1">
              <p className="text-xs text-muted-foreground">
                {usesTwoFactor ? verification("labels.twoFactorCode") : verification("labels.currentEmailCode")}
              </p>
              <div className="flex gap-2">
                <Input
                  type="text"
                  name={usesTwoFactor ? "otp" : undefined}
                  value={currentCode}
                  inputMode={usesTwoFactor ? "text" : "numeric"}
                  autoComplete="one-time-code"
                  pattern={usesTwoFactor ? undefined : "[0-9]*"}
                  placeholder={usesTwoFactor ? verification("placeholders.otpOrRecovery") : verification("placeholders.sixDigitCode")}
                  disabled={disabled}
                  onChange={(event) => setCurrentCode(sanitizeVerificationCode(currentVerificationMethod, event.target.value))}
                  className="min-w-0"
                />
                {currentVerificationMethod === "email" ? (
                  <Button type="button" variant="secondary" className="min-w-[4.5rem] shrink-0 px-3 shadow-none" disabled={currentResendDisabled} onClick={() => void onSendCurrentCode(currentVerificationMethod)}>
                    {sendingCode ? <SpinnerLabel>{common("sending")}</SpinnerLabel> : currentCodeCooldownSeconds > 0 ? common("resendIn", { seconds: currentCodeCooldownSeconds }) : common("sendCode")}
                  </Button>
                ) : null}
              </div>
              {currentDebugCode ? <p className="text-xs font-medium text-muted-foreground">{common("debugCode", { code: currentDebugCode })}</p> : null}
              {alternativeCurrentMethod ? (
                <div className="flex justify-center pt-2">
                  <Button
                    type="button"
                    variant="link"
                    className="h-auto px-0 text-xs font-medium text-muted-foreground hover:text-foreground"
                    disabled={disabled}
                    onClick={() => onCurrentVerificationMethodChange(alternativeCurrentMethod)}
                  >
                    {alternativeCurrentMethod === "email" ? verification("actions.useEmail") : verification("actions.useTwoFactor")}
                  </Button>
                </div>
              ) : null}
            </div>
          ) : null}
          {!isCurrentStep ? (
            <>
              <div className="space-y-1">
                <p className="text-xs text-muted-foreground">{verification("labels.newEmail")}</p>
                <Input id="new-email" type="email" value={email} disabled={disabled} onChange={(event) => onEmailChange(event.target.value)} />
              </div>
              {emailVerificationEnabled ? (
                <div className="space-y-1">
                  <p className="text-xs text-muted-foreground">{verification("labels.verificationCode")}</p>
                  <div className="flex gap-2">
                    <Input
                      type="text"
                      value={newCode}
                      inputMode="numeric"
                      autoComplete="one-time-code"
                      pattern="[0-9]*"
                      placeholder={verification("placeholders.sixDigitCode")}
                      disabled={disabled}
                      onChange={(event) => setNewCode(sanitizeVerificationCode("email", event.target.value))}
                      className="min-w-0"
                    />
                    <Button
                      type="button"
                      variant="secondary"
                      className="min-w-[4.5rem] shrink-0 px-3 shadow-none"
                      disabled={newResendDisabled || !email.trim()}
                      onClick={() => void onSendNewCode()}
                    >
                      {sendingCode ? <SpinnerLabel>{common("sending")}</SpinnerLabel> : newCodeCooldownSeconds > 0 ? common("resendIn", { seconds: newCodeCooldownSeconds }) : common("sendCode")}
                    </Button>
                  </div>
                  {debugCode ? <p className="text-xs font-medium text-muted-foreground">{common("debugCode", { code: debugCode })}</p> : null}
                </div>
              ) : null}
            </>
          ) : null}
        </div>
        <DialogFooter>
          <Button variant="ghost" disabled={pending} onClick={() => onOpenChange(false)}>{common("cancel")}</Button>
          {isCurrentStep ? (
            <Button
              type="button"
              disabled={disabled || !isVerificationCodeReady(currentVerificationMethod, currentCode)}
              onClick={() => setStep("email")}
            >
              {common("continue")}
            </Button>
          ) : (
            <>
              {needsCurrentVerification ? (
                <Button type="button" variant="ghost" disabled={pending} onClick={() => setStep("current")}>{common("back")}</Button>
              ) : null}
              <Button
                type="button"
                disabled={submitDisabled}
                onClick={() => void onSubmit({ currentVerificationMethod, currentCode, newCode })}
              >
                {pending ? <SpinnerLabel>{common("saving")}</SpinnerLabel> : emailVerificationEnabled ? common("verify") : common("save")}
              </Button>
            </>
          )}
        </DialogFooter>
      </DialogContent>
    </Dialog>
  );
}

export function CurrentEmailVerificationDialog({
  open,
  onOpenChange,
  email,
  pending,
  sendingCode,
  resendCooldownSeconds,
  debugCode,
  onSendCode,
  onSubmit,
}: {
  open: boolean;
  onOpenChange: (open: boolean) => void;
  email: string;
  pending: boolean;
  sendingCode: boolean;
  resendCooldownSeconds: number;
  debugCode: string;
  onSendCode: () => Promise<void>;
  onSubmit: (code: string) => Promise<void>;
}) {
  const verification = useTranslations("settings.accountPage.securityDialog.verification");

  return (
    <SecurityVerificationDialog
      open={open}
      onOpenChange={onOpenChange}
      selectedMethod="email"
      availableMethods={["email"]}
      onMethodChange={() => undefined}
      title={verification("title.currentEmail")}
      description={verification("description.currentEmail", { email })}
      debugCode={debugCode}
      pending={pending}
      sendingCode={sendingCode}
      resendCooldownSeconds={resendCooldownSeconds}
      onSendCode={() => onSendCode()}
      onSubmit={(code) => onSubmit(code)}
    />
  );
}

export function ChangePasswordDialog({
  open,
  onOpenChange,
  passwordEnabled,
  pending,
  sendingCode,
  resendCooldownSeconds,
  debugCode,
  verificationMethods,
  required = false,
  onSendCode,
  onSubmit,
}: {
  open: boolean;
  onOpenChange: (open: boolean) => void;
  passwordEnabled: boolean;
  pending: boolean;
  sendingCode: boolean;
  resendCooldownSeconds: number;
  debugCode: string;
  verificationMethods: SecurityVerificationMethod[];
  required?: boolean;
  onSendCode: (method: SecurityVerificationMethod) => Promise<void>;
  onSubmit: (payload: { currentPassword: string; newPassword: string; verificationMethod: SecurityVerificationMethod; code: string }) => Promise<void>;
}) {
  const t = useTranslations("settings.accountPage.securityDialog.password");
  const common = useTranslations("settings.accountPage.securityDialog.common");
  const [currentPassword, setCurrentPassword] = React.useState("");
  const [newPassword, setNewPassword] = React.useState("");
  const [verificationOpen, setVerificationOpen] = React.useState(false);
  const [selectedVerificationMethod, setSelectedVerificationMethod] = React.useState<SecurityVerificationMethod>(verificationMethods[0] ?? "none");
  const [pendingPayload, setPendingPayload] = React.useState<{ currentPassword: string; newPassword: string } | null>(null);

  React.useEffect(() => {
    if (!open) {
      setCurrentPassword("");
      setNewPassword("");
      setVerificationOpen(false);
      setSelectedVerificationMethod(verificationMethods[0] ?? "none");
      setPendingPayload(null);
    }
  }, [open, verificationMethods]);

  const disabled = pending || sendingCode;
  const dialogTitle = required ? t("title.required") : passwordEnabled ? t("title.change") : t("title.set");
  const dialogDescription = required
    ? t("description.required")
    : passwordEnabled
    ? t("description.change")
    : t("description.set");
  const passwordLabel = passwordEnabled ? t("labels.newPassword") : t("labels.loginPassword");
  const currentPasswordValue = passwordEnabled ? currentPassword : "";
  const submitDisabled = disabled || (passwordEnabled && !currentPassword) || !isPasswordPolicyValid(newPassword);

  const handleSave = React.useCallback(() => {
    const payload = { currentPassword: currentPasswordValue, newPassword };
    const method = verificationMethods[0] ?? "none";
    if (required) {
      void onSubmit({ ...payload, verificationMethod: "none", code: "" });
      return;
    }
    if (method === "none") {
      void onSubmit({ ...payload, verificationMethod: method, code: "" });
      return;
    }
    setSelectedVerificationMethod(method);
    setPendingPayload(payload);
    setVerificationOpen(true);
  }, [currentPasswordValue, newPassword, onSubmit, required, verificationMethods]);

  return (
    <>
      <Dialog open={open} onOpenChange={(nextOpen) => {
        if (required && !nextOpen) return;
        onOpenChange(nextOpen);
      }}>
        <DialogContent>
          <DialogHeader>
            <DialogTitle>{dialogTitle}</DialogTitle>
            <DialogDescription>{dialogDescription}</DialogDescription>
          </DialogHeader>
          <div className="space-y-4">
            {passwordEnabled ? (
              <div className="space-y-1">
                <p className="text-xs text-muted-foreground">{t("labels.currentPassword")}</p>
                <Input
                  id="current-password"
                  type="password"
                  autoComplete="current-password"
                  value={currentPassword}
                  onChange={(event) => setCurrentPassword(event.target.value)}
                  disabled={disabled}
                />
              </div>
            ) : null}
            <div className="space-y-1">
              <p className="text-xs text-muted-foreground">{passwordLabel}</p>
              <Input
                id="new-password"
                type="password"
                autoComplete="new-password"
                placeholder={t("placeholders.password")}
                value={newPassword}
                onChange={(event) => setNewPassword(event.target.value)}
                disabled={disabled}
                minLength={PASSWORD_MIN_LENGTH}
              />
            </div>
          </div>
          <DialogFooter>
            {required ? null : <Button variant="ghost" disabled={pending} onClick={() => onOpenChange(false)}>{common("cancel")}</Button>}
            <Button
              type="button"
              disabled={submitDisabled}
              onClick={handleSave}
            >
              {disabled ? <SpinnerLabel>{pending ? common("saving") : common("processing")}</SpinnerLabel> : common("save")}
            </Button>
          </DialogFooter>
        </DialogContent>
      </Dialog>
      <SecurityVerificationDialog
        open={verificationOpen}
        onOpenChange={setVerificationOpen}
        selectedMethod={selectedVerificationMethod}
        availableMethods={verificationMethods}
        onMethodChange={setSelectedVerificationMethod}
        title={t("verificationTitle")}
        description={selectedVerificationMethod === "two_factor" ? t("verificationDescription.twoFactor") : t("verificationDescription.email")}
        debugCode={selectedVerificationMethod === "email" ? debugCode : ""}
        pending={pending}
        sendingCode={sendingCode}
        resendCooldownSeconds={resendCooldownSeconds}
        onSendCode={onSendCode}
        onSubmit={(code, method) => onSubmit({ ...(pendingPayload ?? { currentPassword: currentPasswordValue, newPassword }), verificationMethod: method, code })}
      />
    </>
  );
}

export function EmailSecurityDialog({
  open,
  onOpenChange,
  bootstrap,
  emailVerificationEnabled,
  currentVerificationMethods,
  pending,
  sendingCode,
  currentCodeCooldownSeconds,
  newCodeCooldownSeconds,
  debugCode,
  currentDebugCode,
  onSendBootstrapCode,
  onCompleteBootstrap,
  onSendCurrentCode,
  onSendNewCode,
  onCompleteChange,
}: {
  open: boolean;
  onOpenChange: (open: boolean) => void;
  bootstrap: boolean;
  emailVerificationEnabled: boolean;
  currentVerificationMethods: SecurityVerificationMethod[];
  pending: boolean;
  sendingCode: boolean;
  currentCodeCooldownSeconds: number;
  newCodeCooldownSeconds: number;
  debugCode: string;
  currentDebugCode: string;
  onSendBootstrapCode: (email: string) => Promise<void>;
  onCompleteBootstrap: (payload: { email: string; code: string }) => Promise<void>;
  onSendCurrentCode: (method: SecurityVerificationMethod) => Promise<void>;
  onSendNewCode: (email: string) => Promise<void>;
  onCompleteChange: (payload: { email: string; currentVerificationMethod: SecurityVerificationMethod; currentCode: string; newCode: string }) => Promise<void>;
}) {
  const t = useTranslations("settings.accountPage.securityDialog.email");
  const common = useTranslations("settings.accountPage.securityDialog.common");
  const [email, setEmail] = React.useState("");
  const [selectedCurrentVerificationMethod, setSelectedCurrentVerificationMethod] = React.useState<SecurityVerificationMethod>(currentVerificationMethods[0] ?? "none");

  React.useEffect(() => {
    if (!open) {
      setEmail("");
      setSelectedCurrentVerificationMethod(currentVerificationMethods[0] ?? "none");
    }
  }, [currentVerificationMethods, open]);

  const disabled = pending || sendingCode;
  const currentVerificationMethod = bootstrap ? "none" : (currentVerificationMethods[0] ?? "none");
  const needsStagedVerification = emailVerificationEnabled || currentVerificationMethod !== "none";
  const description = bootstrap
    ? emailVerificationEnabled
      ? t("description.bootstrapVerified")
      : t("description.saveOnly")
    : emailVerificationEnabled
      ? t("description.change")
      : t("description.saveOnly");

  const handleSave = React.useCallback(() => {
    void (bootstrap
      ? onCompleteBootstrap({ email, code: "" })
      : onCompleteChange({ email, currentVerificationMethod, currentCode: "", newCode: "" }));
  }, [bootstrap, currentVerificationMethod, email, onCompleteBootstrap, onCompleteChange]);

  if (needsStagedVerification) {
    return (
      <EmailChangeVerificationDialog
        open={open}
        onOpenChange={onOpenChange}
        bootstrap={bootstrap}
        email={email}
        onEmailChange={setEmail}
        emailVerificationEnabled={emailVerificationEnabled}
        currentVerificationMethod={selectedCurrentVerificationMethod}
        currentVerificationMethods={currentVerificationMethods}
        onCurrentVerificationMethodChange={setSelectedCurrentVerificationMethod}
        pending={pending}
        sendingCode={sendingCode}
        currentCodeCooldownSeconds={currentCodeCooldownSeconds}
        newCodeCooldownSeconds={newCodeCooldownSeconds}
        debugCode={debugCode}
        currentDebugCode={selectedCurrentVerificationMethod === "email" ? currentDebugCode : ""}
        onSendCurrentCode={onSendCurrentCode}
        onSendNewCode={() => (bootstrap ? onSendBootstrapCode(email) : onSendNewCode(email))}
        onSubmit={({ currentVerificationMethod, currentCode, newCode }) => (bootstrap
          ? onCompleteBootstrap({ email, code: newCode })
          : onCompleteChange({ email, currentVerificationMethod, currentCode, newCode }))}
      />
    );
  }

  return (
    <Dialog open={open} onOpenChange={onOpenChange}>
      <DialogContent className="sm:max-w-[480px]">
        <DialogHeader>
          <DialogTitle>{bootstrap ? t("title.set") : t("title.change")}</DialogTitle>
          <DialogDescription>{description}</DialogDescription>
        </DialogHeader>
        <div className="space-y-4">
          <div className="space-y-1">
            <p className="text-xs text-muted-foreground">{t("labels.newEmail")}</p>
            <Input id="new-email" type="email" value={email} disabled={disabled} onChange={(event) => setEmail(event.target.value)} />
          </div>
        </div>
        <DialogFooter>
          <Button variant="ghost" disabled={pending} onClick={() => onOpenChange(false)}>{common("cancel")}</Button>
          <Button
            type="button"
            disabled={disabled || !email}
            onClick={handleSave}
          >
            {disabled ? <SpinnerLabel>{pending ? common("saving") : common("processing")}</SpinnerLabel> : common("save")}
          </Button>
        </DialogFooter>
      </DialogContent>
    </Dialog>
  );
}

export function TwoFactorDialog({
  open,
  onOpenChange,
  enabled,
  setupSecret,
  setupURL,
  setupExpiresAt,
  recoveryCodes,
  onStartSetup,
  onConfirmSetup,
  onDisable,
  onRegenerateRecoveryCodes,
  onCancelSetup,
  onClearRecoveryCodes,
}: {
  open: boolean;
  onOpenChange: (open: boolean) => void;
  enabled: boolean;
  setupSecret: string;
  setupURL: string;
  setupExpiresAt: string;
  recoveryCodes: string[];
  onStartSetup: () => Promise<boolean>;
  onConfirmSetup: (code: string) => Promise<void>;
  onDisable: (code: string) => Promise<boolean>;
  onRegenerateRecoveryCodes: (code: string) => Promise<void>;
  onCancelSetup: () => Promise<void>;
  onClearRecoveryCodes: () => void;
}) {
  type TwoFactorDialogMode = "setup" | "manage" | "recovery";
  const t = useTranslations("settings.accountPage.securityDialog.twoFactor");
  const common = useTranslations("settings.accountPage.securityDialog.common");
  const { locale } = useAppLocale();
  const [code, setCode] = React.useState("");
  const [actionPending, setActionPending] = React.useState(false);
  const codeInputRef = React.useRef<HTMLInputElement | null>(null);
  const [setupView, setSetupView] = React.useState<"qr" | "manual">("qr");
  const [closingSnapshot, setClosingSnapshot] = React.useState<{
    mode: TwoFactorDialogMode;
    setupSecret: string;
    setupURL: string;
    setupExpiresAt: string;
    recoveryCodes: string[];
  } | null>(null);
  const previousOpenRef = React.useRef(open);
  const [now, setNow] = React.useState(() => Date.now());
  const mode: TwoFactorDialogMode = recoveryCodes.length > 0 ? "recovery" : enabled ? "manage" : "setup";
  const displayMode = closingSnapshot?.mode ?? mode;
  const displaySetupSecret = closingSnapshot?.setupSecret ?? setupSecret;
  const displaySetupURL = closingSnapshot?.setupURL ?? setupURL;
  const displaySetupExpiresAt = closingSnapshot?.setupExpiresAt ?? setupExpiresAt;
  const displayRecoveryCodes = closingSnapshot?.recoveryCodes ?? recoveryCodes;
  const qrCodeSVG = React.useMemo(
    () => (displaySetupURL ? createQRCodeSVG(displaySetupURL, 3, t("qrLabel")) : ""),
    [displaySetupURL, t],
  );
  const qrCodeUnavailable = Boolean(displaySetupURL && !qrCodeSVG);
  const setupExpiresAtTime = displaySetupExpiresAt ? new Date(displaySetupExpiresAt).getTime() : 0;
  const setupRemaining = setupExpiresAtTime ? setupExpiresAtTime - now : 0;
  const setupExpired = Boolean(displaySetupSecret && setupExpiresAtTime && setupRemaining <= 0);
  const dialogTitle = displayMode === "setup" ? t("title.setup") : displayMode === "manage" ? t("title.manage") : t("title.recovery");
  const dialogDescription = displayMode === "setup"
    ? t("description.setup")
    : displayMode === "manage"
      ? t("description.manage")
      : t("description.recovery");
  const copyText = React.useCallback(async (value: string, label: string) => {
    try {
      await navigator.clipboard.writeText(value);
      toast.success(common("copiedLabel", { label }));
    } catch {
      toast.error(common("copyFailed"), { description: common("copyManually") });
    }
  }, [common]);

  React.useEffect(() => {
    if (open && !previousOpenRef.current) {
      setClosingSnapshot(null);
    }
    previousOpenRef.current = open;
  }, [open]);

  React.useEffect(() => {
    if (!open) {
      setCode("");
      setActionPending(false);
      setSetupView("qr");
    }
  }, [open]);

  React.useEffect(() => {
    if (displaySetupSecret) {
      setSetupView("qr");
    }
  }, [displaySetupSecret]);

  React.useEffect(() => {
    if (!open || !displaySetupSecret || !setupExpiresAtTime) {
      return undefined;
    }
    setNow(Date.now());
    const timer = window.setInterval(() => setNow(Date.now()), 1000);
    return () => window.clearInterval(timer);
  }, [displaySetupSecret, open, setupExpiresAtTime]);

  const readCurrentCode = React.useCallback(() => {
    return (codeInputRef.current?.value || code).trim();
  }, [code]);
  const requireCurrentCode = React.useCallback((digitsOnly: boolean) => {
    const currentCode = digitsOnly
      ? readCurrentCode().replace(/\D/g, "").slice(0, 6)
      : readCurrentCode().replace(/[^a-zA-Z0-9-]/g, "").slice(0, 32);
    if (currentCode.length < 6) {
      codeInputRef.current?.focus();
      toast.error(t("errors.codeRequired"));
      return "";
    }
    setCode(currentCode);
    return currentCode;
  }, [readCurrentCode, t]);
  const handleTwoFactorOtpSubmit = React.useCallback((event: React.FormEvent<HTMLFormElement>) => {
    event.preventDefault();
  }, []);
  const snapshotCurrentDialog = React.useCallback(() => {
    return {
      mode,
      setupSecret,
      setupURL,
      setupExpiresAt,
      recoveryCodes,
    };
  }, [mode, recoveryCodes, setupExpiresAt, setupSecret, setupURL]);
  const closeDialog = React.useCallback(() => {
    setClosingSnapshot(snapshotCurrentDialog());
    onOpenChange(false);
    if (mode === "setup" && setupSecret) {
      void onCancelSetup();
    } else {
      onClearRecoveryCodes();
    }
  }, [mode, onCancelSetup, onClearRecoveryCodes, onOpenChange, setupSecret, snapshotCurrentDialog]);

  return (
    <Dialog
      open={open}
      onOpenChange={(nextOpen) => {
        if (!nextOpen && actionPending) {
          return;
        }
        if (!nextOpen) {
          closeDialog();
          return;
        }
        onOpenChange(nextOpen);
      }}
    >
      <DialogContent className="sm:max-w-[480px]">
        <DialogHeader>
          <DialogTitle>{dialogTitle}</DialogTitle>
          <DialogDescription>{dialogDescription}</DialogDescription>
        </DialogHeader>
        <div className="space-y-4">
          {displayMode === "setup" && displaySetupSecret ? (
            <>
              <div className="grid grid-cols-2 gap-1 rounded-md bg-muted/35 p-1">
                <Button type="button" variant={setupView === "qr" ? "secondary" : "ghost"} className="h-8 px-3 text-xs shadow-none" onClick={() => setSetupView("qr")}>
                  {t("tabs.qr")}
                </Button>
                <Button type="button" variant={setupView === "manual" ? "secondary" : "ghost"} className="h-8 px-3 text-xs shadow-none" onClick={() => setSetupView("manual")}>
                  {t("tabs.manual")}
                </Button>
              </div>
              {setupView === "qr" ? (
                <div className="space-y-3">
                  <div className="flex justify-center">
                    {qrCodeSVG ? (
                      <div
                        className="flex size-48 items-center justify-center [&_svg]:size-full"
                        dangerouslySetInnerHTML={{ __html: qrCodeSVG }}
                      />
                    ) : (
                      <div className="flex size-48 items-center justify-center rounded-lg border border-border/60 bg-muted/20 px-5 text-center text-xs leading-5 text-muted-foreground">
                        {qrCodeUnavailable ? t("qrUnavailable") : <SpinnerLabel>{common("processing")}</SpinnerLabel>}
                      </div>
                    )}
                  </div>
                  {displaySetupExpiresAt ? (
                    <div className="space-y-1 text-center text-xs text-muted-foreground">
                      <p>{setupExpired ? t("setupExpired") : t("scanThenVerify")}</p>
                      {setupExpired ? (
                        <Button type="button" variant="ghost" className="h-7 px-2 text-xs" onClick={() => void onStartSetup()}>
                          {t("regenerate")}
                        </Button>
                      ) : (
                        <p>{t("validUntil", { time: formatDateTime(displaySetupExpiresAt, locale) })}</p>
                      )}
                    </div>
                  ) : null}
                </div>
              ) : null}
              {setupView === "manual" ? (
                <div className="space-y-3">
                  <div className="space-y-1">
                    <p className="text-xs text-muted-foreground">{t("secret")}</p>
                    <div className="flex gap-2">
                      <Input value={displaySetupSecret} readOnly className="min-w-0" />
                      <Button
                        type="button"
                        variant="ghost"
                        size="icon"
                        className="size-9 shrink-0 text-muted-foreground shadow-none"
                        onClick={() => void copyText(displaySetupSecret, t("secret"))}
                        aria-label={t("copySecret")}
                        title={t("copySecret")}
                      >
                        <Copy className="size-3.5" />
                      </Button>
                    </div>
                  </div>
                  <div className="space-y-1">
                    <p className="text-xs text-muted-foreground">{t("otpAuthUri")}</p>
                    <div className="flex gap-2">
                      <Input value={displaySetupURL} readOnly className="min-w-0" />
                      <Button
                        type="button"
                        variant="ghost"
                        size="icon"
                        className="size-9 shrink-0 text-muted-foreground shadow-none"
                        onClick={() => void copyText(displaySetupURL, t("otpAuthUri"))}
                        aria-label={t("copyOtpUri")}
                        title={t("copyOtpUri")}
                      >
                        <Copy className="size-3.5" />
                      </Button>
                    </div>
                  </div>
                </div>
              ) : null}
              <form className="space-y-1" onSubmit={handleTwoFactorOtpSubmit}>
                <p className="text-xs text-muted-foreground">{t("code")}</p>
                <Input
                  ref={codeInputRef}
                  id="otp"
                  name="otp"
                  type="text"
                  inputMode="numeric"
                  autoComplete="one-time-code"
                  pattern="[0-9]*"
                  placeholder={t("sixDigitCode")}
                  value={code}
                  onInput={(event) => setCode(event.currentTarget.value.replace(/\D/g, "").slice(0, 6))}
                  onChange={(event) => setCode(event.target.value.replace(/\D/g, "").slice(0, 6))}
                />
              </form>
            </>
          ) : null}
          {displayMode === "manage" ? (
            <form className="space-y-1" onSubmit={handleTwoFactorOtpSubmit}>
              <p className="text-xs text-muted-foreground">{t("codeOrRecovery")}</p>
              <div className="flex gap-2">
                <Input
                  ref={codeInputRef}
                  name="otp"
                  type="text"
                  autoComplete="one-time-code"
                  placeholder={t("codeOrRecovery")}
                  value={code}
                  className="min-w-0"
                  onInput={(event) => setCode(event.currentTarget.value.replace(/[^a-zA-Z0-9-]/g, "").slice(0, 32))}
                  onChange={(event) => setCode(event.target.value.replace(/[^a-zA-Z0-9-]/g, "").slice(0, 32))}
                />
                <Button
                  type="button"
                  variant="outline"
                  className="shrink-0"
                  disabled={actionPending}
                  onClick={() => {
                    const currentCode = requireCurrentCode(false);
                    if (!currentCode) return;
                    setActionPending(true);
                    void onRegenerateRecoveryCodes(currentCode).finally(() => setActionPending(false));
                  }}
                >
                  {actionPending ? <SpinnerLabel>{common("processing")}</SpinnerLabel> : t("regenerateRecoveryCodes")}
                </Button>
              </div>
            </form>
          ) : null}
          {displayMode === "recovery" ? (
            <div className="space-y-2">
              <div className="flex items-center justify-between gap-2">
                <p className="text-xs text-muted-foreground">{t("recoveryCodes")}</p>
                <Button
                  type="button"
                  variant="ghost"
                  size="icon"
                  className="size-7 text-muted-foreground shadow-none"
                  onClick={() => void copyText(displayRecoveryCodes.join("\n"), t("recoveryCodes"))}
                  aria-label={t("copyRecoveryCodes")}
                  title={t("copyRecoveryCodes")}
                >
                  <Copy className="size-3.5" />
                </Button>
              </div>
              <div className="grid grid-cols-2 gap-2 rounded-md bg-muted/35 p-3 text-xs text-muted-foreground">
                {displayRecoveryCodes.map((item) => (
                  <span key={item}>{item}</span>
                ))}
              </div>
            </div>
          ) : null}
        </div>
        <DialogFooter>
          {displayMode === "recovery" ? (
            <>
              <Button
                type="button"
                variant="outline"
                disabled={actionPending}
                onClick={() => {
                  const currentCode = requireCurrentCode(false);
                  if (!currentCode) return;
                  setActionPending(true);
                  void onRegenerateRecoveryCodes(currentCode).finally(() => setActionPending(false));
                }}
              >
                {actionPending ? <SpinnerLabel>{common("processing")}</SpinnerLabel> : t("regenerateRecoveryCodes")}
              </Button>
              <Button type="button" disabled={actionPending} onClick={closeDialog}>
                {common("done")}
              </Button>
            </>
          ) : (
            <Button variant="ghost" disabled={actionPending} onClick={closeDialog}>
              {common("cancel")}
            </Button>
          )}
          {displayMode === "setup" && displaySetupSecret ? (
            <Button
              type="button"
              disabled={setupExpired || actionPending}
              onClick={() => {
                const currentCode = requireCurrentCode(true);
                if (!currentCode) return;
                setActionPending(true);
                void onConfirmSetup(currentCode).finally(() => setActionPending(false));
              }}
            >
              {actionPending ? <SpinnerLabel>{common("processing")}</SpinnerLabel> : t("enable")}
            </Button>
          ) : null}
          {displayMode === "manage" ? (
            <Button
              type="button"
              variant="destructive"
              disabled={actionPending}
              onClick={() => {
                const currentCode = requireCurrentCode(false);
                if (currentCode) {
                  const snapshot = snapshotCurrentDialog();
                  setClosingSnapshot(snapshot);
                  setActionPending(true);
                  void onDisable(currentCode).then((disabled) => {
                    if (disabled) {
                      onOpenChange(false);
                      onClearRecoveryCodes();
                    } else {
                      setClosingSnapshot(null);
                    }
                  }).finally(() => setActionPending(false));
                }
              }}
            >
              {actionPending ? <SpinnerLabel>{common("processing")}</SpinnerLabel> : t("disable")}
            </Button>
          ) : null}
        </DialogFooter>
      </DialogContent>
    </Dialog>
  );
}
