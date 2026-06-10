"use client";

import * as React from "react";
import { useTranslations } from "next-intl";

import { Button } from "@/components/ui/button";
import { Dialog, DialogContent, DialogDescription, DialogFooter, DialogHeader, DialogTitle } from "@/components/ui/dialog";
import { Input } from "@/components/ui/input";
import { SpinnerLabel } from "@/components/ui/spinner";
import type { SecurityVerificationMethod } from "@/shared/api/auth.types";
import { SecurityVerificationDialog, isVerificationCodeReady, sanitizeVerificationCode } from "./account-verification-dialog";

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
