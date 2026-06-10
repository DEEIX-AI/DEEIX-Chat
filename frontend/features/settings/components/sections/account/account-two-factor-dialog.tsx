"use client";

import * as React from "react";
import { useTranslations } from "next-intl";
import { toast } from "sonner";

import { Button } from "@/components/ui/button";
import { Dialog, DialogContent, DialogDescription, DialogFooter, DialogHeader, DialogTitle } from "@/components/ui/dialog";
import { Input } from "@/components/ui/input";
import { SpinnerLabel } from "@/components/ui/spinner";
import { formatDateTime } from "@/features/settings/model/account-page";
import { useAppLocale } from "@/i18n/app-i18n-provider";
import { CopyActionButton } from "@/shared/components/copy-action";
import { createQRCodeSVG } from "@/shared/lib/qr-code";

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
  const copyMessages = React.useMemo(() => ({
    copied: common("copiedLabel", { label: "" }).trim(),
    failed: common("copyFailed"),
    failedDescription: common("copyManually"),
  }), [common]);
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
                      <CopyActionButton
                        type="button"
                        variant="ghost"
                        size="icon"
                        className="size-9 shrink-0 text-muted-foreground shadow-none"
                        value={displaySetupSecret}
                        messages={copyMessages}
                        copyOptions={{ copied: common("copiedLabel", { label: t("secret") }) }}
                        aria-label={t("copySecret")}
                        title={t("copySecret")}
                      />
                    </div>
                  </div>
                  <div className="space-y-1">
                    <p className="text-xs text-muted-foreground">{t("otpAuthUri")}</p>
                    <div className="flex gap-2">
                      <Input value={displaySetupURL} readOnly className="min-w-0" />
                      <CopyActionButton
                        type="button"
                        variant="ghost"
                        size="icon"
                        className="size-9 shrink-0 text-muted-foreground shadow-none"
                        value={displaySetupURL}
                        messages={copyMessages}
                        copyOptions={{ copied: common("copiedLabel", { label: t("otpAuthUri") }) }}
                        aria-label={t("copyOtpUri")}
                        title={t("copyOtpUri")}
                      />
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
                <CopyActionButton
                  type="button"
                  variant="ghost"
                  size="icon"
                  className="size-7 text-muted-foreground shadow-none"
                  value={displayRecoveryCodes.join("\n")}
                  messages={copyMessages}
                  copyOptions={{ copied: common("copiedLabel", { label: t("recoveryCodes") }) }}
                  aria-label={t("copyRecoveryCodes")}
                  title={t("copyRecoveryCodes")}
                />
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
