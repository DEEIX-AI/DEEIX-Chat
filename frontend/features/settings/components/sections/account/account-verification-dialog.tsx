"use client";

import * as React from "react";
import { useTranslations } from "next-intl";

import { Button } from "@/components/ui/button";
import { Dialog, DialogContent, DialogDescription, DialogFooter, DialogHeader, DialogTitle } from "@/components/ui/dialog";
import { Input } from "@/components/ui/input";
import { SpinnerLabel } from "@/components/ui/spinner";
import type { SecurityVerificationMethod } from "@/shared/api/auth.types";

export function sanitizeVerificationCode(method: SecurityVerificationMethod, value: string) {
  if (method === "two_factor") {
    return value.replace(/[^a-zA-Z0-9-]/g, "").slice(0, 32);
  }
  return value.replace(/\D/g, "").slice(0, 6);
}

export function isVerificationCodeReady(method: SecurityVerificationMethod, value: string) {
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
