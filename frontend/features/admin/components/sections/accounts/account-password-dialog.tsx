"use client";

import { useTranslations } from "next-intl";

import { Button } from "@/components/ui/button";
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from "@/components/ui/dialog";
import { Input } from "@/components/ui/input";
import { SpinnerLabel } from "@/components/ui/spinner";
import { PASSWORD_MIN_LENGTH, isPasswordPolicyValid } from "@/shared/auth/account-policy";

export function AccountPasswordResetDialog({
  open,
  onOpenChange,
  pending,
  password,
  onPasswordChange,
  onConfirm,
  onCancel,
}: {
  open: boolean;
  onOpenChange: (open: boolean) => void;
  pending: boolean;
  password: string;
  onPasswordChange: (value: string) => void;
  onConfirm: () => void;
  onCancel: () => void;
}) {
  const t = useTranslations("adminUsers.passwordDialog");
  const disabled = pending;
  return (
    <Dialog open={open} onOpenChange={onOpenChange}>
      <DialogContent>
        <DialogHeader>
          <DialogTitle>{t("title")}</DialogTitle>
          <DialogDescription>
            {t("description")}
          </DialogDescription>
        </DialogHeader>

        <div className="space-y-4">
          <div className="space-y-1">
            <p className="text-xs text-muted-foreground">{t("newPassword")}</p>
            <Input
              type="password"
              value={password}
              placeholder={t("passwordPlaceholder")}
              disabled={disabled}
              minLength={PASSWORD_MIN_LENGTH}
              onChange={(event) => onPasswordChange(event.target.value)}
            />
          </div>
        </div>

        <DialogFooter>
          <Button variant="ghost" onClick={onCancel} disabled={pending}>
            {t("cancel")}
          </Button>
          <Button type="button" onClick={onConfirm} disabled={disabled || !isPasswordPolicyValid(password)}>
            {pending ? <SpinnerLabel>{t("resetting")}</SpinnerLabel> : t("reset")}
          </Button>
        </DialogFooter>
      </DialogContent>
    </Dialog>
  );
}
