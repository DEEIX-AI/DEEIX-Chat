"use client";

import * as React from "react";
import { useTranslations } from "next-intl";

import { Button } from "@/components/ui/button";
import { Dialog, DialogContent, DialogDescription, DialogFooter, DialogHeader, DialogTitle } from "@/components/ui/dialog";
import { Input } from "@/components/ui/input";
import { SpinnerLabel } from "@/components/ui/spinner";
import type { SecurityVerificationMethod } from "@/shared/api/auth.types";
import { PASSWORD_MIN_LENGTH, isPasswordPolicyValid } from "@/shared/auth/account-policy";
import { SecurityVerificationDialog } from "./account-verification-dialog";

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
